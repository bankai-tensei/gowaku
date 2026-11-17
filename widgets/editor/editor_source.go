package editor

import "io"

// LineSource holds lines that an Editor shows and edits without holding them,
// as the lines of a file of gigabytes that the application reads where they
// are asked for. Controller.SetSource takes it; edits keep in memory only the
// lines they change, and the rest are read from the source when shown.
//
// The editor calls the methods on the UI goroutine. The lines of a source do
// not change while the controller shows it: a source that learns new lines at
// its end, as a file being indexed or a log being written, tells
// Controller.SourceGrew, and one that changed otherwise is shown again with
// SetSource, or with Rebase to keep the edits.
//
// A line holds no line break. Lines break between them, so that a source of n
// lines holds n-1 breaks; a text ending in a break has an empty last line.
// Offsets count the bytes of the lines and one byte per break. See
// docs/ADR/0157-editor-line-source.md.
type LineSource interface {
	// Len returns the number of lines known so far, at least one. It never
	// gets smaller while the controller shows the source: a source that reads
	// its last line again, as one following a log does, keeps returning the
	// old number until it has found as many lines.
	Len() int
	// Line returns line i, for i from 0 to Len()-1. The editor asks for the
	// lines in view on every frame, so it should answer from a cache.
	Line(i int) string
	// Offset returns where line i starts, for i from 0 to Len(): the bytes
	// of the lines before it plus one per line before it, so that Offset(Len())
	// is one more than the length of the text.
	Offset(i int) int64
	// LineAt returns the line that holds offset off, for off from 0 to
	// Offset(Len()); offsets past the last line give the last line.
	LineAt(off int64) int
	// Reader returns a reader of the lines for another goroutine, as a
	// search of a long text runs on.
	Reader() LineReader
}

// LineReader reads the lines of a LineSource on any goroutine.
type LineReader interface {
	// ReadLines calls fn with lines from to to-1, in order, until fn returns
	// false. It reads the lines as the source held them when Reader was
	// called.
	ReadLines(from, to int, fn func(i int, line string) bool) error
	// Close lets the reader go.
	Close() error
}

// LineWriter is a LineSource that writes its lines as they are, with their
// own breaks, as \r\n in a file of \r\n. Controller.WriteTo uses it for the
// lines it did not edit; without it, those lines are written as the lines in
// memory are.
//
// The LineReader of a source can be a LineWriter as well, and must then
// write on any goroutine as it reads: Snapshot.WriteTo uses it, and never the
// LineWriter of the source itself, which belongs to the UI goroutine. Its
// lines are those the source held when Reader was called.
type LineWriter interface {
	// WriteLines writes lines from to to-1 to w, each followed by its break
	// but for the last line of the source, which has none.
	WriteLines(w io.Writer, from, to int) error
}

// SourceEdit is an edit of the text against its LineSource: lines From to
// To-1 of the source gave way to Inserted.
type SourceEdit struct {
	From, To int
	// Removed are the lines of the source the edit replaced, and Before and
	// After up to two lines of the source on each side of them, as the text
	// read them when the edit was made, so that the edit can be placed again
	// in a source that changed since: see Controller.Rebase. An edit that
	// replaced more than 4,096 lines keeps none, and Removed is then nil.
	Removed, Before, After []string
	Inserted               []string
}

// PlacedEdit places edit Edit of Controller.Edits at line At of a new
// source, where the lines Removed of the edit now start.
type PlacedEdit struct {
	Edit, At int
}
