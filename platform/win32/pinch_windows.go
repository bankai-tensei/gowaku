package win32

import (
	"log/slog"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
)

// Pinches on a precision touchpad come through DirectManipulation, as in Edge
// and Chrome: each window has a viewport that is given the contacts of the
// touchpad (DM_POINTERHITTEST) and configured for scaling only, so that
// scrolling keeps arriving as WM_MOUSEWHEEL. The scale of its content becomes
// EventPinch. Where DirectManipulation is missing, Windows sends a pinch as
// Ctrl+wheel, which core turns into a pinch. See
// docs/ADR/0052-pinch-touch-and-pen.md.

var (
	procGetPointerType = user32.NewProc("GetPointerType")
	procGetCursorPos   = user32.NewProc("GetCursorPos")
)

// The GUIDs of DirectManipulation.
var (
	CLSID_DirectManipulationManager = windows.GUID{Data1: 0x54E211B6, Data2: 0x3650, Data3: 0x4F75,
		Data4: [8]byte{0x83, 0x34, 0xFA, 0x35, 0x95, 0x98, 0xE1, 0xC5}}
	IID_IDirectManipulationManager = windows.GUID{Data1: 0xFBF5D3B4, Data2: 0x70C7, Data3: 0x4163,
		Data4: [8]byte{0x93, 0x22, 0x5A, 0x6F, 0x66, 0x0D, 0x6F, 0xBC}}
	IID_IDirectManipulationUpdateManager = windows.GUID{Data1: 0xB0AE62FD, Data2: 0xBE34, Data3: 0x46E7,
		Data4: [8]byte{0x9C, 0xAA, 0xD3, 0x61, 0xFA, 0xCB, 0xB9, 0xCC}}
	IID_IDirectManipulationViewport = windows.GUID{Data1: 0x28B85A3D, Data2: 0x60A0, Data3: 0x48BD,
		Data4: [8]byte{0x9B, 0xA1, 0x5C, 0xE8, 0xD9, 0xEA, 0x3A, 0x6D}}
	IID_IDirectManipulationContent = windows.GUID{Data1: 0xB89962CB, Data2: 0x3D89, Data3: 0x442B,
		Data4: [8]byte{0xBB, 0x58, 0x50, 0x98, 0xFA, 0x0F, 0x9F, 0x16}}
	IID_IDirectManipulationViewportEventHandler = windows.GUID{Data1: 0x952121DA, Data2: 0xD69F, Data3: 0x45F9,
		Data4: [8]byte{0xB0, 0xF9, 0xF2, 0x39, 0x44, 0x32, 0x1A, 0x6D}}
)

// The vtable slots and values of DirectManipulation, from directmanipulation.h.
const (
	iDMManager_Activate         = 3  // IDirectManipulationManager::Activate
	iDMManager_Deactivate       = 4  // IDirectManipulationManager::Deactivate
	iDMManager_GetUpdateManager = 7  // IDirectManipulationManager::GetUpdateManager
	iDMManager_CreateViewport   = 8  // IDirectManipulationManager::CreateViewport
	iDMUpdate_Update            = 5  // IDirectManipulationUpdateManager::Update
	iDMViewport_Enable          = 3  // IDirectManipulationViewport::Enable
	iDMViewport_SetContact      = 5  // IDirectManipulationViewport::SetContact
	iDMViewport_SetViewportRect = 12 // IDirectManipulationViewport::SetViewportRect
	iDMViewport_GetPrimary      = 16 // IDirectManipulationViewport::GetPrimaryContent
	iDMViewport_SetOptions      = 19 // IDirectManipulationViewport::SetViewportOptions
	iDMViewport_ActivateConfig  = 22 // IDirectManipulationViewport::ActivateConfiguration
	iDMViewport_AddEventHandler = 25 // IDirectManipulationViewport::AddEventHandler
	iDMViewport_Abandon         = 30 // IDirectManipulationViewport::Abandon
	iDMContent_GetTransform     = 9  // IDirectManipulationContent::GetContentTransform
	iDMContent_SyncTransform    = 10 // IDirectManipulationContent::SyncContentTransform

	DIRECTMANIPULATION_CONFIGURATION_INTERACTION = 0x1
	DIRECTMANIPULATION_CONFIGURATION_SCALING     = 0x10
	DIRECTMANIPULATION_VIEWPORT_OPTIONS_MANUAL   = 0x4
	DIRECTMANIPULATION_RUNNING                   = 3
	DIRECTMANIPULATION_INERTIA                   = 4
	DIRECTMANIPULATION_READY                     = 5
	DM_POINTERHITTEST                            = 0x0250
	PT_TOUCHPAD                                  = 5

	// pinchTimerID drives the updates of DirectManipulation while a pinch
	// runs, at the pace of frames.
	pinchTimerID     = 2
	pinchTimerPeriod = 16
	// pinchQuantum ignores changes of scale too small to matter.
	pinchQuantum = 1e-4
)

