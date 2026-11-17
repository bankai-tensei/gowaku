package d3d11

import (
	"fmt"
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// CreateTarget implements gpu.Backend.
func (d *Device) CreateTarget(width, height int) (gpu.Texture, error) {
	texture, err := d.device.createTexture2D(&D3D11_TEXTURE2D_DESC{
		Width:      uint32(width),
		Height:     uint32(height),
		MipLevels:  1,
		ArraySize:  1,
		Format:     DXGI_FORMAT_R8G8B8A8_UNORM,
		SampleDesc: DXGI_SAMPLE_DESC{Count: 1},
		Usage:      D3D11_USAGE_DEFAULT,
		BindFlags:  D3D11_BIND_SHADER_RESOURCE | D3D11_BIND_RENDER_TARGET,
	})
	if err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	t := &Texture{device: d, texture: texture, format: gpu.TextureRGBA8, width: width, height: height}
	if t.view, err = d.device.createShaderResourceView(texture); err != nil {
		texture.Release()
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	if t.target, err = d.device.createRenderTargetView(texture); err != nil {
		unknownOf(t.view).Release()
		texture.Release()
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	d.clearTarget(t, image.Rect(0, 0, width, height))
	return t, nil
}

// clearTarget makes r of t transparent.
func (d *Device) clearTarget(t *Texture, r image.Rectangle) {
	d.clearRect = D3D11_RECT{left: int32(r.Min.X), top: int32(r.Min.Y), right: int32(r.Max.X), bottom: int32(r.Max.Y)}
	d.context1.clearView(t.target, &d.transparent, &d.clearRect)
}

// DrawToTarget implements gpu.Backend. Without a depth buffer the depth tests of
// the pipeline pass, which is what level 0 of the clips asks for.
func (d *Device) DrawToTarget(t gpu.Texture, batch *gpu.Batch, scale float32) error {
	tx := t.(*Texture)
	if tx.target == nil {
		panic("gowaku: DrawToTarget called on a texture that CreateTarget did not make")
	}
	if batch.Multisampled() {
		return d.drawMultisampled(tx, batch, scale)
	}
	d.startTiming()
	for _, r := range batch.Clears {
		d.clearTarget(tx, r)
	}
	if len(batch.Instances) == 0 {
		return nil
	}
	sf := &d.targetSurface
	sf.viewport = D3D11_VIEWPORT{Width: float32(tx.width), Height: float32(tx.height), MaxDepth: 1}
	sf.scale = scale
	sf.rtv, sf.dsv = tx.target, nil
	sf.resolving, sf.color = false, nil
	sf.drawn = tx
	if err := d.writeConstants(sf); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	// The texture cannot be read and written at once, and the last frame can
	// have left it bound, as an image or as tex1 of a custom shader.
	var none *ID3D11ShaderResourceView
	d.context.psSetShaderResource(0, &none)
	d.context.psSetShaderResource(1, &none)
	d.bindFrame(sf, nil)
	if err := d.drawBatch(sf, batch); err != nil {
		return err
	}
	var unbound *ID3D11RenderTargetView
	d.context.omSetRenderTarget(&unbound, nil)
	return nil
}
