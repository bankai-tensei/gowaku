package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// sizeTeller builds a red box over half the width it is given, or a green one
// below 300, and counts its builds and keeps the constraints of the last.
type sizeTeller struct {
	builds int
	last   layout.Constraints
}

func (s *sizeTeller) build(_ core.BuildContext, c layout.Constraints) core.Widget {
	s.builds++
	s.last = c
	color := wtest.Red
	if c.MaxW < 300 {
		color = wtest.Green
	}
	return Align{Child: box(c.MaxW/2, 10, color)}
}

func rrectsOf(ops []paint.Op) []paint.Op {
	var out []paint.Op
	for _, op := range ops {
		if op.Kind == paint.OpRRect {
			out = append(out, op)
		}
	}
	return out
}

func TestLayoutBuilder_BuildsForTheConstraints(t *testing.T) {
	var s sizeTeller
	tree := core.NewTree(LayoutBuilder{Content: s.build})
	t.Cleanup(tree.Dispose)

	ops := rrectsOf(wtest.Ops(tree, wtest.Screen))
	if s.builds != 1 || s.last != geom.Tight(wtest.Screen) {
		t.Fatalf("built %d times with %v, want once with %v", s.builds, s.last, geom.Tight(wtest.Screen))
	}
	if len(ops) != 1 || ops[0].RRect.Rect != (geom.Rect{W: 200, H: 10}) || ops[0].Color != wtest.Red {
		t.Fatalf("first frame painted %v, want the red box 200 wide", ops)
	}

	// A frame with nothing to lay out builds nothing.
	tree.MarkNeedsPaint()
	wtest.Ops(tree, wtest.Screen)
	if s.builds != 1 {
		t.Errorf("a frame of the same size built %d times in all, want 1", s.builds)
	}

	// A resize builds again, before the frame paints.
	ops = rrectsOf(wtest.Ops(tree, wtest.Page))
	if s.builds != 2 || s.last != geom.Tight(wtest.Page) {
		t.Errorf("after a resize built %d times with %v", s.builds, s.last)
	}
	if len(ops) != 1 || ops[0].RRect.Rect != (geom.Rect{W: 100, H: 10}) || ops[0].Color != wtest.Green {
		t.Errorf("after a resize painted %v, want the green box 100 wide", ops)
	}
}

func TestLayoutBuilder_UnrelatedRebuildBuildsNothing(t *testing.T) {
	var s sizeTeller
	var other *wtest.HolderState
	tree := core.NewTree(Column{Children: []core.Widget{
		wtest.Holder{First: box(10, 10, wtest.Blue), State: &other},
		Expanded{Child: LayoutBuilder{Content: s.build}},
	}})
	t.Cleanup(tree.Dispose)
	wtest.Ops(tree, wtest.Screen)
	if want := (layout.Constraints{MaxW: 400, MinH: 290, MaxH: 290}); s.builds != 1 || s.last != want {
		t.Fatalf("built %d times with %v, want once with %v", s.builds, s.last, want)
	}
	// A sibling that rebuilds to the same size leaves the constraints alone.
	other.SetState(func() { other.Child = box(10, 10, wtest.Red) })
	wtest.Ops(tree, wtest.Screen)
	if s.builds != 1 {
		t.Errorf("a sibling's rebuild built the content again: %d builds", s.builds)
	}
	// One that grows changes them.
	other.SetState(func() { other.Child = box(10, 50, wtest.Red) })
	wtest.Ops(tree, wtest.Screen)
	if s.builds != 2 || s.last.MaxH != 250 {
		t.Errorf("after the sibling grew built %d times with %v", s.builds, s.last)
	}
}

func TestLayoutBuilder_ParentRebuildBuildsWithTheKnownConstraints(t *testing.T) {
	var a, b sizeTeller
	tree, swap := wtest.SwapTree(t, LayoutBuilder{Content: a.build})
	wtest.Ops(tree, wtest.Screen)
	swap(LayoutBuilder{Content: b.build})
	wtest.Ops(tree, wtest.Screen)
	if a.builds != 1 || b.builds != 1 || b.last != geom.Tight(wtest.Screen) {
		t.Errorf("builds %d and %d, the new content with %v", a.builds, b.builds, b.last)
	}
}

func TestLayoutBuilder_ZeroValue(t *testing.T) {
	tree := core.NewTree(Center{Child: LayoutBuilder{}})
	t.Cleanup(tree.Dispose)
	if ops := wtest.Ops(tree, wtest.Screen); len(ops) != 0 {
		t.Errorf("a LayoutBuilder without Content painted %v", ops)
	}
}

// findLayoutBuilder returns the first renderLayoutBuilder below ro.
func findLayoutBuilder(ro core.RenderObject) *renderLayoutBuilder {
	if r, ok := ro.(*renderLayoutBuilder); ok {
		return r
	}
	for i := range ro.ChildCount() {
		if r := findLayoutBuilder(ro.ChildAt(i)); r != nil {
			return r
		}
	}
	return nil
}

func TestLayoutBuilder_SameConstraintsDoNotAllocate(t *testing.T) {
	var s sizeTeller
	tree := core.NewTree(Center{Child: LayoutBuilder{Content: s.build}})
	t.Cleanup(tree.Dispose)
	wtest.Ops(tree, wtest.Screen)
	r := findLayoutBuilder(tree.RootRenderObject())
	c := geom.Tight(wtest.Screen)
	allocs := testing.AllocsPerRun(100, func() {
		r.MarkNeedsLayout()
		tree.Layout(c)
	})
	if allocs != 0 || tree.NeedsBuild() {
		t.Errorf("a layout with the same constraints allocated %v times, needs build %v", allocs, tree.NeedsBuild())
	}
}

func TestAspectRatio_Layout(t *testing.T) {
	tests := []struct {
		name string
		w    core.Widget
		want geom.Rect
	}{
		{"Center の中で幅いっぱい", Center{Child: AspectRatio{Ratio: 4, Child: Container{Color: wtest.Red}}}, geom.Rect{Y: 25, W: 200, H: 50}},
		{"高さが足りなければ高さに合わせる", Center{Child: AspectRatio{Ratio: 1, Child: Container{Color: wtest.Red}}}, geom.Rect{X: 50, W: 100, H: 100}},
		{"ゼロ値の比は正方形", Align{Child: SizedBox{Width: 40, Child: AspectRatio{Child: Container{Color: wtest.Red}}}}, geom.Rect{W: 40, H: 40}},
		{"tight は比を無視する", AspectRatio{Ratio: 3, Child: Container{Color: wtest.Red}}, geom.Rect{W: 200, H: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wtest.Rects(t, tt.w, wtest.Page)
			if len(got) != 1 || !wtest.NearRect(got[0], tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
