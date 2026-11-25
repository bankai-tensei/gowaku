package win32

import (
	"syscall"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// A window with CustomFrame keeps the borders of the OS, which resize it and
// cast the shadow of DWM, and gives the room of the caption to the client
// area. WM_NCHITTEST asks the application what each point of its title bar is.
// See docs/ADR/0048-custom-title-bar.md.

var (
	procGetSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi")
	procIsZoomed               = user32.NewProc("IsZoomed")
)

// The messages, hit codes, metrics and commands of a custom frame.
const (
	WM_NCCALCSIZE      = 0x0083
	WM_NCHITTEST       = 0x0084
	WM_NCMOUSEMOVE     = 0x00A0
	WM_NCLBUTTONDOWN   = 0x00A1
	WM_NCLBUTTONUP     = 0x00A2
	WM_NCLBUTTONDBLCLK = 0x00A3
	WM_NCMOUSELEAVE    = 0x02A2

	HTCAPTION     = 2
	HTMAXBUTTON   = 9
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	TME_NONCLIENT = 0x00000010

	SM_CXFRAME        = 32
	SM_CYFRAME        = 33
	SM_CXPADDEDBORDER = 92

	SW_MAXIMIZE      = 3
	SW_MINIMIZE      = 6
	SW_RESTORE       = 9
	SW_HIDE          = 0
	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE_      = 0x0002
)

// ClientDecorated implements platform.Window. Windows draws the title bar
// unless the window has a custom frame.
func (w *Window) ClientDecorated() bool { return w.customFrame }

// Maximized implements platform.Window.
func (w *Window) Maximized() bool {
	if w.hwnd == 0 {
		return false
	}
	r, _, _ := syscall.SyscallN(procIsZoomed.Addr(), uintptr(w.hwnd))
	return r != 0
}

// BeginMove implements platform.Window. The caption of a custom frame is
// HTCAPTION, which Windows moves by itself.
func (w *Window) BeginMove() {}

// ToggleMaximize implements platform.Window.
func (w *Window) ToggleMaximize() {
	if w.deco.fullscreen {
		return
	}
	cmd := uintptr(SW_MAXIMIZE)
	if w.Maximized() {
		cmd = SW_RESTORE
	}
	_, _, _ = syscall.SyscallN(procShowWindow.Addr(), uintptr(w.hwnd), cmd)
}

// Minimize implements platform.Window.
func (w *Window) Minimize() {
	_, _, _ = syscall.SyscallN(procShowWindow.Addr(), uintptr(w.hwnd), SW_MINIMIZE)
}

// SetVisible implements platform.Window: SW_HIDE takes the window and its
// button off the task bar, and SW_SHOW puts them back.
func (w *Window) SetVisible(visible bool) {
	if w.hwnd == 0 {
		return
	}
	if !visible {
		_, _, _ = syscall.SyscallN(procShowWindow.Addr(), uintptr(w.hwnd), SW_HIDE)
		return
	}
	_, _, _ = syscall.SyscallN(procShowWindow.Addr(), uintptr(w.hwnd), SW_SHOW)
	w.Activate()
}

// ShowWindowMenu implements platform.Window. A secondary click on HTCAPTION
// opens the system menu by itself.
func (w *Window) ShowWindowMenu(_, _ int) {}

// SetHitTest implements platform.Window.
func (w *Window) SetHitTest(fn func(physicalX, physicalY int) platform.HitArea) { w.hitTest = fn }

// metric returns a system metric at the DPI of the window.
func (w *Window) metric(index uintptr) int32 {
	r, _, _ := syscall.SyscallN(procGetSystemMetricsForDpi.Addr(), index, uintptr(w.dpi))
	return int32(r)
}

// frameThickness is the height of the resize border at the top, which the
// client area covers, and how far a maximized window reaches past the screen.
func (w *Window) frameThickness() int32 {
	return w.metric(SM_CYFRAME) + w.metric(SM_CXPADDEDBORDER)
}

// applyFrame makes Windows compute the frame again, after customFrame changed.
func (w *Window) applyFrame() {
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, 0, 0, 0, 0,
		SWP_FRAMECHANGED|SWP_NOMOVE_|SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// ncCalcSize answers WM_NCCALCSIZE for a custom frame: the default borders,
// but the client area goes up to the top of the window. lParam points to an
// NCCALCSIZE_PARAMS, whose first RECT is the one to change.
func (w *Window) ncCalcSize(hwnd, wParam, lParam uintptr) uintptr {
	// Reading through &lParam turns the integer into a pointer without a
	// uintptr-to-pointer conversion. The OS owns the struct for this call.
	r := *(**RECT)(unsafe.Pointer(&lParam))
	top := r.top
	defWindowProc(hwnd, WM_NCCALCSIZE, wParam, lParam)
	r.top = top
	if w.Maximized() {
		// A maximized window reaches past the screen by its frame.
		r.top += w.frameThickness()
	}
	return 0
}

// ncHitTest answers WM_NCHITTEST for a custom frame.
func (w *Window) ncHitTest(hwnd, lParam uintptr) uintptr {
	if r := defWindowProc(hwnd, WM_NCHITTEST, 0, lParam); r != HTCLIENT {
		// The borders and corners of the sides and the bottom.
		return r
	}
	if w.deco.fullscreen {
		// No frame and no title bar: all of it is the application's.
		return HTCLIENT
	}
	x, y := pointerPos(lParam)
	pt := POINT{int32(x), int32(y)}
	// pt is only used during the call.
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), hwnd, uintptr(unsafe.Pointer(&pt)))
	if !w.Maximized() && pt.y < w.frameThickness() {
		corner := w.metric(SM_CXFRAME) + w.metric(SM_CXPADDEDBORDER)
		switch {
		case pt.x < 2*corner:
			return HTTOPLEFT
		case pt.x >= int32(w.physicalWidth)-2*corner:
			return HTTOPRIGHT
		}
		return HTTOP
	}
	if w.hitTest != nil {
		switch w.hitTest(int(pt.x), int(pt.y)) {
		case platform.HitCaption:
			return HTCAPTION
		case platform.HitMaximize:
			return HTMAXBUTTON
		}
	}
	return HTCLIENT
}

