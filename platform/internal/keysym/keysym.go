// Package keysym maps the keysyms of X11 and xkbcommon, which share their
// values, to input.KeyCode. platform/x11 and platform/wayland use it.
package keysym

import (
	"strings"

	"github.com/bankai-tensei/gowaku/input"
)

// The keysyms that KeyCode knows, with the names of keysymdef.h.
const (
	XK_space        = 0x20
	XK_ISO_Left_Tab = 0xfe20
	XK_BackSpace    = 0xff08
	XK_Tab          = 0xff09
	XK_Return       = 0xff0d
	XK_Escape       = 0xff1b
	XK_Home         = 0xff50
	XK_Left         = 0xff51
	XK_Up           = 0xff52
	XK_Right        = 0xff53
	XK_Down         = 0xff54
	XK_Page_Up      = 0xff55
	XK_Page_Down    = 0xff56
	XK_End          = 0xff57
	XK_Insert       = 0xff63
	XK_KP_Enter     = 0xff8d
	XK_KP_Home      = 0xff95
	XK_KP_Left      = 0xff96
	XK_KP_Up        = 0xff97
	XK_KP_Right     = 0xff98
	XK_KP_Down      = 0xff99
	XK_KP_Page_Up   = 0xff9a
	XK_KP_Page_Down = 0xff9b
	XK_KP_End       = 0xff9c
	XK_KP_Begin     = 0xff9d
	XK_KP_Insert    = 0xff9e
	XK_KP_Delete    = 0xff9f
	XK_KP_Decimal   = 0xffae
	XK_KP_0         = 0xffb0
	XK_KP_9         = 0xffb9
	XK_Delete       = 0xffff
	XK_Menu         = 0xff67
	XK_F1           = 0xffbe
	XK_F12          = 0xffc9
	XK_Alt_L        = 0xffe9
	XK_Alt_R        = 0xffea
	XK_Meta_L       = 0xffe7
	XK_Meta_R       = 0xffe8

	XK_KP_Multiply = 0xffaa
	XK_KP_Add      = 0xffab
	XK_KP_Subtract = 0xffad
	XK_KP_Divide   = 0xffaf

	// The dead keys of layouts such as us(intl) and de, which stand for their
	// accents, as Windows has them.
	XK_dead_grave      = 0xfe50
	XK_dead_acute      = 0xfe51
	XK_dead_circumflex = 0xfe52
	XK_dead_tilde      = 0xfe53
)

// KeyCode maps a keysym, as found without modifiers in the layout in effect,
// to an input.KeyCode. A punctuation key is what it types (ADR 0116).
func KeyCode(keysym uint64) input.KeyCode {
	switch {
	case keysym >= 'a' && keysym <= 'z':
		return input.KeyCode(keysym - 'a' + 'A')
	case keysym >= 'A' && keysym <= 'Z', keysym >= '0' && keysym <= '9':
		return input.KeyCode(keysym)
	case keysym >= XK_KP_0 && keysym <= XK_KP_9:
		return input.KeyCode(keysym - XK_KP_0 + '0')
	case keysym >= XK_F1 && keysym <= XK_F12:
		return input.KeyF1 + input.KeyCode(keysym-XK_F1)
	}
	switch keysym {
	case XK_BackSpace:
		return input.KeyBackspace
	case XK_Tab, XK_ISO_Left_Tab:
		return input.KeyTab
	case XK_Return, XK_KP_Enter:
		return input.KeyEnter
	case XK_Escape:
		return input.KeyEscape
	case XK_space:
		return input.KeySpace
	case XK_Left, XK_KP_Left:
		return input.KeyLeft
	case XK_Right, XK_KP_Right:
		return input.KeyRight
	case XK_Up, XK_KP_Up:
		return input.KeyUp
	case XK_Down, XK_KP_Down:
		return input.KeyDown
	case XK_Home, XK_KP_Home:
		return input.KeyHome
	case XK_End, XK_KP_End:
		return input.KeyEnd
	case XK_Page_Up, XK_KP_Page_Up:
		return input.KeyPageUp
	case XK_Page_Down, XK_KP_Page_Down:
		return input.KeyPageDown
	case XK_Delete, XK_KP_Delete:
		return input.KeyDelete
	case XK_Insert, XK_KP_Insert:
		return input.KeyInsert
	case XK_KP_Decimal:
		return input.KeyPeriod
	case XK_Menu:
		return input.KeyContextMenu
	// Some layouts put Alt on the Meta keysyms.
	case XK_Alt_L, XK_Alt_R, XK_Meta_L, XK_Meta_R:
		return input.KeyAlt
	case XK_KP_Multiply:
		return input.KeyAsterisk
	case XK_KP_Add:
		return input.KeyPlus
	case XK_KP_Subtract:
		return input.KeyMinus
	case XK_KP_Divide:
		return input.KeySlash
	case XK_dead_grave:
		return input.KeyBackquote
	case XK_dead_acute:
		return input.KeyQuote
	case XK_dead_circumflex:
		return input.KeyCaret
	case XK_dead_tilde:
		return '~'
	}
	// The keysyms of Latin-1 are its codes. On a Japanese layout the ¥ key and
	// the ろ key are both backslash, and yen is taken as one too.
	if keysym < 0x100 {
		return input.PunctuationKey(rune(keysym))
	}
	return input.KeyNone
}

// IsKeypadNumLockKey reports whether a keypad symbol can mean either a digit or
// navigation, depending on Num Lock and Shift.
func IsKeypadNumLockKey(keysym uint64) bool {
	return keysym >= XK_KP_Home && keysym <= XK_KP_Delete ||
		keysym >= XK_KP_0 && keysym <= XK_KP_9 || keysym == XK_KP_Decimal
}

// TypedText returns what a key press adds to a document: the printable part of
// the text of the key. Control characters arrive as keys.
func TypedText(b []byte) string {
	s := string(b)
	if strings.IndexFunc(s, isControl) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return -1
		}
		return r
	}, s)
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7F }
