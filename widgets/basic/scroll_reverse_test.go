package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func wideChild(width float32) core.Widget {
	return Container{Width: width, Color: geom.Hex(0x123456)}
}

// childAt returns where the content of v starts along its axis.
func childAt(v *layout.RenderViewport) float32 {
	off := v.ChildAt(0).Offset()
	if v.Axis == layout.Horizontal {
		return off.X
	}
	return off.Y
}

// wheelSideways turns a horizontal wheel, positive toward the right.
func wheelSideways(tree *core.Tree, notches float32) {
	tree.DispatchPointer(core.PointerEvent{
		Kind: core.PointerScroll, Pos: geom.Offset{X: 10, Y: 10}, Scroll: geom.Offset{X: notches},
	})
}

func TestScrollReverse_OpensAtTheRightEnd(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Child: wideChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	if got := v.Position.Value(); got != 0 {
		t.Errorf("offset %v, want 0", got)
	}
	if got := childAt(v); got != 200-1000 {
		t.Errorf("content starts at %v, want its right end at the right of the view", got)
	}
}

func TestScrollReverse_OpensAtTheBottom(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Reverse: true, Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	if got := childAt(viewportOf(t, tree)); got != 300-1000 {
		t.Errorf("content starts at %v, want its bottom at the bottom of the view", got)
	}
}

func TestScrollReverse_ShortContentSitsAtTheEnd(t *testing.T) {
	for _, tc := range []struct {
		name  string
		w     Scroll
		inset float32
	}{
		{"horizontal", Scroll{Horizontal: true, Reverse: true, Child: wideChild(120)}, 200 - 120},
		{"vertical", Scroll{Reverse: true, Child: tallChild(100)}, 300 - 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := wtest.TextTree(t, tc.w)
			size := geom.Size{W: 200, H: 300}
			testutil.RenderTree(tree, size)
			v := viewportOf(t, tree)
			if got := childAt(v); got != tc.inset {
				t.Errorf("content starts at %v, want %v", got, tc.inset)
			}
			wtest.Wheel(tree, -2)
			wheelSideways(tree, 2)
			wtest.RunUntilIdle(t, tree, size, nil)
			if got := childAt(v); got != tc.inset {
				t.Errorf("after the wheel the content rests at %v, want %v", got, tc.inset)
			}
		})
	}
}

func TestScrollReverse_WheelMovesTheContentTheWayItTurns(t *testing.T) {
	size := geom.Size{W: 200, H: 300}
	t.Run("horizontal", func(t *testing.T) {
		var c ScrollController
		c.JumpTo(400)
		tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Controller: &c, Child: wideChild(1000)})
		testutil.RenderTree(tree, size)
		v := viewportOf(t, tree)
		before := childAt(v)
		// To the right, as without Reverse: the content moves left, toward
		// the start of the scroll range at the right end.
		wheelSideways(tree, 1)
		wtest.RunUntilIdle(t, tree, size, nil)
		if got := c.Offset(); got != 400-kit.NotchDistance {
			t.Errorf("offset %v, want %v", got, 400-kit.NotchDistance)
		}
		if got := childAt(v); got != before-kit.NotchDistance {
			t.Errorf("content at %v, want %v", got, before-kit.NotchDistance)
		}
	})
	t.Run("vertical", func(t *testing.T) {
		var c ScrollController
		c.JumpTo(300)
		tree := wtest.TextTree(t, Scroll{Reverse: true, Controller: &c, Child: tallChild(1000)})
		testutil.RenderTree(tree, size)
		v := viewportOf(t, tree)
		before := childAt(v)
		// Toward the user: the content moves up, toward the bottom end.
		wtest.Wheel(tree, -1)
		wtest.RunUntilIdle(t, tree, size, nil)
		if got := childAt(v); got != before-kit.NotchDistance {
			t.Errorf("content at %v, want %v", got, before-kit.NotchDistance)
		}
		if got := c.Offset(); got != 300-kit.NotchDistance {
			t.Errorf("offset %v, want %v", got, 300-kit.NotchDistance)
		}
	})
}

