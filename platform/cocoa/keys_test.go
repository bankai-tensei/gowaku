package cocoa

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestKeyCodeOf_PhysicalKeys(t *testing.T) {
	for _, c := range []struct {
		name string
		code uint16
		want input.KeyCode
	}{
		{"A", kVK_ANSI_A, input.KeyA},
		{"Z", kVK_ANSI_Z, input.KeyZ},
		{"5 は 6 より後のコード", kVK_ANSI_5, input.Key5},
		{"テンキーの 7", kVK_ANSI_Keypad7, input.Key7},
		{"Return", kVK_Return, input.KeyEnter},
		{"テンキーの Enter", kVK_ANSI_KeypadEnter, input.KeyEnter},
		{"delete は Backspace", kVK_Delete, input.KeyBackspace},
		{"fn+delete は Delete", kVK_ForwardDelete, input.KeyDelete},
		{"左矢印", kVK_LeftArrow, input.KeyLeft},
		{"F12", kVK_F12, input.KeyF12},
		{"知らないキー", 0x3F, input.KeyNone},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := keyCodeOf(c.code, ""); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestKeyCodeOf_PunctuationFollowsLayout(t *testing.T) {
	for _, c := range []struct {
		name      string
		code      uint16
		unshifted string
		want      input.KeyCode
	}{
		{"US の [", kVK_ANSI_LeftBracket, "[", input.KeyBracketLeft},
		{"US の \\", kVK_ANSI_Backslash, `\`, input.KeyBackslash},
		{"US の ;", kVK_ANSI_Semicolon, ";", input.KeySemicolon},
		{"US の '", kVK_ANSI_Quote, "'", input.KeyQuote},
		{"US の `", kVK_ANSI_Grave, "`", input.KeyBackquote},
		{"JIS の @ は [ の場所", kVK_ANSI_LeftBracket, "@", input.KeyAt},
		{"JIS の [ は ] の場所", kVK_ANSI_RightBracket, "[", input.KeyBracketLeft},
		{"JIS の : は ' の場所", kVK_ANSI_Quote, ":", input.KeyColon},
		{"JIS の ^ は = の場所", kVK_ANSI_Equal, "^", input.KeyCaret},
		{"JIS の ¥", kVK_JIS_Yen, "¥", input.KeyBackslash},
		{"JIS の ¥ を \\ にした設定", kVK_JIS_Yen, `\`, input.KeyBackslash},
		{"ドイツの ü", kVK_ANSI_LeftBracket, "ü", input.KeyNone},
		{"文字が分からなければ ANSI の位置", kVK_ANSI_RightBracket, "", input.KeyBracketRight},
		{"テンキーの + は配列に依らない", kVK_ANSI_KeypadPlus, "", input.KeyPlus},
		{"英字は物理位置のまま", kVK_ANSI_A, "q", input.KeyA},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := keyCodeOf(c.code, c.unshifted); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestModifiersOf_CommandIsControl(t *testing.T) {
	for _, c := range []struct {
		name  string
		flags uint64
		want  input.Modifiers
	}{
		{"なし", 0, 0},
		{"Command", NSEventModifierFlagCommand, input.ModControl},
		{"Control", NSEventModifierFlagControl, input.ModControl},
		{"Option", NSEventModifierFlagOption, input.ModAlt},
		{"Shift+Command", NSEventModifierFlagShift | NSEventModifierFlagCommand, input.ModShift | input.ModControl},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := modifiersOf(c.flags); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestTypedText_DropsControlAndFunctionKeys(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a", "a"},
		{"あ", "あ"},
		{"\r", ""},
		{"\x7f", ""},
		{"", ""}, // NSUpArrowFunctionKey
		{"😀", "😀"},
	} {
		if got := typedText(c.in); got != c.want {
			t.Errorf("typedText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
