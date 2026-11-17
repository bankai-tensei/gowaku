package text

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// inf stands for an unconstrained width.
const inf = float32(1e9)

func goShaper(t testing.TB) *Shaper {
	t.Helper()
	s, err := NewShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// systemShaper returns a Shaper with a Japanese font, or skips the test.
func systemShaper(t testing.TB) *Shaper {
	t.Helper()
	s, err := NewSystemShaper()
	if err != nil {
		t.Skip(err)
	}
	if _, ok := s.fonts.FindSystemFont("Yu Gothic"); !ok {
		if _, ok := s.fonts.FindSystemFont("Noto Sans CJK JP"); !ok {
			t.Skip("no Japanese font installed")
		}
	}
	return s
}

func lines(l *Layout, content string) []string {
	out := make([]string, 0, l.LineCount())
	for i := range l.LineCount() {
		a, b := l.LineRange(i)
		out = append(out, content[a:b])
	}
	return out
}

func ExampleShaper_Shape() {
	s, err := NewShaper(goregular.TTF)
	if err != nil {
		panic(err)
	}
	const content = "lines break between words"
	l := s.Shape(content, Style{Size: 16}).Layout(120)
	for i := range l.LineCount() {
		start, end := l.LineRange(i)
		fmt.Printf("%q\n", content[start:end])
	}
	// Output:
	// "lines break "
	// "between words"
}

func TestNewShaper_NeedsFonts(t *testing.T) {
	if _, err := NewShaper(); err == nil {
		t.Error("want an error without fonts")
	}
	if _, err := NewShaper([]byte("not a font")); err == nil {
		t.Error("want an error for garbage")
	}
}

func TestLayout_Empty(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("", Style{Size: 10}).Layout(100)
	if l.LineCount() != 1 {
		t.Fatalf("got %d lines", l.LineCount())
	}
	if want := (geom.Size{H: 15}); l.Size() != want {
		t.Errorf("got %v, want %v", l.Size(), want)
	}
	if len(l.Runs()) != 0 {
		t.Errorf("got %d runs", len(l.Runs()))
	}
}

func TestLayout_WrapsWesternWords(t *testing.T) {
	s := goShaper(t)
	const content = "the quick brown fox"
	p := s.Shape(content, Style{Size: 20})
	one := p.Layout(inf)
	if one.LineCount() != 1 {
		t.Fatalf("got %d lines", one.LineCount())
	}
	narrow := p.Layout(one.Size().W * 0.6)
	got := strings.Join(lines(narrow, content), "|")
	if got != "the quick |brown fox" {
		t.Errorf("got %q", got)
	}
	if narrow.Size().W > one.Size().W*0.6 {
		t.Errorf("width %v is past the limit", narrow.Size().W)
	}
}

func TestParagraph_MinLength(t *testing.T) {
	s := goShaper(t)
	const content = "a long yellow fruit\nbanana 100"
	p := s.Shape(content, Style{Size: 20})
	words := strings.Fields(content)
	var widest float32
	for _, w := range words {
		widest = max(widest, s.Shape(w, Style{Size: 20}).Layout(inf).Size().W)
	}
	if got := p.MinLength(); got != widest {
		t.Errorf("MinLength = %v, want the width of the widest word %v", got, widest)
	}
	// At that width every line ends between words.
	var got []string
	for _, ln := range lines(p.Layout(p.MinLength()), content) {
		got = append(got, strings.Fields(ln)...)
	}
	if strings.Join(got, " ") != strings.Join(words, " ") {
		t.Errorf("at MinLength a word was split: %q", lines(p.Layout(p.MinLength()), content))
	}
	if n := testing.AllocsPerRun(10, func() { p.MinLength() }); n != 0 {
		t.Errorf("MinLength allocates %v times", n)
	}
	if got := s.Shape("", Style{Size: 20}).MinLength(); got != 0 {
		t.Errorf("empty MinLength = %v", got)
	}
}

func TestLayout_HardBreaks(t *testing.T) {
	s := goShaper(t)
	const content = "a\n\r\nb\r\n"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	got := strings.Join(lines(l, content), "|")
	if got != "a||b|" || l.LineCount() != 4 {
		t.Errorf("got %d lines: %q", l.LineCount(), got)
	}
	if l.Size().H != 60 {
		t.Errorf("height %v", l.Size().H)
	}
}

func TestLayout_Align(t *testing.T) {
	s := goShaper(t)
	const content = "wide line here\nx"
	for _, tt := range []struct {
		name  string
		align Align
		want  func(w, line float32) float32
	}{
		{"start", AlignStart, func(float32, float32) float32 { return 0 }},
		{"center", AlignCenter, func(w, line float32) float32 { return (w - line) / 2 }},
		{"end", AlignEnd, func(w, line float32) float32 { return w - line }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			l := s.Shape(content, Style{Size: 20, Align: tt.align}).Layout(inf)
			ln := &l.lines[1]
			if got, want := ln.x, tt.want(l.Size().W, ln.width); got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

// Centered and end-aligned text takes the whole length it is laid out at, and
// its lines are aligned within it; text set to the start takes its longest
// line.
func TestLayout_AlignWithinTheLength(t *testing.T) {
	s := goShaper(t)
	for _, tt := range []struct {
		align Align
		width float32
	}{{AlignStart, 0}, {AlignCenter, 200}, {AlignEnd, 200}} {
		l := s.Shape("short", Style{Size: 20, Align: tt.align}).Layout(200)
		ln := &l.lines[0]
		want := tt.width
		if want == 0 {
			want = ln.width
		}
		if l.Size().W != want {
			t.Errorf("align %d: width %v, want %v", tt.align, l.Size().W, want)
		}
		if tt.align == AlignCenter && ln.x != (200-ln.width)/2 {
			t.Errorf("centered line at %v, want %v", ln.x, (200-ln.width)/2)
		}
	}
	if w := s.Shape("short", Style{Size: 20, Align: AlignCenter}).Layout(geom.Inf).Size().W; w >= 200 {
		t.Errorf("unbounded centered text is %v wide, want its line", w)
	}
}

func TestLayout_RunsArePositioned(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("ab\ncd", Style{Size: 10, LineHeight: 2}).Layout(inf)
	runs := l.Runs()
	if len(runs) != 2 {
		t.Fatalf("got %d runs", len(runs))
	}
	if d := runs[1].Origin.Y - runs[0].Origin.Y; d != 20 {
		t.Errorf("baselines %v apart", d)
	}
	if runs[0].Size != 10 || len(runs[0].Glyphs) != 2 {
		t.Errorf("got %+v", runs[0])
	}
	if runs[0].Glyphs[1].X <= runs[0].Glyphs[0].X {
		t.Errorf("glyphs do not advance: %+v", runs[0].Glyphs)
	}
}

func TestStyle_ColorDoesNotChangeLayout(t *testing.T) {
	a := Style{Color: geom.Hex(0xff0000)}
	if !a.SameLayout(Style{Size: defaultSize}) {
		t.Error("color and defaults must not count")
	}
	if a.SameLayout(Style{Size: 20}) {
		t.Error("size must count")
	}
}

func TestStyle_PaintColor(t *testing.T) {
	var s Style
	if got := s.PaintColor(); got != (geom.Color{A: 1}) {
		t.Errorf("got %v", got)
	}
}

func TestCaret_RoundTrip(t *testing.T) {
	s := goShaper(t)
	const content = "hello world again"
	p := s.Shape(content, Style{Size: 20})
	l := p.Layout(p.Layout(inf).Size().W / 2)
	if l.LineCount() < 2 {
		t.Fatal("want at least two lines")
	}
	for off := range len(content) + 1 {
		r := l.CaretRect(off)
		got := l.HitTest(geom.Offset{X: r.X, Y: r.Y + r.H/2})
		// A trailing space hangs, so its right side folds onto the line end.
		if got != off && content[max(off-1, 0)] != ' ' {
			t.Errorf("offset %d: caret at %v hits %d", off, r, got)
		}
	}
}

func TestCaret_HardBreakAndEmptyLine(t *testing.T) {
	s := goShaper(t)
	const content = "a\n\nb"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	if got := l.CaretRect(2); got.Y != 15 || got.X != 0 {
		t.Errorf("empty line: got %v", got)
	}
	if got := l.CaretRect(3); got.Y != 30 {
		t.Errorf("last line: got %v", got)
	}
	if got := l.HitTest(geom.Offset{X: 50, Y: 20}); got != 2 {
		t.Errorf("hit on empty line: got %d", got)
	}
}

func TestHitTest_OutsideClamps(t *testing.T) {
	s := goShaper(t)
	const content = "abc\ndef"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	tests := []struct {
		name string
		pos  geom.Offset
		want int
	}{
		{"左上の外", geom.Offset{X: -5, Y: -5}, 0},
		{"右下の外", geom.Offset{X: 500, Y: 500}, len(content)},
		{"1 行目の右の外", geom.Offset{X: 500, Y: 1}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := l.HitTest(tt.pos); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSelectionRects(t *testing.T) {
	s := goShaper(t)
	const content = "abc\ndef\nghi"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	rects := l.SelectionRects(1, 9, nil)
	if len(rects) != 3 {
		t.Fatalf("got %v", rects)
	}
	if rects[0].X != l.CaretRect(1).X {
		t.Errorf("first rect starts at %v", rects[0].X)
	}
	if rects[1].X != 0 || rects[1].Y != 15 {
		t.Errorf("middle rect is %v", rects[1])
	}
	if got, want := rects[2].X+rects[2].W, l.CaretRect(9).X; got != want {
		t.Errorf("last rect ends at %v, want %v", got, want)
	}
	if got := l.SelectionRects(2, 2, nil); len(got) != 0 {
		t.Errorf("empty selection: %v", got)
	}
}

func TestClusters_CombiningMarkStaysWhole(t *testing.T) {
	s := goShaper(t)
	const content = "éx"
	p := s.Shape(content, Style{Size: 10})
	if len(p.clusters) != 2 {
		t.Fatalf("got %d clusters", len(p.clusters))
	}
	if c := p.clusters[0]; c.start != 0 || c.end != 3 {
		t.Errorf("got %+v", c)
	}
	l := p.Layout(inf)
	if l.CaretRect(1) != l.CaretRect(0) {
		t.Error("an offset inside a cluster must move to its start")
	}
}

func TestJapanese_FontsAreFullWidth(t *testing.T) {
	s := systemShaper(t)
	p := s.Shape("「日本語」、かな。", Style{Size: 16})
	for i, u := range p.units {
		if !u.FullWidth {
			t.Errorf("unit %d %q: advance %v is not full width", i, u.Rune, u.Advance)
		}
	}
}

func TestJapanese_Composition(t *testing.T) {
	s := systemShaper(t)
	const size = 20
	style := Style{Size: size}
	tests := []struct {
		name    string
		content string
		width   float32
		style   Style
		want    string
	}{
		{"行頭禁則", "吾輩は猫である。名前はまだ無い。", 7 * size, style, "吾輩は猫であ|る。名前はまだ|無い。"},
		{"行末禁則", "吾輩は「猫」である", 4 * size, style, "吾輩は|「猫」で|ある"},
		{"ぶら下げ", "吾輩は猫である。名前はまだ無い。", 7 * size, Style{Size: size, Hanging: true}, "吾輩は猫である。|名前はまだ無い。"},
		{"strict は小書きかなを行頭に置かない", "あいうえおっか", 5 * size, style, "あいうえ|おっか"},
		{"loose は小書きかなを行頭に置く", "あいうえおっか", 5 * size, Style{Size: size, LineBreak: jlreq.Loose}, "あいうえお|っか"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := s.Shape(tt.content, tt.style).Layout(tt.width)
			if got := strings.Join(lines(l, tt.content), "|"); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestJapanese_LineStartBracketInkAlignsWithMargin(t *testing.T) {
	s := systemShaper(t)
	const size = 20
	l := s.Shape("「猫」", Style{Size: size}).Layout(inf)
	if got := l.Runs()[0].Glyphs[0].X; got != -size/2 {
		t.Errorf("got %v, want %v", got, -size/2)
	}
}

func TestCaret_Stepping(t *testing.T) {
	s := goShaper(t)
	const content = "e\u0301x\r\ny"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	var forward []int
	for at := 0; at < len(content); {
		at = l.NextCaret(at)
		forward = append(forward, at)
	}
	want := []int{3, 4, 6, 7}
	if !slices.Equal(forward, want) {
		t.Errorf("forward: got %v, want %v", forward, want)
	}
	var back []int
	for at := len(content); at > 0; {
		at = l.PrevCaret(at)
		back = append(back, at)
	}
	if want := []int{6, 4, 3, 0}; !slices.Equal(back, want) {
		t.Errorf("back: got %v, want %v", back, want)
	}
	if l.NextCaret(len(content)) != len(content) || l.PrevCaret(0) != 0 {
		t.Error("the ends must hold")
	}
}

func TestCaret_EmptyLinesAreStops(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("a\n\nb", Style{Size: 10}).Layout(inf)
	if got := l.NextCaret(1); got != 2 {
		t.Errorf("got %d", got)
	}
	if got := l.NextCaret(2); got != 3 {
		t.Errorf("got %d", got)
	}
}

func TestWordRange(t *testing.T) {
	s := goShaper(t)
	const content = "日本語のテキストとhello world"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	tests := []struct {
		name   string
		offset int
		want   string
	}{
		{"漢字", 3, "日本語"},
		{"ひらがな", 9, "の"},
		{"カタカナ", 15, "テキスト"},
		{"欧文", strings.Index(content, "llo"), "hello"},
		{"末尾", len(content), "world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := l.WordRange(tt.offset)
			if got := content[a:b]; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWord_Stepping(t *testing.T) {
	s := goShaper(t)
	const content = "one two  three"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	if got := l.NextWord(0); got != 4 {
		t.Errorf("next: got %d", got)
	}
	if got := l.NextWord(4); got != 9 {
		t.Errorf("next over two spaces: got %d", got)
	}
	if got := l.PrevWord(9); got != 4 {
		t.Errorf("prev: got %d", got)
	}
	if got := l.PrevWord(len(content)); got != 9 {
		t.Errorf("prev from the end: got %d", got)
	}
	if got := l.PrevWord(4); got != 0 {
		t.Errorf("prev to the start: got %d", got)
	}
}

func TestLineEnds_AndVerticalMove(t *testing.T) {
	s := goShaper(t)
	const content = "abc\ndefgh\nij"
	l := s.Shape(content, Style{Size: 10}).Layout(inf)
	if a, b := l.LineStart(5), l.LineEnd(5); a != 4 || b != 9 {
		t.Errorf("got %d %d", a, b)
	}
	x := l.CaretRect(2).X
	if got := l.VerticalMove(2, 1, x); got != 6 {
		t.Errorf("down: got %d", got)
	}
	if got := l.VerticalMove(6, -1, x); got != 2 {
		t.Errorf("up: got %d", got)
	}
	if got := l.VerticalMove(2, -1, x); got != 2 {
		t.Errorf("past the first line: got %d", got)
	}
	if got := l.VerticalMove(8, 1, l.CaretRect(8).X); got != len(content) {
		t.Errorf("onto a shorter line: got %d", got)
	}
}

func TestSpans_KeepTheirLook(t *testing.T) {
	s := goShaper(t)
	red := geom.Hex(0xff0000)
	spans := []Span{{Text: "ab"}, {Text: "cd", Style: Style{Size: 20, Color: red}}, {Text: "e"}}
	p := s.ShapeSpans(spans, Style{Size: 10})
	if p.Text() != "abcde" {
		t.Fatalf("got %q", p.Text())
	}
	l := p.Layout(inf)
	runs := l.Runs()
	if len(runs) != 3 {
		t.Fatalf("got %d runs", len(runs))
	}
	if runs[1].Size != 20 || runs[1].Color != red || len(runs[1].Glyphs) != 2 {
		t.Errorf("got %+v", runs[1])
	}
	if runs[0].Color != (geom.Color{}) {
		t.Errorf("got %+v", runs[0])
	}
	// The line takes the height of its largest text.
	if got := l.Size().H; got != 30 {
		t.Errorf("height %v", got)
	}
	if runs[2].Glyphs[0].X <= runs[1].Glyphs[1].X {
		t.Error("spans must follow each other")
	}
}

func TestSpans_HardBreakInside(t *testing.T) {
	s := goShaper(t)
	l := s.ShapeSpans([]Span{{Text: "a\nb"}, {Text: "c"}}, Style{Size: 10}).Layout(inf)
	if l.LineCount() != 2 {
		t.Errorf("got %d lines", l.LineCount())
	}
	if got := lines(l, "a\nbc"); got[1] != "bc" {
		t.Errorf("got %q", got)
	}
}

func TestSpans_ShapeEqualsOneSpan(t *testing.T) {
	s := goShaper(t)
	a := s.Shape("hello world", Style{Size: 10}).Layout(40)
	b := s.ShapeSpans([]Span{{Text: "hello "}, {Text: "world"}}, Style{Size: 10}).Layout(40)
	if a.LineCount() != b.LineCount() || a.Size() != b.Size() {
		t.Errorf("got %v and %v", a.Size(), b.Size())
	}
}

func TestRuby_StaysAboveItsBase(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	l := s.ShapeSpans([]Span{{Text: "前"}, {Text: "漢字", Ruby: "かんじ"}, {Text: "後"}}, Style{Size: em}).Layout(inf)
	runs := l.Runs()
	ruby := runs[len(runs)-1]
	if ruby.Size != em*rubySize || len(ruby.Glyphs) != 3 {
		t.Fatalf("got %+v", ruby)
	}
	if ruby.Origin.Y >= runs[0].Origin.Y {
		t.Errorf("ruby at %v, base at %v", ruby.Origin.Y, runs[0].Origin.Y)
	}
	// Ruby of 1.5 em over a base of 2 em spreads out, centered over the base.
	left := runs[0].Glyphs[1].X
	center := left + em
	mid := ruby.Glyphs[1].X + em*rubySize/2
	if d := mid - center; d > 0.5 || d < -0.5 {
		t.Errorf("ruby centered at %v, base at %v", mid, center)
	}
	if gap := ruby.Glyphs[1].X - ruby.Glyphs[0].X; gap <= em*rubySize {
		t.Errorf("short ruby must spread out, step %v", gap)
	}
}

func TestRuby_GroupStaysOnOneLine(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	spans := []Span{{Text: "あい"}, {Text: "東京都", Ruby: "とうきょうと"}, {Text: "うえ"}}
	l := s.ShapeSpans(spans, Style{Size: em}).Layout(4 * em)
	if l.LineCount() < 2 {
		t.Fatal("want a break")
	}
	got := lines(l, "あい東京都うえ")
	if got[0] != "あい" {
		t.Errorf("the group must move as one: %q", got)
	}
}

func TestRuby_LongRubyOverhangsKanaThenOpensTheBase(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	width := func(spans []Span) float32 { return s.ShapeSpans(spans, Style{Size: em}).Layout(inf).Size().W }
	// Four ruby characters are 2 em over a base of 1 em.
	free := width([]Span{{Text: "あ"}, {Text: "字", Ruby: "ああああ"}, {Text: "い"}})
	if free != 3*em {
		t.Errorf("between kana the ruby may overhang: %v", free)
	}
	kanji := width([]Span{{Text: "漢"}, {Text: "字", Ruby: "ああああ"}, {Text: "漢"}})
	if kanji != 4*em {
		t.Errorf("between kanji the base must open up: %v", kanji)
	}
}

func TestRuby_ReservesRoom(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	p := s.ShapeSpans([]Span{{Text: "字", Ruby: "じ"}}, Style{Size: em})
	l := p.Layout(inf)
	if got := l.Size().H; got < em*RubyLineHeight {
		t.Errorf("height %v", got)
	}
	runs := l.Runs()
	ruby := runs[len(runs)-1]
	if top := ruby.Origin.Y - rubyInk*rubySize*em; top < -0.01 {
		t.Errorf("ruby starts at %v, above the layout", top)
	}
}

func TestAtlas_ReturnsTheSameImage(t *testing.T) {
	s := goShaper(t)
	id := glyphID(t, s, "a")
	a := NewAtlas(s)
	a.BeginFrame()
	k := GlyphKey{Face: 0, ID: id, Size: 16}
	first := a.Glyph(k)
	if first.Width == 0 || first.Height == 0 {
		t.Fatalf("got %+v", first)
	}
	if got := a.Glyph(k); got != first {
		t.Errorf("got %+v, want %+v", got, first)
	}
	r, pix := a.TakeDirty(0)
	want := image.Rect(first.X, first.Y, first.X+first.Width, first.Y+first.Height)
	if r != want {
		t.Errorf("dirty %v, want %v", r, want)
	}
	if ink := inkOf(pix, a.PageSize(), want); ink == 0 {
		t.Error("the glyph left no ink")
	}
	if r, _ := a.TakeDirty(0); !r.Empty() {
		t.Errorf("still dirty: %v", r)
	}
}

func TestAtlas_SubpixelPositionsDiffer(t *testing.T) {
	s := goShaper(t)
	id := glyphID(t, s, "l")
	a := NewAtlas(s)
	a.BeginFrame()
	img0 := a.Glyph(GlyphKey{ID: id, Size: 16})
	img2 := a.Glyph(GlyphKey{ID: id, Size: 16, Subpixel: 2})
	if img0.X == img2.X && img0.Y == img2.Y {
		t.Error("positions must get their own images")
	}
}

func TestAtlas_SpaceHasNoImage(t *testing.T) {
	s := goShaper(t)
	id := glyphID(t, s, " ")
	a := NewAtlas(s)
	a.BeginFrame()
	if got := a.Glyph(GlyphKey{ID: id, Size: 16}); got.Width != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestAtlas_HitDoesNotAllocate(t *testing.T) {
	s := goShaper(t)
	a := NewAtlas(s)
	a.BeginFrame()
	k := GlyphKey{ID: glyphID(t, s, "a"), Size: 16}
	a.Glyph(k)
	if got := testing.AllocsPerRun(100, func() { a.Glyph(k) }); got != 0 {
		t.Errorf("got %v allocs", got)
	}
}

func TestAtlas_EvictsOldPages(t *testing.T) {
	s := goShaper(t)
	// A page this small holds one big glyph, so every size needs its own.
	a := newAtlas(s, 64, 2)
	id := glyphID(t, s, "M")
	key := func(size float32) GlyphKey { return GlyphKey{ID: id, Size: size} }
	a.BeginFrame()
	a.Glyph(key(50))
	a.BeginFrame()
	a.Glyph(key(49))
	a.BeginFrame()
	a.Glyph(key(49)) // keeps the second page in use
	a.Glyph(key(48))
	if a.PageCount() != 2 {
		t.Fatalf("got %d pages", a.PageCount())
	}
	if _, ok := a.entries[key(50)]; ok {
		t.Error("the oldest page must be forgotten")
	}
	if _, ok := a.entries[key(49)]; !ok {
		t.Error("a page in use must survive")
	}
}

func TestAtlas_FrameLargerThanLimitGrows(t *testing.T) {
	s := goShaper(t)
	a := newAtlas(s, 64, 1)
	id := glyphID(t, s, "M")
	a.BeginFrame()
	a.Glyph(GlyphKey{ID: id, Size: 50})
	a.Glyph(GlyphKey{ID: id, Size: 49})
	if a.PageCount() != 2 {
		t.Errorf("got %d pages", a.PageCount())
	}
}

func TestAtlas_ShapeRasterizesAPath(t *testing.T) {
	s := goShaper(t)
	a := NewAtlas(s)
	a.BeginFrame()
	p := &paint.Path{}
	p.AddRRect(geom.RRect{Rect: geom.Rect{X: 2, Y: 3, W: 10, H: 10}})
	k := ShapeKey{Path: p, Scale: 2}
	img := a.Shape(k)
	if img.Left != 4 || img.Top != 6 || img.Width < 20 || img.Height < 20 {
		t.Fatalf("got %+v", img)
	}
	r, pix := a.TakeDirty(img.Page)
	if inkOf(pix, a.PageSize(), r) != 20*20*255 {
		t.Errorf("got %d of ink", inkOf(pix, a.PageSize(), r))
	}
	if got := a.Shape(k); got != img {
		t.Errorf("got %+v", got)
	}
}

func TestAtlas_ShapeGoesByWhatThePathHolds(t *testing.T) {
	s := goShaper(t)
	a := NewAtlas(s)
	a.BeginFrame()
	p := &paint.Path{}
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}})
	small := a.Shape(ShapeKey{Path: p, Scale: 1})
	p.Reset()
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 20, H: 20}})
	large := a.Shape(ShapeKey{Path: p, Scale: 1})
	if large == small || large.Width <= small.Width {
		t.Errorf("a changed path must get its own image, got %+v after %+v", large, small)
	}
	q := &paint.Path{}
	q.AddRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}})
	if got := a.Shape(ShapeKey{Path: q, Scale: 1}); got != small {
		t.Errorf("an equal path must share the image, got %+v, want %+v", got, small)
	}
	if allocs := testing.AllocsPerRun(10, func() { a.Shape(ShapeKey{Path: q, Scale: 1}) }); allocs != 0 {
		t.Errorf("a hit made %v allocs", allocs)
	}
}

func TestAtlas_TooLargeGlyphIsBlank(t *testing.T) {
	s := goShaper(t)
	a := newAtlas(s, 64, 1)
	a.BeginFrame()
	if got := a.Glyph(GlyphKey{ID: glyphID(t, s, "M"), Size: 500}); got.Width != 0 {
		t.Errorf("got %+v", got)
	}
}

func glyphID(t testing.TB, s *Shaper, r string) uint32 {
	t.Helper()
	p := s.Shape(r, Style{Size: 16})
	if len(p.glyphs) != 1 {
		t.Fatalf("got %d glyphs", len(p.glyphs))
	}
	return p.glyphs[0].id
}

func inkOf(pix []byte, stride int, r image.Rectangle) int {
	sum := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			sum += int(pix[y*stride+x])
		}
	}
	return sum
}

func BenchmarkShape_ShortJapanese(b *testing.B) {
	s := systemShaper(b)
	b.ReportAllocs()
	for b.Loop() {
		s.Shape("項目の説明文 Item 123", Style{Size: 16}).Layout(300)
	}
}

// BenchmarkAtlas_Rasterize is the cost of one glyph seen for the first time.
func BenchmarkAtlas_Rasterize(b *testing.B) {
	s := systemShaper(b)
	id := s.Shape("語", Style{Size: 16}).glyphs[0].id
	a := NewAtlas(s)
	a.BeginFrame()
	size := float32(16)
	b.ReportAllocs()
	for b.Loop() {
		size += 0.001 // a new key each time
		a.Glyph(GlyphKey{ID: id, Size: size})
	}
}

func BenchmarkLayout_Rewrap(b *testing.B) {
	s := systemShaper(b)
	p := s.Shape(strings.Repeat("吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。", 20), Style{Size: 16})
	b.ReportAllocs()
	for b.Loop() {
		p.Layout(400)
	}
}

// nonZeroArea measures the area inside p by the non-zero rule, by sampling a
// fine grid.
func nonZeroArea(p *paint.Path) float32 {
	points, starts := p.Polygon()
	b := p.Bounds()
	const step = 1.0 / 64
	inside := 0
	for y := b.Y + step/2; y < b.Y+b.H; y += step {
		for x := b.X + step/2; x < b.X+b.W; x += step {
			w := 0
			for i, start := range starts {
				end := len(points)
				if i+1 < len(starts) {
					end = starts[i+1]
				}
				for k := start; k < end; k++ {
					a, c := points[k], points[start+(k+1-start)%(end-start)]
					cross := (c.X-a.X)*(y-a.Y) - (x-a.X)*(c.Y-a.Y)
					if a.Y <= y && c.Y > y && cross > 0 {
						w++
					} else if a.Y > y && c.Y <= y && cross < 0 {
						w--
					}
				}
			}
			if w != 0 {
				inside++
			}
		}
	}
	return float32(inside) * step * step
}

func TestAtlas_OverlappingContoursAreNotBolder(t *testing.T) {
	s := goShaper(t)
	triangle := func(p *paint.Path) {
		p.MoveTo(1.3, 1.2)
		p.LineTo(11.7, 2.1)
		p.LineTo(4.4, 9.9)
		p.Close()
	}
	rect := func(p *paint.Path, reverse bool) {
		pts := [4][2]float32{{4, 3}, {14, 3}, {14, 6}, {4, 6}}
		if reverse {
			pts[1], pts[3] = pts[3], pts[1]
		}
		p.MoveTo(pts[0][0], pts[0][1])
		for _, q := range pts[1:] {
			p.LineTo(q[0], q[1])
		}
		p.Close()
	}
	once, twice, union, crossed := &paint.Path{}, &paint.Path{}, &paint.Path{}, &paint.Path{}
	triangle(once)
	triangle(twice)
	triangle(twice)
	triangle(union)
	rect(union, false)
	triangle(crossed)
	rect(crossed, true)
	for name, p := range map[string]*paint.Path{"once": once, "twice": twice, "union": union, "crossed": crossed} {
		a := NewAtlas(s)
		a.BeginFrame()
		img := a.Shape(ShapeKey{Path: p, Scale: 2})
		r, pix := a.TakeDirty(img.Page)
		got := float32(inkOf(pix, a.PageSize(), r)) / 255 / 4
		// Both rasterizers are exact to well under a percent.
		if want := nonZeroArea(p); got < want*0.99 || got > want*1.01 {
			t.Errorf("%s: %v of ink, want %v", name, got, want)
		}
	}
}
