//go:build linux || windows

package vulkan

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

const (
	// framesInFlight is how many frames the CPU may record ahead of the GPU.
	// Two makes the per-frame buffers a ring.
	framesInFlight = 2

	// defaultSamples is the MSAA level of a frame that needs it. See
	// docs/ADR/0017-multisample-on-demand.md.
	defaultSamples = 4

	// maxArgs is the most any Vulkan function gowaku calls takes.
	maxArgs = 12

	// quadVertices is the length of the triangle strip of one instance.
	quadVertices = 4

	// descriptorsPerPool is how many textures one descriptor pool serves.
	descriptorsPerPool = 256

	initialArena = 1 << 16

	instanceStride = uint64(unsafe.Sizeof(gpu.Instance{}))
	vertexStride   = uint64(unsafe.Sizeof(geom.Offset{}))
)

// Options configures New. The zero value selects the most power-efficient GPU
// and waits for vertical blank.
type Options struct {
	Power gpu.PowerPreference
	// Samples is the MSAA level, which smooths paths and shaped clips. Zero
	// selects 4, and 1 turns it off. A level the GPU lacks falls back to the
	// next one it has.
	Samples uint32
	// PresentImmediately stops EndFrame from waiting for vertical blank, if the
	// system can do that. It exists to measure how fast frames can be drawn.
	PresentImmediately bool
}

// frameConstants mirrors cbuffer Frame in sdf.hlsli.
type frameConstants struct {
	viewportWidth, viewportHeight float32
	scale                         float32
	padding                       float32
}

// frame is what one frame in flight owns. The GPU may read it until the fence
// signals, so that the CPU never writes what is being drawn from.
type frame struct {
	fence     VkFence
	submitted bool
	commands  VkCommandBuffer

	constants buffer
	set       VkDescriptorSet
	// targets hold the constants of what DrawToTarget draws in this frame,
	// the first targetUsed of them in use.
	targets    []targetConstants
	targetUsed int

	instances, vertices, staging arena
	// retired releases what the commands of this frame may still use.
	retired []func()
}

// Device is the Vulkan implementation of gpu.Backend. It owns the single device
// of the process; every Surface shares it.
//
// The structs passed to Vulkan on the hot path are fields, because pointers
// handed to a C call escape and locals would allocate on every frame.
type Device struct {
	procs
	instance VkInstance
	// surfaces says which entries of surfaceKinds the instance has.
	surfaces [len(surfaceKinds)]bool
	physical VkPhysicalDevice
	device   VkDevice
	queue    VkQueue
	family   uint32
	name     string
	memory   VkPhysicalDeviceMemoryProperties

	samples     uint32
	depthFormat uint32
	// scissorInShader repeats the scissor test in the pixel shader of
	// multisampled passes, for lavapipe; see SCISSOR_IN_SHADER in sdf.ps.hlsl.
	// scissorBounds is the push constant that carries the rectangle.
	scissorInShader bool
	scissorBounds   [4]int32
	immediate       bool
	colorFormat     uint32
	// targetPass and targetDepth exist from the first CreateTarget on.
	targetPass    VkRenderPass
	targetDepth   target
	targetSurface Surface
	// multi is where batches that need multisampling draw for a target.
	multi           multiTarget
	rects           []VkClearRect
	clearAttachment VkClearAttachment
	passes          [passCount]framePass
	// pass is that of the frame in progress.
	pass          *framePass
	sdfVertex     VkShaderModule
	sdfFragment   VkShaderModule
	pathVertex    VkShaderModule
	sampler       VkSampler
	frameLayout   VkDescriptorSetLayout
	textureLayout VkDescriptorSetLayout
	layouts       [6]VkDescriptorSetLayout
	pipeLayout    VkPipelineLayout
	framePool     VkDescriptorPool
	texturePools  []VkDescriptorPool
	// constantPools hold the sets of targetConstants, constantSets of them.
	constantPools []VkDescriptorPool
	constantSets  int
	commandPool   VkCommandPool

	frames  [framesInFlight]frame
	current int
	// recording means that frames[current] accepts commands.
	recording bool
	// drawing is the surface between BeginFrame and EndFrame.
	drawing *Surface
	// blank stands for a texture a draw call does not name: a pipeline's sets
	// must all be bound, used or not.
	blank *Texture
	// backdrop is the copy of the target that blend modes which read it see,
	// made when first needed and as large as the largest target so far.
	backdrop *Texture
	// blurA and blurB are the scratch targets of blurs, made when first
	// needed, and blurs the blurs of the batch in progress.
	blurA, blurB *Texture
	blurs        []blurPlan

	// What is bound now, because a batch switches between these often.
	bound      VkPipeline
	boundLevel int
	// textureSet holds the sets of the atlas, the image, the ramps, the
	// backdrop and the blurred picture.
	textureSet [5]VkDescriptorSet

	beginInfo   VkCommandBufferBeginInfo
	passInfo    VkRenderPassBeginInfo
	clears      [2]VkClearValue
	viewport    VkViewport
	surfaceSize VkViewport
	scissor     VkRect2D
	lastScissor VkRect2D
	bindBuffers [2]VkBuffer
	bindOffsets [2]uint64
	waitStage   uint32
	submitInfo  VkSubmitInfo
	presentInfo VkPresentInfoKHR
	barrier     VkImageMemoryBarrier
	copyRegion  VkBufferImageCopy
	imageCopy   VkImageCopy
	clearColor  VkClearValue
	clearRange  VkImageSubresourceRange

	// args holds the arguments of hot.
	args [maxArgs]uintptr

	liveSurfaces int

	// timestamps says whether the queue can take timestamps, each
	// timestampPeriod nanoseconds long, and timer measures frames on the GPU
	// from EnableGPUTiming on.
	timestamps      bool
	timestampPeriod float32
	timer           *timer

	// shaders holds the custom shaders that PrepareShader took.
	shaders map[*gpu.Shader]*customShader
}

