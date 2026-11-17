package field

import (
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// submitted submits the form of ctrl, which must fail, and draws the errors.
func (r *fieldRig) submitFails(ctrl *FormController) {
	r.T.Helper()
	if ctrl.Submit() {
		r.T.Fatal("an empty required field must fail the submit")
	}
	r.frame()
}

// wantInvalid checks that the node of role labeled label says err as an
// invalid field, that err shows below it, and that the field is drawn red.
func (r *fieldRig) wantInvalid(role semantics.Role, label, err string) {
	r.T.Helper()
	n := r.must(role, label)
	if n.Hint != err || n.State&semantics.Invalid == 0 {
		r.T.Errorf("%s: hint %q state %b, want %q and invalid", label, n.Hint, n.State, err)
	}
	if r.Find(semantics.RoleText, err) == nil {
		r.T.Errorf("%s: the error %q must show below it", label, err)
	}
	if !r.red() {
		r.T.Errorf("%s: the field must turn red", label)
	}
}

// Every field control in a FormItem shows its error the same way: a red
// border, the error as its hint and the Invalid state.
func TestFormItem_EveryFieldControlTurnsRed(t *testing.T) {
	never := func(string) string { return "wrong" }
	cases := []struct {
		name  string
		role  semantics.Role
		label string
		item  core.Widget
	}{
		{"DatePicker", semantics.RoleComboBox, "Birth date", FormItem[time.Time]{Required: true, Child: DatePicker{
			Label: "Birth date", OnChange: func(time.Time) {},
		}}},
		{"TimePicker", semantics.RoleGroup, "Start", FormItem[time.Duration]{
			Validators: []func(time.Duration) string{func(time.Duration) string { return "wrong" }},
			Child:      TimePicker{Label: "Start", OnChange: func(time.Duration) {}},
		}},
		{"ColorPicker", semantics.RoleGroup, "Color", FormItem[geom.Color]{
			Validators: []func(geom.Color) string{func(geom.Color) string { return "wrong" }},
			Child:      ColorPicker{Label: "Color", OnChange: func(geom.Color) {}},
		}},
		{"NumberField", semantics.RoleTextField, "Count", FormItem[float64]{
			Validators: []func(float64) string{func(float64) string { return "wrong" }},
			Child:      NumberField{Label: "Count", OnChange: func(float64) {}},
		}},
		{"TokenField", semantics.RoleList, "Tags", FormItem[string]{Validators: []func(string) string{never}, Child: TokenField{
			Label: "Tags", OnChange: func([]string) {},
		}}},
		{"Search", semantics.RoleTextField, "Search", FormItem[string]{Validators: []func(string) string{never}, Child: Search{
			OnChange: func(string) {},
		}}},
		{"Autocomplete", semantics.RoleComboBox, "City", FormItem[string]{Validators: []func(string) string{never}, Child: Autocomplete[string]{
			Label: "City", OnChange: func(string) {},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ctrl FormController
			r := newFieldRig(t, Form{Controller: &ctrl, Child: tc.item})
			if r.red() {
				t.Fatal("no error before a submit")
			}
			r.submitFails(&ctrl)
			err := "wrong"
			if tc.name == "DatePicker" {
				err = "Choose one"
			}
			r.wantInvalid(tc.role, tc.label, err)
		})
	}
}

// The error of Required names what the control asks for.
func TestFormItem_RequiredSpeaksOfTheControl(t *testing.T) {
	fruit := func(v string) core.Widget {
		return FormItem[string]{Value: v, Required: true, Child: DropdownMenu[string]{Label: "Fruit", Options: fruits(), OnChange: func(string) {}}}
	}
	cases := []struct {
		name     string
		item     core.Widget
		en, ja   string
		override bool
	}{
		{"Input", FormItem[string]{Required: true, Child: Input{Label: "Name"}}, "Required", "入力してください", false},
		{"DropdownMenu", fruit(""), "Choose one", "選択してください", false},
		{"DatePicker", FormItem[time.Time]{Required: true, Child: DatePicker{Label: "Birth date", OnChange: func(time.Time) {}}}, "Choose one", "選択してください", false},
		{"RadioGroup", FormItem[string]{Required: true, Child: control.RadioGroup[string]{
			Label: "Plan", Options: []control.RadioOption[string]{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}, OnChange: func(string) {},
		}}, "Choose one", "選択してください", false},
		{"Chip", FormItem[bool]{Required: true, Child: control.Chip{Label: "Morning", OnChange: func(bool) {}}}, "Choose one", "選択してください", false},
		{"Checkbox", FormItem[bool]{Required: true, Child: control.Checkbox{Label: "I agree", OnChange: func(bool) {}}}, "Check this box", "チェックしてください", false},
		{"Switch", FormItem[bool]{Required: true, Child: control.Switch{Label: "Notify", OnChange: func(bool) {}}}, "Check this box", "チェックしてください", false},
		{"RequiredMessage", FormItem[bool]{Required: true, RequiredMessage: "規約への同意が必要です", Child: control.Checkbox{
			Label: "I agree", OnChange: func(bool) {},
		}}, "規約への同意が必要です", "規約への同意が必要です", true},
	}
	for _, tc := range cases {
		for _, lang := range []struct {
			tag  language.Tag
			want string
		}{{language.English, tc.en}, {language.Japanese, tc.ja}} {
			t.Run(tc.name+"/"+lang.tag.String(), func(t *testing.T) {
				var ctrl FormController
				r := newFieldRig(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: lang.tag}, Child: Form{
					Controller: &ctrl, Child: tc.item,
				}})
				r.submitFails(&ctrl)
				if r.Find(semantics.RoleText, lang.want) == nil {
					t.Errorf("the error must say %q", lang.want)
				}
			})
		}
	}
}

