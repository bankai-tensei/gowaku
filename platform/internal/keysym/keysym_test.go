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