var _ gpu.Backend = (*Device)(nil)

// ptr converts a pointer for call, and moves what it points to off the stack.
func ptr[T any](p *T) uintptr {
	kept.slots[kept.next%uint8(len(kept.slots))] = unsafe.Pointer(p)
	kept.next++
	return uintptr(unsafe.Pointer(p))
}

// kept holds the latest arguments of ptr. A uintptr hides a pointer from Go in two
// ways, and go:uintptrescapes helps only with a conversion written in the call
// itself, not one inside ptr. The stack moves as it grows, so that C would write
// to the old copy: storing the pointer here moves the object to the heap. And the
// collector may free an object that nothing refers to during the call, with what
// its fields point to: this keeps it until later calls have pushed it out, and no
// call has as many pointers as there are slots. It belongs to the UI goroutine, like
// every call of this package, and it is the one mutable variable here.
var kept struct {
	next  uint8
	slots [16]unsafe.Pointer
}

// hot is call for what runs on every frame. The arguments of call escape to the
// heap, because the C call needs them there. Copying them to a field instead
// keeps them on the stack of the caller. Pointers among them must lead to memory
// that outlives the call by itself, which means fields of the Device or Surface.
func (d *Device) hot(fn uintptr, args ...uintptr) uintptr {
	n := copy(d.args[:], args)
	r, _, _ := purego.SyscallN(fn, d.args[:n]...)
	return r
}

// hotResult is hot for the functions that return a VkResult.
func (d *Device) hotResult(fn uintptr, args ...uintptr) VkResult {
	return VkResult(int32(d.hot(fn, args...)))
}

// failure describes a VkResult. A lost device panics: it is a broken contract,
// not an error to return.
func failure(r VkResult, op string) error {
	if r == VK_ERROR_DEVICE_LOST {
		panic("gowaku: GPU device lost during " + op)
	}
	if r < 0 {
		return fmt.Errorf("vulkan: %s: VkResult %d", op, int32(r))
	}
	return nil
}

// New creates the device. It returns gpu.ErrNoGPU if the system has no Vulkan
// or no GPU that can present.
func New(opts Options) (*Device, error) {
	d := &Device{samples: opts.Samples, immediate: opts.PresentImmediately}
	if d.samples == 0 {
		d.samples = defaultSamples
	}
	if err := d.init(opts); err != nil {
		d.release()
		return nil, err
	}
	slog.Debug("vulkan: device created", "adapter", d.name, "power", opts.Power, "samples", d.samples)
	return d, nil
}

func (d *Device) init(opts Options) error {
	if err := d.loadGlobal(); err != nil {
		return err
	}
	if err := d.createInstance(); err != nil {
		return err
	}
	if err := d.loadInstance(d.instance); err != nil {
		return err
	}
	if err := d.loadSurfaces(d.instance, d.surfaces); err != nil {
		return err
	}
	if err := d.pickDevice(opts.Power); err != nil {
		return err
	}
	if err := d.createDevice(); err != nil {
		return err
	}
	if err := d.loadDevice(d.device); err != nil {
		return err
	}
	call(d.vkGetDeviceQueue, uintptr(d.device), uintptr(d.family), 0, ptr(&d.queue))
	if err := d.createResources(); err != nil {
		return err
	}
	var err error
	d.blank, err = d.createTexture(1, 1, gpu.TextureR8)
	return err
}

// surfaceKind is a window system: the instance extension that draws to it,
// and the function of the extension that makes a surface.
type surfaceKind struct{ extension, function string }

