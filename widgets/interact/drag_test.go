package interact

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// dragHost records the drags that a tree starts.
type dragHost struct {
	core.Host
	data    *input.DragData
	allowed input.DropEffect
	done    func(input.DropEffect)
}

func (h *dragHost) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	h.data, h.allowed, h.done = &d, allowed, done
}

func TestDragTarget_AcceptsKindsLightsUpAndTakesTheDrop(t *testing.T) {
	var hover []bool
	var got input.DragData
	var at geom.Offset
	tree := wtest.TextTree(t, basic.Column{Children: []core.Widget{
		basic.SizedBox{Height: 50},
		DragTarget{
			Kinds:   input.DataFiles,
			OnHover: func(over bool) { hover = append(hover, over) },
			OnDrop:  func(d input.DragData, pos geom.Offset) { got, at = d, pos },
			Child:   basic.Container{Width: 100, Height: 100},
		},
	}})
	wtest.Ops(tree, wtest.Screen)
	files := core.DragEvent{Pos: geom.Offset{X: 10, Y: 60}, Kinds: input.DataFiles, Allowed: input.DropCopy | input.DropMove}
	if e := tree.DispatchDrag(core.DragOver, files); e != input.DropCopy {
		t.Errorf("files over the target: %v, want a copy", e)
	}
	text := files
	text.Kinds = input.DataText
	if e := tree.DispatchDrag(core.DragOver, text); e != input.DropNone {
		t.Errorf("text over a target of files: %v, want none", e)
	}
	moveOnly := files
	moveOnly.Allowed = input.DropMove
	if e := tree.DispatchDrag(core.DragOver, moveOnly); e != input.DropMove {
		t.Errorf("a source that only moves: %v, want a move", e)
	}
	outside := files
	outside.Pos = geom.Offset{X: 10, Y: 10}
	tree.DispatchDrag(core.DragOver, outside)
	drop := files
	drop.Data = input.DragData{Files: []string{"/a"}}
	if e := tree.DispatchDrag(core.DragDrop, drop); e != input.DropCopy {
		t.Errorf("drop: %v", e)
	}
	if !slices.Equal(got.Files, []string{"/a"}) || at != (geom.Offset{X: 10, Y: 10}) {
		t.Errorf("dropped %v at %v", got, at)
	}
	if !slices.Equal(hover, []bool{true, false, true, false, true, false}) {
		t.Errorf("hover %v", hover)
	}
}

func TestDraggable_TapOrDrag(t *testing.T) {
	var taps int
	var ended []input.DropEffect
	tree := wtest.TextTree(t, basic.Align{Child: Draggable{
		Data:      input.DragData{Text: "x"},
		Allowed:   input.DropCopy | input.DropMove,
		OnTap:     func() { taps++ },
		OnDragEnd: func(e input.DropEffect) { ended = append(ended, e) },
		Child:     basic.Container{Width: 50, Height: 50},
	}})
	host := &dragHost{}
	tree.SetHost(host)
	wtest.Ops(tree, wtest.Screen)
	pos := geom.Offset{X: 5, Y: 5}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 6, Y: 6}})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 6, Y: 6}})
	if taps != 1 || host.data != nil {
		t.Fatalf("a press that barely moves is a tap: %d taps, drag %v", taps, host.data)
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 20, Y: 5}})
	if host.data == nil || host.data.Text != "x" || host.allowed != input.DropCopy|input.DropMove {
		t.Fatalf("moving starts the drag: %v %v", host.data, host.allowed)
	}
	host.done(input.DropMove)
	if taps != 1 || !slices.Equal(ended, []input.DropEffect{input.DropMove}) {
		t.Errorf("taps %d, ended %v", taps, ended)
	}
}
