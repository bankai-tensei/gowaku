package text

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestVertical_ColumnsStackFromTheRight(t *testing.T) {
	s := systemShaper(t)
	const em = 10
	st := Style{Size: em, Orientation: Vertical}
	l := s.Shape("あいうえお", st).Layout(3 * em)
	if got := l.LineCount(); got != 2 {
		t.Fatalf("got %d columns, want 2", got)
	}
	if got, want := l.Size(), (geom.Size{W: 2 * em * 1.5, H: 3 * em}); got != want {
		t.Errorf("size %v, want %v", got, want)
	}
	runs := l.Runs()
	first, last := runs[0], runs[len(runs)-1]
	if first.Origin.X <= last.Origin.X {
		t.Errorf("the first column (x=%v) must be right of the last (x=%v)", first.Origin.X, last.Origin.X)
	}
	g := first.Glyphs
	if len(g) < 2 || g[1].Y-g[0].Y != em {
		t.Errorf("glyphs must step down one em: %v", g)
	}
	// An upright glyph is centered on the column: its origin is half an em
	// left of the center.
	if g[0].X != -em/2 {
		t.Errorf("glyph x = %v, want %v", g[0].X, -em/2)
	}
}

func TestVertical_PunctuationTakesItsVerticalForm(t *testing.T) {
	s := systemShaper(t)
	for _, r := range []string{"、", "「", "ー"} {
		h := s.Shape(r, Style{Size: 16}).glyphs[0].id
		p := s.Shape(r, Style{Size: 16, Orientation: Vertical})
		if p.glyphs[0].id == h {
			t.Errorf("%s keeps its horizontal glyph", r)
		}
		if p.clusters[0].sideways {
			t.Errorf("%s must stand upright", r)
		}
	}
}

func TestVertical_WesternTurnsSideways(t *testing.T) {
	s := systemShaper(t)
	p := s.Shape("漢ab", Style{Size: 16, Orientation: Vertical})
	if p.clusters[0].sideways {
		t.Error("漢 must stand upright")
	}
	if !p.clusters[1].sideways || !p.clusters[2].sideways {
		t.Error("Latin must turn sideways")
	}
	runs := p.Layout(inf).Runs()
	sideways := 0
	for _, r := range runs {
		if r.Sideways {
			sideways++
		}
	}
	if sideways != 1 {
		t.Errorf("got %d sideways runs, want 1", sideways)
	}
}

func TestVertical_UprightSpan(t *testing.T) {
	s := systemShaper(t)
	p := s.ShapeSpans([]Span{{Text: "GPU", Upright: true}}, Style{Size: 16, Orientation: Vertical})
	for i := range p.clusters {
		if p.clusters[i].sideways {
			t.Errorf("cluster %d turned sideways", i)
		}
		if p.units[i].Advance != 16 {
			t.Errorf("cluster %d advances %v, want one em", i, p.units[i].Advance)
		}
	}
}

func TestTateChuYoko_TakesOneEm(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	spans := []Span{{Text: "明治"}, {Text: "39", TateChuYoko: true}, {Text: "年"}}
	p := s.ShapeSpans(spans, Style{Size: em, Orientation: Vertical})
	if got := len(p.clusters); got != 4 {
		t.Fatalf("got %d clusters, want 4", got)
	}
	u := p.units[2]
	if u.Advance != em || !u.Ideographic {
		t.Errorf("tate-chu-yoko unit %+v, want one ideographic em", u)
	}
	// Both digits sit within the em, side by side.
	c := p.clusters[2]
	a, b := p.glyphs[c.glyphStart], p.glyphs[c.glyphStart+1]
	if a.cross >= b.cross || a.cross < -em/2 || b.cross > em/2 {
		t.Errorf("digits at %v and %v, want within ±%v", a.cross, b.cross, em/2)
	}
	if a.inline != b.inline {
		t.Error("digits must share a baseline")
	}
	l := p.Layout(inf)
	if got := l.NextCaret(len("明治")); got != len("明治39") {
		t.Errorf("caret must step over the group, got %d", got)
	}
	// Horizontal text ignores the flag.
	h := s.ShapeSpans(spans, Style{Size: em})
	if got := len(h.clusters); got != 5 {
		t.Errorf("horizontal: got %d clusters, want 5", got)
	}
}

func TestTateChuYoko_ShrinksToFit(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	p := s.ShapeSpans([]Span{{Text: "2026", TateChuYoko: true}}, Style{Size: em, Orientation: Vertical})
	c := p.clusters[0]
	if c.size >= em {
		t.Errorf("size %v, want smaller than %v", c.size, em)
	}
	last := p.glyphs[c.glyphEnd-1]
	if p.glyphs[c.glyphStart].cross < -em/2-0.5 || last.cross > em/2 {
		t.Error("the digits must stay within the em")
	}
}

