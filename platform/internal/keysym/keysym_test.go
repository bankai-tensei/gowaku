package keysym

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestKeyCode(t *testing.T) {
	tests := []struct {
		name   string
		keysym uint64
		want   input.KeyCode
	}{
		{"小文字", 'c', input.KeyC},
		{"大文字", 'C', input.KeyC},
		{"数字", '9', input.Key9},
		{"テンキーの 4", XK_KP_0 + 4, input.Key4},
		{"テンキーの 0", XK_KP_0, input.Key0},
		{"テンキーの小数点", XK_KP_Decimal, input.KeyPeriod},
		{"Enter", XK_Return, input.KeyEnter},
		{"テンキーの Enter", XK_KP_Enter, input.KeyEnter},
		{"Shift+Tab", XK_ISO_Left_Tab, input.KeyTab},
		{"Delete", XK_Delete, input.KeyDelete},
		{"テンキーの左", XK_KP_Left, input.KeyLeft},
		{"US の \\", '\\', input.KeyBackslash},
		{"US の ;", ';', input.KeySemicolon},
		{"US の [", '[', input.KeyBracketLeft},
		{"US の '", '\'', input.KeyQuote},
		{"US の `", '`', input.KeyBackquote},
		{"JIS の @", '@', input.KeyAt},
		{"JIS の :", ':', input.KeyColon},
		{"JIS の ^", '^', input.KeyCaret},
		{"テンキーの +", XK_KP_Add, input.KeyPlus},
		{"デッドキーの ^", XK_dead_circumflex, input.KeyCaret},
		{"yen は \\", 0xa5, input.KeyBackslash},
		{"ü は対象外", 0xfc, input.KeyNone},
		{"かな", 0x4b1, input.KeyNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KeyCode(tt.keysym); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKeypadNumLockKey(t *testing.T) {
	for _, tt := range []struct {
		keysym uint64
		want   bool
	}{
		{XK_KP_Left, true},
		{XK_KP_Begin, true},
		{XK_KP_0 + 4, true},
		{XK_KP_Decimal, true},
		{XK_KP_Enter, false},
		{XK_KP_Add, false},
		{'4', false},
	} {
		if got := IsKeypadNumLockKey(tt.keysym); got != tt.want {
			t.Errorf("IsKeypadNumLockKey(%#x) = %v, want %v", tt.keysym, got, tt.want)
		}
	}
}

func TestKeyCode_NumericKeypadDigits(t *testing.T) {
	for digit := uint64(0); digit <= 9; digit++ {
		want := input.KeyCode('0' + digit)
		if got := KeyCode(XK_KP_0 + digit); got != want {
			t.Errorf("KeyCode(KP_%d) = %v, want %v", digit, got, want)
		}
	}
}

func TestTypedText(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"英字", "a", "a"},
		{"確定した日本語", "日本語", "日本語"},
		{"Enter", "\r", ""},
		{"Backspace", "\b", ""},
		{"Delete", "\x7f", ""},
		{"混在", "a\tb", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypedText([]byte(tt.in)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
