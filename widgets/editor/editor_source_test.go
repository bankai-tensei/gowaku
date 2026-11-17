package editor

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
	"time"
)

// memSource is a LineSource over lines in memory, of which the first n are
// known, as a file being read is. Its lines break at \n and \r\n, which it
// writes back as they were.
type memSource struct {
	lines []string
	crlf  []bool
	// off[i] is the offset of line i, one byte per break.
	off []int64
	n   int
	// reads counts the calls of Line.
	reads int
}

func newMemSource(text string) *memSource {
	s := &memSource{}
	s.set(text)
	s.n = len(s.lines)
	return s
}

// set makes the lines those of text, keeping n.
func (s *memSource) set(text string) {
	raw := strings.Split(text, "\n")
	s.lines, s.crlf = make([]string, len(raw)), make([]bool, len(raw))
	s.off = make([]int64, len(raw)+1)
	for i, l := range raw {
		if i < len(raw)-1 && strings.HasSuffix(l, "\r") {
			l, s.crlf[i] = l[:len(l)-1], true
		}
		s.lines[i] = l
		s.off[i+1] = s.off[i] + int64(len(l)) + 1
	}
}

func (s *memSource) Len() int { return max(s.n, 1) }

func (s *memSource) Line(i int) string {
	s.reads++
	return s.lines[i]
}

func (s *memSource) Offset(i int) int64 { return s.off[i] }

func (s *memSource) LineAt(off int64) int {
	i := sort.Search(s.Len(), func(i int) bool { return s.off[i+1] > off })
	return min(i, s.Len()-1)
}

func (s *memSource) Reader() LineReader { return memReader{lines: s.lines[:s.Len()]} }

func (s *memSource) WriteLines(w io.Writer, from, to int) error {
	for i := from; i < to; i++ {
		if _, err := io.WriteString(w, s.lines[i]); err != nil {
			return err
		}
		if i < len(s.lines)-1 {
			nl := "\n"
			if s.crlf[i] {
				nl = "\r\n"
			}
			if _, err := io.WriteString(w, nl); err != nil {
				return err
			}
		}
	}
	return nil
}

type memReader struct{ lines []string }

func (r memReader) ReadLines(from, to int, fn func(i int, line string) bool) error {
	for i := from; i < to; i++ {
		if !fn(i, r.lines[i]) {
			return nil
		}
	}
	return nil
}

func (memReader) Close() error { return nil }

// plainSource is a memSource that is no LineWriter.
type plainSource struct{ s *memSource }

func (p plainSource) Len() int             { return p.s.Len() }
func (p plainSource) Line(i int) string    { return p.s.Line(i) }
func (p plainSource) Offset(i int) int64   { return p.s.Offset(i) }
func (p plainSource) LineAt(off int64) int { return p.s.LineAt(off) }
func (p plainSource) Reader() LineReader   { return p.s.Reader() }

func clip(s string) string {
	if len(s) > 80 {
		return s[:40] + "…" + s[len(s)-40:]
	}
	return s
}

// checkText compares the text of ctl, line by line and as offsets, with want.
func checkText(t *testing.T, ctl *Controller, want string) {
	t.Helper()
	lines := strings.Split(want, "\n")
	if got := ctl.LineCount(); got != len(lines) {
		t.Fatalf("%d lines, want %d", got, len(lines))
	}
	for i, l := range lines {
		if got := ctl.Line(i); got != l {
			t.Fatalf("line %d = %q, want %q", i, got, l)
		}
	}
	if got := ctl.Len(); got != len(want) {
		t.Fatalf("Len = %d, want %d", got, len(want))
	}
	if got := ctl.Text(); got != want {
		t.Fatalf("Text differs: %q, want %q", clip(got), clip(want))
	}
	off := 0
	for i, l := range lines {
		for _, col := range []int{0, len(l) / 2, len(l)} {
			// A position inside a character moves to its start.
			for col > 0 && col < len(l) && !startsRune(l[col]) {
				col--
			}
			p := TextPosition{Line: i, Column: col}
			if got := ctl.Offset(p); got != off+col {
				t.Fatalf("Offset(%v) = %d, want %d", p, got, off+col)
			}
			if got := ctl.PositionAt(off + col); got != p && col < len(l) {
				t.Fatalf("PositionAt(%d) = %v, want %v", off+col, got, p)
			}
		}
		off += len(l) + 1
	}
}

