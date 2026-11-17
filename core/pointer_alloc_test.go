package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// hookProbe is a pointer target that runs fn on every event.
type hookProbe struct {
	StatelessBase
	fn func(e *PointerEvent)
}

func (h hookProbe) CreateRenderObject() RenderObject { return &hookObject{fn: h.fn} }

func (hookProbe) UpdateRenderObject(RenderObject) {}

type hookObject struct {
	RenderBase
	fn func(e *PointerEvent)
}

func (r *hookObject) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 100, H: 100})
}

func (r *hookObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *hookObject) HandlePointer(e *PointerEvent) { r.fn(e) }

func TestPointer_DeliveryDoesNotAllocate(t *testing.T) {
	moves := 0
	tree := inputTree(t, hookProbe{fn: func(e *PointerEvent) {
		if e.Kind == PointerMove {
			moves++
		}
		e.Handled = true
	}})
	// A press, so that the moves go to the object that took it, as in a drag.
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(10, 10)})
	x := float32(10)
	allocs := testing.AllocsPerRun(200, func() {
		x++
		tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(x, 10)})
	})
	if allocs != 0 {
		t.Errorf("a pointer move allocates %v times, want 0", allocs)
	}
	if moves == 0 {
		t.Fatal("the moves did not reach the target")
	}
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: at(x, 10)})
	allocs = testing.AllocsPerRun(200, func() {
		x--
		tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(x, 10)})
	})
	if allocs != 0 {
		t.Errorf("a hover move allocates %v times, want 0", allocs)
	}
}

func TestPointer_DispatchFromAHandlerKeepsTheOuterEvent(t *testing.T) {
	var tree *Tree
	var log []PointerKind
	var outerPos geom.Offset
	tree = inputTree(t, hookProbe{fn: func(e *PointerEvent) {
		log = append(log, e.Kind)
		if e.Kind == PointerDown {
			// The scroll inside is not handled; that must not leak out.
			tree.DispatchPointer(PointerEvent{Kind: PointerScroll, Pos: at(50, 50)})
			outerPos = e.Pos
			e.Handled = true
		}
	}})
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(10, 20)})
	if outerPos != at(10, 20) || e2s(log) != "down scroll" {
		t.Errorf("the outer event read %v after %s", outerPos, e2s(log))
	}
	// The press was handled, so the move goes to the object that took it.
	log = nil
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(300, 300)})
	if e2s(log) != "move" {
		t.Errorf("the press was not captured: %s", e2s(log))
	}
}

func e2s(kinds []PointerKind) string {
	names := map[PointerKind]string{PointerDown: "down", PointerMove: "move", PointerUp: "up", PointerScroll: "scroll"}
	s := ""
	for i, k := range kinds {
		if i > 0 {
			s += " "
		}
		s += names[k]
	}
	return s
}

// hoverProbe hears the moves of the pointer over it.
type hoverProbe struct {
	StatelessBase
	fn func(pos geom.Offset)
}

func (h hoverProbe) CreateRenderObject() RenderObject { return &hoverObject{fn: h.fn} }

func (hoverProbe) UpdateRenderObject(RenderObject) {}

type hoverObject struct {
	RenderBase
	fn func(pos geom.Offset)
}

func (r *hoverObject) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 100, H: 100})
}

func (r *hoverObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *hoverObject) HandleHoverMove(pos geom.Offset) { r.fn(pos) }

func TestPointer_HoverMovesDoNotAllocate(t *testing.T) {
	var last geom.Offset
	moves := 0
	tree := inputTree(t, hoverProbe{fn: func(pos geom.Offset) { last = pos; moves++ }})
	x := float32(10)
	allocs := testing.AllocsPerRun(200, func() {
		x = 30 - x
		tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(x, 10)})
	})
	if allocs != 0 {
		t.Errorf("a hover move to a HoverMoveTarget allocates %v times, want 0", allocs)
	}
	if moves == 0 || last != at(x, 10) {
		t.Errorf("%d moves, the last at %v, want at %v", moves, last, at(x, 10))
	}
}
