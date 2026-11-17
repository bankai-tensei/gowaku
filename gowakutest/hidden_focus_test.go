package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// fading is three buttons in a row, of which the middle one can be hidden.
type fading struct {
	gowaku.StatefulBase
	presses map[string]int
	hide    *func()
}

type fadingState struct {
	gowaku.StateOf[fading]
	hidden bool
}

func (fading) CreateState() gowaku.State { return &fadingState{} }

func (s *fadingState) InitState() {
	*s.Widget().hide = func() { s.SetState(func() { s.hidden = true }) }
}

func (s *fadingState) Build(gowaku.BuildContext) gowaku.Widget {
	p := s.Widget().presses
	button := func(label string) gowaku.Widget {
		return control.Button{Label: label, OnPress: func() { p[label]++ }}
	}
	var opacity float32 = 1
	if s.hidden {
		opacity = 0
	}
	return basic.HStack(button("A"), basic.Opacity{Opacity: opacity, Child: button("B")}, button("C"))
}

// focusB returns a tester whose middle button has the focus, and the function
// that hides it.
func focusB(t *testing.T) (*gowakutest.Tester, map[string]int, func()) {
	t.Helper()
	presses := map[string]int{}
	var hide func()
	tt := gowakutest.New(t, fading{presses: presses, hide: &hide}, goFonts)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeySpace, 0)
	if presses["B"] != 1 {
		t.Fatalf("B does not have the focus: %v", presses)
	}
	return tt, presses, hide
}

func TestHiddenFocus_KeysDoNotReachAHiddenButton(t *testing.T) {
	tt, presses, hide := focusB(t)
	hide()
	tt.Settle()
	tt.Key(input.KeySpace, 0)
	tt.Key(input.KeyEnter, 0)
	if presses["B"] != 1 {
		t.Errorf("the hidden button was pressed: %v", presses)
	}
}

func TestHiddenFocus_TabGoesOnFromTheHiddenButton(t *testing.T) {
	tt, presses, hide := focusB(t)
	hide()
	tt.Settle()
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeySpace, 0)
	if presses["C"] != 1 || presses["A"] != 0 {
		t.Errorf("Tab after hiding B did not go to C: %v", presses)
	}
}

func TestHiddenFocus_ShiftTabGoesBackFromTheHiddenButton(t *testing.T) {
	tt, presses, hide := focusB(t)
	hide()
	tt.Settle()
	tt.Key(input.KeyTab, input.ModShift)
	tt.Key(input.KeySpace, 0)
	if presses["A"] != 1 || presses["C"] != 0 {
		t.Errorf("Shift+Tab after hiding B did not go to A: %v", presses)
	}
}