// cname returns the string in a fixed-size C array.
func cname(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// instanceExtensions reports which entries of surfaceKinds the loader has.
func (d *Device) instanceExtensions() ([len(surfaceKinds)]bool, error) {
	var found [len(surfaceKinds)]bool
	var count uint32
	if r := callResult(d.vkEnumerateInstanceExtensionProperties, 0, ptr(&count), 0); r != VK_SUCCESS {
		return found, failure(r, "enumerate instance extensions")
	}
	if count == 0 {
		return found, nil
	}
	props := make([]VkExtensionProperties, count)
	r := callResult(d.vkEnumerateInstanceExtensionProperties, 0, ptr(&count), ptr(&props[0]))
	if r != VK_SUCCESS && r != VK_INCOMPLETE {
		return found, failure(r, "enumerate instance extensions")
	}
	for i := range props[:count] {
		name := cname(props[i].extensionName[:])
		for i, kind := range surfaceKinds {
			if name == kind.extension {
				found[i] = true
			}
		}
	}
	return found, nil
}

func (d *Device) createInstance() error {
	found, err := d.instanceExtensions()
	if err != nil {
		return err
	}
	names := []*byte{cstring("VK_KHR_surface")}
	for i, kind := range surfaceKinds {
		if found[i] {
			names = append(names, cstring(kind.extension))
		}
	}
	if len(names) == 1 {
		return fmt.Errorf("vulkan: the loader has no surface extension for this system: %w", gpu.ErrNoGPU)
	}
	d.surfaces = found
	app := VkApplicationInfo{
		sType:       VK_STRUCTURE_TYPE_APPLICATION_INFO,
		pEngineName: cstring("gowaku"),
		apiVersion:  VK_API_VERSION_1_0,
	}
	info := VkInstanceCreateInfo{
		sType:                   VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
		pApplicationInfo:        &app,
		enabledExtensionCount:   uint32(len(names)),
		ppEnabledExtensionNames: &names[0],
	}
	r := callResult(d.vkCreateInstance, ptr(&info), 0, ptr(&d.instance))
	switch r {
	case VK_ERROR_INCOMPATIBLE_DRIVER, VK_ERROR_EXTENSION_NOT_PRESENT:
		return fmt.Errorf("vulkan: create instance: %w (VkResult %d)", gpu.ErrNoGPU, int32(r))
	}
	return failure(r, "create instance")
}

// rank orders the kinds of device by preference. A CPU implementation comes
// last but still counts: without one a virtual machine shows no window at all.
func rank(deviceType uint32, power gpu.PowerPreference) int {
	switch deviceType {
	case VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU:
		if power == gpu.HighPerformance {
			return 0
		}
		return 1
	case VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU:
		if power == gpu.HighPerformance {
			return 1
		}
		return 0
	case VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU:
		return 2
	case VK_PHYSICAL_DEVICE_TYPE_CPU:
		return 4
	}
	return 3
}

func (d *Device) physicalDevices() ([]VkPhysicalDevice, error) {
	var count uint32
	if err := failure(callResult(d.vkEnumeratePhysicalDevices, uintptr(d.instance), ptr(&count), 0), "enumerate devices"); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gpu.ErrNoGPU
	}
	devices := make([]VkPhysicalDevice, count)
	r := callResult(d.vkEnumeratePhysicalDevices, uintptr(d.instance), ptr(&count), ptr(&devices[0]))
	if err := failure(r, "enumerate devices"); err != nil {
		return nil, err
	}
	return devices[:count], nil
}

// graphicsFamily returns the first queue family that can draw.
func (d *Device) graphicsFamily(p VkPhysicalDevice) (uint32, bool) {
	var count uint32
	call(d.vkGetPhysicalDeviceQueueFamilyProperties, uintptr(p), ptr(&count), 0)
	if count == 0 {
		return 0, false
	}
	families := make([]VkQueueFamilyProperties, count)
	call(d.vkGetPhysicalDeviceQueueFamilyProperties, uintptr(p), ptr(&count), ptr(&families[0]))
	for i := range families[:count] {
		if families[i].queueFlags&VK_QUEUE_GRAPHICS_BIT != 0 {
			return uint32(i), true
		}
	}
	return 0, false
}

func (d *Device) hasSwapchain(p VkPhysicalDevice) bool {
	var count uint32
	r := callResult(d.vkEnumerateDeviceExtensionProperties, uintptr(p), 0, ptr(&count), 0)
	if r != VK_SUCCESS || count == 0 {
		return false
	}
	extensions := make([]VkExtensionProperties, count)
	r = callResult(d.vkEnumerateDeviceExtensionProperties, uintptr(p), 0, ptr(&count), ptr(&extensions[0]))
	if r < 0 {
		return false
	}
	const name = "VK_KHR_swapchain\x00"
	for i := range extensions[:count] {
		if string(extensions[i].extensionName[:len(name)]) == name {
			return true
		}
	}
	return false
}

func (d *Device) pickDevice(power gpu.PowerPreference) error {
	devices, err := d.physicalDevices()
	if err != nil {
		return err
	}
	best := -1
	var props VkPhysicalDeviceProperties
	for _, p := range devices {
		family, ok := d.graphicsFamily(p)
		if !ok || !d.hasSwapchain(p) {
			continue
		}
		call(d.vkGetPhysicalDeviceProperties, uintptr(p), ptr(&props))
		r := rank(props.u32(offsetDeviceType), power)
		if best >= 0 && r >= best {
			continue
		}
		best, d.physical, d.family, d.name = r, p, family, props.name()
		d.scissorInShader = props.u32(offsetVendorID) == VK_VENDOR_ID_MESA && props.u32(offsetDeviceType) == VK_PHYSICAL_DEVICE_TYPE_CPU
		d.samples = sampleCount(d.samples, &props)
		d.timestampPeriod = props.f32(offsetTimestampPeriod)
		d.timestamps = props.u32(offsetTimestampComputeAndGraphics) != 0
	}
	if best < 0 {
		return gpu.ErrNoGPU
	}
	call(d.vkGetPhysicalDeviceMemoryProperties, uintptr(d.physical), ptr(&d.memory))
	return d.pickDepthFormat()
}

// sampleCount returns the highest level up to want that the attachments of a
// frame can all have.
func sampleCount(want uint32, props *VkPhysicalDeviceProperties) uint32 {
	supported := props.u32(offsetColorSampleCounts) & props.u32(offsetDepthSampleCounts) & props.u32(offsetStencilSampleCounts)
	for want > 1 && supported&want == 0 {
		want >>= 1
	}
	return want
}

