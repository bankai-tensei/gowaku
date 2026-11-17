package editor

import (
	"bufio"
	"io"
)

// Snapshot is the text of a Controller as it was when Controller.Snapshot took
// it, which later edits do not change. It is for saving a long text without
// stopping the window: the UI goroutine takes the snapshot, which is quick,
// and another goroutine writes it with WriteTo, which reads every line.
//
// A snapshot holds a LineReader of the source, so it must be closed.
// See docs/ADR/0159-editor-snapshot.md.
type Snapshot struct {
	pieces []piece
	// rd reads the lines of the source pieces, or is nil without a source,
	// and lw is rd when it also writes lines with their own breaks.
	rd LineReader
	lw LineWriter
	// srcLen is the number of lines of the source when the snapshot was
	// taken, lines the number of lines of the text, and bytes their lengths
	// plus one per line.
	srcLen, lines int
	bytes         int64
	// buf and gen are the buffer the snapshot was taken from and its count
	// of edits then, for Controller.Changed.
	buf *lineBuffer
	gen uint64
}

// Snapshot returns the text as it is now, for WriteTo on another goroutine.
// Call it on the UI goroutine, and Close the snapshot when it is written.
//
// It copies the list of the pieces of the text and of the lines held in
// memory, 16 bytes per such line and not their text, and asks the source for
// a reader; it reads no line. The lines of the source are read through that
// reader, as the source held them now: see LineSource.Reader.
func (c *Controller) Snapshot() *Snapshot {
	b := &c.buf
	b.ensure()
	s := &Snapshot{
		pieces: make([]piece, len(b.pieces)),
		lines:  b.lineCount(),
		bytes:  int64(b.length()) + 1,
		buf:    b,
		gen:    b.gen,
	}
	for k, p := range b.pieces {
		// An edit changes the lines of a piece in memory in place.
		s.pieces[k] = piece{source: p.source, start: p.start, n: p.n, lines: append([]string(nil), p.lines...)}
	}
	if b.src != nil {
		s.srcLen = b.src.Len()
		s.rd = b.src.Reader()
		s.lw, _ = s.rd.(LineWriter)
	}
	return s
}

// Changed reports whether the text changed since s was taken, by an edit, an
// undo, SetText, SetSource, Rebase or SourceGrew. It is false for the snapshot
// of another controller. Call it on the UI goroutine.
//
// After s was written to a file, ReplaceSource with a source of that file is
// right only while Changed is false: ReplaceSource itself compares the number
// of lines and bytes, which an edit can leave as they were.
func (c *Controller) Changed(s *Snapshot) bool {
	return s.buf != &c.buf || s.gen != c.buf.gen
}

// Size returns the number of bytes WriteTo writes with nl between the lines,
// to show how far a save is. It is exact but for the lines a LineWriter
// writes with breaks of another length than nl, as \r\n where nl is \n.
func (s *Snapshot) Size(nl string) int64 {
	return s.bytes - int64(s.lines) + int64(s.lines-1)*int64(len(nl))
}

// Lines returns the number of lines of the text.
func (s *Snapshot) Lines() int { return s.lines }

// WriteTo writes the text to w, as Controller.WriteTo would have when the
// snapshot was taken: the lines of the source that the text did not edit as
// the reader of the source writes them, when it is a LineWriter, so that
// their breaks stay as they were, and the other lines with nl after each but
// the last. It may run on any goroutine, while the controller is edited and
// shown, and reads the source only through its LineReader. One call runs at a
// time.
func (s *Snapshot) WriteTo(w io.Writer, nl string) error {
	out := lineOut{w: bufio.NewWriterSize(w, 1<<16)}
	last := len(s.pieces) - 1
	for k, p := range s.pieces {
		end := p.start + p.n
		switch {
		case p.source && s.lw != nil:
			srcEnd := end == s.srcLen
			if k == last && !srcEnd {
				// The text ends inside the source: its last line without the
				// break the source has after it.
				if err := s.lw.WriteLines(out.w, p.start, end-1); err != nil {
					return err
				}
				if err := s.read(&out, end-1, end, "", false); err != nil {
					return err
				}
				continue
			}
			if err := s.lw.WriteLines(out.w, p.start, end); err != nil {
				return err
			}
			if k < last && srcEnd {
				// The last line of the source, which has no break, and more
				// text after it.
				out.put(nl)
			}
		case p.source:
			if err := s.read(&out, p.start, end, nl, k < last); err != nil {
				return err
			}
		default:
			for j, l := range p.lines {
				out.put(l)
				if k < last || j < p.n-1 {
					out.put(nl)
				}
			}
		}
		if out.err != nil {
			return out.err
		}
	}
	if out.err != nil {
		return out.err
	}
	return out.w.Flush()
}

// read writes lines from to to-1 of the source with nl between them, and
// after the last of them too if more text follows.
func (s *Snapshot) read(out *lineOut, from, to int, nl string, more bool) error {
	err := s.rd.ReadLines(from, to, func(i int, line string) bool {
		out.put(line)
		if more || i < to-1 {
			out.put(nl)
		}
		return out.err == nil
	})
	if err != nil {
		return err
	}
	return out.err
}

// Close lets the reader of the source go. The snapshot writes nothing after
// it. Closing twice is harmless.
func (s *Snapshot) Close() error {
	if s.rd == nil {
		return nil
	}
	rd := s.rd
	s.rd, s.lw, s.pieces = nil, nil, nil
	return rd.Close()
}

// lineOut writes strings and keeps the first error.
type lineOut struct {
	w   *bufio.Writer
	err error
}

func (o *lineOut) put(s string) {
	if o.err == nil {
		_, o.err = o.w.WriteString(s)
	}
}
