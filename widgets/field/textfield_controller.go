package field

import (
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// InputController reaches a Input from code: it reads and sets the
// selection, edits the text as the user would, and gives the field the
// keyboard. The zero value is ready; keep it where it outlives a build, such
// as in a State, and put it in Input.Controller. Its methods are for
// callbacks and InitState, not for Build, as they run the callbacks of the
// field. The exception is a controller that no field shows yet, as that of a
// field a Build is about to make: SetSelection and Focus then run nothing,
// and the field takes the selection when made and the keyboard once laid out.
//
// Offsets are bytes of Text, the text as written: with the ruby notation of
// Input.Ruby, and without what an IME is still converting. Methods that
// take an offset keep it within the text and move it back to the start of a
// character.
//
// The controller remembers the selection while no field shows it, and a field
// that comes to show it takes that selection, so that a field built again for
// the same text, as when a tab comes back, keeps its caret. Methods that edit
// do nothing while no field shows the controller.
//
// The text stays the field's: a new Value replaces it as before, and clears
// the undo history. The selection then stays where it was, kept within the
// new text, and OnSelectionChange does not run, as the parent made the change.
type InputController struct {
	// view is the field that shows the controller, or nil.
	view *RenderEditable
	// anchor and caret are the selection while no field shows it.
	anchor, caret int
	// text is the text the field last had.
	text string
	// focus asks the next field to lay out to take the keyboard.
	focus bool
	// selectAll asks the next field to show the controller to select its
	// whole text, which the controller cannot know before.
	selectAll bool
}

// field returns the field that shows the controller, if it is in a tree.
func (c *InputController) field() *RenderEditable {
	if c.view != nil && c.view.Attached() {
		return c.view
	}
	return nil
}

// attach makes r the field of the controller. The field takes the selection
// the controller remembers.
func (c *InputController) attach(r *RenderEditable) {
	if c.view == r {
		return
	}
	c.view = r
	if c.selectAll {
		c.selectAll = false
		c.anchor, c.caret = 0, len(r.Content)
	}
	r.Anchor, r.Caret = charStart(r.Content, c.anchor), charStart(r.Content, c.caret)
	r.reported = [2]int{r.Anchor, r.Caret}
	c.text = r.Content
	if c.focus {
		r.MarkNeedsLayout()
	}
}

// detach lets go of r if it is the field of the controller.
func (c *InputController) detach(r *RenderEditable) {
	if c.view == r {
		c.view = nil
	}
}

// remember keeps the state of r, for a field that shows the controller later.
func (c *InputController) remember(r *RenderEditable) {
	if c.view == r {
		c.anchor, c.caret, c.text = r.Anchor, r.Caret, r.Content
	}
}

// Text returns the text of the field, which runs ahead of Value until the
// parent passes OnChange back. Without a field it is the text the last field
// had.
func (c *InputController) Text() string {
	if r := c.field(); r != nil {
		return r.Content
	}
	return c.text
}

// Selection returns the selection: anchor is the end where it started, and
// caret the end that moves, which comes first in a selection made backwards.
// They are equal when nothing is selected.
func (c *InputController) Selection() (anchor, caret int) {
	if r := c.field(); r != nil {
		return r.Anchor, r.Caret
	}
	return c.anchor, c.caret
}

// SetSelection selects from anchor to caret and scrolls the caret into view.
// Equal offsets put the caret there. An end inside a ruby mark shows its
// notation, as the arrow keys do. What an IME was converting is kept first.
func (c *InputController) SetSelection(anchor, caret int) {
	r := c.field()
	if r == nil {
		c.anchor, c.caret = max(anchor, 0), max(caret, 0)
		c.selectAll = false
		return
	}
	r.commitComposition()
	r.Anchor = charStart(r.Content, anchor)
	r.goalOK = false
	r.MoveTo(charStart(r.Content, caret), true)
}

// SelectAll selects the whole text. Called before a field shows the
// controller, it selects the whole text that field is built with, as for the
// field of a rename dialog:
//
//	s.name.SelectAll()
//	s.name.Focus()
//	overlay.ShowDialog(ctx, func(close func()) core.Widget {
//		return overlay.Dialog{Title: "Rename", Content: field.Input{Value: old, Controller: &s.name}}
//	})
func (c *InputController) SelectAll() {
	if c.field() == nil {
		c.SetSelection(0, len(c.text))
		c.selectAll = true
		return
	}
	c.SetSelection(0, len(c.Text()))
}

// SelectedText returns the text of the selection.
func (c *InputController) SelectedText() string {
	a, b := c.Selection()
	s := c.Text()
	a, b = charStart(s, a), charStart(s, b)
	return s[min(a, b):max(a, b)]
}

// Focus gives the field the keyboard: now if it is laid out, or else when it
// next is, as for a field that is about to be built. A field asked before it
// is built keeps the keyboard over the autofocus of the dialog or sheet it
// opens in, so Focus may come before or right after overlay.ShowDialog.
func (c *InputController) Focus() {
	if r := c.field(); r != nil && r.laidOut != nil {
		c.focus = false
		r.RequestFocus()
		return
	}
	c.focus = true
	if c.view != nil && c.view.Attached() {
		c.view.MarkNeedsLayout()
	}
}

// Focused reports whether the field has the keyboard.
func (c *InputController) Focused() bool {
	r := c.field()
	return r != nil && r.focused
}

// Replace puts s in place of the text from start to end, as one step that undo
// takes back, and runs OnChange. A selection before the stretch stays, one
// after it moves with its text, and one that overlaps it ends up as a caret
// after s. A single-line field turns line breaks in s into spaces.
//
// Replacing the stretch from the first match to the last at once is how to
// replace all matches of a search as one step.
func (c *InputController) Replace(start, end int, s string) {
	r := c.field()
	if r == nil {
		return
	}
	r.commitComposition()
	start, end = charStart(r.Content, start), charStart(r.Content, end)
	if start > end {
		start, end = end, start
	}
	s = r.sanitize(s)
	if start == end && s == "" {
		return
	}
	anchor, caret := r.Anchor, r.Caret
	r.history.Seal()
	r.history.Record(kit.Edit{Kind: kit.EditOther, At: start, Removed: r.Content[start:end], Inserted: s, Anchor: anchor, Caret: caret})
	r.history.Seal()
	r.splice(start, end, s)
	shift := func(o int) int { return o + len(s) - (end - start) }
	switch {
	case max(anchor, caret) <= start:
	case min(anchor, caret) >= end:
		r.Anchor, r.Caret = shift(anchor), shift(caret)
	default:
		r.Anchor, r.Caret = start+len(s), start+len(s)
	}
	r.changed()
}

// Insert puts s in place of the selection, as a paste does, and puts the caret
// after it.
func (c *InputController) Insert(s string) {
	if r := c.field(); r != nil {
		r.commitComposition()
		r.insert(s, kit.EditOther)
	}
}

// CanUndo reports whether Undo has a step to take back.
func (c *InputController) CanUndo() bool {
	r := c.field()
	return r != nil && len(r.history.Undo) > 0
}

// CanRedo reports whether Redo has a step to repeat.
func (c *InputController) CanRedo() bool {
	r := c.field()
	return r != nil && len(r.history.Redo) > 0
}

// Undo takes back the last step, as Ctrl+Z does.
func (c *InputController) Undo() {
	if r := c.field(); r != nil {
		r.commitComposition()
		r.undo()
	}
}

// Redo repeats the last step that Undo took back.
func (c *InputController) Redo() {
	if r := c.field(); r != nil {
		r.commitComposition()
		r.redo()
	}
}

// Copy puts the selection on the clipboard.
func (c *InputController) Copy() {
	if r := c.field(); r != nil {
		if a, b := r.Selection(); a != b && !r.field.Obscure {
			r.Host().SetClipboardText(r.Content[a:b])
		}
	}
}

// Cut puts the selection on the clipboard and removes it, as one step.
func (c *InputController) Cut() {
	if r := c.field(); r != nil {
		r.commitComposition()
		if a, b := r.Selection(); a != b && !r.field.Obscure {
			r.Host().SetClipboardText(r.Content[a:b])
			r.replace(a, b, "", kit.EditOther)
		}
	}
}

// Paste puts the text of the clipboard in place of the selection.
func (c *InputController) Paste() {
	if r := c.field(); r != nil {
		c.Insert(r.Host().ClipboardText())
	}
}

// charStart keeps offset i within s and moves it back to the start of a
// character.
func charStart(s string, i int) int {
	i = min(max(i, 0), len(s))
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}