// pickDepthFormat prefers the format of the Direct3D 11 backend. Vulkan
// guarantees only that one of these two exists.
func (d *Device) pickDepthFormat() error {
	for _, format := range []uint32{VK_FORMAT_D24_UNORM_S8_UINT, VK_FORMAT_D32_SFLOAT_S8_UINT} {
		var props VkFormatProperties
		call(d.vkGetPhysicalDeviceFormatProperties, uintptr(d.physical), uintptr(format), ptr(&props))
		if props.optimalTilingFeatures&VK_FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT != 0 {
			d.depthFormat = format
			return nil
		}
	}
	return errors.New("vulkan: no depth and stencil format")
}

func (d *Device) createDevice() error {
	priority := float32(1)
	queue := VkDeviceQueueCreateInfo{
		sType:            VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO,
		queueFamilyIndex: d.family,
		queueCount:       1,
		pQueuePriorities: &priority,
	}
	names := []*byte{cstring("VK_KHR_swapchain")}
	info := VkDeviceCreateInfo{
		sType:                   VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO,
		queueCreateInfoCount:    1,
		pQueueCreateInfos:       &queue,
		enabledExtensionCount:   1,
		ppEnabledExtensionNames: &names[0],
	}
	r := callResult(d.vkCreateDevice, uintptr(d.physical), ptr(&info), 0, ptr(&d.device))
	if r < 0 {
		return fmt.Errorf("vulkan: create device on %q: %w: %w", d.name, gpu.ErrNoGPU, failure(r, "vkCreateDevice"))
	}
	return nil
}

func (d *Device) createResources() error {
	var err error
	if d.sdfVertex, err = d.createShader(sdfVS); err != nil {
		return err
	}
	if d.sdfFragment, err = d.createShader(sdfPS); err != nil {
		return err
	}
	if d.pathVertex, err = d.createShader(pathVS); err != nil {
		return err
	}
	sampler := VkSamplerCreateInfo{
		sType:        VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO,
		magFilter:    VK_FILTER_LINEAR,
		minFilter:    VK_FILTER_LINEAR,
		mipmapMode:   VK_SAMPLER_MIPMAP_MODE_LINEAR,
		addressModeU: VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
		addressModeV: VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
		addressModeW: VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
		maxLod:       1000,
	}
	r := callResult(d.vkCreateSampler, uintptr(d.device), ptr(&sampler), 0, ptr(&d.sampler))
	if err := failure(r, "create sampler"); err != nil {
		return err
	}
	if err := d.createLayouts(); err != nil {
		return err
	}
	pool := VkCommandPoolCreateInfo{
		sType:            VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
		flags:            VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
		queueFamilyIndex: d.family,
	}
	r = callResult(d.vkCreateCommandPool, uintptr(d.device), ptr(&pool), 0, ptr(&d.commandPool))
	if err := failure(r, "create command pool"); err != nil {
		return err
	}
	if err := d.createFramePool(); err != nil {
		return err
	}
	for i := range d.frames {
		if err := d.createFrame(&d.frames[i]); err != nil {
			return err
		}
	}
	d.beginInfo = VkCommandBufferBeginInfo{
		sType: VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO,
		flags: VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT,
	}
	d.waitStage = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT
	// Every level, so that clears and barriers cover the mip levels of a
	// TextureRGBA8Mipmapped too.
	d.clearRange = VkImageSubresourceRange{aspectMask: VK_IMAGE_ASPECT_COLOR_BIT, levelCount: VK_REMAINING_MIP_LEVELS, layerCount: 1}
	return nil
}

func (d *Device) createShader(code []byte) (VkShaderModule, error) {
	// Vulkan reads the code as 32-bit words. Copying gives it their alignment,
	// which embedded data does not promise.
	words := make([]uint32, (len(code)+3)/4)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(code)), code)
	info := VkShaderModuleCreateInfo{
		sType:    VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO,
		codeSize: uintptr(len(code)),
		pCode:    (*byte)(unsafe.Pointer(&words[0])),
	}
	var module VkShaderModule
	r := callResult(d.vkCreateShaderModule, uintptr(d.device), ptr(&info), 0, ptr(&module))
	return module, failure(r, "create shader module")
}

