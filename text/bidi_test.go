package text

import (
	"image"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestReorder_L2(t *testing.T) {
	tests := []struct {
		name   string
		levels []uint8
		want   []int
	}{
		{"all LTR", []uint8{0, 0, 0}, []int{0, 1, 2}},
		{"all RTL", []uint8{1, 1, 1}, []int{2, 1, 0}},
		{"RTL inside LTR", []uint8{0, 1, 1, 0}, []int{0, 2, 1, 3}},
		// Arabic, digits, Arabic in an LTR line: the digits stay between
		// the words, and the words swap.
		{"numbers inside RTL", []uint8{0, 1, 2, 2, 1, 0}, []int{0, 4, 2, 3, 1, 5}},
	}
	for _, tt := range tests {
		got := make([]int, len(tt.levels))
		reorder(tt.levels, got)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// arabicShaper returns a system Shaper that has a font for Arabic, or skips.
func arabicShaper(t *testing.T) *Shaper {
	t.Helper()
	s := systemShaper(t)
	if _, ok := s.fonts.FindSystemFont("Segoe UI"); !ok {
		if _, ok := s.fonts.FindSystemFont("Noto Sans Arabic"); !ok {
			t.Skip("no font for Arabic")
		}
	}
	return s
}

// visual returns the text of line i of l in the order of the screen, one
// cluster at a time.
func visual(l *Layout, i int) []string {
	ln := &l.lines[i]
	p := l.paragraph
	idx := make([]int, 0, ln.clusterEnd-ln.clusterStart)
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		idx = append(idx, c)
	}
	sort.SliceStable(idx, func(a, b int) bool { return l.xs[idx[a]] < l.xs[idx[b]] })
	out := make([]string, len(idx))
	for k, c := range idx {
		out[k] = p.text[p.clusters[c].start:p.clusters[c].end]
	}
	return out
}

func TestBidi_ArabicRunsRightToLeft(t *testing.T) {
	s := arabicShaper(t)
	const content = "ابت 12"
	l := s.Shape(content, Style{Size: 16}).Layout(400)
	if l.Direction() != DirectionRTL {
		t.Fatal("a paragraph that starts in Arabic runs right to left")
	}
	// On screen: the number keeps its order, left of the word, whose letters
	// run from the right.
	want := []string{"1", "2", " ", "ت", "ب", "ا"}
	if got := visual(l, 0); !slices.Equal(got, want) {
		t.Errorf("visual order %q, want %q", got, want)
	}
	// The line starts at the right edge of the room.
	if got := l.Size().W; got != 400 {
		t.Errorf("width %v, want the full 400", got)
	}
	// The caret at the start is on the right, at the end on the left.
	if start, end := l.CaretRect(0).X, l.CaretRect(len(content)).X; start <= end {
		t.Errorf("caret at the start %v must be right of the end %v", start, end)
	}
}

func TestBidi_LTRParagraphKeepsEmbeddedArabicTogether(t *testing.T) {
	s := arabicShaper(t)
	const content = "ab ابت cd"
	l := s.Shape(content, Style{Size: 16}).Layout(geom.Inf)
	want := []string{"a", "b", " ", "ت", "ب", "ا", " ", "c", "d"}
	if got := visual(l, 0); !slices.Equal(got, want) {
		t.Errorf("visual order %q, want %q", got, want)
	}
	if l.Direction() != DirectionLTR {
		t.Error("a paragraph that starts in Latin runs left to right")
	}
}

func TestBidi_VisualMove(t *testing.T) {
	s := arabicShaper(t)
	const content = "ab ابت cd"
	l := s.Shape(content, Style{Size: 16}).Layout(geom.Inf)
	// Walk right from the start: every step must move the caret right on
	// screen until the end of the line.
	at := 0
	x := l.CaretRect(at).X
	for step := 0; step < 20; step++ {
		next := l.VisualMove(at, 1)
		if next == at || next >= len(content) && at >= len(content) {
			break
		}
		nx := l.CaretRect(next).X
		if nx < x {
			t.Fatalf("step %d: right went from %d (x=%v) to %d (x=%v)", step, at, x, next, nx)
		}
		at, x = next, nx
	}
	if at != len(content) {
		t.Errorf("walking right ended at %d, want %d", at, len(content))
	}
	// Hit testing a point gives back an offset whose caret is near it.
	for _, px := range []float32{2, 30, 45, 60} {
		off := l.HitTest(geom.Offset{X: px, Y: 5})
		if d := geom.Abs32(l.CaretRect(off).X - px); d > 12 {
			t.Errorf("hit at %v gave %d, caret %v away", px, off, d)
		}
	}
}

func TestBidi_SelectionSplitsOnScreen(t *testing.T) {
	s := arabicShaper(t)
	const content = "ab ابت cd"
	l := s.Shape(content, Style{Size: 16}).Layout(geom.Inf)
	// "b ا" in logical order is two pieces on screen: "b " on the left and
	// "ا" at the right end of the Arabic word.
	start := len("a")
	end := len("ab ا")
	rects := l.SelectionRects(start, end, nil)
	if len(rects) != 2 {
		t.Errorf("got %d rectangles, want 2: %v", len(rects), rects)
	}
}

func TestVariableFont_WeightChangesTheGlyphs(t *testing.T) {
	s := systemShaper(t)
	if _, ok := s.fonts.FindSystemFont("Bahnschrift"); !ok {
		t.Skip("no Bahnschrift")
	}
	width := func(st Style) float32 { return s.Shape("Variable", st).Layout(geom.Inf).Size().W }
	// ink sums the coverage of an I, which only its stem makes.
	a := NewAtlas(s)
	ink := func(st Style) int {
		p := s.Shape("I", st)
		img := a.Glyph(GlyphKey{Face: p.clusters[0].face, ID: p.glyphs[0].id, Size: st.Size})
		_, pix := a.TakeDirty(img.Page)
		return inkOf(pix, a.size, image.Rect(img.X, img.Y, img.X+img.Width, img.Y+img.Height))
	}
	lightStyle := Style{Family: "Bahnschrift", Size: 40, Weight: 300}
	boldStyle := Style{Family: "Bahnschrift", Size: 40, Weight: 700}
	if l, b := ink(lightStyle), ink(boldStyle); b < l*3/2 {
		t.Errorf("bold ink %d must well exceed light ink %d", b, l)
	}
	light := width(lightStyle)
	narrow := width(Style{Family: "Bahnschrift", Size: 40, Weight: 300, Variations: [4]Variation{{Axis: "wdth", Value: 75}}})
	if narrow >= light*0.95 {
		t.Errorf("wdth 75 (%v) must be narrower than 100 (%v)", narrow, light)
	}
}

func TestHyphenate_BreaksLongWords(t *testing.T) {
	s := goShaper(t)
	const content = "Typesetting international responsibility"
	st := Style{Size: 16}
	plain := s.Shape(content, st).Layout(130)
	st.Hyphenate = true
	hy := s.Shape(content, st).Layout(130)
	a, _ := hy.LineRange(0)
	_, b := hy.LineRange(0)
	_, pb := plain.LineRange(0)
	if b <= pb {
		t.Fatalf("the first line must take more with hyphens: %q vs %q", content[a:b], content[:pb])
	}
	// The hyphen is drawn at the end of the line, after the last cluster.
	runs := hy.Runs()
	if len(runs) < 2 {
		t.Fatalf("got %d runs", len(runs))
	}
	if !hy.lines[0].hyphen {
		t.Error("the first line must end with a hyphen")
	}
}

// TestBidi_MatchesChrome sets testdata/bidi_corpus.txt and compares the order
// of the characters on screen with what Chrome draws for the same lines in
// <p dir="auto"> (testdata/bidi_chrome.txt, read back from Chrome).
func TestBidi_MatchesChrome(t *testing.T) {
	s := arabicShaper(t)
	corpus, err := os.ReadFile("testdata/bidi_corpus.txt")
	if err != nil {
		t.Fatal(err)
	}
	chrome, err := os.ReadFile("testdata/bidi_chrome.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(corpus), "\r", "")), "\n")
	wants := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(chrome), "\r", "")), "\n")
	for i, content := range lines {
		l := s.Shape(content, Style{Family: "Segoe UI", Size: 16}).Layout(geom.Inf)
		var got []string
		for _, c := range visual(l, 0) {
			// Chrome boxes each character of a ligature apart: right to
			// left, the letters of a cluster run backwards.
			rs := []rune(c)
			if len(rs) > 1 && isRTLText(rs[0]) {
				slices.Reverse(rs)
			}
			for _, r := range rs {
				got = append(got, string(r))
			}
		}
		if g := strings.Join(got, "|"); g != wants[i] {
			t.Errorf("line %d:\n got %s\nwant %s", i+1, g, wants[i])
		}
	}
}

