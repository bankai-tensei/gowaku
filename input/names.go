package input

// String returns the name of the key as a shortcut shows it: "A", "F5",
// "Enter", "Del", and a punctuation key as its character: "\", "@", "[". It is
// "" for KeyNone and for codes that are not keys.
func (k KeyCode) String() string {
	switch {
	case k >= KeyA && k <= KeyZ, k >= Key0 && k <= Key9:
		return string(rune(k))
	case k >= KeyF1 && k <= KeyF12:
		return fKeyNames[k-KeyF1]
	case k < 0x80 && isPunctuation(rune(k)):
		return string(rune(k))
	}
	switch k {
	case KeyBackspace:
		return "Backspace"
	case KeyTab:
		return "Tab"
	case KeyEnter:
		return "Enter"
	case KeyEscape:
		return "Esc"
	case KeySpace:
		return "Space"
	case KeyLeft:
		return "Left"
	case KeyRight:
		return "Right"
	case KeyUp:
		return "Up"
	case KeyDown:
		return "Down"
	case KeyHome:
		return "Home"
	case KeyEnd:
		return "End"
	case KeyPageUp:
		return "PgUp"
	case KeyPageDown:
		return "PgDn"
	case KeyDelete:
		return "Del"
	case KeyInsert:
		return "Ins"
	case KeyAlt:
		return "Alt"
	case KeyContextMenu:
		return "Menu"
	}
	return ""
}

var fKeyNames = [...]string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"}

// String returns the modifiers as a shortcut shows them, in the order of
// Windows: "Ctrl+Shift+Alt+". It ends with a plus unless m is empty.
func (m Modifiers) String() string {
	s := ""
	if m&ModControl != 0 {
		s += "Ctrl+"
	}
	if m&ModShift != 0 {
		s += "Shift+"
	}
	if m&ModAlt != 0 {
		s += "Alt+"
	}
	return s
}