// createLayouts makes the six sets of the shaders: the frame, the glyph atlas,
// the image, the gradient ramps, the backdrop and the blurred picture of a blur.
// The last five share a layout, so that a texture needs only one descriptor set
// whichever slot it goes to. Six sets are within the limits of desktop drivers,
// which report eight or more. See docs/ADR/0013-vulkan-backend.md.
func (d *Device) createLayouts() error {
	frameBindings := []VkDescriptorSetLayoutBinding{
		{
			binding:         0,
			descriptorType:  VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER,
			descriptorCount: 1,
			stageFlags:      VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT,
		},
		{
			binding:            1,
			descriptorType:     VK_DESCRIPTOR_TYPE_SAMPLER,
			descriptorCount:    1,
			stageFlags:         VK_SHADER_STAGE_FRAGMENT_BIT,
			pImmutableSamplers: &d.sampler,
		},
	}
	textureBindings := []VkDescriptorSetLayoutBinding{{
		descriptorType:  VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE,
		descriptorCount: 1,
		stageFlags:      VK_SHADER_STAGE_FRAGMENT_BIT,
	}}
	create := func(bindings []VkDescriptorSetLayoutBinding, out *VkDescriptorSetLayout) error {
		info := VkDescriptorSetLayoutCreateInfo{
			sType:        VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO,
			bindingCount: uint32(len(bindings)),
			pBindings:    &bindings[0],
		}
		return failure(callResult(d.vkCreateDescriptorLayout, uintptr(d.device), ptr(&info), 0, ptr(out)), "create set layout")
	}
	if err := create(frameBindings, &d.frameLayout); err != nil {
		return err
	}
	if err := create(textureBindings, &d.textureLayout); err != nil {
		return err
	}
	d.layouts = [6]VkDescriptorSetLayout{d.frameLayout, d.textureLayout, d.textureLayout, d.textureLayout, d.textureLayout, d.textureLayout}
	// The scissor rectangle of SCISSOR_IN_SHADER. Every device declares it, as
	// the pixel shader does.
	scissor := VkPushConstantRange{stageFlags: VK_SHADER_STAGE_FRAGMENT_BIT, size: uint32(unsafe.Sizeof(d.scissorBounds))}
	info := VkPipelineLayoutCreateInfo{
		sType:                  VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO,
		setLayoutCount:         uint32(len(d.layouts)),
		pSetLayouts:            &d.layouts[0],
		pushConstantRangeCount: 1,
		pPushConstantRanges:    unsafe.Pointer(&scissor),
	}
	return failure(callResult(d.vkCreatePipelineLayout, uintptr(d.device), ptr(&info), 0, ptr(&d.pipeLayout)), "create pipeline layout")
}

func (d *Device) createFramePool() error {
	sizes := []VkDescriptorPoolSize{
		{VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, 2 * framesInFlight},
		{VK_DESCRIPTOR_TYPE_SAMPLER, 2 * framesInFlight},
	}
	info := VkDescriptorPoolCreateInfo{
		sType:         VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO,
		maxSets:       2 * framesInFlight,
		poolSizeCount: uint32(len(sizes)),
		pPoolSizes:    &sizes[0],
	}
	return failure(callResult(d.vkCreateDescriptorPool, uintptr(d.device), ptr(&info), 0, ptr(&d.framePool)), "create descriptor pool")
}

func (d *Device) createFrame(f *frame) error {
	fence := VkFenceCreateInfo{sType: VK_STRUCTURE_TYPE_FENCE_CREATE_INFO}
	if err := failure(callResult(d.vkCreateFence, uintptr(d.device), ptr(&fence), 0, ptr(&f.fence)), "create fence"); err != nil {
		return err
	}
	alloc := VkCommandBufferAllocateInfo{
		sType:              VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
		commandPool:        d.commandPool,
		level:              VK_COMMAND_BUFFER_LEVEL_PRIMARY,
		commandBufferCount: 1,
	}
	r := callResult(d.vkAllocateCommandBuffers, uintptr(d.device), ptr(&alloc), ptr(&f.commands))
	if err := failure(r, "allocate command buffer"); err != nil {
		return err
	}
	f.instances.usage = VK_BUFFER_USAGE_VERTEX_BUFFER_BIT
	f.vertices.usage = VK_BUFFER_USAGE_VERTEX_BUFFER_BIT
	f.staging.usage = VK_BUFFER_USAGE_TRANSFER_SRC_BIT
	return d.createConstants(&f.constants, &f.set)
}

// createConstants makes a uniform buffer and the descriptor set that refers to it.
func (d *Device) createConstants(b *buffer, out *VkDescriptorSet) error {
	var err error
	if *b, err = d.newBuffer(uint64(unsafe.Sizeof(frameConstants{})), VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT); err != nil {
		return err
	}
	set := VkDescriptorSetAllocateInfo{
		sType:              VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO,
		descriptorPool:     d.framePool,
		descriptorSetCount: 1,
		pSetLayouts:        &d.frameLayout,
	}
	r := callResult(d.vkAllocateDescriptorSets, uintptr(d.device), ptr(&set), ptr(out))
	if err := failure(r, "allocate descriptor set"); err != nil {
		return err
	}
	info := VkDescriptorBufferInfo{buffer: b.handle, rng: VK_WHOLE_SIZE}
	write := VkWriteDescriptorSet{
		sType:           VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET,
		dstSet:          *out,
		descriptorCount: 1,
		descriptorType:  VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER,
		pBufferInfo:     &info,
	}
	call(d.vkUpdateDescriptorSets, uintptr(d.device), 1, ptr(&write), 0, 0)
	return nil
}

// AdapterName returns the description of the GPU in use.
func (d *Device) AdapterName() string { return d.name }

// prepare makes the current frame accept commands. Uploads call it as well as
// BeginFrame, so that they land in the same command buffer ahead of the render
// pass.
func (d *Device) prepare() error {
	if d.recording {
		return nil
	}
	f := &d.frames[d.current]
	if f.submitted {
		if err := d.waitFor(f); err != nil {
			return err
		}
	}
	for _, release := range f.retired {
		release()
	}
	clear(f.retired)
	f.retired = f.retired[:0]
	f.instances.used, f.vertices.used, f.staging.used = 0, 0, 0
	f.targetUsed = 0

	r := d.hotResult(d.vkBeginCommandBuffer, uintptr(f.commands), ptr(&d.beginInfo))
	if err := failure(r, "begin command buffer"); err != nil {
		return err
	}
	d.recording = true
	if d.timer != nil {
		d.startTiming(f)
	}
	return nil
}

