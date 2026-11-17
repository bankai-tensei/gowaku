//go:build linux || windows

package vulkan

import (
	"errors"
	"fmt"
	"image"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
)

// targetDepthSize is the size of the depth buffer that targets share. Nothing on
// a target uses it, but the pipelines come from a render pass that has one.
const targetDepthSize = 2048

// CreateTarget implements gpu.Backend. The target has the format of the surfaces,
// so that the pipelines of their render pass fit. It needs a surface to exist.
func (d *Device) CreateTarget(width, height int) (gpu.Texture, error) {
	if d.drawing != nil {
		panic("gowaku: CreateTarget called between BeginFrame and EndFrame")
	}
	if d.passes[passPlain].renderPass == 0 {
		return nil, errors.New("vulkan: CreateTarget called before the first surface")
	}
	if width > targetDepthSize || height > targetDepthSize {
		return nil, fmt.Errorf("vulkan: target of %dx%d exceeds %d", width, height, targetDepthSize)
	}
	if err := d.prepare(); err != nil {
		return nil, err
	}
	t, err := d.newTarget(width, height)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// newTarget makes a target. It records outside a render pass.
func (d *Device) newTarget(width, height int) (*Texture, error) {
	if err := d.createTargetPass(); err != nil {
		return nil, err
	}
	t := &Texture{device: d, format: gpu.TextureRGBA8, width: uint32(width), height: uint32(height)}
	// Blend modes that read the target copy from it.
	usage := uint32(VK_IMAGE_USAGE_SAMPLED_BIT | VK_IMAGE_USAGE_TRANSFER_DST_BIT | VK_IMAGE_USAGE_TRANSFER_SRC_BIT |
		VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT)
	if err := t.init(t.width, t.height, d.colorFormat, usage); err != nil {
		t.free()
		return nil, err
	}
	attachments := []VkImageView{t.view, d.targetDepth.view}
	info := VkFramebufferCreateInfo{
		sType:           VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
		renderPass:      d.targetPass,
		attachmentCount: uint32(len(attachments)),
		pAttachments:    &attachments[0],
		width:           t.width,
		height:          t.height,
		layers:          1,
	}
	r := callResult(d.vkCreateFramebuffer, uintptr(d.device), ptr(&info), 0, ptr(&t.framebuffer))
	if err := failure(r, "create framebuffer"); err != nil {
		t.free()
		return nil, err
	}
	return t, nil
}

// createTargetPass makes the render pass of targets, once. It is compatible with
// the plain pass of frames, whose pipelines it uses. What differs is that color
// stays. DrawToTarget moves the target to and from sampling with barriers, because
// another dependency would break the compatibility.
func (d *Device) createTargetPass() error {
	if d.targetPass != 0 {
		return nil
	}
	shared := Surface{device: d, extent: VkExtent2D{targetDepthSize, targetDepthSize}}
	err := shared.createTarget(&d.targetDepth, d.depthFormat, VK_SAMPLE_COUNT_1_BIT,
		VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT, depthAspect)
	if err != nil {
		return err
	}
	attachments := []VkAttachmentDescription{
		{
			format:         d.colorFormat,
			samples:        VK_SAMPLE_COUNT_1_BIT,
			loadOp:         VK_ATTACHMENT_LOAD_OP_LOAD,
			storeOp:        VK_ATTACHMENT_STORE_OP_STORE,
			stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_DONT_CARE,
			stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
			initialLayout:  VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
			finalLayout:    VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
		},
		{
			format:         d.depthFormat,
			samples:        VK_SAMPLE_COUNT_1_BIT,
			loadOp:         VK_ATTACHMENT_LOAD_OP_CLEAR,
			storeOp:        VK_ATTACHMENT_STORE_OP_DONT_CARE,
			stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_CLEAR,
			stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
			finalLayout:    VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		},
	}
	colorRef := VkAttachmentReference{0, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL}
	depthRef := VkAttachmentReference{1, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL}
	subpass := VkSubpassDescription{
		pipelineBindPoint:       VK_PIPELINE_BIND_POINT_GRAPHICS,
		colorAttachmentCount:    1,
		pColorAttachments:       &colorRef,
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
	return failure(callResult(d.vkCreateRenderPass, uintptr(d.device), ptr(&info), 0, ptr(&d.targetPass)), "create render pass")
}

// DrawToTarget implements gpu.Backend. The commands land in the command buffer of
// the next frame, ahead of its render pass, like uploads do.
func (d *Device) DrawToTarget(t gpu.Texture, batch *gpu.Batch, scale float32) error {
	if d.drawing != nil {
		panic("gowaku: DrawToTarget called between BeginFrame and EndFrame")
	}
	tx := t.(*Texture)
	if tx.framebuffer == 0 {
		panic("gowaku: DrawToTarget called on a texture that CreateTarget did not make")
	}
	if batch.Multisampled() && d.samples > 1 {
		return d.drawMultisampled(tx, batch, scale)
	}
	// Without multisampling a batch that needs it draws here too, on the depth
	// buffer that targets share.
	if err := d.prepare(); err != nil {
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
	sf.limit = image.Rectangle{}

	d.clears[0], d.clears[1] = VkClearValue{}, VkClearValue{}
	d.passInfo = VkRenderPassBeginInfo{
		sType:           VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		renderPass:      d.targetPass,
		framebuffer:     tx.framebuffer,
		renderArea:      VkRect2D{extent: sf.extent},
		clearValueCount: uint32(len(d.clears)),
		pClearValues:    &d.clears[0],
	}
	commands := uintptr(f.commands)
	d.imageBarrier(f, tx.image, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
		VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_COLOR_ATTACHMENT_READ_BIT|VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
		VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL)
	d.hot(d.vkCmdBeginRenderPass, commands, ptr(&d.passInfo), VK_SUBPASS_CONTENTS_INLINE)
	d.clearRects(f, batch)

	d.pass = &d.passes[passPlain]
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
	return err
}

// clearRects makes batch.Clears transparent.
func (d *Device) clearRects(f *frame, batch *gpu.Batch) {
	n := len(batch.Clears)
	if n == 0 {
		return
	}
	d.rects = d.rects[:0]
	for _, r := range batch.Clears {
		d.rects = append(d.rects, VkClearRect{
			rect:       VkRect2D{VkOffset2D{int32(r.Min.X), int32(r.Min.Y)}, VkExtent2D{uint32(r.Dx()), uint32(r.Dy())}},
			layerCount: 1,
		})
	}
	d.clearAttachment = VkClearAttachment{aspectMask: VK_IMAGE_ASPECT_COLOR_BIT}
	d.hot(d.vkCmdClearAttachments, uintptr(f.commands), 1, ptr(&d.clearAttachment), uintptr(n), uintptr(unsafe.Pointer(&d.rects[0])))
}
