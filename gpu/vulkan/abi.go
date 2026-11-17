//go:build linux || windows

package vulkan

import "unsafe"

// Handles. Dispatchable handles are pointers and the rest are 64-bit values,
// which are the same size on the 64-bit systems gowaku runs on.
type (
	VkInstance            uintptr
	VkPhysicalDevice      uintptr
	VkDevice              uintptr
	VkQueue               uintptr
	VkCommandBuffer       uintptr
	VkSurfaceKHR          uint64
	VkSwapchainKHR        uint64
	VkSemaphore           uint64
	VkFence               uint64
	VkDeviceMemory        uint64
	VkBuffer              uint64
	VkImage               uint64
	VkImageView           uint64
	VkSampler             uint64
	VkShaderModule        uint64
	VkRenderPass          uint64
	VkFramebuffer         uint64
	VkPipeline            uint64
	VkPipelineCache       uint64
	VkPipelineLayout      uint64
	VkDescriptorSet       uint64
	VkDescriptorPool      uint64
	VkDescriptorSetLayout uint64
	VkCommandPool         uint64
	VkQueryPool           uint64
)

// VkResult is the status of a call. Errors are negative.
type VkResult int32

// The results gowaku tells apart.
const (
	VK_SUCCESS                     VkResult = 0
	VK_INCOMPLETE                  VkResult = 5
	VK_SUBOPTIMAL_KHR              VkResult = 1000001003
	VK_ERROR_DEVICE_LOST           VkResult = -4
	VK_ERROR_OUT_OF_POOL_MEMORY    VkResult = -1000069000
	VK_ERROR_FRAGMENTED_POOL       VkResult = -12
	VK_ERROR_OUT_OF_DATE_KHR       VkResult = -1000001004
	VK_ERROR_SURFACE_LOST_KHR      VkResult = -1000000000
	VK_ERROR_INCOMPATIBLE_DRIVER   VkResult = -9
	VK_ERROR_EXTENSION_NOT_PRESENT VkResult = -7
)

// VkStructureType values.
const (
	VK_STRUCTURE_TYPE_APPLICATION_INFO                          = 0
	VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO                      = 1
	VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO                  = 2
	VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO                        = 3
	VK_STRUCTURE_TYPE_SUBMIT_INFO                               = 4
	VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO                      = 5
	VK_STRUCTURE_TYPE_FENCE_CREATE_INFO                         = 8
	VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO                    = 11
	VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO                     = 9
	VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO                        = 12
	VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO                         = 14
	VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO                    = 15
	VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO                 = 16
	VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO         = 18
	VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO   = 19
	VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO = 20
	VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO       = 22
	VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO  = 23
	VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO    = 24
	VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO  = 25
	VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO    = 26
	VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO        = 27
	VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO             = 28
	VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO               = 30
	VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO                       = 31
	VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO         = 32
	VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO               = 33
	VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO              = 34
	VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET                      = 35
	VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO                   = 37
	VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO                   = 38
	VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO                  = 39
	VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO              = 40
	VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO                 = 42
	VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO                    = 43
	VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER                      = 45
	VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR                 = 1000001000
	VK_STRUCTURE_TYPE_PRESENT_INFO_KHR                          = 1000001001
	VK_STRUCTURE_TYPE_XLIB_SURFACE_CREATE_INFO_KHR              = 1000004000
	VK_STRUCTURE_TYPE_WAYLAND_SURFACE_CREATE_INFO_KHR           = 1000006000
	VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR             = 1000009000
)

