package win32

import (
	"syscall"
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
)

var (
	procEnableWindow        = user32.NewProc("EnableWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
)

// MONITORINFO is the struct of GetMonitorInfoW.
type MONITORINFO struct {
	cbSize    uint32
	rcMonitor RECT
	rcWork    RECT
	dwFlags   uint32
}

// The style, monitor and position flags of owned windows.
const (
	WS_MINIMIZEBOX           = 0x00020000
	MONITOR_DEFAULTTONEAREST = 0x00000002
	SWP_NOSIZE               = 0x0001
)

// ownerOf returns the HWND of the owner in opts, or zero.
func ownerOf(opts *platform.WindowOptions) HWND {
	if o, ok := opts.Owner.(*Window); ok && o != nil {
		return o.hwnd
	}
	return 0
}

// Activate implements platform.Window.
func (w *Window) Activate() {
	if w.hwnd != 0 {
		_, _, _ = syscall.SyscallN(procSetForegroundWindow.Addr(), uintptr(w.hwnd))
	}
}

// centerOver moves the window, not shown yet, to the middle of owner, inside
// the work area of the monitor of owner.
func (w *Window) centerOver(owner HWND) {
	var o, r RECT
	// o and r are only used during the calls.
	if ok, _, _ := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(owner), uintptr(unsafe.Pointer(&o))); ok == 0 {
		return
	}
	if ok, _, _ := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	width, height := r.right-r.left, r.bottom-r.top
	x := o.left + (o.right-o.left-width)/2
	y := o.top + (o.bottom-o.top-height)/2
	if a, ok := workAreaOf(owner); ok {
		x = max(a.left, min(x, a.right-width))
		y = max(a.top, min(y, a.bottom-height))
	}
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// workAreaOf returns the work area, the screen less the task bar, of the
// monitor that hwnd is on or nearest to, in physical pixels.
func workAreaOf(hwnd HWND) (RECT, bool) {
	mon, _, _ := syscall.SyscallN(procMonitorFromWindow.Addr(), uintptr(hwnd), MONITOR_DEFAULTTONEAREST)
	info := MONITORINFO{cbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
	// info is only used during the call.
	if ok, _, _ := syscall.SyscallN(procGetMonitorInfoW.Addr(), mon, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return RECT{}, false
	}
	return info.rcWork, true
}

// fitInWorkArea moves the window, not shown yet, so that it does not reach
// past the work area of its monitor. The position the OS picked by default
// cascades down and to the right regardless of the size of the window.
func (w *Window) fitInWorkArea() {
	var r RECT
	// r is only used during the call.
	if ok, _, _ := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	a, ok := workAreaOf(w.hwnd)
	if !ok {
		return
	}
	width, height := r.right-r.left, r.bottom-r.top
	x := max(a.left, min(r.left, a.right-width))
	y := max(a.top, min(r.top, a.bottom-height))
	if x == r.left && y == r.top {
		return
	}
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// enableOwner gives the input back to the owner of a modal window. It must run
// before the modal window goes, or Windows activates some other application.
func (w *Window) enableOwner() {
	if w.disabledOwner != 0 {
		_, _, _ = syscall.SyscallN(procEnableWindow.Addr(), uintptr(w.disabledOwner), 1)
		w.disabledOwner = 0
	}
}
