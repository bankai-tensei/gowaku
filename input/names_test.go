package input

import "testing"

func TestPunctuationKey(t *testing.T) {
	for _, c := range []struct {
		r    rune
		want KeyCode
	}{
		{'\\', KeyBackslash},
		{'@', KeyAt},
		{'[', KeyBracketLeft},
		{']', KeyBracketRight},
		{';', KeySemicolon},
		{':', KeyColon},
		{'\'', KeyQuote},
		{'`', KeyBackquote},
		{'^', KeyCaret},
		{'~', '~'},
		{'!', '!'},
		{'a', KeyNone},
		{'A', KeyNone},
		{'5', KeyNone},
		{' ', KeyNone},
		{'¥', KeyBackslash},
		{'ü', KeyNone},
		{0x7F, KeyNone},
	} {
		if got := PunctuationKey(c.r); got != c.want {
			t.Errorf("PunctuationKey(%q) = %d, want %d", c.r, got, c.want)
		}
	}
}

func TestKeyCodeString_Punctuation(t *testing.T) {
	for _, c := range []struct {
		k    KeyCode
		want string
	}{
		{KeyBackslash, `\`},
		{KeyAt, "@"},
		{KeyBracketLeft, "["},
		{KeyQuote, "'"},
		{KeyComma, ","},
		{KeyPlus, "+"},
		{KeySpace, "Space"},
		{KeyNone, ""},
		{KeyCode(0x7F), ""},
	} {
		if got := c.k.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", c.k, got, c.want)
		}
	}
	if got := ModControl.String() + KeyBackslash.String(); got != `Ctrl+\` {
		t.Errorf("got %q, want Ctrl+\\", got)
	}
}
