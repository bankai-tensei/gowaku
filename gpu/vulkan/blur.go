//go:build linux || windows

package vulkan

import "github.com/bankai-tensei/gowaku/gpu"

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
		c := &batch.Draws[i]
		if c.Blur <= 0 || c.Count == 0 {
			continue
		}
		in := &batch.Instances[c.First]
		width, height := int(sf.extent.width), int(sf.extent.height)
		fromImage := in.Kind.Base() == gpu.InstanceFiltered
		if fromImage {
			t, ok := c.Image.(*Texture)
			if !ok {
				d.blurs = append(d.blurs, blurPlan{})
				continue
			}
			width, height = int(t.width), int(t.height)
		} else if sf.drawn == nil && !sf.copyable {
			// See prepareBlend.
			d.blurs = append(d.blurs, blurPlan{})
			continue
		}
		b, ok := gpu.PlanBlur(c, in, sf.scale, width, height)
		if ok {
			k := len(d.blurs) * 3
			b.Passes((*[3]gpu.Instance)(passes[k:k+3]), sf.scale, fromImage)
		}
		d.blurs = append(d.blurs, blurPlan{blur: b, ok: ok})
	}
}

// drawBlur runs the passes of a blur, whose instances start at first, and binds
// the blurred picture for the instance of c. It reports whether c draws
// anything.
//
// The passes cannot happen inside the render pass of sf, which ends, as for a
// blend mode that reads the target, and resumes after them. Each pass is a render
// pass of its own on a scratch target, with the pipelines of the plain pass: the
// pass of targets is compatible with it. The viewport stays that of sf, so that a
// physical pixel of sf is a texel of the scratch target.
func (d *Device) drawBlur(f *frame, sf *Surface, c *gpu.DrawCall, plan *blurPlan, first int) (bool, error) {
	if !plan.ok {
		return false, nil
	}
	b := &plan.blur
	commands := uintptr(f.commands)
	d.hot(d.vkCmdEndRenderPass, commands)
	sf.rendering = false
	saved := d.passInfo
	err := d.blurPasses(f, sf, c, b, first)
	d.passInfo = saved
	d.resume(f, sf)
	if err != nil {
		return false, err
	}
	d.bindFiltered(f, d.blurA)
	return true, nil
}

// blurPasses records the three passes of b outside a render pass.
func (d *Device) blurPasses(f *frame, sf *Surface, c *gpu.DrawCall, b *gpu.Blur, first int) error {
	if c.Image == nil {
		if err := d.copyBackdrop(f, sf, b.Source); err != nil {
			return err
		}
	} else {
		d.bindTextures(f, c)
	}
	if err := d.scratchTarget(&d.blurA, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return err
	}
	if err := d.scratchTarget(&d.blurB, b.Out.Max.X, b.Out.Max.Y); err != nil {
		return err
	}
	pass := d.pass
	d.pass = &d.passes[passPlain]
	err := d.recordBlurPasses(f, b, first)
	d.pass = pass
	return err
}

// recordBlurPasses records the passes into the scratch targets.
func (d *Device) recordBlurPasses(f *frame, b *gpu.Blur, first int) error {
	replace := gpu.DrawCall{Blend: gpu.BlendSrc}
	if err := d.blendPipeline(&replace); err != nil {
		return err
	}
	area := VkRect2D{
		VkOffset2D{int32(b.Out.Min.X), int32(b.Out.Min.Y)},
		VkExtent2D{uint32(b.Out.Dx()), uint32(b.Out.Dy())},
	}
	commands := uintptr(f.commands)
	for i, step := range [3]struct{ to, from *Texture }{{d.blurA, nil}, {d.blurB, d.blurA}, {d.blurA, d.blurB}} {
		d.imageBarrier(f, step.to.image, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
			VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_COLOR_ATTACHMENT_READ_BIT|VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
			VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL)
		d.passInfo = VkRenderPassBeginInfo{
			sType:           VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
			renderPass:      d.targetPass,
			framebuffer:     step.to.framebuffer,
			renderArea:      area,
			clearValueCount: uint32(len(d.clears)),
			pClearValues:    &d.clears[0],
		}
		d.hot(d.vkCmdBeginRenderPass, commands, ptr(&d.passInfo), VK_SUBPASS_CONTENTS_INLINE)
		d.bound, d.boundLevel = 0, -1
		d.scissor = area
		d.applyScissor(commands)
		if step.from != nil {
			d.bindFiltered(f, step.from)
		}
		d.use(f, d.pipeline(pipelineQuad, gpu.BlendSrc), 0)
		d.hot(d.vkCmdDraw, commands, quadVertices, 1, 0, uintptr(first+i))
		d.hot(d.vkCmdEndRenderPass, commands)
		d.imageBarrier(f, step.to.image, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
			VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
			VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL)
	}
	return nil
}

// bindFiltered binds t as what the blur passes and blurred instances read,
// unless it is already bound.
func (d *Device) bindFiltered(f *frame, t *Texture) {
	if t.set == d.textureSet[4] {
		return
	}
	d.textureSet[4] = t.set
	d.hot(d.vkCmdBindDescriptorSets, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 5, 1, ptr(&d.textureSet[4]), 0, 0)
}

// scratchTarget makes *slot a target of at least width by height, replacing a
// smaller one. Blurs share the two of the device, which only grow. It records
// outside a render pass.
func (d *Device) scratchTarget(slot **Texture, width, height int) error {
	if t := *slot; t != nil && int(t.width) >= width && int(t.height) >= height {
		return nil
	}
	if t := *slot; t != nil {
		width, height = max(width, int(t.width)), max(height, int(t.height))
		if d.textureSet[4] == t.set {
			d.textureSet[4] = 0
		}
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