// pinch is the DirectManipulation of a window.
type pinch struct {
	w        *Window
	manager  *com.Unknown
	update   *com.Unknown
	viewport *com.Unknown
	content  *com.Unknown
	handler  *com.Unknown
	// scale is the scale of the content at the last update, which starts at
	// 1 with each pinch.
	scale   float32
	running bool
}

var pinchSetup = sync.OnceValue(func() *com.Interface {
	cb := syscall.NewCallback
	return com.NewInterface(IID_IDirectManipulationViewportEventHandler,
		cb(pinchStatusChanged), cb(pinchViewportUpdated), cb(pinchContentUpdated))
})

// startPinch gives the window a viewport of DirectManipulation. Without one,
// pinches arrive as Ctrl+wheel.
func (w *Window) startPinch() {
	if !w.platform.ole {
		return
	}
	var manager *com.Unknown
	// manager is written during the call.
	if hr, _, _ := syscall.SyscallN(procCoCreateInstance.Addr(), uintptr(unsafe.Pointer(&CLSID_DirectManipulationManager)), 0,
		CLSCTX_INPROC_SERVER, uintptr(unsafe.Pointer(&IID_IDirectManipulationManager)),
		uintptr(unsafe.Pointer(&manager))); com.Failed(hr) || manager == nil {
		slog.Debug("win32: no DirectManipulation", "hr", hr)
		return
	}
	pc := &pinch{w: w, manager: manager, scale: 1}
	ok := pc.create()
	if !ok {
		pc.release()
		return
	}
	w.pinch = pc
}

// create builds the viewport, and reports whether everything worked.
func (pc *pinch) create() bool {
	hwnd := uintptr(pc.w.hwnd)
	// Each out pointer is written during its call.
	if com.Failed(pc.manager.Call(iDMManager_GetUpdateManager, uintptr(unsafe.Pointer(&IID_IDirectManipulationUpdateManager)),
		uintptr(unsafe.Pointer(&pc.update)))) {
		return false
	}
	if com.Failed(pc.manager.Call(iDMManager_CreateViewport, 0, hwnd, uintptr(unsafe.Pointer(&IID_IDirectManipulationViewport)),
		uintptr(unsafe.Pointer(&pc.viewport)))) {
		return false
	}
	v := pc.viewport
	if com.Failed(v.Call(iDMViewport_ActivateConfig,
		DIRECTMANIPULATION_CONFIGURATION_INTERACTION|DIRECTMANIPULATION_CONFIGURATION_SCALING)) {
		return false
	}
	if com.Failed(v.Call(iDMViewport_SetOptions, DIRECTMANIPULATION_VIEWPORT_OPTIONS_MANUAL)) {
		return false
	}
	pc.handler = com.New(pc, pinchSetup())
	var cookie uint32
	if com.Failed(v.Call(iDMViewport_AddEventHandler, hwnd, pc.handler.Addr(), uintptr(unsafe.Pointer(&cookie)))) {
		return false
	}
	if com.Failed(v.Call(iDMViewport_GetPrimary, uintptr(unsafe.Pointer(&IID_IDirectManipulationContent)),
		uintptr(unsafe.Pointer(&pc.content)))) {
		return false
	}
	pc.resize()
	return !com.Failed(pc.manager.Call(iDMManager_Activate, hwnd)) && !com.Failed(v.Call(iDMViewport_Enable))
}

// resize gives the viewport the size of the client area.
func (pc *pinch) resize() {
	r := RECT{right: int32(max(pc.w.physicalWidth, 1)), bottom: int32(max(pc.w.physicalHeight, 1))}
	// r is only read during the call.
	pc.viewport.Call(iDMViewport_SetViewportRect, uintptr(unsafe.Pointer(&r)))
}

