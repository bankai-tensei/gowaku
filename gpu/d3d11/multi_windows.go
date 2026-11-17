package d3d11

import (
	"fmt"
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// multiTarget is the scratch on which DrawToTarget draws a batch that needs
// multisampling: color and depth with the samples of the device, and a texture
// of one sample of the same size that the color resolves into, Direct3D 11
// resolving whole textures only. It grows to the largest target. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
type multiTarget struct {
	color, depth, resolved *ID3D11Texture2D
	rtv                    *ID3D11RenderTargetView
	dsv                    *ID3D11DepthStencilView
	width, height          int
}

func (m *multiTarget) release() {
	for _, u := range []*unknown{unknownOf(m.dsv), unknownOf(m.depth), unknownOf(m.rtv), unknownOf(m.color), unknownOf(m.resolved)} {
		if u != nil {
			u.Release()
		}
	}
	*m = multiTarget{}
}

// ensureMulti makes the scratch at least width by height.
func (d *Device) ensureMulti(width, height int) error {
	m := &d.multi
	if m.color != nil && m.width >= width && m.height >= height {
		return nil
	}
	width, height = max(width, m.width), max(height, m.height)
	m.release()
	desc := D3D11_TEXTURE2D_DESC{
		Width:      uint32(width),
		Height:     uint32(height),
		MipLevels:  1,
		ArraySize:  1,
		Format:     DXGI_FORMAT_R8G8B8A8_UNORM,
		SampleDesc: DXGI_SAMPLE_DESC{Count: d.samples},
		Usage:      D3D11_USAGE_DEFAULT,
		BindFlags:  D3D11_BIND_RENDER_TARGET,
	}
	var err error
	if m.color, err = d.device.createTexture2D(&desc); err != nil {
		m.release()
		return err
	}
	if m.rtv, err = d.device.createRenderTargetView(m.color); err != nil {
		m.release()
		return err
	}
	desc.Format, desc.BindFlags = DXGI_FORMAT_D24_UNORM_S8_UINT, D3D11_BIND_DEPTH_STENCIL
	if m.depth, err = d.device.createTexture2D(&desc); err != nil {
		m.release()
		return err
	}
	if m.dsv, err = d.device.createDepthStencilView(m.depth); err != nil {
		m.release()
		return err
	}
	if d.samples > 1 {
		desc.Format, desc.BindFlags, desc.SampleDesc = DXGI_FORMAT_R8G8B8A8_UNORM, 0, DXGI_SAMPLE_DESC{Count: 1}
		if m.resolved, err = d.device.createTexture2D(&desc); err != nil {
			m.release()
			return err
		}
	}
	m.width, m.height = width, height
	return nil
}

// drawMultisampled draws batch on the multisampled scratch, from transparent,
// and resolves the bounds of its clears into tx.
func (d *Device) drawMultisampled(tx *Texture, batch *gpu.Batch, scale float32) error {
	r := batch.ClearBounds().Intersect(image.Rect(0, 0, tx.width, tx.height))
	if r.Empty() {
		panic("gowaku: DrawToTarget called with a multisampled batch without clears")
	}
	if err := d.ensureMulti(tx.width, tx.height); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	d.startTiming()
	m := &d.multi
	sf := &d.targetSurface
	sf.viewport = D3D11_VIEWPORT{Width: float32(tx.width), Height: float32(tx.height), MaxDepth: 1}
	sf.scale = scale
	sf.rtv, sf.dsv = m.rtv, m.dsv
	sf.resolving, sf.color, sf.drawn = true, m.color, nil
	if err := d.writeConstants(sf); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	// The textures can still be bound from the last frame or target.
	var none *ID3D11ShaderResourceView
	d.context.psSetShaderResource(0, &none)
	d.context.psSetShaderResource(1, &none)
	d.context.clearRenderTargetView(m.rtv, &d.transparent)
	d.context.clearDepthStencilView(m.dsv)
	d.bindFrame(sf, m.dsv)
	err := d.drawBatch(sf, batch)
	var unbound *ID3D11RenderTargetView
	d.context.omSetRenderTarget(&unbound, nil)
	src := m.color
	if m.resolved != nil {
		d.context.resolveSubresource(m.resolved, m.color, DXGI_FORMAT_R8G8B8A8_UNORM)
		src = m.resolved
	}
	d.box = D3D11_BOX{left: uint32(r.Min.X), top: uint32(r.Min.Y), right: uint32(r.Max.X), bottom: uint32(r.Max.Y), back: 1}
	d.context.copySubresourceRegion(tx.texture, d.box.left, d.box.top, src, &d.box)
	sf.resolving, sf.color, sf.dsv = false, nil, nil
	return err
}
