package editor

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// threadSource is a memSource whose reader writes lines with their own
// breaks too, on any goroutine, and counts what is open.
type threadSource struct {
	*memSource
	open, closed int
}

func (s *threadSource) Reader() LineReader {
	s.open++
	// The reader keeps its own lines, those known now, as a reader of a file
	// keeps its own handle and a copy of the index.
	n := s.Len()
	return &threadReader{src: s, lines: append([]string(nil), s.lines[:n]...), crlf: append([]bool(nil), s.crlf[:n]...)}
}

type threadReader struct {
	src   *threadSource
	lines []string
	crlf  []bool
}

func (r *threadReader) ReadLines(from, to int, fn func(i int, line string) bool) error {
	for i := from; i < to; i++ {
		if !fn(i, r.lines[i]) {
			return nil
		}
	}
	return nil
}

func (r *threadReader) WriteLines(w io.Writer, from, to int) error {
	for i := from; i < to; i++ {
		nl := "\n"
		if r.crlf[i] {
			nl = "\r\n"
		}
		if i == len(r.lines)-1 {
			nl = ""
		}
		if _, err := io.WriteString(w, r.lines[i]+nl); err != nil {
			return err
		}
	}
	return nil
}

func (r *threadReader) Close() error {
	r.src.closed++
	return nil
}

