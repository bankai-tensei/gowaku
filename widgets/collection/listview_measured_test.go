package collection

import (
	"fmt"
	"strings"
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

// noteHeight gives rows of 20 to 84 pixels that nobody tells the list.
func noteHeight(i int) float32 { return 20 + float32(i*37%5)*16 }

// measuredRows is a list of n rows as tall as height says, which it learns
// only by laying them out. built counts the rows it builds, if not nil.
func measuredRows(n int, height func(int) float32, c *ListController, built *int) List {
	return List{
		ItemCount: n, MeasureItems: true, EstimatedItemExtent: 100, Controller: c,
		ItemBuilder: func(i int) core.Widget {
			if built != nil {
				*built++
			}
			return basic.SizedBox{Height: height(i), Child: basic.Text{Content: fmt.Sprintf("row %d", i)}}
		},
	}
}

// checkMeasuredRows verifies that the rows that exist have their heights,
// follow one another without gaps, and cover the view.
func checkMeasuredRows(t *testing.T, l *layout.RenderList, view float32, height func(int) float32) {
	t.Helper()
	n := l.ChildCount()
	if n == 0 {
		t.Fatal("no rows")
	}
	for k := range n {
		i := l.First + k
		c := l.ChildAt(k)
		if got := c.Size().H; got != height(i) {
			t.Errorf("row %d is %v high, want %v", i, got, height(i))
		}
		if !l.Measured.Measured(i) || l.Measured.Extent(i) != height(i) {
			t.Errorf("row %d is not recorded at its height", i)
		}
		if k > 0 {
			prev := l.ChildAt(k - 1)
			// Each row is rounded to float32 from its own float64 top, so
			// the sum of the row before may be off by the last bit.
			if got, want := c.Offset().Y, prev.Offset().Y+prev.Size().H; !nearly(got, want) {
				t.Errorf("row %d at %v, want %v right under the row before", i, got, want)
			}
		}
		if got, want := c.Offset().Y, float32(l.Measured.Offset(i)-l.Position.Value()); !nearly(got, want) {
			t.Errorf("row %d at %v, but the table puts it at %v", i, got, want)
		}
	}
	first, last := l.ChildAt(0), l.ChildAt(n-1)
	if first.Offset().Y > 0 {
		t.Errorf("the first row starts at %v, leaving a gap at the top", first.Offset().Y)
	}
	if bottom := last.Offset().Y + last.Size().H; bottom < view && l.First+n < l.Measured.Len() {
		t.Errorf("the rows end at %v, leaving a gap above %v", bottom, view)
	}
}

func nearly(a, b float32) bool {
	d := a - b
	return d > -0.01 && d < 0.01
}

// screenY returns where row i is in the view, and whether it exists.
func screenY(l *layout.RenderList, i int) (float32, bool) {
	k := i - l.First
	if k < 0 || k >= l.ChildCount() {
		return 0, false
	}
	return l.ChildAt(k).Offset().Y, true
}

func TestListView_Measured_PlacesRowsFromTheTop(t *testing.T) {
	tree := wtest.TextTree(t, measuredRows(1000, noteHeight, nil, nil))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	if l.Measured == nil || l.Extents != nil {
		t.Fatal("MeasureItems must give the list measured heights")
	}
	checkRows(t, l, size.H, noteHeight)
	checkMeasuredRows(t, l, size.H, noteHeight)
	// Only the rows near the view are laid out.
	if got := l.Measured.MeasuredCount(); got != l.ChildCount() {
		t.Errorf("%d rows measured, %d exist", got, l.ChildCount())
	}
}

func TestListView_Measured_ScrollsThroughTheRows(t *testing.T) {
	tree := wtest.TextTree(t, measuredRows(1000, noteHeight, nil, nil))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	for _, notches := range []float32{-0.5, -1, -0.8, -1, -0.6} {
		wtest.Wheel(tree, notches)
		wtest.RunUntilIdle(t, tree, size, func() { checkMeasuredRows(t, l, size.H, noteHeight) })
	}
	// Small steps down from the top measured every row on the way, so the
	// rows sit where their real heights put them.
	checkRows(t, l, size.H, noteHeight)
	// A long fling passes rows by without laying them out; the rows still
	// follow one another and cover the view.
	for _, notches := range []float32{-40, 12, -3} {
		wtest.Wheel(tree, notches)
		wtest.RunUntilIdle(t, tree, size, func() { checkMeasuredRows(t, l, size.H, noteHeight) })
	}
}

func TestListView_Measured_ZeroValueEstimateIsTheMean(t *testing.T) {
	w := measuredRows(1000, func(int) float32 { return 30 }, nil, nil)
	w.EstimatedItemExtent = 0
	tree := wtest.TextTree(t, w)
	testutil.RenderTree(tree, geom.Size{W: 200, H: 300})
	l := renderListOf(t, tree)
	if got := l.Measured.Estimate(); got != 30 {
		t.Errorf("estimate = %v, want the mean 30", got)
	}
	if got := l.Measured.Total(); got != 30000 {
		t.Errorf("total = %v", got)
	}
}

func TestListView_Measured_JumpToIndexMeasuresOnlyNearRows(t *testing.T) {
	var c ListController
	built := 0
	tree := wtest.TextTree(t, measuredRows(100000, noteHeight, &c, &built))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	for _, j := range []struct {
		index int
		align float32
	}{{50000, 0}, {73123, 0.5}, {10, 1}, {99990, 0}, {4, 0}} {
		built = 0
		c.JumpToIndex(j.index, j.align)
		testutil.RenderTree(tree, size)
		y, ok := screenY(l, j.index)
		if !ok {
			t.Fatalf("row %d does not exist after the jump", j.index)
		}
		// The view stays within the rows, so the first and the last rows may
		// not come to align.
		m := l.Measured
		top := m.Offset(j.index)
		want := float32(top - max(0, min(top-float64(j.align*(size.H-noteHeight(j.index))), m.Total()-float64(size.H))))
		if !nearly(y, want) {
			t.Errorf("row %d at %v after the jump, want %v", j.index, y, want)
		}
		checkMeasuredRows(t, l, size.H, noteHeight)
		if built > 60 {
			t.Errorf("the jump to %d built %d rows", j.index, built)
		}
	}
	if got := l.Measured.MeasuredCount(); got > 150 {
		t.Errorf("%d rows were measured", got)
	}
}

func TestListView_Measured_JumpToEnd(t *testing.T) {
	var c ListController
	// A jump before the list exists waits for it.
	c.JumpToEnd()
	tree := wtest.TextTree(t, measuredRows(100000, noteHeight, &c, nil))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	checkEnd := func() {
		t.Helper()
		if l.First+l.ChildCount() != 100000 {
			t.Fatalf("the rows end at %d", l.First+l.ChildCount())
		}
		last := l.ChildAt(l.ChildCount() - 1)
		if bottom := last.Offset().Y + last.Size().H; !nearly(bottom, size.H) {
			t.Errorf("the last row ends at %v, want %v", bottom, size.H)
		}
		if got, want := l.Position.Value(), l.Measured.Total()-float64(size.H); !nearly(float32(got-want), 0) {
			t.Errorf("offset %v, want the end %v", got, want)
		}
		checkMeasuredRows(t, l, size.H, noteHeight)
	}
	checkEnd()
	c.JumpToIndex(0, 0)
	testutil.RenderTree(tree, size)
	c.JumpToEnd()
	testutil.RenderTree(tree, size)
	checkEnd()
	// The scroll bar knows the end: the view cannot go past it.
	wtest.Wheel(tree, -5)
	wtest.RunUntilIdle(t, tree, size, nil)
	checkEnd()
}

// Rows above the view that take another height do not move the rows in view.
func TestListView_Measured_RowsAboveChangeWithoutMovingTheView(t *testing.T) {
	var c ListController
	var h *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: measuredRows(10000, noteHeight, &c, nil), State: &h})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	c.JumpToIndex(5000, 0)
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	// The touchpad moves the view at once.
	wtest.Wheel(tree, 0.37)
	testutil.RenderTree(tree, size)
	y0, ok := screenY(l, 5000)
	if !ok || !nearly(y0, 37) {
		t.Fatalf("row 5000 at %v, want 37 after the touchpad moved the view", y0)
	}
	// The rows built above the view grow, as an expanded row would. Row
	// 4999 shows its bottom above row 5000, and so stays where it is too.
	taller := func(i int) float32 {
		if i < 4999 {
			return 3 * noteHeight(i)
		}
		return noteHeight(i)
	}
	h.SetState(func() { h.Child = measuredRows(10000, taller, &c, nil) })
	testutil.RenderTree(tree, size)
	if y, _ := screenY(l, 5000); !nearly(y, y0) {
		t.Errorf("row 5000 moved from %v to %v when rows above it grew", y0, y)
	}
	checkMeasuredRows(t, l, size.H, taller)

	// Forgetting rows above changes their estimate, not the view.
	for i := 4900; i < 4999; i++ {
		c.Remeasure(i)
	}
	testutil.RenderTree(tree, size)
	if y, _ := screenY(l, 5000); !nearly(y, y0) {
		t.Errorf("row 5000 moved from %v to %v when rows above it were forgotten", y0, y)
	}
	if l.Measured.Measured(4900) {
		t.Error("Remeasure must forget the height of a row out of view")
	}
	checkMeasuredRows(t, l, size.H, taller)
}

