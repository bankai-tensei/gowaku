package text

import (
	"slices"
	"testing"
)

// placedMark is where the ink of an emphasis mark was drawn: its middle along
// the line, and across it (y in horizontal text, x in vertical text).
type placedMark struct {
	center, cross float32
}

// placedMarks returns the emphasis marks of l in order. Every mark is one
// glyph of the mark of its section.
func placedMarks(l *Layout) []placedMark {
	p := l.paragraph
	runs := l.Runs()
	var out []placedMark
	k := 0
	for r := l.rubySpans[len(l.rubySpans)-1]; r < len(runs); r++ {
		run := runs[r]
		for _, g := range run.Glyphs {
			for k < len(p.marks) && l.lineOfCluster(p.marks[k].cluster) == nil {
				k++
			}
			mk := p.sections[p.clusters[p.marks[k].cluster].section].mark
			sg := mk.shaped.glyphs[0]
			if p.vertical {
				out = append(out, placedMark{run.Origin.Y + g.Y - sg.inline + mk.inline, run.Origin.X + g.X - sg.cross + mk.cross})
			} else {
				out = append(out, placedMark{run.Origin.X + g.X - sg.inline + mk.inline, run.Origin.Y + g.Y - sg.cross + mk.cross})
			}
			k++
		}
	}
	return out
}

// Every character but punctuation and spaces gets one mark, centered on it,
// above it in horizontal text and right of it in vertical text (JLReq).
func TestEmphasis_OneMarkPerCharacterCentered(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	spans := []Span{{Text: "前"}, {Text: "強、調　い。", Emphasis: EmphasisSesame}}
	for _, o := range []Orientation{Horizontal, Vertical} {
		l := s.ShapeSpans(spans, Style{Size: em, Orientation: o}).Layout(inf)
		got := placedMarks(l)
		if len(got) != 3 {
			t.Fatalf("%v: %d marks, want 3 (強, 調, い): %v", o, len(got), got)
		}
		for i, c := range []int{1, 3, 5} { // 強, 調, い are clusters 1, 3 and 5
			want := l.xs[c] + l.adv[c]/2
			if !near(got[i].center, want) {
				t.Errorf("%v: mark %d at %v, want the middle of its character %v", o, i, got[i].center, want)
			}
		}
		ln := &l.lines[0]
		origin := l.lineOrigin(ln)
		if o == Vertical {
			// Right of the column: the ink is centered half an em plus a
			// quarter em right of the center of the base.
			if want := origin.X + 0.75*em; !near(got[0].cross, want) {
				t.Errorf("vertical: marks at x %v, want %v", got[0].cross, want)
			}
		} else if want := origin.Y - (l.paragraph.emCenter+0.75)*em; !near(got[0].cross, want) {
			t.Errorf("horizontal: marks at y %v, want %v", got[0].cross, want)
		}
	}
}

// Text with emphasis makes room between its lines as ruby does.
func TestEmphasis_TakesTheRoomOfRuby(t *testing.T) {
	s := systemShaper(t)
	plain := s.ShapeSpans([]Span{{Text: "強調"}}, Style{Size: 20})
	marked := s.ShapeSpans([]Span{{Text: "強調", Emphasis: EmphasisCircle}}, Style{Size: 20})
	ruby := s.ShapeSpans([]Span{{Text: "強調", Ruby: "きょうちょう"}}, Style{Size: 20})
	if plain.lineHeight != defaultLineHeight {
		t.Errorf("plain line height %v", plain.lineHeight)
	}
	if marked.lineHeight != RubyLineHeight || marked.topRoom != ruby.topRoom {
		t.Errorf("marks: line height %v and room %v, want those of ruby, %v and %v",
			marked.lineHeight, marked.topRoom, ruby.lineHeight, ruby.topRoom)
	}
}

// Marks on characters with ruby stand outside the ruby, one per base
// character over the base as the ruby spreads it, and the lines open up for
// both.
func TestEmphasis_OutsideRuby(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	for _, o := range []Orientation{Horizontal, Vertical} {
		st := Style{Size: em, Orientation: o}
		// The ruby is shorter than its base and leaves the base as it is.
		p := s.ShapeSpans([]Span{{Text: "漢字", Ruby: "かん", Emphasis: EmphasisSesame}}, st)
		if want := float32(RubyLineHeight + rubySize); p.lineHeight != want {
			t.Errorf("%v: line height %v, want %v", o, p.lineHeight, want)
		}
		l := p.Layout(inf)
		got := placedMarks(l)
		if len(got) != 2 || !near(got[0].center, em/2) || !near(got[1].center, 1.5*em) {
			t.Errorf("%v: marks %v, want at %v and %v", o, got, em/2, 1.5*em)
		}
		origin := l.lineOrigin(&l.lines[0])
		ruby := l.rubyOrigin(&p.rubies[0], &l.lines[0])
		if o == Vertical {
			// The ruby column is centered 0.75 em right of the base.
			if want := ruby.X + rubySize*em; !near(got[0].cross, want) {
				t.Errorf("vertical: marks at x %v, want %v, right of the ruby at %v", got[0].cross, want, ruby.X)
			}
		} else if want := origin.Y - (p.emCenter+1.25)*em; !near(got[0].cross, want) {
			t.Errorf("horizontal: marks at y %v, want %v, above the ruby at %v", got[0].cross, want, ruby.Y)
		}

		// Ruby longer than its base spreads the base 1:2:1, and the marks
		// follow the characters.
		l = s.ShapeSpans([]Span{{Text: "漢字", Ruby: "かんじかんじ", Emphasis: EmphasisSesame}}, st).Layout(inf)
		got = placedMarks(l)
		// 3 em of ruby over 2 em of base: each character takes 1.5 em.
		if len(got) != 2 || !near(got[0].center, 0.75*em) || !near(got[1].center, 2.25*em) {
			t.Errorf("%v: marks over a spread base %v, want at %v and %v", o, got, 0.75*em, 2.25*em)
		}
	}
}

