package win32

import (
	"image"
	"syscall"
	"unsafe"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform/internal/icon"
)

// The icon, size limits, always on top, fullscreen and transparency of a
// window. See docs/ADR/0111-window-decorations.md.

var (
	procSendMessageW       = user32.NewProc("SendMessageW")
	procGetWindowLongW     = user32.NewProc("GetWindowLongW")
	procSetWindowLongW     = user32.NewProc("SetWindowLongW")
	procGetWindowPlacement = user32.NewProc("GetWindowPlacement")
	procSetWindowPlacement = user32.NewProc("SetWindowPlacement")
)

// The messages, styles and metrics of the decorations.
const (
	WM_SETICON       = 0x0080
	WM_GETMINMAXINFO = 0x0024
	ICON_SMALL       = 0
	ICON_BIG         = 1
	SM_CXICON        = 11
	SM_CXSMICON      = 49

	WS_EX_NOREDIRECTIONBITMAP = 0x00200000

	SWP_NOOWNERZORDER = 0x0200

	// GWL_STYLE is -16, as an unsigned argument.
	GWL_STYLE = ^uintptr(15)
	// HWND_TOP is 0, HWND_TOPMOST (HWND)-1 and HWND_NOTOPMOST (HWND)-2.
	HWND_TOP       = 0
	HWND_TOPMOST   = ^uintptr(0)
	HWND_NOTOPMOST = ^uintptr(1)
)

// WINDOWPLACEMENT is the Win32 WINDOWPLACEMENT structure.
type WINDOWPLACEMENT struct {
	length           uint32
	flags            uint32
	showCmd          uint32
	ptMinPosition    POINT
	ptMaxPosition    POINT
	rcNormalPosition RECT
}

// MINMAXINFO is the Win32 MINMAXINFO structure.
type MINMAXINFO struct {
	// ptReserved, ptMaxSize and ptMaxPosition, which gowaku leaves alone.
	_              [3]POINT
	ptMinTrackSize POINT
	ptMaxTrackSize POINT
}

// decorations is the state of the decorations of a Window.
type decorations struct {
	// icons are what SetIcon was given, and bigIcon and smallIcon the icons
	// made of them at the DPI of the window, which the window owns.
	icons              []*image.NRGBA
	bigIcon, smallIcon uintptr
	// minSize and maxSize bound the client area, in logical pixels.
	minSize, maxSize geom.Size
	topmost          bool
	// fullscreen means that the window covers its monitor without a frame,
	// and restore and style are what it goes back to.
	fullscreen bool
	restore    WINDOWPLACEMENT
	style      uint32
	// transparent means that the window has no redirection surface, and
	// DirectComposition shows the alpha of its swap chain.
	transparent bool
}

// SetIcon implements platform.Window, with WM_SETICON in the sizes of the
// metrics of the DPI of the window. WM_DPICHANGED makes them again.
func (w *Window) SetIcon(images []*image.NRGBA) {
	w.deco.icons = images
	w.applyIcons()
}

// applyIcons makes the icons of the window at its DPI and hands them over,
// freeing those they replace.
func (w *Window) applyIcons() {
	if w.hwnd == 0 {
		return
	}
	var big, small uintptr
	if icon.Largest(w.deco.icons) > 0 {
		big = iconOf(icon.Fit(w.deco.icons, int(w.metric(SM_CXICON))))
		small = iconOf(icon.Fit(w.deco.icons, int(w.metric(SM_CXSMICON))))
	}
	_, _, _ = syscall.SyscallN(procSendMessageW.Addr(), uintptr(w.hwnd), WM_SETICON, ICON_BIG, big)
	_, _, _ = syscall.SyscallN(procSendMessageW.Addr(), uintptr(w.hwnd), WM_SETICON, ICON_SMALL, small)
	w.destroyIcons()
	w.deco.bigIcon, w.deco.smallIcon = big, small
}

// destroyIcons frees the icons the window was given.
func (w *Window) destroyIcons() {
	destroyIcon(w.deco.bigIcon)
	destroyIcon(w.deco.smallIcon)
	w.deco.bigIcon, w.deco.smallIcon = 0, 0
}

// SetSizeLimits implements platform.Window. WM_GETMINMAXINFO asks for the
// limits at the DPI of the moment.
func (w *Window) SetSizeLimits(minSize, maxSize geom.Size) {
	w.deco.minSize, w.deco.maxSize = minSize, maxSize
	if w.hwnd == 0 || w.deco.fullscreen || w.Maximized() {
		return
	}
	scale := w.Scale()
	width, height := float32(w.physicalWidth)/scale, float32(w.physicalHeight)/scale
	if cw, ch := w.clampSize(width, height); cw != width || ch != height {
		_ = w.setClientSize(cw, ch) //nolint:errcheck // a failure leaves a size the user can still change
	}
}

// clampSize returns a logical client size within the limits.
func (w *Window) clampSize(width, height float32) (float32, float32) {
	return clampAxis(width, w.deco.minSize.W, w.deco.maxSize.W), clampAxis(height, w.deco.minSize.H, w.deco.maxSize.H)
}

// clampAxis bounds v by lo and hi, where zero is no bound.
func clampAxis(v, lo, hi float32) float32 {
	if hi > 0 && v > hi {
		v = hi
	}
	if lo > 0 && v < lo {
		v = lo
	}
	return v
}

