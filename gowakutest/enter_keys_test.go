package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// commitForm commits with Ctrl+Enter, goes to the next item with
// Ctrl+Shift+Enter and Alt+Enter, and has a bare Enter shortcut too, beside
// text fields of every kind.
type commitForm struct {
	gowaku.StatelessBase
	runs map[string]int
}

func (f commitForm) Build(gowaku.BuildContext) gowaku.Widget {
	run := func(name string) func() { return func() { f.runs[name]++ } }
	submit := func(name string) func(string) { return func(string) { f.runs["submit "+name]++ } }
	return interact.Actions{
		Actions: []interact.Action{
			{Label: "Commit", Shortcut: interact.Shortcut{Key: input.KeyEnter, Modifiers: input.ModControl}, OnInvoke: run("commit")},
			{Label: "Next", Shortcut: interact.Shortcut{Key: input.KeyEnter, Modifiers: input.ModControl | input.ModShift}, OnInvoke: run("next")},
			{Label: "Other", Shortcut: interact.Shortcut{Key: input.KeyEnter, Modifiers: input.ModAlt}, OnInvoke: run("other")},
			{Label: "Open", Shortcut: interact.Shortcut{Key: input.KeyEnter}, OnInvoke: run("open")},
			{Label: "Open below", Shortcut: interact.Shortcut{Key: input.KeyEnter, Modifiers: input.ModShift}, OnInvoke: run("open below")},
		},
		Child: basic.Column{Spacing: 8, Children: []gowaku.Widget{
			field.Input{Label: "Title"},
			field.Input{Label: "Subject", OnSubmit: submit("Subject")},
			field.Input{Label: "Message", Multiline: true},
			field.Input{Label: "Chat", Multiline: true, SubmitOnEnter: true, OnSubmit: submit("Chat")},
			basic.SizedBox{Width: 300, Height: 80, Child: editor.Editor{Label: "Notes"}},
		}},
	}
}

var enterLabels = []string{"Title", "Subject", "Message", "Chat", "Notes"}

func TestEnterKeys_CtrlAndAltEnterRunShortcutsInTextFields(t *testing.T) {
	for _, label := range enterLabels {
		t.Run(label, func(t *testing.T) {
			runs := map[string]int{}
			tt := gowakutest.New(t, commitForm{runs: runs}, goFonts)
			tt.Tap(label)
			typeKey(tt, input.KeyA, 0, "a")
			tt.Key(input.KeyEnter, input.ModControl)
			tt.Key(input.KeyEnter, input.ModControl|input.ModShift)
			tt.Key(input.KeyEnter, input.ModAlt)
			if runs["commit"] != 1 || runs["next"] != 1 || runs["other"] != 1 {
				t.Errorf("Ctrl+Enter, Ctrl+Shift+Enter and Alt+Enter ran %v, want each once", runs)
			}
			if runs["submit "+label] != 0 {
				t.Errorf("Enter with Ctrl or Alt submitted the field: %v", runs)
			}
			if n, _ := tt.Find(label); n.Value != "a" {
				t.Errorf("the field holds %q, want %q", n.Value, "a")
			}
		})
	}
}

func TestEnterKeys_PlainEnterStaysWithTheField(t *testing.T) {
	cases := []struct {
		label  string
		mods   input.Modifiers
		value  string
		submit int
	}{
		{"Title", 0, "a", 0},
		{"Title", input.ModShift, "a", 0},
		{"Subject", 0, "a", 1},
		{"Subject", input.ModShift, "a", 1},
		{"Message", 0, "a\n", 0},
		{"Message", input.ModShift, "a\n", 0},
		{"Chat", 0, "a", 1},
		{"Chat", input.ModShift, "a\n", 0},
		{"Notes", 0, "a\n", 0},
		{"Notes", input.ModShift, "a\n", 0},
	}
	for _, c := range cases {
		t.Run(c.label+" "+c.mods.String()+"Enter", func(t *testing.T) {
			runs := map[string]int{}
			tt := gowakutest.New(t, commitForm{runs: runs}, goFonts)
			tt.Tap(c.label)
			typeKey(tt, input.KeyA, 0, "a")
			tt.Key(input.KeyEnter, c.mods)
			if runs["open"] != 0 || runs["open below"] != 0 {
				t.Errorf("Enter in the field ran a shortcut: %v", runs)
			}
			if got := runs["submit "+c.label]; got != c.submit {
				t.Errorf("submitted %d times, want %d", got, c.submit)
			}
			if n, _ := tt.Find(c.label); n.Value != c.value {
				t.Errorf("the field holds %q, want %q", n.Value, c.value)
			}
		})
	}
}

func TestEnterKeys_ComposingKeepsCtrlEnter(t *testing.T) {
	for _, label := range []string{"Subject", "Message", "Notes"} {
		t.Run(label, func(t *testing.T) {
			runs := map[string]int{}
			tt := gowakutest.New(t, commitForm{runs: runs}, goFonts)
			tt.Tap(label)
			tt.Tree().DispatchComposition(&input.Composition{Text: "か"})
			tt.Pump()
			tt.Key(input.KeyEnter, input.ModControl)
			tt.Key(input.KeyEnter, 0)
			if len(runs) != 0 {
				t.Errorf("Enter during a composition ran %v", runs)
			}
			tt.Tree().DispatchComposition(nil)
			tt.Type("か")
			if n, _ := tt.Find(label); n.Value != "か" {
				t.Errorf("the field holds %q, want %q", n.Value, "か")
			}
		})
	}
}
