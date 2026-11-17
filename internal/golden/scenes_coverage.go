package golden

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// coverageScenes returns the scenes of the widgets that no other scene covers:
// the settings page (Group, GroupRow, Sidebar, SegmentedControl,
// NumberStepper, ProgressBar, ProgressRing), the menus (MenuBar, ContextMenu)
// and the rest that draws something of its own (RichText, DecoratedBox, Grid,
// FocusRing, AnimatedContainer, AnimatedOpacity).
func coverageScenes() []Scene {
	return []Scene{
		{Name: "coverage-settings", Width: 760, Height: 470, Scale: 1, Root: coverageSettings},
		{Name: "coverage-settings-dark", Width: 1140, Height: 705, Scale: 1.5, Dark: true, Root: coverageSettings},
		{Name: "coverage-menubar", Width: 480, Height: 300, Scale: 1, Root: coverageMenuBar, Act: coveragePress("Edit", 0, input.ButtonPrimary)},
		{Name: "coverage-menubar-dark", Width: 720, Height: 450, Scale: 1.5, Dark: true, Root: coverageMenuBar, Act: coveragePress("Edit", 0, input.ButtonPrimary)},
		{Name: "coverage-context-menu", Width: 480, Height: 300, Scale: 1, Root: coverageContextMenu, Act: coveragePress("Drop files here", 36, input.ButtonSecondary)},
		{Name: "coverage-context-menu-dark", Width: 720, Height: 450, Scale: 1.5, Dark: true, Root: coverageContextMenu, Act: coveragePress("Drop files here", 36, input.ButtonSecondary)},
		{Name: "coverage-misc", Width: 720, Height: 400, Scale: 1, Root: coverageMisc},
		{Name: "coverage-misc-dark", Width: 1080, Height: 600, Scale: 1.5, Dark: true, Root: coverageMisc},
	}
}

// coveragePress clicks below pixels under the middle of the node that says
// label, found in the semantics tree so that the scene need not know where the
// layout put it.
func coveragePress(label string, below float32, button input.Button) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		var at *semantics.Node
		tree.Semantics().Walk(func(n *semantics.Node) bool {
			if n.Label == label {
				at = n
				return false
			}
			return true
		})
		if at == nil {
			panic("golden: no node " + label)
		}
		pos := geom.Offset{X: at.Rect.X + at.Rect.W/2, Y: at.Rect.Y + at.Rect.H/2 + below}
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: button})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: button})
	}
}

// coverageThemed builds content with the theme above it.
func coverageThemed(content func(th *theme.Theme) core.Widget) core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget { return content(basic.ThemeOf(ctx)) }}
}

func coverageSettings() core.Widget {
	nothing := func() {}
	square, circle, triangle := displayIcon(0), displayIcon(1), displayIcon(2)
	sidebar := collection.Sidebar{Title: "Library", Children: []core.Widget{
		collection.SidebarItem{Label: "Songs", Icon: square, Selected: true, OnPress: nothing},
		collection.SidebarItem{Label: "Albums", Icon: circle, OnPress: nothing},
		collection.SidebarItem{Label: "Artists", Icon: triangle, OnPress: nothing},
		collection.SidebarItem{Label: "Playlists", OnPress: nothing},
	}}
	seconds := func(v int) string { return fmt.Sprintf("%d s", v) }
	playback := collection.Group{Title: "Playback", Children: []core.Widget{
		collection.GroupRow{Label: "Shuffle", Child: control.Switch{Value: true, OnChange: func(bool) {}}},
		collection.GroupRow{Label: "Repeat", Child: control.SegmentedControl{
			Options: []string{"Off", "All", "One"}, Selected: 1, OnChange: func(int) {},
		}},
		collection.GroupRow{Label: "Crossfade", Detail: "Seconds between songs", Child: control.NumberStepper{
			Value: 4, Max: 12, Format: seconds, OnChange: func(int) {},
		}},
		collection.GroupRow{Label: "Gap", Child: control.NumberStepper{Value: 0, Max: 5, Format: seconds}},
	}}
	downloads := collection.Group{Title: "Downloads", Children: []core.Widget{
		collection.GroupRow{Label: "Library sync", Child: basic.SizedBox{Width: 180, Child: control.ProgressBar{Value: 0.62}}},
		collection.GroupRow{Label: "Album art", Detail: "128 of 400", Child: control.ProgressRing{Value: 0.32}},
		collection.GroupRow{Label: "Lyrics", Child: coverageThemed(func(th *theme.Theme) core.Widget {
			return control.ProgressRing{Value: 0.75, Size: 40, Color: th.Color.Success, Child: basic.Text{
				Content: "75", Style: coverageColored(th.Text.Caption, th.Color.Ink2),
			}}
		})},
	}}
	return basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		sidebar,
		basic.Expanded{Child: basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space20,
			Children: []core.Widget{playback, downloads},
		}}},
	}}
}

// coverageColored returns s in color c.
func coverageColored(s text.Style, c geom.Color) text.Style {
	s.Color = c
	return s
}

