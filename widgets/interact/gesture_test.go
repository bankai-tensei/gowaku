package interact_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
)

// gestureLog records what a GestureDetector reports.
type gestureLog struct {
	taps, doubles, cancels, starts, ends int
	start, end                           geom.Offset
	moved                                geom.Offset
	lastPos                              geom.Offset
}

// detector returns a GestureDetector of 100 by 100 at the top left that
// records into l; drags and double taps only when asked.
func (l *gestureLog) detector(drag, double bool) interact.GestureDetector {
	g := interact.GestureDetector{
		OnTap:       func() { l.taps++ },
		OnTapCancel: func() { l.cancels++ },
		Child:       basic.SizedBox{Width: 100, Height: 100},
	}
	if double {
		g.OnDoubleTap = func() { l.doubles++ }
	}
	if drag {
		g.OnDragStart = func(pos geom.Offset) { l.starts++; l.start = pos }
		g.OnDragUpdate = func(pos, delta geom.Offset) { l.moved = l.moved.Add(delta); l.lastPos = pos }
		g.OnDragEnd = func(pos geom.Offset) { l.ends++; l.end = pos }
	}
	return g
}

func TestGestureDetector_DragIsNoTap(t *testing.T) {
	var l gestureLog
	tt := gowakutest.New(t, basic.Align{Child: l.detector(true, false)}, pressableFonts)
	tt.DragAt(geom.Offset{X: 20, Y: 20}, geom.Offset{X: 80, Y: 50})
	if l.taps != 0 {
		t.Errorf("a drag ran OnTap %d times", l.taps)
	}
	if l.starts != 1 || l.ends != 1 || l.cancels != 1 {
		t.Errorf("starts %d, ends %d, cancels %d; want 1 each", l.starts, l.ends, l.cancels)
	}
	if l.start != (geom.Offset{X: 20, Y: 20}) {
		t.Errorf("start at %v, want the press", l.start)
	}
	if l.moved != (geom.Offset{X: 60, Y: 30}) || l.lastPos != (geom.Offset{X: 80, Y: 50}) || l.end != l.lastPos {
		t.Errorf("moved %v to %v, ended at %v; want 60,30 to 80,50", l.moved, l.lastPos, l.end)
	}
}

func TestGestureDetector_SmallMoveIsATap(t *testing.T) {
	var l gestureLog
	tt := gowakutest.New(t, basic.Align{Child: l.detector(true, false)}, pressableFonts)
	tt.DragAt(geom.Offset{X: 20, Y: 20}, geom.Offset{X: 25, Y: 24})
	if l.taps != 1 || l.starts != 0 || l.ends != 0 {
		t.Errorf("taps %d, starts %d, ends %d; want a tap and no drag", l.taps, l.starts, l.ends)
	}
}

func TestGestureDetector_DragLeavesTheChild(t *testing.T) {
	var l gestureLog
	tt := gowakutest.New(t, basic.Align{Child: l.detector(true, false)}, pressableFonts)
	tt.DragAt(geom.Offset{X: 50, Y: 50}, geom.Offset{X: 300, Y: 250})
	if l.ends != 1 || l.end != (geom.Offset{X: 300, Y: 250}) || l.cancels != 1 {
		t.Errorf("ends %d at %v, cancels %d; want the drag to follow outside", l.ends, l.end, l.cancels)
	}
}

func TestGestureDetector_DoubleTap(t *testing.T) {
	var l gestureLog
	tt := gowakutest.New(t, basic.Align{Child: l.detector(false, true)}, pressableFonts)
	at := geom.Offset{X: 30, Y: 30}
	tt.DoubleTapAt(at)
	if l.taps != 1 || l.doubles != 1 {
		t.Errorf("taps %d, double taps %d; want the first press a tap, the second a double tap", l.taps, l.doubles)
	}
	tt.TapAt(at)
	tt.TapAt(at)
	if l.taps != 3 || l.doubles != 1 {
		t.Errorf("two separate taps: taps %d, double taps %d; want 3 and 1", l.taps, l.doubles)
	}
}

func TestGestureDetector_DoubleClickWithoutOnDoubleTapIsTwoTaps(t *testing.T) {
	var l gestureLog
	tt := gowakutest.New(t, basic.Align{Child: l.detector(false, false)}, pressableFonts)
	tt.DoubleTapAt(geom.Offset{X: 30, Y: 30})
	if l.taps != 2 {
		t.Errorf("taps %d, want 2", l.taps)
	}
}

