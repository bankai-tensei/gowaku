//go:build linux || windows

package vulkan

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// call runs a C function of the Vulkan API. The caller's pointers converted to
// uintptr stay alive for the call, like those of syscall.SyscallN.
//
//go:uintptrescapes
func call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

// callResult is call for the functions that return a VkResult. The value lives
// in the low 32 bits of the register.
//
//go:uintptrescapes
func callResult(fn uintptr, args ...uintptr) VkResult {
	r, _, _ := purego.SyscallN(fn, args...)
	return VkResult(int32(r))
}

// cstring returns s as a NUL-terminated C string.
func cstring(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

// procs holds every function gowaku calls. Loading fills all of it or reports
// what is missing, so that a call never goes through a nil pointer.
type procs struct {
	vkGetInstanceProcAddr uintptr
	vkGetDeviceProcAddr   uintptr

	vkCreateInstance                       uintptr
	vkEnumerateInstanceExtensionProperties uintptr

	vkDestroyInstance                         uintptr
	vkEnumeratePhysicalDevices                uintptr
	vkGetPhysicalDeviceProperties             uintptr
	vkGetPhysicalDeviceMemoryProperties       uintptr
	vkGetPhysicalDeviceQueueFamilyProperties  uintptr
	vkGetPhysicalDeviceFormatProperties       uintptr
	vkEnumerateDeviceExtensionProperties      uintptr
	vkCreateDevice                            uintptr
	vkDestroySurfaceKHR                       uintptr
	vkGetPhysicalDeviceSurfaceSupportKHR      uintptr
	vkGetPhysicalDeviceSurfaceCapabilitiesKHR uintptr
	vkGetPhysicalDeviceSurfaceFormatsKHR      uintptr
	vkGetPhysicalDeviceSurfacePresentModesKHR uintptr
	// vkCreateSurface holds the vkCreate*SurfaceKHR of each kind in
	// surfaceKinds whose extension the instance has, and zero for the others.
	vkCreateSurface [len(surfaceKinds)]uintptr

	vkDestroyDevice           uintptr
	vkGetDeviceQueue          uintptr
	vkDeviceWaitIdle          uintptr
	vkAllocateMemory          uintptr
	vkFreeMemory              uintptr
	vkMapMemory               uintptr
	vkCreateBuffer            uintptr
	vkDestroyBuffer           uintptr
	vkGetBufferMemoryReqs     uintptr
	vkBindBufferMemory        uintptr
	vkCreateImage             uintptr
	vkDestroyImage            uintptr
	vkGetImageMemoryReqs      uintptr
	vkBindImageMemory         uintptr
	vkCreateImageView         uintptr
	vkDestroyImageView        uintptr
	vkCreateSampler           uintptr
	vkDestroySampler          uintptr
	vkCreateShaderModule      uintptr
	vkDestroyShaderModule     uintptr
	vkCreateDescriptorLayout  uintptr
	vkDestroyDescriptorLayout uintptr
	vkCreatePipelineLayout    uintptr
	vkDestroyPipelineLayout   uintptr
	vkCreateDescriptorPool    uintptr
	vkDestroyDescriptorPool   uintptr
	vkAllocateDescriptorSets  uintptr
	vkFreeDescriptorSets      uintptr
	vkUpdateDescriptorSets    uintptr
	vkCreateRenderPass        uintptr
	vkDestroyRenderPass       uintptr
	vkCreateFramebuffer       uintptr
	vkDestroyFramebuffer      uintptr
	vkCreateGraphicsPipelines uintptr
	vkDestroyPipeline         uintptr
	vkCreateCommandPool       uintptr
	vkDestroyCommandPool      uintptr
	vkAllocateCommandBuffers  uintptr
	vkResetCommandBuffer      uintptr
	vkBeginCommandBuffer      uintptr
	vkEndCommandBuffer        uintptr
	vkCreateFence             uintptr
	vkDestroyFence            uintptr
	vkWaitForFences           uintptr
	vkResetFences             uintptr
	vkCreateSemaphore         uintptr
	vkDestroySemaphore        uintptr
	vkQueueSubmit             uintptr
	vkCreateSwapchainKHR      uintptr
	vkDestroySwapchainKHR     uintptr
	vkGetSwapchainImagesKHR   uintptr
	vkAcquireNextImageKHR     uintptr
	vkQueuePresentKHR         uintptr

	vkCmdBeginRenderPass    uintptr
	vkCmdEndRenderPass      uintptr
	vkCmdBindPipeline       uintptr
	vkCmdBindDescriptorSets uintptr
	vkCmdBindVertexBuffers  uintptr
	vkCmdSetViewport        uintptr
	vkCmdSetScissor         uintptr
	vkCmdPushConstants      uintptr
	vkCmdDraw               uintptr
	vkCmdPipelineBarrier    uintptr
	vkCmdCopyBufferToImage  uintptr
	vkCmdCopyImageToBuffer  uintptr
	vkCmdCopyImage          uintptr
	vkCmdClearColorImage    uintptr
	vkCmdClearAttachments   uintptr
	vkCmdResetQueryPool     uintptr
	vkCmdWriteTimestamp     uintptr
	vkCreateQueryPool       uintptr
	vkDestroyQueryPool      uintptr
	vkGetQueryPoolResults   uintptr
}

type symbol struct {
	name string
	proc *uintptr
}

// loadLoader opens the Vulkan loader once per process.
var loadLoader = sync.OnceValues(openLoader)

func (p *procs) loadGlobal() error {
	addr, err := loadLoader()
	if err != nil {
		return err
	}
	p.vkGetInstanceProcAddr = addr
	return p.resolve(0, addr, []symbol{
		{"vkCreateInstance", &p.vkCreateInstance},
		{"vkEnumerateInstanceExtensionProperties", &p.vkEnumerateInstanceExtensionProperties},
	})
}

func (p *procs) loadInstance(instance VkInstance) error {
	return p.resolve(uintptr(instance), p.vkGetInstanceProcAddr, []symbol{
		{"vkDestroyInstance", &p.vkDestroyInstance},
		{"vkEnumeratePhysicalDevices", &p.vkEnumeratePhysicalDevices},
		{"vkGetPhysicalDeviceProperties", &p.vkGetPhysicalDeviceProperties},
		{"vkGetPhysicalDeviceMemoryProperties", &p.vkGetPhysicalDeviceMemoryProperties},
		{"vkGetPhysicalDeviceQueueFamilyProperties", &p.vkGetPhysicalDeviceQueueFamilyProperties},
		{"vkGetPhysicalDeviceFormatProperties", &p.vkGetPhysicalDeviceFormatProperties},
		{"vkEnumerateDeviceExtensionProperties", &p.vkEnumerateDeviceExtensionProperties},
		{"vkCreateDevice", &p.vkCreateDevice},
		{"vkDestroySurfaceKHR", &p.vkDestroySurfaceKHR},
		{"vkGetPhysicalDeviceSurfaceSupportKHR", &p.vkGetPhysicalDeviceSurfaceSupportKHR},
		{"vkGetPhysicalDeviceSurfaceCapabilitiesKHR", &p.vkGetPhysicalDeviceSurfaceCapabilitiesKHR},
		{"vkGetPhysicalDeviceSurfaceFormatsKHR", &p.vkGetPhysicalDeviceSurfaceFormatsKHR},
		{"vkGetPhysicalDeviceSurfacePresentModesKHR", &p.vkGetPhysicalDeviceSurfacePresentModesKHR},
		{"vkGetDeviceProcAddr", &p.vkGetDeviceProcAddr},
	})
}

// loadSurfaces finds the surface functions of the extensions in enabled.
func (p *procs) loadSurfaces(instance VkInstance, enabled [len(surfaceKinds)]bool) error {
	for i, kind := range surfaceKinds {
		if !enabled[i] {
			continue
		}
		if err := p.resolve(uintptr(instance), p.vkGetInstanceProcAddr, []symbol{{kind.function, &p.vkCreateSurface[i]}}); err != nil {
			return err
		}
	}
	return nil
}

func (p *procs) loadDevice(device VkDevice) error {
	return p.resolve(uintptr(device), p.vkGetDeviceProcAddr, []symbol{
		{"vkDestroyDevice", &p.vkDestroyDevice},
		{"vkGetDeviceQueue", &p.vkGetDeviceQueue},
		{"vkDeviceWaitIdle", &p.vkDeviceWaitIdle},
		{"vkAllocateMemory", &p.vkAllocateMemory},
		{"vkFreeMemory", &p.vkFreeMemory},
		{"vkMapMemory", &p.vkMapMemory},
		{"vkCreateBuffer", &p.vkCreateBuffer},
		{"vkDestroyBuffer", &p.vkDestroyBuffer},
		{"vkGetBufferMemoryRequirements", &p.vkGetBufferMemoryReqs},
		{"vkBindBufferMemory", &p.vkBindBufferMemory},
		{"vkCreateImage", &p.vkCreateImage},
		{"vkDestroyImage", &p.vkDestroyImage},
		{"vkGetImageMemoryRequirements", &p.vkGetImageMemoryReqs},
		{"vkBindImageMemory", &p.vkBindImageMemory},
		{"vkCreateImageView", &p.vkCreateImageView},
		{"vkDestroyImageView", &p.vkDestroyImageView},
		{"vkCreateSampler", &p.vkCreateSampler},
		{"vkDestroySampler", &p.vkDestroySampler},
		{"vkCreateShaderModule", &p.vkCreateShaderModule},
		{"vkDestroyShaderModule", &p.vkDestroyShaderModule},
		{"vkCreateDescriptorSetLayout", &p.vkCreateDescriptorLayout},
		{"vkDestroyDescriptorSetLayout", &p.vkDestroyDescriptorLayout},
		{"vkCreatePipelineLayout", &p.vkCreatePipelineLayout},
		{"vkDestroyPipelineLayout", &p.vkDestroyPipelineLayout},
		{"vkCreateDescriptorPool", &p.vkCreateDescriptorPool},
		{"vkDestroyDescriptorPool", &p.vkDestroyDescriptorPool},
		{"vkAllocateDescriptorSets", &p.vkAllocateDescriptorSets},
		{"vkFreeDescriptorSets", &p.vkFreeDescriptorSets},
		{"vkUpdateDescriptorSets", &p.vkUpdateDescriptorSets},
		{"vkCreateRenderPass", &p.vkCreateRenderPass},
		{"vkDestroyRenderPass", &p.vkDestroyRenderPass},
		{"vkCreateFramebuffer", &p.vkCreateFramebuffer},
		{"vkDestroyFramebuffer", &p.vkDestroyFramebuffer},
		{"vkCreateGraphicsPipelines", &p.vkCreateGraphicsPipelines},
		{"vkDestroyPipeline", &p.vkDestroyPipeline},
		{"vkCreateCommandPool", &p.vkCreateCommandPool},
		{"vkDestroyCommandPool", &p.vkDestroyCommandPool},
		{"vkAllocateCommandBuffers", &p.vkAllocateCommandBuffers},
		{"vkResetCommandBuffer", &p.vkResetCommandBuffer},
		{"vkBeginCommandBuffer", &p.vkBeginCommandBuffer},
		{"vkEndCommandBuffer", &p.vkEndCommandBuffer},
		{"vkCreateFence", &p.vkCreateFence},
		{"vkDestroyFence", &p.vkDestroyFence},
		{"vkWaitForFences", &p.vkWaitForFences},
		{"vkResetFences", &p.vkResetFences},
		{"vkCreateSemaphore", &p.vkCreateSemaphore},
		{"vkDestroySemaphore", &p.vkDestroySemaphore},
		{"vkQueueSubmit", &p.vkQueueSubmit},
		{"vkCreateSwapchainKHR", &p.vkCreateSwapchainKHR},
		{"vkDestroySwapchainKHR", &p.vkDestroySwapchainKHR},
		{"vkGetSwapchainImagesKHR", &p.vkGetSwapchainImagesKHR},
		{"vkAcquireNextImageKHR", &p.vkAcquireNextImageKHR},
		{"vkQueuePresentKHR", &p.vkQueuePresentKHR},
		{"vkCmdBeginRenderPass", &p.vkCmdBeginRenderPass},
		{"vkCmdEndRenderPass", &p.vkCmdEndRenderPass},
		{"vkCmdBindPipeline", &p.vkCmdBindPipeline},
		{"vkCmdBindDescriptorSets", &p.vkCmdBindDescriptorSets},
		{"vkCmdBindVertexBuffers", &p.vkCmdBindVertexBuffers},
		{"vkCmdSetViewport", &p.vkCmdSetViewport},
		{"vkCmdSetScissor", &p.vkCmdSetScissor},
		{"vkCmdPushConstants", &p.vkCmdPushConstants},
		{"vkCmdDraw", &p.vkCmdDraw},
		{"vkCmdPipelineBarrier", &p.vkCmdPipelineBarrier},
		{"vkCmdCopyBufferToImage", &p.vkCmdCopyBufferToImage},
		{"vkCmdCopyImageToBuffer", &p.vkCmdCopyImageToBuffer},
		{"vkCmdCopyImage", &p.vkCmdCopyImage},
		{"vkCmdClearColorImage", &p.vkCmdClearColorImage},
		{"vkCmdClearAttachments", &p.vkCmdClearAttachments},
		{"vkCmdResetQueryPool", &p.vkCmdResetQueryPool},
		{"vkCmdWriteTimestamp", &p.vkCmdWriteTimestamp},
		{"vkCreateQueryPool", &p.vkCreateQueryPool},
		{"vkDestroyQueryPool", &p.vkDestroyQueryPool},
		{"vkGetQueryPoolResults", &p.vkGetQueryPoolResults},
	})
}

// resolve looks up symbols through a vkGet*ProcAddr function.
func (*procs) resolve(owner, getProcAddr uintptr, symbols []symbol) error {
	for _, s := range symbols {
		name := cstring(s.name)
		*s.proc = call(getProcAddr, owner, uintptr(unsafe.Pointer(name)))
		if *s.proc == 0 {
			return fmt.Errorf("vulkan: driver lacks %s", s.name)
		}
	}
	return nil
}
