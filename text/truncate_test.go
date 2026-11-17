package text

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// ellipsisGlyph returns the first glyph of the ellipsis in l and its inline
// position, or false.
func ellipsisGlyph(l *Layout) (inline float32, ok bool) {
	for i := range l.lines {
		ln := &l.lines[i]
		if ln.ellipsis {
			return ln.x + ln.ellipsisAt, true
		}
	}
	return 0, false
}

// checkTruncated checks what every truncated layout must hold: the number of
// lines, an ellipsis on the last one, and no line or glyph past the length.
func checkTruncated(t *testing.T, l *Layout, lines int, length float32) {
	t.Helper()
	if got := l.LineCount(); got != lines {
		t.Errorf("%d lines, want %d", got, lines)
	}
	if !l.Truncated() {
		t.Error("Truncated is false")
	}
	if _, ok := ellipsisGlyph(l); !ok {
		t.Error("no ellipsis")
	} else if !l.lines[len(l.lines)-1].ellipsis {
		t.Error("the ellipsis is not on the last line")
	}
	const slack = 0.01
	for i := range l.lines {
		ln := &l.lines[i]
		if ln.x+ln.width > length+slack || ln.x < -slack {
			t.Errorf("line %d spans %v to %v, past %v", i, ln.x, ln.x+ln.width, length)
		}
	}
	for i := range l.spans {
		if sp := &l.spans[i]; sp.hi > length+slack || sp.lo < -slack {
			t.Errorf("span %d spans %v to %v, past %v", i, sp.lo, sp.hi, length)
		}
	}
	if got := l.Size(); l.Orientation() == Horizontal && got.W > length+slack {
		t.Errorf("size %v is wider than %v", got, length)
	}
}

func TestTruncate_SingleLineCutsAtTheWidth(t *testing.T) {
	s := goShaper(t)
	const content = "The quick brown fox jumps over the lazy dog"
	full := s.Shape(content, Style{Size: 16}).Layout(geom.Inf).Size().W
	for _, width := range []float32{40, 77, 120, 200} {
		l := s.Shape(content, Style{Size: 16, MaxLines: 1}).Layout(width)
		checkTruncated(t, l, 1, width)
		start, end := l.LineRange(0)
		kept := content[start:end]
		if strings.HasSuffix(kept, " ") {
			t.Errorf("width %v: %q keeps the space before the ellipsis", width, kept)
		}
		// The line is filled: one more cluster and the ellipsis would not fit.
		one := s.Shape(content[:end+1]+ellipsisText, Style{Size: 16}).Layout(geom.Inf).Size().W
		if content[end] != ' ' && one <= width {
			t.Errorf("width %v: %q leaves room for more", width, kept)
		}
	}
	if l := s.Shape(content, Style{Size: 16, MaxLines: 1}).Layout(full + 1); l.Truncated() || l.LineCount() != 1 {
		t.Error("text that fits must not be truncated")
	}
}

func TestTruncate_KeepsTheFirstLinesWhole(t *testing.T) {
	s := goShaper(t)
	const content = "lines break between words and go on and on until the box ends"
	whole := s.Shape(content, Style{Size: 16}).Layout(120)
	l := s.Shape(content, Style{Size: 16, MaxLines: 2}).Layout(120)
	checkTruncated(t, l, 2, 120)
	if a, b := lines(l, content)[0], lines(whole, content)[0]; a != b {
		t.Errorf("first line %q, want it as without MaxLines: %q", a, b)
	}
	// The last line takes more than its own break would give it.
	if _, end := l.LineRange(1); end <= whole.lines[1].start {
		t.Errorf("the last line ends at %d", end)
	}
	if got, want := l.Size().H, 2*whole.lines[0].height; got != want {
		t.Errorf("height %v, want two lines: %v", got, want)
	}
	// What a screen reader reads stays whole.
	if got := l.paragraph.Text(); got != content {
		t.Errorf("text %q", got)
	}
}

func TestTruncate_HardBreaks(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("first\nsecond\nthird", Style{Size: 16, MaxLines: 2}).Layout(400)
	checkTruncated(t, l, 2, 400)
	if got := lines(l, "first\nsecond\nthird"); got[1] != "second" {
		t.Errorf("lines %q", got)
	}
	// An ellipsis follows the kept text when the box has room.
	at, _ := ellipsisGlyph(l)
	if want := l.CaretPosition(len("first\nsecond")); at != want {
		t.Errorf("ellipsis at %v, want right after the text at %v", at, want)
	}
	// Only empty lines left out: nothing is missing.
	if l := s.Shape("first\n\n", Style{Size: 16, MaxLines: 1}).Layout(400); l.Truncated() || l.LineCount() != 1 {
		t.Errorf("trailing empty lines: truncated %v, %d lines", l.Truncated(), l.LineCount())
	}
}

func TestTruncate_TooNarrowForTheEllipsis(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("abcdef", Style{Size: 16, MaxLines: 1}).Layout(4)
	if l.LineCount() != 1 || !l.Truncated() {
		t.Fatalf("%d lines, truncated %v", l.LineCount(), l.Truncated())
	}
	if _, ok := ellipsisGlyph(l); ok {
		t.Error("an ellipsis wider than the box must not be drawn")
	}
	if got := l.Size().W; got > 4 {
		t.Errorf("width %v", got)
	}
}

