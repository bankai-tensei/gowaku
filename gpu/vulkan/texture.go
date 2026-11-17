//go:build linux || windows

package vulkan

import (
	"errors"
	"image"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
)

// Texture is a texture the pixel shader can read.
type Texture struct {
	device *Device
	image  VkImage
	memory VkDeviceMemory
	view   VkImageView
	set    VkDescriptorSet
	pool   VkDescriptorPool
	format gpu.TextureFormat
	// framebuffer exists on what CreateTarget made, and multiFramebuffer on
	// those that a multisampled batch was drawn to, for the scratch of
	// generation multiGen.
	framebuffer      VkFramebuffer
	multiFramebuffer VkFramebuffer
	multiGen         uint32
	width, height    uint32
	// levels is the number of mip levels, which zero leaves at one.
	levels uint32
}

var _ gpu.Texture = (*Texture)(nil)

// CreateTexture implements gpu.Backend.
func (d *Device) CreateTexture(width, height int, format gpu.TextureFormat) (gpu.Texture, error) {
	t, err := d.createTexture(width, height, format)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *Device) createTexture(width, height int, format gpu.TextureFormat) (*Texture, error) {
	if d.drawing != nil {
		panic("gowaku: CreateTexture called between BeginFrame and EndFrame")
	}
	var vk uint32
	switch format {
	case gpu.TextureR8:
		vk = VK_FORMAT_R8_UNORM
	case gpu.TextureRGBA8, gpu.TextureRGBA8Mipmapped:
		vk = VK_FORMAT_R8G8B8A8_UNORM
	default:
		return nil, errors.New("vulkan: unknown texture format")
	}
	if err := d.prepare(); err != nil {
		return nil, err
	}
	t := &Texture{device: d, format: format, width: uint32(width), height: uint32(height)}
	if format == gpu.TextureRGBA8Mipmapped {
		t.levels = uint32(gpu.MipLevels(width, height))
	}
	if err := t.init(uint32(width), uint32(height), vk, VK_IMAGE_USAGE_SAMPLED_BIT|VK_IMAGE_USAGE_TRANSFER_DST_BIT); err != nil {
		t.free()
		return nil, err
	}
	return t, nil
}

