package editor

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
)

// benchDoc is 100,000 lines of prose-like code.
func benchDoc() string { return numbered(100_000) }

func BenchmarkTextEditor_Load100k(b *testing.B) {
	doc := benchDoc()
	shaper := testutil.NewShaper(b)
	b.ReportAllocs()
	for b.Loop() {
		var ctl Controller
		ctl.SetText(doc)
		tree := core.NewTree(testutil.WithShaper(shaper, Editor{Controller: &ctl, LineNumbers: true}))
		testutil.RenderTree(tree, geom.Size{W: 800, H: 600})
		tree.Dispose()
	}
}

// BenchmarkTextEditor_ScrollPaint100k scrolls a few lines back and forth in a
// document of 100,000 lines and lays out and paints each step. The lines stay
// in the cache, so it allocates nothing.
func BenchmarkTextEditor_ScrollPaint100k(b *testing.B) {
	for _, wrap := range []bool{false, true} {
		name := "nowrap"
		if wrap {
			name = "wrap"
		}
		b.Run(name, func(b *testing.B) {
			var ctl Controller
			ctl.SetText(benchDoc())
			h := newEditor(b, &ctl, Editor{LineNumbers: true, Wrap: wrap, HighlightCurrentLine: true})
			r := h.view()
			r.state.Jump(50_000 * float64(r.lineHeight))
			h.frame()
			base := r.top()
			c := geom.Tight(h.size)
			var cv paint.Canvas
			step := 0
			for range 4 {
				r.state.Jump(base + float64(step%8)*7)
				step++
				h.tree.Layout(c)
				cv.Reset()
				h.tree.Paint(&cv)
			}
			b.ReportAllocs()
			for b.Loop() {
				r.state.Jump(base + float64(step%8)*7)
				step++
				h.tree.Layout(c)
				cv.Reset()
				h.tree.Paint(&cv)
			}
		})
	}
}

// BenchmarkTextEditor_TypeMiddle100k types a character in the middle of a
// document of 100,000 lines and takes it back, with a layout and a paint each.
// The edit shapes the one line again, so it allocates that line's layout.
func BenchmarkTextEditor_TypeMiddle100k(b *testing.B) {
	var ctl Controller
	ctl.SetText(benchDoc())
	h := newEditor(b, &ctl, Editor{LineNumbers: true})
	ctl.SetSelection(TextRange{Start: TextPosition{50_000, 4}, End: TextPosition{50_000, 4}})
	h.frame()
	c := geom.Tight(h.size)
	var cv paint.Canvas
	r := h.view()
	b.ReportAllocs()
	for b.Loop() {
		r.HandleText("x")
		h.tree.Layout(c)
		cv.Reset()
		h.tree.Paint(&cv)
		ctl.Replace(TextRange{Start: TextPosition{50_000, 4}, End: TextPosition{50_000, 5}}, "")
		h.tree.Layout(c)
		cv.Reset()
		h.tree.Paint(&cv)
	}
}

// BenchmarkTextEditor_ScrollPaintTwoViews100k scrolls one of two views of a
// document of 100,000 lines, scrolled sideways, while the other follows it
// through OnScroll, and lays out and paints both each step.
func BenchmarkTextEditor_ScrollPaintTwoViews100k(b *testing.B) {
	var ctl Controller
	var left, right View
	ctl.SetText(benchDoc())
	left.OnScroll = func(p ScrollPosition) { right.SetScroll(p) }
	h := newSplit(b, &ctl, &left, &right, Editor{LineNumbers: true})
	r := left.r
	r.state.Jump(50_000 * float64(r.lineHeight))
	r.state.horz.Jump(8)
	h.frame()
	base := r.top()
	c := geom.Tight(h.size)
	var cv paint.Canvas
	step := 0
	b.ReportAllocs()
	for b.Loop() {
		r.state.Jump(base + float64(step%8)*7)
		step++
		h.tree.Layout(c)
		cv.Reset()
		h.tree.Paint(&cv)
	}
}
