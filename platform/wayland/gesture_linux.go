package wayland

import "github.com/bankai-tensei/gowaku/platform"

// Pinches on a touchpad come through zwp_pointer_gestures_v1, whose pinch
// reports the scale since it began; EventPinch carries the change since the
// last update. See docs/ADR/0052-pinch-touch-and-pen.md.

// pinchState is a pinch on a touchpad.
type pinchState struct {
	s      *seat
	proxy  zwpPointerGesturePinchV1
	window *Window
	scale  float64
}

// startPinch takes the pinches of the pointer of the seat, once both exist.
func (s *seat) startPinch() {
	p := s.p
	if p.gestures == 0 || s.pointer == 0 || s.pinch.proxy != 0 {
		return
	}
	s.pinch.s = s
	s.pinch.proxy = p.gestures.getPinchGesture(p.c, s.pointer)
	p.c.listenZwpPointerGesturePinchV1(s.pinch.proxy, &s.pinch)
}

// stopPinch lets go of the pinches, before the pointer goes.
func (s *seat) stopPinch() {
	if s.pinch.proxy != 0 {
		s.pinch.proxy.destroy(s.p.c)
		s.pinch = pinchState{}
	}
}

// zwpPointerGesturePinchV1Begin implements zwpPointerGesturePinchV1Listener.
func (g *pinchState) zwpPointerGesturePinchV1Begin(_, _ uint32, surface wlSurface, _ uint32) {
	g.window, g.scale = g.s.p.windows[surface], 1
}

// zwpPointerGesturePinchV1Update implements zwpPointerGesturePinchV1Listener.
func (g *pinchState) zwpPointerGesturePinchV1Update(time uint32, _, _, scale, _ float64) {
	w := g.window
	if w == nil || scale <= 0 || scale == g.scale {
		return
	}
	factor := scale / g.scale
	g.scale = scale
	s := g.s
	s.push(platform.Event{
		Kind: platform.EventPinch, Window: w.id, X: float32(s.pointerX * w.scale), Y: float32(s.pointerY * w.scale),
		Pinch: float32(factor), Modifiers: s.kb.modifiers(), Time: time,
	})
}

// zwpPointerGesturePinchV1End implements zwpPointerGesturePinchV1Listener.
func (g *pinchState) zwpPointerGesturePinchV1End(_, _ uint32, _ int32) { g.window = nil }