// Enumerants and flags.
const (
	VK_API_VERSION_1_0 = 1 << 22

	VK_PHYSICAL_DEVICE_TYPE_OTHER          = 0
	VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU = 1
	VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU   = 2
	VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU    = 3
	VK_PHYSICAL_DEVICE_TYPE_CPU            = 4

	// VK_VENDOR_ID_MESA is the vendor of Mesa's software devices, lavapipe
	// among them.
	VK_VENDOR_ID_MESA = 0x10005

	VK_QUEUE_GRAPHICS_BIT = 0x1

	VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT  = 0x1
	VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT  = 0x2
	VK_MEMORY_PROPERTY_HOST_COHERENT_BIT = 0x4

	VK_FORMAT_R8_UNORM            = 9
	VK_FORMAT_R8G8B8A8_UNORM      = 37
	VK_FORMAT_B8G8R8A8_UNORM      = 44
	VK_FORMAT_R32G32_UINT         = 101
	VK_FORMAT_R32G32_SFLOAT       = 103
	VK_FORMAT_R32G32B32A32_SFLOAT = 109
	VK_FORMAT_D24_UNORM_S8_UINT   = 129
	VK_FORMAT_D32_SFLOAT_S8_UINT  = 130

	VK_FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT = 0x200

	VK_COLOR_SPACE_SRGB_NONLINEAR_KHR = 0

	VK_BUFFER_USAGE_TRANSFER_SRC_BIT   = 0x1
	VK_BUFFER_USAGE_TRANSFER_DST_BIT   = 0x2
	VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT = 0x10
	VK_BUFFER_USAGE_VERTEX_BUFFER_BIT  = 0x80

	VK_IMAGE_USAGE_TRANSFER_SRC_BIT             = 0x1
	VK_IMAGE_USAGE_TRANSFER_DST_BIT             = 0x2
	VK_IMAGE_USAGE_SAMPLED_BIT                  = 0x4
	VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT         = 0x10
	VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT = 0x20

	VK_IMAGE_TYPE_2D        = 1
	VK_IMAGE_VIEW_TYPE_2D   = 1
	VK_IMAGE_TILING_OPTIMAL = 0

	VK_IMAGE_ASPECT_COLOR_BIT   = 0x1
	VK_IMAGE_ASPECT_DEPTH_BIT   = 0x2
	VK_IMAGE_ASPECT_STENCIL_BIT = 0x4

	VK_IMAGE_LAYOUT_UNDEFINED                        = 0
	VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL         = 2
	VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL = 3
	VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL         = 5
	VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL             = 6
	VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL             = 7
	VK_IMAGE_LAYOUT_PRESENT_SRC_KHR                  = 1000001002

	VK_SAMPLE_COUNT_1_BIT = 0x1

	VK_ATTACHMENT_LOAD_OP_LOAD       = 0
	VK_ATTACHMENT_LOAD_OP_CLEAR      = 1
	VK_ATTACHMENT_LOAD_OP_DONT_CARE  = 2
	VK_ATTACHMENT_STORE_OP_STORE     = 0
	VK_ATTACHMENT_STORE_OP_DONT_CARE = 1

	VK_PIPELINE_BIND_POINT_GRAPHICS = 0
	VK_SUBPASS_EXTERNAL             = 0xFFFFFFFF

	VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT             = 0x1
	VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT         = 0x80
	VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT    = 0x100
	VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT     = 0x200
	VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT = 0x400
	VK_PIPELINE_STAGE_TRANSFER_BIT                = 0x1000
	VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT          = 0x2000

	VK_ACCESS_SHADER_READ_BIT                    = 0x20
	VK_ACCESS_COLOR_ATTACHMENT_READ_BIT          = 0x80
	VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT         = 0x100
	VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT  = 0x200
	VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT = 0x400
	VK_ACCESS_TRANSFER_READ_BIT                  = 0x800
	VK_ACCESS_TRANSFER_WRITE_BIT                 = 0x1000

	VK_SHADER_STAGE_VERTEX_BIT   = 0x1
	VK_SHADER_STAGE_FRAGMENT_BIT = 0x10

	VK_DESCRIPTOR_TYPE_SAMPLER        = 0
	VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE  = 2
	VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER = 6

	VK_DESCRIPTOR_POOL_CREATE_FREE_DESCRIPTOR_SET_BIT = 0x1

	VK_FILTER_LINEAR                      = 1
	VK_SAMPLER_MIPMAP_MODE_LINEAR         = 1
	VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE = 2

	VK_VERTEX_INPUT_RATE_VERTEX   = 0
	VK_VERTEX_INPUT_RATE_INSTANCE = 1

	VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST  = 3
	VK_PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP = 4

	VK_POLYGON_MODE_FILL    = 0
	VK_CULL_MODE_NONE       = 0
	VK_FRONT_FACE_CLOCKWISE = 1

	VK_COMPARE_OP_LESS      = 1
	VK_COMPARE_OP_EQUAL     = 2
	VK_COMPARE_OP_GREATER   = 4
	VK_COMPARE_OP_NOT_EQUAL = 5
	VK_COMPARE_OP_ALWAYS    = 7

	VK_STENCIL_OP_KEEP               = 0
	VK_STENCIL_OP_ZERO               = 1
	VK_STENCIL_OP_INVERT             = 5
	VK_STENCIL_OP_INCREMENT_AND_WRAP = 6
	VK_STENCIL_OP_DECREMENT_AND_WRAP = 7

	VK_BLEND_FACTOR_ZERO                = 0
	VK_BLEND_FACTOR_ONE                 = 1
	VK_BLEND_FACTOR_SRC_COLOR           = 2
	VK_BLEND_FACTOR_ONE_MINUS_SRC_COLOR = 3
	VK_BLEND_FACTOR_SRC_ALPHA           = 6
	VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA = 7
	VK_BLEND_FACTOR_DST_ALPHA           = 8
	VK_BLEND_FACTOR_ONE_MINUS_DST_ALPHA = 9
	VK_BLEND_OP_ADD                     = 0

	VK_COLOR_COMPONENT_ALL = 0xF

	VK_DYNAMIC_STATE_VIEWPORT = 0
	VK_DYNAMIC_STATE_SCISSOR  = 1

	VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT = 0x2
	VK_COMMAND_BUFFER_LEVEL_PRIMARY                 = 0
	VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT     = 0x1
	VK_SUBPASS_CONTENTS_INLINE                      = 0

	VK_SHARING_MODE_EXCLUSIVE = 0

	VK_PRESENT_MODE_IMMEDIATE_KHR = 0
	VK_PRESENT_MODE_MAILBOX_KHR   = 1
	VK_PRESENT_MODE_FIFO_KHR      = 2

	VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR          = 0x1
	VK_COMPOSITE_ALPHA_PRE_MULTIPLIED_BIT_KHR  = 0x2
	VK_COMPOSITE_ALPHA_POST_MULTIPLIED_BIT_KHR = 0x4
	VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR         = 0x8

	VK_QUEUE_FAMILY_IGNORED = 0xFFFFFFFF
	VK_REMAINING_MIP_LEVELS = 0xFFFFFFFF
	VK_WHOLE_SIZE           = ^uint64(0)
	UINT64_MAX              = ^uint64(0)
)

