package field_test

import (
	"testing"
	"time"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// hideLabelCases are the fields that draw their Label as a caption above
// themselves.
var hideLabelCases = []struct {
	name string
	w    func(label string, hide bool) core.Widget
}{
	{"Input", func(l string, h bool) core.Widget { return fieldw.Input{Label: l, HideLabel: h} }},
	{"Multiline", func(l string, h bool) core.Widget { return fieldw.Input{Label: l, HideLabel: h, Multiline: true} }},
	{"Search", func(l string, h bool) core.Widget { return fieldw.Search{Label: l, HideLabel: h} }},
	{"DropdownMenu", func(l string, h bool) core.Widget {
		return fieldw.DropdownMenu[int]{Label: l, HideLabel: h, Value: 1, OnChange: func(int) {},
			Options: []fieldw.DropdownOption[int]{{Value: 1, Label: "Low"}, {Value: 2, Label: "High"}}}
	}},
	{"NumberField", func(l string, h bool) core.Widget {
		return fieldw.NumberField{Label: l, HideLabel: h, Value: 3, OnChange: func(float64) {}}
	}},
	{"NumberField disabled", func(l string, h bool) core.Widget { return fieldw.NumberField{Label: l, HideLabel: h, Value: 3} }},
	{"DatePicker", func(l string, h bool) core.Widget {
		return fieldw.DatePicker{Label: l, HideLabel: h, OnChange: func(time.Time) {}}
	}},
	{"TimePicker", func(l string, h bool) core.Widget {
		return fieldw.TimePicker{Label: l, HideLabel: h, OnChange: func(time.Duration) {}}
	}},
	{"Autocomplete", func(l string, h bool) core.Widget {
		return fieldw.Autocomplete[string]{Label: l, HideLabel: h, Options: []string{"Low", "High"}}
	}},
	{"TokenField", func(l string, h bool) core.Widget {
		return fieldw.TokenField{Label: l, HideLabel: h, OnChange: func([]string) {}}
	}},
}

// bottom is the lowest edge of n and the nodes below it.
func bottom(n *gowakutest.Node) float32 {
	b := n.Rect.Y + n.Rect.H
	for _, c := range n.Children {
		b = max(b, bottom(c))
	}
	return b
}

func TestHideLabel_NamesTheFieldWithoutACaption(t *testing.T) {
	mount := func(t *testing.T, w core.Widget) *gowakutest.Tester {
		return gowakutest.New(t, basic.Align{Child: basic.SizedBox{Width: 240, Child: w}}, pressableFonts)
	}
	for _, c := range hideLabelCases {
		t.Run(c.name, func(t *testing.T) {
			tt := mount(t, c.w("Volume", true))
			named := 0
			for _, n := range tt.FindAll("Volume") {
				if n.Role == semantics.RoleText {
					t.Errorf("a caption is drawn:\n%s", tt.Dump())
				} else {
					named++
				}
			}
			if named == 0 {
				t.Errorf("no control is named Volume:\n%s", tt.Dump())
			}
			for _, w := range tt.Warnings() {
				if w.Kind == gowakutest.WarnUnlabeled {
					t.Errorf("%v:\n%s", w, tt.Dump())
				}
			}
			// The hidden caption takes no room: the field is as tall as
			// one without a Label, and shorter than one with a caption.
			hidden := bottom(tt.Semantics())
			bare := bottom(mount(t, c.w("", false)).Semantics())
			captioned := bottom(mount(t, c.w("Volume", false)).Semantics())
			if hidden != bare || captioned <= hidden {
				t.Errorf("bottom with HideLabel %v, without a Label %v, with a caption %v", hidden, bare, captioned)
			}
		})
	}
}

func TestHideLabel_WithoutLabelWarns(t *testing.T) {
	tt := gowakutest.New(t, basic.Align{Child: basic.SizedBox{Width: 240, Child: fieldw.Input{HideLabel: true}}}, pressableFonts)
	unlabeled := 0
	for _, w := range tt.Warnings() {
		if w.Kind == gowakutest.WarnUnlabeled {
			unlabeled++
		}
	}
	if unlabeled != 1 {
		t.Errorf("an Input with neither Label nor Placeholder gave %d unlabeled warnings, want 1", unlabeled)
	}
}
