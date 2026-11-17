package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

// given is a widget whose render object the test made.
type given struct{ obj RenderObject }

func (given) Key() Key                           { return nil }
func (g given) CreateRenderObject() RenderObject { return g.obj }
func (given) UpdateRenderObject(RenderObject)    {}

// shortcutBox is a leaf that hears shortcuts, and may take the focus.
type shortcutBox struct {
	RenderBase
	name string
	log  *[]string
	used bool
}

func (b *shortcutBox) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 10, H: 10})
}

func (b *shortcutBox) Paint(*paint.Canvas, geom.Offset) {}

func (b *shortcutBox) HandleShortcut(*KeyEvent) bool {
	*b.log = append(*b.log, b.name)
	return b.used
}

// focusBox takes the focus and lets every key through.
type focusBox struct {
	shortcutBox
}

func (*focusBox) CanFocus() bool                       { return true }
func (*focusBox) HandleKey(*KeyEvent) bool             { return false }
func (*focusBox) HandleText(string)                    {}
func (*focusBox) HandleComposition(*input.Composition) {}
func (*focusBox) FocusChanged(bool)                    {}

func TestShortcut_NearestToFocusFirstThenTheRest(t *testing.T) {
	var log []string
	focused := &focusBox{shortcutBox{name: "focus", log: &log}}
	other := &shortcutBox{name: "other", log: &log}
	tree := NewTree(list{children: []Widget{given{focused}, given{other}}})
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	tree.RequestFocus(focused)
	tree.DispatchKey(KeyEvent{Key: input.KeyS, Modifiers: input.ModControl})
	if len(log) != 2 || log[0] != "focus" || log[1] != "other" {
		t.Errorf("order %v, want [focus other]", log)
	}
	log = log[:0]
	tree.RequestFocus(nil)
	other.used = true
	tree.DispatchKey(KeyEvent{Key: input.KeyS, Modifiers: input.ModControl})
	if len(log) != 1 || log[0] != "other" {
		t.Errorf("without a focus, the last painted goes first and stops it: %v", log)
	}
}

func TestShortcut_AltAloneArrivesOnRelease(t *testing.T) {
	var log []string
	box := &shortcutBox{name: "bar", log: &log, used: true}
	tree := NewTree(list{children: []Widget{given{box}}})
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))

	tree.DispatchKey(KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt})
	if len(log) != 0 {
		t.Fatal("the press of Alt must not arrive")
	}
	tree.DispatchKey(KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt, Repeat: true})
	tree.DispatchKeyUp(KeyEvent{Key: input.KeyAlt})
	if len(log) != 1 {
		t.Fatalf("a held Alt released alone must arrive once, got %v", log)
	}

	for _, between := range []func(){
		func() { tree.DispatchKey(KeyEvent{Key: input.KeyF, Modifiers: input.ModAlt}) },
		func() { tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: geom.Offset{X: 5, Y: 5}}) },
		func() { tree.SetWindowFocused(false); tree.SetWindowFocused(true) },
		func() { tree.DispatchKey(KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt | input.ModControl}) },
	} {
		log = log[:0]
		tree.DispatchKey(KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt})
		between()
		log = log[:0]
		tree.DispatchKeyUp(KeyEvent{Key: input.KeyAlt})
		if len(log) != 0 {
			t.Errorf("Alt with something in between must not arrive alone: %v", log)
		}
	}
}

func TestDropEffect_Pick(t *testing.T) {
	if got := (input.DropCopy | input.DropMove).Pick(input.DropMove); got != input.DropMove {
		t.Errorf("got %v", got)
	}
	if got := input.DropLink.Pick(input.DropCopy); got != input.DropNone {
		t.Errorf("got %v", got)
	}
}
