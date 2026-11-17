package cocoa

import (
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/input"
)

// The virtual key codes of HIToolbox/Events.h that keyCodeOf maps. They name
// physical keys on the ANSI layout, whatever the layout of the user.
const (
	kVK_ANSI_A            = 0x00
	kVK_ANSI_S            = 0x01
	kVK_ANSI_D            = 0x02
	kVK_ANSI_F            = 0x03
	kVK_ANSI_H            = 0x04
	kVK_ANSI_G            = 0x05
	kVK_ANSI_Z            = 0x06
	kVK_ANSI_X            = 0x07
	kVK_ANSI_C            = 0x08
	kVK_ANSI_V            = 0x09
	kVK_ANSI_B            = 0x0B
	kVK_ANSI_Q            = 0x0C
	kVK_ANSI_W            = 0x0D
	kVK_ANSI_E            = 0x0E
	kVK_ANSI_R            = 0x0F
	kVK_ANSI_Y            = 0x10
	kVK_ANSI_T            = 0x11
	kVK_ANSI_1            = 0x12
	kVK_ANSI_2            = 0x13
	kVK_ANSI_3            = 0x14
	kVK_ANSI_4            = 0x15
	kVK_ANSI_6            = 0x16
	kVK_ANSI_5            = 0x17
	kVK_ANSI_Equal        = 0x18
	kVK_ANSI_9            = 0x19
	kVK_ANSI_7            = 0x1A
	kVK_ANSI_Minus        = 0x1B
	kVK_ANSI_8            = 0x1C
	kVK_ANSI_0            = 0x1D
	kVK_ANSI_O            = 0x1F
	kVK_ANSI_U            = 0x20
	kVK_ANSI_I            = 0x22
	kVK_ANSI_P            = 0x23
	kVK_Return            = 0x24
	kVK_ANSI_L            = 0x25
	kVK_ANSI_J            = 0x26
	kVK_ANSI_K            = 0x28
	kVK_ANSI_Comma        = 0x2B
	kVK_ANSI_Slash        = 0x2C
	kVK_ANSI_N            = 0x2D
	kVK_ANSI_M            = 0x2E
	kVK_ANSI_Period       = 0x2F
	kVK_Tab               = 0x30
	kVK_Space             = 0x31
	kVK_Delete            = 0x33
	kVK_Escape            = 0x35
	kVK_Option            = 0x3A
	kVK_RightOption       = 0x3D
	kVK_ANSI_KeypadEnter  = 0x4C
	kVK_ANSI_KeypadMinus  = 0x4E
	kVK_ANSI_KeypadEquals = 0x51
	kVK_ANSI_Keypad0      = 0x52
	kVK_ANSI_Keypad1      = 0x53
	kVK_ANSI_Keypad2      = 0x54
	kVK_ANSI_Keypad3      = 0x55
	kVK_ANSI_Keypad4      = 0x56
	kVK_ANSI_Keypad5      = 0x57
	kVK_ANSI_Keypad6      = 0x58
	kVK_ANSI_Keypad7      = 0x59
	kVK_ANSI_Keypad8      = 0x5B
	kVK_ANSI_Keypad9      = 0x5C
	kVK_ANSI_KeypadComma  = 0x5F
	kVK_F5                = 0x60
	kVK_F6                = 0x61
	kVK_F7                = 0x62
	kVK_F3                = 0x63
	kVK_F8                = 0x64
	kVK_F9                = 0x65
	kVK_F11               = 0x67
	kVK_F10               = 0x6D
	kVK_ContextualMenu    = 0x6E
	kVK_F12               = 0x6F
	kVK_Help              = 0x72
	kVK_Home              = 0x73
	kVK_PageUp            = 0x74
	kVK_ForwardDelete     = 0x75
	kVK_F4                = 0x76
	kVK_End               = 0x77
	kVK_F2                = 0x78
	kVK_PageDown          = 0x79
	kVK_F1                = 0x7A
	kVK_LeftArrow         = 0x7B
	kVK_RightArrow        = 0x7C
	kVK_DownArrow         = 0x7D
	kVK_UpArrow           = 0x7E

	kVK_ISO_Section         = 0x0A
	kVK_ANSI_RightBracket   = 0x1E
	kVK_ANSI_LeftBracket    = 0x21
	kVK_ANSI_Quote          = 0x27
	kVK_ANSI_Semicolon      = 0x29
	kVK_ANSI_Backslash      = 0x2A
	kVK_ANSI_Grave          = 0x32
	kVK_ANSI_KeypadMultiply = 0x43
	kVK_ANSI_KeypadPlus     = 0x45
	kVK_ANSI_KeypadDivide   = 0x4B
	kVK_JIS_Yen             = 0x5D
	kVK_JIS_Underscore      = 0x5E
)

