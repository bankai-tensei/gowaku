package field

import (
	"regexp"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const tooShort = "3 文字以上にしてください"

// nameForm is a form of a name of three letters or more, a free note, and a
// way to submit it.
type nameForm struct {
	ctrl      FormController
	submitted int
}

func (f *nameForm) widget() core.Widget {
	return formHolder[[2]string]{Build: func(v [2]string, set func([2]string)) core.Widget {
		return Form{Controller: &f.ctrl, OnSubmit: func() { f.submitted++ }, Child: basic.Column{Spacing: 8, Children: []core.Widget{
			FormItem[string]{Value: v[0], Required: true, Validators: []func(string) string{MinLength(3, tooShort)}, Child: Input{
				Label: "Name", Value: v[0], OnChange: func(s string) { set([2]string{s, v[1]}) },
			}},
			FormItem[string]{Value: v[1], Required: true, Child: Input{
				Label: "Note", Value: v[1], OnChange: func(s string) { set([2]string{v[0], s}) },
			}},
			control.Button{Label: "Send", OnPress: func() { f.ctrl.Submit() }},
		}}}
	}}
}

// errorOf returns the error a screen reader hears for the text field labeled
// label, and checks that the error text below it says the same.
func (r *fieldRig) errorOf(label string) string {
	r.T.Helper()
	hint := r.must(semantics.RoleTextField, label).Hint
	if hint != "" && r.Find(semantics.RoleText, hint) == nil {
		r.T.Errorf("the error %q of %s must show below it", hint, label)
	}
	return hint
}

func (r *fieldRig) wantError(label, want string) {
	r.T.Helper()
	if got := r.errorOf(label); got != want {
		r.T.Errorf("%s: error %q, want %q", label, got, want)
	}
}

func (r *fieldRig) red() bool {
	danger := theme.Default().Light.Color.Danger
	for _, op := range wtest.Ops(r.tree, r.Size) {
		if op.Color == danger || op.BorderColor == danger {
			return true
		}
	}
	return false
}

func TestFormField_WaitsForTheBlurThenFollowsEveryKeystroke(t *testing.T) {
	var f nameForm
	r := newFieldRig(t, f.widget())
	r.focusField("Name")
	r.typeText("a")
	r.typeText("b")
	r.wantError("Name", "")
	if r.red() {
		t.Fatal("a field must not turn red while it is typed into for the first time")
	}
	r.Key(input.KeyTab, 0)
	r.wantFocus(semantics.RoleTextField, "Note")
	r.wantError("Name", tooShort)
	r.wantError("Note", "")
	if st := r.must(semantics.RoleTextField, "Name").State; st&semantics.Invalid == 0 || st&semantics.Required == 0 {
		t.Errorf("a required field with an error must be invalid and required, got %b", st)
	}
	if st := r.must(semantics.RoleTextField, "Note").State; st&semantics.Invalid != 0 || st&semantics.Required == 0 {
		t.Errorf("a required field not judged yet must be required only, got %b", st)
	}
	if !r.red() {
		t.Error("a field with an error must show red")
	}
	r.focusField("Name")
	r.Key(input.KeyEnd, 0)
	r.typeText("c")
	r.wantError("Name", "")
	r.Key(input.KeyBackspace, 0)
	r.wantError("Name", tooShort)
	r.typeText("d")
	r.wantError("Name", "")
	// Leaving an empty field that is required is an error too.
	r.focusField("Note")
	r.Key(input.KeyTab, 0)
	r.wantError("Note", "Required")
	if f.submitted != 0 {
		t.Error("nothing was submitted")
	}
}

func TestForm_SubmitShowsEveryErrorAndFocusesTheFirst(t *testing.T) {
	var f nameForm
	r := newFieldRig(t, f.widget())
	r.act(r.must(semantics.RoleButton, "Send"), semantics.ActionTap)
	r.wantError("Name", "Required")
	r.wantError("Note", "Required")
	r.wantFocus(semantics.RoleTextField, "Name")
	if f.submitted != 0 {
		t.Fatal("a wrong form must not be submitted")
	}
	r.typeText("Taro")
	r.wantError("Name", "")
	r.focusField("Note")
	r.typeText(" ")
	r.wantError("Note", "Required")
	r.typeText("x")
	r.wantError("Note", "")
	if !f.ctrl.Submit() || f.submitted != 1 {
		t.Errorf("a right form must be submitted, got %d", f.submitted)
	}
}

func TestForm_EnterSubmitsAndResetHidesTheErrors(t *testing.T) {
	var f nameForm
	r := newFieldRig(t, f.widget())
	r.focusField("Note")
	r.typeText("x")
	r.Key(input.KeyEnter, 0)
	r.wantError("Name", "Required")
	r.wantFocus(semantics.RoleTextField, "Name")
	f.ctrl.Reset()
	r.frame()
	r.wantError("Name", "")
	r.wantError("Note", "")
	r.typeText("Hanako")
	r.Key(input.KeyEnter, 0)
	if f.submitted != 1 {
		t.Errorf("Enter must submit, got %d", f.submitted)
	}
}

func TestFormField_TheListOfADropdownIsNotABlur(t *testing.T) {
	r := newFieldRig(t, formHolder[string]{Build: func(v string, set func(string)) core.Widget {
		return basic.Column{Spacing: 8, Children: []core.Widget{
			FormItem[string]{Value: v, Required: true, Child: DropdownMenu[string]{
				Label: "Fruit", Value: v, Options: fruits(), OnChange: set,
			}},
			control.Button{Label: "next", OnPress: func() {}},
		}}
	}})
	hint := func() string { return r.must(semantics.RoleComboBox, "Fruit").Hint }
	r.Key(input.KeyTab, 0)
	r.Key(input.KeySpace, 0)
	r.wantFocus(semantics.RoleListItem, "Apple")
	if hint() != "" {
		t.Fatal("opening the list must not judge the field")
	}
	r.Key(input.KeyEscape, 0)
	r.Key(input.KeyTab, 0)
	r.wantFocus(semantics.RoleButton, "next")
	if hint() != "Choose one" || r.Find(semantics.RoleText, "Choose one") == nil {
		t.Fatalf("leaving it empty must show the error, got %q", hint())
	}
	if st := r.must(semantics.RoleComboBox, "Fruit").State; st&(semantics.Invalid|semantics.Required) != semantics.Invalid|semantics.Required {
		t.Errorf("the empty dropdown must be invalid and required, got %b", st)
	}
	r.Key(input.KeyTab, input.ModShift)
	r.Key(input.KeyDown, 0)
	if hint() != "" {
		t.Errorf("a choice must clear the error at once, got %q", hint())
	}
}

func TestFormField_CheckboxIsJudgedOnSubmit(t *testing.T) {
	var ctrl FormController
	r := newFieldRig(t, formHolder[bool]{Build: func(v bool, set func(bool)) core.Widget {
		return Form{Controller: &ctrl, Child: FormItem[bool]{
			Value: v, Validators: []func(bool) string{func(ok bool) string {
				if !ok {
					return "同意が必要です"
				}
				return ""
			}},
			Child: control.Checkbox{Label: "同意する", Value: v, OnChange: set},
		}}
	}})
	box := r.must(semantics.RoleCheckbox, "同意する")
	r.click(box)
	r.click(box)
	if r.Find(semantics.RoleText, "同意が必要です") != nil {
		t.Fatal("no error before a submit")
	}
	if ctrl.Submit() {
		t.Fatal("an unchecked box must fail")
	}
	r.frame()
	r.wantFocus(semantics.RoleCheckbox, "同意する")
	if n := r.Find(semantics.RoleText, "同意が必要です"); n == nil || n.Live != semantics.LivePolite {
		t.Fatalf("the error must show and be announced: %+v", n)
	}
	r.click(box)
	if r.Find(semantics.RoleText, "同意が必要です") != nil {
		t.Error("checking the box must clear the error")
	}
}

func TestFormField_SpeaksTheLocale(t *testing.T) {
	var ctrl FormController
	r := newFieldRig(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.Japanese}, Child: Form{
		Controller: &ctrl,
		Child:      FormItem[string]{Required: true, Child: Input{Label: "名前"}},
	}})
	ctrl.Submit()
	r.frame()
	r.wantError("名前", "入力してください")
}

func TestValidators(t *testing.T) {
	word := regexp.MustCompile(`[a-z]+`)
	cases := []struct {
		name  string
		v     func(string) string
		value string
		ok    bool
	}{
		{"MinLength: 足りない", MinLength(3, "x"), "ab", false},
		{"MinLength: 文字で数える", MinLength(3, "x"), "あいう", true},
		{"MinLength: 空は通す", MinLength(3, "x"), "", true},
		{"Pattern: 全体が合う", Pattern(word, "x"), "abc", true},
		{"Pattern: 一部だけは不可", Pattern(word, "x"), "abc1", false},
		{"Pattern: 空は通す", Pattern(word, "x"), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v(tc.value) == ""; got != tc.ok {
				t.Errorf("got %v, want %v", got, tc.ok)
			}
		})
	}
	wtest.ExpectPanic(t, "gowaku: MinLength needs a message", func() { MinLength(1, "") })
	wtest.ExpectPanic(t, "gowaku: FormController.Submit called", func() { (&FormController{}).Submit() })
}
