//go:build linux || windows

package vulkan

import "unsafe"

// targetConstantsPerPool is how many sets of target constants one descriptor
// pool serves.
const targetConstantsPerPool = 32

// targetConstants is a uniform buffer of frame constants, and its set, that the
// targets drawn in one frame with those constants bind. The commands of a frame
// run after all of them are recorded, so that targets of different sizes or
// scales, such as the pictures of groups and the passes of their filters, each
// need their own. See docs/ADR/0114-gpu-layers-masks-filters.md.
type targetConstants struct {
	buffer buffer
	set    VkDescriptorSet
	value  frameConstants
}

// targetSet returns the set that holds c for a target drawn in frame f, made if
// no target of the frame has had those constants yet. Steady frames reuse what
// they made.
func (d *Device) targetSet(f *frame, c frameConstants) (*VkDescriptorSet, error) {
	for i := range f.targetUsed {
		if t := &f.targets[i]; t.value == c {
			return &t.set, nil
		}
	}
	if f.targetUsed == len(f.targets) {
		var t targetConstants
		if err := d.createTargetConstants(&t); err != nil {
			return nil, err
		}
		f.targets = append(f.targets, t)
	}
	t := &f.targets[f.targetUsed]
	f.targetUsed++
	// The fence of this frame has passed, so the GPU is not reading this.
	*(*frameConstants)(t.buffer.mapped) = c
	t.value = c
	return &t.set, nil
}

// createTargetConstants makes the buffer and the set of t, from a pool of its
// own kind, which grows by a pool when full.
func (d *Device) createTargetConstants(t *targetConstants) error {
	var err error
	if t.buffer, err = d.newBuffer(uint64(unsafe.Sizeof(frameConstants{})), VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT); err != nil {
		return err
	}
	if d.constantSets%targetConstantsPerPool == 0 {
		size := VkDescriptorPoolSize{VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, targetConstantsPerPool}
		info := VkDescriptorPoolCreateInfo{
			sType:         VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
			maxSets:       targetConstantsPerPool,
			poolSizeCount: 1,
			pPoolSizes:    &size,
		}
		var pool VkDescriptorPool
		if err := failure(callResult(d.vkCreateDescriptorPool, uintptr(d.device), ptr(&info), 0, ptr(&pool)), "create descriptor pool"); err != nil {
			d.freeBuffer(&t.buffer)
			return err
		}
		d.constantPools = append(d.constantPools, pool)
	}
	alloc := VkDescriptorSetAllocateInfo{
		sType:              VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
		descriptorPool:     d.constantPools[len(d.constantPools)-1],
		descriptorSetCount: 1,
		pSetLayouts:        &d.frameLayout,
	}
	if err := failure(callResult(d.vkAllocateDescriptorSets, uintptr(d.device), ptr(&alloc), ptr(&t.set)), "allocate descriptor set"); err != nil {
		d.freeBuffer(&t.buffer)
		return err
	}
	d.constantSets++
	info := VkDescriptorBufferInfo{buffer: t.buffer.handle, rng: VK_WHOLE_SIZE}
	write := VkWriteDescriptorSet{
		sType:           VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		dstSet:          t.set,
		descriptorCount: 1,
		descriptorType:  VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER,
		pBufferInfo:     &info,
	}
	call(d.vkUpdateDescriptorSets, uintptr(d.device), 1, ptr(&write), 0, 0)
	return nil
}