// Scrolling up from a jump measures rows that were estimated, and the content
// moves with the wheel only: never back, and never by more than the motion.
func TestListView_Measured_ScrollingUpIntoEstimatedRowsDoesNotJump(t *testing.T) {
	var c ListController
	tree := wtest.TextTree(t, measuredRows(10000, noteHeight, &c, nil))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	c.JumpToIndex(5000, 0)
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	track := 5000
	y, _ := screenY(l, track)
	for range 6 {
		wtest.Wheel(tree, 1)
	}
	wtest.RunUntilIdle(t, tree, size, func() {
		ny, ok := screenY(l, track)
		if !ok {
			// Follow a row that still exists.
			track = l.First + l.ChildCount()/2
			y, _ = screenY(l, track)
			return
		}
		if ny < y-0.01 {
			t.Errorf("row %d went back from %v to %v while scrolling up", track, y, ny)
		}
		if ny-y > 6*kit.NotchDistance {
			t.Errorf("row %d jumped from %v to %v", track, y, ny)
		}
		y = ny
		checkMeasuredRows(t, l, size.H, noteHeight)
	})
	// The wheel went up by 600 pixels in all, past the rows in view, all of
	// them measured on the way.
	if l.Position.Value() < 0 {
		t.Errorf("offset %v", l.Position.Value())
	}
}

