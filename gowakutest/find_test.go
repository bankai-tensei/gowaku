package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// TestFind_PrefersWhatCanAct: a heading and the caption of a field share the
// label of the field, and come first in reading order; Tap must still reach
// the field.
func TestFind_PrefersWhatCanAct(t *testing.T) {
	value := ""
	tt := gowakutest.New(t, basic.Column{Children: []gowaku.Widget{
		interact.Semantics{Role: semantics.RoleHeading, Label: "Notes", Child: basic.Text{Content: "Notes"}},
		notes{value: &value},
	}}, goFonts)
	n, ok := tt.Find("Notes")
	if !ok || n.Role != semantics.RoleTextField {
		t.Fatalf("Find returned %v, want the text field:\n%s", n, tt.Dump())
	}
	tt.Tap("Notes")
	tt.Type("hi")
	if value != "hi" {
		t.Errorf("the field holds %q after a tap and typing, want %q:\n%s", value, "hi", tt.Dump())
	}
}

// TestFind_FirstAmongEquals: of two buttons of the same label, the first in
// reading order wins, as before.
func TestFind_FirstAmongEquals(t *testing.T) {
	var got []int
	tt := gowakutest.New(t, basic.VStack(
		basic.Text{Content: "OK"},
		control.Button{Label: "OK", OnPress: func() { got = append(got, 1) }},
		control.Button{Label: "OK", OnPress: func() { got = append(got, 2) }},
	), goFonts)
	tt.Tap("OK")
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("pressed %v, want the first button", got)
	}
}
