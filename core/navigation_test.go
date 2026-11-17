package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// navList is a list that goes back and forward, as a Navigator does.
type navList struct {
	list
	obj *navObject
}

func (n navList) CreateRenderObject() RenderObject { return n.obj }

type navObject struct {
	renderList
	name string
	log  *[]string
	// can says whether it has somewhere to go.
	can bool
}

func (n *navObject) HandleNavigation(forward bool) bool {
	dir := "back"
	if forward {
		dir = "forward"
	}
	*n.log = append(*n.log, n.name+" "+dir)
	return n.can
}

// keyEater takes the focus and uses every key, as a text field uses the arrows.
type keyEater struct {
	focusBox
	keys int
}

func (k *keyEater) HandleKey(*KeyEvent) bool { k.keys++; return true }

func navTree(t *testing.T) (tree *Tree, outer, inner *navObject, field *keyEater, log *[]string) {
	t.Helper()
	log = &[]string{}
	outer = &navObject{name: "outer", log: log, can: true}
	inner = &navObject{name: "inner", log: log}
	field = &keyEater{}
	tree = NewTree(navList{obj: outer, list: list{children: []Widget{
		navList{obj: inner, list: list{children: []Widget{given{field}}}},
	}}})
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	return tree, outer, inner, field, log
}

func TestNavigation_AltArrowGoesToTheNearestTargetBeforeTheFocus(t *testing.T) {
	tree, _, inner, field, log := navTree(t)
	tree.RequestFocus(field)

	tree.DispatchKey(KeyEvent{Key: input.KeyLeft, Modifiers: input.ModAlt})
	if got := *log; len(got) != 2 || got[0] != "inner back" || got[1] != "outer back" {
		t.Errorf("an inner target with nowhere to go passes the request out: %v", got)
	}
	if field.keys != 0 {
		t.Error("a request that a target took must not reach the focus")
	}

	*log = (*log)[:0]
	inner.can = true
	tree.DispatchKey(KeyEvent{Key: input.KeyRight, Modifiers: input.ModAlt})
	if got := *log; len(got) != 1 || got[0] != "inner forward" {
		t.Errorf("the inner target goes first and stops it: %v", got)
	}
}

func TestNavigation_OtherKeysReachTheFocus(t *testing.T) {
	tree, _, _, field, log := navTree(t)
	tree.RequestFocus(field)
	for _, e := range []KeyEvent{
		{Key: input.KeyLeft},
		{Key: input.KeyLeft, Modifiers: input.ModAlt | input.ModShift},
		{Key: input.KeyLeft, Modifiers: input.ModControl | input.ModAlt},
		{Key: input.KeyLeft, Modifiers: input.ModAlt, Repeat: true},
		{Key: input.KeyUp, Modifiers: input.ModAlt},
	} {
		tree.DispatchKey(e)
	}
	if len(*log) != 0 {
		t.Errorf("only Alt+Left and Alt+Right, unrepeated, navigate: %v", *log)
	}
	if field.keys != 5 {
		t.Errorf("the focus got %d keys, want 5", field.keys)
	}
}

func TestNavigation_UnusedRequestFallsThroughToTheFocus(t *testing.T) {
	tree, outer, _, field, log := navTree(t)
	outer.can = false
	tree.RequestFocus(field)
	tree.DispatchKey(KeyEvent{Key: input.KeyLeft, Modifiers: input.ModAlt})
	if len(*log) != 2 || field.keys != 1 {
		t.Errorf("with nowhere to go the key goes on to the focus: log %v, keys %d", *log, field.keys)
	}
}

func TestNavigation_WithoutFocusTheInnerTargetGoesFirst(t *testing.T) {
	tree, _, inner, _, log := navTree(t)
	inner.can = true
	tree.DispatchKey(KeyEvent{Key: input.KeyLeft, Modifiers: input.ModAlt})
	if got := *log; len(got) != 1 || got[0] != "inner back" {
		t.Errorf("got %v, want [inner back]", got)
	}
}

func TestNavigation_ThumbButtonsAreNotPresses(t *testing.T) {
	tree, outer, inner, field, log := navTree(t)
	tree.RequestFocus(field)
	at := geom.Offset{X: 5, Y: 5}

	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at, Button: input.ButtonBack})
	if len(*log) != 0 {
		t.Errorf("the press of a thumb button does nothing: %v", *log)
	}
	if tree.Focused() != field {
		t.Error("the press of a thumb button must not move the focus")
	}
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: at, Button: input.ButtonBack})
	if got := *log; len(got) != 2 || got[0] != "inner back" || got[1] != "outer back" {
		t.Errorf("the release goes back, deepest first: %v", got)
	}

	*log = (*log)[:0]
	inner.can, outer.can = true, true
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at, Button: input.ButtonForward})
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: at, Button: input.ButtonForward})
	if got := *log; len(got) != 1 || got[0] != "inner forward" {
		t.Errorf("got %v, want [inner forward]", got)
	}

	*log = (*log)[:0]
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: geom.Offset{X: 50, Y: 90}, Button: input.ButtonBack})
	if got := *log; len(got) != 1 || got[0] != "outer back" {
		t.Errorf("below the inner list only the outer one is under the pointer: %v", got)
	}
}
