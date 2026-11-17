package collection

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// gridFrameRunner plays frames of a small wheel motion back and forth over a
// grid of 100,000 thumbnails, as treeFrameRunner does for a tree.
type gridFrameRunner struct {
	tree  *core.Tree
	state *gridState
	size  geom.Size
	cv    paint.Canvas
	now   time.Duration
	dir   float32
}

func newGridFrameRunner(t testing.TB) *gridFrameRunner {
	colors := [...]geom.Color{wtest.Red, geom.Hex(0x3070d0), geom.Hex(0x40a060)}
	c := &TableController{}
	c.Select(5)
	g := GridView{
		ItemCount: 100000, MinColumnWidth: 88, Spacing: 8, RunSpacing: 8, ExtraHeight: 20,
		Padding: geom.All(12), Controller: c,
		ItemBuilder: func(i int) core.Widget {
			return basic.Container{Color: colors[i%len(colors)], Radius: 6}
		},
	}
	tree := wtest.TextTree(t, g)
	r := &gridFrameRunner{tree: tree, size: geom.Size{W: 800, H: 600}, now: time.Hour, dir: 1}
	wtest.Ops(tree, r.size)
	r.state = gridBodyOf(t, tree).host
	if n := r.state.geo.cols; n != 8 {
		t.Fatalf("%d columns", n)
	}
	// Go to the middle, between rows, and warm up.
	g8 := &r.state.geo
	r.state.Jump(g8.rowTop(6000) + float64(g8.stride)/2)
	wtest.Ops(tree, r.size)
	for range 200 {
		r.frame()
	}
	return r
}

// frame is one frame: the wheel when the view is still, tick, the check for
// rows coming into range, layout and paint.
func (r *gridFrameRunner) frame() {
	if !r.tree.Ticking() {
		r.dir = -r.dir
		r.state.Wheel(r.dir * 0.03)
	}
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	r.tree.Build()
	r.tree.Layout(geom.Tight(r.size))
	r.cv.Reset()
	r.tree.Paint(&r.cv)
}

// BenchmarkGridView_ScrollFrame_100k is one frame of scrolling a grid of
// 100,000 thumbnails while the same rows stay in range. Frames that bring new
// rows in build their widgets and allocate, as List does.
func BenchmarkGridView_ScrollFrame_100k(b *testing.B) {
	r := newGridFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}

// BenchmarkGridView_LayoutPaint_100k lays out and paints the tiles in view of
// a grid of 100,000 thumbnails, from scratch.
func BenchmarkGridView_LayoutPaint_100k(b *testing.B) {
	r := newGridFrameRunner(b)
	body := gridBodyOf(b, r.tree)
	b.ReportAllocs()
	for b.Loop() {
		for _, row := range body.children {
			row.MarkNeedsLayout()
			for i := range row.ChildCount() {
				row.ChildAt(i).MarkNeedsLayout()
			}
		}
		r.tree.Layout(geom.Tight(r.size))
		r.cv.Reset()
		r.tree.MarkNeedsPaint()
		r.tree.Paint(&r.cv)
	}
}

// BenchmarkGridView_RowsIntoRange_100k is a frame that scrolls a whole view
// of rows into range: the tiles are built, which allocates.
func BenchmarkGridView_RowsIntoRange_100k(b *testing.B) {
	r := newGridFrameRunner(b)
	b.ReportAllocs()
	page := r.size.H
	for b.Loop() {
		r.dir = -r.dir
		r.state.Jump(r.state.Offset.Value() + float64(r.dir*page))
		r.tree.Build()
		r.tree.Layout(geom.Tight(r.size))
		for r.tree.NeedsBuild() {
			r.tree.Build()
			r.tree.Layout(geom.Tight(r.size))
		}
		r.cv.Reset()
		r.tree.Paint(&r.cv)
	}
}
