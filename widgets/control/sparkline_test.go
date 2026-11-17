package control

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// sparkOps renders a sparkline in a cell of size and returns its commands.
func sparkOps(t *testing.T, s Sparkline, size geom.Size) []paint.Op {
	t.Helper()
	tree := core.NewTree(basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: size.W, Height: size.H, Child: s}})
	t.Cleanup(tree.Dispose)
	return wtest.Ops(tree, geom.Size{W: 300, H: 200})
}

func sparkKinds(ops []paint.Op) map[paint.OpKind]int {
	out := map[paint.OpKind]int{}
	for i := range ops {
		out[ops[i].Kind]++
	}
	return out
}

func nan() float64 { return math.NaN() }

func TestSparkline_LineData(t *testing.T) {
	cell := geom.Size{W: 100, H: 20}
	cases := []struct {
		name     string
		s        Sparkline
		paths    int
		rrects   int
		contours int
	}{
		{"ゼロ値", Sparkline{}, 0, 0, 0},
		{"空", Sparkline{Data: []float64{}}, 0, 0, 0},
		{"1 点", Sparkline{Data: []float64{3}}, 1, 0, 1},
		{"1 点と印", Sparkline{Data: []float64{3}, MarkLast: true}, 1, 1, 1},
		{"一定", Sparkline{Data: []float64{2, 2, 2, 2}}, 1, 0, 1},
		{"負の値", Sparkline{Data: []float64{-3, -1, -4, -1}}, 1, 0, 1},
		{"NaN で切れる", Sparkline{Data: []float64{1, 2, nan(), 3, 4}}, 1, 0, 2},
		{"全部 NaN", Sparkline{Data: []float64{nan(), math.Inf(1)}}, 0, 0, 0},
		{"面", Sparkline{Data: []float64{1, 3, 2}, Fill: true}, 2, 0, 1},
		{"面と NaN", Sparkline{Data: []float64{1, 3, nan(), 2, 5, nan(), 7}, Fill: true, MarkLast: true}, 2, 1, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := sparkOps(t, tc.s, cell)
			k := sparkKinds(ops)
			if k[paint.OpPath] != tc.paths || k[paint.OpRRect] != tc.rrects {
				t.Fatalf("got %v", k)
			}
			for i := range ops {
				op := &ops[i]
				if op.Kind != paint.OpPath || op.Stroke.Width == 0 {
					continue
				}
				_, starts := op.Path.Polygon()
				if len(starts) != tc.contours {
					t.Errorf("got %d contours, want %d", len(starts), tc.contours)
				}
				b := op.Path.Bounds().Translate(op.Origin)
				if b.X < 0 || b.Y < 0 || b.X+b.W > cell.W+1e-3 || b.Y+b.H > cell.H+1e-3 {
					t.Errorf("the line leaves its cell: %+v", b)
				}
			}
		})
	}
}

func TestSparkline_ConstantSitsInTheMiddle(t *testing.T) {
	ops := sparkOps(t, Sparkline{Data: []float64{5, 5, 5}}, geom.Size{W: 60, H: 20})
	b := ops[0].Path.Bounds()
	if b.H != 0 || b.Y != 10 {
		t.Errorf("a constant line runs through the middle, got %+v", b)
	}
}

func TestSparkline_MinMaxClamps(t *testing.T) {
	ops := sparkOps(t, Sparkline{Data: []float64{-100, 0, 100}, Min: 0, Max: 10}, geom.Size{W: 60, H: 20})
	b := ops[0].Path.Bounds()
	half := float32(sparklineThickness / 2)
	if b.Y != half || b.Y+b.H != 20-half {
		t.Errorf("values outside Min and Max sit on the edges, got %+v", b)
	}
}

