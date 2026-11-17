package editor

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

var (
	addedGround = geom.Color{G: 0.8, A: 0.25}
	addedGutter = geom.Color{G: 0.6, A: 0.5}
	addedBar    = geom.Color{G: 0.5, A: 1}
)

// diffStyle colors the lines that start with "+" as added ones.
func diffStyle(_ int, s string) LineStyle {
	if strings.HasPrefix(s, "+") || s == "" {
		return LineStyle{Background: addedGround, Gutter: addedGutter, Bar: addedBar}
	}
	return LineStyle{}
}

// paintOps lays the tree of h out and paints it into cv.
func (h *editorHarness) paintOps(cv *paint.Canvas) []paint.Op {
	h.frame()
	cv.Reset()
	h.tree.Paint(cv)
	return cv.DisplayList().Ops
}

// opsOf returns the indexes of the rectangles of ops filled with c.
func opsOf(ops []paint.Op, c geom.Color) []int {
	var at []int
	for i := range ops {
		if ops[i].Kind == paint.OpRRect && ops[i].Color == c {
			at = append(at, i)
		}
	}
	return at
}

func TestEditor_LineStyleFillsTheWholeWidth(t *testing.T) {
	var ctl Controller
	ctl.SetText("context\n+added\n\nend")
	h := newEditor(t, &ctl, Editor{LineStyle: diffStyle, LineNumbers: true})
	r := h.view()
	var cv paint.Canvas
	ops := h.paintOps(&cv)
	grounds := opsOf(ops, addedGround)
	if len(grounds) != 2 {
		t.Fatalf("%d line backgrounds, want 2 (the added line and the empty one)", len(grounds))
	}
	for k, line := range []int{1, 2} {
		got := ops[grounds[k]].RRect.Rect
		want := geom.Rect{X: r.gutter, Y: editorPadY + float32(line)*r.lineHeight, W: h.size.W - r.gutter, H: r.lineHeight}
		if got != want {
			t.Errorf("background of line %d at %v, want %v", line, got, want)
		}
	}
	gutters := opsOf(ops, addedGutter)
	if len(gutters) != 2 || ops[gutters[0]].RRect.Rect.X != 0 || ops[gutters[0]].RRect.Rect.W != r.gutter {
		t.Errorf("gutter backgrounds %v", gutters)
	}
	bars := opsOf(ops, addedBar)
	if len(bars) != 2 || ops[bars[0]].RRect.Rect.W != LineBarWidth || ops[bars[0]].RRect.Rect.X != r.gutter {
		t.Errorf("bars %v", bars)
	}
}

func TestEditor_LineStyleStaysWhenScrolledSideways(t *testing.T) {
	var ctl Controller
	ctl.SetText("+" + strings.Repeat("long line ", 60) + "\nshort")
	h := newEditor(t, &ctl, Editor{LineStyle: diffStyle})
	r := h.view()
	r.state.horz.Scroller.Jump(200)
	var cv paint.Canvas
	ops := h.paintOps(&cv)
	if r.hscroll() == 0 {
		t.Fatal("the editor did not scroll sideways")
	}
	grounds := opsOf(ops, addedGround)
	if len(grounds) != 1 {
		t.Fatalf("%d backgrounds", len(grounds))
	}
	if got := ops[grounds[0]].RRect.Rect; got.X != r.gutter || got.W != h.size.W-r.gutter {
		t.Errorf("background at %v while scrolled by %g", got, r.hscroll())
	}
	if got := ops[opsOf(ops, addedBar)[0]].RRect.Rect; got.X != r.gutter {
		t.Errorf("bar at %v while scrolled", got)
	}
}

func TestEditor_LineStyleCoversEveryRowOfAWrappedLine(t *testing.T) {
	var ctl Controller
	ctl.SetText("+" + strings.Repeat("wrapped words ", 40) + "\nshort")
	h := newEditor(t, &ctl, Editor{LineStyle: diffStyle, Wrap: true})
	r := h.view()
	var cv paint.Canvas
	ops := h.paintOps(&cv)
	rows := r.cache.get(0).layout.LineCount()
	if rows < 2 {
		t.Fatalf("the line did not wrap: %d rows", rows)
	}
	grounds := opsOf(ops, addedGround)
	if len(grounds) != 1 {
		t.Fatalf("%d backgrounds", len(grounds))
	}
	if got, want := ops[grounds[0]].RRect.Rect.H, float32(rows)*r.lineHeight; got != want {
		t.Errorf("background %g high, want %g for %d rows", got, want, rows)
	}
	if got := ops[opsOf(ops, addedBar)[0]].RRect.Rect.H; got != float32(rows)*r.lineHeight {
		t.Errorf("bar %g high", got)
	}
}