func isRTLText(r rune) bool { return r >= 0x0590 && r <= 0x08FF }

func TestBidi_VisualMovePassesEveryCaretOnce(t *testing.T) {
	s := arabicShaper(t)
	for _, content := range []string{"abc אב", "אב abc", "ab ابت cd", "אב", "abc אב def גד", "אב abc גד", "12 אב"} {
		l := s.Shape(content, Style{Size: 16}).Layout(geom.Inf)
		if !l.lines[0].bidi {
			t.Skipf("%q: no font for Hebrew", content)
		}
		// Go to the left end, which may take as many steps as there are carets.
		at := 0
		for range len(content) + 1 {
			at = l.VisualMove(at, -1)
		}
		if l.VisualMove(at, -1) != at {
			t.Errorf("%q: left never comes to rest", content)
			continue
		}
		right := []int{at}
		x := l.CaretRect(at).X
		for range len(content) + 1 {
			next := l.VisualMove(at, 1)
			if next == at {
				break
			}
			if slices.Contains(right, next) {
				t.Fatalf("%q: right came back to %d after %v", content, next, right)
			}
			if nx := l.CaretRect(next).X; nx < x {
				t.Errorf("%q: right went left on screen, from %d to %d", content, at, next)
			} else {
				x = nx
			}
			right = append(right, next)
			at = next
		}
		if want := len(l.paragraph.clusters) + 1; len(right) != want {
			t.Errorf("%q: right passed %v, want all %d carets", content, right, want)
		}
		left := []int{at}
		for range len(content) + 1 {
			next := l.VisualMove(at, -1)
			if next == at {
				break
			}
			left = append(left, next)
			at = next
		}
		slices.Reverse(left)
		if !slices.Equal(left, right) {
			t.Errorf("%q: left passed %v backwards, right %v", content, left, right)
		}
	}
}
