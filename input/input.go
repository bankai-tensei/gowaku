package input

// Button is a pointer button.
type Button uint8

// The pointer buttons.
const (
	ButtonPrimary Button = iota
	ButtonSecondary
	ButtonMiddle
	// ButtonBack and ButtonForward are the side buttons of a mouse, under the
	// thumb, which go back and forward through pages as in a browser. The tree
	// never delivers them as presses; see core.NavigationTarget.
	ButtonBack
	ButtonForward
)

// Modifiers is a set of modifier keys.
//
// On macOS, Command arrives as ModControl, as the Control key does, so that a
// shortcut written once with ModControl takes Ctrl on Windows and Linux and
// Command on the Mac. Option arrives as ModAlt.
type Modifiers uint8

// The modifier keys: ModShift, ModControl (also Command on macOS) and ModAlt
// (Option on macOS).
const (
	ModShift Modifiers = 1 << iota
	ModControl
	ModAlt
)

// Cursor is the shape of the mouse cursor.
type Cursor uint8

// The cursors. The zero value is the arrow.
const (
	CursorArrow Cursor = iota
	CursorIBeam
	CursorHand
	// CursorColResize and CursorRowResize are the double arrows of a divider
	// that moves left and right, or up and down, as between the panes of a
	// SplitPane.
	CursorColResize
	CursorRowResize
)

// KeyCode identifies a key by what shortcuts call it.
//
// Letters and digits have the values of their ASCII codes, letters in upper
// case. A punctuation key is the ASCII character that it types without
// modifiers on the layout in use: the key right of P is KeyBracketLeft on a US
// layout and KeyAt on a Japanese one. A character that needs Shift, such as @
// on a US layout, arrives as the key that types it, with ModShift (Shift+Key2).
// Every punctuation character of ASCII has a code, named below or not; see
// PunctuationKey. A key that types a character outside ASCII, such as ü, is
// KeyNone, unless it is a letter or a digit key; ¥ is KeyBackslash. See
// docs/ADR/0116-keys-and-open-url.md for how each system finds the character.
type KeyCode uint16