// Offsets into VkPhysicalDeviceProperties, whose VkPhysicalDeviceLimits has
// more than a hundred fields of which gowaku reads three.
const (
	offsetVendorID            = 8
	offsetDeviceType          = 16
	offsetDeviceName          = 20
	offsetColorSampleCounts   = 296 + 376
	offsetDepthSampleCounts   = 296 + 380
	offsetStencilSampleCounts = 296 + 384
	sizeofDeviceProperties    = 824
	deviceNameSize            = 256
)

// The offsets of timing in VkPhysicalDeviceProperties.
const (
	offsetTimestampComputeAndGraphics = 296 + 420
	offsetTimestampPeriod             = 296 + 424
)

// The query type and result flags of GPU timing.
const (
	VK_QUERY_TYPE_TIMESTAMP  = 2
	VK_QUERY_RESULT_64_BIT   = 0x1
	VK_QUERY_RESULT_WAIT_BIT = 0x2
)

// VkQueryPoolCreateInfo is the struct of the same name.
type VkQueryPoolCreateInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	flags              uint32
	queryType          uint32
	queryCount         uint32
	pipelineStatistics uint32
}

// VkPhysicalDeviceProperties is read through the offsets above.
type VkPhysicalDeviceProperties struct {
	bytes [sizeofDeviceProperties / 8]uint64
}

func (p *VkPhysicalDeviceProperties) u32(offset uintptr) uint32 {
	// offset lies inside bytes, which keeps the alignment of a uint64.
	return *(*uint32)(unsafe.Add(unsafe.Pointer(&p.bytes), offset))
}

