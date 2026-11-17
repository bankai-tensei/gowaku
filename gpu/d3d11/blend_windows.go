package d3d11

import (
	"fmt"
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// blendFactors maps the factors of gpu to those of Direct3D.
var blendFactors = [...]uint32{
	gpu.FactorZero:        D3D11_BLEND_ZERO,
	gpu.FactorOne:         D3D11_BLEND_ONE,
	gpu.FactorSrcColor:    D3D11_BLEND_SRC_COLOR,
	gpu.FactorInvSrcColor: D3D11_BLEND_INV_SRC_COLOR,
	gpu.FactorSrcAlpha:    D3D11_BLEND_SRC_ALPHA,
	gpu.FactorInvSrcAlpha: D3D11_BLEND_INV_SRC_ALPHA,
	gpu.FactorDstAlpha:    D3D11_BLEND_DEST_ALPHA,
	gpu.FactorInvDstAlpha: D3D11_BLEND_INV_DEST_ALPHA,
}

// alphaFactor returns the factor of the alpha channel that stands for f, which
// cannot name a color there.
func alphaFactor(f gpu.BlendFactor) uint32 {
	switch f {
	case gpu.FactorSrcColor:
		return D3D11_BLEND_SRC_ALPHA
	case gpu.FactorInvSrcColor:
		return D3D11_BLEND_INV_SRC_ALPHA
	}
	return blendFactors[f]
}

// blendState returns the blend state of mode, which it creates on first use.
// Modes with the same factors share one, and source-over is that of the device.
func (d *Device) blendState(mode gpu.BlendMode) (*ID3D11BlendState, error) {
	if int(mode) >= len(d.blends) {
		mode = gpu.BlendSrcOver
	}
	if s := d.blends[mode]; s != nil {
		return s, nil
	}
	state := mode.State()
	if s := d.blends[state]; s != nil {
		d.blends[mode] = s
		return s, nil
	}
	src, dst := state.Factors()
	var desc D3D11_BLEND_DESC
	desc.RenderTarget[0] = D3D11_RENDER_TARGET_BLEND_DESC{
		BlendEnable:           1,
		SrcBlend:              blendFactors[src],
		DestBlend:             blendFactors[dst],
		BlendOp:               D3D11_BLEND_OP_ADD,
		SrcBlendAlpha:         alphaFactor(src),
		DestBlendAlpha:        alphaFactor(dst),
		BlendOpAlpha:          D3D11_BLEND_OP_ADD,
		RenderTargetWriteMask: D3D11_COLOR_WRITE_ENABLE_ALL,
	}
	s, err := d.device.createBlendState(&desc)
	if err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	d.blends[state], d.blends[mode] = s, s
	return s, nil
}

// prepareBlend makes ready what the blend mode of call needs: its state, and for
// a mode that reads the target, the backdrop. It reports whether call draws
// anything at all.
func (d *Device) prepareBlend(sf *Surface, call *gpu.DrawCall) (bool, error) {
	if _, err := d.blendState(call.Blend); err != nil {
		return false, err
	}
	if !call.Blend.ReadsBackdrop() {
		return true, nil
	}
	width, height := int(sf.viewport.Width), int(sf.viewport.Height)
	r := gpu.BackdropRect(call, sf.scale, width, height)
	if r.Empty() {
		return false, nil
	}
	return true, d.copyBackdrop(sf, r)
}

// copyBackdrop copies r of what sf draws to into the backdrop, at the same
// place, and binds it.
func (d *Device) copyBackdrop(sf *Surface, r image.Rectangle) error {
	width, height := int(sf.viewport.Width), int(sf.viewport.Height)
	d.box = D3D11_BOX{left: uint32(r.Min.X), top: uint32(r.Min.Y), right: uint32(r.Max.X), bottom: uint32(r.Max.Y), back: 1}
	var t *Texture
	var err error
	switch {
	case sf.drawn != nil:
		// A target: the pixels move to a scratch texture as large as the
		// largest target, at the same place.
		if t, err = d.backdropTexture(&d.targetBackdrop, width, height, DXGI_FORMAT_R8G8B8A8_UNORM); err != nil {
			return err
		}
		d.context.copySubresourceRegion(t.texture, d.box.left, d.box.top, sf.drawn.texture, &d.box)
	case sf.resolving:
		// Direct3D 11 resolves whole textures only, so the backdrop has the
		// size of the surface. See docs/ADR/0019-layers.md.
		if t, err = d.backdropTexture(&sf.backdrop, width, height, DXGI_FORMAT_B8G8R8A8_UNORM); err != nil {
			return err
		}
		d.context.resolveSubresource(t.texture, sf.color, DXGI_FORMAT_B8G8R8A8_UNORM)
	default:
		if t, err = d.backdropTexture(&sf.backdrop, width, height, DXGI_FORMAT_B8G8R8A8_UNORM); err != nil {
			return err
		}
		back, err := sf.swapChain.backBuffer()
		if err != nil {
			return fmt.Errorf("d3d11: %w", err)
		}
		d.context.copySubresourceRegion(t.texture, d.box.left, d.box.top, back, &d.box)
		back.Release()
	}
	if t != d.boundBackdrop {
		d.boundBackdrop = t
		d.context.psSetShaderResource(3, &t.view)
	}
	return nil
}

// backdropTexture returns *slot, replaced first if it is smaller than width by
// height. A resolve needs the exact size, which every slot of a surface has.
func (d *Device) backdropTexture(slot **Texture, width, height int, format uint32) (*Texture, error) {
	if t := *slot; t != nil && t.width >= width && t.height >= height {
		return t, nil
	}
	if t := *slot; t != nil {
		width, height = max(width, t.width), max(height, t.height)
		t.Destroy()
		*slot = nil
	}
	texture, err := d.device.createTexture2D(&D3D11_TEXTURE2D_DESC{
		Width:      uint32(width),
		Height:     uint32(height),
		MipLevels:  1,
		ArraySize:  1,
		Format:     format,
		SampleDesc: DXGI_SAMPLE_DESC{Count: 1},
		Usage:      D3D11_USAGE_DEFAULT,
		BindFlags:  D3D11_BIND_SHADER_RESOURCE,
	})
	if err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	view, err := d.device.createShaderResourceView(texture)
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	*slot = &Texture{device: d, texture: texture, view: view, format: gpu.TextureRGBA8, width: width, height: height}
	return *slot, nil
}
