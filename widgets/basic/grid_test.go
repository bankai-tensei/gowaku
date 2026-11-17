package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
)

// tiles returns n children that fill whatever tile they get.
func tiles(n int) []core.Widget {
	out := make([]core.Widget, n)
	for i := range out {
		out[i] = Container{Color: wtest.Red}
	}
	return out
}

func TestGrid_ColumnsFor(t *testing.T) {
	cases := []struct {
		name  string
		grid  layout.RenderGrid
		width float32
		want  int
	}{
		{"ゼロ値は 1 列", layout.RenderGrid{}, 500, 1},
		{"最小幅より狭くても 1 列", layout.RenderGrid{MinColumnWidth: 100, Spacing: 10}, 50, 1},
		{"2 列にはわずかに足りない", layout.RenderGrid{MinColumnWidth: 100, Spacing: 10}, 209, 1},
		{"ちょうど 2 列", layout.RenderGrid{MinColumnWidth: 100, Spacing: 10}, 210, 2},
		{"3 列", layout.RenderGrid{MinColumnWidth: 100, Spacing: 10}, 320, 3},
		{"間隔なし", layout.RenderGrid{MinColumnWidth: 100}, 399, 3},
		{"Columns が優先", layout.RenderGrid{Columns: 4, MinColumnWidth: 100}, 120, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.grid.ColumnsFor(tc.width); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGrid_Placement(t *testing.T) {
	checkPlacement(t, []placementCase{
		{"ゼロ値は 1 列で子の高さ", Grid{Children: []core.Widget{box(10, 20, wtest.Red), box(10, 30, wtest.Green)}},
			[]geom.Rect{{W: 200, H: 20}, {Y: 20, W: 200, H: 30}}},
		{"正方形と見出しの高さ", Grid{MinColumnWidth: 90, Spacing: 10, RunSpacing: 6, AspectRatio: 1, ExtraHeight: 20, Children: tiles(3)},
			[]geom.Rect{{W: 95, H: 115}, {X: 105, W: 95, H: 115}, {Y: 121, W: 95, H: 115}}},
		{"縦横比 2 は半分の高さ", Grid{Columns: 2, AspectRatio: 2, Children: tiles(2)},
			[]geom.Rect{{W: 100, H: 50}, {X: 100, W: 100, H: 50}}},
		{"TileHeight は縦横比より優先", Grid{Columns: 4, Spacing: 4, TileHeight: 30, AspectRatio: 1, Children: tiles(5)},
			[]geom.Rect{{W: 47, H: 30}, {X: 51, W: 47, H: 30}, {X: 102, W: 47, H: 30}, {X: 153, W: 47, H: 30}, {Y: 30, W: 47, H: 30}}},
		{"行の高さは一番高い子に揃える", Grid{Columns: 2, RunSpacing: 5, Children: []core.Widget{box(10, 20, wtest.Red), box(10, 40, wtest.Green), box(10, 10, wtest.Blue)}},
			[]geom.Rect{{W: 100, H: 40}, {X: 100, W: 100, H: 40}, {Y: 45, W: 100, H: 10}}},
	})
}

func TestGrid_ColumnsFollowTheWidth(t *testing.T) {
	g := Grid{MinColumnWidth: 100, Spacing: 10, AspectRatio: 1, Children: tiles(4)}
	cases := []struct {
		width float32
		tile  float32
		cols  int
	}{
		{150, 150, 1},
		{250, 120, 2},
		{400, 126.667, 3},
		{500, 117.5, 4},
	}
	for _, tc := range cases {
		got := wtest.Rects(t, g, geom.Size{W: tc.width, H: 600})
		if len(got) != 4 {
			t.Fatalf("width %v: got %v", tc.width, got)
		}
		if d := got[0].W - tc.tile; d < -0.01 || d > 0.01 || got[0].H != got[0].W {
			t.Errorf("width %v: tile %v, want %v square", tc.width, got[0], tc.tile)
		}
		// The first tile of the second row tells the column count.
		perRow := 0
		for _, r := range got {
			if r.Y == 0 {
				perRow++
			}
		}
		if perRow != tc.cols {
			t.Errorf("width %v: %d columns, want %d", tc.width, perRow, tc.cols)
		}
	}
}

func TestGrid_InVerticalScroll(t *testing.T) {
	got := wtest.Rects(t, Scroll{Child: Grid{MinColumnWidth: 60, Spacing: 10, RunSpacing: 10, AspectRatio: 1, ExtraHeight: 10, Children: tiles(9)}}, tall)
	if len(got) < 9 {
		t.Fatalf("got %v", got)
	}
	// Three columns of 60 and rows of 70 with gaps of 10, laid out under an
	// unbounded height.
	if want := (geom.Rect{X: 140, Y: 160, W: 60, H: 70}); !wtest.NearRect(got[8], want) {
		t.Errorf("last tile: got %v, want %v", got[8], want)
	}
}

func TestGrid_UnboundedWidthPanics(t *testing.T) {
	wtest.ExpectPanic(t, "Grid has an unbounded width", func() {
		wtest.Rects(t, Row{Children: []core.Widget{Grid{Columns: 2, Children: tiles(2)}}}, wtest.Page)
	})
	wtest.ExpectPanic(t, "Grid has an unbounded width", func() {
		wtest.Rects(t, Scroll{Horizontal: true, Child: Grid{Children: tiles(2)}}, wtest.Page)
	})
}

func TestGrid_WithExpandedInRowLaysOut(t *testing.T) {
	got := wtest.Rects(t, Row{Children: []core.Widget{box(100, 10, wtest.Green), Expanded{Child: Grid{Columns: 2, TileHeight: 10, Children: tiles(2)}}}}, wtest.Page)
	checkRects(t, got, []geom.Rect{{W: 100, H: 10}, {X: 100, W: 50, H: 10}, {X: 150, W: 50, H: 10}})
}

func TestGrid_ZeroValue(t *testing.T) {
	if got := testutil.Render(Grid{}, wtest.Page); got != "" {
		t.Errorf("got %q, want nothing drawn", got)
	}
}

func TestGrid_UpdateRelayouts(t *testing.T) {
	cols := 1
	var s *wtest.SwapState
	tree := core.NewTree(wtest.Swap{Build: func() core.Widget { return Grid{Columns: cols, TileHeight: 10, Children: tiles(2)} }, State: &s})
	defer tree.Dispose()
	checkRects(t, treeRects(tree, wtest.Page), []geom.Rect{{W: 200, H: 10}, {Y: 10, W: 200, H: 10}})
	s.SetState(func() { cols = 2 })
	checkRects(t, treeRects(tree, wtest.Page), []geom.Rect{{W: 100, H: 10}, {X: 100, W: 100, H: 10}})
}

func benchmarkGrid(b *testing.B, g Grid) {
	b.Helper()
	g.Children = tiles(1000)
	tree := core.NewTree(Scroll{Child: g})
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

func BenchmarkLayout_Grid1kAspect(b *testing.B) {
	benchmarkGrid(b, Grid{MinColumnWidth: 40, Spacing: 4, RunSpacing: 4, AspectRatio: 1, ExtraHeight: 12})
}

func BenchmarkLayout_Grid1kNatural(b *testing.B) {
	benchmarkGrid(b, Grid{MinColumnWidth: 40, Spacing: 4, RunSpacing: 4})
}