func TestScrollReverse_WheelPastTheEndsBouncesBack(t *testing.T) {
	var c ScrollController
	tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Controller: &c, Child: wideChild(400)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	wheelSideways(tree, 9)
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := c.Offset(); got != 0 {
		t.Errorf("rests at %v, want the right end", got)
	}
	wheelSideways(tree, -9)
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := c.Offset(); got != 200 {
		t.Errorf("rests at %v, want the left end", got)
	}
}

func TestScrollReverse_ResizeKeepsTheRightEnd(t *testing.T) {
	var c ScrollController
	tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Controller: &c, Child: wideChild(1000)})
	testutil.RenderTree(tree, geom.Size{W: 200, H: 300})
	v := viewportOf(t, tree)
	testutil.RenderTree(tree, geom.Size{W: 350, H: 300})
	if got := childAt(v); got != 350-1000 {
		t.Errorf("wider: content at %v, want its right end at the right", got)
	}
	c.JumpTo(100)
	testutil.RenderTree(tree, geom.Size{W: 250, H: 300})
	if got := childAt(v); got != 250-1000+100 {
		t.Errorf("narrower: content at %v, want 100 short of the right end", got)
	}
	testutil.RenderTree(tree, geom.Size{W: 1200, H: 300})
	if got, want := childAt(v), float32(1200-1000); got != want {
		t.Errorf("wider than the content: at %v, want %v", got, want)
	}
}

func TestScrollReverse_ScrollBarThumbStartsAtTheRight(t *testing.T) {
	var c ScrollController
	tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Controller: &c, Child: wideChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	// A press at the left end of the bar brings the thumb there: the end of
	// the scroll range.
	bar := geom.Offset{X: 2, Y: size.H - 4}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: bar})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: bar})
	if got := c.Offset(); got != 800 {
		t.Errorf("a press at the left of the bar goes to %v, want 800", got)
	}
	right := geom.Offset{X: size.W - 2, Y: size.H - 4}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: right})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: right})
	if got := c.Offset(); got != 0 {
		t.Errorf("a press at the right of the bar goes to %v, want 0", got)
	}
}

func TestScrollReverse_ScrollIntoView(t *testing.T) {
	target := geom.Hex(0xff0000)
	tree := wtest.TextTree(t, Scroll{Horizontal: true, Reverse: true, Child: Row{Size: layout.MainMin, Children: []core.Widget{
		Container{Width: 500, Color: geom.Hex(0x101010)},
		Container{Width: 40, Color: target},
		Container{Width: 1000, Color: geom.Hex(0x202020)},
	}}})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	box := findColored(t, tree.RootRenderObject(), target)
	whole := geom.Rect{W: 40, H: 300}
	for _, tc := range []struct {
		align float32
		want  float32
	}{{0, 0}, {1, 200 - 40}, {0.5, 80}} {
		ScrollIntoView(box, whole, tc.align)
		testutil.RenderTree(tree, size)
		if x := core.GlobalOffset(box).X; x != tc.want {
			t.Errorf("align %v puts the box at %v, want %v", tc.align, x, tc.want)
		}
	}
}

func TestScrollController_JumpsBeforeAndAfterLayout(t *testing.T) {
	var c ScrollController
	c.JumpTo(5000)
	if got := c.Offset(); got != 5000 {
		t.Errorf("waiting jump reads %v", got)
	}
	tree := wtest.TextTree(t, Scroll{Controller: &c, Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	if got := c.Offset(); got != 700 {
		t.Errorf("a jump past the end lands at %v, want 700", got)
	}
	if got := c.MaxOffset(); got != 700 {
		t.Errorf("MaxOffset %v, want 700", got)
	}
	c.JumpTo(-20)
	if got := c.Offset(); got != 0 {
		t.Errorf("a jump before the start lands at %v", got)
	}
	c.JumpTo(9999)
	if got := c.Offset(); got != 700 {
		t.Errorf("a jump past the end lands at %v, want 700", got)
	}
	if got := childAt(viewportOf(t, tree)); got != -700 {
		t.Errorf("content at %v, want -700", got)
	}
}
