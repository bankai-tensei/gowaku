package d3d11

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/gpu"
)

// blurPlan is the blur of one draw call of the batch in progress.
type blurPlan struct {
	blur gpu.Blur
	ok   bool
}

// planBlurs works out the blurs of batch and writes their passes into passes,
// three per draw call that blurs, in order. See docs/ADR/0044-drawing-extensions.md.
func (d *Device) planBlurs(sf *Surface, batch *gpu.Batch, passes []gpu.Instance) {
	d.blurs = d.blurs[:0]
	for i := range batch.Draws {
		call := &batch.Draws[i]
		if call.Blur <= 0 || call.Count == 0 {
			continue
		}
		in := &batch.Instances[call.First]
		width, height := int(sf.viewport.Width), int(sf.viewport.Height)
		fromImage := in.Kind.Base() == gpu.InstanceFiltered
		if fromImage {
			t, ok := call.Image.(*Texture)
			if !ok {
				d.blurs = append(d.blurs, blurPlan{})
				continue
			}
			width, height = t.width, t.height
		}
		b, ok := gpu.PlanBlur(call, in, sf.scale, width, height)
		if ok {
			k := len(d.blurs) * 3
			b.Passes((*[3]gpu.Instance)(passes[k:k+3]), sf.scale, fromImage)
		}
		d.blurs = append(d.blurs, blurPlan{blur: b, ok: ok})
	}
}

// drawBlur runs the passes of a blur, whose instances start at first, and binds
// the blurred picture for the instance of call. It reports whether call draws
// anything.
//
// The passes draw to scratch targets with the viewport of sf, so that the frame
// constants hold: a physical pixel of sf is a texel of the scratch target. They
// have no depth buffer, which lets the depth test pass.
func (d *Device) drawBlur(sf *Surface, call *gpu.DrawCall, plan *blurPlan, first int) (bool, error) {
	if !plan.ok {
		return false, nil
	}
	b := &plan.blur
	if call.Image == nil {
		if err := d.copyBackdrop(sf, b.Source); err != nil {
			return false, err
		}
	} else {
		d.bindTexture(call)
	}
	replace, err := d.blendState(gpu.BlendSrc)
	if err != nil {
		return false, err
	}
	if err := d.scratchTarget(&d.blurA, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return false, err
	}
	if err := d.scratchTarget(&d.blurB, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return false, err
	}
	d.scissor = D3D11_RECT{left: int32(b.Out.Min.X), top: int32(b.Out.Min.Y), right: int32(b.Out.Max.X), bottom: int32(b.Out.Max.Y)}
	d.context.rsSetScissorRect(&d.scissor)
	p := &d.pipeline
	for i, pass := range [3]struct{ to, from *Texture }{{d.blurA, nil}, {d.blurB, d.blurA}, {d.blurA, d.blurB}} {
		// A texture cannot be read and written at once.
		d.bindFiltered(nil)
		d.context.omSetRenderTarget(&pass.to.target, nil)
		d.bindFiltered(pass.from)
		p.use(d, sf, stageQuads, replace, p.quad, 0)
		d.context.drawInstanced(quadVertices, 1, 0, uint32(first+i))
	}
	d.context.omSetRenderTarget(&sf.rtv, sf.dsv)
	d.bindFiltered(d.blurA)
	return true, nil
}

// bindFiltered binds t as what the blur passes and blurred instances read, unless
// it is already bound. A nil t unbinds it.
func (d *Device) bindFiltered(t *Texture) {
	if t == d.boundFiltered {
		return
	}
	d.boundFiltered = t
	if t == nil {
		var none *ID3D11ShaderResourceView
		d.context.psSetShaderResource(4, &none)
		return
	}
	d.context.psSetShaderResource(4, &t.view)
}

// scratchTarget makes *slot a target of at least width by height, replacing a
// smaller one. Blurs share the two of the device, which only grow.
func (d *Device) scratchTarget(slot **Texture, width, height int) error {
	if t := *slot; t != nil && t.width >= width && t.height >= height {
		return nil
	}
	if t := *slot; t != nil {
		width, height = max(width, t.width), max(height, t.height)
		if d.boundFiltered == t {
			d.bindFiltered(nil)
		}
		t.Destroy()
		*slot = nil
	}
	t, err := d.CreateTarget(width, height)
	if err != nil {
		return fmt.Errorf("d3d11: blur: %w", err)
	}
	*slot = t.(*Texture)
	return nil
}
