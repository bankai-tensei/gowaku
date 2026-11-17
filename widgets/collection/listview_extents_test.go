package collection

import (
	"fmt"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// trackHeight is a table header, then albums of a title row and nine tracks.
func trackHeight(i int) float32 {
	switch {
	case i == 0:
		return 32
	case i%10 == 1:
		return 24
	default:
		return 44
	}
}

func varyingRows(n int, height func(int) float32) List {
	return List{ItemCount: n, ItemExtentOf: height, ItemBuilder: func(i int) core.Widget {
		return basic.Text{Content: fmt.Sprintf("row %d", i)}
	}}
}

// topOf returns the top of row i, summed the slow way.
func topOf(i int, height func(int) float32) float64 {
	var top float64
	for k := range i {
		top += float64(height(k))
	}
	return top
}

func renderListOf(t testing.TB, tree *core.Tree) *layout.RenderList {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 10}).Path {
		if l, ok := ro.(*layout.RenderList); ok {
			return l
		}
	}
	t.Fatal("no list")
	return nil
}

// checkRows verifies that the rows that exist are placed and sized from their
// heights, and that they cover the view.
func checkRows(t *testing.T, l *layout.RenderList, view float32, height func(int) float32) {
	t.Helper()
	off := l.Position.Value()
	n := l.ChildCount()
	if n == 0 {
		t.Fatal("no rows")
	}
	for k := range n {
		i := l.First + k
		c := l.ChildAt(k)
		if got, want := c.Offset().Y, float32(topOf(i, height)-off); got != want {
			t.Errorf("row %d at %v, want %v", i, got, want)
		}
		if got := c.Size().H; got != height(i) {
			t.Errorf("row %d is %v high, want %v", i, got, height(i))
		}
	}
	first, last := l.ChildAt(0), l.ChildAt(n-1)
	if first.Offset().Y > 0 {
		t.Errorf("the first row starts at %v, leaving a gap at the top", first.Offset().Y)
	}
	if bottom := last.Offset().Y + last.Size().H; bottom < view {
		t.Errorf("the rows end at %v, leaving a gap above %v", bottom, view)
	}
}

func TestListView_VaryingExtents_PlacesRows(t *testing.T) {
	tree := wtest.TextTree(t, varyingRows(1000, trackHeight))
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	if l.Extents == nil {
		t.Fatal("ItemExtentOf must give the list a table")
	}
	if l.First != 0 {
		t.Errorf("First = %d", l.First)
	}
	// 32 + 24 + 44 = 100: rows 0..2 fill the view, and three more are cached.
	if got := l.ChildCount(); got != 3+1+cachedItems {
		t.Errorf("%d rows exist", got)
	}
	checkRows(t, l, size.H, trackHeight)
}

func TestListView_VaryingExtents_VisibleRangeWhileScrolling(t *testing.T) {
	tree := wtest.TextTree(t, varyingRows(1000, trackHeight))
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	for _, notches := range []float32{-1, -3.3, -7, 2.5, -40} {
		wtest.Wheel(tree, notches)
		wtest.RunUntilIdle(t, tree, size, nil)
		off := l.Position.Value()
		var tab layout.ExtentTable
		tab.Reset(1000, trackHeight)
		want := max(tab.IndexAt(off)-cachedItems, 0)
		if l.First != want {
			t.Errorf("at %v First = %d, want %d", off, l.First, want)
		}
		checkRows(t, l, size.H, trackHeight)
	}
}

func TestListView_VaryingExtents_ScrollToTheEnd(t *testing.T) {
	const n = 1000
	tree := wtest.TextTree(t, varyingRows(n, trackHeight))
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	for range 500 {
		wtest.Wheel(tree, -1)
	}
	wtest.RunUntilIdle(t, tree, size, nil)
	l := renderListOf(t, tree)
	total := topOf(n, trackHeight)
	if got := l.Position.Value(); got != total-float64(size.H) {
		t.Errorf("the end is at %v, want %v", got, total-float64(size.H))
	}
	last := l.ChildAt(l.ChildCount() - 1)
	if l.First+l.ChildCount() != n {
		t.Errorf("the last row must exist, rows end at %d", l.First+l.ChildCount())
	}
	if bottom := last.Offset().Y + last.Size().H; bottom != size.H {
		t.Errorf("the last row ends at %v, want %v", bottom, size.H)
	}
	checkRows(t, l, size.H, trackHeight)
}

