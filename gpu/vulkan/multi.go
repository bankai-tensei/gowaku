//go:build linux || windows

package vulkan

import (
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
)

// multiTarget is the scratch on which DrawToTarget draws a batch that needs
// multisampling: color and depth with the samples of the multisampled pass of
// frames, as large as the largest target so far, and a render pass compatible
// with that one, whose pipelines it uses, that resolves the color into the
// target at its end. gen counts the times the scratch was made, which the
// framebuffers of targets refer to. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
type multiTarget struct {
	pass         VkRenderPass
	color, depth target
	size         VkExtent2D
	gen          uint32
}

// createMultiPass makes the render pass of multisampled targets, once: the
// attachments of the multisampled pass of frames, the target as the resolve,
// and the same dependency, which keeps the two compatible.
func (d *Device) createMultiPass() error {
	if d.multi.pass != 0 {
		return nil
	}
	attachments := []VkAttachmentDescription{
		{
			format:         d.colorFormat,
			samples:        d.samples,
			loadOp:         VK_ATTACHMENT_LOAD_OP_CLEAR,
			storeOp:        VK_ATTACHMENT_STORE_OP_DONT_CARE,
			stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_DONT_CARE,
			stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
			finalLayout:    VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
		},
		{
			format:         d.depthFormat,
			samples:        d.samples,
			loadOp:         VK_ATTACHMENT_LOAD_OP_CLEAR,
			storeOp:        VK_ATTACHMENT_STORE_OP_DONT_CARE,
			stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_CLEAR,
			stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
			finalLayout:    VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		},
		{
			format:         d.colorFormat,
			samples:        VK_SAMPLE_COUNT_1_BIT,
			loadOp:         VK_ATTACHMENT_LOAD_OP_DONT_CARE,
			storeOp:        VK_ATTACHMENT_STORE_OP_STORE,
			stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_DONT_CARE,
			stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
			initialLayout:  VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
			finalLayout:    VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
		},
	}
	colorRef := VkAttachmentReference{0, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL}
	depthRef := VkAttachmentReference{1, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL}
	resolveRef := VkAttachmentReference{2, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL}
	subpass := VkSubpassDescription{
		pipelineBindPoint:       VK_PIPELINE_BIND_POINT_GRAPHICS,
		colorAttachmentCount:    1,
		pColorAttachments:       &colorRef,
		pResolveAttachments:     &resolveRef,
		pDepthStencilAttachment: &depthRef,
	}
	dependency := frameDependency()
	info := VkRenderPassCreateInfo{
		sType:           VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO,
		attachmentCount: uint32(len(attachments)),
		pAttachments:    &attachments[0],
		subpassCount:    1,
		pSubpasses:      &subpass,
		dependencyCount: 1,
		pDependencies:   &dependency,
	}
	return failure(callResult(d.vkCreateRenderPass, uintptr(d.device), ptr(&info), 0, ptr(&d.multi.pass)), "create render pass")
}

// ensureMulti makes what drawing a multisampled batch to a target of width by
// height needs: the multisampled pass and its pipelines, the pass of targets,
// and the scratch, remade larger if it is too small.
func (d *Device) ensureMulti(width, height uint32) error {
	if d.passes[passMulti].renderPass == 0 {
		if err := d.createPass(&d.passes[passMulti], d.samples); err != nil {
			return err
		}
	}
	if err := d.createMultiPass(); err != nil {
		return err
	}
	m := &d.multi
	if m.color.view != 0 && m.size.width >= width && m.size.height >= height {
		return nil
	}
	if m.color.view != 0 {
		old := [2]target{m.color, m.depth}
		d.retire(func() {
			for i := range old {
				d.freeTarget(&old[i])
			}
		})
	}
	m.size = VkExtent2D{max(width, m.size.width), max(height, m.size.height)}
	m.color, m.depth = target{}, target{}
	m.gen++
	shared := Surface{device: d, extent: m.size}
	if err := shared.createTarget(&m.color, d.colorFormat, d.samples, VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT,
		VK_IMAGE_ASPECT_COLOR_BIT); err != nil {
		return err
	}
	return shared.createTarget(&m.depth, d.depthFormat, d.samples, VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT,
		depthAspect)
}

// freeTarget destroys t.
func (d *Device) freeTarget(t *target) {
	dev := uintptr(d.device)
	if t.view != 0 {
		call(d.vkDestroyImageView, dev, uintptr(t.view), 0)
	}
	if t.image != 0 {
		call(d.vkDestroyImage, dev, uintptr(t.image), 0)
	}
	if t.memory != 0 {
		call(d.vkFreeMemory, dev, uintptr(t.memory), 0)
	}
	*t = target{}
}

