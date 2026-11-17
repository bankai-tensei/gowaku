//go:build darwin && arm64

package metal

import (
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// The texture slots of sdfFragment.
const (
	slotAtlas = iota
	slotPicture
	slotRamps
	slotBackdrop
	slotFiltered
)

// prepareBlend makes ready what a draw call with a blend mode other than
// source-over needs: for a mode that reads the target, a copy of what Bounds
// covers in the backdrop. It reports false if the call draws nothing.
func (d *Device) prepareBlend(sf *Surface, c *gpu.DrawCall) (bool, error) {
	if !c.Blend.ReadsBackdrop() {
		return true, nil
	}
	r := gpu.BackdropRect(c, sf.scale, sf.width, sf.height)
	if r.Empty() {
		return false, nil
	}
	d.endRender(false)
	err := d.copyBackdrop(sf, r)
	d.resume(sf)
	return err == nil, err
}

// copyBackdrop copies r of the target of sf to the same place of the backdrop.
// The render encoder must be closed, and the target resolved.
func (d *Device) copyBackdrop(sf *Surface, r image.Rectangle) error {
	backdrop, err := d.backdropTexture(sf.width, sf.height)
	if err != nil {
		return err
	}
	blit := d.blitEncoder()
	d.origin = MTLOrigin{uint64(r.Min.X), uint64(r.Min.Y), 0}
	d.toOrigin = d.origin
	d.size = MTLSize{uint64(r.Dx()), uint64(r.Dy()), 1}
	d.send(blit, d.sel.copyTexture, sf.target(), 0, 0, ptr(&d.origin), ptr(&d.size),
		uintptr(backdrop.texture), 0, 0, ptr(&d.toOrigin))
	d.textures[slotBackdrop] = uintptr(backdrop.texture)
	return nil
}

// resume opens the render encoder of sf again after an interruption, with
// everything bound as it was.
func (d *Device) resume(sf *Surface) {
	sf.begin(true)
	d.bindBuffers(&d.buffers)
}

// backdropTexture returns the backdrop, made or grown to at least the given
// size. Like a target it holds the format of the drawable.
func (d *Device) backdropTexture(width, height int) (*Texture, error) {
	if t := d.backdrop; t != nil && t.width >= width && t.height >= height {
		return t, nil
	}
	if t := d.backdrop; t != nil {
		width, height = max(width, t.width), max(height, t.height)
		t.Destroy()
		d.backdrop = nil
	}
	t, err := d.newTexture(width, height, gpu.TextureRGBA8, MTLPixelFormatBGRA8Unorm, MTLTextureUsageShaderRead, 1)
	if err != nil {
		return nil, err
	}
	d.backdrop = t
	return t, nil
}