func TestListView_VaryingExtents_ScrollBarMeasuresAllRows(t *testing.T) {
	tree := wtest.TextTree(t, varyingRows(100, trackHeight))
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	var bar *layout.RenderScrollBar
	for _, ro := range tree.HitTest(geom.Offset{X: 198, Y: 50}).Path {
		if b, ok := ro.(*layout.RenderScrollBar); ok {
			bar = b
		}
	}
	if bar == nil {
		t.Fatal("no scroll bar")
	}
	if got, want := bar.Metrics.Content, topOf(100, trackHeight); got != want {
		t.Errorf("content = %v, want %v", got, want)
	}
}

func TestListView_VaryingExtents_ItemCountAndHeightsChange(t *testing.T) {
	var h *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: varyingRows(1000, trackHeight), State: &h})
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	for range 500 {
		wtest.Wheel(tree, -1)
	}
	wtest.RunUntilIdle(t, tree, size, nil)
	l := renderListOf(t, tree)

	// Fewer rows: the view moves back to the new end.
	h.SetState(func() { h.Child = varyingRows(50, trackHeight) })
	testutil.RenderTree(tree, size)
	wtest.RunUntilIdle(t, tree, size, nil)
	if got, want := l.Position.Value(), topOf(50, trackHeight)-float64(size.H); got != want {
		t.Errorf("after shrinking the end is at %v, want %v", got, want)
	}
	if l.First+l.ChildCount() != 50 {
		t.Errorf("rows end at %d, want 50", l.First+l.ChildCount())
	}
	checkRows(t, l, size.H, trackHeight)

	// More rows: the table grows, and the view can reach the new end.
	h.SetState(func() { h.Child = varyingRows(3000, trackHeight) })
	testutil.RenderTree(tree, size)
	for range 2000 {
		wtest.Wheel(tree, -1)
	}
	wtest.RunUntilIdle(t, tree, size, nil)
	if got, want := l.Position.Value(), topOf(3000, trackHeight)-float64(size.H); got != want {
		t.Errorf("after growing the end is at %v, want %v", got, want)
	}
	checkRows(t, l, size.H, trackHeight)

	// Same count, other heights: a parent rebuild reads them again.
	double := func(i int) float32 { return 2 * trackHeight(i) }
	h.SetState(func() { h.Child = varyingRows(3000, double) })
	testutil.RenderTree(tree, size)
	checkRows(t, l, size.H, double)
}

func TestListView_VaryingExtents_HiddenRowsAndEmpty(t *testing.T) {
	hidden := func(i int) float32 {
		if i%2 == 1 {
			return 0
		}
		return 30
	}
	tree := wtest.TextTree(t, varyingRows(100, hidden))
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	wtest.Wheel(tree, -2)
	wtest.RunUntilIdle(t, tree, size, nil)
	checkRows(t, renderListOf(t, tree), size.H, hidden)

	for _, n := range []int{0, 2} {
		tree := wtest.TextTree(t, varyingRows(n, trackHeight))
		testutil.RenderTree(tree, size)
		wtest.Wheel(tree, -3)
		wtest.RunUntilIdle(t, tree, size, nil)
	}
}

func TestListView_VaryingExtents_NegativeHeightPanics(t *testing.T) {
	wtest.ExpectPanic(t, "ItemExtentOf returned a negative", func() {
		testutil.RenderTree(wtest.TextTree(t, varyingRows(3, func(int) float32 { return -1 })), geom.Size{W: 200, H: 100})
	})
}

// The fixed path keeps no table, and a constant ItemExtentOf draws exactly what
// ItemExtent draws.
func TestListView_FixedExtentPathUnchanged(t *testing.T) {
	size := geom.Size{W: 200, H: 100}
	fixed := wtest.TextTree(t, rowTexts(1000))
	varying := wtest.TextTree(t, varyingRows(1000, func(int) float32 { return 20 }))
	for step, notches := range []float32{0, -1, -2.5, -30, 4} {
		if notches != 0 {
			wtest.Wheel(fixed, notches)
			wtest.Wheel(varying, notches)
			wtest.RunUntilIdle(t, fixed, size, nil)
			wtest.RunUntilIdle(t, varying, size, nil)
		}
		a, b := testutil.RenderTree(fixed, size), testutil.RenderTree(varying, size)
		if a != b {
			t.Errorf("step %d: the frames differ\nfixed:\n%s\nvarying:\n%s", step, a, b)
		}
	}
	if l := renderListOf(t, fixed); l.Extents != nil || l.Extent != 20 {
		t.Errorf("the fixed path has a table %v or extent %v", l.Extents, l.Extent)
	}
	zero := wtest.TextTree(t, List{ItemCount: 10, ItemBuilder: func(int) core.Widget { return basic.SizedBox{} }})
	testutil.RenderTree(zero, size)
	if l := renderListOf(t, zero); l.Extent != defaultItemExtent || l.Extents != nil {
		t.Errorf("the zero value must keep rows of %v", defaultItemExtent)
	}
}

