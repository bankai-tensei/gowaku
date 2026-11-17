package golden

import (
	"regexp"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// fieldScenes returns the scenes of the field widgets.
func fieldScenes() []Scene {
	s := make([]Scene, 0, 8)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		var form field.FormController
		s = append(s,
			Scene{Name: "fields" + suffix, Width: 720, Height: 450, Scale: 1.5, Dark: dark, Root: fieldPage},
			Scene{Name: "dropdown-open" + suffix, Width: 720, Height: 450, Scale: 1.5, Dark: dark, Root: fieldPage, Act: openDropdown},
			Scene{Name: "autocomplete-open" + suffix, Width: 720, Height: 450, Scale: 1.5, Dark: dark, Root: fieldPage, Act: typeCity},
			Scene{
				Name: "form-error" + suffix, Width: 720, Height: 450, Scale: 1.5, Dark: dark,
				Root: func() core.Widget { return formPage(&form) },
				Act:  func(*core.Tree, core.BuildContext) { form.Submit() },
			},
		)
	}
	return s
}

func cities() []string {
	return []string{"Amsterdam", "Athens", "Berlin", "Lisbon", "Madrid", "Paris", "Prague", "Rome", "Vienna", "Warsaw"}
}

func plans() []field.DropdownOption[string] {
	return []field.DropdownOption[string]{
		{Value: "free", Label: "Free"},
		{Value: "team", Label: "Team"},
		{Value: "business", Label: "Business"},
		{Value: "school", Label: "School", Disabled: true},
		{Value: "enterprise", Label: "Enterprise"},
	}
}

// fieldPage shows the fields at rest: search fields with and without text,
// dropdowns and an autocomplete.
func fieldPage() core.Widget {
	return basic.Padding{Insets: geom.All(16), Child: basic.Row{Cross: layout.CrossStart, Spacing: 24, Children: []core.Widget{
		basic.SizedBox{Width: 220, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 16, Children: []core.Widget{
			field.Search{Value: "gowaku", OnChange: func(string) {}},
			field.Search{OnChange: func(string) {}},
			field.Autocomplete[string]{Label: "City", Placeholder: "Type a city", Options: cities()},
		}}},
		basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 16, Children: []core.Widget{
			field.DropdownMenu[string]{Label: "Plan", Value: "team", Options: plans(), OnChange: func(string) {}},
			field.DropdownMenu[string]{Placeholder: "Choose a plan", Options: plans(), OnChange: func(string) {}},
			field.DropdownMenu[string]{Value: "free", Options: plans()},
		}},
	}}}
}

// formPage is a form whose fields are wrong, for a submit to show.
func formPage(form *field.FormController) core.Widget {
	email := regexp.MustCompile(`[^@\s]+@[^@\s]+\.[^@\s]+`)
	return basic.Padding{Insets: geom.All(16), Child: basic.SizedBox{Width: 280, Child: field.Form{
		Controller: form,
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 12, Children: []core.Widget{
			field.FormItem[string]{Required: true, Child: field.Input{Label: "Name", OnChange: func(string) {}}},
			field.FormItem[string]{
				Value:      "taro@",
				Validators: []func(string) string{field.Pattern(email, "Enter an address like name@example.com")},
				Child:      field.Input{Label: "Email", Value: "taro@", OnChange: func(string) {}},
			},
			field.FormItem[string]{Required: true, Child: field.DropdownMenu[string]{
				Label: "Plan", Placeholder: "Choose a plan", Options: plans(), OnChange: func(string) {},
			}},
			basic.Row{Main: layout.MainEnd, Children: []core.Widget{
				control.Button{Label: "Create account", Style: control.ButtonPrimary, OnPress: func() { form.Submit() }},
			}},
		}},
	}}}
}

// nodeOf returns the node of role labeled label in the accessibility tree.
func nodeOf(tree *core.Tree, role semantics.Role, label string) semantics.ID {
	root, _ := tree.SemanticsTree()
	var id semantics.ID
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == role && n.Label == label {
			id = n.ID
		}
		return id == 0
	})
	return id
}

func openDropdown(tree *core.Tree, _ core.BuildContext) {
	tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleComboBox, "Plan"), semantics.ActionExpand, core.SemanticsArg{})
}

// typeCity types into the autocomplete.
func typeCity(tree *core.Tree, _ core.BuildContext) {
	tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleTextField, "City"), semantics.ActionFocus, core.SemanticsArg{})
	tree.DispatchText("a")
}
