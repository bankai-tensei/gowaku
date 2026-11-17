package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// musicPlayer is a music player: bare Space plays and pauses, Shift+P skips and
// Ctrl+S saves, beside the text fields of its search and notes.
type musicPlayer struct {
	gowaku.StatelessBase
	runs     map[string]int
	readOnly bool
}

func (p musicPlayer) Build(gowaku.BuildContext) gowaku.Widget {
	run := func(name string) func() { return func() { p.runs[name]++ } }
	return interact.Actions{
		Actions: []interact.Action{
			{Label: "Play", Shortcut: interact.Shortcut{Key: input.KeySpace}, OnInvoke: run("play")},
			{Label: "Skip", Shortcut: interact.Shortcut{Key: input.KeyP, Modifiers: input.ModShift}, OnInvoke: run("skip")},
			{Label: "Save", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: run("save")},
		},
		Child: basic.Column{Spacing: 8, Children: []gowaku.Widget{
			field.Input{Label: "Name"},
			field.Search{Label: "Find"},
			field.Input{Label: "Story", Multiline: true},
			basic.SizedBox{Width: 300, Height: 80, Child: editor.Editor{Label: "Notes", ReadOnly: p.readOnly}},
			control.Button{Label: "Next"},
		}},
	}
}

// typeKey sends the press of a typing key and then its character, as a
// keyboard does.
func typeKey(tt *gowakutest.Tester, k input.KeyCode, mods input.Modifiers, s string) {
	tt.Key(k, mods)
	tt.Type(s)
}

func TestTypingKeys_DoNotRunBareShortcutsInTextFields(t *testing.T) {
	for _, label := range []string{"Name", "Find", "Story", "Notes"} {
		t.Run(label, func(t *testing.T) {
			runs := map[string]int{}
			tt := gowakutest.New(t, musicPlayer{runs: runs}, goFonts)
			tt.Tap(label)
			typeKey(tt, input.KeyA, 0, "a")
			typeKey(tt, input.KeySpace, 0, " ")
			typeKey(tt, input.KeyP, input.ModShift, "P")
			if runs["play"] != 0 || runs["skip"] != 0 {
				t.Errorf("typing ran the shortcuts: %v", runs)
			}
			if n, _ := tt.Find(label); n.Value != "a P" {
				t.Errorf("the field holds %q, want %q", n.Value, "a P")
			}
			tt.Key(input.KeyS, input.ModControl)
			if runs["save"] != 1 {
				t.Errorf("Ctrl+S ran %d times in the field, want once", runs["save"])
			}
		})
	}
}

func TestTypingKeys_ComposingClaimsTheKeys(t *testing.T) {
	runs := map[string]int{}
	tt := gowakutest.New(t, musicPlayer{runs: runs}, goFonts)
	tt.Tap("Name")
	tt.Tree().DispatchComposition(&input.Composition{Text: "か"})
	tt.Pump()
	tt.Key(input.KeySpace, 0)
	tt.Key(input.KeyS, input.ModControl)
	if runs["play"] != 0 || runs["save"] != 0 {
		t.Errorf("keys of a composition ran the shortcuts: %v", runs)
	}
}

func TestTypingKeys_ReachShortcutsOutsideTextFields(t *testing.T) {
	runs := map[string]int{}
	tt := gowakutest.New(t, musicPlayer{runs: runs, readOnly: true}, goFonts)
	tt.Tap("Notes")
	tt.Key(input.KeySpace, 0)
	if runs["play"] != 1 {
		t.Errorf("Space in a read-only editor played %d times, want once", runs["play"])
	}
	tt.Tap("Next")
	tt.Key(input.KeyP, input.ModShift)
	if runs["skip"] != 1 {
		t.Errorf("Shift+P on a button skipped %d times, want once", runs["skip"])
	}
}