func TestSparkline_Bars(t *testing.T) {
	cell := geom.Size{W: 100, H: 40}
	t.Run("正の値は下端から", func(t *testing.T) {
		ops := sparkOps(t, Sparkline{Kind: SparklineBars, Data: []float64{1, 2, 4, 0}}, cell)
		if len(ops) != 3 {
			t.Fatalf("a zero bar is not drawn, got %d commands", len(ops))
		}
		for i := range ops {
			r := ops[i].RRect.Rect
			if r.Y+r.H != 40 {
				t.Errorf("bar %d does not stand on the floor: %+v", i, r)
			}
		}
		if ops[2].RRect.Rect.H != 40 {
			t.Errorf("the largest bar fills the height, got %+v", ops[2].RRect.Rect)
		}
	})
	t.Run("負の値はゼロから下へ", func(t *testing.T) {
		ops := sparkOps(t, Sparkline{Kind: SparklineBars, Data: []float64{3, -1, nan(), 1}, Color: wtest.Green, Negative: wtest.Red}, cell)
		// The baseline, then three bars: NaN leaves a gap.
		if len(ops) != 4 {
			t.Fatalf("got %d commands", len(ops))
		}
		zero := float32(30)
		if base := ops[0].RRect.Rect; geom.Abs32(base.Y+base.H/2-zero) > 1e-3 || base.W != 100 {
			t.Errorf("the baseline is at zero across the cell, got %+v", base)
		}
		up, down := ops[1], ops[2]
		if up.Color != wtest.Green || up.RRect.Rect.Y+up.RRect.Rect.H != zero {
			t.Errorf("a positive bar rises from zero, got %+v", up.RRect.Rect)
		}
		if down.Color != wtest.Red || down.RRect.Rect.Y != zero || down.RRect.Rect.H != 10 {
			t.Errorf("a negative bar hangs from zero, got %+v %v", down.RRect.Rect, down.Color)
		}
		if ops[3].RRect.Rect.X <= ops[2].RRect.Rect.X+25 {
			t.Errorf("the NaN slot stays empty, got %+v", ops[3].RRect.Rect)
		}
	})
	t.Run("全部ゼロ", func(t *testing.T) {
		if ops := sparkOps(t, Sparkline{Kind: SparklineBars, Data: []float64{0, 0}}, cell); len(ops) != 0 {
			t.Errorf("got %d commands", len(ops))
		}
	})
	t.Run("細いセルでも隙間", func(t *testing.T) {
		data := make([]float64, 50)
		for i := range data {
			data[i] = float64(i + 1)
		}
		ops := sparkOps(t, Sparkline{Kind: SparklineBars, Data: data}, geom.Size{W: 100, H: 20})
		if len(ops) != 50 {
			t.Fatalf("got %d bars", len(ops))
		}
		if gap := ops[1].RRect.Rect.X - (ops[0].RRect.Rect.X + ops[0].RRect.Rect.W); gap < 0.99 {
			t.Errorf("bars keep a pixel apart, got %g", gap)
		}
	})
}

func TestSparkline_ZeroSizeCell(t *testing.T) {
	for _, size := range []geom.Size{{}, {W: 0, H: 20}, {W: 80, H: 0}} {
		for _, k := range []SparklineKind{SparklineLine, SparklineBars} {
			tree := core.NewTree(Sparkline{Kind: k, Data: []float64{1, 2, 3}, Fill: true, MarkLast: true})
			if ops := wtest.Ops(tree, size); len(ops) != 0 {
				t.Errorf("%v %v: got %d commands", size, k, len(ops))
			}
			tree.Dispose()
		}
	}
}

func TestSparkline_SizeWithoutBounds(t *testing.T) {
	tree := core.NewTree(basic.Row{Size: layout.MainMin, Children: []core.Widget{Sparkline{Data: []float64{1, 2}}}})
	defer tree.Dispose()
	testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
	n := wtest.FindRole(tree.Semantics(), semantics.RoleImage)
	if n == nil || n.Rect.W != sparklineWidth {
		t.Fatalf("got %+v", n)
	}
	tree2 := core.NewTree(basic.Align{Child: Sparkline{Data: []float64{1}, Width: 30, Height: 10}})
	defer tree2.Dispose()
	testutil.RenderTree(tree2, geom.Size{W: 300, H: 100})
	if n := wtest.FindRole(tree2.Semantics(), semantics.RoleImage); n == nil || n.Rect.W != 30 || n.Rect.H != 10 {
		t.Errorf("got %+v", n)
	}
}

func TestSparkline_Semantics(t *testing.T) {
	cases := []struct {
		name string
		tag  language.Tag
		s    Sparkline
		want string
	}{
		{"ラベル", language.Japanese, Sparkline{Data: []float64{1}, Label: "売上は上昇"}, "売上は上昇"},
		{"英語の要約", language.English, Sparkline{Data: []float64{3, 12.345, nan(), 8}}, "Minimum 3, Maximum 12.35, Latest 8"},
		{"日本語の要約", language.Japanese, Sparkline{Data: []float64{-2, 5}}, "最小 -2, 最大 5, 最新 5"},
		{"空", language.English, Sparkline{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := core.NewTree(basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: tc.tag}, Child: basic.Localizations{Child: tc.s}})
			defer tree.Dispose()
			testutil.RenderTree(tree, geom.Size{W: 100, H: 20})
			n := wtest.FindRole(tree.Semantics(), semantics.RoleImage)
			if n == nil || n.Label != tc.want {
				t.Fatalf("got %+v, want %q", n, tc.want)
			}
		})
	}
}

