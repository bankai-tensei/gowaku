package catalog

import (
	"regexp"
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// Buttons and actions.

func sceneControlButton() core.Widget {
	save := func() {}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
			control.Button{Label: "Save", Style: control.ButtonPrimary, OnPress: save},
			control.Button{Label: "Duplicate", OnPress: save},
			control.Button{Label: "Learn more", Style: control.ButtonQuiet, OnPress: save},
			control.Button{Label: "Delete", Style: control.ButtonDanger, OnPress: save},
		}},
		basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
			control.Button{Label: "Copy", Icon: icons.Copy(), OnPress: save},
			control.Button{Label: "Small", Size: control.ButtonSmall, OnPress: save},
			control.Button{Label: "Disabled"},
			control.Button{Label: "Settings", Icon: icons.Settings(), HideLabel: true, OnPress: save},
			control.Button{Label: "More", Icon: icons.More(), HideLabel: true, Style: control.ButtonQuiet, OnPress: save},
		}},
	}}
}

func sceneInteractPressable() core.Widget {
	// catalog:begin
	return interact.Pressable{Label: "Open project", OnPress: func() {}, Build: func(s interact.PressState) core.Widget {
		return themed(func(th *theme.Theme) core.Widget {
			ground := th.Color.Card
			if s.Hovered {
				ground = th.Color.AccentTint
			}
			return interact.FocusRing{Visible: s.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
				Width: 220, Padding: geom.All(theme.Space12), Radius: th.Radius.Control, Color: ground,
				Child: basic.Row{Spacing: theme.Space8, Children: []core.Widget{
					basic.Icon{Path: icons.Folder(), Size: 20, Color: th.Color.AccentInk},
					basic.Text{Content: "Open project", Style: colored(th.Text.Control, th.Color.Ink)},
				}},
			}}
		})
	}}
	// catalog:end
}

// Input.

func sceneFieldInput() core.Widget {
	set := func(string) {}
	return basic.SizedBox{Width: 300, Height: 260, Child: basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		field.Input{Label: "Name", Value: "Hanako Yamada", OnChange: set},
		field.Input{Label: "Email", Placeholder: "name@example.com", OnChange: set},
		basic.Expanded{Child: field.Input{
			Label: "Notes", Multiline: true, Value: "A multiline field fills the height\nthat an Expanded gives it.", OnChange: set,
		}},
	}}}
}

func sceneFieldSearch() core.Widget {
	return basic.SizedBox{Width: 260, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
		field.Search{OnChange: func(string) {}},
		field.Search{Value: "quarterly report", OnChange: func(string) {}},
	}}}
}

func sceneFieldAutocomplete() core.Widget {
	cities := []string{"Amsterdam", "Athens", "Berlin", "Lisbon", "Madrid", "Paris", "Prague", "Rome"}
	return basic.SizedBox{Width: 260, Child: field.Autocomplete[string]{
		Label: "City", Placeholder: "Type a city", Options: cities, OnSelect: func(string) {},
	}}
}

func sceneEditorEditor() core.Widget {
	var doc editor.Controller
	doc.SetText("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, gowaku\")\n}")
	return basic.SizedBox{Width: 420, Height: 180, Child: editor.Editor{
		Controller: &doc, Label: "Source", LineNumbers: true, Border: true, HighlightCurrentLine: true,
	}}
}

func sceneControlNumberStepper() core.Widget {
	seconds := func(v int) string { return strconv.Itoa(v) + " s" }
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		control.NumberStepper{Value: 4, Max: 12, Label: "Copies", OnChange: func(int) {}},
		control.NumberStepper{Value: 30, Max: 60, Step: 5, Format: seconds, Label: "Delay", OnChange: func(int) {}},
	}}
}

func sceneControlSlider() core.Widget {
	return basic.SizedBox{Width: 260, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		control.Slider{Value: 0.35, Label: "Volume", OnChange: func(float32) {}},
		control.Slider{Value: 40, Min: 0, Max: 100, Step: 10, Snap: true, Label: "Zoom", OnChange: func(float32) {}},
		control.Slider{Value: 0.7, Label: "Disabled"},
	}}}
}

func sceneFieldForm() core.Widget {
	var form field.FormController
	email := regexp.MustCompile(`[^@\s]+@[^@\s]+\.[^@\s]+`)
	return basic.SizedBox{Width: 280, Child: field.Form{Controller: &form, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
			field.FormItem[string]{Required: true, Child: field.Input{Label: "Name", OnChange: func(string) {}}},
			field.FormItem[string]{
				Value:      "taro@",
				Validators: []func(string) string{field.Pattern(email, "Enter an address like name@example.com")},
				Child:      field.Input{Label: "Email", Value: "taro@", OnChange: func(string) {}},
			},
			field.FormItem[time.Time]{Required: true, Child: field.DatePicker{
				Label: "Birth date", Placeholder: "Choose a date", OnChange: func(time.Time) {},
				InitialMonth: time.Date(1995, time.January, 1, 0, 0, 0, 0, time.UTC),
			}},
			field.FormItem[bool]{Required: true, Child: control.Checkbox{Label: "I agree to the terms", OnChange: func(bool) {}}},
			basic.Row{Main: layout.MainEnd, Children: []core.Widget{
				control.Button{Label: "Create account", Style: control.ButtonPrimary, OnPress: func() { form.Submit() }},
			}},
		},
	}}}
}

