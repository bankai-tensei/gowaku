package editor

import (
	"errors"
	"io"
)

// SetSource shows the lines of src in place of the text, without holding
// them: the editor reads the lines it shows, and an edit keeps in memory only
// the lines it changed, so that a file of gigabytes, read where its lines are
// asked for, opens at once. The undo history is forgotten and the caret goes
// to the start, as with SetText. The controller holds src until SetText or
// another SetSource; the application lets it go after.
//
// A text longer than 4 MB, or of a source, is searched on a goroutine of its
// own through LineSource.Reader; see Find. Wrapping lays out every line it
// comes to and keeps a count of rows for each line, which a source of tens of
// millions of lines should leave off. See docs/ADR/0157-editor-line-source.md.
func (c *Controller) SetSource(src LineSource) {
	c.stopFind()
	c.buf.resetSource(src)
	c.restart()
}

// Source returns the source of SetSource, or nil for a text set by SetText.
func (c *Controller) Source() LineSource { return c.buf.src }

// SourceGrew tells that the lines of the source from line from on are new or
// changed, at its end: a source that is still being read found more lines, or
// a log grew and went on its last line. The text grows at its end, after the
// edits, and a search goes on into the new lines. Call it on the UI goroutine.
//
// A source only grows: its Len must never get smaller than it was, or the
// editor reads lines that are gone. SourceGrew panics when it did. A source
// that changed otherwise is shown with SetSource or Rebase; a source that
// reopens its last line, as one following a log does, keeps reporting the old
// Len until it has found as many lines again.
func (c *Controller) SourceGrew(from int) {
	ch, ok := c.buf.grow(from)
	if !ok {
		return
	}
	c.findGrown(ch, from, c.buf.src.Len())
	c.current = -1
	for _, v := range c.views {
		if v.r != nil {
			v.r.linesChanged(ch)
		}
	}
}

// WriteTo writes the text to w: the lines of the source that the text did not
// edit as the source writes them, when it is a LineWriter, so that their
// breaks stay as they were, and the other lines with nl after each but the
// last. It reads every line of the source it writes.
func (c *Controller) WriteTo(w io.Writer, nl string) error { return c.buf.writeTo(w, nl) }

// ReplaceSource makes the text read the lines of src, which hold the text as
// it is now: after the text was written to the file that src reads, as
// WriteTo and a save do. Unlike SetSource it keeps the undo history, the
// selections, the scroll positions and the search, since the text is the
// same, and the edits are no longer edits against the source. It returns
// ErrNotSameText, and changes nothing, when src holds another number of lines
// or bytes than the text.
func (c *Controller) ReplaceSource(src LineSource) error {
	if !c.buf.replaceSource(src) {
		return ErrNotSameText
	}
	return nil
}

// ErrNotSameText is returned by ReplaceSource for a source whose text is not
// that of the controller.
var ErrNotSameText = errors.New("editor: the source does not hold the text")

// Edits returns the edits of the text against its source, in order, or nil for
// a text set by SetText. With the lines they replaced and those around them,
// as the text read them when it made the edits, they let the application
// place them in a source that changed since, as a patch is applied, and show
// the text again with Rebase.
func (c *Controller) Edits() []SourceEdit { return c.buf.edits() }

// Rebase shows the lines of src, a new source for what changed, with the edits
// of Edits that placed places in it: each replaces as many lines of src, from
// line At on, as it replaced of the old source. Edits not placed, placed out
// of order or overlapping an earlier one are left out. The undo history is
// forgotten, as its positions belong to the old text; the selections and the
// scroll positions move with their lines, as far as the edits before them
// moved, and stay on their columns.
func (c *Controller) Rebase(src LineSource, placed []PlacedEdit) {
	c.stopFind()
	edits := c.buf.edits()
	// The new lines of the selections and the scroll positions, found on the
	// pieces as they are before the edits move.
	move := c.buf.lineMap(edits, placed, src.Len())
	moved := func(p TextPosition) TextPosition { return TextPosition{Line: move(p.Line), Column: p.Column} }
	type kept struct {
		v             *View
		anchor, caret TextPosition
		scroll        ScrollPosition
	}
	views := make([]kept, 0, len(c.views))
	for _, v := range c.views {
		scroll := v.Scroll()
		scroll.Line = move(scroll.Line)
		views = append(views, kept{v: v, anchor: moved(v.sel.anchor), caret: moved(v.sel.caret), scroll: scroll})
	}
	base := selection{anchor: moved(c.base.anchor), caret: moved(c.base.caret)}
	c.buf.rebase(src, edits, placed)
	c.history.Clear()
	c.refind()
	c.current = -1
	c.base = selection{anchor: c.buf.clamp(base.anchor), caret: c.buf.clamp(base.caret)}
	for _, k := range views {
		k.v.sel = selection{anchor: c.buf.clamp(k.anchor), caret: c.buf.clamp(k.caret)}
		k.v.told = k.v.sel
		if k.v.r != nil {
			k.v.r.textReset()
		}
		k.v.SetScroll(k.scroll)
	}
}

// follow takes the Poster of the State of an editor that shows the
// controller, and the matches a search found meanwhile.
func (c *Controller) follow(post func(func())) {
	c.post.Store(&post)
	c.applyFind()
}
