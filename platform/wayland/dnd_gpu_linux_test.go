//go:build gpu

package wayland

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// press presses or releases the primary button of v.
func (v *virtualPointer) press(t *testing.T, down bool) {
	state := uint32(0)
	if down {
		state = 1
	}
	v.pointer.button(v.c, v.stamp(), btnLeft, state)
	v.pointer.frame(v.c)
	if err := v.c.roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// A real drag through the compositor, from a small window to a large one under
// it, driven by a virtual pointer. It needs a wlroots compositor (sway.sh),
// which centers floating windows in its 1280×800 output.
func TestDrag_ThroughTheCompositor(t *testing.T) {
	v := newVirtualPointer(t)
	p := newTestPlatform(t)
	big, err := p.CreateWindow(platform.WindowOptions{Title: "target", Width: 1200, Height: 760})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, big)
	small, err := p.CreateWindow(platform.WindowOptions{Title: "source", Width: 200, Height: 150})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, small)
	drain(p, nil)
	var events []platform.DropEvent
	big.SetDropHandler(func(e *platform.DropEvent) input.DropEffect {
		events = append(events, *e)
		return input.DropMove
	})
	small.SetDropHandler(func(*platform.DropEvent) input.DropEffect { return input.DropNone })

	v.moveTo(t, 640, 400, 1280, 800)
	v.press(t, true)
	if !drain(p, func(ev *platform.Event) bool {
		return ev.Kind == platform.EventPointerDown && ev.Window == small.ID()
	}) {
		t.Skip("the press did not reach the small window; is this sway with its windows centered?")
	}
	ended := input.DropEffect(255)
	small.StartDrag(input.DragData{Text: "和文", Files: []string{"/tmp/メモ.txt"}}, input.DropCopy|input.DropMove,
		func(e input.DropEffect) { ended = e })
	drain(p, nil)
	v.moveTo(t, 100, 100, 1280, 800)
	drain(p, func(*platform.Event) bool { return len(events) > 0 })
	v.moveTo(t, 120, 110, 1280, 800)
	drain(p, nil)
	v.press(t, false)
	drain(p, func(*platform.Event) bool { return ended != 255 })

	if ended != input.DropMove {
		t.Errorf("the drag ended with %v, want a move", ended)
	}
	if len(events) < 2 {
		t.Fatalf("events %+v", events)
	}
	drop := events[len(events)-1]
	if drop.Phase != platform.DropDrop || drop.Data.Text != "和文" || !slices.Equal(drop.Data.Files, []string{"/tmp/メモ.txt"}) {
		t.Errorf("drop %+v", drop)
	}
	if events[0].Kinds != input.DataText|input.DataFiles || events[0].Allowed != input.DropCopy|input.DropMove {
		t.Errorf("enter %+v", events[0])
	}
	t.Logf("%d events, the first at %.0f,%.0f", len(events), events[0].X, events[0].Y)
}

// A virtual pointer makes no gestures, so the pinch object is checked to
// exist and is fed what the compositor would send.
func TestPinch_GesturesOfThePointer(t *testing.T) {
	newVirtualPointer(t)
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{Title: "pinch"})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, w)
	drain(p, nil)
	if p.gestures == 0 || p.seat.pinch.proxy == 0 {
		t.Skip("the compositor has no pointer gestures, or the seat no pointer")
	}
	g := &p.seat.pinch
	g.zwpPointerGesturePinchV1Begin(1, 0, w.(*Window).surface, 2)
	g.zwpPointerGesturePinchV1Update(1, 0, 0, 1.5, 0)
	g.zwpPointerGesturePinchV1Update(2, 0, 0, 3, 0)
	g.zwpPointerGesturePinchV1End(2, 3, 0)
	var factors []float32
	drain(p, func(ev *platform.Event) bool {
		if ev.Kind == platform.EventPinch {
			factors = append(factors, ev.Pinch)
		}
		return len(factors) == 2
	})
	if len(factors) != 2 || factors[0] != 1.5 || factors[1] != 2 {
		t.Errorf("factors %v, want [1.5 2]", factors)
	}
}

// Headless compositors have no fingers or pens, so the listeners are fed what
// a compositor would send.
func TestTouchAndPen_BecomePointerEvents(t *testing.T) {
	p := newTestPlatform(t)
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "touch"})
	if err != nil {
		t.Fatal(err)
	}
	drain(p, nil)
	w := pw.(*Window)
	touch := &touchState{s: &p.seat, fingers: map[int32]finger{}}
	touch.wlTouchDown(1, 10, w.surface, 7, 20, 30)
	touch.wlTouchMotion(11, 7, 25, 30)
	touch.wlTouchUp(2, 12, 7)
	pen := &tool{t: &tabletState{s: &p.seat}}
	pen.zwpTabletToolV2ProximityIn(1, 0, w.surface)
	pen.zwpTabletToolV2Motion(5, 6)
	pen.zwpTabletToolV2Pressure(tabletPressureMax / 2)
	pen.zwpTabletToolV2Down(3)
	pen.zwpTabletToolV2Frame(20)
	pen.zwpTabletToolV2Up()
	pen.zwpTabletToolV2Frame(21)

	var got []platform.Event
	drain(p, func(ev *platform.Event) bool {
		if ev.Device != input.DeviceMouse {
			got = append(got, *ev)
		}
		return len(got) == 6
	})
	want := []struct {
		kind   platform.EventKind
		device input.PointerDevice
		x      float32
	}{
		{platform.EventPointerDown, input.DeviceTouch, 20}, {platform.EventPointerMove, input.DeviceTouch, 25},
		{platform.EventPointerUp, input.DeviceTouch, 25}, {platform.EventPointerMove, input.DevicePen, 5},
		{platform.EventPointerDown, input.DevicePen, 5}, {platform.EventPointerUp, input.DevicePen, 5},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, wv := range want {
		if got[i].Kind != wv.kind || got[i].Device != wv.device || got[i].X != wv.x*float32(w.scale) {
			t.Errorf("event %d: %+v, want %v", i, got[i], wv)
		}
	}
	if got[0].PointerID != 7 || got[4].Pressure < 0.49 || got[4].Pressure > 0.51 {
		t.Errorf("id %d, pressure %v", got[0].PointerID, got[4].Pressure)
	}
}