func TestListView_Measured_ItemCountChanges(t *testing.T) {
	var c ListController
	var h *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: measuredRows(1000, noteHeight, &c, nil), State: &h})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	measured := l.Measured.MeasuredCount()

	// More rows keep the heights measured so far.
	h.SetState(func() { h.Child = measuredRows(5000, noteHeight, &c, nil) })
	testutil.RenderTree(tree, size)
	if l.Measured.Len() != 5000 || l.Measured.MeasuredCount() != measured {
		t.Errorf("after growing %d rows, %d measured; want 5000, %d", l.Measured.Len(), l.Measured.MeasuredCount(), measured)
	}
	c.JumpToEnd()
	testutil.RenderTree(tree, size)
	if l.First+l.ChildCount() != 5000 {
		t.Errorf("rows end at %d, want 5000", l.First+l.ChildCount())
	}

	// Fewer rows: the view comes back to the new end.
	h.SetState(func() { h.Child = measuredRows(50, noteHeight, &c, nil) })
	testutil.RenderTree(tree, size)
	wtest.RunUntilIdle(t, tree, size, nil)
	if l.First+l.ChildCount() != 50 {
		t.Errorf("rows end at %d, want 50", l.First+l.ChildCount())
	}
	if got, want := l.Position.Value(), l.Measured.Total()-float64(size.H); got > want+0.01 {
		t.Errorf("offset %v past the end %v", got, want)
	}
	checkMeasuredRows(t, l, size.H, noteHeight)

	// None at all.
	h.SetState(func() { h.Child = measuredRows(0, noteHeight, &c, nil) })
	testutil.RenderTree(tree, size)
	c.JumpToEnd()
	testutil.RenderTree(tree, size)
}

func TestListView_Measured_NewWidthForgetsTheHeights(t *testing.T) {
	var c ListController
	tree := wtest.TextTree(t, measuredRows(1000, noteHeight, &c, nil))
	testutil.RenderTree(tree, geom.Size{W: 200, H: 300})
	wtest.Wheel(tree, -20)
	wtest.RunUntilIdle(t, tree, geom.Size{W: 200, H: 300}, nil)
	l := renderListOf(t, tree)
	if l.Measured.MeasuredCount() <= l.ChildCount() {
		t.Fatal("scrolling must have measured more rows than exist")
	}
	anchor := l.First + cachedItems + 1
	y0, _ := screenY(l, anchor)
	testutil.RenderTree(tree, geom.Size{W: 260, H: 300})
	if got := l.Measured.MeasuredCount(); got != l.ChildCount() {
		t.Errorf("%d rows measured after a new width, want the %d that exist", got, l.ChildCount())
	}
	if y, _ := screenY(l, anchor); !nearly(y, y0) {
		t.Errorf("row %d moved from %v to %v with the new width", anchor, y0, y)
	}
	c.RemeasureAll()
	testutil.RenderTree(tree, geom.Size{W: 260, H: 300})
	if got := l.Measured.MeasuredCount(); got != l.ChildCount() {
		t.Errorf("%d rows measured after RemeasureAll, want %d", got, l.ChildCount())
	}
}