func written(t *testing.T, c *Controller, nl string) string {
	t.Helper()
	var b strings.Builder
	if err := c.WriteTo(&b, nl); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestSetSource_ShowsTheLines(t *testing.T) {
	content := "first\r\nsecond\nthird\r\n\nlast"
	var ctl Controller
	src := newMemSource(content)
	ctl.SetSource(src)
	checkText(t, &ctl, strings.ReplaceAll(content, "\r\n", "\n"))
	if ctl.Source() != src {
		t.Error("Source must return the source")
	}
	ctl.SetText("x")
	if ctl.Source() != nil {
		t.Error("SetText must let the source go")
	}
}

func TestSetSource_ReadsOnlyTheLinesInView(t *testing.T) {
	var b strings.Builder
	for i := range 1_000_000 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	src := newMemSource(b.String())
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{LineNumbers: true, ReadOnly: true})
	src.reads = 0
	h.view().state.Jump(1e9)
	h.frame()
	if src.reads == 0 || src.reads > 2000 {
		t.Errorf("a jump to the end read %d lines", src.reads)
	}
}

func TestSourceGrew_GrowsTheTextInSteps(t *testing.T) {
	var b strings.Builder
	for i := range 3000 {
		fmt.Fprintf(&b, "line %d %s\r\n", i, strings.Repeat("x", i%17))
	}
	b.WriteString("end without a break")
	content := b.String()
	src := newMemSource(content)
	src.n = 10
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{LineNumbers: true, ReadOnly: true})
	for src.n < len(src.lines) {
		from := src.n
		src.n = min(src.n+211, len(src.lines))
		ctl.SourceGrew(from)
		h.frame()
	}
	checkText(t, &ctl, strings.ReplaceAll(content, "\r\n", "\n"))
}

func TestSourceGrew_GoesOnTheLastLine(t *testing.T) {
	src := newMemSource("a\nb")
	var ctl Controller
	ctl.SetSource(src)
	ctl.Replace(TextRange{Start: TextPosition{0, 0}, End: TextPosition{0, 0}}, ">")
	// The source learns that its last line went on, and a line after it.
	src.set("a\nbc\nd")
	src.n = 3
	ctl.SourceGrew(1)
	checkText(t, &ctl, ">a\nbc\nd")
}

// TestSource_EditsMatchAString makes random edits to a source, some while it
// still grows, and the same edits to a string, and compares them after each.
func TestSource_EditsMatchAString(t *testing.T) {
	var b strings.Builder
	for i := range 400 {
		fmt.Fprintf(&b, "%03d %s\n", i, strings.Repeat("ab", i%5))
	}
	src := newMemSource(b.String())
	src.n = 40
	var ctl Controller
	ctl.SetSource(src)
	r := rand.New(rand.NewPCG(3, 4))
	inserts := []string{"", "Z", "new line\n", "\n", "two\nlines\n", "日本語"}
	edit := func(want string) string {
		n := len(want)
		a := r.IntN(n + 1)
		z := min(a+r.IntN(40), n)
		for a > 0 && a < n && !startsRune(want[a]) {
			a--
		}
		for z > 0 && z < n && !startsRune(want[z]) {
			z--
		}
		z = max(z, a)
		s := inserts[r.IntN(len(inserts))]
		ctl.Replace(TextRange{Start: ctl.PositionAt(a), End: ctl.PositionAt(z)}, s)
		return want[:a] + s + want[z:]
	}
	want := ctl.Text()
	for range 20 {
		want = edit(want)
		if got := ctl.Text(); got != want {
			t.Fatalf("while growing: %q, want %q", clip(got), clip(want))
		}
	}
	// The rest of the source comes.
	from := src.n
	src.n = len(src.lines)
	ctl.SourceGrew(from)
	want += "\n" + strings.Join(src.lines[from:], "\n")
	checkText(t, &ctl, want)
	for range 300 {
		want = edit(want)
		checkText(t, &ctl, want)
	}
	if n := len(ctl.buf.pieces); n > 600 {
		t.Errorf("%d pieces after 320 edits", n)
	}
}

