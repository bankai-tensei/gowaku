package collection

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func rowTexts(n int) List {
	return List{ItemCount: n, ItemExtent: 20, ItemBuilder: func(i int) core.Widget {
		return basic.Text{Content: fmt.Sprintf("row %d", i)}
	}}
}

func TestListView_BuildsOnlyWhatIsNear(t *testing.T) {
	built := map[int]bool{}
	list := List{ItemCount: 10000, ItemExtent: 20, ItemBuilder: func(i int) core.Widget {
		built[i] = true
		return basic.SizedBox{}
	}}
	tree := wtest.TextTree(t, list)
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	if n := len(built); n > 5+2*cachedItems+2 {
		t.Errorf("built %d rows", n)
	}
	if !built[0] || !built[4] {
		t.Errorf("the rows in view must exist: %v", built)
	}
}

func TestListView_FirstFrameIsComplete(t *testing.T) {
	tree := wtest.TextTree(t, rowTexts(100))
	got := testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	if n := strings.Count(got, "\n"); n < 5 {
		t.Errorf("a frame must show the rows at once, got %d ops", n)
	}
}

func TestListView_ScrollKeepsRowsAndReachesTheEnd(t *testing.T) {
	tree := wtest.TextTree(t, rowTexts(1000))
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	for range 300 {
		wtest.Wheel(tree, -1)
	}
	wtest.RunUntilIdle(t, tree, size, nil)
	got := testutil.RenderTree(tree, size)
	if strings.Count(got, "\n") < 5 {
		t.Errorf("the end must show rows, got %q", got)
	}
	y := func() float32 {
		for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 95}).Path {
			if p, ok := ro.(*basic.RenderParagraph); ok {
				return core.GlobalOffset(p).Y
			}
		}
		return -1
	}
	if got := y(); got != 80 {
		t.Errorf("the last row is at %v, want 80", got)
	}
}

func TestListView_EmptyAndShort(t *testing.T) {
	for _, n := range []int{0, 2} {
		tree := wtest.TextTree(t, rowTexts(n))
		size := geom.Size{W: 200, H: 100}
		testutil.RenderTree(tree, size)
		wtest.Wheel(tree, -3)
		wtest.RunUntilIdle(t, tree, size, nil)
	}
}

func TestListView_RepaintWhileScrollingDoesNotAllocate(t *testing.T) {
	tree := wtest.TextTree(t, rowTexts(10000))
	size := geom.Size{W: 400, H: 600}
	testutil.RenderTree(tree, size)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	allocs := testing.AllocsPerRun(10, func() {
		cv.Reset()
		tree.Paint(&cv)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

// BenchmarkListView_ScrollFrame is one frame of a scroll through 10,000 rows:
// tick, build of the rows that came into range, layout and paint. Shaping new
// rows allocates, which is allowed on a cache miss (CODING_STYLE §7).
func BenchmarkListView_ScrollFrame(b *testing.B) {
	tree := wtest.TextTree(b, rowTexts(10000))
	size := geom.Size{W: 400, H: 800}
	testutil.RenderTree(tree, size)
	var cv paint.Canvas
	now := time.Second
	b.ReportAllocs()
	for b.Loop() {
		if !tree.Ticking() {
			wtest.Wheel(tree, -10)
		}
		now += 8 * time.Millisecond
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		tree.Build()
		tree.Layout(geom.Tight(size))
		cv.Reset()
		tree.Paint(&cv)
	}
}

func TestListView_InColumnPanics(t *testing.T) {
	list := List{ItemCount: 3, ItemBuilder: func(int) core.Widget { return basic.Container{Width: 10, Height: 10, Color: wtest.Red} }}
	wtest.ExpectPanic(t, "collection.List has an unbounded height", func() {
		wtest.Rects(t, basic.Column{Children: []core.Widget{list}}, wtest.Page)
	})
}
