package text

import (
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

var yellow = geom.Color{R: 1, G: 0.9, B: 0.2, A: 1}

func TestBackground_CoversTheSpanOnItsLine(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	l := s.ShapeSpans([]Span{{Text: "ab"}, {Text: "cd", Background: yellow}, {Text: "ef"}}, Style{Size: em}).Layout(inf)
	got := l.Backgrounds()
	if len(got) != 1 {
		t.Fatalf("%d boxes, want 1: %v", len(got), got)
	}
	b := got[0]
	baseline := l.lineOrigin(&l.lines[0]).Y
	p := l.paragraph
	want := geom.Rect{X: l.xs[2], Y: baseline - (p.emCenter+0.5)*em, W: l.adv[2] + l.adv[3]}
	if b.Color != yellow || !near(b.Rect.X, want.X) || !near(b.Rect.W, want.W) || !near(b.Rect.Y, want.Y) {
		t.Errorf("box %+v, want %+v from the em box top", b, want)
	}
	if bottom := b.Rect.Y + b.Rect.H; bottom < baseline+(0.5-p.emCenter)*em-1e-3 || bottom > l.lines[0].top+l.lines[0].height {
		t.Errorf("the box ends at %v, want between the em box bottom and the end of the line", bottom)
	}
	if plain := s.Shape("abcdef", Style{Size: em}).Layout(inf); len(plain.Backgrounds()) != 0 {
		t.Errorf("text without a background has boxes: %v", plain.Backgrounds())
	}
}

func TestBackground_OneBoxPerLine(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	word := strings.Repeat("あ", 6)
	for _, o := range []Orientation{Horizontal, Vertical} {
		l := s.ShapeSpans([]Span{{Text: "前"}, {Text: word, Background: yellow}}, Style{Size: em, Orientation: o}).Layout(4 * em)
		got := l.Backgrounds()
		if l.LineCount() != 2 || len(got) != 2 {
			t.Fatalf("%v: %d lines and %d boxes, want 2 and 2: %v", o, l.LineCount(), len(got), got)
		}
		first, second := got[0].Rect, got[1].Rect
		switch o {
		case Horizontal:
			if !near(first.W, 3*em) || !near(second.W, 3*em) || second.Y <= first.Y+first.H {
				t.Errorf("%v: boxes %v and %v, want 3 em, then 3 em on the next line, apart", o, first, second)
			}
		case Vertical:
			center := l.lineOrigin(&l.lines[0]).X
			if !near(first.X, center-em/2) || !near(first.W, em) || !near(first.Y, em) || !near(first.H, 3*em) {
				t.Errorf("%v: first box %v, want the em column at %v from 1 em down, 3 em long", o, first, center)
			}
			if !near(second.H, 3*em) || second.X+second.W >= first.X {
				t.Errorf("%v: second box %v, want 3 em long, in the column left of %v", o, second, first)
			}
		}
	}
}

// The ruby stands above the box, as above a highlight of the CSS mark element.
func TestBackground_LeavesTheRubyOut(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	for _, o := range []Orientation{Horizontal, Vertical} {
		l := s.ShapeSpans([]Span{{Text: "漢字", Ruby: "かんじ", Background: yellow}}, Style{Size: em, Orientation: o}).Layout(inf)
		got := l.Backgrounds()
		if len(got) != 1 {
			t.Fatalf("%v: %d boxes, want 1", o, len(got))
		}
		b := got[0].Rect
		ruby := l.Runs()[l.rubySpans[0]]
		if o == Horizontal && b.Y < ruby.Origin.Y {
			t.Errorf("%v: the box %v reaches above the baseline of the ruby %v", o, b, ruby.Origin.Y)
		}
		if o == Vertical && b.X+b.W > ruby.Origin.X {
			t.Errorf("%v: the box %v reaches right of the center of the ruby %v", o, b, ruby.Origin.X)
		}
	}
}

func TestBackground_RightToLeftMakesOneBox(t *testing.T) {
	s := systemShaper(t)
	l := s.ShapeSpans([]Span{{Text: "שלום ", Background: yellow}, {Text: "עולם"}}, Style{Size: 20}).Layout(inf)
	got := l.Backgrounds()
	if len(got) != 1 {
		t.Fatalf("%d boxes, want 1: %v", len(got), got)
	}
	// The first word of a right-to-left line is on the right.
	if right := got[0].Rect.X + got[0].Rect.W; !near(right, l.Size().W) {
		t.Errorf("the box ends at %v, want the right end %v", right, l.Size().W)
	}
}

func TestBackground_Relayout(t *testing.T) {
	s := systemShaper(t)
	st := Style{Size: 20}
	spans := []Span{{Text: "一行目\n"}, {Text: "強調", Background: yellow}, {Text: "の行\n三行目"}}
	p := s.ShapeSpans(spans, st)
	l := p.Layout(200)
	spans[0].Text = "一行目を長く\n"
	p = s.ReshapeSpans(p, spans, st)
	l = p.Relayout(l, 200)
	want := s.ShapeSpans(spans, st).Layout(200).Backgrounds()
	if got := l.Backgrounds(); !slices.Equal(got, want) {
		t.Errorf("after Relayout the boxes are %v, want %v", got, want)
	}
	// A change of the background alone is an edit too.
	spans[1].Background = geom.Color{B: 1, A: 1}
	p = s.ReshapeSpans(p, spans, st)
	l = p.Relayout(l, 200)
	if got := l.Backgrounds(); len(got) != 1 || got[0].Color != spans[1].Background {
		t.Errorf("after a new background the boxes are %v", got)
	}
}