// multiFramebuffer returns the framebuffer of tx in the multisampled pass of
// targets, made again if the scratch has changed since.
func (d *Device) multiFramebuffer(tx *Texture) (VkFramebuffer, error) {
	if tx.multiFramebuffer != 0 && tx.multiGen == d.multi.gen {
		return tx.multiFramebuffer, nil
	}
	if fb := tx.multiFramebuffer; fb != 0 {
		d.retire(func() { call(d.vkDestroyFramebuffer, uintptr(d.device), uintptr(fb), 0) })
		tx.multiFramebuffer = 0
	}
	attachments := []VkImageView{d.multi.color.view, d.multi.depth.view, tx.view}
	info := VkFramebufferCreateInfo{
		sType:           VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
		renderPass:      d.multi.pass,
		attachmentCount: uint32(len(attachments)),
		pAttachments:    &attachments[0],
		width:           tx.width,
		height:          tx.height,
		layers:          1,
	}
	r := callResult(d.vkCreateFramebuffer, uintptr(d.device), ptr(&info), 0, ptr(&tx.multiFramebuffer))
	if err := failure(r, "create framebuffer"); err != nil {
		return 0, err
	}
	tx.multiGen = d.multi.gen
	return tx.multiFramebuffer, nil
}

// drawMultisampled draws batch on the multisampled scratch, from transparent,
// and resolves the bounds of its clears into tx. Drawing stays inside those
// bounds, which are the area of the render pass.
func (d *Device) drawMultisampled(tx *Texture, batch *gpu.Batch, scale float32) error {
	r := batch.ClearBounds().Intersect(image.Rect(0, 0, int(tx.width), int(tx.height)))
	if r.Empty() {
		panic("gowaku: DrawToTarget called with a multisampled batch without clears")
	}
	if err := d.prepare(); err != nil {
		return err
	}
	if err := d.ensureMulti(tx.width, tx.height); err != nil {
		return err
	}
	framebuffer, err := d.multiFramebuffer(tx)
	if err != nil {
		return err
	}
	f := &d.frames[d.current]
	set, err := d.targetSet(f, frameConstants{
		viewportWidth:  float32(tx.width),
		viewportHeight: float32(tx.height),
		scale:          scale,
	})
	if err != nil {
		return err
	}
	sf := &d.targetSurface
	sf.extent, sf.scale, sf.drawn = VkExtent2D{tx.width, tx.height}, scale, tx
	sf.limit = r
	area := VkRect2D{VkOffset2D{int32(r.Min.X), int32(r.Min.Y)}, VkExtent2D{uint32(r.Dx()), uint32(r.Dy())}}
	d.clears[0], d.clears[1] = VkClearValue{}, VkClearValue{}
	d.passInfo = VkRenderPassBeginInfo{
		sType:           VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		renderPass:      d.multi.pass,
		framebuffer:     framebuffer,
		renderArea:      area,
		clearValueCount: uint32(len(d.clears)),
		pClearValues:    &d.clears[0],
	}
	commands := uintptr(f.commands)
	d.imageBarrier(f, tx.image, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
		VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_COLOR_ATTACHMENT_READ_BIT|VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
		VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL)
	d.hot(d.vkCmdBeginRenderPass, commands, ptr(&d.passInfo), VK_SUBPASS_CONTENTS_INLINE)
	d.pass = &d.passes[passMulti]
	d.bound, d.boundLevel = 0, -1
	d.surfaceSize = VkViewport{width: float32(tx.width), height: float32(tx.height), maxDepth: 1}
	d.lastScissor = VkRect2D{}
	d.textureSet = [5]VkDescriptorSet{d.blank.set, d.blank.set, d.blank.set, d.blank.set, d.blank.set}
	d.hot(d.vkCmdBindDescriptorSets, commands, VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 0, 1, ptr(set), 0, 0)
	d.hot(d.vkCmdBindDescriptorSets, commands, VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 1, 5, ptr(&d.textureSet[0]), 0, 0)
	err = d.drawBatch(sf, batch)
	d.hot(d.vkCmdEndRenderPass, commands)
	d.imageBarrier(f, tx.image, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
		VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
		VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL)
	sf.limit = image.Rectangle{}
	return err
}
