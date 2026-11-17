package win32

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

const (
	defaultWidth  = 800
	defaultHeight = 600
	// workAreaShare is the largest part of the work area of its monitor, in
	// percent, that a window takes when it opens.
	workAreaShare = 90
)

// Window is the Win32 implementation of platform.Window.
type Window struct {
	platform *Platform
	// hwnd is zero once the OS window is gone.
	hwnd HWND
	id   platform.WindowID
	dpi  uint32

	physicalWidth, physicalHeight int

	cursor input.Cursor
	// buttonsDown has a bit per input.Button, and holds the mouse capture.
	buttonsDown uint8
	// tracking means that WM_MOUSELEAVE is requested.
	tracking bool
	// highSurrogate is the first half of a character arriving through WM_CHAR.
	highSurrogate rune
	// imeVertical is what the IME was last told about the writing direction.
	imeVertical bool
	// imeWanted is what SetIMEEnabled last asked, and imeSecure what
	// SetSecureInput did, which keeps the input context away regardless.
	imeWanted, imeSecure bool
	// a11y is the UI Automation provider of the window.
	a11y *bridge
	// disabledOwner is the owner that a modal window disabled, or zero.
	disabledOwner HWND
	// customFrame means the application draws the title bar, and hitTest
	// says what its points are. ncTracking means that WM_NCMOUSELEAVE is
	// requested.
	customFrame bool
	hitTest     func(physicalX, physicalY int) platform.HitArea
	ncTracking  bool
	// dropHandler answers drags over the window, whose IDropTarget OLE knows
	// when dropRegistered is set.
	dropHandler    func(e *platform.DropEvent) input.DropEffect
	dropRegistered bool
	// pinch is the DirectManipulation of the window, or nil.
	pinch *pinch
	// deco is the icon, size limits and states of the window.
	deco decorations
}

var _ platform.Window = (*Window)(nil)

var _ platform.SecureInputWindow = (*Window)(nil)

// Accessibility implements platform.Window.
func (w *Window) Accessibility() platform.Accessibility {
	if w.a11y == nil {
		return nil
	}
	return w.a11y
}

// CreateWindow implements platform.Platform.
func (p *Platform) CreateWindow(opts platform.WindowOptions) (platform.Window, error) {
	owner := ownerOf(&opts)
	style := uintptr(WS_OVERLAPPEDWINDOW)
	if owner != 0 {
		// A window that belongs to another cannot be minimized on its own.
		style &^= WS_MINIMIZEBOX
	}
	var exStyle uintptr
	if opts.Transparent {
		// DirectComposition shows the swap chain with its alpha; a
		// redirection surface would put an opaque one behind it.
		exStyle = WS_EX_NOREDIRECTIONBITMAP
	}
	hwnd, err := p.createNativeWindow(exStyle, style, uintptr(owner))
	if err != nil {
		return nil, fmt.Errorf("win32: create window: %w", err)
	}
	p.lastID++
	w := &Window{platform: p, hwnd: hwnd, id: p.lastID, customFrame: opts.CustomFrame}
	w.deco.transparent = opts.Transparent
	w.deco.minSize, w.deco.maxSize = opts.MinSize, opts.MaxSize
	w.a11y = newBridge(w)
	if w.customFrame {
		// WM_NCCALCSIZE must find the window to drop the caption.
		p.windows[hwnd] = w
		w.applyFrame()
	}

	// The DPI is unknown until the OS has picked a monitor, so the exact size
	// can only be set after creation. The window is still hidden here.
	dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	w.dpi = uint32(dpi)
	width, height := opts.Width, opts.Height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	width, height = w.clampSize(width, height)
	if err := w.setClientSize(width, height); err != nil {
		w.Close()
		return nil, err
	}
	w.SetTitle(opts.Title)
	if opts.Placed && owner == 0 {
		if err := w.place(opts.ClientX, opts.ClientY, opts.Width, opts.Height); err != nil {
			w.Close()
			return nil, err
		}
	} else if owner == 0 {
		w.fitInWorkArea()
	}

	p.windows[hwnd] = w
	if err := w.refreshClientSize(); err != nil {
		w.Close()
		return nil, err
	}
	w.SetIMEEnabled(false)
	if opts.DarkTitleBar {
		w.SetDarkTitleBar(true)
	}
	if owner != 0 {
		w.centerOver(owner)
		if opts.Modal {
			_, _, _ = syscall.SyscallN(procEnableWindow.Addr(), uintptr(owner), 0)
			w.disabledOwner = owner
		}
	}
	w.registerDrop()
	w.startPinch()
	if opts.Icon != nil {
		w.SetIcon(opts.Icon)
	}
	if opts.AlwaysOnTop {
		w.SetAlwaysOnTop(true)
	}
	if opts.Fullscreen {
		w.SetFullscreen(true)
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), SW_SHOW)
	return w, nil
}