// waitFor blocks until the GPU has finished f.
func (d *Device) waitFor(f *frame) error {
	r := d.hotResult(d.vkWaitForFences, uintptr(d.device), 1, ptr(&f.fence), 1, uintptr(UINT64_MAX))
	if err := failure(r, "wait for fence"); err != nil {
		return err
	}
	f.submitted = false
	return failure(d.hotResult(d.vkResetFences, uintptr(d.device), 1, ptr(&f.fence)), "reset fence")
}

// retire runs release once the GPU cannot be using what it frees any more. The
// queue runs in order, so the fence of the newest frame covers all the others.
func (d *Device) retire(release func()) {
	newest := d.current
	if !d.recording {
		newest = (d.current + framesInFlight - 1) % framesInFlight
	}
	f := &d.frames[newest]
	f.retired = append(f.retired, release)
}

func (d *Device) waitIdle() error {
	return failure(callResult(d.vkDeviceWaitIdle, uintptr(d.device)), "wait for device")
}

// BeginFrame implements gpu.Backend.
func (d *Device) BeginFrame(s gpu.Surface, background geom.Color) error {
	sf := s.(*Surface)
	if sf.inFrame || d.drawing != nil {
		panic("gowaku: BeginFrame called twice without EndFrame")
	}
	if err := d.prepare(); err != nil {
		return err
	}
	if err := sf.acquire(); err != nil {
		return err
	}
	if err := sf.selectPass(); err != nil {
		return err
	}
	sf.inFrame = true
	d.drawing = sf

	f := &d.frames[d.current]
	// The fence of this frame has passed, so the GPU is not reading this.
	*(*frameConstants)(f.constants.mapped) = frameConstants{
		viewportWidth:  float32(sf.extent.width),
		viewportHeight: float32(sf.extent.height),
		scale:          sf.scale,
	}

	color := [4]float32{background.R, background.G, background.B, background.A}
	d.clears[0] = *(*VkClearValue)(unsafe.Pointer(&color))
	d.clears[1] = VkClearValue{}
	d.passInfo = VkRenderPassBeginInfo{
		sType:           VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO,
		renderPass:      d.pass.renderPass,
		framebuffer:     sf.framebuffers[sf.passIndex][sf.imageIndex],
		renderArea:      VkRect2D{extent: sf.extent},
		clearValueCount: uint32(len(d.clears)),
		pClearValues:    &d.clears[0],
	}
	d.hot(d.vkCmdBeginRenderPass, uintptr(f.commands), ptr(&d.passInfo), VK_SUBPASS_CONTENTS_INLINE)
	sf.rendering = true

	d.bound, d.boundLevel = 0, -1
	d.surfaceSize = VkViewport{width: float32(sf.extent.width), height: float32(sf.extent.height), maxDepth: 1}
	d.lastScissor = VkRect2D{}
	d.textureSet = [5]VkDescriptorSet{d.blank.set, d.blank.set, d.blank.set, d.blank.set, d.blank.set}
	d.hot(d.vkCmdBindDescriptorSets, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 0, 1, ptr(&f.set), 0, 0)
	d.hot(d.vkCmdBindDescriptorSets, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 1, 5, ptr(&d.textureSet[0]), 0, 0)
	return nil
}

// Draw implements gpu.Backend.
func (d *Device) Draw(s gpu.Surface, batch *gpu.Batch) error {
	sf := s.(*Surface)
	if !sf.rendering {
		panic("gowaku: Draw called outside BeginFrame and EndFrame")
	}
	return d.drawBatch(sf, batch)
}

