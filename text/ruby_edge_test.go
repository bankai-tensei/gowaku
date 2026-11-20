package text

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// rubyExtents returns the inline extent of the ruby of l, one ruby after the
// other, from the em box of its first glyph to that of its last.
func rubyExtents(l *Layout, rubyEm float32) [][2]float32 {
	var out [][2]float32
	p := l.paragraph
	runs := l.Runs()
	for i := range p.rubies {
		a, b := l.rubySpans[i], l.rubySpans[i+1]
		if a == b {
			continue
		}
		// Glyphs stand off their em box by the same amount, as the ruby was
		// shaped on its own.
		q := p.rubies[i].shaped
		last := runs[b-1]
		inline := func(r, g int) float32 {
			run := runs[r]
			if p.vertical {
				return run.Origin.Y + run.Glyphs[g].Y
			}
			return run.Origin.X + run.Glyphs[g].X
		}
		lo := inline(a, 0) - q.glyphs[0].inline
		hi := inline(b-1, len(last.Glyphs)-1) - q.glyphs[len(q.glyphs)-1].inline + rubyEm
		out = append(out, [2]float32{lo, hi})
	}
	return out
}

// Ruby longer than its base must not reach past the head or the end of its
// line, in either orientation (JLReq 3.3.6): at the head of a line it starts
// with its base, at the end it ends with it.
func TestRuby_StaysWithinTheLine(t *testing.T) {
	s := systemShaper(t)
	const em = 32
	const rubyEm = em * rubySize
	ruby := Span{Text: "考", Ruby: "かんがえ"}
	cases := []struct {
		name   string
		spans  []Span
		length float32
	}{
		{"alone", []Span{ruby}, inf},
		{"head of the paragraph", []Span{ruby, {Text: "えた"}}, inf},
		{"head of a later line", []Span{{Text: "ああ"}, ruby, {Text: "えた"}}, 2 * em},
		{"end of a line", []Span{{Text: "ああ"}, ruby, {Text: "えお"}}, 3 * em},
		{"head after kana, kanji after", []Span{{Text: "漢あ"}, ruby, {Text: "漢"}}, 2 * em},
		{"between kana", []Span{{Text: "あ"}, ruby, {Text: "い"}}, inf},
		{"justified", []Span{{Text: "ああ"}, ruby, {Text: "えおかきくけこ"}}, 3.3 * em},
	}
	for _, o := range []Orientation{Horizontal, Vertical} {
		for _, c := range cases {
			st := Style{Size: em, Orientation: o}
			if c.name == "justified" {
				st.Align = AlignJustify
			}
			l := s.ShapeSpans(c.spans, st).Layout(c.length)
			length := min(c.length, l.extent)
			for _, e := range rubyExtents(l, rubyEm) {
				if e[0] < -0.01 || e[1] > length+0.01 {
					t.Errorf("orientation %d, %s: ruby at %v..%v, line 0..%v", o, c.name, e[0], e[1], length)
				}
			}
		}
	}
}

// Ruby overhangs kana only, never a kanji beside it: the base opens up on
// that side instead (JLReq 3.3.6).
func TestRuby_OverhangsOnlyTheKanaSide(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	const rubyEm = em * rubySize
	l := s.ShapeSpans([]Span{{Text: "漢"}, {Text: "字", Ruby: "ああああ"}, {Text: "い"}}, Style{Size: em}).Layout(inf)
	e := rubyExtents(l, rubyEm)
	if len(e) != 1 {
		t.Fatalf("got %v", e)
	}
	// The kanji ends at 1 em, and the ruby of 2 em reaches half an em over い.
	if e[0][0] < em-0.01 {
		t.Errorf("ruby starts at %v, over the kanji that ends at %v", e[0][0], float32(em))
	}
	if w := l.Size().W; w != 3.5*em {
		t.Errorf("width %v, want %v", w, 3.5*em)
	}
}

// look is how a glyph is drawn: its face, and whether it is turned.
type look struct {
	face     uint32
	sideways bool
}

// baseLooks returns the look of every glyph of the runs of l at size em, which
// leaves the ruby out.
func baseLooks(l *Layout, em float32) []look {
	var out []look
	for _, r := range l.Runs() {
		if r.Size != em {
			continue
		}
		for range r.Glyphs {
			out = append(out, look{uint32(r.Face), r.Sideways})
		}
	}
	return out
}

// The characters of a group ruby keep the face they were shaped with and,
// in vertical text, their own orientation: Latin turns while kanji stand,
// and a character from a fallback font is drawn with that font.
func TestRuby_GroupKeepsTheLookOfEachCharacter(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	for _, o := range []Orientation{Horizontal, Vertical} {
		// A Latin family first, so that the kanji come from a fallback.
		st := Style{Size: em, Orientation: o, Family: "Segoe UI, DejaVu Sans, Helvetica"}
		want := baseLooks(s.Shape("AI時代", st).Layout(inf), em)
		got := baseLooks(s.ShapeSpans([]Span{{Text: "AI時代", Ruby: "えーあいじだい"}}, st).Layout(inf), em)
		if !slices.Equal(got, want) {
			t.Errorf("orientation %d: the base is drawn as %v, want %v", o, got, want)
		}
	}
}

// The space between Japanese and Western text after a group ruby follows the
// last character of its base, and the space before it the first (JLReq
// 3.2.5).
func TestRuby_GroupMeetsItsNeighborsByItsEnds(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	space := func(spans []Span) float32 {
		w := func(ws jlreq.WesternSpacing) float32 {
			return s.ShapeSpans(spans, Style{Size: em, WesternSpacing: ws}).Layout(inf).Size().W
		}
		return w(jlreq.WesternQuarterEm) - w(jlreq.WesternNone)
	}
	for _, c := range []struct {
		name  string
		spans []Span
		want  float32
	}{
		{"Latin then kanji, kana after", []Span{{Text: "AI時代", Ruby: "えーあいじだい"}, {Text: "は"}}, 0},
		{"kanji then Latin, kana after", []Span{{Text: "時代AI", Ruby: "じだいえーあい"}, {Text: "は"}}, em / 4},
		{"kana before Latin then kanji", []Span{{Text: "は"}, {Text: "AI時代", Ruby: "えーあいじだい"}}, em / 4},
		{"kana before kanji then Latin", []Span{{Text: "は"}, {Text: "時代AI", Ruby: "じだいえーあい"}}, 0},
	} {
		if got := space(c.spans); got < c.want-0.01 || got > c.want+0.01 {
			t.Errorf("%s: space %v, want %v", c.name, got, c.want)
		}
	}
}

// Ruby may overhang small kana and the prolonged sound mark as it does other
// kana (JLReq 3.3.8).
func TestRuby_OverhangsSmallKanaAndTheProlongedMark(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	for _, after := range []string{"い", "ょ", "ー"} {
		// Four ruby characters are 2 em over a base of 1 em, and reach half
		// an em over the character after it.
		l := s.ShapeSpans([]Span{{Text: "漢"}, {Text: "字", Ruby: "ああああ"}, {Text: after}}, Style{Size: em}).Layout(inf)
		if w := l.Size().W; w != 3.5*em {
			t.Errorf("before %s: width %v, want %v", after, w, 3.5*em)
		}
	}
}