func TestSparkline_DataChangedInPlaceShowsAfterARebuild(t *testing.T) {
	data := []float64{1, 2, 3}
	size := geom.Size{W: 60, H: 20}
	tree, swap := wtest.SwapTree(t, Sparkline{Data: data})
	points := func() []geom.Offset {
		ops := wtest.Ops(tree, size)
		if len(ops) != 1 {
			t.Fatalf("got %d commands", len(ops))
		}
		pts, _ := ops[0].Path.Polygon()
		return append([]geom.Offset(nil), pts...)
	}
	first := points()
	again := points()
	if first[2] != again[2] {
		t.Fatal("an unchanged sparkline paints the same")
	}
	data[2] = -3
	swap(Sparkline{Data: data})
	second := points()
	if second[2] == first[2] {
		t.Errorf("the changed value must move, got %v and %v", first, second)
	}
}

// sparkRows is a page of n rows, each a sparkline in a table cell: lines
// with fill and a dot, and bars, by turns.
func sparkRows(n int, series [][]float64) core.Widget {
	rows := make([]core.Widget, n)
	for i := range rows {
		s := Sparkline{Data: series[i%len(series)], Fill: true, MarkLast: true}
		if i%2 == 1 {
			s.Kind = SparklineBars
		}
		rows[i] = basic.SizedBox{Width: 120, Height: 24, Child: s}
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Children: rows}
}

func sparkSeries() [][]float64 {
	out := make([][]float64, 7)
	for k := range out {
		d := make([]float64, 24+k)
		for i := range d {
			d[i] = math.Sin(float64(i+k)/3) * float64(10+k)
			if (i+k)%11 == 0 {
				d[i] = math.NaN()
			}
		}
		out[k] = d
	}
	return out
}

// collectSparks appends every sparkline render object under ro.
func collectSparks(ro core.RenderObject, out []*renderSparkline) []*renderSparkline {
	if s, ok := ro.(*renderSparkline); ok {
		out = append(out, s)
	}
	for i := range ro.ChildCount() {
		out = collectSparks(ro.ChildAt(i), out)
	}
	return out
}

func TestSparkline_PaintDoesNotAllocate(t *testing.T) {
	series := sparkSeries()
	tree := core.NewTree(sparkRows(100, series))
	defer tree.Dispose()
	testutil.RenderTree(tree, geom.Size{W: 200, H: 3000})
	sparks := collectSparks(tree.RootRenderObject(), nil)
	var cv paint.Canvas
	frame := func(k int) {
		for i, s := range sparks {
			s.Data, s.built = series[(i+k)%len(series)], false
		}
		cv.Reset()
		cv.Translate(0, -float32(k%50))
		tree.Paint(&cv)
	}
	for k := range 20 {
		frame(k)
	}
	k := 0
	if got := testing.AllocsPerRun(50, func() { k++; frame(k) }); got != 0 {
		t.Errorf("a frame of recycled sparklines allocates %v times", got)
	}
}

// BenchmarkSparkline_Paint_1kRows paints a thousand table rows of sparklines
// while they scroll by: the canvas moves each frame, and the paths are reused.
func BenchmarkSparkline_Paint_1kRows(b *testing.B) {
	tree := core.NewTree(sparkRows(1000, sparkSeries()))
	defer tree.Dispose()
	testutil.RenderTree(tree, geom.Size{W: 200, H: 24000})
	var cv paint.Canvas
	for k := range 10 {
		cv.Reset()
		cv.Translate(0, -float32(k))
		tree.Paint(&cv)
	}
	b.ReportAllocs()
	k := 0
	for b.Loop() {
		k++
		cv.Reset()
		cv.Translate(0, -float32(k%240))
		tree.Paint(&cv)
	}
}

// BenchmarkSparkline_Recycle_1kRows is a scroll through a virtualized table,
// where every row shows another series each frame, so each line is built
// again from its data before it is painted.
func BenchmarkSparkline_Recycle_1kRows(b *testing.B) {
	series := sparkSeries()
	tree := core.NewTree(sparkRows(1000, series))
	defer tree.Dispose()
	testutil.RenderTree(tree, geom.Size{W: 200, H: 24000})
	sparks := collectSparks(tree.RootRenderObject(), nil)
	var cv paint.Canvas
	frame := func(k int) {
		for i, s := range sparks {
			s.Data, s.built = series[(i+k)%len(series)], false
		}
		cv.Reset()
		cv.Translate(0, -float32(k%240))
		tree.Paint(&cv)
	}
	for k := range 10 {
		frame(k)
	}
	b.ReportAllocs()
	k := 0
	for b.Loop() {
		k++
		frame(k)
	}
}