func TestTruncate_AlignEnd(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("The quick brown fox jumps over the lazy dog", Style{Size: 16, MaxLines: 1, Align: AlignEnd}).Layout(100)
	checkTruncated(t, l, 1, 100)
}

func TestTruncate_Japanese(t *testing.T) {
	s := systemShaper(t)
	const content = "吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。"
	for _, st := range []Style{{Size: 16, MaxLines: 1}, {Size: 16, MaxLines: 2}, {Size: 16, MaxLines: 2, Hanging: true, Align: AlignJustify}} {
		l := s.Shape(content, st).Layout(100)
		checkTruncated(t, l, st.MaxLines, 100)
		// CJK breaks anywhere, so the line is full to within one character
		// and the ellipsis.
		ln := &l.lines[len(l.lines)-1]
		if ln.width < 100-2*16 {
			t.Errorf("%+v: the last line is %v long", st, ln.width)
		}
	}
	// An opening bracket does not stand before the ellipsis.
	const quote = "あいうえお「かきくけこ」"
	w := s.Shape("あいうえお「…", Style{Size: 16}).Layout(geom.Inf).Size().W
	l := s.Shape(quote, Style{Size: 16, MaxLines: 1}).Layout(w)
	checkTruncated(t, l, 1, w)
	if _, end := l.LineRange(0); strings.HasSuffix(quote[:end], "「") {
		t.Errorf("kept %q", quote[:end])
	}
}

func TestTruncate_EmojiStayWhole(t *testing.T) {
	s := systemShaper(t)
	const content = "👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦"
	p := s.Shape(content, Style{Size: 16, MaxLines: 1})
	if len(p.clusters) != 6 {
		t.Skipf("%d clusters: no emoji font that joins the family", len(p.clusters))
	}
	l := p.Layout(80)
	checkTruncated(t, l, 1, 80)
	_, end := l.LineRange(0)
	for c := range p.clusters {
		if end > p.clusters[c].start && end < p.clusters[c].end {
			t.Fatalf("the cut at %d splits a cluster", end)
		}
	}
}

func TestTruncate_RightToLeft(t *testing.T) {
	s := arabicShaper(t)
	const content = "مرحبا بالعالم هذا نص طويل جدا لا يتسع في سطر واحد"
	l := s.Shape(content, Style{Size: 16, MaxLines: 1}).Layout(120)
	checkTruncated(t, l, 1, 120)
	ln := &l.lines[0]
	at, _ := ellipsisGlyph(l)
	// The logical end of a right-to-left line is on its left.
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		if ln.x+l.xs[c] < at {
			t.Fatalf("cluster %d at %v is left of the ellipsis at %v", c, ln.x+l.xs[c], at)
		}
	}
	// An LTR paragraph with Arabic inside has its ellipsis on the right.
	const mixed = "Hello مرحبا بالعالم and the world goes on"
	l = s.Shape(mixed, Style{Size: 16, MaxLines: 1}).Layout(120)
	checkTruncated(t, l, 1, 120)
	ln = &l.lines[0]
	at, _ = ellipsisGlyph(l)
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		if ln.x+l.xs[c]+l.adv[c] > at+0.01 {
			t.Fatalf("cluster %d ends at %v, right of the ellipsis at %v", c, ln.x+l.xs[c]+l.adv[c], at)
		}
	}
}

func TestTruncate_Vertical(t *testing.T) {
	s := systemShaper(t)
	const content = "吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。"
	st := Style{Size: 16, MaxLines: 2, Orientation: Vertical}
	l := s.Shape(content, st).Layout(100)
	checkTruncated(t, l, 2, 100)
	if got := l.Size().H; got > 100 {
		t.Errorf("columns %v tall, past 100", got)
	}
	// The ellipsis takes its vertical form.
	p := l.paragraph
	q := p.sections[l.lines[1].ellipsisSection].ellipsis
	h := s.Shape(ellipsisText, Style{Size: 16})
	if q.glyphs[0].id == h.glyphs[0].id && !q.clusters[0].sideways {
		t.Error("the ellipsis keeps its horizontal form in a vertical line")
	}
}

func TestTruncate_LayoutTwiceIsTheSame(t *testing.T) {
	s := goShaper(t)
	p := s.Shape("The quick brown fox jumps over the lazy dog", Style{Size: 16, MaxLines: 2})
	a, b := p.Layout(90), p.Layout(90)
	if len(a.glyphs) != len(b.glyphs) || a.Size() != b.Size() {
		t.Errorf("layouts differ: %d and %d glyphs, %v and %v", len(a.glyphs), len(b.glyphs), a.Size(), b.Size())
	}
}

func TestStyle_MaxLinesChangesLayout(t *testing.T) {
	if (Style{}).SameLayout(Style{MaxLines: 1}) {
		t.Error("MaxLines must take part in SameLayout")
	}
	if got := (Style{}).WithMaxLines(2).MaxLines; got != 2 {
		t.Errorf("WithMaxLines: %d", got)
	}
}