// drawBatch records batch into the render pass in progress.
func (d *Device) drawBatch(sf *Surface, batch *gpu.Batch) error {
	n := len(batch.Instances)
	if n == 0 {
		return nil
	}
	f := &d.frames[d.current]
	// Each blur adds the instances of its passes after those of the batch.
	blurs := batch.CountBlurs()
	total := n + 3*blurs
	offset, target, err := d.reserve(&f.instances, uint64(total)*instanceStride)
	if err != nil {
		return err
	}
	// target is host-visible memory with room for total instances.
	instances := unsafe.Slice((*gpu.Instance)(target), total)
	copy(instances, batch.Instances)
	if blurs > 0 {
		d.planBlurs(sf, batch, instances[n:])
	}
	d.bindBuffers[0], d.bindOffsets[0] = f.instances.buffer.handle, offset
	bindings := uintptr(1)
	if v := len(batch.Vertices); v > 0 {
		offset, target, err := d.reserve(&f.vertices, uint64(v)*vertexStride)
		if err != nil {
			return err
		}
		// target is host-visible memory with room for v vertices.
		copy(unsafe.Slice((*geom.Offset)(target), v), batch.Vertices)
		d.bindBuffers[1], d.bindOffsets[1] = f.vertices.buffer.handle, offset
		bindings = 2
	}
	commands := uintptr(f.commands)
	d.hot(d.vkCmdBindVertexBuffers, commands, 0, bindings, ptr(&d.bindBuffers[0]), ptr(&d.bindOffsets[0]))
	if t, ok := batch.Ramps.(*Texture); ok && t.set != d.textureSet[2] {
		d.textureSet[2] = t.set
		d.hot(d.vkCmdBindDescriptorSets, commands, VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 3, 1, ptr(&d.textureSet[2]), 0, 0)
	}
	blur := 0
	for i := range batch.Draws {
		c := &batch.Draws[i]
		if c.Count == 0 {
			continue
		}
		var shader *customShader
		if c.Shader != nil {
			if shader = d.shaders[c.Shader]; shader == nil {
				// Not prepared, or refused: render reports it.
				continue
			}
		}
		if c.Blur > 0 {
			drawn, err := d.drawBlur(f, sf, c, &d.blurs[blur], n+3*blur)
			blur++
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		if c.Blend != gpu.BlendSrcOver {
			drawn, err := d.prepareBlend(f, sf, c)
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		var custom VkPipeline
		if shader != nil {
			if custom, err = d.shaderPipeline(shader, c.Blend); err != nil {
				return err
			}
		}
		d.setScissor(sf, c)
		d.bindTextures(f, c)
		d.draw(f, c, custom)
	}
	return nil
}

// setScissor limits drawing to the clip of call. An unclipped call gets the
// whole surface.
func (d *Device) setScissor(sf *Surface, c *gpu.DrawCall) {
	width, height := int32(sf.extent.width), int32(sf.extent.height)
	left, top, right, bottom := int32(0), int32(0), width, height
	if c.Clipped {
		left, top, right, bottom = gpu.PhysicalBounds(c.Clip, sf.scale)
		left, top = min(max(left, 0), width), min(max(top, 0), height)
		right, bottom = min(max(right, left), width), min(max(bottom, top), height)
	}
	if l := sf.limit; !l.Empty() {
		// The area of the render pass of a multisampled target.
		left, top = max(left, int32(l.Min.X)), max(top, int32(l.Min.Y))
		right, bottom = max(min(right, int32(l.Max.X)), left), max(min(bottom, int32(l.Max.Y)), top)
	}
	d.scissor = VkRect2D{VkOffset2D{left, top}, VkExtent2D{uint32(right - left), uint32(bottom - top)}}
	if d.scissor != d.lastScissor {
		d.applyScissor(uintptr(d.frames[d.current].commands))
	}
}

// applyScissor sets d.scissor, and hands it to the pixel shader on a device
// that tests it there too.
func (d *Device) applyScissor(commands uintptr) {
	d.lastScissor = d.scissor
	d.hot(d.vkCmdSetScissor, commands, 0, 1, ptr(&d.scissor))
	if d.scissorInShader {
		s := d.scissor
		d.scissorBounds = [4]int32{s.offset.x, s.offset.y, s.offset.x + int32(s.extent.width), s.offset.y + int32(s.extent.height)}
		d.hot(d.vkCmdPushConstants, commands, uintptr(d.pipeLayout), VK_SHADER_STAGE_FRAGMENT_BIT, 0,
			unsafe.Sizeof(d.scissorBounds), ptr(&d.scissorBounds))
	}
}

// bindTextures binds the textures of c, unless they are already bound.
func (d *Device) bindTextures(f *frame, c *gpu.DrawCall) {
	if t, ok := c.Texture.(*Texture); ok && t.set != d.textureSet[0] {
		d.textureSet[0] = t.set
		d.hot(d.vkCmdBindDescriptorSets, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 1, 1, ptr(&d.textureSet[0]), 0, 0)
	}
	if t, ok := c.Image.(*Texture); ok && t.set != d.textureSet[1] {
		d.textureSet[1] = t.set
		d.hot(d.vkCmdBindDescriptorSets, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(d.pipeLayout), 2, 1, ptr(&d.textureSet[1]), 0, 0)
	}
}

// EndFrame implements gpu.Backend.
func (d *Device) EndFrame(s gpu.Surface) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: EndFrame called without BeginFrame")
	}
	if err := d.submit(sf); err != nil {
		return err
	}
	sf.inFrame = false
	d.drawing = nil
	return sf.present()
}

// submit ends the commands of the frame on sf and sends them to the GPU. It does
// nothing the second time, for a frame that ReadPixels has already sent.
func (d *Device) submit(sf *Surface) error {
	if !d.recording {
		return nil
	}
	f := &d.frames[d.current]
	if sf.rendering {
		d.hot(d.vkCmdEndRenderPass, uintptr(f.commands))
		sf.rendering = false
	}
	if d.timer != nil {
		d.stopTiming(f)
	}
	if err := failure(d.hotResult(d.vkEndCommandBuffer, uintptr(f.commands)), "end command buffer"); err != nil {
		return err
	}
	d.submitInfo = VkSubmitInfo{
		sType:                VK_STRUCTURE_TYPE_SUBMIT_INFO,
		waitSemaphoreCount:   1,
		pWaitSemaphores:      &sf.acquired[sf.acquiredIndex],
		pWaitDstStageMask:    &d.waitStage,
		commandBufferCount:   1,
		pCommandBuffers:      &f.commands,
		signalSemaphoreCount: 1,
		pSignalSemaphores:    &sf.rendered[sf.imageIndex],
	}
	r := d.hotResult(d.vkQueueSubmit, uintptr(d.queue), 1, ptr(&d.submitInfo), uintptr(f.fence))
	if err := failure(r, "submit"); err != nil {
		return err
	}
	f.submitted = true
	d.recording = false
	d.current = (d.current + 1) % framesInFlight
	return nil
}

// Destroy implements gpu.Backend.
func (d *Device) Destroy() {
	if d.liveSurfaces != 0 {
		panic("gowaku: Device.Destroy called while surfaces are alive")
	}
	d.release()
}

