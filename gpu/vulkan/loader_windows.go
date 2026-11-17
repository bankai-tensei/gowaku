package vulkan

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// surfaceKinds are the window systems that this system can draw to.
var surfaceKinds = [...]surfaceKind{{"VK_KHR_win32_surface", "vkCreateWin32SurfaceKHR"}}

// VkWin32SurfaceCreateInfoKHR is from vulkan_win32.h.
type VkWin32SurfaceCreateInfoKHR struct {
	sType     uint32
	pNext     uintptr
	flags     uint32
	hinstance uintptr
	hwnd      uintptr
}

// openLoader finds vkGetInstanceProcAddr, from which everything else follows.
// gowaku draws with Direct3D 11 on Windows. This path exists so that the two
// backends can be compared on the same machine.
func openLoader() (uintptr, error) {
	proc := windows.NewLazySystemDLL("vulkan-1.dll").NewProc("vkGetInstanceProcAddr")
	if err := proc.Find(); err != nil {
		return 0, fmt.Errorf("vulkan: load vulkan-1.dll: %w: %w", gpu.ErrNoGPU, err)
	}
	return proc.Addr(), nil
}

// createPlatformSurface makes a surface from a Win32 window.
func (d *Device) createPlatformSurface(h platform.NativeHandle) (VkSurfaceKHR, VkResult, error) {
	win, ok := h.(platform.Win32Handle)
	if !ok {
		return 0, 0, errors.New("vulkan: native handle is not a Win32 window")
	}
	var module windows.Handle
	if err := windows.GetModuleHandleEx(windows.GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT, nil, &module); err != nil {
		return 0, 0, fmt.Errorf("vulkan: get module handle: %w", err)
	}
	info := VkWin32SurfaceCreateInfoKHR{
		sType:     VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR,
		hinstance: uintptr(module),
		hwnd:      uintptr(win.HWND),
	}
	var surface VkSurfaceKHR
	r, err := d.createSurface(0, ptr(&info), &surface)
	return surface, r, err
}