// Dragging the thumb across 100,000 rows lays out only the rows it stops at.
func TestListView_Measured_ScrollBarDragMeasuresLittle(t *testing.T) {
	built := 0
	tree := wtest.TextTree(t, measuredRows(100000, noteHeight, nil, &built))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	var bar *layout.RenderScrollBar
	for _, ro := range tree.HitTest(geom.Offset{X: 198, Y: 50}).Path {
		if b, ok := ro.(*layout.RenderScrollBar); ok {
			bar = b
		}
	}
	if bar == nil {
		t.Fatal("no scroll bar")
	}
	l := renderListOf(t, tree)
	for step := 1; step <= 50; step++ {
		f := float64(step) / 50
		bar.OnDrag(f * (bar.Metrics.Content - bar.Metrics.View))
		testutil.RenderTree(tree, size)
		checkMeasuredRows(t, l, size.H, noteHeight)
	}
	if l.First+l.ChildCount() != 100000 {
		t.Errorf("the drag to the end ends at row %d", l.First+l.ChildCount())
	}
	if got := l.Measured.MeasuredCount(); got > 51*30 {
		t.Errorf("%d rows measured over 50 drags", got)
	}
	if built > 51*40 {
		t.Errorf("%d rows built over 50 drags", built)
	}
}

// A scroll bar thumb that moved because rows took their heights stays close
// to where the user left it.
func TestListView_Measured_ScrollBarSettles(t *testing.T) {
	tree := wtest.TextTree(t, measuredRows(10000, noteHeight, nil, nil))
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	var bar *layout.RenderScrollBar
	for _, ro := range tree.HitTest(geom.Offset{X: 198, Y: 50}).Path {
		if b, ok := ro.(*layout.RenderScrollBar); ok {
			bar = b
		}
	}
	bar.OnDrag(0.5 * (bar.Metrics.Content - bar.Metrics.View))
	testutil.RenderTree(tree, size)
	fraction := l.Position.Value() / (bar.Metrics.Content - bar.Metrics.View)
	if fraction < 0.49 || fraction > 0.51 {
		t.Errorf("the thumb went from 0.5 to %v as the rows took their heights", fraction)
	}
}

// measuredText is a list of rows of wrapping text, which no one measures in
// advance.
func measuredText(n int) List {
	words := strings.Fields("gowaku lays out rows of wrapping text whose heights nobody knows until they are laid out")
	return List{ItemCount: n, MeasureItems: true, ItemBuilder: func(i int) core.Widget {
		k := 3 + i*7%len(words)*3
		var b strings.Builder
		for w := range k {
			if w > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(words[(i+w)%len(words)])
		}
		return basic.Padding{Insets: geom.Symmetric(12, 6), Child: basic.Text{Content: b.String()}}
	}}
}

func TestListView_Measured_WrappingTextRows(t *testing.T) {
	tree := wtest.TextTree(t, measuredText(1000))
	size := geom.Size{W: 300, H: 400}
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	heights := map[float32]bool{}
	for k := range l.ChildCount() {
		heights[l.ChildAt(k).Size().H] = true
	}
	if len(heights) < 2 {
		t.Errorf("the rows must wrap to different heights, got %v", heights)
	}
}

// newMeasuredFrameRunner is scrollFrameRunner over 100,000 rows of wrapping
// text, settled near the middle where a small motion brings no other rows into
// range: neither edge of the view is within a few pixels of the edge of a row.
func newMeasuredFrameRunner(t testing.TB) *scrollFrameRunner {
	var c ListController
	w := measuredText(100000)
	w.Controller = &c
	tree := wtest.TextTree(t, w)
	r := &scrollFrameRunner{tree: tree, size: geom.Size{W: 400, H: 600}, now: time.Hour, dir: 1}
	testutil.RenderTree(tree, r.size)
	c.JumpToIndex(50000, 0)
	testutil.RenderTree(tree, r.size)
	r.list = renderListOf(t, tree)
	m := r.list.Measured
	clear := func(y float64) bool {
		i := m.IndexAt(y)
		return y-m.Offset(i) > 5 && m.Offset(i+1)-y > 5
	}
	top, h := m.Offset(50000), m.Extent(50000)
	for d := float32(6); d < h-6; d++ {
		if clear(top + float64(d) + float64(r.size.H)) {
			wtest.Wheel(tree, -d/kit.NotchDistance)
			break
		}
	}
	testutil.RenderTree(tree, r.size)
	if p := r.list.Position.Value(); !clear(p) || !clear(p+float64(r.size.H)) {
		t.Fatalf("no offset near row 50000 leaves both edges clear")
	}
	for range 200 {
		r.frame()
	}
	return r
}

