package catalog

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// The scenes of InfoBar, StatusBar and CommandPalette (ADR 0096, 0097).

func sceneControlInfoBar() core.Widget {
	closed := func() {}
	return basic.SizedBox{Width: 480, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: []core.Widget{
			// catalog:begin
			control.InfoBar{
				Open: true, Severity: control.SeverityWarning,
				Title: "You are offline", Message: "Changes are saved on this device and sync when you reconnect.",
				OnClose: closed,
			},
			control.InfoBar{
				Open: true, Severity: control.SeverityError,
				Title: "Could not save", Message: "The disk is full.",
				Action: "Retry", OnAction: func() {}, OnClose: closed,
			},
			// catalog:end
		},
	}}
}

func sceneNavStatusBar() core.Widget {
	return basic.SizedBox{Width: 520, Child: nav.StatusBar{
		Leading: []nav.StatusItem{
			{Text: "Ln 12, Col 4", Priority: 2},
			{Text: "No selection"},
		},
		Trailing: []nav.StatusItem{
			{Text: "UTF-8", Tooltip: "Encoding", OnPress: func() {}, Priority: 1},
			{Text: "LF"},
			{Text: "Go", Priority: 1},
		},
	}}
}

func sceneFieldCommandPalette() core.Widget {
	run := func() {}
	actions := []interact.Action{
		{Label: "&New File", OnInvoke: run},
		{Label: "&Open File...", OnInvoke: run},
		{Label: "&Save", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: run},
		{Label: "Save &As...", OnInvoke: run},
		{Label: "&Export..."},
		{Label: "Toggle &Sidebar", Checkable: true, Checked: true, OnInvoke: run},
	}
	return basic.SizedBox{Width: 600, Height: 440, Child: field.CommandPalette{
		Actions: actions,
		Child: interact.Actions{Actions: actions, Child: basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
			return basic.Align{Alignment: layout.AlignTopLeft, Child: control.Button{
				Label: "Open palette", OnPress: func() { field.OpenCommandPalette(ctx) },
			}}
		}}},
	}}
}
