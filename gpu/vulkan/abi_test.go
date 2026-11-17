//go:build linux || windows

package vulkan

import (
	"testing"
	"unsafe"
)

// The sizes come from a C program built against vulkan_core.h of the SDK
// (CODING_STYLE §8). Windows and Linux agree on them for 64-bit targets.
func TestABI_StructSizes(t *testing.T) {
	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"VkApplicationInfo", unsafe.Sizeof(VkApplicationInfo{}), 48},
		{"VkClearAttachment", unsafe.Sizeof(VkClearAttachment{}), 24},
		{"VkClearRect", unsafe.Sizeof(VkClearRect{}), 24},
		{"VkInstanceCreateInfo", unsafe.Sizeof(VkInstanceCreateInfo{}), 64},
		{"VkDeviceQueueCreateInfo", unsafe.Sizeof(VkDeviceQueueCreateInfo{}), 40},
		{"VkDeviceCreateInfo", unsafe.Sizeof(VkDeviceCreateInfo{}), 72},
		{"VkQueueFamilyProperties", unsafe.Sizeof(VkQueueFamilyProperties{}), 24},
		{"VkPhysicalDeviceProperties", unsafe.Sizeof(VkPhysicalDeviceProperties{}), 824},
		{"VkPhysicalDeviceMemoryProperties", unsafe.Sizeof(VkPhysicalDeviceMemoryProperties{}), 520},
		{"VkMemoryRequirements", unsafe.Sizeof(VkMemoryRequirements{}), 24},
		{"VkMemoryAllocateInfo", unsafe.Sizeof(VkMemoryAllocateInfo{}), 32},
		{"VkBufferCreateInfo", unsafe.Sizeof(VkBufferCreateInfo{}), 56},
		{"VkImageCreateInfo", unsafe.Sizeof(VkImageCreateInfo{}), 88},
		{"VkImageViewCreateInfo", unsafe.Sizeof(VkImageViewCreateInfo{}), 80},
		{"VkSamplerCreateInfo", unsafe.Sizeof(VkSamplerCreateInfo{}), 80},
		{"VkShaderModuleCreateInfo", unsafe.Sizeof(VkShaderModuleCreateInfo{}), 40},
		{"VkDescriptorSetLayoutBinding", unsafe.Sizeof(VkDescriptorSetLayoutBinding{}), 24},
		{"VkDescriptorSetLayoutCreateInfo", unsafe.Sizeof(VkDescriptorSetLayoutCreateInfo{}), 32},
		{"VkPipelineLayoutCreateInfo", unsafe.Sizeof(VkPipelineLayoutCreateInfo{}), 48},
		{"VkPushConstantRange", unsafe.Sizeof(VkPushConstantRange{}), 12},
		{"VkSpecializationMapEntry", unsafe.Sizeof(VkSpecializationMapEntry{}), 16},
		{"VkSpecializationInfo", unsafe.Sizeof(VkSpecializationInfo{}), 32},
		{"VkDescriptorPoolSize", unsafe.Sizeof(VkDescriptorPoolSize{}), 8},
		{"VkQueryPoolCreateInfo", unsafe.Sizeof(VkQueryPoolCreateInfo{}), 32},
		{"VkDescriptorPoolCreateInfo", unsafe.Sizeof(VkDescriptorPoolCreateInfo{}), 40},
		{"VkDescriptorSetAllocateInfo", unsafe.Sizeof(VkDescriptorSetAllocateInfo{}), 40},
		{"VkDescriptorBufferInfo", unsafe.Sizeof(VkDescriptorBufferInfo{}), 24},
		{"VkDescriptorImageInfo", unsafe.Sizeof(VkDescriptorImageInfo{}), 24},
		{"VkWriteDescriptorSet", unsafe.Sizeof(VkWriteDescriptorSet{}), 64},
		{"VkAttachmentDescription", unsafe.Sizeof(VkAttachmentDescription{}), 36},
		{"VkAttachmentReference", unsafe.Sizeof(VkAttachmentReference{}), 8},
		{"VkSubpassDescription", unsafe.Sizeof(VkSubpassDescription{}), 72},
		{"VkSubpassDependency", unsafe.Sizeof(VkSubpassDependency{}), 28},
		{"VkRenderPassCreateInfo", unsafe.Sizeof(VkRenderPassCreateInfo{}), 64},
		{"VkFramebufferCreateInfo", unsafe.Sizeof(VkFramebufferCreateInfo{}), 64},
		{"VkPipelineShaderStageCreateInfo", unsafe.Sizeof(VkPipelineShaderStageCreateInfo{}), 48},
		{"VkVertexInputBindingDescription", unsafe.Sizeof(VkVertexInputBindingDescription{}), 12},
		{"VkVertexInputAttributeDescription", unsafe.Sizeof(VkVertexInputAttributeDescription{}), 16},
		{"VkPipelineVertexInputStateCreateInfo", unsafe.Sizeof(VkPipelineVertexInputStateCreateInfo{}), 48},
		{"VkPipelineInputAssemblyStateCreateInfo", unsafe.Sizeof(VkPipelineInputAssemblyStateCreateInfo{}), 32},
		{"VkPipelineViewportStateCreateInfo", unsafe.Sizeof(VkPipelineViewportStateCreateInfo{}), 48},
		{"VkPipelineRasterizationStateCreateInfo", unsafe.Sizeof(VkPipelineRasterizationStateCreateInfo{}), 64},
		{"VkPipelineMultisampleStateCreateInfo", unsafe.Sizeof(VkPipelineMultisampleStateCreateInfo{}), 48},
		{"VkStencilOpState", unsafe.Sizeof(VkStencilOpState{}), 28},
		{"VkPipelineDepthStencilStateCreateInfo", unsafe.Sizeof(VkPipelineDepthStencilStateCreateInfo{}), 104},
		{"VkPipelineColorBlendAttachmentState", unsafe.Sizeof(VkPipelineColorBlendAttachmentState{}), 32},
		{"VkPipelineColorBlendStateCreateInfo", unsafe.Sizeof(VkPipelineColorBlendStateCreateInfo{}), 56},
		{"VkPipelineDynamicStateCreateInfo", unsafe.Sizeof(VkPipelineDynamicStateCreateInfo{}), 32},
		{"VkGraphicsPipelineCreateInfo", unsafe.Sizeof(VkGraphicsPipelineCreateInfo{}), 144},
		{"VkCommandPoolCreateInfo", unsafe.Sizeof(VkCommandPoolCreateInfo{}), 24},
		{"VkCommandBufferAllocateInfo", unsafe.Sizeof(VkCommandBufferAllocateInfo{}), 32},
		{"VkCommandBufferBeginInfo", unsafe.Sizeof(VkCommandBufferBeginInfo{}), 32},
		{"VkRenderPassBeginInfo", unsafe.Sizeof(VkRenderPassBeginInfo{}), 64},
		{"VkClearValue", unsafe.Sizeof(VkClearValue{}), 16},
		{"VkViewport", unsafe.Sizeof(VkViewport{}), 24},
		{"VkRect2D", unsafe.Sizeof(VkRect2D{}), 16},
		{"VkImageMemoryBarrier", unsafe.Sizeof(VkImageMemoryBarrier{}), 72},
		{"VkBufferImageCopy", unsafe.Sizeof(VkBufferImageCopy{}), 56},
		{"VkImageCopy", unsafe.Sizeof(VkImageCopy{}), 68},
		{"VkFenceCreateInfo", unsafe.Sizeof(VkFenceCreateInfo{}), 24},
		{"VkSemaphoreCreateInfo", unsafe.Sizeof(VkSemaphoreCreateInfo{}), 24},
		{"VkSubmitInfo", unsafe.Sizeof(VkSubmitInfo{}), 72},
		{"VkPresentInfoKHR", unsafe.Sizeof(VkPresentInfoKHR{}), 64},
		{"VkSwapchainCreateInfoKHR", unsafe.Sizeof(VkSwapchainCreateInfoKHR{}), 104},
		{"VkSurfaceCapabilitiesKHR", unsafe.Sizeof(VkSurfaceCapabilitiesKHR{}), 52},
		{"VkSurfaceFormatKHR", unsafe.Sizeof(VkSurfaceFormatKHR{}), 8},
		{"VkExtensionProperties", unsafe.Sizeof(VkExtensionProperties{}), 260},
		{"VkImageSubresourceRange", unsafe.Sizeof(VkImageSubresourceRange{}), 20},
		{"VkImageSubresourceLayers", unsafe.Sizeof(VkImageSubresourceLayers{}), 16},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: Go %d, C %d", tt.name, tt.got, tt.want)
		}
	}
}
