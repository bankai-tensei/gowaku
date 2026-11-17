package kit

import (
	"strings"
	"unicode/utf8"
)

// MaxHistory bounds the steps a field remembers.
const MaxHistory = 500

// EditKind says how an edit came about, which decides what it merges with.
type EditKind uint8

const (
	// EditOther never merges: a paste, a cut, a commit of an IME.
	EditOther EditKind = iota
	// EditTyping is one typed character.
	EditTyping
	// EditBackspace removes text before the caret.
	EditBackspace
	// EditForward removes text after the caret.
	EditForward
)

// Edit is one undoable step. It stores the difference, so that a step in a long
// text costs what the edit costs.
type Edit struct {
	Kind EditKind
	// at is the offset of the change.
	At int
	// removed was at offset at before the edit, inserted is there after it.
	Removed, Inserted string
	// anchor and caret are the selection before the edit.
	Anchor, Caret int
}

// History is the undo and redo stacks of a text field.
type History struct {
	Undo, Redo []Edit
	// sealed keeps the next edit from merging with the last one.
	sealed bool
}

// Seal ends the current run of typing or deleting. A caret move does this.
func (h *History) Seal() { h.sealed = true }

// Clear forgets everything. The steps no longer fit a text set from outside.
func (h *History) Clear() {
	h.Undo, h.Redo = h.Undo[:0], h.Redo[:0]
	h.sealed = false
}

// Record adds e, merging it with the last step if they belong together.
func (h *History) Record(e Edit) {
	h.Redo = h.Redo[:0]
	if n := len(h.Undo); n > 0 && !h.sealed && h.Undo[n-1].merge(&e) {
		return
	}
	h.sealed = false
	if len(h.Undo) == MaxHistory {
		copy(h.Undo, h.Undo[1:])
		h.Undo = h.Undo[:MaxHistory-1]
	}
	h.Undo = append(h.Undo, e)
}

// merge folds next into e and reports whether it did. Typing merges up to the
// end of a word, so that one undo takes back a word and not a letter.
func (e *Edit) merge(next *Edit) bool {
	if e.Kind != next.Kind {
		return false
	}
	switch e.Kind {
	case EditTyping:
		if next.At != e.At+len(e.Inserted) || next.Removed != "" || startsWord(e.Inserted, next.Inserted) {
			return false
		}
		e.Inserted += next.Inserted
		return true
	case EditBackspace:
		if next.At+len(next.Removed) != e.At {
			return false
		}
		e.At = next.At
		e.Removed = next.Removed + e.Removed
		return true
	case EditForward:
		if next.At != e.At {
			return false
		}
		e.Removed += next.Removed
		return true
	}
	return false
}

// startsWord reports whether next begins a new word after what was typed.
func startsWord(typed, next string) bool {
	if next == "\n" {
		return true
	}
	last, _ := utf8.DecodeLastRuneInString(typed)
	return isBlank(last) && !strings.ContainsFunc(next, isBlank)
}

func isBlank(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '　' }

// TakeUndo removes the last step and moves it to the redo stack.
func (h *History) TakeUndo() (Edit, bool) {
	n := len(h.Undo)
	if n == 0 {
		return Edit{}, false
	}
	e := h.Undo[n-1]
	h.Undo = h.Undo[:n-1]
	h.Redo = append(h.Redo, e)
	h.sealed = true
	return e, true
}

// TakeRedo removes the last undone step and moves it back to the undo stack.
func (h *History) TakeRedo() (Edit, bool) {
	n := len(h.Redo)
	if n == 0 {
		return Edit{}, false
	}
	e := h.Redo[n-1]
	h.Redo = h.Redo[:n-1]
	h.Undo = append(h.Undo, e)
	h.sealed = true
	return e, true
}