func TestWriteTo_KeepsTheBreaksOfTheSource(t *testing.T) {
	src := newMemSource("keep\r\nedit me\r\nkeep too\r\nlast")
	var ctl Controller
	ctl.SetSource(src)
	ctl.Replace(TextRange{Start: TextPosition{1, 0}, End: TextPosition{1, 7}}, "edited\nand added")
	// The lines of the source keep their \r\n; the new ones take nl.
	if got, want := written(t, &ctl, "\r\n"), "keep\r\nedited\r\nand added\r\nkeep too\r\nlast"; got != want {
		t.Errorf("wrote %q, want %q", got, want)
	}
	// A source that is no LineWriter is written as the lines in memory are.
	var plain Controller
	plain.SetSource(plainSource{newMemSource("a\r\nb\r\nc")})
	if got := written(t, &plain, "\n"); got != "a\nb\nc" {
		t.Errorf("wrote %q", got)
	}
}

func TestWriteTo_EditsAtTheEndsOfTheSource(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		edit          func(*Controller)
		want          string
	}{
		{"append to the last line", "a\nb", func(c *Controller) { c.Replace(TextRange{TextPosition{1, 1}, TextPosition{1, 1}}, "c") }, "a\nbc"},
		{"delete the last line", "a\nb\n", func(c *Controller) { c.Replace(TextRange{TextPosition{1, 0}, TextPosition{2, 0}}, "") }, "a\n"},
		{"delete the end of the text", "a\nb\nc", func(c *Controller) { c.Replace(TextRange{TextPosition{0, 1}, TextPosition{2, 1}}, "") }, "a"},
		{"edit the first line", "a\nb\nc", func(c *Controller) { c.Replace(TextRange{TextPosition{0, 0}, TextPosition{0, 0}}, ">") }, ">a\nb\nc"},
		{"keep the trailing break", "a\nb\n", func(c *Controller) { c.Replace(TextRange{TextPosition{0, 0}, TextPosition{0, 1}}, "A") }, "A\nb\n"},
		{"end inside the source", "a\nb\nc\nd", func(c *Controller) { c.Replace(TextRange{TextPosition{3, 0}, TextPosition{3, 1}}, "") }, "a\nb\nc\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ctl Controller
			ctl.SetSource(newMemSource(tc.content))
			tc.edit(&ctl)
			if got := written(t, &ctl, "\n"); got != tc.want {
				t.Fatalf("wrote %q, want %q", got, tc.want)
			}
		})
	}
}

// place finds where an edit goes in src by its lines and those around them,
// as an application that rebases does, the nearest to near.
func placeIn(src *memSource, e SourceEdit, near int) int {
	fits := func(p int) bool {
		if p-len(e.Before) < 0 || p+len(e.Removed)+len(e.After) > src.Len() {
			return false
		}
		for i, l := range e.Before {
			if src.lines[p-len(e.Before)+i] != l {
				return false
			}
		}
		for i, l := range e.Removed {
			if src.lines[p+i] != l {
				return false
			}
		}
		for i, l := range e.After {
			if src.lines[p+len(e.Removed)+i] != l {
				return false
			}
		}
		return true
	}
	for d := 0; d < src.Len(); d++ {
		if fits(near + d) {
			return near + d
		}
		if fits(near - d) {
			return near - d
		}
	}
	return -1
}

