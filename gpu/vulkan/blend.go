//go:build linux || windows

package vulkan

import (
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// prepareBlend makes ready what the blend mode of c needs: its pipeline, and for
// a mode that reads the target, the backdrop. It reports whether c draws anything
// at all.
//
// A render pass cannot copy its own attachment, so that the backdrop ends the
// pass, copies what c covers from the single-sampled image, which the end of a
// multisampled pass has just resolved, and resumes with a compatible pass that
// loads everything. See docs/ADR/0044-drawing-extensions.md.
func (d *Device) prepareBlend(f *frame, sf *Surface, c *gpu.DrawCall) (bool, error) {
	if err := d.blendPipeline(c); err != nil {
		return false, err
	}
	if !c.Blend.ReadsBackdrop() {
		return true, nil
	}
	width, height := int(sf.extent.width), int(sf.extent.height)
	r := gpu.BackdropRect(c, sf.scale, width, height)
	if r.Empty() {
		return false, nil
	}
	if sf.drawn == nil && !sf.copyable {
		// The window system gives images that cannot be copied from,
		// which no driver of a desktop GPU does. The draw goes away.
		return false, nil
	}
	d.hot(d.vkCmdEndRenderPass, uintptr(f.commands))
	sf.rendering = false
	if err := d.copyBackdrop(f, sf, r); err != nil {
		return false, err
	}
	d.resume(f, sf)
	return true, nil
}

// copyBackdrop copies r of what sf draws to into the backdrop, at the same place,
// and binds it. It records outside a render pass. sf must be copyable unless it
// stands in for a target.
func (d *Device) copyBackdrop(f *frame, sf *Surface, r image.Rectangle) error {
	width, height := int(sf.extent.width), int(sf.extent.height)
	var source VkImage
	layout := uint32(VK_IMAGE_LAYOUT_PRESENT_SRC_KHR)
	if sf.drawn != nil {
		source, layout = sf.drawn.image, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL
	} else {
		source = sf.images[sf.imageIndex]
	}
	commands := uintptr(f.commands)
	backdrop, err := d.backdropTexture(uint32(width), uint32(height))
	if err != nil {
		return err
	}
	d.imageBarrier(f, source, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
		VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
		layout, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL)
	d.imageBarrier(f, backdrop.image, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
		VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_TRANSFER_WRITE_BIT,
		VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL)
	layers := VkImageSubresourceLayers{aspectMask: VK_IMAGE_ASPECT_COLOR_BIT, layerCount: 1}
	at := VkOffset3D{int32(r.Min.X), int32(r.Min.Y), 0}
	d.imageCopy = VkImageCopy{
		srcSubresource: layers,
		srcOffset:      at,
		dstSubresource: layers,
		dstOffset:      at,
		extent:         VkExtent3D{uint32(r.Dx()), uint32(r.Dy()), 1},
	}
	d.hot(d.vkCmdCopyImage, commands, uintptr(source), VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
		uintptr(backdrop.image), VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, ptr(&d.imageCopy))
	d.imageBarrier(f, backdrop.image, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
		VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
		VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL)
	// A multisampled pass resolves into the image again, which waits here for
	// the copy to have read it.
	d.imageBarrier(f, source, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
		0, VK_ACCESS_COLOR_ATTACHMENT_READ_BIT|VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
		VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL)
	if backdrop.set != d.textureSet[3] {
		d.textureSet[3] = backdrop.set
		d.hot(d.vkCmdBindDescriptorSets, commands, VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 4, 1, ptr(&d.textureSet[3]), 0, 0)
	}
	return nil
}

// resume goes on with the render pass of sf after something that had to end it.
// The rest of passInfo is that of the pass that ended: the same framebuffer,
// which a compatible pass that loads everything can use.
func (d *Device) resume(f *frame, sf *Surface) {
	d.passInfo.renderPass = d.pass.resume
	if sf.drawn != nil {
		d.passInfo.renderPass = d.targetPass
	}
	d.hot(d.vkCmdBeginRenderPass, uintptr(f.commands), ptr(&d.passInfo), VK_SUBPASS_CONTENTS_INLINE)
	sf.rendering = true
	// What is bound lasts across render passes, but the viewport and scissor
	// are set again to be safe.
	d.bound, d.boundLevel, d.lastScissor = 0, -1, VkRect2D{}
}

// backdropTexture returns the backdrop, replaced first by a larger one if it is
// smaller than width by height. It records outside a render pass.
func (d *Device) backdropTexture(width, height uint32) (*Texture, error) {
	if t := d.backdrop; t != nil && t.width >= width && t.height >= height {
		return t, nil
	}
	if t := d.backdrop; t != nil {
		width, height = max(width, t.width), max(height, t.height)
		if d.textureSet[3] == t.set {
			d.textureSet[3] = 0
		}
		t.Destroy()
		d.backdrop = nil
	}
	t := &Texture{device: d, format: gpu.TextureRGBA8, width: width, height: height}
	if err := t.init(width, height, d.colorFormat, VK_IMAGE_USAGE_SAMPLED_BIT|VK_IMAGE_USAGE_TRANSFER_DST_BIT); err != nil {
		t.free()
		return nil, err
	}
	d.backdrop = t
	return t, nil
}