// usPunctuation is what the punctuation keys type on the ANSI layout. A key
// here is what it types on the layout in use (punctuationKeyOf), and this is
// the answer only when the event does not say. On a JIS keyboard the keys sit
// where the ANSI ones do, and type other characters: the key of [ types @.
var usPunctuation = map[uint16]input.KeyCode{
	kVK_ANSI_Equal: input.KeyEqual, kVK_ANSI_Minus: input.KeyMinus,
	kVK_ANSI_LeftBracket: input.KeyBracketLeft, kVK_ANSI_RightBracket: input.KeyBracketRight,
	kVK_ANSI_Quote: input.KeyQuote, kVK_ANSI_Semicolon: input.KeySemicolon,
	kVK_ANSI_Backslash: input.KeyBackslash, kVK_ANSI_Comma: input.KeyComma,
	kVK_ANSI_Slash: input.KeySlash, kVK_ANSI_Period: input.KeyPeriod,
	kVK_ANSI_Grave: input.KeyBackquote, kVK_ISO_Section: input.KeyNone,
	kVK_JIS_Yen: input.KeyBackslash, kVK_JIS_Underscore: '_',
}

// isPunctuationCode reports whether the key of a virtual key code is what it
// types, so that keyDown asks the event for its characters.
func isPunctuationCode(code uint16) bool {
	_, ok := usPunctuation[code]
	return ok
}

// punctuationKeyOf returns the key of a punctuation key from what it types
// without modifiers, unshifted, which is "" if the event does not say.
func punctuationKeyOf(code uint16, unshifted string) input.KeyCode {
	if unshifted == "" {
		return usPunctuation[code]
	}
	r, n := utf8.DecodeRuneInString(unshifted)
	if n != len(unshifted) {
		return input.KeyNone
	}
	return input.PunctuationKey(r)
}

