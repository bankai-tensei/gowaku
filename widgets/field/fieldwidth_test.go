package field_test

import (
	"testing"
	"time"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func priority(label string, hide bool) fieldw.DropdownMenu[int] {
	return fieldw.DropdownMenu[int]{Label: label, HideLabel: hide, Placeholder: "Choose", OnChange: func(int) {},
		Options: []fieldw.DropdownOption[int]{{Value: 1, Label: "Low"}, {Value: 2, Label: "High"}}}
}

// The fields of a form take the width the form gives them, whatever their
// kind, and their own width in a Row.
func TestFields_TakeTheWidthTheyAreGiven(t *testing.T) {
	for _, hide := range []bool{false, true} {
		fields := func() []core.Widget {
			return []core.Widget{
				fieldw.Input{Label: "Name", HideLabel: hide},
				fieldw.DatePicker{Label: "Due", HideLabel: hide, OnChange: func(time.Time) {}},
				priority("Priority", hide),
			}
		}
		for _, cross := range []layout.CrossAxisAlignment{layout.CrossStart, layout.CrossStretch} {
			tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 400,
				Child: basic.Column{Cross: cross, Children: fields()}}}, pressableFonts)
			for _, name := range []string{"Name", "Due", "Priority"} {
				var n *gowakutest.Node
				for _, m := range tt.FindAll(name) {
					if m.Role != semantics.RoleText {
						n = m
					}
				}
				if n == nil {
					t.Fatalf("no field %s:\n%s", name, tt.Dump())
				}
				if n.Rect.W != 400 {
					t.Errorf("hide %v, cross %v: %s is %v wide, want 400:\n%s", hide, cross, name, n.Rect.W, tt.Dump())
				}
			}
		}
	}
	// In a Row, which leaves the width open, a dropdown is as wide as its
	// longest option, and a date field as wide as a date.
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 600, Child: basic.Row{Children: []core.Widget{
		priority("Priority", false),
		fieldw.DatePicker{Label: "Due", OnChange: func(time.Time) {}},
	}}}}, pressableFonts)
	for _, name := range []string{"Priority", "Due"} {
		n, ok := tt.FindRole(semantics.RoleComboBox, name)
		if !ok {
			t.Fatalf("no field %s:\n%s", name, tt.Dump())
		}
		if n.Rect.W <= 0 || n.Rect.W >= 300 {
			t.Errorf("%s in a Row is %v wide, want its own width", name, n.Rect.W)
		}
	}
}

// The placeholder of a dropdown is treated as those of Input and DatePicker:
// no contrast warning, and it names or hints at the field.
func TestDropdownMenu_PlaceholderAsTheOtherFields(t *testing.T) {
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 300, Child: basic.Column{Children: []core.Widget{
		fieldw.Input{Label: "Name", Placeholder: "Your name"},
		fieldw.DatePicker{Label: "Due", Placeholder: "Pick a day", OnChange: func(time.Time) {}},
		priority("Priority", false),
		fieldw.DropdownMenu[int]{Placeholder: "Size", OnChange: func(int) {}, Options: []fieldw.DropdownOption[int]{{Value: 1, Label: "S"}}},
	}}}}, pressableFonts)
	for _, w := range tt.Warnings() {
		t.Errorf("%v", w)
	}
	n, ok := tt.FindRole(semantics.RoleComboBox, "Priority")
	if !ok {
		t.Fatalf("no dropdown:\n%s", tt.Dump())
	}
	if n.Hint != "Choose" || n.Value != "" {
		t.Errorf("hint %q, value %q: the placeholder is the hint of a named field", n.Hint, n.Value)
	}
	if _, ok := tt.FindRole(semantics.RoleText, "Choose"); ok {
		t.Errorf("the placeholder is read as text of its own:\n%s", tt.Dump())
	}
	if _, ok := tt.FindRole(semantics.RoleComboBox, "Size"); !ok {
		t.Errorf("the placeholder names a field without a Label:\n%s", tt.Dump())
	}
}