func (p *VkPhysicalDeviceProperties) f32(offset uintptr) float32 {
	// offset lies inside bytes, which keeps the alignment of a uint64.
	return *(*float32)(unsafe.Add(unsafe.Pointer(&p.bytes), offset))
}

func (p *VkPhysicalDeviceProperties) name() string {
	// The name is a NUL-terminated array inside bytes.
	b := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(&p.bytes), offsetDeviceName)), deviceNameSize)
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// The structures below keep the names of vulkan_core.h. Pointers inside them
// are Go pointers, so that the garbage collector sees what a call reads.

type VkApplicationInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	pApplicationName   *byte
	applicationVersion uint32
	pEngineName        *byte
	engineVersion      uint32
	apiVersion         uint32
}

type VkInstanceCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	pApplicationInfo        *VkApplicationInfo
	enabledLayerCount       uint32
	ppEnabledLayerNames     **byte
	enabledExtensionCount   uint32
	ppEnabledExtensionNames **byte
}

type VkDeviceQueueCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	queueFamilyIndex uint32
	queueCount       uint32
	pQueuePriorities *float32
}

type VkDeviceCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	queueCreateInfoCount    uint32
	pQueueCreateInfos       *VkDeviceQueueCreateInfo
	enabledLayerCount       uint32
	ppEnabledLayerNames     **byte
	enabledExtensionCount   uint32
	ppEnabledExtensionNames **byte
	pEnabledFeatures        unsafe.Pointer
}

type VkExtent2D struct{ width, height uint32 }

type VkExtent3D struct{ width, height, depth uint32 }

type VkOffset2D struct{ x, y int32 }

type VkOffset3D struct{ x, y, z int32 }

type VkRect2D struct {
	offset VkOffset2D
	extent VkExtent2D
}

type VkViewport struct {
	x, y, width, height, minDepth, maxDepth float32
}

type VkQueueFamilyProperties struct {
	queueFlags                  uint32
	queueCount                  uint32
	timestampValidBits          uint32
	minImageTransferGranularity VkExtent3D
}

type VkExtensionProperties struct {
	extensionName [256]byte
	specVersion   uint32
}

type VkFormatProperties struct {
	linearTilingFeatures  uint32
	optimalTilingFeatures uint32
	bufferFeatures        uint32
}

type VkMemoryType struct {
	propertyFlags uint32
	heapIndex     uint32
}

type VkMemoryHeap struct {
	size  uint64
	flags uint32
}

type VkPhysicalDeviceMemoryProperties struct {
	memoryTypeCount uint32
	memoryTypes     [32]VkMemoryType
	memoryHeapCount uint32
	memoryHeaps     [16]VkMemoryHeap
}

type VkMemoryRequirements struct {
	size           uint64
	alignment      uint64
	memoryTypeBits uint32
}

type VkMemoryAllocateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	allocationSize  uint64
	memoryTypeIndex uint32
}

type VkBufferCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	size                  uint64
	usage                 uint32
	sharingMode           uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
}

type VkImageCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	imageType             uint32
	format                uint32
	extent                VkExtent3D
	mipLevels             uint32
	arrayLayers           uint32
	samples               uint32
	tiling                uint32
	usage                 uint32
	sharingMode           uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
	initialLayout         uint32
}

type VkComponentMapping struct{ r, g, b, a uint32 }

type VkImageSubresourceRange struct {
	aspectMask     uint32
	baseMipLevel   uint32
	levelCount     uint32
	baseArrayLayer uint32
	layerCount     uint32
}

type VkImageSubresourceLayers struct {
	aspectMask     uint32
	mipLevel       uint32
	baseArrayLayer uint32
	layerCount     uint32
}

type VkImageViewCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	image            VkImage
	viewType         uint32
	format           uint32
	components       VkComponentMapping
	subresourceRange VkImageSubresourceRange
}

type VkSamplerCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	magFilter               uint32
	minFilter               uint32
	mipmapMode              uint32
	addressModeU            uint32
	addressModeV            uint32
	addressModeW            uint32
	mipLodBias              float32
	anisotropyEnable        uint32
	maxAnisotropy           float32
	compareEnable           uint32
	compareOp               uint32
	minLod                  float32
	maxLod                  float32
	borderColor             uint32
	unnormalizedCoordinates uint32
}