func snapshotWritten(t *testing.T, s *Snapshot, nl string) string {
	t.Helper()
	var b strings.Builder
	if err := s.WriteTo(&b, nl); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func caretAtPos(line, col int) TextRange {
	return TextRange{Start: TextPosition{line, col}, End: TextPosition{line, col}}
}

func span(l1, c1, l2, c2 int) TextRange {
	return TextRange{Start: TextPosition{l1, c1}, End: TextPosition{l2, c2}}
}

// snapshotCases are texts and edits that reach every branch of the writer.
var snapshotCases = []struct {
	name, content string
	edit          func(*Controller)
}{
	{"untouched", "a\r\nb\nc", func(*Controller) {}},
	{"one line", "only", func(*Controller) {}},
	{"empty", "", func(*Controller) {}},
	{"ends in a break", "a\r\nb\r\n", func(*Controller) {}},
	{"edit in the middle", "keep\r\nedit me\r\nkeep too\r\nlast", func(c *Controller) { c.Replace(span(1, 0, 1, 7), "edited\nand added") }},
	{"append to the last line", "a\nb", func(c *Controller) { c.Replace(caretAtPos(1, 1), "c") }},
	{"add lines after the last", "a\r\nb", func(c *Controller) { c.Replace(caretAtPos(1, 1), "\nc\nd") }},
	{"delete the last line", "a\nb\n", func(c *Controller) { c.Replace(span(1, 0, 2, 0), "") }},
	{"delete the end of the text", "a\nb\nc", func(c *Controller) { c.Replace(span(0, 1, 2, 1), "") }},
	{"edit the first line", "a\r\nb\r\nc", func(c *Controller) { c.Replace(caretAtPos(0, 0), ">") }},
	{"keep the trailing break", "a\nb\n", func(c *Controller) { c.Replace(span(0, 0, 0, 1), "A") }},
	{"end inside the source", "a\r\nb\r\nc\r\nd", func(c *Controller) { c.Replace(span(3, 0, 3, 1), "") }},
	{"end inside the source, one line", "a\r\nb", func(c *Controller) { c.Replace(span(0, 1, 1, 1), "") }},
	{"several edits", "1\r\n2\r\n3\r\n4\r\n5\r\n6", func(c *Controller) {
		c.Replace(span(4, 0, 4, 1), "five")
		c.Replace(span(1, 0, 1, 1), "two\ntwo and a half")
		c.Replace(caretAtPos(0, 0), "zero\n")
	}},
	{"everything replaced", "a\nb", func(c *Controller) { c.Replace(span(0, 0, 1, 1), "x\ny\nz") }},
}

func TestSnapshot_WritesWhatTheControllerWrites(t *testing.T) {
	for _, tc := range snapshotCases {
		for _, nl := range []string{"\n", "\r\n"} {
			// A source whose reader keeps the breaks, against a controller
			// whose source does.
			var want, got Controller
			want.SetSource(newMemSource(tc.content))
			src := &threadSource{memSource: newMemSource(tc.content)}
			got.SetSource(src)
			tc.edit(&want)
			tc.edit(&got)
			snap := got.Snapshot()
			if w, g := written(t, &want, nl), snapshotWritten(t, snap, nl); g != w {
				t.Errorf("%s, nl %q, a reader that writes: snapshot wrote %q, the controller %q", tc.name, nl, g, w)
			}
			if err := snap.Close(); err != nil || src.closed != 1 {
				t.Errorf("%s: close: %v, %d readers of the snapshot closed, want 1", tc.name, err, src.closed)
			}

			// A reader that only reads: every break is nl, as in a
			// controller whose source is no LineWriter.
			var plainWant, plainGot Controller
			plainWant.SetSource(plainSource{newMemSource(tc.content)})
			plainGot.SetSource(plainSource{newMemSource(tc.content)})
			tc.edit(&plainWant)
			tc.edit(&plainGot)
			snap = plainGot.Snapshot()
			w, g := written(t, &plainWant, nl), snapshotWritten(t, snap, nl)
			if g != w {
				t.Errorf("%s, nl %q, a reader that reads: snapshot wrote %q, the controller %q", tc.name, nl, g, w)
			}
			if size := snap.Size(nl); size != int64(len(g)) {
				t.Errorf("%s, nl %q: Size %d, wrote %d bytes", tc.name, nl, size, len(g))
			}
			if lines := strings.Count(g, nl) + 1; snap.Lines() != lines {
				t.Errorf("%s, nl %q: Lines %d, wrote %d", tc.name, nl, snap.Lines(), lines)
			}
			snap.Close() //nolint:errcheck // a reader in memory

			// A text without a source.
			var textWant, textGot Controller
			textWant.SetText(tc.content)
			textGot.SetText(tc.content)
			tc.edit(&textWant)
			tc.edit(&textGot)
			snap = textGot.Snapshot()
			if w, g := written(t, &textWant, nl), snapshotWritten(t, snap, nl); g != w {
				t.Errorf("%s, nl %q, no source: snapshot wrote %q, the controller %q", tc.name, nl, g, w)
			}
			if err := snap.Close(); err != nil {
				t.Errorf("%s: close without a source: %v", tc.name, err)
			}
		}
	}
}

func TestSnapshot_ZeroController(t *testing.T) {
	var c Controller
	s := c.Snapshot()
	if got := snapshotWritten(t, s, "\n"); got != "" || s.Size("\n") != 0 || s.Lines() != 1 {
		t.Errorf("wrote %q, Size %d, Lines %d", got, s.Size("\n"), s.Lines())
	}
	if err := s.Close(); err != nil {
		t.Error(err)
	}
}

func TestSnapshot_KeepsTheTextThroughLaterEdits(t *testing.T) {
	for _, source := range []bool{true, false} {
		var c Controller
		content := "one\r\ntwo\r\nthree\r\nfour"
		if source {
			c.SetSource(&threadSource{memSource: newMemSource(content)})
		} else {
			c.SetText(content)
		}
		c.Replace(span(1, 0, 1, 3), "TWO")
		snap := c.Snapshot()
		before := snapshotWritten(t, snap, "\r\n")
		if c.Changed(snap) {
			t.Error("nothing changed yet")
		}
		// Edits of the same piece in memory, of a line of the source, an
		// undo, and a whole new text.
		c.Replace(span(1, 0, 1, 3), "2")
		c.Replace(caretAtPos(3, 0), "new line\n")
		if !c.Changed(snap) {
			t.Error("an edit is a change")
		}
		c.Undo()
		c.SetText("something else")
		if got := snapshotWritten(t, snap, "\r\n"); got != before {
			t.Errorf("source %v: the snapshot wrote %q after the edits, %q before", source, got, before)
		}
		if want := "one\r\nTWO\r\nthree\r\nfour"; before != want {
			t.Errorf("source %v: wrote %q, want %q", source, before, want)
		}
		snap.Close() //nolint:errcheck // a reader in memory
	}
}

func TestSnapshot_ChangedTellsWhetherReplaceSourceIsRight(t *testing.T) {
	var c, other Controller
	c.SetSource(&threadSource{memSource: newMemSource("ab\ncd")})
	c.Replace(caretAtPos(0, 2), "!")
	snap := c.Snapshot()
	defer snap.Close() //nolint:errcheck // a reader in memory
	saved := snapshotWritten(t, snap, "\n")
	if !other.Changed(snap) {
		t.Error("the snapshot of another controller counts as changed")
	}
	// Saved and unchanged: the file takes over, and that is no change.
	if c.Changed(snap) {
		t.Fatal("changed before any edit")
	}
	if err := c.ReplaceSource(newMemSource(saved)); err != nil {
		t.Fatal(err)
	}
	if c.Changed(snap) {
		t.Error("ReplaceSource does not change the text")
	}
	// An edit that keeps the number of lines and bytes: ReplaceSource alone
	// would take the saved file for the text.
	c.Replace(span(1, 0, 1, 1), "X")
	if !c.Changed(snap) {
		t.Error("an edit of the same length is a change")
	}
	if err := c.ReplaceSource(newMemSource(saved)); err != nil {
		t.Errorf("ReplaceSource compares lines and bytes only, got %v", err)
	}
	c.Replace(caretAtPos(1, 0), "longer")
	if err := c.ReplaceSource(newMemSource(saved)); !errors.Is(err, ErrNotSameText) {
		t.Errorf("got %v, want ErrNotSameText", err)
	}
}

func TestSnapshot_SourceThatGrewAfterIt(t *testing.T) {
	mem := newMemSource("a\r\nb\r\nc\r\nd")
	mem.n = 2
	src := &threadSource{memSource: mem}
	var c Controller
	c.SetSource(src)
	snap := c.Snapshot()
	defer snap.Close() //nolint:errcheck // a reader in memory
	mem.n = 4
	c.SourceGrew(2)
	if !c.Changed(snap) {
		t.Error("a source that grew is a change")
	}
	// The text of then ended at line b, inside what the source holds now.
	if got := snapshotWritten(t, snap, "\n"); got != "a\r\nb" {
		t.Errorf("wrote %q, want the two lines of then", got)
	}
}

type failingWriter struct{ after int }

var errDiskFull = errors.New("disk full")

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after -= len(p); w.after < 0 {
		return 0, errDiskFull
	}
	return len(p), nil
}

