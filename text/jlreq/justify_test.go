package jlreq

import (
	"testing"
)

// positions breaks s at the width and returns the breaker.
func breakAt(s string, widthInEm float32, opts Options) (*Breaker, []Unit) {
	us := units(s)
	var b Breaker
	b.Break(us, widthInEm*em, opts)
	return &b, us
}

func near(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }

func TestJLReq_JustifyJapanese(t *testing.T) {
	// 7 characters on lines of 5.5 em: the first line has 5 em of text and
	// half an em spread over its 4 gaps.
	b, _ := breakAt("あいうえおかき", 5.5, Options{Justify: true})
	if len(b.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(b.Lines))
	}
	if got := b.Lines[0].Width; !near(got, 55) {
		t.Errorf("width of the first line = %v, want 55", got)
	}
	for k, want := range []float32{0, 11.25, 22.5, 33.75, 45} {
		if got := b.X[k]; !near(got, want) {
			t.Errorf("X[%d] = %v, want %v", k, got, want)
		}
	}
	// The last line keeps its natural width.
	if got := b.Lines[1].Width; !near(got, 20) {
		t.Errorf("width of the last line = %v, want 20", got)
	}
}

func TestJLReq_JustifyLeavesBracketsClosed(t *testing.T) {
	// No gap opens after 「 or before 」 and 。.
	b, _ := breakAt("「あい」う。えおかきく", 6, Options{Justify: true, Punctuation: PunctuationFull})
	ln := b.Lines[0]
	if ln.End != 6 {
		t.Fatalf("first line ends at %d, want 6", ln.End)
	}
	if got := b.X[1] - b.X[0]; !near(got, em) {
		t.Errorf("gap after 「 opened: %v", got-em)
	}
	if got := b.X[3] - b.X[2]; !near(got, em) {
		t.Errorf("gap before 」 opened: %v", got-em)
	}
	if got := b.X[5] - b.X[4]; !near(got, em) {
		t.Errorf("gap before 。 opened: %v", got-em)
	}
}

func TestJLReq_JustifyWordSpacesFirst(t *testing.T) {
	// "ab cd" plus a kanji after a quarter em: the word space and the Japanese
	// boundary take a quarter em each before any kanji gap opens.
	opts := Options{Justify: true}
	b, us := breakAt("ab cd漢字字字字字字", 6, opts)
	ln := b.Lines[0]
	extra := 6*em - naturalLine(us, ln)
	if extra <= 0 {
		t.Fatalf("no room to justify")
	}
	if got := b.Lines[0].Width; !near(got, 6*em) {
		t.Errorf("width = %v, want %v", got, 6*em)
	}
	spaceGrow := (b.X[3] - b.X[2]) - us[2].Advance
	if !near(spaceGrow, min(extra, em/2)*0.5) {
		t.Errorf("word space grew by %v", spaceGrow)
	}
}

// naturalLine is the width of ln before justification: the position of its
// last unit with no gap opened.
func naturalLine(us []Unit, ln Line) float32 {
	var b2 Breaker
	b2.Break(us[ln.Start:ln.End], 1e9, Options{})
	return b2.Lines[0].Width
}

func TestJLReq_JustifyNoGaps(t *testing.T) {
	b, _ := breakAt("abcdefghijkl", 4, Options{Justify: true})
	for k := range 8 {
		if got := b.X[k]; !near(got, float32(k)*em/2) {
			t.Errorf("X[%d] = %v, want %v", k, got, float32(k)*em/2)
		}
	}
}

func TestJLReq_Indent(t *testing.T) {
	opts := Options{Indent: em, HangingIndent: 2 * em}
	b, _ := breakAt("あいうえおかきくけ", 4, opts)
	got := make([]int, 0, len(b.Lines))
	for _, l := range b.Lines {
		got = append(got, l.End-l.Start)
	}
	if want := []int{3, 2, 2, 2}; !equalInts(got, want) {
		t.Errorf("line lengths = %v, want %v", got, want)
	}
	if b.X[0] != em || b.X[3] != 2*em {
		t.Errorf("X = %v, %v; want the indents", b.X[0], b.X[3])
	}
	if got := b.Lines[0].Width; got != 4*em {
		t.Errorf("width of the first line = %v, want it to include the indent", got)
	}
}

func TestJLReq_TabStops(t *testing.T) {
	us := units("a\tbc\td")
	us[1].Tab, us[4].Tab = true, true
	var b Breaker
	b.Break(us, 100*em, Options{TabStop: 4 * em})
	// a at 0, tab to 4em, b c at 4 and 4.5em, tab to 8em, d at 8em.
	for k, want := range []float32{0, 5, 40, 45, 50, 80} {
		if got := b.X[k]; !near(got, want) {
			t.Errorf("X[%d] = %v, want %v", k, got, want)
		}
	}
	if got := b.Advance[4]; !near(got, 30) {
		t.Errorf("advance of the second tab = %v, want 30", got)
	}
}

func TestJLReq_IdeographicUnit(t *testing.T) {
	// Digits set as one ideographic unit get no Western spacing.
	us := units("あ12い")
	us = append(us[:1:1], Unit{Rune: '1', Em: em, Advance: em, FullWidth: true, Ideographic: true}, us[3])
	var b Breaker
	b.Break(us, 100*em, Options{})
	if got := b.X[2]; got != 2*em {
		t.Errorf("X[2] = %v, want %v", got, 2*em)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestJLReq_HyphenBreaksAWord(t *testing.T) {
	// "ab cdefgh" at 3.5 em: "ab " fits, and "cd" plus a hyphen fits after it.
	us := units("ab cdefgh")
	us[4].Hyphen = em / 2 // a break may follow "cd"
	var b Breaker
	b.Break(us, 3.5*em, Options{})
	if len(b.Lines) < 2 || b.Lines[0].End != 5 || !b.Lines[0].Hyphen {
		t.Fatalf("lines %+v, want the first to end after cd with a hyphen", b.Lines)
	}
	if got := b.Lines[0].Width; got != 3*em {
		t.Errorf("width %v, want %v with the hyphen", got, 3*em)
	}
	// Without room for the hyphen, the word moves down whole.
	b.Break(us, 2.9*em, Options{})
	if b.Lines[0].Hyphen {
		t.Errorf("no room for the hyphen: %+v", b.Lines[0])
	}
}

func TestJLReq_JustifyKeepsWesternPunctuation(t *testing.T) {
	// "ab, cd" spread: only the space opens, not the gap before the comma.
	us := units("ab, cdefghij")
	var b Breaker
	b.Break(us, 4*em, Options{Justify: true})
	if got := b.X[2] - b.X[1]; got != us[1].Advance {
		t.Errorf("the comma moved away from its word by %v", got-us[1].Advance)
	}
}
