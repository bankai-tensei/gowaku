package collection

import (
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// millionRows is a table of a million rows whose cells repeat a few hundred
// texts, so that a scroll draws texts already shaped, as a table of real data
// does after its first screens; shaping a new text allocates, which a cache
// miss may (CODING_STYLE §7).
func millionRows() Table[int] {
	names := make([]string, 512)
	for i := range names {
		names[i] = "Item " + strconv.Itoa(i)
	}
	rows := make([]int, 1_000_000)
	for i := range rows {
		rows[i] = i
	}
	text := func(k int) func(int) string {
		return func(i int) string { return names[(i*k)%len(names)] }
	}
	return Table[int]{Rows: rows, Frozen: 1, Columns: []TableColumn[int]{
		{Title: "Name", Width: 160, Text: text(1), Sortable: true},
		{Title: "Kind", Width: 120, Text: text(3)},
		{Title: "Size", Width: 90, Text: text(5), Trailing: true},
		{Title: "Owner", Width: 140, Text: text(7)},
		{Title: "Modified", Width: 160, Text: text(11)},
	}}
}

// tableOf returns the render object of the table in tree.
func tableOf(t testing.TB, tree *core.Tree) *renderTable {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 100}).Path {
		if r, ok := ro.(*renderTable); ok {
			return r
		}
	}
	t.Fatal("no table")
	return nil
}

// tableFrame is one frame of a table: tick, build, layout and paint.
func tableFrame(tree *core.Tree, cv *paint.Canvas, now time.Duration) {
	tree.Tick(now)
	tree.Build()
	tree.Layout(geom.Tight(tableScreen))
	cv.Reset()
	tree.Paint(cv)
}

func TestDataTable_ScrollingAMillionRowsDoesNotAllocate(t *testing.T) {
	tree := tableTree(t, millionRows())
	r := tableOf(t, tree)
	var cv paint.Canvas
	now := time.Second
	step := func() {
		// Down a third of a row at a time, through the middle of the rows, and
		// back up to where it started.
		at := r.c.vert.Offset.Value() + 11
		if at > 16_000_000+64*tableRowHeight {
			at = 16_000_000
		}
		r.c.vert.Jump(at)
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
	r.c.vert.Jump(16_000_000)
	for range 400 {
		step()
	}
	if allocs := testing.AllocsPerRun(100, step); allocs != 0 {
		t.Errorf("a scroll step allocates %v times", allocs)
	}
	relayout := func() {
		r.MarkNeedsLayout()
		tree.Layout(geom.Tight(tableScreen))
		cv.Reset()
		tree.Paint(&cv)
	}
	if allocs := testing.AllocsPerRun(100, relayout); allocs != 0 {
		t.Errorf("a layout and paint allocate %v times", allocs)
	}
}

// BenchmarkDataTable_LayoutPaint_1M lays out and paints a table of a million
// rows, scrolled to the middle.
func BenchmarkDataTable_LayoutPaint_1M(b *testing.B) {
	tree := tableTree(b, millionRows())
	r := tableOf(b, tree)
	r.c.vert.Jump(16_000_000)
	var cv paint.Canvas
	tableFrame(tree, &cv, time.Second)
	b.ReportAllocs()
	for b.Loop() {
		r.MarkNeedsLayout()
		tree.Layout(geom.Tight(tableScreen))
		cv.Reset()
		tree.Paint(&cv)
	}
}

// BenchmarkDataTable_ScrollStep_1M is one frame of a scroll through a table
// of a million rows: the offset moves, and the frame ticks, builds (nothing),
// lays out and paints.
func BenchmarkDataTable_ScrollStep_1M(b *testing.B) {
	tree := tableTree(b, millionRows())
	r := tableOf(b, tree)
	var cv paint.Canvas
	now := time.Second
	base := float64(16_000_000)
	r.c.vert.Jump(base)
	for i := range 200 {
		r.c.vert.Jump(base + float64(i*11%(64*tableRowHeight)))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		i++
		r.c.vert.Jump(base + float64(i*11%(64*tableRowHeight)))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
}