func (t *Texture) init(width, height, format, usage uint32) error {
	d := t.device
	dev := uintptr(d.device)
	info := VkImageCreateInfo{
		sType:       VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		imageType:   VK_IMAGE_TYPE_2D,
		format:      format,
		extent:      VkExtent3D{width, height, 1},
		mipLevels:   max(t.levels, 1),
		arrayLayers: 1,
		samples:     VK_SAMPLE_COUNT_1_BIT,
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
	if t.view, err = d.createViewOfLevels(t.image, format, VK_IMAGE_ASPECT_COLOR_BIT, max(t.levels, 1)); err != nil {
		return err
	}
	if err := d.allocateTextureSet(t); err != nil {
		return err
	}
	descriptor := VkDescriptorImageInfo{imageView: t.view, imageLayout: VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL}
	write := VkWriteDescriptorSet{
		sType:           VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		dstSet:          t.set,
		descriptorCount: 1,
		descriptorType:  VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE,
		pImageInfo:      &descriptor,
	}
	call(d.vkUpdateDescriptorSets, dev, 1, ptr(&write), 0, 0)

	// A texture starts out empty, as in Direct3D 11, and readable, so that the
	// layout of the descriptor holds from here on.
	f := &d.frames[d.current]
	d.imageBarrier(f, t.image, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
		0, VK_ACCESS_TRANSFER_WRITE_BIT,
		VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL)
	call(d.vkCmdClearColorImage, uintptr(f.commands), uintptr(t.image), VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
		ptr(&d.clearColor), 1, ptr(&d.clearRange))
	d.imageBarrier(f, t.image, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
		VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
		VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL)
	return nil
}

// allocateTextureSet takes a descriptor set from the newest pool, and adds a pool
// when that one is full.
func (d *Device) allocateTextureSet(t *Texture) error {
	info := VkDescriptorSetAllocateInfo{
		sType:              VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
		descriptorSetCount: 1,
		pSetLayouts:        &d.textureLayout,
	}
	if n := len(d.texturePools); n > 0 {
		info.descriptorPool = d.texturePools[n-1]
		r := callResult(d.vkAllocateDescriptorSets, uintptr(d.device), ptr(&info), ptr(&t.set))
		if r == VK_SUCCESS {
			t.pool = info.descriptorPool
			return nil
		}
		if r != VK_ERROR_OUT_OF_POOL_MEMORY && r != VK_ERROR_FRAGMENTED_POOL {
			return failure(r, "allocate descriptor set")
		}
	}
	size := VkDescriptorPoolSize{VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE, descriptorsPerPool}
	pool := VkDescriptorPoolCreateInfo{
		sType:         VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
		flags:         VK_DESCRIPTOR_POOL_CREATE_FREE_DESCRIPTOR_SET_BIT,
		maxSets:       descriptorsPerPool,
		poolSizeCount: 1,
		pPoolSizes:    &size,
	}
	r := callResult(d.vkCreateDescriptorPool, uintptr(d.device), ptr(&pool), 0, ptr(&info.descriptorPool))
	if err := failure(r, "create descriptor pool"); err != nil {
		return err
	}
	d.texturePools = append(d.texturePools, info.descriptorPool)
	t.pool = info.descriptorPool
	r = callResult(d.vkAllocateDescriptorSets, uintptr(d.device), ptr(&info), ptr(&t.set))
	return failure(r, "allocate descriptor set")
}

// UpdateTexture implements gpu.Backend. The copy runs ahead of the next frame.
func (d *Device) UpdateTexture(t gpu.Texture, region image.Rectangle, pix []byte, stride int) error {
	if region.Empty() {
		return nil
	}
	if d.drawing != nil {
		panic("gowaku: UpdateTexture called between BeginFrame and EndFrame")
	}
	if err := d.prepare(); err != nil {
		return err
	}
	tx := t.(*Texture)
	mipmapped := tx.format == gpu.TextureRGBA8Mipmapped
	if mipmapped && region != image.Rect(0, 0, int(tx.width), int(tx.height)) {
		panic("gowaku: UpdateTexture of a mipmapped texture must cover all of it")
	}
	f := &d.frames[d.current]
	d.imageBarrier(f, tx.image, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
		VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_TRANSFER_WRITE_BIT,
		VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL)
	err := d.copyLevel(f, tx, 0, region, pix, stride)
	if err == nil && mipmapped {
		mips := gpu.BuildMips(pix, stride, int(tx.width), int(tx.height))
		at := 0
		for level := 1; level < int(tx.levels) && err == nil; level++ {
			r := gpu.MipRegion(int(tx.width), int(tx.height), level)
			err = d.copyLevel(f, tx, level, r, mips[at:], r.Dx()*4)
			at += r.Dx() * r.Dy() * 4
		}
	}
	// The barrier goes in after a failure too, so that the layout of the
	// descriptor holds.
	d.imageBarrier(f, tx.image, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
		VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
		VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL)
	return err
}

// copyLevel records the copy of region of pix, with the given stride, to the
// same place of a mip level of tx, which is in the layout for transfers.
func (d *Device) copyLevel(f *frame, tx *Texture, level int, region image.Rectangle, pix []byte, stride int) error {
	bpp := tx.format.BytesPerPixel()
	width, height := region.Dx(), region.Dy()
	row := width * bpp
	offset, target, err := d.reserve(&f.staging, uint64(row*height))
	if err != nil {
		return err
	}
	// target is host-visible memory with room for the whole region.
	dst := unsafe.Slice((*byte)(target), row*height)
	for y := range height {
		start := (region.Min.Y+y)*stride + region.Min.X*bpp
		copy(dst[y*row:(y+1)*row], pix[start:start+row])
	}
	d.copyRegion = VkBufferImageCopy{
		bufferOffset:     offset,
		imageSubresource: VkImageSubresourceLayers{aspectMask: VK_IMAGE_ASPECT_COLOR_BIT, mipLevel: uint32(level), layerCount: 1},
		imageOffset:      VkOffset3D{int32(region.Min.X), int32(region.Min.Y), 0},
		imageExtent:      VkExtent3D{uint32(width), uint32(height), 1},
	}
	d.hot(d.vkCmdCopyBufferToImage, uintptr(f.commands), uintptr(f.staging.buffer.handle), uintptr(tx.image),
		VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, ptr(&d.copyRegion))
	return nil
}

// Destroy implements gpu.Texture. Frames in flight can still read the texture,
// so it goes when they are done.
func (t *Texture) Destroy() {
	d := t.device
	if d.blank == t {
		t.free()
		return
	}
	old := *t
	d.retire(old.free)
	*t = Texture{}
}

func (t *Texture) free() {
	d := t.device
	dev := uintptr(d.device)
	if t.framebuffer != 0 {
		call(d.vkDestroyFramebuffer, dev, uintptr(t.framebuffer), 0)
	}
	if t.multiFramebuffer != 0 {
		call(d.vkDestroyFramebuffer, dev, uintptr(t.multiFramebuffer), 0)
	}
	if t.set != 0 {
		call(d.vkFreeDescriptorSets, dev, uintptr(t.pool), 1, ptr(&t.set))
	}
	if t.view != 0 {
		call(d.vkDestroyImageView, dev, uintptr(t.view), 0)
	}
	if t.image != 0 {
		call(d.vkDestroyImage, dev, uintptr(t.image), 0)
	}
	if t.memory != 0 {
		call(d.vkFreeMemory, dev, uintptr(t.memory), 0)
	}
}