// settleMidRow scrolls a list of 10,000 varying rows to near its middle, then
// to an offset where neither edge of the view is within margin of the edge of
// a row, so that a small motion brings no other rows into range.
func settleMidRow(t testing.TB, tree *core.Tree, size geom.Size, height func(int) float32, margin float32) {
	t.Helper()
	testutil.RenderTree(tree, size)
	wtest.Wheel(tree, -2000)
	wtest.RunUntilIdle(t, tree, size, nil)
	l := renderListOf(t, tree)
	var tab layout.ExtentTable
	tab.Reset(10000, height)
	clear := func(y float64) bool {
		i := tab.IndexAt(y)
		return y-tab.Offset(i) > float64(margin) && tab.Offset(i+1)-y > float64(margin)
	}
	from := l.Position.Value()
	to := from - 500
	for p := to; p < from; p++ {
		if clear(p) && clear(p+float64(size.H)) {
			to = p
			break
		}
	}
	wtest.Wheel(tree, float32((from-to)/kit.NotchDistance))
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := l.Position.Value(); got != to {
		t.Fatalf("settled at %v, want %v", got, to)
	}
}

// scrollFrameRunner plays frames of a small wheel motion back and forth. The
// wheel goes straight to the list's OnScroll: the boxing of the event in
// core.Tree.DispatchPointer is not the list's to measure.
type scrollFrameRunner struct {
	tree *core.Tree
	list *layout.RenderList
	size geom.Size
	cv   paint.Canvas
	now  time.Duration
	dir  float32
}

// frame is one frame: the wheel when the view is still, tick, the check for
// rows coming into range, layout and paint.
func (r *scrollFrameRunner) frame() {
	if !r.tree.Ticking() {
		r.dir = -r.dir
		r.list.OnScroll(r.dir * 0.03)
	}
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	r.tree.Build()
	r.tree.Layout(geom.Tight(r.size))
	r.cv.Reset()
	r.tree.Paint(&r.cv)
}

func newScrollFrameRunner(t testing.TB) *scrollFrameRunner {
	tree := wtest.TextTree(t, varyingRows(10000, trackHeight))
	r := &scrollFrameRunner{tree: tree, size: geom.Size{W: 400, H: 600}, now: time.Hour, dir: 1}
	settleMidRow(t, tree, r.size, trackHeight, 5)
	r.list = renderListOf(t, tree)
	// Warm up: the first motion creates the spring, the bar's fade and the
	// canvas buffers.
	for range 200 {
		r.frame()
	}
	return r
}

func TestListView_VaryingExtents_ScrollingDoesNotAllocate(t *testing.T) {
	r := newScrollFrameRunner(t)
	first := r.list.First
	p0 := r.list.Position.Value()
	moved := false
	for range 50 {
		r.frame()
		moved = moved || r.list.Position.Value() != p0
	}
	if !moved {
		t.Fatal("the view must move")
	}
	// One run of many frames, so that a rare allocation is not averaged away.
	allocs := testing.AllocsPerRun(1, func() {
		for range 1000 {
			r.frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs in 1000 frames", allocs)
	}
	if r.list.First != first {
		t.Error("the motion must stay within the rows that exist")
	}
}

// BenchmarkListView_VaryingScrollFrame_10k is one frame of scrolling a list of
// 10,000 rows of three heights, while the same rows stay in range. Frames that
// bring new rows in build their widgets and allocate, as the fixed path does
// (see BenchmarkListView_ScrollFrame).
func BenchmarkListView_VaryingScrollFrame_10k(b *testing.B) {
	r := newScrollFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}
