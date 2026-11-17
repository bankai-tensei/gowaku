//go:build gpu

package win32

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// This injects a pen into the system, at the middle of a window that it puts
// in front of all others for a moment, so it runs with the GPU tests, locally
// only.

var (
	user32t                           = windows.NewLazySystemDLL("user32.dll")
	procCreateSyntheticPointerDevice  = user32t.NewProc("CreateSyntheticPointerDevice")
	procInjectSyntheticPointerInput   = user32t.NewProc("InjectSyntheticPointerInput")
	procDestroySyntheticPointerDevice = user32t.NewProc("DestroySyntheticPointerDevice")
	procSetWindowPosT                 = user32t.NewProc("SetWindowPos")
	procClientToScreenT               = user32t.NewProc("ClientToScreen")
)

const (
	POINTER_FEEDBACK_NONE = 3
	POINTER_FLAG_INRANGE  = 0x00000002
	POINTER_FLAG_CONTACT  = 0x00000004
	POINTER_FLAG_FIRST    = 0x00000010
	POINTER_FLAG_DOWN     = 0x00010000
	POINTER_FLAG_UPDATE   = 0x00020000
	POINTER_FLAG_UP       = 0x00040000
	PEN_MASK_PRESSURE     = 0x00000001
)

// pointerTypeInfo is POINTER_TYPE_INFO with a pen in its union.
type pointerTypeInfo struct {
	kind uint32
	_    uint32
	pen  POINTER_PEN_INFO
	_    [24]byte
}

func TestPointer_InjectedPenWithPressure(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "pen", Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	// HWND_TOPMOST, SWP_NOMOVE | SWP_NOSIZE | SWP_SHOWWINDOW.
	procSetWindowPosT.Call(uintptr(w.hwnd), ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x40)
	defer procSetWindowPosT.Call(uintptr(w.hwnd), ^uintptr(1), 0, 0, 0, 0, 0x1|0x2) // HWND_NOTOPMOST
	device, _, _ := procCreateSyntheticPointerDevice.Call(PT_PEN, 1, POINTER_FEEDBACK_NONE)
	if device == 0 {
		t.Skip("no synthetic pointers")
	}
	defer procDestroySyntheticPointerDevice.Call(device)
	for p.WaitEvent(50*time.Millisecond).Kind != platform.EventNone {
	}

	center := POINT{int32(w.physicalWidth / 2), int32(w.physicalHeight / 2)}
	procClientToScreenT.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&center)))
	inject := func(flags uint32, dx int32, pressure uint32) {
		info := pointerTypeInfo{kind: PT_PEN}
		info.pen.pointerInfo.pointerType = PT_PEN
		info.pen.pointerInfo.pointerFlags = flags
		info.pen.pointerInfo.ptPixelLocation = POINT{center.x + dx, center.y}
		info.pen.penMask = PEN_MASK_PRESSURE
		info.pen.pressure = pressure
		if r, _, err := procInjectSyntheticPointerInput.Call(device, uintptr(unsafe.Pointer(&info)), 1); r == 0 {
			t.Fatalf("inject: %v", err)
		}
	}
	inject(POINTER_FLAG_INRANGE|POINTER_FLAG_UPDATE, 0, 0)
	inject(POINTER_FLAG_INRANGE|POINTER_FLAG_CONTACT|POINTER_FLAG_FIRST|POINTER_FLAG_DOWN, 0, 512)
	inject(POINTER_FLAG_INRANGE|POINTER_FLAG_CONTACT|POINTER_FLAG_FIRST|POINTER_FLAG_UPDATE, 10, 768)
	inject(POINTER_FLAG_INRANGE|POINTER_FLAG_UP, 10, 0)
	inject(POINTER_FLAG_UPDATE, 10, 0)

	var events []platform.Event
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		ev := p.WaitEvent(50 * time.Millisecond)
		if ev.Device == input.DevicePen {
			events = append(events, ev)
		}
		if ev.Kind == platform.EventPointerUp {
			break
		}
	}
	var down, move *platform.Event
	for i := range events {
		switch e := &events[i]; {
		case e.Kind == platform.EventPointerDown:
			down = e
		case e.Kind == platform.EventPointerMove && e.Pressure > 0.7:
			move = e
		}
	}
	if down == nil || down.Pressure != 0.5 {
		t.Fatalf("pen events %+v", events)
	}
	if move == nil || move.Pressure != 0.75 || move.X-down.X != 10 {
		t.Errorf("the move with more pressure: %+v", move)
	}
}