func TestGestureDetector_ButtonInsideKeepsItsPress(t *testing.T) {
	var l gestureLog
	presses := 0
	g := l.detector(true, true)
	g.Child = basic.SizedBox{Width: 100, Height: 100, Child: basic.Align{Child: interact.Pressable{
		Label: "Inner", OnPress: func() { presses++ }, Child: basic.SizedBox{Width: 40, Height: 40},
	}}}
	tt := gowakutest.New(t, basic.Align{Child: g}, pressableFonts)
	tt.DragAt(geom.Offset{X: 20, Y: 20}, geom.Offset{X: 90, Y: 90})
	tt.TapAt(geom.Offset{X: 20, Y: 20})
	if l.starts != 0 || l.taps != 0 || presses != 1 {
		t.Errorf("starts %d, taps %d, presses %d; want the button to keep its presses", l.starts, l.taps, presses)
	}
	tt.DragAt(geom.Offset{X: 70, Y: 70}, geom.Offset{X: 90, Y: 90})
	if l.starts != 1 {
		t.Errorf("starts %d beside the button, want 1", l.starts)
	}
}

// touchDrag moves a finger from from to to on tt, in steps.
func touchDrag(tt *gowakutest.Tester, from, to geom.Offset) {
	finger := func(kind core.PointerKind, pos geom.Offset) {
		tt.Tree().DispatchPointer(core.PointerEvent{Kind: kind, ID: 1, Pos: pos, Device: input.DeviceTouch})
		tt.Pump()
	}
	finger(core.PointerDown, from)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		finger(core.PointerMove, geom.Offset{X: from.X + (to.X-from.X)*f, Y: from.Y + (to.Y-from.Y)*f})
	}
	finger(core.PointerUp, to)
	tt.Settle()
}

func TestGestureDetector_FingerScrollsUnlessItDrags(t *testing.T) {
	for _, drag := range []bool{false, true} {
		var l gestureLog
		var ctl basic.ScrollController
		g := l.detector(drag, false)
		g.Child = basic.SizedBox{Width: 100, Height: 1000}
		tt := gowakutest.New(t, basic.SizedBox{Width: 200, Height: 200, Child: basic.Scroll{Controller: &ctl, Child: g}}, gowakutest.Options{
			Fonts: pressableFonts.Fonts, Width: 200, Height: 200,
		})
		touchDrag(tt, geom.Offset{X: 50, Y: 150}, geom.Offset{X: 50, Y: 50})
		scrolled := ctl.Offset() > 0
		if scrolled == drag {
			t.Errorf("drag callbacks %v: offset %v, drag starts %d", drag, ctl.Offset(), l.starts)
		}
		if drag && (l.starts != 1 || l.ends != 1) {
			t.Errorf("a finger on a detector that drags: starts %d, ends %d", l.starts, l.ends)
		}
		if l.taps != 0 {
			t.Errorf("drag callbacks %v: a finger drag tapped", drag)
		}
	}
}

func TestFocus_ClickFocusesOnlyThroughWhatTakesThePress(t *testing.T) {
	for _, takes := range []bool{false, true} {
		focused := false
		var child core.Widget = basic.SizedBox{Width: 100, Height: 100}
		if takes {
			child = interact.GestureDetector{OnTapDown: func() {}, Child: child}
		}
		tt := gowakutest.New(t, basic.Align{Child: interact.Focus{
			Enabled: true, OnFocusChange: func(f bool) { focused = f }, Child: child,
		}}, pressableFonts)
		tt.TapAt(geom.Offset{X: 50, Y: 50})
		if focused != takes {
			t.Errorf("a press taker inside %v: focused %v", takes, focused)
		}
	}
}

func TestListener_DoesNotSeePressesAChildTakes(t *testing.T) {
	var kinds []core.PointerKind
	tt := gowakutest.New(t, basic.Align{Child: interact.Listener{
		OnPointer: func(e *core.PointerEvent) { kinds = append(kinds, e.Kind) },
		Child: basic.Row{Children: []core.Widget{
			interact.Pressable{Label: "Button", OnPress: func() {}, Child: basic.SizedBox{Width: 50, Height: 50}},
			basic.SizedBox{Width: 50, Height: 50},
		}},
	}}, pressableFonts)
	tt.TapAt(geom.Offset{X: 25, Y: 25})
	if len(kinds) != 0 {
		t.Errorf("a press on the button reached the listener: %v", kinds)
	}
	tt.TapAt(geom.Offset{X: 75, Y: 25})
	if len(kinds) != 2 || kinds[0] != core.PointerDown || kinds[1] != core.PointerUp {
		t.Errorf("a press beside the button: %v, want down and up", kinds)
	}
}
