package field_test

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
)

// dialogOpener opens a dialog with a field when open is called, as a command
// palette does.
type dialogOpener struct {
	core.StatelessBase
	field *fieldw.InputController
	open  *func()
}

func (o dialogOpener) Build(ctx core.BuildContext) core.Widget {
	*o.open = func() {
		overlay.ShowDialog(ctx, func(func()) core.Widget {
			return overlay.Dialog{Content: fieldw.Input{Controller: o.field, Label: "Query"}}
		})
	}
	return basic.SizedBox{}
}

// A field that takes the keyboard on the first frame of a dialog keeps it while
// the dialog fades in: the fade must not count as hiding the field from the
// focus (core.Hider), or the next frame takes the keyboard away.
func TestDialog_FieldFocusedAsItOpensKeepsTheKeyboard(t *testing.T) {
	field := &fieldw.InputController{}
	var open func()
	tt := gowakutest.New(t, dialogOpener{field: field, open: &open}, pressableFonts)
	open()
	// The first frame of the dialog, at the time it opened: fade 0.
	tt.Pump()
	field.Focus()
	if !field.Focused() {
		t.Fatal("the field did not take the keyboard")
	}
	// The next frame, before the clock moves.
	tt.Pump()
	if !field.Focused() {
		t.Error("the field lost the keyboard on the next frame of the fade")
	}
	tt.Settle()
	if !field.Focused() {
		t.Error("the field lost the keyboard once the dialog came in")
	}
}

// renamePage opens a Rename dialog from a button, as from a menu item, the way
// an application writes it: the controller is asked before the field exists.
type renamePage struct {
	core.StatefulBase
	// how is when the page asks the controller: "before" or "after"
	// ShowDialog, or "autofocus" for Input.Autofocus instead; sheet opens a
	// sheet rather than a dialog.
	how   string
	sheet bool
	name  *string
}

type renamePageState struct {
	core.StateBase
	ctl fieldw.InputController
}

func (renamePage) CreateState() core.State { return &renamePageState{} }

func (s *renamePageState) Build(ctx core.BuildContext) core.Widget {
	p := s.Widget().(renamePage)
	open := func() {
		if p.how == "before" {
			s.ctl.SelectAll()
			s.ctl.Focus()
		}
		content := func(dismiss func()) core.Widget {
			return fieldw.Input{
				Label: "Name", Value: *p.name, Controller: &s.ctl, Autofocus: p.how == "autofocus",
				OnChange: func(v string) { *p.name = v },
				OnSubmit: func(string) { dismiss() },
			}
		}
		if p.sheet {
			overlay.ShowSheet(ctx, overlay.Sheet{Label: "Rename"}, content)
		} else {
			overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
				return overlay.Dialog{Title: "Rename", Content: content(dismiss)}
			})
		}
		if p.how == "after" {
			s.ctl.Focus()
			s.ctl.SelectAll()
		}
	}
	return control.Button{Label: "Rename…", OnPress: open}
}

// A field in a dialog that a callback opens gets the keyboard, whether the
// callback asks its controller before or after ShowDialog, or the field has
// Autofocus: the request of the field wins over the autofocus of the dialog.
func TestDialog_FieldAskedForTheKeyboardBeforeItExistsGetsIt(t *testing.T) {
	for _, sheet := range []bool{false, true} {
		for _, how := range []string{"before", "after", "autofocus"} {
			name := how
			if sheet {
				name += " in a sheet"
			}
			t.Run(name, func(t *testing.T) {
				value := "draft.txt"
				tt := gowakutest.New(t, renamePage{how: how, sheet: sheet, name: &value}, pressableFonts)
				tt.Tap("Rename…")
				tt.Type("x")
				want := "x"
				if how == "autofocus" {
					// Autofocus gives the keyboard and leaves the caret where it was.
					want = "xdraft.txt"
				}
				if value != want {
					t.Fatalf("the text is %q after typing, want %q", value, want)
				}
				// Enter submits, which closes the dialog, and the keyboard goes back
				// to the button that opened it.
				tt.Key(input.KeyEnter, 0)
				if _, ok := tt.Find("Name"); ok {
					t.Error("Enter did not close the dialog")
				}
			})
		}
	}
}

// pressableFonts keeps the tests independent of the fonts of the machine.
var pressableFonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}
