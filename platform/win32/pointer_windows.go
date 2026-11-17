package win32

import (
	"syscall"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// Touch screens and pens send WM_POINTER messages. Windows turns those it is
// not given back into mouse messages, so the ones of fingers and pens are taken
// here, with the pressure of the pen, and the rest go on as mouse messages.
// See docs/ADR/0052-pinch-touch-and-pen.md.

var procGetPointerPenInfo = user32.NewProc("GetPointerPenInfo")

// The values of pointer input.
const (
	WM_POINTERUPDATE = 0x0245
	WM_POINTERDOWN   = 0x0246
	WM_POINTERUP     = 0x0247
	PT_TOUCH         = 2
	PT_PEN           = 3
	// penPressureMax is the pressure of POINTER_PEN_INFO at full force.
	penPressureMax = 1024
)

// POINTER_INFO is the common part of a pointer.
type POINTER_INFO struct {
	pointerType           uint32
	pointerId             uint32
	frameId               uint32
	pointerFlags          uint32
	sourceDevice          uintptr
	hwndTarget            uintptr
	ptPixelLocation       POINT
	ptHimetricLocation    POINT
	ptPixelLocationRaw    POINT
	ptHimetricLocationRaw POINT
	dwTime                uint32
	historyCount          uint32
	InputData             int32
	dwKeyStates           uint32
	PerformanceCount      uint64
	ButtonChangeType      int32
}

// POINTER_PEN_INFO is a pen.
type POINTER_PEN_INFO struct {
	pointerInfo POINTER_INFO
	penFlags    uint32
	penMask     uint32
	pressure    uint32
	rotation    uint32
	tiltX       int32
	tiltY       int32
}

// pointerMessage takes a WM_POINTER message of a finger or a pen. It reports
// whether it did; the others become mouse messages.
func (p *Platform) pointerMessage(w *Window, msg, wParam, lParam uintptr) bool {
	id := wParam & 0xFFFF
	var kind uint32
	// kind is written during the call.
	if r, _, _ := syscall.SyscallN(procGetPointerType.Addr(), id, uintptr(unsafe.Pointer(&kind))); r == 0 {
		return false
	}
	var device input.PointerDevice
	switch kind {
	case PT_TOUCH:
		device = input.DeviceTouch
	case PT_PEN:
		device = input.DevicePen
	default:
		return false
	}
	ev := platform.Event{
		Window: w.id, Device: device, PointerID: uint32(id), Button: input.ButtonPrimary,
		Modifiers: modifiers(), Time: p.messageTime(),
	}
	switch msg {
	case WM_POINTERDOWN:
		ev.Kind = platform.EventPointerDown
	case WM_POINTERUP:
		ev.Kind = platform.EventPointerUp
	default:
		ev.Kind = platform.EventPointerMove
	}
	pt := POINT{int32(int16(lParam & 0xFFFF)), int32(int16(lParam >> 16 & 0xFFFF))}
	// pt is only used during the call.
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	ev.X, ev.Y = float32(pt.x), float32(pt.y)
	if device == input.DevicePen {
		var info POINTER_PEN_INFO
		// info is written during the call.
		if r, _, _ := syscall.SyscallN(procGetPointerPenInfo.Addr(), id, uintptr(unsafe.Pointer(&info))); r != 0 {
			ev.Pressure = float32(info.pressure) / penPressureMax
		}
	}
	p.queue.Push(ev)
	p.runModalHandler()
	return true
}
