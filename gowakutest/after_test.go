package gowakutest_test

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

const savedFor = 1200 * time.Millisecond

// saveButton says "Saved" for a while after a press, as a toolbar does.
type saveButton struct {
	core.StatefulBase
	// onInit shows "Saved" from InitState on.
	onInit bool
	// cancel, if set, receives the cancel function of the last After.
	cancel *func()
}

type saveButtonState struct {
	core.StateBase
	saved bool
}

func (saveButton) CreateState() core.State { return &saveButtonState{} }

func (s *saveButtonState) InitState() {
	if s.Widget().(saveButton).onInit {
		s.saved = true
		s.After(savedFor, s.unsave)
	}
}

func (s *saveButtonState) unsave() { s.SetState(func() { s.saved = false }) }

func (s *saveButtonState) Build(core.BuildContext) core.Widget {
	label := "Save"
	if s.saved {
		label = "Saved"
	}
	return control.Button{Label: label, OnPress: func() {
		s.SetState(func() { s.saved = true })
		cancel := s.After(savedFor, s.unsave)
		if c := s.Widget().(saveButton).cancel; c != nil {
			*c = cancel
		}
	}}
}

func expectLabel(t *testing.T, tt *gowakutest.Tester, label string) {
	t.Helper()
	if _, ok := tt.Find(label); !ok {
		t.Fatalf("no %q in\n%s", label, tt.Dump())
	}
}

func TestAfter_RunsOnceTheDelayHasPassed(t *testing.T) {
	tt := gowakutest.New(t, saveButton{}, goFonts)
	pressed := tt.Now()
	tt.Tap("Save")
	expectLabel(t, tt, "Saved")
	// The window sleeps while After waits.
	if tt.Tree().Ticking() {
		t.Error("After asks for frames while it waits")
	}
	// The delay counts from the press, before Tap settled the button.
	tt.Advance(pressed + savedFor - time.Millisecond - tt.Now())
	expectLabel(t, tt, "Saved")
	tt.Advance(time.Millisecond)
	expectLabel(t, tt, "Save")
	if _, ok := tt.Tree().NextWake(); ok || tt.Tree().Ticking() {
		t.Error("After keeps a ticker after it ran")
	}
}

func TestAfter_FromInitState(t *testing.T) {
	tt := gowakutest.New(t, saveButton{onInit: true}, goFonts)
	expectLabel(t, tt, "Saved")
	tt.Advance(savedFor)
	expectLabel(t, tt, "Save")
}

func TestAfter_Cancel(t *testing.T) {
	var cancel func()
	tt := gowakutest.New(t, saveButton{cancel: &cancel}, goFonts)
	tt.Tap("Save")
	cancel()
	cancel()
	tt.Advance(2 * savedFor)
	expectLabel(t, tt, "Saved")
}

// toggled shows its child until hide is called.
type toggled struct {
	core.StatefulBase
	child core.Widget
	hide  *func()
}

type toggledState struct {
	core.StateBase
	hidden bool
}

func (toggled) CreateState() core.State { return &toggledState{} }

func (s *toggledState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(toggled)
	*w.hide = func() { s.SetState(func() { s.hidden = true }) }
	if s.hidden {
		return basic.SizedBox{}
	}
	return w.child
}

// A State that leaves the tree takes its After with it: SetState on it would
// panic.
func TestAfter_CancelledWhenTheStateLeaves(t *testing.T) {
	var hide func()
	tt := gowakutest.New(t, toggled{child: saveButton{}, hide: &hide}, goFonts)
	tt.Tap("Save")
	hide()
	tt.Pump()
	tt.Advance(2 * savedFor)
	if _, ok := tt.Tree().NextWake(); ok {
		t.Error("the After of a State that left still waits")
	}
}
