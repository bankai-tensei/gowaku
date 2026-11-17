package text

import "testing"

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
