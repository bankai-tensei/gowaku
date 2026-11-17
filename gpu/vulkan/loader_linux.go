package vulkan

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// surfaceKinds are the window systems that this system can draw to. A loader
// may have either; the instance takes those it has.
var surfaceKinds = [...]surfaceKind{
	surfaceX11:     {"VK_KHR_xlib_surface", "vkCreateXlibSurfaceKHR"},
	surfaceWayland: {"VK_KHR_wayland_surface", "vkCreateWaylandSurfaceKHR"},
}

// The indices of surfaceKinds.
const (
	surfaceX11 = iota
	surfaceWayland
)

// VkXlibSurfaceCreateInfoKHR is from vulkan_xlib.h.
type VkXlibSurfaceCreateInfoKHR struct {
	sType  uint32
	pNext  uintptr
	flags  uint32
	dpy    uintptr
	window uintptr
}

// VkWaylandSurfaceCreateInfoKHR is from vulkan_wayland.h.
type VkWaylandSurfaceCreateInfoKHR struct {
	sType   uint32
	pNext   uintptr
	flags   uint32
	display uintptr
	surface uintptr
}

// openLoader finds vkGetInstanceProcAddr, from which everything else follows.
func openLoader() (uintptr, error) {
	lib, err := purego.Dlopen("libvulkan.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return 0, fmt.Errorf("vulkan: load libvulkan.so.1: %w: %w", gpu.ErrNoGPU, err)
	}
	addr, err := purego.Dlsym(lib, "vkGetInstanceProcAddr")
	if err != nil {
		return 0, fmt.Errorf("vulkan: find vkGetInstanceProcAddr: %w", err)
	}
	return addr, nil
}

// createPlatformSurface makes a surface from a window of the X server or of a
// Wayland compositor.
func (d *Device) createPlatformSurface(h platform.NativeHandle) (VkSurfaceKHR, VkResult, error) {
	var surface VkSurfaceKHR
	switch win := h.(type) {
	case platform.X11Handle:
		info := VkXlibSurfaceCreateInfoKHR{
			sType:  VK_STRUCTURE_TYPE_XLIB_SURFACE_CREATE_INFO_KHR,
			dpy:    win.Display,
			window: win.Window,
		}
		r, err := d.createSurface(surfaceX11, ptr(&info), &surface)
		return surface, r, err
	case platform.WaylandHandle:
		info := VkWaylandSurfaceCreateInfoKHR{
			sType:   VK_STRUCTURE_TYPE_WAYLAND_SURFACE_CREATE_INFO_KHR,
			display: win.Display,
			surface: win.Surface,
		}
		r, err := d.createSurface(surfaceWayland, ptr(&info), &surface)
		return surface, r, err
	}
	return 0, 0, errors.New("vulkan: native handle is neither an X11 window nor a Wayland surface")
}