// coverageItems are the items of the menus of the scenes: a shortcut, a check
// mark, a disabled item, a separator and a submenu.
func coverageItems() []overlay.MenuItem {
	nothing := func() {}
	return []overlay.MenuItem{
		{Action: interact.Action{Label: "Undo", Shortcut: interact.Shortcut{Key: input.KeyZ, Modifiers: input.ModControl}, OnInvoke: nothing}},
		{Action: interact.Action{Label: "Redo"}},
		{Separator: true},
		{Action: interact.Action{Label: "Copy", Shortcut: interact.Shortcut{Key: input.KeyC, Modifiers: input.ModControl}, OnInvoke: nothing}},
		{Action: interact.Action{Label: "Word wrap", Checkable: true, Checked: true, OnInvoke: nothing}},
		{Action: interact.Action{Label: "Transform"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "Upper case", OnInvoke: nothing}},
			{Action: interact.Action{Label: "Lower case", OnInvoke: nothing}},
		}},
	}
}

func coverageMenuBar() core.Widget {
	nothing := func() {}
	return basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		overlay.MenuBar{Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "File"}, Items: []overlay.MenuItem{
				{Action: interact.Action{Label: "Save", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: nothing}},
			}},
			{Action: interact.Action{Label: "Edit"}, Items: coverageItems()},
			{Action: interact.Action{Label: "View"}, Items: []overlay.MenuItem{
				{Action: interact.Action{Label: "Zoom in", OnInvoke: nothing}},
			}},
		}},
		coverageThemed(func(th *theme.Theme) core.Widget { return basic.Container{Height: 1, Color: th.Color.Hair} }),
	}}
}

func coverageContextMenu() core.Widget {
	return basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Align{Alignment: layout.AlignTopLeft, Child: overlay.ContextMenu{
		Items: coverageItems(),
		Child: coverageThemed(func(th *theme.Theme) core.Widget {
			return basic.Container{
				Width: 260, Height: 220, Radius: th.Radius.Group, Color: th.Color.Card,
				Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
				Padding: geom.All(theme.Space12),
				Child: basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Text{
					Content: "Drop files here", Style: coverageColored(th.Text.Body, th.Color.Ink2),
				}},
			}
		}),
	}}}
}

func coverageMisc() core.Widget {
	return coverageThemed(func(th *theme.Theme) core.Widget {
		c := &th.Color
		rich := basic.SizedBox{Width: 300, Child: basic.RichText{Style: coverageColored(th.Text.Body, c.Ink), Spans: []text.Span{
			{Text: "Rich text mixes "},
			{Text: "accented", Style: coverageColored(th.Text.Body, c.AccentInk)},
			{Text: ", "},
			{Text: "large", Style: text.Style{Size: 20, Color: c.Ink}},
			{Text: " and "},
			{Text: "quiet", Style: coverageColored(th.Text.Caption, c.Ink3)},
			{Text: " runs in one paragraph that wraps onto a second line."},
		}}}
		tile := func(i int) core.Widget {
			return basic.Container{Color: c.Fill, Radius: th.Radius.Control, Child: basic.Center{Child: basic.Text{
				Content: fmt.Sprintf("Tile %d", i+1), Style: coverageColored(th.Text.Caption, c.Ink2),
			}}}
		}
		tiles := make([]core.Widget, 6)
		for i := range tiles {
			tiles[i] = tile(i)
		}
		grid := basic.SizedBox{Width: 300, Child: basic.Grid{Columns: 3, Spacing: theme.Space8, RunSpacing: theme.Space8, TileHeight: 44, Children: tiles}}
		box := basic.DecoratedBox{
			Decoration: layout.Decoration{Color: c.Card, Radius: th.Radius.Group, Border: &paint.Stroke{Width: 1, Color: c.Hair}},
			Child: basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Text{
				Content: "DecoratedBox", Style: coverageColored(th.Text.Body, c.Ink),
			}},
		}
		ring := interact.FocusRing{Visible: true, Radius: th.Radius.Control, Child: basic.Container{
			Width: 120, Height: 36, Radius: th.Radius.Control, Color: c.AccentTint,
			Child: basic.Center{Child: basic.Text{Content: "Focus ring", Style: coverageColored(th.Text.Control, c.AccentInk)}},
		}}
		animated := basic.AnimatedContainer{
			Width: 120, Height: 36, Radius: 18, Color: c.Accent,
			Child: basic.Center{Child: basic.Text{Content: "Animated", Style: coverageColored(th.Text.Control, c.OnAccent)}},
		}
		faded := basic.AnimatedOpacity{Opacity: 0.4, Child: basic.Container{
			Width: 120, Height: 36, Radius: th.Radius.Control, Color: c.Ink,
			Child: basic.Center{Child: basic.Text{Content: "Opacity 0.4", Style: coverageColored(th.Text.Control, c.Canvas)}},
		}}
		return basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Row{
			Cross: layout.CrossStart, Spacing: theme.Space28, Children: []core.Widget{
				basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space20, Children: []core.Widget{rich, grid}},
				basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space20, Children: []core.Widget{box, ring, animated, faded}},
			},
		}}
	})
}
