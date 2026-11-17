package platform

// NativeHandle identifies an OS window to the gpu layer. The set of
// implementations is closed; gpu backends type-switch on the concrete type
// they support.
type NativeHandle interface {
	nativeHandle()
}

// HWND is a Win32 window handle.
type HWND uintptr

// Win32Handle is the NativeHandle of a Win32 window.
type Win32Handle struct {
	HWND HWND
	// Transparent means that the window has no redirection surface and wants
	// a swap chain with premultiplied alpha through DirectComposition.
	Transparent bool
}

func (Win32Handle) nativeHandle() {}

// X11Handle is the NativeHandle of an X11 window.
type X11Handle struct {
	// Display is the Display* of Xlib.
	Display uintptr
	// Window is the XID of the window.
	Window uintptr
	// Transparent means that the window has a visual with alpha, and wants a
	// swap chain that composites with premultiplied alpha.
	Transparent bool
}

func (X11Handle) nativeHandle() {}

// WaylandHandle is the NativeHandle of a Wayland surface.
type WaylandHandle struct {
	// Display is the struct wl_display* of libwayland-client.
	Display uintptr
	// Surface is the struct wl_surface* of the window.
	Surface uintptr
	// Transparent means that the window wants a swap chain that composites
	// with premultiplied alpha.
	Transparent bool
}

func (WaylandHandle) nativeHandle() {}

// CocoaHandle is the NativeHandle of a Cocoa window.
type CocoaHandle struct {
	// Layer is the CAMetalLayer* that backs the content view of the window.
	Layer uintptr
}

func (CocoaHandle) nativeHandle() {}
