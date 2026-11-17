package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// tall is a page tall enough to show what a Scroll holds, which still gives
// its child an unbounded height.
var tall = geom.Size{W: 200, H: 300}

// boxes returns n boxes of the same size.
func boxes(n int, w, h float32) []core.Widget {
	out := make([]core.Widget, n)
	for i := range out {
		out[i] = box(w, h, wtest.Red)
	}
	return out
}

// placementCase is a widget and the rectangles it should draw on page.
type placementCase struct {
	name string
	w    core.Widget
	want []geom.Rect
}

func checkPlacement(t *testing.T, tests []placementCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRects(t, wtest.Rects(t, tt.w, wtest.Page), tt.want)
		})
	}
}

func checkRects(t *testing.T, got, want []geom.Rect) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if !wtest.NearRect(got[i], want[i]) {
			t.Errorf("rect %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

// treeRects lays out and paints a live tree, and returns the rectangle of every
// command.
func treeRects(tree *core.Tree, size geom.Size) []geom.Rect {
	tree.Build()
	tree.Layout(geom.Tight(size))
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	var out []geom.Rect
	for _, op := range cv.DisplayList().Ops {
		out = append(out, op.RRect.Rect)
	}
	return out
}

func TestWrap_Placement(t *testing.T) {
	checkPlacement(t, []placementCase{
		{"ちょうど収まる子は折り返さない", Wrap{Spacing: 10, RunSpacing: 5, Children: boxes(4, 60, 20)},
			[]geom.Rect{{W: 60, H: 20}, {X: 70, W: 60, H: 20}, {X: 140, W: 60, H: 20}, {Y: 25, W: 60, H: 20}}},
		{"収まらない子は次の行へ", Wrap{Spacing: 10, Children: boxes(3, 95, 20)},
			[]geom.Rect{{W: 95, H: 20}, {X: 105, W: 95, H: 20}, {Y: 20, W: 95, H: 20}}},
		{"幅より長い子は幅に縮めて 1 行", Wrap{Children: []core.Widget{box(30, 10, wtest.Red), box(300, 10, wtest.Green), box(30, 10, wtest.Blue)}},
			[]geom.Rect{{W: 30, H: 10}, {Y: 10, W: 200, H: 10}, {Y: 20, W: 30, H: 10}}},
		{"行の高さは一番高い子", Wrap{Children: []core.Widget{box(150, 10, wtest.Red), box(40, 30, wtest.Green), box(50, 10, wtest.Blue)}},
			[]geom.Rect{{W: 150, H: 10}, {X: 150, W: 40, H: 30}, {Y: 30, W: 50, H: 10}}},
		{"Main: MainCenter は行ごと", Wrap{Main: layout.MainCenter, Spacing: 10, Children: boxes(4, 50, 10)},
			[]geom.Rect{{X: 15, W: 50, H: 10}, {X: 75, W: 50, H: 10}, {X: 135, W: 50, H: 10}, {X: 75, Y: 10, W: 50, H: 10}}},
		{"Main: MainEnd", Wrap{Main: layout.MainEnd, Children: boxes(1, 50, 10)}, []geom.Rect{{X: 150, W: 50, H: 10}}},
		{"Main: MainSpaceBetween", Wrap{Main: layout.MainSpaceBetween, Spacing: 10, Children: boxes(3, 50, 10)},
			[]geom.Rect{{W: 50, H: 10}, {X: 75, W: 50, H: 10}, {X: 150, W: 50, H: 10}}},
		{"Cross: CrossCenter は行の中で", Wrap{Cross: layout.CrossCenter, Children: []core.Widget{box(50, 20, wtest.Red), box(50, 10, wtest.Green)}},
			[]geom.Rect{{W: 50, H: 20}, {X: 50, Y: 5, W: 50, H: 10}}},
		{"Cross: CrossEnd", Wrap{Cross: layout.CrossEnd, Children: []core.Widget{box(50, 20, wtest.Red), box(50, 10, wtest.Green)}},
			[]geom.Rect{{W: 50, H: 20}, {X: 50, Y: 10, W: 50, H: 10}}},
		{"Cross: CrossStretch は行の高さに揃える", Wrap{Cross: layout.CrossStretch, Children: []core.Widget{box(50, 20, wtest.Red), box(50, 10, wtest.Green)}},
			[]geom.Rect{{W: 50, H: 20}, {X: 50, W: 50, H: 20}}},
		{"RunAlignment: MainEnd は行の束を下へ", Wrap{RunAlignment: layout.MainEnd, RunSpacing: 5, Children: boxes(2, 150, 20)},
			[]geom.Rect{{Y: 55, W: 150, H: 20}, {Y: 80, W: 150, H: 20}}},
		{"RunAlignment: MainSpaceBetween", Wrap{RunAlignment: layout.MainSpaceBetween, Children: boxes(2, 150, 20)},
			[]geom.Rect{{W: 150, H: 20}, {Y: 80, W: 150, H: 20}}},
		{"縦の Wrap は列で折り返す", Wrap{Axis: layout.Vertical, Spacing: 10, RunSpacing: 4, Children: boxes(3, 20, 40)},
			[]geom.Rect{{W: 20, H: 40}, {Y: 50, W: 20, H: 40}, {X: 24, W: 20, H: 40}}},
		{"Size: MainMin は最長の行に縮む", Center{Child: Wrap{Size: layout.MainMin, Spacing: 10, Children: boxes(3, 70, 10)}},
			[]geom.Rect{{X: 25, Y: 40, W: 70, H: 10}, {X: 105, Y: 40, W: 70, H: 10}, {X: 25, Y: 50, W: 70, H: 10}}},
		{"長さに上限が無ければ折り返さない", Row{Children: []core.Widget{Wrap{Spacing: 10, Children: boxes(3, 90, 10)}}},
			[]geom.Rect{{W: 90, H: 10}, {X: 100, W: 90, H: 10}, {X: 200, W: 90, H: 10}}},
	})
}

func TestWrap_InVerticalScroll(t *testing.T) {
	got := wtest.Rects(t, Scroll{Child: Wrap{Spacing: 10, RunSpacing: 10, Children: boxes(9, 60, 40)}}, tall)
	// Three rows of three, all in the view.
	if len(got) < 9 {
		t.Fatalf("got %v", got)
	}
	if want := (geom.Rect{X: 140, Y: 100, W: 60, H: 40}); !wtest.NearRect(got[8], want) {
		t.Errorf("last box: got %v, want %v", got[8], want)
	}
}

func TestWrap_ZeroValue(t *testing.T) {
	if got := testutil.Render(Wrap{}, wtest.Page); got != "" {
		t.Errorf("got %q, want nothing drawn", got)
	}
}

func TestWrap_UpdateRelayouts(t *testing.T) {
	spacing := float32(0)
	var s *wtest.SwapState
	tree := core.NewTree(wtest.Swap{Build: func() core.Widget { return Wrap{Spacing: spacing, Children: boxes(2, 95, 10)} }, State: &s})
	defer tree.Dispose()
	checkRects(t, treeRects(tree, wtest.Page), []geom.Rect{{W: 95, H: 10}, {X: 95, W: 95, H: 10}})
	s.SetState(func() { spacing = 20 })
	checkRects(t, treeRects(tree, wtest.Page), []geom.Rect{{W: 95, H: 10}, {Y: 10, W: 95, H: 10}})
}

func BenchmarkLayout_Wrap1k(b *testing.B) {
	children := make([]core.Widget, 1000)
	for i := range children {
		children[i] = box(float32(20+i%7*5), float32(10+i%3*4), wtest.Red)
	}
	tree := core.NewTree(Scroll{Child: Wrap{
		Main: layout.MainSpaceBetween, Cross: layout.CrossStretch, Spacing: 4, RunSpacing: 4, Children: children,
	}})
	testutil.RenderTree(tree, wtest.Page)
	c1, c2 := geom.Tight(wtest.Page), geom.Tight(geom.Size{W: 237, H: 100})
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		if flip {
			tree.Layout(c2)
		} else {
			tree.Layout(c1)
		}
	}
}