// Ruby does not reach over a kana that has marks of its own.
func TestEmphasis_RubyDoesNotOverhangMarks(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	ruby := Span{Text: "字", Ruby: "ああああ"}
	free := s.ShapeSpans([]Span{ruby, {Text: "い"}}, Style{Size: em}).Layout(inf)
	marked := s.ShapeSpans([]Span{ruby, {Text: "い", Emphasis: EmphasisSesame}}, Style{Size: em}).Layout(inf)
	if w := free.Size().W; w != 2.5*em {
		t.Errorf("width %v, want %v with the ruby over い", w, 2.5*em)
	}
	if w := marked.Size().W; w != 3*em {
		t.Errorf("width %v, want %v with the base opened up", w, 3*em)
	}
}

// Marks follow their characters onto later lines and out of text cut short.
func TestEmphasis_LinesAndTruncation(t *testing.T) {
	s := systemShaper(t)
	const em = 20
	spans := []Span{{Text: "強調強調強調", Emphasis: EmphasisDot}}
	l := s.ShapeSpans(spans, Style{Size: em}).Layout(2 * em)
	got := placedMarks(l)
	if len(got) != 6 {
		t.Fatalf("%d marks, want 6", len(got))
	}
	for i, m := range got {
		if want := float32(i%2)*em + em/2; !near(m.center, want) {
			t.Errorf("mark %d at %v, want %v", i, m.center, want)
		}
	}
	if got[0].cross == got[2].cross {
		t.Error("the marks of the second line must stand over the second line")
	}
	l = s.ShapeSpans(spans, Style{Size: em, MaxLines: 1}).Layout(3 * em)
	if got := placedMarks(l); len(got) != 2 {
		t.Errorf("%d marks on a line cut to two characters and an ellipsis, want 2", len(got))
	}
}

// Reshape and Relayout keep the marks as ShapeSpans and Layout place them.
func TestEmphasis_Incremental(t *testing.T) {
	s := systemShaper(t)
	st := Style{Size: 20}
	before := []Span{{Text: "前の行\n"}, {Text: "強調", Emphasis: EmphasisSesame}, {Text: "する\n後の"}, {Text: "行", Ruby: "ぎょう", Emphasis: EmphasisCircle}}
	after := []Span{{Text: "前の行\n"}, {Text: "強調", Emphasis: EmphasisSesame}, {Text: "してみる\n後の"}, {Text: "行", Ruby: "ぎょう", Emphasis: EmphasisCircle}}
	old := s.ShapeSpans(before, st)
	oldLayout := old.Layout(200)
	p := s.ReshapeSpans(old, after, st)
	if p != old {
		t.Fatal("the edit should reshape in place")
	}
	l := p.Relayout(oldLayout, 200)
	if l != oldLayout {
		t.Fatal("the edit should lay out in place")
	}
	fresh := s.ShapeSpans(after, st)
	if !slices.Equal(p.marks, fresh.marks) {
		t.Errorf("marks %v, want %v", p.marks, fresh.marks)
	}
	if got, want := placedMarks(l), placedMarks(fresh.Layout(200)); !slices.Equal(got, want) {
		t.Errorf("relaid out marks %v, want %v", got, want)
	}

	// The first marks over ruby make the lines taller: the paragraph is
	// shaped anew.
	plain := s.ShapeSpans([]Span{{Text: "前\n"}, {Text: "行", Ruby: "ぎょう"}}, st)
	stacked := []Span{{Text: "前\n"}, {Text: "行", Ruby: "ぎょう", Emphasis: EmphasisSesame}}
	if q := s.ReshapeSpans(plain, stacked, st); q.lineHeight != float32(RubyLineHeight+rubySize) {
		t.Errorf("line height %v after marks came over ruby", q.lineHeight)
	}
}

func TestEmphasis_String(t *testing.T) {
	if EmphasisSesame.String() != "Sesame" || EmphasisNone.mark() != "" || EmphasisSesame.mark() != "﹅" {
		t.Error("names or marks are wrong")
	}
}
