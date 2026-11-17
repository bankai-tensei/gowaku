package d3d11

import (
	"errors"
	"fmt"
	"image"
	"runtime"

	"github.com/bankai-tensei/gowaku/gpu"
)

// Texture is a texture the pixel shader can read.
type Texture struct {
	device  *Device
	texture *ID3D11Texture2D
	view    *ID3D11ShaderResourceView
	format  gpu.TextureFormat
	// target exists on what CreateTarget made.
	target        *ID3D11RenderTargetView
	width, height int
}

var _ gpu.Texture = (*Texture)(nil)

// CreateTexture implements gpu.Backend.
func (d *Device) CreateTexture(width, height int, format gpu.TextureFormat) (gpu.Texture, error) {
	var dxgi uint32
	levels := 1
	switch format {
	case gpu.TextureR8:
		dxgi = DXGI_FORMAT_R8_UNORM
	case gpu.TextureRGBA8:
		dxgi = DXGI_FORMAT_R8G8B8A8_UNORM
	case gpu.TextureRGBA8Mipmapped:
		dxgi = DXGI_FORMAT_R8G8B8A8_UNORM
		levels = gpu.MipLevels(width, height)
	default:
		return nil, errors.New("d3d11: unknown texture format")
	}
	texture, err := d.device.createTexture2D(&D3D11_TEXTURE2D_DESC{
		Width:      uint32(width),
		Height:     uint32(height),
		MipLevels:  uint32(levels),
		ArraySize:  1,
		Format:     dxgi,
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
	return &Texture{device: d, texture: texture, view: view, format: format, width: width, height: height}, nil
}

// UpdateTexture implements gpu.Backend.
func (d *Device) UpdateTexture(t gpu.Texture, region image.Rectangle, pix []byte, stride int) error {
	if region.Empty() {
		return nil
	}
	tx := t.(*Texture)
	d.box = D3D11_BOX{
		left:   uint32(region.Min.X),
		top:    uint32(region.Min.Y),
		front:  0,
		right:  uint32(region.Max.X),
		bottom: uint32(region.Max.Y),
		back:   1,
	}
	if tx.format == gpu.TextureRGBA8Mipmapped && region != image.Rect(0, 0, tx.width, tx.height) {
		panic("gowaku: UpdateTexture of a mipmapped texture must cover all of it")
	}
	first := &pix[region.Min.Y*stride+region.Min.X*tx.format.BytesPerPixel()]
	d.context.updateSubresource(tx.texture, 0, &d.box, first, uint32(stride))
	// The call reads through a pointer that Go no longer sees as a reference.
	runtime.KeepAlive(pix)
	if tx.format == gpu.TextureRGBA8Mipmapped {
		mips := gpu.BuildMips(pix, stride, tx.width, tx.height)
		at := 0
		for level := 1; level < gpu.MipLevels(tx.width, tx.height); level++ {
			w, h := gpu.MipSize(tx.width, tx.height, level)
			d.box = D3D11_BOX{right: uint32(w), bottom: uint32(h), back: 1}
			d.context.updateSubresource(tx.texture, uint32(level), &d.box, &mips[at], uint32(w*4))
			at += w * h * 4
		}
		runtime.KeepAlive(mips)
	}
	return nil
}

// Destroy implements gpu.Texture.
func (t *Texture) Destroy() {
	if t.device.boundTexture == t {
		t.device.boundTexture = nil
	}
	if t.device.boundImage == t {
		t.device.boundImage = nil
	}
	if t.device.boundBackdrop == t {
		t.device.boundBackdrop = nil
	}
	if t.target != nil {
		unknownOf(t.target).Release()
	}
	unknownOf(t.view).Release()
	unknownOf(t.texture).Release()
	*t = Texture{}
}
