package wayland

import (
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// Fingers come through wl_touch, and pens through the tablet protocol
// (tablet-unstable-v2), whose tools report their pressure from 0 to 65535 and
// group their events in frames. See docs/ADR/0052-pinch-touch-and-pen.md.

const (
	seatTouch = 4
	// tabletPressureMax is the pressure of a tool at full force.
	tabletPressureMax = 65535
)

// touchState is the fingers of the seat.
type touchState struct {
	wlTouchNop
	s     *seat
	proxy wlTouch
	// fingers are the windows that the fingers went down on, with where
	// they last were, by id.
	fingers map[int32]finger
}

type finger struct {
	w    *Window
	x, y float64
}

func (s *seat) startTouch() {
	if s.touch.proxy != 0 {
		return
	}
	s.touch = touchState{s: s, proxy: s.proxy.getTouch(s.p.c), fingers: map[int32]finger{}}
	s.p.c.listenWlTouch(s.touch.proxy, &s.touch)
}

func (s *seat) stopTouch() {
	if s.touch.proxy == 0 {
		return
	}
	if s.version >= wlTouchReleaseSince {
		s.touch.proxy.release(s.p.c)
	} else {
		s.p.c.destroy(proxy(s.touch.proxy))
	}
	s.touch = touchState{}
}

func (t *touchState) push(kind platform.EventKind, w *Window, id int32, x, y float64, stamp uint32) {
	t.s.push(platform.Event{
		Kind: kind, Window: w.id, X: float32(x * w.scale), Y: float32(y * w.scale), Device: input.DeviceTouch,
		PointerID: uint32(id), Button: input.ButtonPrimary, Modifiers: t.s.kb.modifiers(), Time: stamp,
	})
}

// wlTouchDown implements wlTouchListener.
func (t *touchState) wlTouchDown(serial, stamp uint32, surface wlSurface, id int32, x, y float64) {
	w := t.s.p.windows[surface]
	if w == nil {
		return
	}
	t.s.serial = serial
	t.fingers[id] = finger{w, x, y}
	t.push(platform.EventPointerDown, w, id, x, y, stamp)
}

// wlTouchMotion implements wlTouchListener.
func (t *touchState) wlTouchMotion(stamp uint32, id int32, x, y float64) {
	if f, ok := t.fingers[id]; ok {
		t.fingers[id] = finger{f.w, x, y}
		t.push(platform.EventPointerMove, f.w, id, x, y, stamp)
	}
}

// wlTouchUp implements wlTouchListener. The finger is where it last moved.
func (t *touchState) wlTouchUp(_, stamp uint32, id int32) {
	if f, ok := t.fingers[id]; ok {
		delete(t.fingers, id)
		t.push(platform.EventPointerUp, f.w, id, f.x, f.y, stamp)
	}
}

// wlTouchCancel implements wlTouchListener: the compositor took the fingers.
func (t *touchState) wlTouchCancel() {
	for id, f := range t.fingers {
		t.push(platform.EventPointerUp, f.w, id, f.x, f.y, 0)
	}
	clear(t.fingers)
}

// tabletState is the pens of the seat.
type tabletState struct {
	s     *seat
	proxy zwpTabletSeatV2
	tools []*tool
}

// tool is one pen, whose events wait for the frame.
type tool struct {
	zwpTabletToolV2Nop
	t        *tabletState
	proxy    zwpTabletToolV2
	window   *Window
	x, y     float64
	pressure float32
	down     bool
	// changed holds what happened since the last frame.
	moved, pressed, released bool
}

// startTablet asks for the pens of the seat, once both the seat and the
// tablet manager exist.
func (s *seat) startTablet() {
	p := s.p
	if p.tablets == 0 || s.proxy == 0 || s.tablet.proxy != 0 {
		return
	}
	s.tablet = tabletState{s: s, proxy: p.tablets.getTabletSeat(p.c, s.proxy)}
	p.c.listenZwpTabletSeatV2(s.tablet.proxy, &s.tablet)
}

// zwpTabletSeatV2TabletAdded implements zwpTabletSeatV2Listener. The tablet
// itself says nothing gowaku uses.
func (t *tabletState) zwpTabletSeatV2TabletAdded(id zwpTabletV2) {
	t.s.p.c.listenZwpTabletV2(id, &zwpTabletV2Nop{})
}

// zwpTabletSeatV2PadAdded implements zwpTabletSeatV2Listener.
func (t *tabletState) zwpTabletSeatV2PadAdded(id zwpTabletPadV2) {
	t.s.p.c.listenZwpTabletPadV2(id, &zwpTabletPadV2Nop{})
}

// zwpTabletSeatV2ToolAdded implements zwpTabletSeatV2Listener.
func (t *tabletState) zwpTabletSeatV2ToolAdded(id zwpTabletToolV2) {
	tl := &tool{t: t, proxy: id}
	t.tools = append(t.tools, tl)
	t.s.p.c.listenZwpTabletToolV2(id, tl)
}

// zwpTabletToolV2ProximityIn implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2ProximityIn(_ uint32, _ zwpTabletV2, surface wlSurface) {
	tl.window = tl.t.s.p.windows[surface]
}

// zwpTabletToolV2ProximityOut implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2ProximityOut() {
	if tl.down {
		tl.released = true
	}
}

// zwpTabletToolV2Down implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2Down(serial uint32) {
	tl.t.s.serial = serial
	tl.pressed = true
}

// zwpTabletToolV2Up implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2Up() { tl.released = true }

// zwpTabletToolV2Motion implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2Motion(x, y float64) { tl.x, tl.y, tl.moved = x, y, true }

// zwpTabletToolV2Pressure implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2Pressure(pressure uint32) {
	tl.pressure, tl.moved = float32(pressure)/tabletPressureMax, true
}

// zwpTabletToolV2Removed implements zwpTabletToolV2Listener.
func (tl *tool) zwpTabletToolV2Removed() {
	tl.proxy.destroy(tl.t.s.p.c)
	for i, o := range tl.t.tools {
		if o == tl {
			tl.t.tools = append(tl.t.tools[:i], tl.t.tools[i+1:]...)
			break
		}
	}
}

// zwpTabletToolV2Frame implements zwpTabletToolV2Listener: what happened since
// the last frame becomes pointer events.
func (tl *tool) zwpTabletToolV2Frame(stamp uint32) {
	w := tl.window
	if w == nil {
		tl.moved, tl.pressed, tl.released = false, false, false
		return
	}
	ev := platform.Event{
		Window: w.id, X: float32(tl.x * w.scale), Y: float32(tl.y * w.scale), Device: input.DevicePen,
		Button: input.ButtonPrimary, Pressure: tl.pressure, Modifiers: tl.t.s.kb.modifiers(), Time: stamp,
	}
	push := func(kind platform.EventKind) {
		ev.Kind = kind
		tl.t.s.push(ev)
	}
	if tl.moved {
		push(platform.EventPointerMove)
	}
	if tl.pressed && !tl.down {
		tl.down = true
		push(platform.EventPointerDown)
	}
	if tl.released && tl.down {
		tl.down = false
		ev.Pressure = 0
		push(platform.EventPointerUp)
	}
	tl.moved, tl.pressed, tl.released = false, false, false
}
