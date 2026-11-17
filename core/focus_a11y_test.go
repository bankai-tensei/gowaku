package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

func chain(log *[]string, leaves ...probe) Widget {
	var w Widget
	for i := len(leaves) - 1; i >= 0; i-- {
		p := leaves[i]
		p.log, p.size = log, geom.Size{W: 100, H: 100}
		if w != nil {
			p.child = w
		}
		w = p
	}
	return w
}

func TestFocus_PressGivesFocusToTheHandler(t *testing.T) {
	var log []string
	tree := inputTree(t, chain(&log, probe{name: "a", focus: true}, probe{name: "b", handle: true, focus: true}))
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(50, 50)})
	if f := tree.Focused(); f == nil || f.(*probeObject).w.name != "b" {
		t.Fatalf("focus on %v, want b", f)
	}
	if tree.FocusVisible() {
		t.Error("a press must hide the focus")
	}
	tree.DispatchKey(KeyEvent{Key: input.KeyA})
	if !tree.FocusVisible() {
		t.Error("a key must show the focus")
	}
}

func TestFocus_PressOnAnUnfocusableHandlerKeepsTheFocus(t *testing.T) {
	var log []string
	// b handles the press but takes no focus, as a scroll bar does; a has it.
	tree := inputTree(t, chain(&log, probe{name: "a", focus: true}, probe{name: "b", handle: true}))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(50, 50)})
	if f := tree.Focused(); f == nil || f.(*probeObject).w.name != "a" {
		t.Fatalf("focus on %v, want a to keep it", f)
	}
}

// visibleProbe records the news of FocusVisibleListener.
type visibleProbe struct {
	probe
}

func (v visibleProbe) CreateRenderObject() RenderObject {
	return &visibleObject{probeObject: probeObject{w: v.probe}}
}

type visibleObject struct {
	probeObject
}

func (r *visibleObject) FocusVisibleChanged(visible bool) {
	if visible {
		r.note("shown")
	} else {
		r.note("hidden")
	}
}

func TestFocus_VisibleChangesReachTheFocus(t *testing.T) {
	var log []string
	inner := probe{name: "b", log: &log, handle: true, size: geom.Size{W: 100, H: 100}}
	tree := inputTree(t, visibleProbe{probe{name: "a", log: &log, focus: true, size: geom.Size{W: 100, H: 100}, child: inner}})
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	log = nil
	// b handles the press and takes no focus, so a keeps it, unshown.
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(50, 50)})
	tree.DispatchKey(KeyEvent{Key: input.KeyA})
	if got := joined(log); got != "a:hidden b:down a:shown a:key" {
		t.Errorf("got %s", got)
	}
}

func TestFocus_TraversalOrder(t *testing.T) {
	var log []string
	tree := inputTree(t, chain(&log,
		probe{name: "a", focus: true},
		probe{name: "b", focus: true, order: 2},
		probe{name: "c", focus: true, order: 1},
	))
	var got []string
	for range 4 {
		tree.DispatchKey(KeyEvent{Key: input.KeyTab})
		got = append(got, tree.Focused().(*probeObject).w.name)
	}
	if want := "c b a c"; joined(got) != want {
		t.Errorf("Tab went %v, want %s", got, want)
	}
	tree.DispatchKey(KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
	if n := tree.Focused().(*probeObject).w.name; n != "a" {
		t.Errorf("Shift+Tab went to %s, want a", n)
	}
}

func TestSemanticsTree_ActionsAndFocus(t *testing.T) {
	var log []string
	tree := inputTree(t, chain(&log, probe{name: "a", focus: true}))
	root, focus := tree.SemanticsTree()
	if focus != 0 || len(root.Children) != 0 {
		t.Fatalf("a probe opens no node: focus %d, %+v", focus, root)
	}
	if tree.PerformSemanticsAction(42, semantics.ActionTap, SemanticsArg{}) {
		t.Error("an unknown node must do nothing")
	}
}
