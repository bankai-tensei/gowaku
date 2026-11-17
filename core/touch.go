package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// A touch screen sends a pointer per finger. The tree delivers the first
// finger as the pointer, so that taps and presses work as with a mouse; once
// it moves past panSlop, the press is canceled and the moves scroll what is
// under the place it went down, if what took the press lets it (PanYielder).
// A second finger cancels the press and turns both into pinches. See
// docs/ADR/0052-pinch-touch-and-pen.md.
const (
	// panSlop is how far a finger moves before it pans, in logical pixels.
	panSlop = 8
	// touchNotch is the distance of a finger that scrolls as one notch of a
	// wheel, which the scroll views of widgets move by 100 logical pixels.
	touchNotch = 100
)

// PanYielder is a PointerTarget that lets a finger that pressed it scroll
// instead, as a button in a list does. Others keep the finger, as a slider
// does.
type PanYielder interface {
	PointerTarget
	YieldsToPan() bool
}

// touches is what the tree knows of the fingers on the screen.
type touches struct {
	primary, second   uint32
	hasPrimary        bool
	hasSecond         bool
	start, last       geom.Offset
	secondPos         geom.Offset
	panning, pinching bool
	distance          float32
}

// dispatchTouch delivers a pointer event of a finger.
func (t *Tree) dispatchTouch(e PointerEvent) {
	ts := &t.touch
	switch e.Kind {
	case PointerDown:
		switch {
		case !ts.hasPrimary:
			*ts = touches{primary: e.ID, hasPrimary: true, start: e.Pos, last: e.Pos}
			t.pointerDown(&e)
		case !ts.hasSecond && !ts.panning:
			ts.second, ts.hasSecond, ts.secondPos = e.ID, true, e.Pos
			t.cancelPress(e)
			ts.pinching, ts.distance = true, ts.last.Sub(e.Pos).Length()
		}
	case PointerMove:
		t.touchMove(e)
	case PointerUp, PointerCancel:
		switch {
		case ts.hasPrimary && e.ID == ts.primary:
			ts.hasPrimary = false
			if !ts.panning && !ts.pinching {
				t.dispatchPointer(e)
			}
		case ts.hasSecond && e.ID == ts.second:
			ts.hasSecond = false
		}
		if !ts.hasPrimary && !ts.hasSecond {
			t.touch = touches{}
		}
	}
}

func (t *Tree) touchMove(e PointerEvent) {
	ts := &t.touch
	switch {
	case ts.hasPrimary && e.ID == ts.primary:
		delta := e.Pos.Sub(ts.last)
		ts.last = e.Pos
		if ts.pinching {
			t.touchPinch()
			return
		}
		if !ts.panning {
			if e.Pos.Sub(ts.start).Length() < panSlop || !t.canPan() {
				t.dispatchPointer(e)
				return
			}
			ts.panning = true
			t.cancelPress(e)
			delta = e.Pos.Sub(ts.start)
		}
		scroll := e
		scroll.Kind, scroll.Pos = PointerScroll, ts.start
		scroll.Scroll = geom.Offset{X: -delta.X / touchNotch, Y: delta.Y / touchNotch}
		t.hit(ts.start)
		t.deliver(t.hits.Path, &scroll)
	case ts.hasSecond && e.ID == ts.second:
		ts.secondPos = e.Pos
		if ts.pinching {
			t.touchPinch()
		}
	}
}

// touchPinch delivers the change of distance between the two fingers.
func (t *Tree) touchPinch() {
	ts := &t.touch
	if !ts.hasPrimary || !ts.hasSecond {
		return
	}
	d := ts.last.Sub(ts.secondPos).Length()
	if d <= 0 || ts.distance <= 0 || d == ts.distance {
		return
	}
	mid := geom.Offset{X: (ts.last.X + ts.secondPos.X) / 2, Y: (ts.last.Y + ts.secondPos.Y) / 2}
	e := PointerEvent{Kind: PointerPinch, Pos: mid, Pinch: d / ts.distance, Device: input.DeviceTouch}
	ts.distance = d
	t.hit(mid)
	t.deliver(t.hits.Path, &e)
}

// canPan reports whether the press in progress lets the finger scroll.
func (t *Tree) canPan() bool {
	for _, ro := range t.captured {
		if y, ok := ro.(PanYielder); !ok || !y.YieldsToPan() {
			return false
		}
	}
	return true
}

// cancelPress ends the press in progress without a release.
func (t *Tree) cancelPress(e PointerEvent) {
	cancel := e
	cancel.Kind = PointerCancel
	t.deliver(t.captured, &cancel)
	t.captured = t.captured[:0]
}
