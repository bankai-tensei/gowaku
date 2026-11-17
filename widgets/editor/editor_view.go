package editor

// ScrollPosition is where a view of a document has scrolled to. It names a
// line rather than a pixel offset into the whole document, so that it stays on
// the same text while lines above it are measured for the first time, wrap
// anew after a resize, or come and go with an edit.
type ScrollPosition struct {
	// Line is the first line in view, from 0. A wrapped line counts once.
	Line int
	// Offset is how far the top of the view is below the top of Line, in
	// pixels. It is 0 when the line starts at the top of the view, and with
	// Wrap it can be more than a row. The first line starts at the top of the
	// content, the room above it included, so the very top is {0, 0, x}.
	Offset float32
	// X is how far the text has scrolled left, in pixels. It is 0 with Wrap.
	X float32
}

// selection is the anchor and the caret of a view.
type selection struct {
	anchor, caret TextPosition
}

// textRange returns the selection as a TextRange.
func (s *selection) textRange() TextRange { return TextRange{Start: s.anchor, End: s.caret} }

// View is one view of the document of a Controller: its selection and its
// scroll position. The text, the undo history, the search and the decorations
// belong to the Controller and are shared by all its views; each Editor that
// shows the Controller has a view of its own. The zero value is ready to use;
// keep it where it outlives a build, such as in a State, and pass it to
// Editor.View.
//
// An Editor without a View keeps a private one, which is all a single editor
// needs. Give it a View to read or set its scroll position, to follow it with
// OnScroll, or to select in that editor rather than in the one the user last
// worked in:
//
//	var doc editor.Controller
//	var left, right editor.View
//	left.OnScroll = func(p editor.ScrollPosition) { right.SetScroll(p) }
//	right.OnScroll = func(p editor.ScrollPosition) { left.SetScroll(p) }
//	// ... editor.Editor{Controller: &doc, View: &left}, editor.Editor{Controller: &doc, View: &right}
//
// A View shows in one Editor at a time; showing it in two at once panics. It
// keeps its selection and its scroll position while it is not shown, and gets
// them back when it shows the same Controller again, in the same Editor or
// another one, so a View per tab restores the place of each tab. Shown with
// another Controller, it starts over at the selection of that document and at
// its top.
type View struct {
	// OnScroll, if set, runs after the scroll position changed: by the wheel
	// or the scroll bar, to follow the caret, or because a resize or an edit
	// moved the lines. It does not run for a position that SetScroll set, even
	// one that had to be clamped into the document, so two views that set each
	// other's position from OnScroll do not chase each other. A change during
	// layout, such as following the caret, is told at the start of the next
	// frame.
	OnScroll func(ScrollPosition)
	// OnSelectionChange, if set, runs after the anchor or the caret of this
	// view moved: by a key, the pointer, an edit, undo or redo, a screen
	// reader, the find bar, or code (Controller.SetSelection, View.SetSelection,
	// FindNext and the like, when they act on this view). An edit in another
	// view that moves this selection along with its text runs it too. It
	// gets Selection, and runs after OnChange for an edit, so that the text
	// is new. It does not run for SetText, nor when an Editor starts to show
	// the view, and not for a selection it told last. A change made while the
	// tree is laid out, such as the commit of an IME composition when a find
	// bar takes the keyboard, is told at the start of the next frame.
	OnSelectionChange func(TextRange)

	sel selection
	// told is the selection OnSelectionChange last told of.
	told selection
	// own means the view has a selection of its own, set through SetSelection
	// or kept from an earlier Controller, which it keeps when it is attached.
	// A fresh view takes the selection that the document already has.
	own bool
	// ctl is the Controller the view is attached to, and r the editor that
	// shows it, or nil. doc is the Controller it was last attached to.
	ctl, doc *Controller
	r        *renderEditor
	// pending is a scroll position to go to on the next layout, from
	// SetScroll or kept while the view was not shown.
	pending    ScrollPosition
	hasPending bool
	// kept means pending is the place the view had when it was last shown,
	// which a SetSelection made since replaces with the caret.
	kept bool
	// reveal asks the next layout of the editor that shows the view to
	// scroll the caret into view, for a selection set while it was not shown.
	reveal bool
	// focus asks the editor that shows the view for the keyboard once it is
	// laid out.
	focus bool
	// reported is the position OnScroll last told, or SetScroll last set.
	reported ScrollPosition
}

// Controller returns the Controller of the Editor that shows the view, or nil.
func (v *View) Controller() *Controller { return v.ctl }

// Selection returns the selection of the view: Start is the anchor and End
// the caret.
func (v *View) Selection() TextRange { return v.sel.textRange() }

// SetSelection selects r in this view, Start being the anchor and End the
// caret, and scrolls the caret into view. Unlike Controller.SetSelection, it
// does not depend on which view the user last worked in.
//
// On a view that no editor shows yet, as when a file opens at a line, the
// selection waits for the editor, and the first layout that shows the view
// scrolls the caret into view, over the place the view had when it was last
// shown. A SetScroll wins over the caret, as it does on a view that shows.
func (v *View) SetSelection(r TextRange) {
	if v.ctl == nil {
		v.sel = selection{anchor: r.Start, caret: r.End}
		v.own = true
		v.reveal = true
		if v.kept {
			v.hasPending, v.kept = false, false
		}
		v.tellSelection()
		return
	}
	r = v.ctl.clampRange(r)
	v.ctl.chooseIn(v, r.Start, r.End)
}

// tellSelection runs OnSelectionChange if the selection is not the one it last
// told of. While the controller is quiet, the editor of the view tells it on
// the next frame instead.
func (v *View) tellSelection() {
	if v.sel == v.told {
		return
	}
	if v.ctl != nil && v.ctl.quiet > 0 {
		if v.r != nil {
			v.r.tellLater()
		}
		return
	}
	v.told = v.sel
	if f := v.OnSelectionChange; f != nil {
		f(v.sel.textRange())
	}
}

// Scroll returns the scroll position of the view.
func (v *View) Scroll() ScrollPosition {
	switch {
	case v.hasPending:
		return v.pending
	case v.r != nil && v.r.rowsValid:
		return v.r.scrollPosition()
	}
	return v.reported
}

// SetScroll scrolls the view to p on the next layout, clamped into the
// document. It does not run OnScroll.
func (v *View) SetScroll(p ScrollPosition) {
	p.Line = max(p.Line, 0)
	p.X = max(p.X, 0)
	v.pending, v.hasPending, v.kept = p, true, false
	if v.r != nil {
		v.r.MarkNeedsLayout()
		v.r.MarkNeedsPaint()
	}
}

// Focus gives the keyboard to the editor that shows the view: now if it is
// laid out, or else when it next is, as for an editor that is about to be
// built. A view that no editor shows keeps the request until one does. To
// focus an Editor from code, give it a View and call Focus on it.
func (v *View) Focus() {
	if r := v.r; r != nil && r.rowsValid && r.Attached() {
		v.focus = false
		r.RequestFocus()
		return
	}
	v.focus = true
	if v.r != nil {
		v.r.MarkNeedsLayout()
	}
}

// Focused reports whether the editor that shows the view has the keyboard.
func (v *View) Focused() bool { return v.r != nil && v.r.focused }

// shiftEdited moves p to where it is after the text from a to z, which come in
// order, became text that ends at end: a position before the edit stays, one
// after it moves with its text, and one inside it goes to the end.
func shiftEdited(p, a, z, end TextPosition) TextPosition {
	switch {
	case !a.Before(p):
		return p
	case !p.Before(z):
		return shiftPos(p, z, end)
	}
	return end
}
