package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// markedBox is a box of a given height whose render object the test finds by
// its color.
func markedBox(h float32, c geom.Color) core.Widget { return Container{Height: h, Color: c} }

// findColored returns the render object of the Container in color c.
func findColored(t *testing.T, ro core.RenderObject, c geom.Color) core.RenderObject {
	t.Helper()
	var found core.RenderObject
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if found != nil {
			return
		}
		if b, ok := ro.(*layout.RenderDecoratedBox); ok && b.Decoration.Color == c {
			found = ro
			return
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(ro)
	if found == nil {
		t.Fatal("no box of that color")
	}
	return found
}

func TestScrollIntoView(t *testing.T) {
	target := geom.Hex(0xff0000)
	tree := wtest.TextTree(t, Scroll{Child: Column{Size: layout.MainMin, Children: []core.Widget{
		markedBox(500, geom.Hex(0x101010)),
		markedBox(40, target),
		markedBox(1000, geom.Hex(0x202020)),
	}}})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	box := findColored(t, tree.RootRenderObject(), target)
	whole := geom.Rect{W: 200, H: 40}

	ScrollIntoView(box, whole, 0)
	testutil.RenderTree(tree, size)
	if y := core.GlobalOffset(box).Y; y != 0 {
		t.Errorf("align 0 puts the box at %v", y)
	}
	ScrollIntoView(box, whole, 1)
	testutil.RenderTree(tree, size)
	if y := core.GlobalOffset(box).Y; y != 300-40 {
		t.Errorf("align 1 puts the box at %v, want %v", y, 300-40)
	}
	// A rect above the box, as room over a heading.
	ScrollIntoView(box, geom.Rect{Y: -10, W: 200, H: 50}, 0)
	testutil.RenderTree(tree, size)
	if y := core.GlobalOffset(box).Y; y != 10 {
		t.Errorf("a rect 10 above the box puts it at %v", y)
	}
	// Near the end the view stops at the end of the content.
	last := findColored(t, tree.RootRenderObject(), geom.Hex(0x202020))
	ScrollIntoView(last, geom.Rect{Y: 900, W: 200, H: 100}, 0)
	testutil.RenderTree(tree, size)
	if v := viewportOf(t, tree).Position.Value(); v != 1540-300 {
		t.Errorf("the view goes to %v, past its end %v", v, 1540-300)
	}
}

func TestScrollIntoView_Nested(t *testing.T) {
	target := geom.Hex(0xff0000)
	tree := wtest.TextTree(t, Scroll{Child: Column{Size: layout.MainMin, Children: []core.Widget{
		markedBox(400, geom.Hex(0x101010)),
		SizedBox{Height: 200, Child: Scroll{Child: Column{Size: layout.MainMin, Children: []core.Widget{
			markedBox(600, geom.Hex(0x303030)),
			markedBox(40, target),
			markedBox(600, geom.Hex(0x303030)),
		}}}},
		markedBox(800, geom.Hex(0x202020)),
	}}})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	box := findColored(t, tree.RootRenderObject(), target)
	ScrollIntoView(box, geom.Rect{W: 200, H: 40}, 0)
	testutil.RenderTree(tree, size)
	if y := core.GlobalOffset(box).Y; y != 0 {
		t.Errorf("the box in the inner Scroll is at %v in the window", y)
	}
}
