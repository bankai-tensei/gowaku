package kit

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
)

// TypingKey reports whether a text field that takes text should claim the
// press e: a key that types a character, alone or with Shift, whose character
// arrives on its own as text. While an IME composes, the press belongs to the
// composition whatever its modifiers. Claiming the press keeps a shortcut of a
// bare key, such as Space for play, from running while the user types, and
// keeps a row around the field from taking Space; Ctrl and Alt shortcuts the
// field does not use still reach the Actions above it.
func TypingKey(e *core.KeyEvent, composing bool) bool {
	return e.Key.Printable() && (composing || e.Modifiers&^input.ModShift == 0)
}

// ShortcutEnter reports whether a text field should leave the press e to the
// shortcuts above it: Enter with Ctrl or Alt, with Shift or without. A field
// uses only Enter and Shift+Enter, to break the line or to submit, so that
// Ctrl+Enter can mean "commit" or "confirm and next" while a field has the
// keyboard. While an IME composes, Enter commits the composition whatever its
// modifiers, and the press is the IME's.
func ShortcutEnter(e *core.KeyEvent, composing bool) bool {
	return e.Key == input.KeyEnter && !composing && e.Modifiers&(input.ModControl|input.ModAlt) != 0
}