func numberedLines(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestEdits_TellTheLinesAroundEvenAfterTheSourceChanged(t *testing.T) {
	src := newMemSource(numberedLines(100))
	var ctl Controller
	ctl.SetSource(src)
	ctl.Replace(TextRange{Start: TextPosition{50, 0}, End: TextPosition{51, 0}}, "MINE 50\n")
	// The source under the text changes, as a file rewritten in place.
	src.set(strings.Repeat("gone\n", 100))
	edits := ctl.Edits()
	if len(edits) != 1 {
		t.Fatalf("%d edits", len(edits))
	}
	e := edits[0]
	if e.From != 50 || e.To != 51 || strings.Join(e.Removed, "|") != "line 50" ||
		strings.Join(e.Before, "|") != "line 48|line 49" || strings.Join(e.After, "|") != "line 51|line 52" ||
		strings.Join(e.Inserted, "|") != "MINE 50" {
		t.Errorf("edit %+v", e)
	}
}

func TestRebase_PlacesTheEditsInANewSource(t *testing.T) {
	src := newMemSource(numberedLines(1000))
	var ctl Controller
	ctl.SetSource(src)
	edit := func(line, endLine int, s string) {
		ctl.Replace(TextRange{Start: TextPosition{line, 0}, End: TextPosition{endLine, 0}}, s)
	}
	// From the bottom up, so that the lines of the text are those of the source.
	edit(800, 802, "")
	edit(500, 500, "INSERTED before 500\n")
	edit(100, 101, "EDITED 100\n")
	fresh := "header 1\nheader 2\nheader 3\n" + strings.Replace(numberedLines(1000), "line 300\n", "line 300 changed\n", 1)
	next := newMemSource(fresh)
	var placed []PlacedEdit
	drift := 0
	for k, e := range ctl.Edits() {
		at := placeIn(next, e, e.From+drift)
		if at < 0 {
			t.Fatalf("edit %d not found", k)
		}
		placed = append(placed, PlacedEdit{Edit: k, At: at})
		drift = at - e.From
	}
	ctl.Rebase(next, placed)
	want := strings.Replace(fresh, "line 100\n", "EDITED 100\n", 1)
	want = strings.Replace(want, "line 500\n", "INSERTED before 500\nline 500\n", 1)
	want = strings.Replace(want, "line 800\nline 801\n", "", 1)
	checkText(t, &ctl, want)
	if ctl.Source() != next {
		t.Error("the text must show the new source")
	}
	// An edit placed over an earlier one is left out.
	ctl.Replace(TextRange{Start: TextPosition{10, 0}, End: TextPosition{11, 0}}, "A\n")
	ctl.Replace(TextRange{Start: TextPosition{20, 0}, End: TextPosition{21, 0}}, "B\n")
	again := newMemSource(ctl.buf.src.(*memSource).lines[0] + "\n" + strings.Join(next.lines[1:], "\n"))
	ctl.Rebase(again, []PlacedEdit{{Edit: 0, At: 10}, {Edit: 1, At: 5}})
	if ctl.Line(10) != "A" || ctl.Line(20) == "B" {
		t.Errorf("lines 10 and 20: %q %q", ctl.Line(10), ctl.Line(20))
	}
}

func TestSource_GrowsInAnEditorWhileSearching(t *testing.T) {
	src := newMemSource(bigLog(2000))
	src.n = 100
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{ReadOnly: true})
	if _, err := ctl.Find("ERROR", FindOptions{MatchCase: true}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for src.n < len(src.lines) || ctl.Searching() {
		if time.Now().After(deadline) {
			t.Fatal("never done")
		}
		if src.n < len(src.lines) {
			from := src.n
			src.n = min(src.n+300, len(src.lines))
			ctl.SourceGrew(from)
		}
		time.Sleep(time.Millisecond)
		h.tree.Tick(time.Second)
		h.frame()
	}
	sameMatches(t, ctl.Matches(), wantMatches(t, &ctl, "ERROR", FindOptions{MatchCase: true}))
}

// TestSourceGrew_MakesNoEntryPerLine grows a source shown in an editor by a
// million lines at once: the editor lays out the lines in view, not one entry
// for each line that came.
func TestSourceGrew_MakesNoEntryPerLine(t *testing.T) {
	src := newMemSource(numberedLines(1_000_000))
	src.n = 10
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{ReadOnly: true})
	h.frame()
	src.n = len(src.lines)
	ctl.SourceGrew(10)
	h.frame()
	if n := len(h.view().cache.entries); n > 2*maxSpliced {
		t.Errorf("the cache holds %d entries after the source grew", n)
	}
	checkText(t, &ctl, numberedLines(1_000_000))
}