// release frees what create made.
func (pc *pinch) release() {
	if pc.viewport != nil {
		pc.viewport.Call(iDMViewport_Abandon)
	}
	if pc.manager != nil && pc.w.hwnd != 0 {
		pc.manager.Call(iDMManager_Deactivate, uintptr(pc.w.hwnd))
	}
	for _, u := range []*com.Unknown{pc.content, pc.viewport, pc.update, pc.handler, pc.manager} {
		u.Release()
	}
	*pc = pinch{}
}

// stopPinch releases the viewport, before the window goes.
func (w *Window) stopPinch() {
	if w.pinch != nil {
		w.pinch.release()
		w.pinch = nil
	}
}

// pointerHitTest gives a contact of the touchpad to DirectManipulation. It
// reports whether it did.
func (w *Window) pointerHitTest(wParam uintptr) bool {
	if w.pinch == nil {
		return false
	}
	id := wParam & 0xFFFF
	var kind uint32
	// kind is written during the call.
	if r, _, _ := syscall.SyscallN(procGetPointerType.Addr(), id, uintptr(unsafe.Pointer(&kind))); r == 0 || kind != PT_TOUCHPAD {
		return false
	}
	w.pinch.viewport.Call(iDMViewport_SetContact, id)
	return true
}

// pinchTick runs the updates of DirectManipulation, which call the handler.
func (w *Window) pinchTick() {
	if w.pinch != nil {
		w.pinch.update.Call(iDMUpdate_Update, 0)
	}
}

func pinchOf(this *com.Unknown) (*pinch, bool) { return com.Get[*pinch](this) }

// pinchStatusChanged is IDirectManipulationViewportEventHandler::
// OnViewportStatusChanged: a pinch starts or ends.
func pinchStatusChanged(this, _ *com.Unknown, current, _ uintptr) uintptr {
	pc, ok := pinchOf(this)
	if !ok || pc.w == nil || pc.w.hwnd == 0 {
		return com.S_OK
	}
	hwnd := uintptr(pc.w.hwnd)
	switch current {
	case DIRECTMANIPULATION_RUNNING, DIRECTMANIPULATION_INERTIA:
		if !pc.running {
			pc.running = true
			_, _, _ = syscall.SyscallN(procSetTimer.Addr(), hwnd, pinchTimerID, pinchTimerPeriod, 0)
		}
	case DIRECTMANIPULATION_READY:
		pc.running = false
		_, _, _ = syscall.SyscallN(procKillTimer.Addr(), hwnd, pinchTimerID)
		// The next pinch starts from a scale of 1.
		identity := [6]float32{1, 0, 0, 1, 0, 0}
		// identity is only read during the call.
		pc.content.Call(iDMContent_SyncTransform, uintptr(unsafe.Pointer(&identity[0])), 6)
		pc.scale = 1
	}
	return com.S_OK
}

// pinchViewportUpdated is IDirectManipulationViewportEventHandler::
// OnViewportUpdated.
func pinchViewportUpdated(*com.Unknown, *com.Unknown) uintptr { return com.S_OK }

// pinchContentUpdated is IDirectManipulationViewportEventHandler::
// OnContentUpdated: the scale of the content changed.
func pinchContentUpdated(this, _, content *com.Unknown) uintptr {
	pc, ok := pinchOf(this)
	if !ok || pc.w == nil || pc.w.hwnd == 0 || content == nil {
		return com.S_OK
	}
	var m [6]float32
	// m is written during the call.
	if com.Failed(content.Call(iDMContent_GetTransform, uintptr(unsafe.Pointer(&m[0])), 6)) || m[0] <= 0 {
		return com.S_OK
	}
	scale := m[0]
	if math.Abs(float64(scale-pc.scale)) < pinchQuantum {
		return com.S_OK
	}
	factor := scale / pc.scale
	pc.scale = scale
	w := pc.w
	var pt POINT
	// pt is only used during the calls.
	_, _, _ = syscall.SyscallN(procGetCursorPos.Addr(), uintptr(unsafe.Pointer(&pt)))
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	w.platform.queue.Push(platform.Event{
		Kind: platform.EventPinch, Window: w.id, X: float32(pt.x), Y: float32(pt.y), Pinch: factor,
		Modifiers: modifiers(), Time: w.platform.messageTime(),
	})
	w.platform.runModalHandler()
	return com.S_OK
}