func TestListView_Measured_ScrollingDoesNotAllocate(t *testing.T) {
	r := newMeasuredFrameRunner(t)
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

// BenchmarkListView_MeasuredScrollFrame_100k is one frame of scrolling a list
// of 100,000 rows of wrapping text whose heights the list measures, while the
// same rows stay in range.
func BenchmarkListView_MeasuredScrollFrame_100k(b *testing.B) {
	r := newMeasuredFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}

// benchJump jumps through a list of n rows of wrapping text, back and forth
// between row 0 and where jump goes, with the layouts that measure the rows
// there and a paint. Rows that come into range build their widgets, so a jump
// allocates, as it does in a fixed list; what matters is that its cost does
// not grow with n.
func benchJump(b *testing.B, n int, measure bool, jump func(c *ListController, n int)) {
	var c ListController
	w := measuredText(n)
	w.Controller = &c
	if !measure {
		w.MeasureItems, w.ItemExtent = false, 60
	}
	tree := wtest.TextTree(b, w)
	size := geom.Size{W: 400, H: 600}
	testutil.RenderTree(tree, size)
	var cv paint.Canvas
	frame := func() {
		tree.Build()
		tree.Layout(geom.Tight(size))
		for tree.NeedsBuild() {
			tree.Build()
			tree.Layout(geom.Tight(size))
		}
		cv.Reset()
		tree.Paint(&cv)
	}
	k := 0
	b.ReportAllocs()
	for b.Loop() {
		if k++; k%2 == 0 {
			c.JumpToIndex(0, 0)
		} else {
			jump(&c, n)
		}
		frame()
	}
}

func toMiddle(c *ListController, n int) { c.JumpToIndex(n/2, 0.5) }
func toEnd(c *ListController, _ int)    { c.JumpToEnd() }

func BenchmarkListView_MeasuredJumpMiddle_1k(b *testing.B)   { benchJump(b, 1000, true, toMiddle) }
func BenchmarkListView_MeasuredJumpMiddle_100k(b *testing.B) { benchJump(b, 100000, true, toMiddle) }
func BenchmarkListView_MeasuredJumpEnd_100k(b *testing.B)    { benchJump(b, 100000, true, toEnd) }

// BenchmarkListView_FixedJumpMiddle_100k is the same jump in a list of rows of
// one height, for the cost of building the rows alone.
func BenchmarkListView_FixedJumpMiddle_100k(b *testing.B) { benchJump(b, 100000, false, toMiddle) }

// The controller moves lists of known heights too, at once and exactly.
func TestListController_KnownHeights(t *testing.T) {
	size := geom.Size{W: 200, H: 100}
	var fixed ListController
	fl := rowTexts(1000)
	fl.Controller = &fixed
	tree := wtest.TextTree(t, fl)
	testutil.RenderTree(tree, size)
	fixed.JumpToIndex(500, 0)
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	if y, ok := screenY(l, 500); !ok || y != 0 {
		t.Errorf("row 500 at %v after the jump", y)
	}
	fixed.JumpToIndex(-3, 0)
	testutil.RenderTree(tree, size)
	if l.Position.Value() != 0 {
		t.Errorf("an index before the first row must go to the top, at %v", l.Position.Value())
	}

	// A jump asked before the list exists waits for the size of its view.
	var varying ListController
	varying.JumpToEnd()
	vl := varyingRows(1000, trackHeight)
	vl.Controller = &varying
	tree = wtest.TextTree(t, vl)
	testutil.RenderTree(tree, size)
	l = renderListOf(t, tree)
	if got, want := l.Position.Value(), topOf(1000, trackHeight)-float64(size.H); got != want {
		t.Errorf("the end is at %v, want %v", got, want)
	}
	varying.JumpToIndex(20, 0.5)
	testutil.RenderTree(tree, size)
	if y, _ := screenY(l, 20); y != (size.H-trackHeight(20))/2 {
		t.Errorf("row 20 at %v, want it in the middle", y)
	}
	checkRows(t, l, size.H, trackHeight)
}
