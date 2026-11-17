package cocoa

import (
	"strings"

	"github.com/bankai-tensei/gowaku/input"
)

// menuTitle drops the mnemonic of a label, which Mac menus do not have: a
// Japanese "ファイル(&F)" loses the parentheses, "&Open" its &, and "&&"
// stays as one &.
func menuTitle(label string) string {
	var b strings.Builder
	for i := 0; i < len(label); i++ {
		c := label[i]
		if c != '&' {
			b.WriteByte(c)
			continue
		}
		if i+1 < len(label) && label[i+1] == '&' {
			b.WriteByte('&')
			i++
			continue
		}
		// "(&F)": drop the open parenthesis already written, the letter and
		// the close parenthesis.
		s := b.String()
		if strings.HasSuffix(s, "(") && i+2 < len(label) && label[i+2] == ')' {
			b.Reset()
			b.WriteString(strings.TrimRight(strings.TrimSuffix(s, "("), " "))
			i += 2
		}
	}
	return b.String()
}

// The function keys of NSEvent.h, which key equivalents take as characters.
const (
	NSUpArrowFunctionKey    = 0xF700
	NSDownArrowFunctionKey  = 0xF701
	NSLeftArrowFunctionKey  = 0xF702
	NSRightArrowFunctionKey = 0xF703
	NSF1FunctionKey         = 0xF704
	NSHomeFunctionKey       = 0xF729
	NSEndFunctionKey        = 0xF72B
	NSPageUpFunctionKey     = 0xF72C
	NSPageDownFunctionKey   = 0xF72D
	NSDeleteFunctionKey     = 0xF728
)

// keyEquivalent returns the key equivalent of a shortcut and its modifier
// mask, Control being Command as in modifiersOf. ok is false for a key that a
// menu cannot show.
func keyEquivalent(key input.KeyCode, mods input.Modifiers) (equivalent string, mask uint64, ok bool) {
	var r rune
	switch {
	case key >= input.KeyA && key <= input.KeyZ:
		r = rune(key) - 'A' + 'a'
	case key >= input.Key0 && key <= input.Key9, key == input.KeySpace,
		key < 0x80 && key != input.KeyNone && input.PunctuationKey(rune(key)) == key:
		r = rune(key)
	case key >= input.KeyF1 && key <= input.KeyF12:
		r = NSF1FunctionKey + rune(key-input.KeyF1)
	default:
		special := map[input.KeyCode]rune{
			input.KeyEnter: '\r', input.KeyTab: '\t', input.KeyEscape: 0x1B, input.KeyBackspace: 0x08,
			input.KeyDelete: NSDeleteFunctionKey, input.KeyUp: NSUpArrowFunctionKey, input.KeyDown: NSDownArrowFunctionKey,
			input.KeyLeft: NSLeftArrowFunctionKey, input.KeyRight: NSRightArrowFunctionKey, input.KeyHome: NSHomeFunctionKey,
			input.KeyEnd: NSEndFunctionKey, input.KeyPageUp: NSPageUpFunctionKey, input.KeyPageDown: NSPageDownFunctionKey,
		}
		var found bool
		if r, found = special[key]; !found {
			return "", 0, false
		}
	}
	if mods&input.ModControl != 0 {
		mask |= NSEventModifierFlagCommand
	}
	if mods&input.ModShift != 0 {
		mask |= NSEventModifierFlagShift
	}
	if mods&input.ModAlt != 0 {
		mask |= NSEventModifierFlagOption
	}
	return string(r), mask, true
}
