package collection

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// The views below are scrolled hundreds of millions of pixels down, where a
// float32 offset would move only in steps of 32 or more (ADR 0150). A move of
// a fraction of a notch, as from a touchpad, must move them by that much.

// nudge is the distance of a tenth of a notch, as a touchpad sends it.
const nudge = float64(float32(0.1) * kit.NotchDistance)

func TestList_ScrollsByAPixelFarDown(t *testing.T) {
	const n, extent = 20_000_000, 32
	var ctl ListController
	list := List{ItemCount: n, ItemExtent: extent, Controller: &ctl, ItemBuilder: func(int) core.Widget {
		return basic.SizedBox{}
	}}
	size := geom.Size{W: 200, H: 100}
	tree := wtest.TextTree(t, list)
	testutil.RenderTree(tree, size)
	ctl.JumpToIndex(n-10, 0)
	testutil.RenderTree(tree, size)
	l := renderListOf(t, tree)
	off := l.Position.Value()
	if want := float64(n-10) * extent; off != want {
		t.Fatalf("the jump lands at %v, want %v", off, want)
	}
	check := func(off float64) {
		t.Helper()
		for k := range l.ChildCount() {
			want := float32(float64(l.First+k)*extent - off)
			if got := l.ChildAt(k).Offset().Y; got != want {
				t.Fatalf("row %d is at %v, want %v", l.First+k, got, want)
			}
		}
	}
	check(off)
	wtest.Wheel(tree, -0.1)
	testutil.RenderTree(tree, size)
	moved := l.Position.Value() - off
	if moved < nudge-1e-3 || moved > nudge+1e-3 {
		t.Fatalf("a tenth of a notch moves the list by %v, want %v", moved, nudge)
	}
	check(l.Position.Value())
}

func TestTable_ScrollsByAPixelFarDown(t *testing.T) {
	const n = 2_000_000
	tb := millionRows()
	rows := make([]int, n)
	for i := range rows {
		rows[i] = i
	}
	tb.Rows, tb.RowHeight = rows, 200
	tree := tableTree(t, tb)
	r := tableOf(t, tree)
	c := r.c
	c.vert.Jump(float64(n-10) * 200)
	wtest.Ops(tree, tableScreen)
	off := c.vert.Offset.Value()
	if off < 300_000_000 {
		t.Fatalf("the jump lands at %v", off)
	}
	first, _ := c.visibleRows()
	y := c.rowY(first)
	for _, d := range []float64{1, 0.5, 3} {
		c.vert.Jump(off + d)
		wtest.Ops(tree, tableScreen)
		if got := c.vert.Offset.Value() - off; got != d {
			t.Fatalf("a jump of %v moves the table by %v", d, got)
		}
		if got := y - c.rowY(first); got != float32(d) {
			t.Fatalf("a jump of %v moves row %d by %v", d, first, got)
		}
	}
	// The wheel works the same far down.
	c.vert.Jump(off)
	wtest.Ops(tree, tableScreen)
	wtest.Wheel(tree, -0.1)
	wtest.Ops(tree, tableScreen)
	moved := c.vert.Offset.Value() - off
	if moved < nudge-1e-3 || moved > nudge+1e-3 {
		t.Fatalf("a tenth of a notch moves the table by %v, want %v", moved, nudge)
	}
}

func TestGridView_ScrollsByAPixelFarDown(t *testing.T) {
	const n = 10_000_000
	g := GridView{
		ItemCount: n, Columns: 2, TileHeight: 100,
		ItemBuilder: func(int) core.Widget { return basic.SizedBox{} },
	}
	size := geom.Size{W: 200, H: 300}
	tree := wtest.TextTree(t, g)
	testutil.RenderTree(tree, size)
	var s *gridState
	for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 10}).Path {
		if b, ok := ro.(*renderGridBody); ok {
			s = b.host
			break
		}
	}
	if s == nil {
		t.Fatal("no grid")
	}
	s.Jump(s.Metrics.Content - s.Metrics.View - 1000)
	testutil.RenderTree(tree, size)
	off := s.Offset.Value()
	if off < 100_000_000 {
		t.Fatalf("the jump lands at %v", off)
	}
	s.Jump(off + 1)
	testutil.RenderTree(tree, size)
	if got := s.Offset.Value() - off; got != 1 {
		t.Fatalf("a jump of 1 moves the grid by %v", got)
	}
}