// release frees everything in reverse order of creation. It also serves a partly
// initialized Device.
func (d *Device) release() {
	if d.device != 0 && d.vkDeviceWaitIdle != 0 {
		// A device that cannot go idle is going away anyway.
		_ = callResult(d.vkDeviceWaitIdle, uintptr(d.device)) //nolint:errcheck // see above
		d.releaseDevice()
	}
	if d.device != 0 && d.vkDestroyDevice != 0 {
		call(d.vkDestroyDevice, uintptr(d.device), 0)
	}
	if d.instance != 0 && d.vkDestroyInstance != 0 {
		call(d.vkDestroyInstance, uintptr(d.instance), 0)
	}
	*d = Device{}
}

func (d *Device) releaseDevice() {
	dev := uintptr(d.device)
	if d.blank != nil {
		d.blank.Destroy()
	}
	for _, t := range []*Texture{d.backdrop, d.blurA, d.blurB} {
		if t != nil {
			t.free()
		}
	}
	for i := range d.frames {
		f := &d.frames[i]
		for _, release := range f.retired {
			release()
		}
		d.freeBuffer(&f.instances.buffer)
		d.freeBuffer(&f.vertices.buffer)
		d.freeBuffer(&f.staging.buffer)
		d.freeBuffer(&f.constants)
		for k := range f.targets {
			d.freeBuffer(&f.targets[k].buffer)
		}
		if f.fence != 0 {
			call(d.vkDestroyFence, dev, uintptr(f.fence), 0)
		}
	}
	destroy := func(fn uintptr, handle uint64) {
		if handle != 0 {
			call(fn, dev, uintptr(handle), 0)
		}
	}
	destroy(d.vkDestroyRenderPass, uint64(d.targetPass))
	destroy(d.vkDestroyRenderPass, uint64(d.multi.pass))
	d.freeTarget(&d.multi.color)
	d.freeTarget(&d.multi.depth)
	destroy(d.vkDestroyImageView, uint64(d.targetDepth.view))
	destroy(d.vkDestroyImage, uint64(d.targetDepth.image))
	if d.targetDepth.memory != 0 {
		call(d.vkFreeMemory, dev, uintptr(d.targetDepth.memory), 0)
	}
	for _, sh := range d.shaders {
		for i := range sh.pipelines {
			for _, p := range sh.pipelines[i] {
				destroy(d.vkDestroyPipeline, uint64(p))
			}
		}
		destroy(d.vkDestroyShaderModule, uint64(sh.module))
	}
	for i := range d.passes {
		pass := &d.passes[i]
		for _, p := range pass.pipelines {
			destroy(d.vkDestroyPipeline, uint64(p))
		}
		for i := range pass.blended {
			for _, p := range pass.blended[i] {
				destroy(d.vkDestroyPipeline, uint64(p))
			}
		}
		destroy(d.vkDestroyRenderPass, uint64(pass.resume))
		destroy(d.vkDestroyRenderPass, uint64(pass.renderPass))
	}
	if d.timer != nil {
		destroy(d.vkDestroyQueryPool, uint64(d.timer.pool))
	}
	destroy(d.vkDestroyCommandPool, uint64(d.commandPool))
	destroy(d.vkDestroyDescriptorPool, uint64(d.framePool))
	for _, p := range d.texturePools {
		destroy(d.vkDestroyDescriptorPool, uint64(p))
	}
	for _, p := range d.constantPools {
		destroy(d.vkDestroyDescriptorPool, uint64(p))
	}
	destroy(d.vkDestroyPipelineLayout, uint64(d.pipeLayout))
	destroy(d.vkDestroyDescriptorLayout, uint64(d.textureLayout))
	destroy(d.vkDestroyDescriptorLayout, uint64(d.frameLayout))
	destroy(d.vkDestroySampler, uint64(d.sampler))
	destroy(d.vkDestroyShaderModule, uint64(d.pathVertex))
	destroy(d.vkDestroyShaderModule, uint64(d.sdfFragment))
	destroy(d.vkDestroyShaderModule, uint64(d.sdfVertex))
}

// createSurface calls the vkCreate*SurfaceKHR of entry kind of surfaceKinds.
func (d *Device) createSurface(kind int, info uintptr, surface *VkSurfaceKHR) (VkResult, error) {
	fn := d.vkCreateSurface[kind]
	if fn == 0 {
		return 0, fmt.Errorf("vulkan: the driver lacks %s", surfaceKinds[kind].extension)
	}
	return callResult(fn, uintptr(d.instance), info, 0, ptr(surface)), nil
}

// supportedFormats returns the surface formats gowaku can draw to, best first.
// None of them is sRGB, so that blending happens on the stored values
// (docs/ADR/0005-sdf-pipeline.md).
func supportedFormats(available []VkSurfaceFormatKHR) (VkSurfaceFormatKHR, bool) {
	for _, want := range []uint32{VK_FORMAT_B8G8R8A8_UNORM, VK_FORMAT_R8G8B8A8_UNORM} {
		i := slices.IndexFunc(available, func(f VkSurfaceFormatKHR) bool {
			return f.format == want && f.colorSpace == VK_COLOR_SPACE_SRGB_NONLINEAR_KHR
		})
		if i >= 0 {
			return available[i], true
		}
	}
	return VkSurfaceFormatKHR{}, false
}
