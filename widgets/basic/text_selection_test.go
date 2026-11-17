package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

var selectionColor = geom.Color{R: 0, G: 0.4, B: 1, A: 0.25}

// selectionOps renders tree and returns the commands in selectionColor.
func selectionOps(tree *core.Tree) []geom.Rect {
	var out []geom.Rect
	for _, op := range wtest.Rrects(tree, geom.Size{W: 400, H: 200}) {
		if op.Color == selectionColor {
			out = append(out, op.RRect.Rect)
		}
	}
	return out
}

func TestParagraphSelection_PaintsUnderTheGlyphs(t *testing.T) {
	tree := wtest.TextTree(t, Align{Alignment: layout.AlignTopLeft, Child: Text{Content: "select me", Style: text.Style{Size: 16}}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 200})
	p := paragraphAtOrigin(t, tree)
	if got := selectionOps(tree); len(got) != 0 {
		t.Fatalf("a text without a selection paints %v", got)
	}
	p.SetSelection(7, 2, selectionColor)
	if a, b := p.Selection(); a != 2 || b != 7 {
		t.Errorf("Selection = %d-%d, want the range in order", a, b)
	}
	got := selectionOps(tree)
	want := p.TextLayout().SelectionRects(2, 7, nil)
	if len(got) != 1 || !wtest.NearRect(got[0], want[0]) {
		t.Fatalf("painted %v, want %v", got, want)
	}
	ops := wtest.Ops(tree, geom.Size{W: 400, H: 200})
	sel, glyphs := -1, -1
	for i := range ops {
		switch {
		case ops[i].Kind == paint.OpRRect && ops[i].Color == selectionColor:
			sel = i
		case ops[i].Kind == paint.OpGlyphs && glyphs < 0:
			glyphs = i
		}
	}
	if sel < 0 || glyphs < sel {
		t.Errorf("the highlight (%d) must come before the glyphs (%d)", sel, glyphs)
	}
	p.ClearSelection()
	if a, b := p.Selection(); a != 0 || b != 0 || len(selectionOps(tree)) != 0 {
		t.Error("ClearSelection must remove the highlight")
	}
}

func TestParagraphSelection_PastTheEndStopsThere(t *testing.T) {
	tree := wtest.TextTree(t, Align{Alignment: layout.AlignTopLeft, Child: Text{Content: "short"}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 200})
	p := paragraphAtOrigin(t, tree)
	p.SetSelection(1, 99, selectionColor)
	got := selectionOps(tree)
	want := p.TextLayout().SelectionRects(1, 5, nil)
	if len(got) != 1 || !wtest.NearRect(got[0], want[0]) {
		t.Errorf("painted %v, want %v", got, want)
	}
	p.SetSelection(3, 3, selectionColor)
	if len(selectionOps(tree)) != 0 {
		t.Error("an empty range paints nothing")
	}
}

func TestParagraphSelection_RubyGroupIsWhole(t *testing.T) {
	tree := wtest.TextTree(t, Align{Alignment: layout.AlignTopLeft, Child: RichText{Spans: []text.Span{
		{Text: "漢字", Ruby: "かんじ"}, {Text: "です"},
	}}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 200})
	p := paragraphAtOrigin(t, tree)
	// An offset inside the group moves to its start.
	p.SetSelection(len("漢"), len("漢字で"), selectionColor)
	got := selectionOps(tree)
	want := p.TextLayout().SelectionRects(0, len("漢字で"), nil)
	if len(got) != 1 || !wtest.NearRect(got[0], want[0]) {
		t.Errorf("painted %v, want %v", got, want)
	}
}

func TestParagraphSelection_PaintDoesNotAllocate(t *testing.T) {
	for _, selected := range []bool{false, true} {
		tree := wtest.TextTree(t, Column{Children: []core.Widget{
			Text{Content: "The quick brown fox"},
			Text{Content: "jumps over\nthe lazy dog"},
		}})
		testutil.RenderTree(tree, geom.Size{W: 500, H: 100})
		if selected {
			var walk func(ro core.RenderObject)
			walk = func(ro core.RenderObject) {
				if p, ok := ro.(*RenderParagraph); ok {
					p.SetSelection(2, 20, selectionColor)
				}
				for i := range ro.ChildCount() {
					walk(ro.ChildAt(i))
				}
			}
			walk(tree.RootRenderObject())
		}
		var cv paint.Canvas
		cv.Reset()
		tree.Paint(&cv)
		allocs := testing.AllocsPerRun(10, func() {
			cv.Reset()
			tree.Paint(&cv)
		})
		if allocs != 0 {
			t.Errorf("selected %v: got %v allocs", selected, allocs)
		}
	}
}

// BenchmarkText_1000PaintSelected is BenchmarkText_1000Paint with every text
// selected: the highlight is drawn without allocating.
func BenchmarkText_1000PaintSelected(b *testing.B) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		b.Skip(err)
	}
	tree := core.NewTree(testutil.WithShaper(shaper, thousandTexts(0)))
	defer tree.Dispose()
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 800, H: 30000}))
	col := tree.RootRenderObject()
	for col.ChildCount() == 1 {
		col = col.ChildAt(0)
	}
	for i := range col.ChildCount() {
		if p, ok := col.ChildAt(i).(*RenderParagraph); ok {
			p.SetSelection(0, len(p.Text()), selectionColor)
		}
	}
	var cv paint.Canvas
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}
