package collection

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
)

// editKey handles a key while the editor of a cell is open, after the editor
// let it go.
func (c *tableCore) editKey(e *core.KeyEvent) bool {
	switch e.Key {
	case input.KeyEscape:
		c.cancelEdit()
		return true
	case input.KeyEnter:
		if e.Modifiers != 0 {
			return false
		}
		c.commitEdit(true)
		return true
	case input.KeyTab:
		if e.Modifiers&^input.ModShift != 0 {
			return false
		}
		c.tabEdit(e.Modifiers&input.ModShift != 0)
		return true
	}
	return false
}

// startEdit opens the editor of the cell of row and column j, in the order
// they show, if the column is editable.
func (c *tableCore) startEdit(row, j int) {
	if row < 0 || row >= c.cfg.rows || j < 0 || j >= len(c.cols) || !c.cols[j].editable {
		return
	}
	if c.editing && !c.commitEdit(false) {
		return
	}
	h := c.cols[j]
	c.ctrl.row, c.ctrl.col = row, h.model
	c.reveal(row, j)
	c.editing = true
	c.editRow, c.editCol = row, h.model
	c.editText = c.src.cellText(row, h.model)
	c.editStart = c.editText
	c.editSeq++
	c.changed()
}

// typed takes text typed while a cell, not an editor, has the keyboard. Text
// typed on an editable cell starts editing it with that text in place of its
// own, as in a spreadsheet; text typed after Enter or F2 but before the editor
// took the keyboard goes into the editor.
func (c *tableCore) typed(s string) {
	if c.editing {
		c.editStart += s
		c.editText += s
		c.changed()
		return
	}
	if strings.TrimSpace(s) == "" {
		// Space selects; it does not start an edit.
		return
	}
	row, j := c.focusRow(), c.focusCol()
	if row < 0 || j < 0 || !c.cols[j].editable {
		return
	}
	c.startEdit(row, j)
	if c.editing {
		c.editStart, c.editText = s, s
	}
}

// editor builds the editor of the open cell.
func (c *tableCore) editor() core.Widget {
	j := c.displayOf(c.editCol)
	ed := TableEditor{
		Row: c.editRow, Column: c.editCol, Text: c.editStart,
		Change: func(s string) { c.editText = s },
		Commit: func() { c.commitEdit(true) },
		Cancel: c.cancelEdit,
	}
	if j >= 0 && c.cols[j].editor != nil {
		return c.cols[j].editor(ed)
	}
	// The title of the column names the field to a screen reader; the
	// header above already shows it.
	var title string
	if j >= 0 {
		title = c.cols[j].title
	}
	return field.Input{Value: c.editStart, Label: title, HideLabel: true, OnChange: ed.Change, OnSubmit: func(string) { c.commitEdit(true) }}
}

// commitEdit hands the edit to OnCommit and closes the editor, unless OnCommit
// refuses it. It reports whether the editor closed. refocus gives the keyboard
// back to the cell.
func (c *tableCore) commitEdit(refocus bool) bool {
	if !c.editing {
		return true
	}
	e := TableEdit{Row: c.editRow, Column: c.editCol, Text: c.editText}
	if c.cfg.onCommit != nil && !c.cfg.onCommit(e) {
		return false
	}
	c.editing = false
	c.refocus = refocus
	c.changed()
	return true
}

// cancelEdit closes the editor without a change.
func (c *tableCore) cancelEdit() {
	if !c.editing {
		return
	}
	c.editing = false
	c.refocus = true
	c.changed()
	if c.cfg.onCancel != nil {
		c.cfg.onCancel(c.editRow, c.editCol)
	}
}

// editorLeft follows the keyboard out of the editor, to elsewhere in the
// window: the edit is committed, or dropped if OnCommit refuses it.
func (c *tableCore) editorLeft() {
	if !c.editing {
		return
	}
	if !c.commitEdit(false) {
		c.editing = false
		c.changed()
		if c.cfg.onCancel != nil {
			c.cfg.onCancel(c.editRow, c.editCol)
		}
	}
}

// tabEdit commits the edit and opens the next editable cell, or the one
// before with back, along the row and then the rows.
func (c *tableCore) tabEdit(back bool) {
	row, j := c.editRow, c.displayOf(c.editCol)
	step := 1
	if back {
		step = -1
	}
	for range 2 {
		for j += step; j >= 0 && j < len(c.cols); j += step {
			if c.cols[j].editable {
				if c.commitEdit(false) {
					c.startEdit(row, j)
				}
				return
			}
		}
		row += step
		if row < 0 || row >= c.cfg.rows {
			break
		}
		j = -1
		if back {
			j = len(c.cols)
		}
	}
	c.commitEdit(true)
}

// focusFirst gives the keyboard to the first object below it that takes it,
// when it first lays out, and reports when the keyboard leaves it. It holds
// the editor of a cell, which may be any widget, and the filter of a column.
type focusFirst struct {
	onLeave func()
	child   core.Widget
}

// Key implements core.Widget.
func (focusFirst) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f focusFirst) CreateRenderObject() core.RenderObject {
	return &renderFocusFirst{onLeave: f.onLeave}
}

// UpdateRenderObject implements core.RenderWidget.
func (f focusFirst) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderFocusFirst).onLeave = f.onLeave
}

// ChildWidget implements core.SingleChildWidget.
func (f focusFirst) ChildWidget() core.Widget { return f.child }

type renderFocusFirst struct {
	layout.RenderProxy
	onLeave func()
	done    bool
}

// Layout implements core.RenderObject.
func (r *renderFocusFirst) Layout(cs layout.Constraints) geom.Size {
	sz := r.RenderProxy.Layout(cs)
	if !r.done && r.ChildCount() > 0 {
		r.done = true
		if f := kit.FirstFocusable(r.ChildAt(0)); f != nil {
			core.RestoreFocus(f)
			if e, ok := f.(*field.RenderEditable); ok {
				// The caret goes to the end, so that typing adds to the text.
				e.MoveTo(len(e.Content), false)
			}
		}
	}
	return sz
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderFocusFirst) FocusWithinChanged(within bool) {
	if !within && r.onLeave != nil {
		r.onLeave()
	}
}
