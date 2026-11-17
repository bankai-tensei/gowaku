package basic

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

var clipPage = geom.Size{W: 200, H: 200}

// clipped puts clip, a 100x100 box, in the middle of the page over a full
// card, with a colored square as the clipped child.
func clipped(clip func(core.Widget) core.Widget, child core.Widget) core.Widget {
	return Center{Child: SizedBox{Width: 100, Height: 100, Child: clip(child)}}
}

func square() core.Widget { return Container{Color: geom.Hex(0x2f5fd0)} }

func hitsBox(path []core.RenderObject) bool {
	for _, o := range path {
		if _, ok := o.(*layout.RenderDecoratedBox); ok {
			return true
		}
	}
	return false
}

func findClip(t *testing.T, ro core.RenderObject) *layout.RenderClip {
	t.Helper()
	c := findClipOrNil(ro)
	if c == nil {
		t.Fatal("no RenderClip in the tree")
	}
	return c
}

func findClipOrNil(ro core.RenderObject) *layout.RenderClip {
	if c, ok := ro.(*layout.RenderClip); ok {
		return c
	}
	for i := range ro.ChildCount() {
		if c := findClipOrNil(ro.ChildAt(i)); c != nil {
			return c
		}
	}
	return nil
}

func TestClipRect_CutsThePainting(t *testing.T) {
	// The child is moved half out of the box: what is outside must not show.
	w := clipped(func(c core.Widget) core.Widget { return ClipRect{Child: c} },
		Transform{Transform: geom.Translation(geom.Offset{X: 50}), Child: square()})
	got := testutil.Render(w, clipPage)
	if !strings.Contains(got, "clip=50,50,100,100") {
		t.Errorf("the square must be cut to the box at 50,50:\n%s", got)
	}
}

func TestClipRRect_CutsWithRoundCorners(t *testing.T) {
	w := clipped(func(c core.Widget) core.Widget { return ClipRRect{Radius: 20, Child: c} }, square())
	tree := core.NewTree(w)
	defer tree.Dispose()
	plain := testutil.Render(clipped(func(c core.Widget) core.Widget { return c }, square()), clipPage)
	if got := testutil.RenderTree(tree, clipPage); got == plain {
		t.Errorf("a rounded clip must change the picture:\n%s", got)
	}
	if r := findClip(t, tree.RootRenderObject()); r.Radius != 20 {
		t.Errorf("radius %g, want 20", r.Radius)
	}
}

func TestClipRRect_ZeroRadiusTakesTheCardCorners(t *testing.T) {
	th := theme.Default().Light
	tree := core.NewTree(Theme{Data: th, Child: clipped(func(c core.Widget) core.Widget { return ClipRRect{Child: c} }, square())})
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	if r := findClip(t, tree.RootRenderObject()); r.Radius != th.Radius.Group {
		t.Errorf("radius %g, want the theme's Group %g", r.Radius, th.Radius.Group)
	}
}

func TestClipRRect_PointerOnlyInsideTheShape(t *testing.T) {
	tree := core.NewTree(clipped(func(c core.Widget) core.Widget { return ClipRRect{Radius: 30, Child: c} }, square()))
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	for _, c := range []struct {
		name string
		pos  geom.Offset
		want bool
	}{
		{"middle", geom.Offset{X: 100, Y: 100}, true},
		{"middle of the left edge", geom.Offset{X: 51, Y: 100}, true},
		{"cut-off top left corner", geom.Offset{X: 53, Y: 53}, false},
		{"cut-off bottom right corner", geom.Offset{X: 147, Y: 147}, false},
		{"inside the round of the top right corner", geom.Offset{X: 135, Y: 65}, true},
		{"outside the box", geom.Offset{X: 20, Y: 100}, false},
	} {
		if got := hitsBox(tree.HitTest(c.pos).Path); got != c.want {
			t.Errorf("%s %v: hit %v, want %v", c.name, c.pos, got, c.want)
		}
	}
}

func TestClipRect_PointerNotOutsideTheBox(t *testing.T) {
	// The picture of the square is moved past the box; the pointer, which
	// follows a Transform, must not find it there.
	tree := core.NewTree(clipped(func(c core.Widget) core.Widget { return ClipRect{Child: c} },
		Transform{Transform: geom.Translation(geom.Offset{X: 50}), Child: square()}))
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	if !hitsBox(tree.HitTest(geom.Offset{X: 140, Y: 100}).Path) {
		t.Error("the visible part of the square must be hit")
	}
	if hitsBox(tree.HitTest(geom.Offset{X: 180, Y: 100}).Path) {
		t.Error("the cut-off part of the square must not be hit")
	}
}

func TestClipRect_EmptyPartLetsThePointerThrough(t *testing.T) {
	// ADR 0142: the clip draws nothing, so where its child is not hit, the
	// layer below is.
	tree := core.NewTree(Stack{Children: []core.Widget{
		PositionedFill{Child: Container{Color: geom.Hex(0xeeeeee)}},
		PositionedFill{Child: ClipRect{Child: Align{Alignment: layout.AlignTopLeft, Child: SizedBox{Width: 10, Height: 10}}}},
	}})
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	if !hitsBox(tree.HitTest(geom.Offset{X: 100, Y: 100}).Path) {
		t.Error("the empty part of the clip must let the pointer reach the layer below")
	}
}

func TestClip_IsAClipper(t *testing.T) {
	tree := core.NewTree(clipped(func(c core.Widget) core.Widget { return ClipRRect{Radius: 8, Child: c} }, square()))
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	var r core.RenderObject = findClip(t, tree.RootRenderObject())
	c, ok := r.(core.Clipper)
	if !ok {
		t.Fatal("RenderClip must implement core.Clipper")
	}
	if rect, on := c.ChildClip(); !on || rect != (geom.Rect{W: 100, H: 100}) {
		t.Errorf("ChildClip %v %v, want the 100x100 box", rect, on)
	}
}

func TestClip_PaintDoesNotAllocate(t *testing.T) {
	for _, radius := range []float32{0, 12} {
		tree := core.NewTree(clipped(func(c core.Widget) core.Widget { return clipBox{radius: radius, child: c} }, square()))
		testutil.RenderTree(tree, clipPage)
		var cv paint.Canvas
		cv.Reset()
		tree.Paint(&cv)
		allocs := testing.AllocsPerRun(20, func() {
			cv.Reset()
			tree.Paint(&cv)
		})
		tree.Dispose()
		if allocs != 0 {
			t.Errorf("radius %g: %g allocations per paint, want 0", radius, allocs)
		}
	}
}

func TestTranslate_ClipCutsAtRest(t *testing.T) {
	// Clip keeps the painting in the box also when X and Y are 0.
	w := Center{Child: SizedBox{Width: 100, Height: 100, Child: Translate{Clip: true,
		Child: Transform{Transform: geom.Translation(geom.Offset{X: 50}), Child: square()}}}}
	if got := testutil.Render(w, clipPage); !strings.Contains(got, "clip=50,50,100,100") {
		t.Errorf("the square must be cut to the box:\n%s", got)
	}
	tree := core.NewTree(Translate{Clip: true, Child: square()})
	defer tree.Dispose()
	testutil.RenderTree(tree, clipPage)
	if _, on := tree.RootRenderObject().(core.Clipper).ChildClip(); !on {
		t.Error("a Translate with Clip at rest must report its clip")
	}
}
