package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// choiceScenes returns the scenes of the choice widgets: radio buttons
// and tabs. The light one has the focus on the tabs, the dark one on the radio
// group inside the page.
func choiceScenes() []Scene {
	return []Scene{
		{Name: "choice", Width: 640, Height: 420, Scale: 1, Root: choices, Act: pressTab(1)},
		{Name: "choice-dark", Width: 960, Height: 630, Scale: 1.5, Dark: true, Root: choices, Act: pressTab(2)},
	}
}

// pressTab moves the keyboard n stops into the scene.
func pressTab(n int) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		for range n {
			tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		}
	}
}

func choices() core.Widget {
	sizes := []control.RadioOption[string]{{Value: "s", Label: "Small"}, {Value: "m", Label: "Medium"}, {Value: "l", Label: "Large"}}
	appearance := basic.Padding{Insets: geom.Symmetric(0, theme.Space16), Child: control.RadioGroup[string]{
		Value: "auto", Label: "Appearance", OnChange: func(string) {},
		Options: []control.RadioOption[string]{
			{Value: "light", Label: "Light"},
			{Value: "dark", Label: "Dark"},
			{Value: "auto", Label: "Automatic"},
			{Value: "contrast", Label: "High contrast", Disabled: true},
		},
	}}
	tabs := basic.SizedBox{Width: 300, Height: 220, Child: control.Tabs{
		Label: "Settings", Selected: 1, OnChange: func(int) {},
		Pages: []control.TabPage{
			{Label: "General"},
			{Label: "Appearance", Child: appearance},
			{Label: "Advanced"},
		},
	}}
	caption := func(s string) core.Widget {
		return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
			th := basic.ThemeOf(ctx)
			style := th.Text.Section
			style.Color = th.Color.Ink2
			return basic.Text{Content: s, Style: style}
		}}
	}
	right := basic.Column{Size: layout.MainMin, Spacing: theme.Space12, Children: []core.Widget{
		caption("Side by side"),
		control.RadioGroup[string]{Value: "m", Options: sizes, Horizontal: true, OnChange: func(string) {}},
		caption("Disabled"),
		control.RadioGroup[string]{Value: "l", Options: sizes, Horizontal: true},
		basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
			control.Radio{Label: "On its own", OnSelect: func() {}},
			control.Radio{Selected: true, Label: "Chosen"},
		}},
		basic.SizedBox{Width: 260, Height: 60, Child: control.Tabs{
			Pages: []control.TabPage{{Label: "Day"}, {Label: "Week"}, {Label: "Month"}},
		}},
	}}
	return basic.Padding{Insets: geom.All(theme.Space28), Child: basic.Row{
		Spacing: theme.Space28, Cross: layout.CrossStart, Children: []core.Widget{tabs, right},
	}}
}
