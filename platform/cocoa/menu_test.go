package cocoa

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestMenuTitle_DropsMnemonics(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"ファイル(&F)", "ファイル"},
		{"名前を付けて保存(&A)…", "名前を付けて保存…"},
		{"&Open", "Open"},
		{"Save && Quit", "Save & Quit"},
		{"Plain", "Plain"},
	} {
		if got := menuTitle(c.in); got != c.want {
			t.Errorf("menuTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKeyEquivalent(t *testing.T) {
	for _, c := range []struct {
		name string
		key  input.KeyCode
		mods input.Modifiers
		want string
		mask uint64
		ok   bool
	}{
		{"Ctrl+S は ⌘S", input.KeyS, input.ModControl, "s", NSEventModifierFlagCommand, true},
		{"Ctrl+Shift+S", input.KeyS, input.ModControl | input.ModShift, "s", NSEventModifierFlagCommand | NSEventModifierFlagShift, true},
		{"F5", input.KeyF5, 0, string(rune(NSF1FunctionKey + 4)), 0, true},
		{"Ctrl+Enter", input.KeyEnter, input.ModControl, "\r", NSEventModifierFlagCommand, true},
		{"Ctrl+\\ は ⌘\\", input.KeyBackslash, input.ModControl, `\`, NSEventModifierFlagCommand, true},
		{"Ctrl+@", input.KeyAt, input.ModControl, "@", NSEventModifierFlagCommand, true},
		{"無し", input.KeyNone, 0, "", 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, mask, ok := keyEquivalent(c.key, c.mods)
			if got != c.want || mask != c.mask || ok != c.ok {
				t.Errorf("got %q %#x %v, want %q %#x %v", got, mask, ok, c.want, c.mask, c.ok)
			}
		})
	}
}
