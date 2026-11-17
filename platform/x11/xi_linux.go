package x11

import (
	"log/slog"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// Pinches on a touchpad are the gesture events of XInput 2.4, which libinput
// sends to X servers and XWayland. libXi is optional: without it, or with an
// older server, windows get no pinches. See docs/ADR/0052-pinch-touch-and-pen.md.

// The values of XInput 2.
const (
	GenericEvent          = 35
	XIAllMasterDevices    = 1
	XI_TouchBegin         = 18
	XI_TouchUpdate        = 19
	XI_TouchEnd           = 20
	XI_GesturePinchBegin  = 27
	XI_GesturePinchUpdate = 28
	XI_GesturePinchEnd    = 29
)

// XGenericEventCookie is the head of a GenericEvent, whose data XGetEventData
// fetches.
type XGenericEventCookie struct {
	Type      int32
	Serial    uintptr
	SendEvent int32
	Display   Display
	Extension int32
	Evtype    int32
	Cookie    uint32
	Data      uintptr
}

// XIEventMask selects the events of XInput 2 of a device.
type XIEventMask struct {
	Deviceid int32
	MaskLen  int32
	Mask     *byte
}

// XIGesturePinchEvent is a gesture event of XInput 2.4.
type XIGesturePinchEvent struct {
	Type      int32
	Serial    uintptr
	SendEvent int32
	Display   Display
	Extension int32
	Evtype    int32
	Time      uintptr
	Deviceid  int32
	Sourceid  int32
	Detail    int32
	Root      XID
	Event     XID
	Child     XID
	RootX     float64
	RootY     float64
	EventX    float64
	EventY    float64
	DeltaX    float64
	DeltaY    float64
	DeltaUX   float64
	DeltaUY   float64
	Scale     float64
	Angle     float64
	Flags     int32
	Mods      [4]int32
	Group     [4]int32
}

// xi holds the functions of XInput 2.
type xi struct {
	XIQueryVersion, XISelectEvents                 uintptr
	XQueryExtension, XGetEventData, XFreeEventData uintptr
}

var openXi = sync.OnceValues(func() (uintptr, error) {
	return purego.Dlopen("libXi.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
})

// initXi finds XInput 2.4, and reports whether the server has it.
func (p *Platform) initXi() {
	lib, err := openXi()
	if err != nil {
		slog.Debug("x11: no libXi, so no pinches", "err", err)
		return
	}
	libs, err := openLibraries()
	if err != nil {
		return
	}
	for _, s := range []struct {
		lib  uintptr
		name string
		proc *uintptr
	}{
		{lib, "XIQueryVersion", &p.xi.XIQueryVersion}, {lib, "XISelectEvents", &p.xi.XISelectEvents},
		{libs.x11, "XQueryExtension", &p.xi.XQueryExtension}, {libs.x11, "XGetEventData", &p.xi.XGetEventData},
		{libs.x11, "XFreeEventData", &p.xi.XFreeEventData},
	} {
		if *s.proc, err = purego.Dlsym(s.lib, s.name); err != nil {
			slog.Debug("x11: libXi lacks a function", "name", s.name)
			return
		}
	}
	var opcode, event, xerror int32
	if call(p.xi.XQueryExtension, uintptr(p.display), ptr(cstring("XInputExtension")), ptr(&opcode), ptr(&event),
		ptr(&xerror)) == 0 {
		return
	}
	major, minor := int32(2), int32(4)
	// Success is 0; the server answers with what it has.
	if call(p.xi.XIQueryVersion, uintptr(p.display), ptr(&major), ptr(&minor)) != 0 || major < 2 || (major == 2 && minor < 2) {
		slog.Debug("x11: the server has no XInput 2.2, so no touches or pinches", "major", major, "minor", minor)
		return
	}
	p.xiOpcode, p.xiMinor = opcode, minor
}

// selectPinches asks the server for the pinches over w.
func (w *Window) selectPinches() {
	p := w.platform
	if p.xiOpcode == 0 {
		return
	}
	mask := [4]byte{}
	events := []int{XI_TouchBegin, XI_TouchUpdate, XI_TouchEnd}
	if p.xiMinor >= 4 {
		events = append(events, XI_GesturePinchBegin, XI_GesturePinchUpdate, XI_GesturePinchEnd)
	}
	for _, e := range events {
		mask[e/8] |= 1 << (e % 8)
	}
	m := XIEventMask{Deviceid: XIAllMasterDevices, MaskLen: int32(len(mask)), Mask: &mask[0]}
	call(p.xi.XISelectEvents, uintptr(p.display), uintptr(w.xid), ptr(&m), 1)
}

// genericEvent handles an event of an extension: the pinches of XInput 2.
func (p *Platform) genericEvent(ev *XEvent) {
	cookie := as[XGenericEventCookie](ev)
	if p.xiOpcode == 0 || cookie.Extension != p.xiOpcode {
		return
	}
	if call(p.xi.XGetEventData, uintptr(p.display), ptr(cookie)) == 0 {
		return
	}
	defer call(p.xi.XFreeEventData, uintptr(p.display), ptr(cookie))
	if cookie.Data == 0 {
		return
	}
	// Reading through &cookie.Data turns the integer into a pointer without a
	// uintptr-to-pointer conversion. Xlib owns the data until XFreeEventData.
	g := *(**XIGesturePinchEvent)(unsafe.Pointer(&cookie.Data))
	w := p.windows[g.Event]
	if w == nil {
		return
	}
	switch cookie.Evtype {
	case XI_TouchBegin, XI_TouchUpdate, XI_TouchEnd:
		// A touch event shares its head with a pinch: detail is the touch.
		kind := platform.EventPointerMove
		switch cookie.Evtype {
		case XI_TouchBegin:
			kind = platform.EventPointerDown
		case XI_TouchEnd:
			kind = platform.EventPointerUp
		}
		p.queue.Push(platform.Event{
			Kind: kind, Window: w.id, X: float32(g.EventX), Y: float32(g.EventY), Device: input.DeviceTouch,
			PointerID: uint32(g.Detail), Button: input.ButtonPrimary, Time: uint32(g.Time),
		})
	case XI_GesturePinchBegin:
		w.pinchScale = 1
	case XI_GesturePinchUpdate:
		if g.Scale <= 0 || g.Scale == w.pinchScale || w.pinchScale <= 0 {
			return
		}
		factor := g.Scale / w.pinchScale
		w.pinchScale = g.Scale
		p.queue.Push(platform.Event{
			Kind: platform.EventPinch, Window: w.id, X: float32(g.EventX), Y: float32(g.EventY),
			Pinch: float32(factor), Modifiers: modifiersOf(uint32(g.Mods[3])), Time: uint32(g.Time),
		})
	case XI_GesturePinchEnd:
		w.pinchScale = 0
	}
}
