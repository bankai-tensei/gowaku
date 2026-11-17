package text

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// rubyDoc is text with ruby that an editor changes: plain pieces and pieces
// with ruby, which may show as their raw notation instead.
type rubyDoc struct {
	pieces []rubyPiece
}

type rubyPiece struct {
	text, ruby string
	raw        bool
}

// spans returns the spans of the document as an editor would make them:
// neighboring plain text in one span, and empty plain text left out.
func (d *rubyDoc) spans() []Span {
	var out []Span
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, Span{Text: plain.String()})
			plain.Reset()
		}
	}
	for _, p := range d.pieces {
		switch {
		case p.ruby == "":
			plain.WriteString(p.text)
		case p.raw:
			plain.WriteString("｜" + p.text + "《" + p.ruby + "》")
		default:
			flush()
			out = append(out, Span{Text: p.text, Ruby: p.ruby})
		}
	}
	flush()
	if len(out) == 0 {
		out = append(out, Span{})
	}
	return out
}

// edit changes the document in one place at random.
func (d *rubyDoc) edit(rng *rand.Rand) {
	words := []string{"x", "あ", "\n", "word ", "。", "漢字", ""}
	i := rng.IntN(len(d.pieces) + 1)
	switch rng.IntN(5) {
	case 0: // a new piece of plain text
		d.pieces = insertPiece(d.pieces, i, rubyPiece{text: words[rng.IntN(len(words))]})
	case 1: // a new piece with ruby
		d.pieces = insertPiece(d.pieces, i, rubyPiece{text: "漢字", ruby: "かんじ", raw: rng.IntN(2) == 0})
	case 2: // the caret enters or leaves ruby
		if i < len(d.pieces) && d.pieces[i].ruby != "" {
			d.pieces[i].raw = !d.pieces[i].raw
		}
	case 3: // a piece goes
		if i < len(d.pieces) {
			d.pieces = append(d.pieces[:i], d.pieces[i+1:]...)
		}
	case 4: // typing into plain text or ruby
		if i < len(d.pieces) {
			if d.pieces[i].ruby != "" {
				d.pieces[i].ruby += "る"
			} else {
				d.pieces[i].text += words[rng.IntN(len(words))]
			}
		}
	}
}

func insertPiece(s []rubyPiece, i int, p rubyPiece) []rubyPiece {
	s = append(s, rubyPiece{})
	copy(s[i+1:], s[i:])
	s[i] = p
	return s
}

func TestReshapeSpans_MatchesAFreshLayout(t *testing.T) {
	for _, shaper := range []struct {
		name string
		s    func(testing.TB) *Shaper
	}{
		{"go", goShaper},
		{"system", systemShaper},
	} {
		for _, st := range []Style{
			{Size: 16},
			{Size: 16, RubyRoom: true},
			{Size: 16, Orientation: Vertical},
			{Size: 16, Orientation: Vertical, RubyRoom: true},
			{Size: 16, Align: AlignCenter},
		} {
			t.Run(shaper.name+"/"+st.Orientation.String(), func(t *testing.T) {
				s := shaper.s(t)
				doc := &rubyDoc{pieces: []rubyPiece{
					{text: "親譲りの"}, {text: "無鉄砲", ruby: "むてっぽう"}, {text: "で小供の時から\n損ばかり"},
					{text: "漢", ruby: "かん"}, {text: "している。\n\n"}, {text: "学校", ruby: "がっこう", raw: true}, {text: "end"},
				}}
				p := s.ShapeSpans(doc.spans(), st)
				l := p.Layout(120)
				rng := rand.New(rand.NewPCG(3, 4))
				for step := range 300 {
					doc.edit(rng)
					spans := doc.spans()
					p = s.ReshapeSpans(p, spans, st)
					l = p.Relayout(l, 120)
					want := s.ShapeSpans(spans, st).Layout(120)
					if diff := sameLayout(l, want); diff != "" {
						t.Fatalf("step %d, %q: %s", step, p.Text(), diff)
					}
				}
			})
		}
	}
}

func TestReshapeSpans_ReusesTheParagraph(t *testing.T) {
	s := goShaper(t)
	st := Style{Size: 10, RubyRoom: true}
	p := s.ShapeSpans([]Span{{Text: "ab\n"}, {Text: "cd", Ruby: "x"}, {Text: "\nef"}}, st)
	q := s.ReshapeSpans(p, []Span{{Text: "ab\n"}, {Text: "cd", Ruby: "x"}, {Text: "\nefg"}}, st)
	if q != p {
		t.Fatal("an edit after the ruby must reshape in place")
	}
	if q.edit.first != 2 || q.edit.oldLast != 2 {
		t.Errorf("edit %+v, want only the last block", q.edit)
	}
	// The ruby goes, and with RubyRoom the lines keep their height.
	r := s.ReshapeSpans(q, []Span{{Text: "ab\ncd\nefg"}}, st)
	if r != q || r.lineHeight != RubyLineHeight {
		t.Errorf("ruby room must stay: %v", r.lineHeight)
	}
	// Without it, the last ruby to go changes the lines, which shapes anew.
	st.RubyRoom = false
	p = s.ShapeSpans([]Span{{Text: "ab\n"}, {Text: "cd", Ruby: "x"}}, st)
	if q := s.ReshapeSpans(p, []Span{{Text: "ab\ncd"}}, st); q == p || q.lineHeight == RubyLineHeight {
		t.Error("the last ruby to go must shape anew")
	}
}

func BenchmarkKeystroke_100kCharsRuby(b *testing.B) {
	s := systemShaper(b)
	content := bigText()
	// A ruby group every hundred characters or so.
	var spans []Span
	for _, line := range strings.SplitAfter(content, "\n") {
		if line == "" {
			continue
		}
		i := strings.Index(line, "無鉄砲")
		spans = append(spans, Span{Text: line[:i]}, Span{Text: "無鉄砲", Ruby: "むてっぽう"}, Span{Text: line[i+len("無鉄砲"):]})
	}
	st := Style{Size: 16}
	p := s.ShapeSpans(spans, st)
	l := p.Layout(600)
	mid := len(spans) / 2 / 3 * 3
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		spans[mid].Text = string(rune('a'+i%26)) + spans[mid].Text
		p = s.ReshapeSpans(p, spans, st)
		l = p.Relayout(l, 600)
	}
	b.ReportMetric(float64(len(spans)/3), "rubies")
}
