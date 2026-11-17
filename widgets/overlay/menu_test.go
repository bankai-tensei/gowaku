package overlay

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
)

var ctrlN = interact.Shortcut{Key: input.KeyN, Modifiers: input.ModControl}

func TestShortcut_StringAndMatches(t *testing.T) {
	s := interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl | input.ModShift}
	if got := s.String(); got != "Ctrl+Shift+S" {
		t.Errorf("String = %q", got)
	}
	if (interact.Shortcut{}).String() != "" || (interact.Shortcut{Key: input.KeyF5}).String() != "F5" {
		t.Error("the zero shortcut is empty, and a bare key has no plus")
	}
	if !s.Matches(&core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl | input.ModShift}) {
		t.Error("the exact chord must match")
	}
	if s.Matches(&core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl}) {
		t.Error("a missing modifier must not match")
	}
	if (interact.Shortcut{}).Matches(&core.KeyEvent{}) {
		t.Error("the zero shortcut matches nothing")
	}
}

func TestParseMnemonic(t *testing.T) {
	tests := []struct {
		name, label, text string
		key               input.KeyCode
	}{
		{"英語", "&File", "File", input.KeyF},
		{"和文の括弧", "ファイル(&F)", "ファイル(F)", input.KeyF},
		{"小文字", "e&xit", "exit", input.KeyX},
		{"二重の &", "Save && Quit", "Save & Quit", input.KeyNone},
		{"無し", "保存", "保存", input.KeyNone},
		{"最初だけ", "&a&b", "ab", input.KeyA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := kit.ParseMnemonic(tt.label)
			if m.Text != tt.text || m.Key != tt.key {
				t.Errorf("got %q %v, want %q %v", m.Text, m.Key, tt.text, tt.key)
			}
		})
	}
}

func TestActions_RunWithoutFocusAndPassDisabled(t *testing.T) {
	var ran, other int
	tree := wtest.TextTree(t, interact.Actions{
		Actions: []interact.Action{
			{Label: "保存", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: func() { ran++ }},
			{Label: "無効", Shortcut: ctrlN},
		},
		Child: interact.Actions{Actions: []interact.Action{{Shortcut: ctrlN, OnInvoke: func() { other++ }}}, Child: basic.Text{Content: "x"}},
	})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl, Repeat: true})
	if ran != 2 {
		t.Errorf("Ctrl+S ran %d times without a focus, want 2 with the repeat", ran)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyN, Modifiers: input.ModControl})
	if other != 1 {
		t.Error("a disabled action must let the shortcut through to another")
	}
}