// The keyboard moving from one part of a TimePicker to the next does not
// leave the field; leaving the last part does.
func TestFormItem_TimePickerPartsAreOneField(t *testing.T) {
	early := func(v time.Duration) string {
		if v < 9*time.Hour {
			return "From 9:00"
		}
		return ""
	}
	r := newFieldRig(t, formHolder[time.Duration]{Build: func(v time.Duration, set func(time.Duration)) core.Widget {
		return basic.Column{Spacing: 8, Children: []core.Widget{
			FormItem[time.Duration]{Value: v, Validators: []func(time.Duration) string{early}, Child: TimePicker{
				Label: "Start", Value: v, HourCycle: HourCycle24, OnChange: set,
			}},
			control.Button{Label: "next", OnPress: func() {}},
		}}
	}})
	r.Key(input.KeyTab, 0)
	r.wantFocus(semantics.RoleStepper, "Hour")
	r.Key(input.KeyRight, 0)
	r.wantFocus(semantics.RoleStepper, "Minute")
	r.frame()
	if r.Find(semantics.RoleText, "From 9:00") != nil {
		t.Fatal("moving between the parts must not judge the field")
	}
	r.Key(input.KeyTab, 0)
	r.wantFocus(semantics.RoleButton, "next")
	r.frame()
	r.wantInvalid(semantics.RoleGroup, "Start", "From 9:00")
}

// Opening the calendar of a DatePicker is not a blur; closing it without a
// day is.
func TestFormItem_TheCalendarOfADatePickerIsNotABlur(t *testing.T) {
	r := newFieldRig(t, formHolder[time.Time]{Build: func(v time.Time, set func(time.Time)) core.Widget {
		return FormItem[time.Time]{Value: v, Required: true, Child: DatePicker{
			Label: "Birth date", Value: v, Today: day(2026, 11, 16), OnChange: set,
		}}
	}})
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnter, 0)
	r.wantFocus(semantics.RoleGridCell, "")
	if r.Find(semantics.RoleText, "Choose one") != nil {
		t.Fatal("opening the calendar must not judge the field")
	}
	r.Key(input.KeyEscape, 0)
	r.wantFocus(semantics.RoleComboBox, "Birth date")
	r.frame()
	if r.Find(semantics.RoleText, "Choose one") != nil {
		t.Fatal("the keyboard came back to the field: no blur")
	}
	r.Key(input.KeyEnter, 0)
	r.Key(input.KeyEnter, 0)
	r.frame()
	if n := r.must(semantics.RoleComboBox, "Birth date"); n.Value == "" || n.Hint != "" {
		t.Errorf("choosing today must fill the field without an error, got %+v", n)
	}
}

// numberForm is a form of a count that must be above zero, submitted by Enter.
type numberForm struct {
	submitted int
	// seen is the count the form held when it was submitted.
	seen float64
}

func (f *numberForm) widget() core.Widget {
	return formHolder[float64]{Build: func(v float64, set func(float64)) core.Widget {
		return Form{OnSubmit: func() { f.submitted++; f.seen = v }, Child: FormItem[float64]{
			Value: v,
			Validators: []func(float64) string{func(n float64) string {
				if n <= 0 {
					return "Above zero"
				}
				return ""
			}},
			Child: NumberField{Label: "Count", Value: v, OnChange: set},
		}}
	}}
}

// Enter in a NumberField submits the form with the value just typed, not
// the one before it.
func TestNumberField_EnterSubmitsTheFormWithTheNewValue(t *testing.T) {
	var f numberForm
	r := newFieldRig(t, f.widget())
	r.focusField("Count")
	r.replaceAll("5")
	r.Key(input.KeyEnter, 0)
	r.settle()
	if f.submitted != 1 || f.seen != 5 {
		t.Fatalf("Enter must submit the form holding 5, got %d submits holding %v", f.submitted, f.seen)
	}
	if r.Find(semantics.RoleText, "Above zero") != nil {
		t.Error("the new value is right: no error")
	}
}

// Enter in any single-line text field of a FormItem submits its Form; outside
// a FormItem it submits nothing.
func TestForm_EnterSubmitsFromEveryTextField(t *testing.T) {
	cases := []struct {
		name, label string
		field       core.Widget
	}{
		{"Input", "Name", Input{Label: "Name", OnChange: func(string) {}}},
		{"Search", "Search", Search{OnChange: func(string) {}}},
		{"NumberField", "Count", NumberField{Label: "Count", OnChange: func(float64) {}}},
		{"Autocomplete", "City", Autocomplete[string]{Label: "City", OnChange: func(string) {}}},
		{"TokenField", "Tags", TokenField{Label: "Tags", OnChange: func([]string) {}}},
	}
	for _, tc := range cases {
		for _, wrapped := range []bool{false, true} {
			name := tc.name
			if wrapped {
				name += "/FormItem"
			}
			t.Run(name, func(t *testing.T) {
				submitted := 0
				child := tc.field
				if wrapped {
					child = FormItem[string]{Child: child}
				}
				r := newFieldRig(t, Form{OnSubmit: func() { submitted++ }, Child: child})
				r.Key(input.KeyTab, 0)
				r.Key(input.KeyEnter, 0)
				r.settle()
				if want := map[bool]int{false: 0, true: 1}[wrapped]; submitted != want {
					t.Errorf("Enter in %s submitted %d times, want %d", tc.label, submitted, want)
				}
			})
		}
	}
}