func TestVertical_CaretRunsAcrossTheColumn(t *testing.T) {
	s := systemShaper(t)
	const em = 10
	l := s.Shape("あいうえお", Style{Size: em, Orientation: Vertical}).Layout(3 * em)
	column := float32(em * 1.5)
	c := l.CaretRect(len("あ"))
	if want := (geom.Rect{X: column, Y: em, W: column}); c != want {
		t.Errorf("caret %v, want %v", c, want)
	}
	// The caret after う is at the top of the second column, to the left.
	c = l.CaretRect(len("あいう"))
	if want := (geom.Rect{X: 0, Y: 0, W: column}); c != want {
		t.Errorf("caret %v, want %v", c, want)
	}
	if got := l.HitTest(geom.Offset{X: column + 2, Y: em*2 + 1}); got != len("あい") {
		t.Errorf("hit = %d, want %d", got, len("あい"))
	}
	if got := l.HitTest(geom.Offset{X: 2, Y: em + 1}); got != len("あいうえ") {
		t.Errorf("hit = %d, want %d", got, len("あいうえ"))
	}
	rects := l.SelectionRects(len("あ"), len("あいうえ"), nil)
	want := []geom.Rect{{X: column, Y: em, W: column, H: 2 * em}, {X: 0, Y: 0, W: column, H: em}}
	if len(rects) != 2 || rects[0] != want[0] || rects[1] != want[1] {
		t.Errorf("selection %v, want %v", rects, want)
	}
	// Moving one line on goes one column left, at the same height.
	if got := l.VerticalMove(len("あ"), 1, l.CaretPosition(len("あ"))); got != len("あいうえ") {
		t.Errorf("move = %d, want %d", got, len("あいうえ"))
	}
}

func TestVertical_CommaHangsBelowTheColumn(t *testing.T) {
	s := systemShaper(t)
	const em = 10
	l := s.Shape("あいう、えお", Style{Size: em, Orientation: Vertical, Hanging: true}).Layout(3 * em)
	if got := lines(l, "あいう、えお"); len(got) != 2 || got[0] != "あいう、" {
		t.Fatalf("columns %q, want the comma on the first", got)
	}
	if got := l.Size().H; got != 3*em {
		t.Errorf("height %v: the hanging comma must stay outside", got)
	}
	g := l.Runs()[0].Glyphs
	if comma := g[3]; comma.Y < 3*em {
		t.Errorf("comma at y=%v, want below %v", comma.Y, 3*em)
	}
}

func TestVertical_RubyStandsRightOfItsBase(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	l := s.ShapeSpans([]Span{{Text: "漢字", Ruby: "かんじ"}}, Style{Size: em, Orientation: Vertical}).Layout(inf)
	runs := l.Runs()
	base, ruby := runs[0], runs[len(runs)-1]
	if ruby.Size != em/2 {
		t.Fatalf("ruby size %v", ruby.Size)
	}
	if got, want := ruby.Origin.X-base.Origin.X, float32(em+em/2)/2; got != want {
		t.Errorf("ruby column %v right of the base, want %v", got, want)
	}
	// The ruby still fits in the layout.
	if right := ruby.Origin.X + em/4; right > l.Size().W {
		t.Errorf("ruby reaches %v, past the width %v", right, l.Size().W)
	}
}

func TestJustify_StretchesAllButTheLastLine(t *testing.T) {
	s := systemShaper(t)
	const em = 10
	st := Style{Size: em, Align: AlignJustify}
	l := s.Shape("あいうえおかきくけこさ", st).Layout(5.5 * em)
	if got := l.LineCount(); got != 3 {
		t.Fatalf("got %d lines", got)
	}
	// The first line ends at the right edge, the last one where its text does.
	if got := l.CaretPosition(len("あいうえお")-1) + em; got < 5.5*em-0.01 {
		t.Errorf("first line ends at %v, want %v", got, 5.5*em)
	}
	if got := l.CaretPosition(len("あいうえおかきくけこさ")); got != em {
		t.Errorf("last line ends at %v, want %v", got, em)
	}
}

func TestIndent_FirstLineOfEachBlock(t *testing.T) {
	s := goShaper(t)
	st := Style{Size: 10, Indent: 20}
	l := s.Shape("ab\ncd\n", st).Layout(inf)
	for _, off := range []int{0, 3, 6} {
		if got := l.CaretRect(off).X; got != 20 {
			t.Errorf("caret at %d: x = %v, want 20", off, got)
		}
	}
}

func TestTabStop(t *testing.T) {
	s := goShaper(t)
	p := s.Shape("a\tb", Style{Size: 10, TabStop: 40})
	l := p.Layout(inf)
	if got := l.CaretPosition(2); got != 40 {
		t.Errorf("b at %v, want 40", got)
	}
	// Without tab stops a tab is a space.
	l = s.Shape("a\tb", Style{Size: 10}).Layout(inf)
	if got, space := l.CaretPosition(2), s.Shape("a b", Style{Size: 10}).Layout(inf).CaretPosition(2); got != space {
		t.Errorf("b at %v, want %v", got, space)
	}
}

func TestAtlas_SidewaysGlyphTurns(t *testing.T) {
	s := goShaper(t)
	a := NewAtlas(s)
	id := glyphID(t, s, "l")
	up := a.Glyph(GlyphKey{ID: id, Size: 40})
	side := a.Glyph(GlyphKey{ID: id, Size: 40, Sideways: true})
	if up.Height <= up.Width {
		t.Fatalf("l is %dx%d", up.Width, up.Height)
	}
	if side.Width != up.Height || side.Height != up.Width {
		t.Errorf("sideways %dx%d, want %dx%d", side.Width, side.Height, up.Height, up.Width)
	}
	// The top of the l points right, away from the origin.
	if side.Left < -1 || side.Left+side.Width < up.Height-1 {
		t.Errorf("sideways left = %d, want the ink right of the origin", side.Left)
	}
}
