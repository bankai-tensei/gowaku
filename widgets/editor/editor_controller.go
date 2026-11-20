package editor

import (
	"slices"
	"sync/atomic"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Controller holds the text of a Editor, its selection, its undo
// history and its search. The zero value is an empty document, ready to use;
// keep it where it outlives a build, such as in a State.
//
// Several Editors can show one Controller at once. They share the text, the
// history, the search and the decorations, while each has its own selection
// and scroll position, held in its View. The selection methods of the
// Controller (Selection, SetSelection, Insert, FindNext and the like) act on
// the primary view: the one the user last focused, clicked or typed in, and
// before that the first one shown. View.SetSelection selects in a given view.
//
// The text lives here and not in the widget, so that a document of 100,000
// lines does not pass through a string on every build: the editor reads the
// lines it shows, and Text joins them only when asked.
//
// Positions count lines from 0 and bytes within a line. Methods that take one
// move it into the text and to the start of a character.
type Controller struct {
	buf     lineBuffer
	history kit.History
	find    finder
	// current is the index of the match the selection is on, or -1.
	current int
	// typed is the query as the find bars show it, even one that did not
	// compile, which badQuery then says.
	typed    string
	badQuery bool
	// decoGen counts the calls of Redecorate, and decoFirst and decoLast the
	// lines that RedecorateLines asked for since the editor last looked.
	decoGen uint64
	// views are the views that editors show, in the order they came.
	// primary is the one the selection methods act on, and nil while no
	// editor shows the controller, when base holds the selection instead.
	views   []*View
	primary *View
	base    selection
	// revealBase means base was set by code while no editor showed the
	// controller, and the first view to take it scrolls to its caret.
	revealBase bool
	// quiet is above 0 while a change of the selection may come during a
	// layout, when OnSelectionChange waits for the next frame.
	quiet int
	// post runs work on the UI goroutine, through the State of an editor
	// that shows the controller; nil while none does.
	post atomic.Pointer[func(func())]
}

// cur returns the selection of the primary view.
func (c *Controller) cur() *selection {
	if c.primary != nil {
		return &c.primary.sel
	}
	return &c.base
}

// attach adds v, which an editor starts to show. A view without a selection of
// its own takes the selection of the document.
func (c *Controller) attach(v *View) {
	if v.ctl == c {
		return
	}
	if v.ctl != nil {
		v.ctl.detach(v)
	}
	if v.doc != nil && v.doc != c {
		// Positions belong to a document: in another one the view starts
		// over, at the selection of that document and at its top. A view
		// never shown keeps what SetSelection and SetScroll gave it.
		v.own, v.hasPending, v.reveal = false, false, false
	}
	v.doc = c
	if !v.own {
		v.sel = *c.cur()
		// A selection that SetSelection made before any editor showed the
		// document comes into view with it.
		v.reveal = c.revealBase
		c.revealBase = false
	}
	v.sel = selection{anchor: c.buf.clamp(v.sel.anchor), caret: c.buf.clamp(v.sel.caret)}
	// Showing the view is not a move that OnSelectionChange tells of.
	v.told = v.sel
	v.own = true
	v.ctl = c
	c.views = append(c.views, v)
	if c.primary == nil {
		c.primary = v
	}
}

// detach removes v, which no editor shows any more. The selection of the last
// view stays with the document.
func (c *Controller) detach(v *View) {
	if i := slices.Index(c.views, v); i >= 0 {
		c.views = slices.Delete(c.views, i, i+1)
	}
	v.ctl = nil
	if c.primary != v {
		return
	}
	c.primary = nil
	if n := len(c.views); n > 0 {
		c.primary = c.views[n-1]
	} else {
		c.base = v.sel
	}
}

// activate makes v the primary view, as when the user works in it.
func (c *Controller) activate(v *View) {
	if v != nil && v.ctl == c {
		c.primary = v
	}
}

// Text returns the whole text, with \n between lines. It joins the lines once
// per edit, which for a large document is worth avoiding on every keystroke.
func (c *Controller) Text() string { return c.buf.text() }

// SetText replaces the whole text and forgets the undo history. The caret goes
// to the start. Line breaks may be \n, \r\n or \r; they become \n.
func (c *Controller) SetText(s string) {
	c.stopFind()
	c.buf.reset(s)
	c.restart()
}

// restart follows a new text: the caret at the start, no history, the search
// and the views anew.
func (c *Controller) restart() {
	c.base = selection{}
	c.revealBase = false
	c.history.Clear()
	c.refind()
	c.current = -1
	for _, v := range c.views {
		v.sel, v.told = selection{}, selection{}
		v.hasPending = false
		if v.r != nil {
			v.r.textReset()
		}
	}
}

// LineCount returns the number of lines, which is at least 1.
func (c *Controller) LineCount() int { return c.buf.lineCount() }

// Line returns line i without its line break. It panics if i is out of range.
func (c *Controller) Line(i int) string {
	if i < 0 || i >= c.buf.lineCount() {
		panic("gowaku: TextEditorController.Line index out of range")
	}
	return c.buf.line(i)
}

// Len returns the length of the text in bytes.
func (c *Controller) Len() int { return c.buf.length() }

// Offset returns p as a byte offset into Text.
func (c *Controller) Offset(p TextPosition) int { return c.buf.offset(c.buf.clamp(p)) }

// PositionAt returns the position of the byte offset off into Text.
func (c *Controller) PositionAt(off int) TextPosition { return c.buf.position(off) }

// TextIn returns the text of r.
func (c *Controller) TextIn(r TextRange) string {
	r = c.clampRange(r).Ordered()
	return c.buf.slice(r.Start, r.End)
}

// Selection returns the selection of the primary view: Start is the anchor
// and End the caret.
func (c *Controller) Selection() TextRange { return c.cur().textRange() }

// SelectedText returns the text of the selection.
func (c *Controller) SelectedText() string { return c.TextIn(c.Selection()) }

// SetSelection selects r in the primary view, Start being the anchor and End
// the caret, and scrolls the caret into view. While no editor shows the
// document, the first one to show it scrolls to the caret on its first layout.
func (c *Controller) SetSelection(r TextRange) {
	r = c.clampRange(r)
	c.choose(r.Start, r.End)
}

func (c *Controller) clampRange(r TextRange) TextRange {
	return TextRange{Start: c.buf.clamp(r.Start), End: c.buf.clamp(r.End)}
}

// select moves the selection, ending a run of typing for the history.
func (c *Controller) choose(anchor, caret TextPosition) { c.chooseIn(c.primary, anchor, caret) }

// chooseIn moves the selection of v, or of the document while no editor shows
// it if v is nil.
func (c *Controller) chooseIn(v *View, anchor, caret TextPosition) {
	c.history.Seal()
	s := &c.base
	if v != nil {
		s = &v.sel
	}
	s.anchor, s.caret = anchor, caret
	c.current = -1
	if v == nil {
		c.revealBase = true
	}
	if v != nil {
		if v.r != nil {
			v.r.selectionChanged()
		}
		v.tellSelection()
	}
}

// Insert replaces the selection with s, as a paste does, and puts the caret
// after it. It is undoable.
func (c *Controller) Insert(s string) {
	sel := c.Selection().Ordered()
	c.replace(sel.Start, sel.End, s, kit.EditOther)
}

// Replace puts s in place of the text of r. It is undoable. A selection before
// r stays, one after it moves with its text, and one that overlaps it ends up
// as a caret after s.
func (c *Controller) Replace(r TextRange, s string) {
	r = c.clampRange(r).Ordered()
	sel := c.cur()
	anchor, caret := sel.anchor, sel.caret
	end := c.edit(r.Start, r.End, s, kit.EditOther)
	switch {
	case !r.Start.Before(maxPos(anchor, caret)):
		sel.anchor, sel.caret = anchor, caret
	case !minPos(anchor, caret).Before(r.End):
		sel.anchor, sel.caret = shiftPos(anchor, r.End, end), shiftPos(caret, r.End, end)
	default:
		sel.anchor, sel.caret = end, end
	}
	c.changed()
}

// replace puts s in place of the text from a to z, which come in order, and
// leaves the caret after it.
func (c *Controller) replace(a, z TextPosition, s string, kind kit.EditKind) {
	if a == z && s == "" {
		return
	}
	end := c.edit(a, z, s, kind)
	sel := c.cur()
	sel.anchor, sel.caret = end, end
	c.changed()
}

// edit changes the text and records the step, without touching the
// selection or telling anyone.
func (c *Controller) edit(a, z TextPosition, s string, kind kit.EditKind) TextPosition {
	// The history holds the text as the buffer does, with \n for every line
	// break, or Undo would take the length of the breaks that went away from
	// the text after s.
	s = normalizeBreaks(s)
	sel := c.cur()
	c.history.Record(kit.Edit{
		Kind: kind, At: c.buf.offset(a), Removed: c.buf.slice(a, z), Inserted: s,
		Anchor: c.buf.offset(sel.anchor), Caret: c.buf.offset(sel.caret),
	})
	return c.splice(a, z, s)
}

// splice changes the text and brings the search and the views along. The
// selections of the views but the primary one move with their text; the caller
// places the primary one.
func (c *Controller) splice(a, z TextPosition, s string) TextPosition {
	end, ch := c.buf.replace(a, z, s)
	c.find.splice(&c.buf, ch)
	c.current = -1
	for _, v := range c.views {
		if v != c.primary {
			v.sel.anchor = shiftEdited(v.sel.anchor, a, z, end)
			v.sel.caret = shiftEdited(v.sel.caret, a, z, end)
		}
		if v.r != nil {
			v.r.linesChanged(ch)
		}
	}
	return end
}

// changed reports a complete edit, selection included. Only the primary view
// follows its caret.
func (c *Controller) changed() {
	for _, v := range c.views {
		if v.r != nil {
			v.r.textChanged(v == c.primary)
		}
	}
	// After every OnChange, so that the text is new; an edit can move the
	// selections of the other views too.
	for _, v := range c.views {
		v.tellSelection()
	}
}

// shiftPos moves p, which lies after the old end of an edit, to where it is
// after the edit ended at end.
func shiftPos(p, oldEnd, end TextPosition) TextPosition {
	if p.Line == oldEnd.Line {
		return TextPosition{Line: end.Line, Column: end.Column + p.Column - oldEnd.Column}
	}
	return TextPosition{Line: p.Line + end.Line - oldEnd.Line, Column: p.Column}
}

func minPos(a, b TextPosition) TextPosition {
	if b.Before(a) {
		return b
	}
	return a
}

func maxPos(a, b TextPosition) TextPosition {
	if a.Before(b) {
		return b
	}
	return a
}

// CanUndo reports whether Undo has a step to take back.
func (c *Controller) CanUndo() bool { return len(c.history.Undo) > 0 }

// CanRedo reports whether Redo has a step to repeat.
func (c *Controller) CanRedo() bool { return len(c.history.Redo) > 0 }

// Undo takes back the last step and restores the selection before it. Typing
// counts as one step per word.
func (c *Controller) Undo() {
	e, ok := c.history.TakeUndo()
	if !ok {
		return
	}
	a, z := c.buf.position(e.At), c.buf.position(e.At+len(e.Inserted))
	c.splice(a, z, e.Removed)
	sel := c.cur()
	sel.anchor, sel.caret = c.buf.position(e.Anchor), c.buf.position(e.Caret)
	c.changed()
}

// Redo repeats the last step that Undo took back.
func (c *Controller) Redo() {
	e, ok := c.history.TakeRedo()
	if !ok {
		return
	}
	a, z := c.buf.position(e.At), c.buf.position(e.At+len(e.Removed))
	end := c.splice(a, z, e.Inserted)
	sel := c.cur()
	sel.anchor, sel.caret = end, end
	c.changed()
}

// Find searches for query and highlights every match, and returns how many it
// found. An empty query ends the search. The matches follow later edits. A
// regular expression that does not compile ends the search and returns the
// error.
//
// A text longer than 4 MB, or of SetSource, is searched on a goroutine
// of its own, so that the editor goes on answering: Find returns 0 at once,
// and the matches come in as they are found, which Searching reports, while
// the find bar counts them. Edits made meanwhile move them as they move the
// others.
func (c *Controller) Find(query string, opts FindOptions) (int, error) {
	c.stopFind()
	err := c.find.set(query, opts)
	c.typed, c.badQuery = query, err != nil
	c.refind()
	c.current = c.find.index(c.Selection())
	c.findChanged()
	return len(c.find.matches), err
}

// endFind ends the search, which cannot fail.
func (c *Controller) endFind() {
	c.stopFind()
	c.find.query, c.find.matches = "", c.find.matches[:0]
	c.typed, c.badQuery = "", false
	c.findChanged()
}

// findChanged repaints every view, as all of them show the matches, and
// brings every open find bar to the search, which is one for the document.
func (c *Controller) findChanged() {
	for _, v := range c.views {
		if v.r != nil {
			v.r.findChanged()
		}
	}
}

// Matches returns the matches of the search, in order. The slice belongs to
// the controller and changes with the next edit.
func (c *Controller) Matches() []TextRange { return c.find.matches }

// CurrentMatch returns the index into Matches of the match that the primary
// view has selected, or -1.
func (c *Controller) CurrentMatch() int { return c.currentMatchOf(c.cur()) }

// currentMatchOf returns the index of the match that s selects, or -1.
func (c *Controller) currentMatchOf(s *selection) int {
	sel := s.textRange()
	if c.current >= 0 && c.current < len(c.find.matches) && c.find.matches[c.current] == sel.Ordered() {
		return c.current
	}
	return c.find.index(sel)
}

// FindNext selects the next match after the caret, wrapping around at the end,
// and reports whether there was one.
func (c *Controller) FindNext() bool {
	sel := c.Selection().Ordered()
	from := sel.End
	if sel.Empty() {
		from = sel.Start
	}
	return c.selectMatch(c.find.next(from))
}

// FindPrevious selects the match before the selection, wrapping around at the
// start, and reports whether there was one.
func (c *Controller) FindPrevious() bool {
	return c.selectMatch(c.find.previous(c.Selection().Ordered().Start))
}

func (c *Controller) selectMatch(k int) bool {
	if k < 0 {
		return false
	}
	m := c.find.matches[k]
	c.choose(m.Start, m.End)
	c.current = k
	return true
}

// ReplaceMatch replaces the selected match with s, then selects the next match.
// If the selection is not a match it only selects the next one. With
// FindOptions.Regexp, $1 and the like in s stand for the groups of the match.
// It reports whether it replaced anything.
func (c *Controller) ReplaceMatch(s string) bool {
	k := c.CurrentMatch()
	if k < 0 {
		c.FindNext()
		return false
	}
	m := c.find.matches[k]
	if with := c.find.expand(&c.buf, c.find.matches[k:k+1], s); with != nil {
		s = with[0]
	}
	c.replace(m.Start, m.End, s, kit.EditOther)
	c.FindNext()
	return true
}

// ReplaceAll replaces every match with s and returns how many it replaced. One
// Undo takes all of it back.
func (c *Controller) ReplaceAll(s string) int {
	n := len(c.find.matches)
	if n == 0 {
		return 0
	}
	// A copy, as refind fills the matches anew in place.
	matches := slices.Clone(c.find.matches)
	first := c.buf.offset(matches[0].Start)
	sel := c.cur()
	anchor, caret := c.buf.offset(sel.anchor), c.buf.offset(sel.caret)
	// Expanded before the text changes: a replacement changes what the regular
	// expression sees around the matches before it.
	with := c.find.expand(&c.buf, matches, s)
	before, after, ends := c.replaceLines(matches, with, s)
	c.refind()
	c.history.Seal()
	c.history.Record(kit.Edit{Kind: kit.EditOther, At: first, Removed: before, Inserted: after, Anchor: anchor, Caret: caret})
	c.history.Seal()
	sel.anchor, sel.caret = ends[n-1], ends[n-1]
	c.changed()
	return n
}

// Redecorate asks the editors of the controller to call Editor.Decorations
// and Editor.LineStyle again for every line, as after a spell checker
// finished. Lines that change are decorated again by themselves.
func (c *Controller) Redecorate() {
	c.decoGen++
	c.RedecorateLines(0, c.buf.lineCount()-1)
}

// RedecorateLines asks the editors of the controller to call
// Editor.Decorations and Editor.LineStyle again for the lines from first to
// last, as after a highlighter learned that a comment opened on first runs on
// to last, or a diff marked them added.
func (c *Controller) RedecorateLines(first, last int) {
	for _, v := range c.views {
		if v.r != nil {
			v.r.redecorate(first, last)
		}
	}
}
