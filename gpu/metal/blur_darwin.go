//go:build darwin && arm64

package metal

import "github.com/bankai-tensei/gowaku/gpu"

// blurPlan is the blur of one draw call of the batch in progress.
type blurPlan struct {
	blur gpu.Blur
	ok   bool
}

// planBlurs plans the blurs of batch, in order, and writes the instances of
// their passes to passes, three per blur.
func (d *Device) planBlurs(sf *Surface, batch *gpu.Batch, passes []gpu.Instance) {
	d.blurs = d.blurs[:0]
	for i := range batch.Draws {
		c := &batch.Draws[i]
		if c.Blur <= 0 || c.Count == 0 {
			continue
		}
		in := &batch.Instances[c.First]
		width, height := sf.width, sf.height
		fromImage := in.Kind.Base() == gpu.InstanceFiltered
		if fromImage {
			t, ok := c.Image.(*Texture)
			if !ok {
				d.blurs = append(d.blurs, blurPlan{})
				continue
			}
			width, height = t.width, t.height
		}
		b, ok := gpu.PlanBlur(c, in, sf.scale, width, height)
		if ok {
			k := len(d.blurs) * 3
			b.Passes((*[3]gpu.Instance)(passes[k:k+3]), sf.scale, fromImage)
		}
		d.blurs = append(d.blurs, blurPlan{blur: b, ok: ok})
	}
}

// drawBlur runs the passes of the blur of c, whose instances start at first,
// and binds the result as the blurred picture. It reports false if the call
// draws nothing.
func (d *Device) drawBlur(sf *Surface, c *gpu.DrawCall, plan *blurPlan, first int) (bool, error) {
	if !plan.ok {
		return false, nil
	}
	d.endRender(false)
	err := d.blurPasses(sf, c, &plan.blur, first)
	d.resume(sf)
	if err != nil {
		return false, err
	}
	d.bindTexture(slotFiltered, uintptr(d.blurA.texture))
	return true, nil
}

// blurPasses shrinks the source into blurA, blurs it across into blurB and down
// back into blurA. The viewport stays that of sf, so that a physical pixel of
// sf is a texel of the scratch targets.
func (d *Device) blurPasses(sf *Surface, c *gpu.DrawCall, b *gpu.Blur, first int) error {
	if c.Image == nil {
		if err := d.copyBackdrop(sf, b.Source); err != nil {
			return err
		}
	} else if t, ok := c.Image.(*Texture); ok {
		d.textures[slotPicture] = uintptr(t.texture)
	}
	if err := d.scratchTarget(&d.blurA, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return err
	}
	if err := d.scratchTarget(&d.blurB, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return err
	}
	flat := &d.pipelines[variantFlat]
	desc := d.targetPass()
	for i, step := range [3]struct{ to, from *Texture }{{d.blurA, nil}, {d.blurB, d.blurA}, {d.blurA, d.blurB}} {
		if step.from != nil {
			d.textures[slotFiltered] = uintptr(step.from.texture)
		}
		d.send(d.targetColor, d.sel.setTexture, uintptr(step.to.texture))
		d.send(d.targetColor, d.sel.setLoadAction, MTLLoadActionLoad)
		d.send(d.targetColor, d.sel.setStoreAction, MTLStoreActionUnknown)
		d.beginRender(desc, flat, sf.width, sf.height)
		d.bindBuffers(&d.buffers)
		d.scissor = MTLScissorRect{uint64(b.Out.Min.X), uint64(b.Out.Min.Y), uint64(b.Out.Dx()), uint64(b.Out.Dy())}
		d.lastScissor = d.scissor
		d.send(d.render, d.sel.setScissorRect, ptr(&d.scissor))
		replace, err := d.quadPipeline(gpu.BlendSrc)
		if err != nil {
			d.endRender(true)
			return err
		}
		d.use(replace, d.noDepth, 0)
		d.quads(&gpu.DrawCall{First: first + i, Count: 1})
		d.endRender(true)
	}
	d.send(d.targetColor, d.sel.setTexture, 0)
	return nil
}

// scratchTarget makes the texture in slot, or grows it, to at least the given
// size.
func (d *Device) scratchTarget(slot **Texture, width, height int) error {
	if t := *slot; t != nil && t.width >= width && t.height >= height {
		return nil
	}
	if t := *slot; t != nil {
		width, height = max(width, t.width), max(height, t.height)
		t.Destroy()
		*slot = nil
	}
	t, err := d.newTarget(width, height)
	if err != nil {
		return err
	}
	*slot = t
	return nil
}
