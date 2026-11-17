//go:build linux || windows

package vulkan

import (
	"errors"
	"unsafe"
)

// buffer is host-visible memory that stays mapped.
type buffer struct {
	handle VkBuffer
	memory VkDeviceMemory
	mapped unsafe.Pointer
	size   uint64
}

// arena hands out parts of a buffer for one frame. A request that does not fit
// moves to a bigger buffer; the old one lives on until the commands that use it
// are done.
type arena struct {
	buffer buffer
	usage  uint32
	used   uint64
}

// memoryType finds a memory type among the allowed bits with every flag.
func (d *Device) memoryType(bits, flags uint32) (uint32, error) {
	for i := range d.memory.memoryTypeCount {
		if bits&(1<<i) != 0 && d.memory.memoryTypes[i].propertyFlags&flags == flags {
			return i, nil
		}
	}
	return 0, errors.New("vulkan: no suitable memory type")
}

func (d *Device) allocate(req *VkMemoryRequirements, flags uint32) (VkDeviceMemory, error) {
	index, err := d.memoryType(req.memoryTypeBits, flags)
	if err != nil {
		return 0, err
	}
	info := VkMemoryAllocateInfo{
		sType:           VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		allocationSize:  req.size,
		memoryTypeIndex: index,
	}
	var memory VkDeviceMemory
	r := callResult(d.vkAllocateMemory, uintptr(d.device), ptr(&info), 0, ptr(&memory))
	return memory, failure(r, "allocate memory")
}

func (d *Device) newBuffer(size uint64, usage uint32) (buffer, error) {
	info := VkBufferCreateInfo{
		sType:       VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,
		size:        size,
		usage:       usage,
		sharingMode: VK_SHARING_MODE_EXCLUSIVE,
	}
	b := buffer{size: size}
	dev := uintptr(d.device)
	if err := failure(callResult(d.vkCreateBuffer, dev, ptr(&info), 0, ptr(&b.handle)), "create buffer"); err != nil {
		return buffer{}, err
	}
	var req VkMemoryRequirements
	call(d.vkGetBufferMemoryReqs, dev, uintptr(b.handle), ptr(&req))
	var err error
	if b.memory, err = d.allocate(&req, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|VK_MEMORY_PROPERTY_HOST_COHERENT_BIT); err != nil {
		d.freeBuffer(&b)
		return buffer{}, err
	}
	if err := failure(callResult(d.vkBindBufferMemory, dev, uintptr(b.handle), uintptr(b.memory), 0), "bind buffer"); err != nil {
		d.freeBuffer(&b)
		return buffer{}, err
	}
	r := callResult(d.vkMapMemory, dev, uintptr(b.memory), 0, uintptr(VK_WHOLE_SIZE), 0, ptr(&b.mapped))
	if err := failure(r, "map memory"); err != nil {
		d.freeBuffer(&b)
		return buffer{}, err
	}
	return b, nil
}

// freeBuffer releases b at once. The GPU must not be using it.
func (d *Device) freeBuffer(b *buffer) {
	dev := uintptr(d.device)
	if b.handle != 0 {
		call(d.vkDestroyBuffer, dev, uintptr(b.handle), 0)
	}
	if b.memory != 0 {
		// Freeing also unmaps.
		call(d.vkFreeMemory, dev, uintptr(b.memory), 0)
	}
	*b = buffer{}
}

// reserve returns where size bytes of a can be written, as an offset into the
// current buffer and as a pointer.
func (d *Device) reserve(a *arena, size uint64) (uint64, unsafe.Pointer, error) {
	// 16 covers every texel and vertex format here, and copies like it.
	const alignment = 16
	offset := (a.used + alignment - 1) &^ (alignment - 1)
	if offset+size > a.buffer.size {
		next, err := d.newBuffer(max(size, a.buffer.size*2, initialArena), a.usage)
		if err != nil {
			return 0, nil, err
		}
		if old := a.buffer; old.handle != 0 {
			d.retire(func() { d.freeBuffer(&old) })
		}
		a.buffer, offset = next, 0
	}
	a.used = offset + size
	// mapped covers the whole buffer, and offset lies inside it.
	return offset, unsafe.Add(a.buffer.mapped, offset), nil
}
