package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
)

// timeline is a stack sized by a 120x80 box, with a fill child over it. Align
// hands it loose constraints, so only the box may decide its size.
func timeline(in geom.Insets) core.Widget {
	return Align{Child: Stack{Children: []core.Widget{
		box(120, 80, wtest.Red),
		PositionedFill{Insets: in, Child: Container{Color: wtest.Green}},
	}}}
}

func TestPositionedFill_TakesTheStackLessInsets(t *testing.T) {
	tests := []struct {
		name string
		in   geom.Insets
		want geom.Rect
	}{
		{"ゼロ値は Stack 全体", geom.Insets{}, geom.Rect{W: 120, H: 80}},
		{"余白の分だけ内側", geom.Insets{Left: 10, Top: 5, Right: 20, Bottom: 15}, geom.Rect{X: 10, Y: 5, W: 90, H: 60}},
		{"縦いっぱいの線", geom.Insets{Left: 30, Right: 89}, geom.Rect{X: 30, W: 1, H: 80}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wtest.Rects(t, timeline(tt.in), wtest.Page)
			if len(got) != 2 || got[0] != (geom.Rect{W: 120, H: 80}) || got[1] != tt.want {
				t.Errorf("got %v, want [box %v]", got, tt.want)
			}
		})
	}
}

func TestPositionedFill_LeavesTheStackSizeAlone(t *testing.T) {
	stackSize := func(w core.Widget) geom.Size {
		tree := core.NewTree(Align{Child: w})
		defer tree.Dispose()
		tree.Build()
		tree.Layout(geom.Tight(wtest.Page))
		return tree.RootRenderObject().ChildAt(0).Size()
	}
	alone := stackSize(Stack{Children: []core.Widget{box(120, 80, wtest.Red)}})
	with := stackSize(Stack{Children: []core.Widget{
		box(120, 80, wtest.Red),
		PositionedFill{Insets: geom.Insets{Left: -50, Top: -50, Right: -50, Bottom: -50}, Child: box(500, 500, wtest.Green)},
	}})
	if alone != (geom.Size{W: 120, H: 80}) || with != alone {
		t.Errorf("stack alone %v, with a fill %v, want both 120x80", alone, with)
	}
	// Only fills: the stack is as small as it may be.
	if got := stackSize(Stack{Children: []core.Widget{PositionedFill{Child: box(10, 10, wtest.Red)}}}); got != (geom.Size{}) {
		t.Errorf("stack of fills only = %v, want 0x0", got)
	}
}

func TestPositionedFill_HitTest(t *testing.T) {
	tree := core.NewTree(timeline(geom.Insets{Left: 30, Right: 80}))
	defer tree.Dispose()
	tree.Build()
	tree.Layout(geom.Tight(wtest.Page))
	hitsFill := func(x, y float32) bool {
		for _, ro := range tree.HitTest(geom.Offset{X: x, Y: y}).Path {
			if _, ok := ro.(*layout.RenderPositionedFill); ok {
				return true
			}
		}
		return false
	}
	if !hitsFill(35, 79) {
		t.Error("a point on the fill must hit it")
	}
	if hitsFill(10, 40) || hitsFill(41, 40) {
		t.Error("a point beside the fill must miss it")
	}
}

func TestPositionedFill_NewInsetsRelayout(t *testing.T) {
	in := geom.Insets{Left: 10}
	var s *wtest.SwapState
	tree := core.NewTree(wtest.Swap{Build: func() core.Widget { return timeline(in) }, State: &s})
	defer tree.Dispose()
	testutil.RenderTree(tree, wtest.Page)
	s.SetState(func() { in = geom.Insets{Left: 60, Bottom: 40} })
	tree.Build()
	tree.Layout(geom.Tight(wtest.Page))
	var fill core.RenderObject
	for _, ro := range tree.HitTest(geom.Offset{X: 61, Y: 1}).Path {
		if _, ok := ro.(*layout.RenderPositionedFill); ok {
			fill = ro
		}
	}
	if fill == nil || fill.Size() != (geom.Size{W: 60, H: 40}) || fill.Offset() != (geom.Offset{X: 60}) {
		t.Fatalf("fill after update = %v, want 60x40 at (60,0)", fill)
	}
}