type failingReader struct{}

var errUnreadable = errors.New("unreadable")

func (failingReader) ReadLines(int, int, func(int, string) bool) error { return errUnreadable }

func (failingReader) WriteLines(io.Writer, int, int) error { return errUnreadable }

func (failingReader) Close() error { return nil }

func TestSnapshot_ErrorsComeBack(t *testing.T) {
	long := strings.Repeat("line of the text\r\n", 20000)
	for _, tc := range snapshotCases {
		var c Controller
		c.SetSource(&threadSource{memSource: newMemSource(tc.content + "\n" + long)})
		tc.edit(&c)
		snap := c.Snapshot()
		if err := snap.WriteTo(&failingWriter{after: 100000}, "\n"); !errors.Is(err, errDiskFull) {
			t.Errorf("%s: got %v, want the error of the writer", tc.name, err)
		}
		snap.Close() //nolint:errcheck // a reader in memory
	}
	var c Controller
	c.SetSource(&threadSource{memSource: newMemSource("a\nb\nc")})
	c.Replace(caretAtPos(1, 0), "x")
	snap := c.Snapshot()
	snap.rd, snap.lw = failingReader{}, failingReader{}
	if err := snap.WriteTo(io.Discard, "\n"); !errors.Is(err, errUnreadable) {
		t.Errorf("got %v, want the error of the source", err)
	}
	snap.lw = nil
	if err := snap.WriteTo(io.Discard, "\n"); !errors.Is(err, errUnreadable) {
		t.Errorf("reading: got %v, want the error of the source", err)
	}
}

// The race detector watches this one: the snapshot is written on another
// goroutine while the UI goroutine edits the text and reads the source.
func TestSnapshot_WritesWhileTheTextIsEdited(t *testing.T) {
	lines := make([]string, 20000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d of the source", i)
	}
	src := &threadSource{memSource: newMemSource(strings.Join(lines, "\r\n"))}
	var c Controller
	c.SetSource(src)
	for i := 0; i < 20000; i += 500 {
		c.Replace(caretAtPos(i, 0), "edited ")
	}
	want := written(t, &c, "\r\n")
	snap := c.Snapshot()
	var wg sync.WaitGroup
	var got strings.Builder
	var err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = snap.WriteTo(&got, "\r\n")
	}()
	for i := 0; i < 20000; i += 37 {
		c.Replace(caretAtPos(i, 0), "later ")
		_ = c.Line(i)
		_ = src.Line(i)
		if i%740 == 0 {
			c.Undo()
		}
	}
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Errorf("the snapshot wrote %d bytes that differ from the %d of the text when it was taken", got.Len(), len(want))
	}
	if err := snap.Close(); err != nil {
		t.Error(err)
	}
	if err := snap.Close(); err != nil {
		t.Errorf("closing twice: %v", err)
	}
}
