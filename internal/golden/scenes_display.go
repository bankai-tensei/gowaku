package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// displayScenes returns the scenes of the display widgets.
func displayScenes() []Scene {
	return []Scene{
		{Name: "display", Width: 720, Height: 520, Scale: 1, Root: display, Act: focusDisplay},
		{Name: "display-dark", Width: 1080, Height: 780, Scale: 1.5, Dark: true, Root: display, Act: focusDisplay},
	}
}

// focusDisplay moves the keyboard into the toolbar, so that the focus ring
// shows on its second button.
func focusDisplay(tree *core.Tree, _ core.BuildContext) {
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyRight})
	// The pointer rests on the chip Unread, which shows its hover.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 98, Y: 201}})
}

// displayIcon returns a 24 unit icon: a square, a circle or a triangle.
func displayIcon(kind int) *paint.Path {
	var p paint.Path
	switch kind {
	case 0:
		p.AddRRect(geom.RRectAll(geom.Rect{X: 5, Y: 5, W: 14, H: 14}, 3))
	case 1:
		p.AddRRect(geom.RRectAll(geom.Rect{X: 4, Y: 4, W: 16, H: 16}, 8))
	default:
		p.MoveTo(12, 4)
		p.LineTo(20, 19)
		p.LineTo(4, 19)
		p.Close()
	}
	return &p
}

func display() core.Widget {
	nothing := func() {}
	square, circle, triangle := displayIcon(0), displayIcon(1), displayIcon(2)
	label := func(s string) core.Widget { return basic.Text{Content: s} }
	return basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
		Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
			nav.Toolbar{Label: "Format", Items: []nav.ToolbarItem{
				{Action: interact.Action{Label: "Bold", OnInvoke: nothing, Checkable: true, Checked: true}, Icon: square},
				{Action: interact.Action{Label: "Italic", OnInvoke: nothing, Checkable: true}, Icon: circle},
				{Action: interact.Action{Label: "Shape"}, Icon: triangle},
				{Separator: true},
				{Action: interact.Action{Label: "Share", OnInvoke: nothing}, Icon: circle, ShowLabel: true},
				{Spacer: true},
				{Action: interact.Action{Label: "Export", OnInvoke: nothing}},
			}},
			basic.Align{Child: basic.SizedBox{Width: 236, Child: nav.Toolbar{Label: "Narrow", Items: []nav.ToolbarItem{
				{Action: interact.Action{Label: "Cut", OnInvoke: nothing}, Icon: square},
				{Action: interact.Action{Label: "Copy", OnInvoke: nothing}, Icon: circle},
				{Separator: true},
				{Action: interact.Action{Label: "Paste", OnInvoke: nothing}},
				{Action: interact.Action{Label: "Paste special", OnInvoke: nothing}},
				{Action: interact.Action{Label: "Find", OnInvoke: nothing}},
			}}}},
			basic.Divider{},
			basic.Row{Spacing: theme.Space28, Cross: layout.CrossCenter, Children: []core.Widget{
				control.Badge{Count: 3, Child: basic.Icon{Path: square, Color: geom.Hex(0x8a8a8e)}},
				control.Badge{Count: 120, Child: basic.Icon{Path: circle, Color: geom.Hex(0x8a8a8e)}},
				control.Badge{Dot: true, Child: basic.Icon{Path: triangle, Color: geom.Hex(0x8a8a8e)}},
				control.Badge{Label: "New"},
				control.Badge{Count: 12, Quiet: true},
				basic.SizedBox{Height: 24, Child: basic.Divider{Axis: layout.Vertical}},
				label("Inbox"),
				control.Badge{Count: 7},
			}},
			basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: []core.Widget{
				control.Chip{Label: "All", Selected: true, OnChange: func(bool) {}},
				control.Chip{Label: "Unread", OnChange: func(bool) {}},
				control.Chip{Label: "Starred", Icon: circle, OnChange: func(bool) {}},
				control.Chip{Label: "Suggested reply", OnPress: nothing},
				control.Chip{Label: "design", OnRemove: nothing},
				control.Chip{Label: "Selected", Icon: square, Selected: true, OnChange: func(bool) {}, OnRemove: nothing},
				control.Chip{Label: "Plain tag"},
			}},
			basic.Row{Spacing: theme.Space16, Cross: layout.CrossStart, Children: []core.Widget{
				basic.Expanded{Child: control.Card{Label: "Summary", Child: basic.Column{
					Size: layout.MainMin, Spacing: theme.Space6, Cross: layout.CrossStart, Children: []core.Widget{
						displayHeading("Summary"),
						label("A plain card holds content."),
						basic.Divider{},
						label("Lines inside stay hairlines."),
					},
				}}},
				basic.Expanded{Child: control.Card{Label: "Open note", OnPress: nothing, Child: basic.Column{
					Size: layout.MainMin, Spacing: theme.Space6, Cross: layout.CrossStart, Children: []core.Widget{
						displayHeading("Pressable"),
						label("The whole card is a button."),
					},
				}}},
			}},
		},
	}}
}

// displayHeading sets s in the Headline of the theme.
func displayHeading(s string) core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		style := th.Text.Headline
		style.Color = th.Color.Ink
		return basic.Text{Content: s, Style: style}
	}}
}
