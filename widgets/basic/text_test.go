package basic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// paragraphAtOrigin returns the text at the top left of a tree that was laid out.
func paragraphAtOrigin(t testing.TB, tree *core.Tree) *RenderParagraph {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 1, Y: 1}).Path {
		if p, ok := ro.(*RenderParagraph); ok {
			return p
		}
	}
	t.Fatal("no text at the origin")
	return nil
}

func TestText_Golden(t *testing.T) {
	w := Pad(10, Column{Spacing: 4, Cross: layout.CrossStart, Children: []core.Widget{
		Text{Content: "hello", Style: text.Style{Size: 20, Color: geom.Hex(0x336699)}},
		Text{Content: "wrapped words go on the next line", Style: text.Style{Size: 16}},
	}})
	got := testutil.RenderTree(wtest.TextTree(t, w), geom.Size{W: 120, H: 200})
	testutil.Golden(t, "text", got)
}

func TestText_ZeroValueTakesOneLine(t *testing.T) {
	tree := wtest.TextTree(t, Center{Child: Text{}})
	if got := testutil.RenderTree(tree, geom.Size{W: 100, H: 100}); got != "" {
		t.Errorf("got %q", got)
	}
	ro := paragraph{shaper: testutil.NewShaper(t)}.CreateRenderObject()
	if got, want := ro.Layout(geom.Loose(geom.Size{W: 100, H: 100})), (geom.Size{H: 21}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestText_WithoutShaperPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want a panic")
		}
	}()
	tree := core.NewTree(Text{})
	defer tree.Dispose()
	tree.Build()
}

func TestText_WrapsToTheConstraint(t *testing.T) {
	tree := wtest.TextTree(t, Align{Child: SizedBox{Width: 60, Child: Text{Content: "aaa bbb ccc ddd eee"}}})
	testutil.RenderTree(tree, geom.Size{W: 500, H: 500})
	p := paragraphAtOrigin(t, tree)
	if n := p.TextLayout().LineCount(); n < 2 {
		t.Errorf("got %d lines", n)
	}
	if p.Size().W != 60 {
		t.Errorf("got width %v", p.Size().W)
	}
}

func TestText_ColorChangeOnlyRepaints(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Text{Content: "same", Style: text.Style{Color: geom.Hex(0x111111)}}, State: &state})
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	p := paragraphAtOrigin(t, tree)
	before := p.TextLayout()

	state.SetState(func() { state.Child = Text{Content: "same", Style: text.Style{Color: geom.Hex(0x222222)}} })
	got := testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	if p.TextLayout() != before {
		t.Error("a new color must not lay out again")
	}
	if want := "#222222ff"; !strings.Contains(got, want) {
		t.Errorf("got %q, want the color %s", got, want)
	}
}

func TestText_ContentChangeLaysOutAgain(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Text{Content: "one"}, State: &state})
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	p := paragraphAtOrigin(t, tree)
	before := p.TextLayout()
	state.SetState(func() { state.Child = Text{Content: "two words"} })
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	if p.TextLayout() == before {
		t.Error("new content must lay out again")
	}
}

func TestText_SameWidgetDoesNothing(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Text{Content: "same"}, State: &state})
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	p := paragraphAtOrigin(t, tree)
	before := p.TextLayout()
	state.SetState(func() { state.Child = Text{Content: "same"} })
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	if p.TextLayout() != before {
		t.Error("the same text must keep its layout")
	}
}

func TestText_Semantics(t *testing.T) {
	tree := wtest.TextTree(t, Text{Content: "label"})
	testutil.RenderTree(tree, geom.Size{W: 200, H: 100})
	root := tree.Semantics()
	if len(root.Children) != 1 || root.Children[0].Role != semantics.RoleText || root.Children[0].Label != "label" {
		t.Errorf("got %+v", root)
	}
}

func TestText_MaxLines(t *testing.T) {
	const content = "a preview of a long message that goes on past the row"
	w := Pad(10, Column{Spacing: 4, Cross: layout.CrossStart, Children: []core.Widget{
		Text{Content: content, Style: text.Style{Size: 16, MaxLines: 1}},
		Text{Content: content + " and on", Style: text.Style{Size: 16, MaxLines: 2}},
	}})
	tree := wtest.TextTree(t, w)
	got := testutil.RenderTree(tree, geom.Size{W: 160, H: 200})
	testutil.Golden(t, "text_maxlines", got)
	var p *RenderParagraph
	for _, ro := range tree.HitTest(geom.Offset{X: 12, Y: 12}).Path {
		if rp, ok := ro.(*RenderParagraph); ok {
			p = rp
		}
	}
	if p == nil {
		t.Fatal("no text")
	}
	if l := p.TextLayout(); l.LineCount() != 1 || !l.Truncated() {
		t.Errorf("%d lines, truncated %v", l.LineCount(), l.Truncated())
	}
	if p.Overflow() != 0 {
		t.Errorf("overflow %v", p.Overflow())
	}
	// A screen reader reads the whole text.
	if n := wtest.FindRole(tree.Semantics(), semantics.RoleText); n == nil || n.Label != content {
		t.Errorf("got %+v", n)
	}
	// Laying out again at the same width keeps the layout.
	c := geom.Loose(geom.Size{W: 140, H: 200})
	p.Layout(c)
	if allocs := testing.AllocsPerRun(10, func() { p.Layout(c) }); allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func TestText_PaintDoesNotAllocate(t *testing.T) {
	tree := wtest.TextTree(t, Text{Content: "The quick brown fox"})
	testutil.RenderTree(tree, geom.Size{W: 500, H: 100})
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

// thousandTexts is a scene of 1000 different short strings.
func thousandTexts(generation int) core.Widget {
	children := make([]core.Widget, 1000)
	for i := range children {
		children[i] = Text{Content: fmt.Sprintf("項目 %d の説明文 Item %d", i, generation), Style: text.Style{Size: 14}}
	}
	return Column{Children: children}
}

// BenchmarkText_1000Cold is the first frame of 1000 new texts: shaping, line
// breaking and painting. It allocates, which is allowed on a cache miss
// (CODING_STYLE §7).
func BenchmarkText_1000Cold(b *testing.B) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		b.Skip(err)
	}
	var cv paint.Canvas
	b.ReportAllocs()
	generation := 0
	for b.Loop() {
		generation++
		tree := core.NewTree(testutil.WithShaper(shaper, thousandTexts(generation)))
		tree.Build()
		tree.Layout(geom.Tight(geom.Size{W: 800, H: 30000}))
		cv.Reset()
		tree.Paint(&cv)
		tree.Dispose()
	}
}

// BenchmarkText_1000Paint is a repaint of 1000 texts with nothing else changed.
func BenchmarkText_1000Paint(b *testing.B) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		b.Skip(err)
	}
	tree := core.NewTree(testutil.WithShaper(shaper, thousandTexts(0)))
	defer tree.Dispose()
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 800, H: 30000}))
	var cv paint.Canvas
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}