type VkShaderModuleCreateInfo struct {
	sType    uint32
	pNext    unsafe.Pointer
	flags    uint32
	codeSize uintptr
	pCode    *byte
}

type VkDescriptorSetLayoutBinding struct {
	binding            uint32
	descriptorType     uint32
	descriptorCount    uint32
	stageFlags         uint32
	pImmutableSamplers *VkSampler
}

type VkDescriptorSetLayoutCreateInfo struct {
	sType        uint32
	pNext        unsafe.Pointer
	flags        uint32
	bindingCount uint32
	pBindings    *VkDescriptorSetLayoutBinding
}

type VkPipelineLayoutCreateInfo struct {
	sType                  uint32
	pNext                  unsafe.Pointer
	flags                  uint32
	setLayoutCount         uint32
	pSetLayouts            *VkDescriptorSetLayout
	pushConstantRangeCount uint32
	pPushConstantRanges    unsafe.Pointer
}

type VkPushConstantRange struct {
	stageFlags uint32
	offset     uint32
	size       uint32
}

type VkSpecializationMapEntry struct {
	constantID uint32
	offset     uint32
	size       uintptr
}

type VkSpecializationInfo struct {
	mapEntryCount uint32
	pMapEntries   *VkSpecializationMapEntry
	dataSize      uintptr
	pData         unsafe.Pointer
}

type VkDescriptorPoolSize struct {
	typ             uint32
	descriptorCount uint32
}

type VkDescriptorPoolCreateInfo struct {
	sType         uint32
	pNext         unsafe.Pointer
	flags         uint32
	maxSets       uint32
	poolSizeCount uint32
	pPoolSizes    *VkDescriptorPoolSize
}

type VkDescriptorSetAllocateInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	descriptorPool     VkDescriptorPool
	descriptorSetCount uint32
	pSetLayouts        *VkDescriptorSetLayout
}

type VkDescriptorBufferInfo struct {
	buffer VkBuffer
	offset uint64
	rng    uint64
}

type VkDescriptorImageInfo struct {
	sampler     VkSampler
	imageView   VkImageView
	imageLayout uint32
}

type VkWriteDescriptorSet struct {
	sType            uint32
	pNext            unsafe.Pointer
	dstSet           VkDescriptorSet
	dstBinding       uint32
	dstArrayElement  uint32
	descriptorCount  uint32
	descriptorType   uint32
	pImageInfo       *VkDescriptorImageInfo
	pBufferInfo      *VkDescriptorBufferInfo
	pTexelBufferView unsafe.Pointer
}

type VkAttachmentDescription struct {
	flags          uint32
	format         uint32
	samples        uint32
	loadOp         uint32
	storeOp        uint32
	stencilLoadOp  uint32
	stencilStoreOp uint32
	initialLayout  uint32
	finalLayout    uint32
}

type VkAttachmentReference struct {
	attachment uint32
	layout     uint32
}

type VkSubpassDescription struct {
	flags                   uint32
	pipelineBindPoint       uint32
	inputAttachmentCount    uint32
	pInputAttachments       *VkAttachmentReference
	colorAttachmentCount    uint32
	pColorAttachments       *VkAttachmentReference
	pResolveAttachments     *VkAttachmentReference
	pDepthStencilAttachment *VkAttachmentReference
	preserveAttachmentCount uint32
	pPreserveAttachments    *uint32
}

type VkSubpassDependency struct {
	srcSubpass      uint32
	dstSubpass      uint32
	srcStageMask    uint32
	dstStageMask    uint32
	srcAccessMask   uint32
	dstAccessMask   uint32
	dependencyFlags uint32
}

type VkRenderPassCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	attachmentCount uint32
	pAttachments    *VkAttachmentDescription
	subpassCount    uint32
	pSubpasses      *VkSubpassDescription
	dependencyCount uint32
	pDependencies   *VkSubpassDependency
}

type VkFramebufferCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	renderPass      VkRenderPass
	attachmentCount uint32
	pAttachments    *VkImageView
	width           uint32
	height          uint32
	layers          uint32
}

type VkPipelineShaderStageCreateInfo struct {
	sType               uint32
	pNext               unsafe.Pointer
	flags               uint32
	stage               uint32
	module              VkShaderModule
	pName               *byte
	pSpecializationInfo unsafe.Pointer
}

type VkVertexInputBindingDescription struct {
	binding   uint32
	stride    uint32
	inputRate uint32
}

type VkVertexInputAttributeDescription struct {
	location uint32
	binding  uint32
	format   uint32
	offset   uint32
}

type VkPipelineVertexInputStateCreateInfo struct {
	sType                           uint32
	pNext                           unsafe.Pointer
	flags                           uint32
	vertexBindingDescriptionCount   uint32
	pVertexBindingDescriptions      *VkVertexInputBindingDescription
	vertexAttributeDescriptionCount uint32
	pVertexAttributeDescriptions    *VkVertexInputAttributeDescription
}

type VkPipelineInputAssemblyStateCreateInfo struct {
	sType                  uint32
	pNext                  unsafe.Pointer
	flags                  uint32
	topology               uint32
	primitiveRestartEnable uint32
}

type VkPipelineViewportStateCreateInfo struct {
	sType         uint32
	pNext         unsafe.Pointer
	flags         uint32
	viewportCount uint32
	pViewports    *VkViewport
	scissorCount  uint32
	pScissors     *VkRect2D
}

type VkPipelineRasterizationStateCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	depthClampEnable        uint32
	rasterizerDiscardEnable uint32
	polygonMode             uint32
	cullMode                uint32
	frontFace               uint32
	depthBiasEnable         uint32
	depthBiasConstantFactor float32
	depthBiasClamp          float32
	depthBiasSlopeFactor    float32
	lineWidth               float32
}

type VkPipelineMultisampleStateCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	rasterizationSamples  uint32
	sampleShadingEnable   uint32
	minSampleShading      float32
	pSampleMask           *uint32
	alphaToCoverageEnable uint32
	alphaToOneEnable      uint32
}

type VkStencilOpState struct {
	failOp      uint32
	passOp      uint32
	depthFailOp uint32
	compareOp   uint32
	compareMask uint32
	writeMask   uint32
	reference   uint32
}

type VkPipelineDepthStencilStateCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	depthTestEnable       uint32
	depthWriteEnable      uint32
	depthCompareOp        uint32
	depthBoundsTestEnable uint32
	stencilTestEnable     uint32
	front                 VkStencilOpState
	back                  VkStencilOpState
	minDepthBounds        float32
	maxDepthBounds        float32
}

type VkPipelineColorBlendAttachmentState struct {
	blendEnable         uint32
	srcColorBlendFactor uint32
	dstColorBlendFactor uint32
	colorBlendOp        uint32
	srcAlphaBlendFactor uint32
	dstAlphaBlendFactor uint32
	alphaBlendOp        uint32
	colorWriteMask      uint32
}

type VkPipelineColorBlendStateCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	logicOpEnable   uint32
	logicOp         uint32
	attachmentCount uint32
	pAttachments    *VkPipelineColorBlendAttachmentState
	blendConstants  [4]float32
}

type VkPipelineDynamicStateCreateInfo struct {
	sType             uint32
	pNext             unsafe.Pointer
	flags             uint32
	dynamicStateCount uint32
	pDynamicStates    *uint32
}

type VkGraphicsPipelineCreateInfo struct {
	sType               uint32
	pNext               unsafe.Pointer
	flags               uint32
	stageCount          uint32
	pStages             *VkPipelineShaderStageCreateInfo
	pVertexInputState   *VkPipelineVertexInputStateCreateInfo
	pInputAssemblyState *VkPipelineInputAssemblyStateCreateInfo
	pTessellationState  unsafe.Pointer
	pViewportState      *VkPipelineViewportStateCreateInfo
	pRasterizationState *VkPipelineRasterizationStateCreateInfo
	pMultisampleState   *VkPipelineMultisampleStateCreateInfo
	pDepthStencilState  *VkPipelineDepthStencilStateCreateInfo
	pColorBlendState    *VkPipelineColorBlendStateCreateInfo
	pDynamicState       *VkPipelineDynamicStateCreateInfo
	layout              VkPipelineLayout
	renderPass          VkRenderPass
	subpass             uint32
	basePipelineHandle  VkPipeline
	basePipelineIndex   int32
}

type VkCommandPoolCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	queueFamilyIndex uint32
}

type VkCommandBufferAllocateInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	commandPool        VkCommandPool
	level              uint32
	commandBufferCount uint32
}

type VkCommandBufferBeginInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	pInheritanceInfo unsafe.Pointer
}

// VkClearValue is a union. Depth and stencil share its first two words.
type VkClearValue struct{ words [4]uint32 }

type VkClearAttachment struct {
	aspectMask      uint32
	colorAttachment uint32
	clearValue      VkClearValue
}

type VkClearRect struct {
	rect           VkRect2D
	baseArrayLayer uint32
	layerCount     uint32
}

type VkRenderPassBeginInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	renderPass      VkRenderPass
	framebuffer     VkFramebuffer
	renderArea      VkRect2D
	clearValueCount uint32
	pClearValues    *VkClearValue
}

type VkImageMemoryBarrier struct {
	sType               uint32
	pNext               unsafe.Pointer
	srcAccessMask       uint32
	dstAccessMask       uint32
	oldLayout           uint32
	newLayout           uint32
	srcQueueFamilyIndex uint32
	dstQueueFamilyIndex uint32
	image               VkImage
	subresourceRange    VkImageSubresourceRange
}

type VkImageCopy struct {
	srcSubresource VkImageSubresourceLayers
	srcOffset      VkOffset3D
	dstSubresource VkImageSubresourceLayers
	dstOffset      VkOffset3D
	extent         VkExtent3D
}

type VkBufferImageCopy struct {
	bufferOffset      uint64
	bufferRowLength   uint32
	bufferImageHeight uint32
	imageSubresource  VkImageSubresourceLayers
	imageOffset       VkOffset3D
	imageExtent       VkExtent3D
}

type VkFenceCreateInfo struct {
	sType uint32
	pNext unsafe.Pointer
	flags uint32
}

type VkSemaphoreCreateInfo struct {
	sType uint32
	pNext unsafe.Pointer
	flags uint32
}

type VkSubmitInfo struct {
	sType                uint32
	pNext                unsafe.Pointer
	waitSemaphoreCount   uint32
	pWaitSemaphores      *VkSemaphore
	pWaitDstStageMask    *uint32
	commandBufferCount   uint32
	pCommandBuffers      *VkCommandBuffer
	signalSemaphoreCount uint32
	pSignalSemaphores    *VkSemaphore
}

type VkPresentInfoKHR struct {
	sType              uint32
	pNext              unsafe.Pointer
	waitSemaphoreCount uint32
	pWaitSemaphores    *VkSemaphore
	swapchainCount     uint32
	pSwapchains        *VkSwapchainKHR
	pImageIndices      *uint32
	pResults           *VkResult
}

type VkSwapchainCreateInfoKHR struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	surface               VkSurfaceKHR
	minImageCount         uint32
	imageFormat           uint32
	imageColorSpace       uint32
	imageExtent           VkExtent2D
	imageArrayLayers      uint32
	imageUsage            uint32
	imageSharingMode      uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
	preTransform          uint32
	compositeAlpha        uint32
	presentMode           uint32
	clipped               uint32
	oldSwapchain          VkSwapchainKHR
}

type VkSurfaceCapabilitiesKHR struct {
	minImageCount           uint32
	maxImageCount           uint32
	currentExtent           VkExtent2D
	minImageExtent          VkExtent2D
	maxImageExtent          VkExtent2D
	maxImageArrayLayers     uint32
	supportedTransforms     uint32
	currentTransform        uint32
	supportedCompositeAlpha uint32
	supportedUsageFlags     uint32
}

type VkSurfaceFormatKHR struct {
	format     uint32
	colorSpace uint32
}
