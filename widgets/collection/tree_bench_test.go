package collection

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// bigTree is 1,000 roots of 100 children each, all open: 101,000 rows.
type bigTree struct {
	roots    []int
	children [][]int
}

func newBigTree() *bigTree {
	b := &bigTree{roots: make([]int, 1000), children: make([][]int, 1000)}
	for i := range b.roots {
		b.roots[i] = -1 - i
		kids := make([]int, 100)
		for k := range kids {
			kids[k] = i*100 + k
		}
		b.children[i] = kids
	}
	return b
}

func (b *bigTree) widget(c *TreeController[int]) core.Widget {
	labels := [2]string{"Folder", "file.txt"}
	return Tree[int]{
		Roots: b.roots,
		ChildrenOf: func(k int) []int {
			if k < 0 {
				return b.children[-1-k]
			}
			return nil
		},
		LabelOf: func(k int) string {
			if k < 0 {
				return labels[0]
			}
			return labels[1]
		},
		Controller:  c,
		Multiselect: true,
	}
}

// treeFrameRunner plays frames of a small wheel motion back and forth over a
// tree of 101,000 open rows, as scrollFrameRunner does for a list.
type treeFrameRunner struct {
	tree  *core.Tree
	state *treeState[int]
	size  geom.Size
	cv    paint.Canvas
	now   time.Duration
	dir   float32
}

func newTreeFrameRunner(t testing.TB) *treeFrameRunner {
	big := newBigTree()
	c := &TreeController[int]{}
	for _, r := range big.roots {
		c.Expand(r)
	}
	c.SetSelected(5, true)
	tree := wtest.TextTree(t, big.widget(c))
	r := &treeFrameRunner{tree: tree, size: geom.Size{W: 400, H: 600}, now: time.Hour, dir: 1}
	wtest.Ops(tree, r.size)
	r.state = treeBodyOf(t, tree).host.(*treeState[int])
	if n := len(r.state.rows); n != 101000 {
		t.Fatalf("%d rows", n)
	}
	// Go to the middle, between rows, and warm up.
	r.state.Jump(50000*treeRowHeight + treeRowHeight/2)
	wtest.Ops(tree, r.size)
	for range 200 {
		r.frame()
	}
	return r
}

// frame is one frame: the wheel when the view is still, tick, the check for
// rows coming into range, layout and paint.
func (r *treeFrameRunner) frame() {
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

func TestTreeView_ScrollingDoesNotAllocate(t *testing.T) {
	r := newTreeFrameRunner(t)
	first := r.state.first
	p0 := r.state.Offset.Value()
	moved := false
	for range 50 {
		r.frame()
		moved = moved || r.state.Offset.Value() != p0
	}
	if !moved {
		t.Fatal("the view must move")
	}
	allocs := testing.AllocsPerRun(1, func() {
		for range 1000 {
			r.frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs in 1000 frames", allocs)
	}
	if r.state.first != first {
		t.Error("the motion must stay within the rows that exist")
	}
}

// BenchmarkTreeView_ScrollFrame_100k is one frame of scrolling a tree of
// 101,000 open rows while the same rows stay in range. Frames that bring
// new rows in build their widgets and allocate, as List does.
func BenchmarkTreeView_ScrollFrame_100k(b *testing.B) {
	r := newTreeFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}

// BenchmarkTreeView_LayoutPaint_100k lays out and paints the rows in view of
// a tree of 101,000 open rows, from scratch.
func BenchmarkTreeView_LayoutPaint_100k(b *testing.B) {
	r := newTreeFrameRunner(b)
	body := treeBodyOf(b, r.tree)
	b.ReportAllocs()
	for b.Loop() {
		for _, c := range body.children {
			c.MarkNeedsLayout()
		}
		r.tree.Layout(geom.Tight(r.size))
		r.cv.Reset()
		r.tree.MarkNeedsPaint()
		r.tree.Paint(&r.cv)
	}
}

// BenchmarkTreeView_RowsIntoRange_100k is a frame that scrolls a whole
// view of rows into range: the rows are built, which allocates.
func BenchmarkTreeView_RowsIntoRange_100k(b *testing.B) {
	r := newTreeFrameRunner(b)
	b.ReportAllocs()
	page := r.size.H
	for b.Loop() {
		r.dir = -r.dir
		r.state.Jump(r.state.Offset.Value() + float64(r.dir*page))
		r.tree.Build()
		r.tree.Layout(geom.Tight(r.size))
		r.cv.Reset()
		r.tree.Paint(&r.cv)
	}
}

// BenchmarkTreeView_Flatten_100k reads the data of 101,000 open rows again,
// as a parent rebuild or an opening does. It reuses the rows, so it does
// not allocate once warm.
func BenchmarkTreeView_Flatten_100k(b *testing.B) {
	r := newTreeFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.state.flatten()
	}
}