// outerSize returns the size of the window whose client area is the logical
// size width by height at the DPI of the window.
func (w *Window) outerSize(width, height float32) (int32, int32) {
	scale := w.Scale()
	r := RECT{right: int32(width*scale + 0.5), bottom: int32(height*scale + 0.5)}
	// r is only used during the call.
	_, _, _ = syscall.SyscallN(procAdjustWindowRectExForDpi.Addr(), uintptr(unsafe.Pointer(&r)), WS_OVERLAPPEDWINDOW, 0, 0, uintptr(w.dpi))
	if w.customFrame {
		r.top = 0
	}
	return r.right - r.left, r.bottom - r.top
}

// minMaxInfo answers WM_GETMINMAXINFO with the limits of the client area, and
// reports whether there were any. lParam points to a MINMAXINFO that the OS
// filled with its defaults.
func (w *Window) minMaxInfo(lParam uintptr) bool {
	lo, hi := w.deco.minSize, w.deco.maxSize
	if w.deco.fullscreen || lo == (geom.Size{}) && hi == (geom.Size{}) {
		return false
	}
	// Reading through &lParam turns the integer into a pointer without a
	// uintptr-to-pointer conversion. The OS owns the struct for this call.
	info := *(**MINMAXINFO)(unsafe.Pointer(&lParam))
	minWidth, minHeight := w.outerSize(lo.W, lo.H)
	maxWidth, maxHeight := w.outerSize(hi.W, hi.H)
	if lo.W > 0 {
		info.ptMinTrackSize.x = max(info.ptMinTrackSize.x, minWidth)
	}
	if lo.H > 0 {
		info.ptMinTrackSize.y = max(info.ptMinTrackSize.y, minHeight)
	}
	if hi.W > 0 {
		info.ptMaxTrackSize.x = max(maxWidth, info.ptMinTrackSize.x)
	}
	if hi.H > 0 {
		info.ptMaxTrackSize.y = max(maxHeight, info.ptMinTrackSize.y)
	}
	return true
}

// SetAlwaysOnTop implements platform.Window: HWND_TOPMOST.
func (w *Window) SetAlwaysOnTop(on bool) {
	w.deco.topmost = on
	if w.hwnd == 0 {
		return
	}
	after := HWND_NOTOPMOST
	if on {
		after = HWND_TOPMOST
	}
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), after, 0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
}

// Fullscreen implements platform.Window.
func (w *Window) Fullscreen() bool { return w.deco.fullscreen }

// Transparent implements platform.Window.
func (w *Window) Transparent() bool { return w.deco.transparent }

// SetFullscreen implements platform.Window, the way of most Windows programs:
// the frame comes off the style and the window covers the monitor, which the
// shell takes for a fullscreen window and hides the task bar for. Leaving
// restores the style and the placement from before, maximized or not.
func (w *Window) SetFullscreen(on bool) {
	if w.hwnd == 0 || on == w.deco.fullscreen {
		return
	}
	hwnd := uintptr(w.hwnd)
	if on {
		mon, _, _ := syscall.SyscallN(procMonitorFromWindow.Addr(), hwnd, MONITOR_DEFAULTTONEAREST)
		info := MONITORINFO{cbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
		w.deco.restore = WINDOWPLACEMENT{length: uint32(unsafe.Sizeof(WINDOWPLACEMENT{}))}
		// info and restore are only used during the calls.
		if ok, _, _ := syscall.SyscallN(procGetMonitorInfoW.Addr(), mon, uintptr(unsafe.Pointer(&info))); ok == 0 {
			return
		}
		if ok, _, _ := syscall.SyscallN(procGetWindowPlacement.Addr(), hwnd, uintptr(unsafe.Pointer(&w.deco.restore))); ok == 0 {
			return
		}
		style, _, _ := syscall.SyscallN(procGetWindowLongW.Addr(), hwnd, GWL_STYLE)
		w.deco.style = uint32(style)
		w.deco.fullscreen = true
		_, _, _ = syscall.SyscallN(procSetWindowLongW.Addr(), hwnd, GWL_STYLE, uintptr(uint32(style)&^WS_OVERLAPPEDWINDOW))
		m := info.rcMonitor
		_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), hwnd, HWND_TOP, uintptr(m.left), uintptr(m.top),
			uintptr(m.right-m.left), uintptr(m.bottom-m.top), SWP_NOOWNERZORDER|SWP_FRAMECHANGED)
	} else {
		w.deco.fullscreen = false
		_, _, _ = syscall.SyscallN(procSetWindowLongW.Addr(), hwnd, GWL_STYLE, uintptr(w.deco.style))
		// restore is only read during the call.
		_, _, _ = syscall.SyscallN(procSetWindowPlacement.Addr(), hwnd, uintptr(unsafe.Pointer(&w.deco.restore)))
		_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), hwnd, 0, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_NOOWNERZORDER|SWP_FRAMECHANGED|SWP_NOACTIVATE)
	}
	// The size may not change, as when the window was maximized on a
	// monitor without a task bar, but the application must hear of it.
	if err := w.refreshClientSize(); err == nil {
		w.platform.queue.Push(w.resizeEvent())
	}
}