// keyCodes maps the virtual key codes to input key codes. The keypad types
// what the main keys type.
var keyCodes = map[uint16]input.KeyCode{
	kVK_ANSI_A: input.KeyA, kVK_ANSI_B: input.KeyB, kVK_ANSI_C: input.KeyC, kVK_ANSI_D: input.KeyD,
	kVK_ANSI_E: input.KeyE, kVK_ANSI_F: input.KeyF, kVK_ANSI_G: input.KeyG, kVK_ANSI_H: input.KeyH,
	kVK_ANSI_I: input.KeyI, kVK_ANSI_J: input.KeyJ, kVK_ANSI_K: input.KeyK, kVK_ANSI_L: input.KeyL,
	kVK_ANSI_M: input.KeyM, kVK_ANSI_N: input.KeyN, kVK_ANSI_O: input.KeyO, kVK_ANSI_P: input.KeyP,
	kVK_ANSI_Q: input.KeyQ, kVK_ANSI_R: input.KeyR, kVK_ANSI_S: input.KeyS, kVK_ANSI_T: input.KeyT,
	kVK_ANSI_U: input.KeyU, kVK_ANSI_V: input.KeyV, kVK_ANSI_W: input.KeyW, kVK_ANSI_X: input.KeyX,
	kVK_ANSI_Y: input.KeyY, kVK_ANSI_Z: input.KeyZ,
	kVK_ANSI_0: input.Key0, kVK_ANSI_1: input.Key1, kVK_ANSI_2: input.Key2, kVK_ANSI_3: input.Key3,
	kVK_ANSI_4: input.Key4, kVK_ANSI_5: input.Key5, kVK_ANSI_6: input.Key6, kVK_ANSI_7: input.Key7,
	kVK_ANSI_8: input.Key8, kVK_ANSI_9: input.Key9,
	kVK_ANSI_Keypad0: input.Key0, kVK_ANSI_Keypad1: input.Key1, kVK_ANSI_Keypad2: input.Key2,
	kVK_ANSI_Keypad3: input.Key3, kVK_ANSI_Keypad4: input.Key4, kVK_ANSI_Keypad5: input.Key5,
	kVK_ANSI_Keypad6: input.Key6, kVK_ANSI_Keypad7: input.Key7, kVK_ANSI_Keypad8: input.Key8,
	kVK_ANSI_Keypad9:     input.Key9,
	kVK_ANSI_KeypadComma: input.KeyComma, kVK_ANSI_KeypadMinus: input.KeyMinus,
	kVK_ANSI_KeypadEquals: input.KeyEqual, kVK_ANSI_KeypadPlus: input.KeyPlus,
	kVK_ANSI_KeypadMultiply: input.KeyAsterisk, kVK_ANSI_KeypadDivide: input.KeySlash,
	kVK_Return: input.KeyEnter, kVK_ANSI_KeypadEnter: input.KeyEnter,
	kVK_Tab: input.KeyTab, kVK_Space: input.KeySpace, kVK_Delete: input.KeyBackspace,
	kVK_Escape: input.KeyEscape, kVK_ForwardDelete: input.KeyDelete, kVK_Help: input.KeyInsert,
	kVK_Option: input.KeyAlt, kVK_RightOption: input.KeyAlt, kVK_ContextualMenu: input.KeyContextMenu,
	kVK_Home: input.KeyHome, kVK_End: input.KeyEnd, kVK_PageUp: input.KeyPageUp, kVK_PageDown: input.KeyPageDown,
	kVK_LeftArrow: input.KeyLeft, kVK_RightArrow: input.KeyRight, kVK_UpArrow: input.KeyUp, kVK_DownArrow: input.KeyDown,
	kVK_F1: input.KeyF1, kVK_F2: input.KeyF2, kVK_F3: input.KeyF3, kVK_F4: input.KeyF4,
	kVK_F5: input.KeyF5, kVK_F6: input.KeyF6, kVK_F7: input.KeyF7, kVK_F8: input.KeyF8,
	kVK_F9: input.KeyF9, kVK_F10: input.KeyF10, kVK_F11: input.KeyF11, kVK_F12: input.KeyF12,
}

// keyCodeOf returns the key of a virtual key code, or KeyNone. unshifted is
// what the key types without modifiers, which only the punctuation keys need;
// see punctuationKeyOf.
func keyCodeOf(code uint16, unshifted string) input.KeyCode {
	if isPunctuationCode(code) {
		return punctuationKeyOf(code, unshifted)
	}
	return keyCodes[code]
}

// The device-independent bits of NSEventModifierFlags.
const (
	NSEventModifierFlagShift   = 1 << 17
	NSEventModifierFlagControl = 1 << 18
	NSEventModifierFlagOption  = 1 << 19
	NSEventModifierFlagCommand = 1 << 20
)

// modifiersOf maps the modifier flags of an NSEvent. Command stands for
// ModControl, so that the shortcuts that Windows and Linux take with Ctrl,
// such as copy and paste, take Command on the Mac; the Control key does the
// same. See docs/ADR/0082-macos-cocoa-metal.md.
func modifiersOf(flags uint64) input.Modifiers {
	var m input.Modifiers
	if flags&NSEventModifierFlagShift != 0 {
		m |= input.ModShift
	}
	if flags&(NSEventModifierFlagControl|NSEventModifierFlagCommand) != 0 {
		m |= input.ModControl
	}
	if flags&NSEventModifierFlagOption != 0 {
		m |= input.ModAlt
	}
	return m
}

// typedText filters what the key bindings hand to insertText: control
// characters arrive as keys, and the function keys of AppKit live in the
// private use area from U+F700.
func typedText(s string) string {
	for _, r := range s {
		if r < 0x20 || r == 0x7F || (r >= 0xF700 && r <= 0xF8FF) {
			return ""
		}
	}
	return s
}