func TestReplaceSource_KeepsTheHistoryTheSelectionAndTheScroll(t *testing.T) {
	src := newMemSource(numberedLines(2000))
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{})
	ctl.SetSelection(TextRange{Start: TextPosition{1200, 2}, End: TextPosition{1200, 2}})
	ctl.Insert("EDIT ")
	h.view().state.Jump(float64(1180) * float64(h.view().lineHeight))
	h.frame()
	before := ctl.Text()
	sel := ctl.Selection()
	scroll := ctl.primary.Scroll()
	// The text is saved: a source of the same text takes the place of the old.
	saved := newMemSource(written(t, &ctl, "\n"))
	if err := ctl.ReplaceSource(saved); err != nil {
		t.Fatal(err)
	}
	h.frame()
	if ctl.Source() != saved || len(ctl.Edits()) != 0 {
		t.Errorf("the text must read the saved source with no edits against it, got %d", len(ctl.Edits()))
	}
	if got := ctl.Selection(); got != sel {
		t.Errorf("selection %v, want %v", got, sel)
	}
	if got := ctl.primary.Scroll(); got.Line != scroll.Line {
		t.Errorf("scroll at line %d, want %d", got.Line, scroll.Line)
	}
	checkText(t, &ctl, before)
	if !ctl.CanUndo() {
		t.Fatal("the undo history must stay")
	}
	ctl.Undo()
	checkText(t, &ctl, numberedLines(2000))
	// A source of another text is refused.
	if err := ctl.ReplaceSource(newMemSource("other")); !errors.Is(err, ErrNotSameText) {
		t.Errorf("ReplaceSource of another text: %v", err)
	}
	checkText(t, &ctl, numberedLines(2000))
}

func TestRebase_MovesTheSelectionAndTheScrollWithTheirLines(t *testing.T) {
	src := newMemSource(numberedLines(3000))
	var ctl Controller
	ctl.SetSource(src)
	h := newEditor(t, &ctl, Editor{})
	// An edit above, and the caret on a line of the source below it.
	ctl.Replace(TextRange{Start: TextPosition{100, 0}, End: TextPosition{101, 0}}, "EDITED 100\nADDED\n")
	ctl.SetSelection(TextRange{Start: TextPosition{2001, 3}, End: TextPosition{2001, 5}})
	h.view().state.Jump(float64(1990) * float64(h.view().lineHeight))
	h.frame()
	scroll := ctl.primary.Scroll().Line
	// Another program put three lines at the top.
	next := newMemSource("one\ntwo\nthree\n" + numberedLines(3000))
	var placed []PlacedEdit
	for k, e := range ctl.Edits() {
		placed = append(placed, PlacedEdit{Edit: k, At: placeIn(next, e, e.From+3)})
	}
	ctl.Rebase(next, placed)
	h.frame()
	// Line 2001 of the text was line 2000 of the source, now line 2003 of
	// the new one, which the placed edit, one line longer, puts at 2004.
	want := TextRange{Start: TextPosition{2004, 3}, End: TextPosition{2004, 5}}
	if got := ctl.Selection(); got != want {
		t.Errorf("selection %v, want %v", got, want)
	}
	if ctl.Line(2004) != "line 2000" {
		t.Fatalf("line 2004 is %q", ctl.Line(2004))
	}
	if got := ctl.primary.Scroll().Line; got != scroll+3 {
		t.Errorf("scroll at line %d, want %d", got, scroll+3)
	}
	// A caret on a line the edit put in follows the edit.
	ctl.SetSelection(TextRange{Start: TextPosition{104, 1}, End: TextPosition{104, 1}})
	if ctl.Line(104) != "ADDED" {
		t.Fatalf("line 104 is %q", ctl.Line(104))
	}
	again := newMemSource("zero\n" + strings.Join(next.lines, "\n"))
	placed = placed[:0]
	for k, e := range ctl.Edits() {
		placed = append(placed, PlacedEdit{Edit: k, At: placeIn(again, e, e.From+1)})
	}
	ctl.Rebase(again, placed)
	if got := ctl.Selection().End; got != (TextPosition{105, 1}) || ctl.Line(105) != "ADDED" {
		t.Errorf("caret %v on %q, want line 105 on ADDED", got, ctl.Line(got.Line))
	}
}

func TestSourceGrew_PanicsWhenTheSourceLostLines(t *testing.T) {
	src := newMemSource(numberedLines(100))
	var ctl Controller
	ctl.SetSource(src)
	src.n = 50
	defer func() {
		r := recover()
		if s, ok := r.(string); !ok || !strings.Contains(s, "must not lose lines") {
			t.Errorf("recovered %v", r)
		}
	}()
	ctl.SourceGrew(40)
}