// The keys.
const (
	KeyNone      KeyCode = 0
	KeyBackspace KeyCode = 8
	KeyTab       KeyCode = 9
	KeyEnter     KeyCode = 13
	KeyEscape    KeyCode = 27
	KeySpace     KeyCode = 32

	Key0 KeyCode = '0'
	Key1 KeyCode = '1'
	Key2 KeyCode = '2'
	Key3 KeyCode = '3'
	Key4 KeyCode = '4'
	Key5 KeyCode = '5'
	Key6 KeyCode = '6'
	Key7 KeyCode = '7'
	Key8 KeyCode = '8'
	Key9 KeyCode = '9'
	KeyA KeyCode = 'A'
	KeyB KeyCode = 'B'
	KeyC KeyCode = 'C'
	KeyD KeyCode = 'D'
	KeyE KeyCode = 'E'
	KeyF KeyCode = 'F'
	KeyG KeyCode = 'G'
	KeyH KeyCode = 'H'
	KeyI KeyCode = 'I'
	KeyJ KeyCode = 'J'
	KeyK KeyCode = 'K'
	KeyL KeyCode = 'L'
	KeyM KeyCode = 'M'
	KeyN KeyCode = 'N'
	KeyO KeyCode = 'O'
	KeyP KeyCode = 'P'
	KeyQ KeyCode = 'Q'
	KeyR KeyCode = 'R'
	KeyS KeyCode = 'S'
	KeyT KeyCode = 'T'
	KeyU KeyCode = 'U'
	KeyV KeyCode = 'V'
	KeyW KeyCode = 'W'
	KeyX KeyCode = 'X'
	KeyY KeyCode = 'Y'
	KeyZ KeyCode = 'Z'

	KeyLeft KeyCode = 256 + iota
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyInsert
	// KeyAlt arrives when Alt is pressed and released with nothing in between,
	// which moves the keyboard to the menu bar on Windows and Linux. A press of
	// Alt with another key arrives as that key with ModAlt.
	KeyAlt
	// KeyContextMenu is the menu key beside the right Ctrl, which opens the
	// context menu.
	KeyContextMenu
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// The punctuation keys, by the ASCII codes of what they type. The first eleven
// are the punctuation keys of a US layout. KeyAt, KeyColon and KeyCaret are
// keys of their own on a Japanese layout, where both the ¥ key and the ろ key
// type a backslash. The keypad operators are KeyAsterisk, KeyPlus, KeyMinus and
// KeySlash on every layout.
const (
	KeyQuote        KeyCode = '\''
	KeyComma        KeyCode = ','
	KeyMinus        KeyCode = '-'
	KeyPeriod       KeyCode = '.'
	KeySlash        KeyCode = '/'
	KeySemicolon    KeyCode = ';'
	KeyEqual        KeyCode = '='
	KeyBracketLeft  KeyCode = '['
	KeyBackslash    KeyCode = '\\'
	KeyBracketRight KeyCode = ']'
	KeyBackquote    KeyCode = '`'
	KeyAt           KeyCode = '@'
	KeyColon        KeyCode = ':'
	KeyCaret        KeyCode = '^'
	KeyPlus         KeyCode = '+'
	KeyAsterisk     KeyCode = '*'
	KeyHash         KeyCode = '#'
	KeyLess         KeyCode = '<'
)

// Printable reports whether the key types a character without Ctrl or Alt:
// Space, a letter, a digit or a punctuation key. A text field claims the
// presses of these keys, whose characters arrive on their own as text, so that
// a shortcut of a bare key does not run while the user types.
func (k KeyCode) Printable() bool { return k >= KeySpace && k <= '~' }

// PunctuationKey returns the key of a punctuation character of ASCII, which is
// the character itself, and KeyNone for any other rune, except that ¥ is
// KeyBackslash: Japanese text has them as one character, and the ¥ key of a
// Mac types ¥ where a PC types a backslash. The platforms pass it what a key
// types without modifiers.
func PunctuationKey(r rune) KeyCode {
	switch {
	case isPunctuation(r):
		return KeyCode(r)
	case r == '¥':
		return KeyBackslash
	}
	return KeyNone
}

func isPunctuation(r rune) bool {
	return r >= '!' && r <= '/' || r >= ':' && r <= '@' || r >= '[' && r <= '`' || r >= '{' && r <= '~'
}

// ClauseKind is the state of a part of a composition.
type ClauseKind uint8

// The clause kinds.
const (
	// ClauseInput is text being typed, not yet converted.
	ClauseInput ClauseKind = iota
	// ClauseConverted is converted text that the IME is not working on.
	ClauseConverted
	// ClauseTarget is the clause that the candidates apply to.
	ClauseTarget
)

// Clause is a part of a composition. Start and End are byte offsets into the
// text of the composition.
type Clause struct {
	Start, End int
	Kind       ClauseKind
}

// Composition is text that an IME is still converting. It is not part of the
// document until the IME commits it.
type Composition struct {
	Text string
	// Cursor is the byte offset of the caret in Text.
	Cursor  int
	Clauses []Clause
}

// Caret returns the byte offset in Text at which to draw the caret. While the
// text is being typed that is Cursor. Once a clause is being converted it is
// the end of Text: an IME then reports the start of that clause as its cursor,
// so a caret drawn there jumps to the front on conversion and back to the end
// on commit. The underline of the clause shows where the IME is working, and
// the candidate list goes to the clause by itself.
func (c *Composition) Caret() int {
	for _, cl := range c.Clauses {
		if cl.Kind == ClauseTarget {
			return len(c.Text)
		}
	}
	return min(max(c.Cursor, 0), len(c.Text))
}
