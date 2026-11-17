package win32

import (
	"syscall"
	"unsafe"
)

var (
	procWindowFromPoint = user32.NewProc("WindowFromPoint")
	procGetAncestor     = user32.NewProc("GetAncestor")
)

// GA_ROOT asks GetAncestor for the top-level window.
const GA_ROOT = 2

// ClientOrigin implements platform.Placement.
func (w *Window) ClientOrigin() (physicalX, physicalY int, ok bool) {
	if w.hwnd == 0 {
		return 0, 0, false
	}
	var pt POINT
	// pt is only used during the call.
	if r, _, _ := syscall.SyscallN(procClientToScreen.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt))); r == 0 {
		return 0, 0, false
	}
	return int(pt.x), int(pt.y), true
}

// MoveClientTo implements platform.Placement. The frame around the client area
// is whatever it is now, which also holds for a window not shown yet.
func (w *Window) MoveClientTo(physicalX, physicalY int) {
	if w.hwnd == 0 {
		return
	}
	var r RECT
	// r is only used during the call.
	if ok, _, _ := syscall.SyscallN(procGetWindowRect.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	cx, cy, ok := w.ClientOrigin()
	if !ok {
		return
	}
	x, y := physicalX-(cx-int(r.left)), physicalY-(cy-int(r.top))
	_, _, _ = syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// ShowsScreenPoint implements platform.Placement.
func (w *Window) ShowsScreenPoint(physicalX, physicalY int) bool {
	if w.hwnd == 0 {
		return false
	}
	cx, cy, ok := w.ClientOrigin()
	if !ok || physicalX < cx || physicalY < cy || physicalX >= cx+w.physicalWidth || physicalY >= cy+w.physicalHeight {
		return false
	}
	// WindowFromPoint takes the POINT by value, which the 64-bit calling
	// conventions pass in one register, x in the low half.
	pt := uintptr(uint64(uint32(int32(physicalX))) | uint64(uint32(int32(physicalY)))<<32)
	h, _, _ := syscall.SyscallN(procWindowFromPoint.Addr(), pt)
	if h == 0 {
		return false
	}
	root, _, _ := syscall.SyscallN(procGetAncestor.Addr(), h, GA_ROOT)
	return HWND(root) == w.hwnd
}

// place moves a window not shown yet to where its client area starts at x, y,
// and sizes it again for the DPI of the monitor there, which the OS does not
// report for a hidden window.
func (w *Window) place(x, y int, width, height float32) error {
	w.MoveClientTo(x, y)
	dpi, _, _ := procGetDpiForWindow.Call(uintptr(w.hwnd))
	if uint32(dpi) == w.dpi || dpi == 0 {
		return nil
	}
	w.dpi = uint32(dpi)
	if err := w.setClientSize(width, height); err != nil {
		return err
	}
	w.MoveClientTo(x, y)
	return nil
}
