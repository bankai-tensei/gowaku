//go:build linux || windows

package vulkan

import (
	"errors"
	"fmt"
	"image"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// depthAspect is what a depth buffer holds.
const depthAspect = VK_IMAGE_ASPECT_DEPTH_BIT | VK_IMAGE_ASPECT_STENCIL_BIT

// target is an image a frame draws to that the swap chain does not provide.
type target struct {
	image  VkImage
	memory VkDeviceMemory
	view   VkImageView
}

// Surface is the swap chain of one window.
type Surface struct {
	device    *Device
	surface   VkSurfaceKHR
	swapchain VkSwapchainKHR
	format    VkSurfaceFormatKHR
	extent    VkExtent2D
	scale     float32
	// width and height are what the window last asked for.
	width, height int

	images []VkImage
	views  []VkImageView
	// framebuffers holds one list per render pass. That of the multisampled pass
	// is empty until a frame needs it, and so are color and multiDepth.
	framebuffers      [passCount][]VkFramebuffer
	depth             target
	color, multiDepth target
	// multisample is what the next frame asks for, and passIndex what this one
	// uses.
	multisample bool
	passIndex   int

	// acquired rotates: a semaphore is free again once the frame that waited on
	// it has finished, which framesInFlight bounds. rendered goes with an image,
	// because an image comes back only after its presentation has begun.
	acquired      []VkSemaphore
	acquiredIndex int
	rendered      []VkSemaphore
	imageIndex    uint32

	// stale means that the swap chain no longer fits the window.
	stale     bool
	inFrame   bool
	rendering bool
	// copyable means that the images can be copied from, which blend modes that
	// read the frame need.
	copyable bool
	// drawn is the target that the stand-in surface of DrawToTarget draws to,
	// and limit, unless empty, what a multisampled batch may draw to of it.
	drawn *Texture
	limit image.Rectangle
	// transparent means that the window wants its frames composited with
	// their alpha, which gowaku draws premultiplied.
	transparent bool
}

// wantsAlpha reports whether the window of h asked to be transparent.
func wantsAlpha(h platform.NativeHandle) bool {
	switch h := h.(type) {
	case platform.X11Handle:
		return h.Transparent
	case platform.WaylandHandle:
		return h.Transparent
	case platform.Win32Handle:
		return h.Transparent
	}
	return false
}

// compositeAlpha picks how the compositor treats the alpha of the frames out
// of what it supports: premultiplied for a transparent window, else opaque.
// Where the first choice is missing, the lowest bit left is as good as any.
func compositeAlpha(supported uint32, transparent bool) uint32 {
	want := uint32(VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR)
	if transparent {
		want = VK_COMPOSITE_ALPHA_PRE_MULTIPLIED_BIT_KHR
	}
	if supported&want != 0 {
		return want
	}
	if transparent && supported&VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR != 0 {
		// The window system decides, which with a visual of alpha on X11
		// is premultiplied.
		return VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR
	}
	return supported & -supported
}

var _ gpu.Surface = (*Surface)(nil)

// CreateSurface implements gpu.Backend.
func (d *Device) CreateSurface(h platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (gpu.Surface, error) {
	surface, r, err := d.createPlatformSurface(h)
	if err != nil {
		return nil, err
	}
	if err := failure(r, "create surface"); err != nil {
		return nil, err
	}
	s := &Surface{device: d, multisample: true, surface: surface, scale: scale, width: physicalWidth, height: physicalHeight,
		transparent: wantsAlpha(h)}
	if err := s.init(); err != nil {
		s.destroy()
		return nil, err
	}
	d.liveSurfaces++
	return s, nil
}

func (s *Surface) init() error {
	d := s.device
	var supported uint32
	r := callResult(d.vkGetPhysicalDeviceSurfaceSupportKHR, uintptr(d.physical), uintptr(d.family), uintptr(s.surface), ptr(&supported))
	if err := failure(r, "query surface support"); err != nil {
		return err
	}
	if supported == 0 {
		return fmt.Errorf("vulkan: %q cannot present to this window", d.name)
	}
	if err := s.pickFormat(); err != nil {
		return err
	}
	if d.passes[passPlain].renderPass == 0 {
		if err := d.createRenderPass(s.format.format); err != nil {
			return err
		}
	} else if d.colorFormat != s.format.format {
		return errors.New("vulkan: windows with different surface formats")
	}
	return s.rebuild()
}

func (s *Surface) pickFormat() error {
	d := s.device
	var count uint32
	r := callResult(d.vkGetPhysicalDeviceSurfaceFormatsKHR, uintptr(d.physical), uintptr(s.surface), ptr(&count), 0)
	if err := failure(r, "query surface formats"); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("vulkan: surface has no formats")
	}
	formats := make([]VkSurfaceFormatKHR, count)
	r = callResult(d.vkGetPhysicalDeviceSurfaceFormatsKHR, uintptr(d.physical), uintptr(s.surface), ptr(&count), ptr(&formats[0]))
	if err := failure(r, "query surface formats"); err != nil {
		return err
	}
	format, ok := supportedFormats(formats[:count])
	if !ok {
		return errors.New("vulkan: surface offers no 8-bit UNORM format")
	}
	s.format = format
	return nil
}

// presentMode picks how frames reach the screen. FIFO waits for vertical blank
// and every implementation has it.
func (s *Surface) presentMode() (uint32, error) {
	d := s.device
	if !d.immediate {
		return VK_PRESENT_MODE_FIFO_KHR, nil
	}
	var count uint32
	r := callResult(d.vkGetPhysicalDeviceSurfacePresentModesKHR, uintptr(d.physical), uintptr(s.surface), ptr(&count), 0)
	if err := failure(r, "query present modes"); err != nil || count == 0 {
		return VK_PRESENT_MODE_FIFO_KHR, err
	}
	modes := make([]uint32, count)
	r = callResult(d.vkGetPhysicalDeviceSurfacePresentModesKHR, uintptr(d.physical), uintptr(s.surface), ptr(&count), ptr(&modes[0]))
	if err := failure(r, "query present modes"); err != nil {
		return VK_PRESENT_MODE_FIFO_KHR, err
	}
	best := uint32(VK_PRESENT_MODE_FIFO_KHR)
	for _, m := range modes[:count] {
		if m == VK_PRESENT_MODE_IMMEDIATE_KHR {
			return m, nil
		}
		if m == VK_PRESENT_MODE_MAILBOX_KHR {
			best = m
		}
	}
	return best, nil
}

// Resize implements gpu.Surface.
func (s *Surface) Resize(physicalWidth, physicalHeight int, scale float32) error {
	if s.inFrame {
		panic("gowaku: Surface.Resize called between BeginFrame and EndFrame")
	}
	if physicalWidth <= 0 || physicalHeight <= 0 {
		return nil
	}
	s.scale = scale
	if physicalWidth == s.width && physicalHeight == s.height && !s.stale {
		return nil
	}
	s.width, s.height = physicalWidth, physicalHeight
	return s.rebuild()
}

// rebuild makes the swap chain and everything sized after it. The old swap chain
// goes only after the GPU is done with it.
func (s *Surface) rebuild() error {
	d := s.device
	if err := d.waitIdle(); err != nil {
		return err
	}
	var caps VkSurfaceCapabilitiesKHR
	r := callResult(d.vkGetPhysicalDeviceSurfaceCapabilitiesKHR, uintptr(d.physical), uintptr(s.surface), ptr(&caps))
	if err := failure(r, "query surface capabilities"); err != nil {
		return err
	}
	// Where the extent is the system's to decide, it says so here. The value
	// marks a surface that takes the size of its swap chain.
	const undefinedExtent = 0xFFFFFFFF
	extent := caps.currentExtent
	if extent.width == undefinedExtent {
		extent = VkExtent2D{
			min(max(uint32(s.width), caps.minImageExtent.width), caps.maxImageExtent.width),
			min(max(uint32(s.height), caps.minImageExtent.height), caps.maxImageExtent.height),
		}
	}
	if extent.width == 0 || extent.height == 0 {
		// A minimized window has no images. The next size brings them back.
		s.stale = true
		return nil
	}
	mode, err := s.presentMode()
	if err != nil {
		return err
	}
	count := max(caps.minImageCount, 2)
	if caps.maxImageCount != 0 {
		count = min(count, caps.maxImageCount)
	}
	alpha := compositeAlpha(caps.supportedCompositeAlpha, s.transparent)
	info := VkSwapchainCreateInfoKHR{
		sType:            VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR,
		surface:          s.surface,
		minImageCount:    count,
		imageFormat:      s.format.format,
		imageColorSpace:  s.format.colorSpace,
		imageExtent:      extent,
		imageArrayLayers: 1,
		imageUsage:       VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT | (caps.supportedUsageFlags & VK_IMAGE_USAGE_TRANSFER_SRC_BIT),
		imageSharingMode: VK_SHARING_MODE_EXCLUSIVE,
		preTransform:     caps.currentTransform,
		compositeAlpha:   alpha,
		presentMode:      mode,
		clipped:          1,
		oldSwapchain:     s.swapchain,
	}
	var next VkSwapchainKHR
	r = callResult(d.vkCreateSwapchainKHR, uintptr(d.device), ptr(&info), 0, ptr(&next))
	if err := failure(r, "create swap chain"); err != nil {
		return err
	}
	s.releaseTargets()
	if s.swapchain != 0 {
		call(d.vkDestroySwapchainKHR, uintptr(d.device), uintptr(s.swapchain), 0)
	}
	s.swapchain, s.extent, s.stale = next, extent, false
	s.copyable = caps.supportedUsageFlags&VK_IMAGE_USAGE_TRANSFER_SRC_BIT != 0
	return s.createTargets()
}

func (s *Surface) createTargets() error {
	d := s.device
	dev := uintptr(d.device)
	var count uint32
	r := callResult(d.vkGetSwapchainImagesKHR, dev, uintptr(s.swapchain), ptr(&count), 0)
	if err := failure(r, "get swap chain images"); err != nil {
		return err
	}
	s.images = make([]VkImage, count)
	r = callResult(d.vkGetSwapchainImagesKHR, dev, uintptr(s.swapchain), ptr(&count), ptr(&s.images[0]))
	if err := failure(r, "get swap chain images"); err != nil {
		return err
	}
	if err := s.createTarget(&s.depth, d.depthFormat, VK_SAMPLE_COUNT_1_BIT, VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT, depthAspect); err != nil {
		return err
	}
	s.views = make([]VkImageView, count)
	s.rendered = make([]VkSemaphore, count)
	// One more than the images: acquiring can run ahead of the frames in flight.
	s.acquired = make([]VkSemaphore, count+1)
	for i, img := range s.images {
		view, err := d.createView(img, s.format.format, VK_IMAGE_ASPECT_COLOR_BIT)
		if err != nil {
			return err
		}
		s.views[i] = view
	}
	if err := s.createFramebuffers(passPlain); err != nil {
		return err
	}
	info := VkSemaphoreCreateInfo{sType: VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO}
	for _, list := range [][]VkSemaphore{s.rendered, s.acquired} {
		for i := range list {
			if err := failure(callResult(d.vkCreateSemaphore, dev, ptr(&info), 0, ptr(&list[i])), "create semaphore"); err != nil {
				return err
			}
		}
	}
	return nil
}

// createFramebuffers makes the framebuffers of a render pass, one for each image.
func (s *Surface) createFramebuffers(pass int) error {
	d := s.device
	list := make([]VkFramebuffer, len(s.views))
	s.framebuffers[pass] = list
	for i, view := range s.views {
		attachments := []VkImageView{view, s.depth.view}
		if pass == passMulti {
			attachments = []VkImageView{s.color.view, s.multiDepth.view, view}
		}
		info := VkFramebufferCreateInfo{
			sType:           VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO,
			renderPass:      d.passes[pass].renderPass,
			attachmentCount: uint32(len(attachments)),
			pAttachments:    &attachments[0],
			width:           s.extent.width,
			height:          s.extent.height,
			layers:          1,
		}
		r := callResult(d.vkCreateFramebuffer, uintptr(d.device), ptr(&info), 0, ptr(&list[i]))
		if err := failure(r, "create framebuffer"); err != nil {
			return err
		}
	}
	return nil
}

// SetMultisample implements gpu.Surface.
func (s *Surface) SetMultisample(on bool) { s.multisample = on }

// selectPass picks the render pass of a new frame, and makes it if it is the
// first to need it.
func (s *Surface) selectPass() error {
	d := s.device
	s.passIndex = passPlain
	if s.multisample && d.samples > 1 {
		s.passIndex = passMulti
		if err := s.createMultisampled(); err != nil {
			return err
		}
	}
	d.pass = &d.passes[s.passIndex]
	return nil
}

func (s *Surface) createMultisampled() error {
	if s.framebuffers[passMulti] != nil {
		return nil
	}
	d := s.device
	if d.passes[passMulti].renderPass == 0 {
		if err := d.createPass(&d.passes[passMulti], d.samples); err != nil {
			return err
		}
	}
	if err := s.createTarget(&s.color, s.format.format, d.samples, VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT, VK_IMAGE_ASPECT_COLOR_BIT); err != nil {
		return err
	}
	if err := s.createTarget(&s.multiDepth, d.depthFormat, d.samples, VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT, depthAspect); err != nil {
		return err
	}
	return s.createFramebuffers(passMulti)
}

func (s *Surface) createTarget(t *target, format, samples, usage, aspect uint32) error {
	d := s.device
	dev := uintptr(d.device)
	info := VkImageCreateInfo{
		sType:       VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		imageType:   VK_IMAGE_TYPE_2D,
		format:      format,
		extent:      VkExtent3D{s.extent.width, s.extent.height, 1},
		mipLevels:   1,
		arrayLayers: 1,
		samples:     samples,
		tiling:      VK_IMAGE_TILING_OPTIMAL,
		usage:       usage,
		sharingMode: VK_SHARING_MODE_EXCLUSIVE,
	}
	if err := failure(callResult(d.vkCreateImage, dev, ptr(&info), 0, ptr(&t.image)), "create image"); err != nil {
		return err
	}
	var req VkMemoryRequirements
	call(d.vkGetImageMemoryReqs, dev, uintptr(t.image), ptr(&req))
	var err error
	if t.memory, err = d.allocate(&req, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT); err != nil {
		return err
	}
	if err := failure(callResult(d.vkBindImageMemory, dev, uintptr(t.image), uintptr(t.memory), 0), "bind image"); err != nil {
		return err
	}
	t.view, err = d.createView(t.image, format, aspect)
	return err
}

func (d *Device) createView(img VkImage, format, aspect uint32) (VkImageView, error) {
	return d.createViewOfLevels(img, format, aspect, 1)
}

// createViewOfLevels is createView for an image of the given number of mip
// levels, which the view covers.
func (d *Device) createViewOfLevels(img VkImage, format, aspect, levels uint32) (VkImageView, error) {
	info := VkImageViewCreateInfo{
		sType:            VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
		image:            img,
		viewType:         VK_IMAGE_VIEW_TYPE_2D,
		format:           format,
		subresourceRange: VkImageSubresourceRange{aspectMask: aspect, levelCount: levels, layerCount: 1},
	}
	var view VkImageView
	r := callResult(d.vkCreateImageView, uintptr(d.device), ptr(&info), 0, ptr(&view))
	return view, failure(r, "create image view")
}

// releaseTargets frees what rebuild makes. The GPU must be idle.
func (s *Surface) releaseTargets() {
	d := s.device
	dev := uintptr(d.device)
	for pass := range s.framebuffers {
		for _, fb := range s.framebuffers[pass] {
			if fb != 0 {
				call(d.vkDestroyFramebuffer, dev, uintptr(fb), 0)
			}
		}
		s.framebuffers[pass] = nil
	}
	for _, v := range s.views {
		if v != 0 {
			call(d.vkDestroyImageView, dev, uintptr(v), 0)
		}
	}
	for _, list := range [][]VkSemaphore{s.rendered, s.acquired} {
		for _, sem := range list {
			if sem != 0 {
				call(d.vkDestroySemaphore, dev, uintptr(sem), 0)
			}
		}
	}
	for _, t := range []*target{&s.color, &s.multiDepth, &s.depth} {
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
	s.views, s.rendered, s.acquired, s.images = nil, nil, nil, nil
	s.acquiredIndex = 0
}

// acquire gets the image of the next frame. A swap chain that went out of date
// since the last frame is replaced first.
func (s *Surface) acquire() error {
	d := s.device
	for {
		if s.stale {
			if err := s.rebuild(); err != nil {
				return err
			}
			if s.stale {
				return errors.New("vulkan: drawing to a window without a size")
			}
		}
		s.acquiredIndex = (s.acquiredIndex + 1) % len(s.acquired)
		r := d.hotResult(d.vkAcquireNextImageKHR, uintptr(d.device), uintptr(s.swapchain), uintptr(UINT64_MAX),
			uintptr(s.acquired[s.acquiredIndex]), 0, ptr(&s.imageIndex))
		switch r {
		case VK_SUCCESS:
			return nil
		case VK_SUBOPTIMAL_KHR:
			// It still works, and the frame after this one gets a better fit.
			s.stale = true
			return nil
		case VK_ERROR_OUT_OF_DATE_KHR:
			s.stale = true
			continue
		}
		return failure(r, "acquire image")
	}
}

func (s *Surface) present() error {
	d := s.device
	d.presentInfo = VkPresentInfoKHR{
		sType:              VK_STRUCTURE_TYPE_PRESENT_INFO_KHR,
		waitSemaphoreCount: 1,
		pWaitSemaphores:    &s.rendered[s.imageIndex],
		swapchainCount:     1,
		pSwapchains:        &s.swapchain,
		pImageIndices:      &s.imageIndex,
	}
	r := d.hotResult(d.vkQueuePresentKHR, uintptr(d.queue), ptr(&d.presentInfo))
	if r == VK_SUBOPTIMAL_KHR || r == VK_ERROR_OUT_OF_DATE_KHR {
		s.stale = true
		return nil
	}
	return failure(r, "present")
}

// Destroy implements gpu.Surface.
func (s *Surface) Destroy() {
	d := s.device
	// A device that cannot go idle is going away anyway.
	_ = d.waitIdle() //nolint:errcheck // see above
	d.liveSurfaces--
	s.destroy()
}

func (s *Surface) destroy() {
	d := s.device
	s.releaseTargets()
	if s.swapchain != 0 {
		call(d.vkDestroySwapchainKHR, uintptr(d.device), uintptr(s.swapchain), 0)
	}
	call(d.vkDestroySurfaceKHR, uintptr(d.instance), uintptr(s.surface), 0)
	*s = Surface{}
}

// ReadPixels returns the frame drawn so far. Call it between Draw and EndFrame:
// it ends the frame's commands, so that nothing more can be drawn. It stalls the
// GPU and exists for tests and screenshots.
func (d *Device) ReadPixels(s gpu.Surface) (*image.NRGBA, error) {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: ReadPixels called outside BeginFrame and EndFrame")
	}
	width, height := int(sf.extent.width), int(sf.extent.height)
	readback, err := d.newBuffer(uint64(width*height*4), VK_BUFFER_USAGE_TRANSFER_DST_BIT)
	if err != nil {
		return nil, err
	}
	defer d.freeBuffer(&readback)

	f := &d.frames[d.current]
	commands := uintptr(f.commands)
	d.hot(d.vkCmdEndRenderPass, commands)
	sf.rendering = false
	img := sf.images[sf.imageIndex]
	d.imageBarrier(f, img, VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
		VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
		VK_IMAGE_LAYOUT_PRESENT_SRC_KHR, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL)
	region := VkBufferImageCopy{
		imageSubresource: VkImageSubresourceLayers{aspectMask: VK_IMAGE_ASPECT_COLOR_BIT, layerCount: 1},
		imageExtent:      VkExtent3D{sf.extent.width, sf.extent.height, 1},
	}
	call(d.vkCmdCopyImageToBuffer, commands, uintptr(img), VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, uintptr(readback.handle), 1, ptr(&region))
	d.imageBarrier(f, img, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT,
		VK_ACCESS_TRANSFER_READ_BIT, 0,
		VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, VK_IMAGE_LAYOUT_PRESENT_SRC_KHR)
	if err := d.submit(sf); err != nil {
		return nil, err
	}
	if err := d.waitFor(f); err != nil {
		return nil, err
	}

	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	// mapped holds width*height pixels, which the fence above made visible.
	src := unsafe.Slice((*byte)(readback.mapped), width*height*4)
	red, blue := 0, 2
	if sf.format.format == VK_FORMAT_B8G8R8A8_UNORM {
		red, blue = 2, 0
	}
	for i := 0; i < len(src); i += 4 {
		// The window ignores alpha, so the frame is opaque by definition.
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = src[i+red], src[i+1], src[i+blue], 0xff
	}
	return out, nil
}

// imageBarrier moves the single level of img to another layout.
func (d *Device) imageBarrier(f *frame, img VkImage, srcStage, dstStage, srcAccess, dstAccess, oldLayout, newLayout uint32) {
	d.barrier = VkImageMemoryBarrier{
		sType:               VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER,
		srcAccessMask:       srcAccess,
		dstAccessMask:       dstAccess,
		oldLayout:           oldLayout,
		newLayout:           newLayout,
		srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED,
		dstQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED,
		image:               img,
		subresourceRange:    d.clearRange,
	}
	d.hot(d.vkCmdPipelineBarrier, uintptr(f.commands), uintptr(srcStage), uintptr(dstStage), 0, 0, 0, 0, 0, 1, ptr(&d.barrier))
}