func TestEditor_LineStyleIsUnderTheSelectionAndTheMatches(t *testing.T) {
	var ctl Controller
	ctl.SetText("+added text\nother")
	h := newEditor(t, &ctl, Editor{LineStyle: diffStyle, HighlightCurrentLine: true,
		Decorations: func(i int, _ string, dst []Decoration) []Decoration {
			if i == 0 {
				dst = append(dst, Decoration{Start: 1, End: 3, Background: geom.Color{R: 1, A: 0.3}})
			}
			return dst
		},
	})
	if _, err := ctl.Find("text", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	ctl.SetSelection(TextRange{Start: pos(0, 0), End: pos(0, 4)})
	r := h.view()
	var cv paint.Canvas
	ops := h.paintOps(&cv)
	ground := opsOf(ops, addedGround)
	if len(ground) != 1 {
		t.Fatalf("%d backgrounds", len(ground))
	}
	for name, c := range map[string]geom.Color{
		"current line": r.look.currentLine, "match": r.look.match,
		"decoration": {R: 1, A: 0.3}, "selection": r.look.selection,
	} {
		at := opsOf(ops, c)
		if len(at) == 0 {
			t.Errorf("no %s drawn", name)
			continue
		}
		if at[len(at)-1] < ground[0] {
			t.Errorf("the %s is under the line background", name)
		}
	}
}

func TestEditor_RedecorateLinesAsksForTheLineStyleAgainInEveryView(t *testing.T) {
	var ctl Controller
	ctl.SetText("one\ntwo\nthree")
	var left, right View
	marked := -1
	calls := 0
	style := func(i int, _ string) LineStyle {
		calls++
		if i == marked {
			return LineStyle{Background: addedGround}
		}
		return LineStyle{}
	}
	h := newSplit(t, &ctl, &left, &right, Editor{LineStyle: style})
	var cv paint.Canvas
	count := func() int {
		h.frame()
		cv.Reset()
		h.tree.Paint(&cv)
		return len(opsOf(cv.DisplayList().Ops, addedGround))
	}
	if n := count(); n != 0 {
		t.Fatalf("%d backgrounds before any line is marked", n)
	}
	calls = 0
	if count(); calls != 0 {
		t.Errorf("a frame without a change asked %d times", calls)
	}
	marked = 1
	ctl.RedecorateLines(1, 1)
	if n := count(); n != 2 {
		t.Errorf("%d backgrounds after RedecorateLines, want one in each view", n)
	}
	if calls != 2 {
		t.Errorf("RedecorateLines of one line asked %d times, want once for each view", calls)
	}
	marked = -1
	ctl.Redecorate()
	if n := count(); n != 0 {
		t.Errorf("%d backgrounds after Redecorate", n)
	}
}

func TestEditor_LineStylePaintAllocatesNothing(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		var ctl Controller
		var sb strings.Builder
		for i := range 2000 {
			if i%3 == 0 {
				sb.WriteString("+")
			}
			sb.WriteString("line of the diff\n")
		}
		ctl.SetText(sb.String())
		h := newEditor(t, &ctl, Editor{LineStyle: diffStyle, LineNumbers: true, Wrap: wrap, HighlightCurrentLine: true})
		r := h.view()
		c := geom.Tight(h.size)
		var cv paint.Canvas
		step := 0
		run := func() {
			step++
			r.state.Jump(float64(step%40) * 7)
			h.tree.Layout(c)
			cv.Reset()
			h.tree.Paint(&cv)
		}
		for range 120 {
			run()
		}
		if allocs := testing.AllocsPerRun(20, run); allocs != 0 {
			t.Errorf("wrap %v: %v allocations per frame", wrap, allocs)
		}
	}
}