// clientLParam turns the screen position of a non-client message into the
// lParam of a client one.
func (w *Window) clientLParam(lParam uintptr) uintptr {
	x, y := pointerPos(lParam)
	pt := POINT{int32(x), int32(y)}
	// pt is only used during the call.
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	return uintptr(uint16(int16(pt.x))) | uintptr(uint16(int16(pt.y)))<<16
}

// maxButton handles the non-client mouse messages over HTMAXBUTTON as client
// ones, so that the maximize button of the application hovers and presses
// while Windows offers its snap layouts. It reports whether it handled msg.
func (p *Platform) maxButton(w *Window, msg, wParam, lParam uintptr) bool {
	if !w.customFrame {
		return false
	}
	if msg == WM_NCMOUSELEAVE {
		w.ncTracking = false
		p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
		p.runModalHandler()
		return true
	}
	if wParam != HTMAXBUTTON {
		return false
	}
	switch msg {
	case WM_NCMOUSEMOVE:
		if !w.ncTracking {
			w.ncTracking = true
			track := TRACKMOUSEEVENT{dwFlags: TME_LEAVE | TME_NONCLIENT, hwndTrack: w.hwnd}
			track.cbSize = uint32(unsafe.Sizeof(track))
			// track is only read during the call.
			_, _, _ = syscall.SyscallN(procTrackMouseEvent.Addr(), uintptr(unsafe.Pointer(&track)))
		}
		p.pushPointer(w, platform.EventPointerMove, input.ButtonPrimary, w.clientLParam(lParam))
	case WM_NCLBUTTONDOWN, WM_NCLBUTTONDBLCLK:
		// The capture makes the release a client message wherever it happens.
		p.buttonDown(w, input.ButtonPrimary, w.clientLParam(lParam))
	case WM_NCLBUTTONUP:
		p.buttonUp(w, input.ButtonPrimary, w.clientLParam(lParam))
	default:
		return false
	}
	return true
}
