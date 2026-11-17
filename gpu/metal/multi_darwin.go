//go:build darwin && arm64

package metal

import (
	"image"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/gpu"
)

// multiTarget is the scratch on which DrawToTarget draws a batch that needs
// multisampling: color and depth with the samples of the device, a texture of
// one sample of the same size that the color resolves into, which a blit then
// copies into the target, and the descriptor of the pass. It grows to the
// largest target. See docs/ADR/0114-gpu-layers-masks-filters.md.
type multiTarget struct {
	desc                  uintptr
	color, depth, stencil uintptr
	msaa, depthMSAA       uintptr
	resolved              uintptr
	width, height         int
}

// release lets go of the textures and the descriptor. Command buffers in
// flight keep what they use.
func (m *multiTarget) release(d *Device) {
	for _, t := range []uintptr{m.msaa, m.depthMSAA, m.resolved, m.desc} {
		if t != 0 {
			d.send(t, d.sel.release)
		}
	}
	*m = multiTarget{}
}

// ensureMulti makes the scratch at least width by height.
func (d *Device) ensureMulti(width, height int) error {
	m := &d.multi
	if m.msaa != 0 && m.width >= width && m.height >= height {
		return nil
	}
	width, height = max(width, m.width), max(height, m.height)
	for _, t := range []uintptr{m.msaa, m.depthMSAA, m.resolved} {
		if t != 0 {
			d.send(t, d.sel.release)
		}
	}
	m.msaa, m.depthMSAA, m.resolved = 0, 0, 0
	if m.desc == 0 {
		m.desc = d.newPassDescriptor(true)
		m.color, m.depth, m.stencil = attachments(m.desc, true)
		// The scratch starts transparent, where a frame starts with its
		// background.
		objc.ID(m.color).Send(objc.RegisterName("setClearColor:"), MTLClearColor{0, 0, 0, 0})
	}
	scratch := Surface{device: d, width: width, height: height}
	var err error
	if m.msaa, err = scratch.renderTexture(MTLPixelFormatBGRA8Unorm, d.samples); err != nil {
		return err
	}
	if m.depthMSAA, err = scratch.renderTexture(MTLPixelFormatDepth32Float_Stencil8, d.samples); err != nil {
		return err
	}
	if d.samples > 1 {
		if m.resolved, err = scratch.renderTexture(MTLPixelFormatBGRA8Unorm, 1); err != nil {
			return err
		}
	}
	m.width, m.height = width, height
	return nil
}

// drawMultisampled draws batch on the multisampled scratch, from transparent,
// and copies the bounds of its clears, resolved, into tx.
func (d *Device) drawMultisampled(tx *Texture, batch *gpu.Batch, scale float32) error {
	r := batch.ClearBounds().Intersect(image.Rect(0, 0, tx.width, tx.height))
	if r.Empty() {
		panic("gowaku: DrawToTarget called with a multisampled batch without clears")
	}
	if err := d.prepare(); err != nil {
		return err
	}
	if err := d.ensureMulti(tx.width, tx.height); err != nil {
		return err
	}
	m := &d.multi
	sf := &d.target
	d.textures = [5]uintptr{}
	*sf = Surface{device: d, width: tx.width, height: tx.height, scale: scale}
	d.constants = frameConstants{viewportWidth: float32(tx.width), viewportHeight: float32(tx.height), scale: scale}
	d.send(m.color, d.sel.setTexture, m.msaa)
	d.send(m.color, d.sel.setResolveTexture, m.resolved)
	d.send(m.color, d.sel.setLoadAction, MTLLoadActionClear)
	// endRender chooses the store actions, which Metal only lets an encoder
	// do for an attachment that left its own unknown. A color that kept the
	// default would store the samples and never resolve them.
	d.send(m.color, d.sel.setStoreAction, MTLStoreActionUnknown)
	for _, a := range []uintptr{m.depth, m.stencil} {
		d.send(a, d.sel.setTexture, m.depthMSAA)
		d.send(a, d.sel.setLoadAction, MTLLoadActionClear)
		d.send(a, d.sel.setStoreAction, MTLStoreActionUnknown)
	}
	variant := variantMulti
	if d.samples == 1 {
		variant = variantPlain
	}
	d.beginRender(m.desc, &d.pipelines[variant], tx.width, tx.height)
	err := d.drawBatch(sf, batch)
	d.endRender(true)
	src := m.resolved
	if src == 0 {
		src = m.msaa
	}
	blit := d.blitEncoder()
	d.origin = MTLOrigin{uint64(r.Min.X), uint64(r.Min.Y), 0}
	d.toOrigin = d.origin
	d.size = MTLSize{uint64(r.Dx()), uint64(r.Dy()), 1}
	d.send(blit, d.sel.copyTexture, src, 0, 0, ptr(&d.origin), ptr(&d.size), uintptr(tx.texture), 0, 0, ptr(&d.toOrigin))
	*sf = Surface{}
	return err
}