// ID implements platform.Window.
func (w *Window) ID() platform.WindowID { return w.id }

// PhysicalSize implements platform.Window.
func (w *Window) PhysicalSize() (width, height int) { return w.physicalWidth, w.physicalHeight }

// Scale implements platform.Window.
func (w *Window) Scale() float32 { return float32(w.dpi) / USER_DEFAULT_SCREEN_DPI }

// SetTitle implements platform.Window.
func (w *Window) SetTitle(title string) {
	t, err := windows.UTF16PtrFromString(title)
	if err != nil {
		panic("gowaku: window title contains a NUL byte")
	}
	_, _, _ = procSetWindowTextW.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(t)))
}

// NativeHandle implements platform.Window.
func (w *Window) NativeHandle() platform.NativeHandle {
	return platform.Win32Handle{HWND: w.hwnd, Transparent: w.deco.transparent}
}

// Close implements platform.Window.
func (w *Window) Close() {
	if w.hwnd == 0 {
		return
	}
	w.enableOwner()
	w.revokeDrop()
	w.stopPinch()
	if r, _, err := procDestroyWindow.Call(uintptr(w.hwnd)); r == 0 {
		panic("gowaku: destroy window: " + err.Error())
	}
	// WM_DESTROY clears this for registered windows; a window that failed
	// during creation never was one.
	w.hwnd = 0
	w.destroyIcons()
}

// setClientSize resizes the window so that its client area is the given
// logical size at the current DPI, but no larger than workAreaShare percent of
// the work area of its monitor.
func (w *Window) setClientSize(width, height float32) error {
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	scale := w.Scale()
	r := RECT{right: int32(width*scale + 0.5), bottom: int32(height*scale + 0.5)}
	// r is only used during the call.
	if ok, _, err := procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), WS_OVERLAPPEDWINDOW, 0, 0, uintptr(w.dpi)); ok == 0 {
		return fmt.Errorf("win32: adjust window rect: %w", err)
	}
	if w.customFrame {
		// The client area starts at the top of the window.
		r.top = 0
	}
	outerWidth, outerHeight := r.right-r.left, r.bottom-r.top
	if a, ok := workAreaOf(w.hwnd); ok {
		// Logical sizes chosen on a large screen do not fit a small one at a
		// high scale; see docs/ADR/0110-fit-windows-to-the-work-area.md.
		outerWidth = min(outerWidth, (a.right-a.left)*workAreaShare/100)
		outerHeight = min(outerHeight, (a.bottom-a.top)*workAreaShare/100)
	}
	if ok, _, err := procSetWindowPos.Call(uintptr(w.hwnd), 0, 0, 0,
		uintptr(outerWidth), uintptr(outerHeight), SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE); ok == 0 {
		return fmt.Errorf("win32: set window size: %w", err)
	}
	return nil
}

func (w *Window) refreshClientSize() error {
	var r RECT
	// r is only used during the call.
	if ok, _, err := procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return fmt.Errorf("win32: get client rect: %w", err)
	}
	w.physicalWidth, w.physicalHeight = int(r.right-r.left), int(r.bottom-r.top)
	return nil
}

// applySuggestedRect moves the window to the rectangle the OS proposed for the
// new DPI. Following it keeps the pointer on the same part of the window.
func (w *Window) applySuggestedRect(r *RECT) {
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0,
		uintptr(r.left), uintptr(r.top), uintptr(r.right-r.left), uintptr(r.bottom-r.top),
		SWP_NOZORDER|SWP_NOACTIVATE)
}

func (w *Window) resizeEvent() platform.Event {
	return platform.Event{
		Kind:           platform.EventResize,
		Window:         w.id,
		PhysicalWidth:  w.physicalWidth,
		PhysicalHeight: w.physicalHeight,
		Scale:          w.Scale(),
	}
}