func sceneFieldTimePicker() core.Widget {
	at := func(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		field.TimePicker{Label: "Start", Value: at(9, 30), MinuteStep: 15, OnChange: func(time.Duration) {}},
		field.TimePicker{Value: at(21, 5) + 42*time.Second, Seconds: true, HourCycle: field.HourCycle24, OnChange: func(time.Duration) {}},
	}}
}

// Selection.

func sceneControlCheckbox() core.Widget {
	set := func(bool) {}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		control.Checkbox{Value: true, Label: "Open at login", OnChange: set},
		control.Checkbox{Label: "Show in menu bar", OnChange: set},
		control.Checkbox{Value: true, Label: "Disabled"},
	}}
}

func sceneControlSwitch() core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		basic.Row{Size: layout.MainMin, Spacing: theme.Space12, Children: []core.Widget{
			control.Switch{Value: true, Label: "Wi-Fi", OnChange: func(bool) {}},
			basic.Text{Content: "Wi-Fi"},
		}},
		basic.Row{Size: layout.MainMin, Spacing: theme.Space12, Children: []core.Widget{
			control.Switch{Label: "Bluetooth", OnChange: func(bool) {}},
			basic.Text{Content: "Bluetooth"},
		}},
	}}
}

func sceneControlRadioGroup() core.Widget {
	return control.RadioGroup[string]{
		Label: "Appearance", Value: "auto", OnChange: func(string) {},
		Options: []control.RadioOption[string]{
			{Value: "light", Label: "Light"},
			{Value: "dark", Label: "Dark"},
			{Value: "auto", Label: "Automatic"},
			{Value: "contrast", Label: "High contrast", Disabled: true},
		},
	}
}

func sceneControlRadio() core.Widget {
	// catalog:begin
	return interact.Semantics{Role: semantics.RoleRadioGroup, Label: "Size", Child: basic.Row{
		Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
			control.Radio{Label: "Small", OnSelect: func() {}},
			control.Radio{Label: "Medium", Selected: true, OnSelect: func() {}},
			control.Radio{Label: "Large", OnSelect: func() {}},
		},
	}}
	// catalog:end
}

func sceneControlSegmentedControl() core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
		control.SegmentedControl{Label: "Repeat", Options: []string{"Off", "All", "One"}, Selected: 1, OnChange: func(int) {}},
		control.SegmentedControl{Label: "View", Options: []string{"Day", "Week", "Month", "Year"}, Selected: 2, OnChange: func(int) {}},
	}}
}

func sceneFieldDropdownMenu() core.Widget {
	return basic.SizedBox{Width: 200, Child: field.DropdownMenu[string]{
		Label: "Plan", Value: "team", OnChange: func(string) {},
		Options: []field.DropdownOption[string]{
			{Value: "free", Label: "Free"},
			{Value: "team", Label: "Team"},
			{Value: "business", Label: "Business"},
			{Value: "school", Label: "School", Disabled: true},
		},
	}}
}

func sceneControlChip() core.Widget {
	return basic.SizedBox{Width: 360, Child: basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: []core.Widget{
		control.Chip{Label: "All", Selected: true, OnChange: func(bool) {}},
		control.Chip{Label: "Unread", OnChange: func(bool) {}},
		control.Chip{Label: "Starred", Icon: icons.Star(), OnChange: func(bool) {}},
		control.Chip{Label: "Suggested reply", OnPress: func() {}},
		control.Chip{Label: "design", OnRemove: func() {}},
		control.Chip{Label: "Plain tag"},
	}}}
}

func sceneFieldCalendar() core.Widget {
	day := func(d int) time.Time { return time.Date(2026, time.November, d, 0, 0, 0, 0, time.UTC) }
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{Color: th.Color.Card, Radius: th.Radius.Group, Padding: geom.All(theme.Space12), Child: field.Calendar{
			Value: day(18), Today: day(9), OnChange: func(time.Time) {},
			Marks: func(d time.Time) []geom.Color {
				if d.Day() == 12 || d.Day() == 24 {
					return []geom.Color{th.Color.AccentInk}
				}
				return nil
			},
		}}
	})
	// catalog:end
}

func sceneFieldDatePicker() core.Widget {
	day := func(d int) time.Time { return time.Date(2026, time.November, d, 0, 0, 0, 0, time.UTC) }
	return field.DatePicker{
		Label: "Due date", Value: day(18), Today: day(9),
		ShowToday: true, ShowClear: true, OnChange: func(time.Time) {},
	}
}
