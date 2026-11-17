package catalog

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// Data.

func sceneCollectionList() core.Widget {
	// catalog:begin
	return basic.SizedBox{Width: 320, Height: 180, Child: collection.List{
		ItemCount: 10000, ItemExtent: 36,
		ItemBuilder: func(i int) core.Widget {
			return basic.Padding{Insets: geom.Symmetric(theme.Space12, theme.Space8), Child: basic.Text{
				Content: "Row " + strconv.Itoa(i+1) + " of 10,000",
			}}
		},
	}}
	// catalog:end
}

// task is a row of the Table scene.
type task struct {
	name, owner, status string
	done                float32
}

func sceneCollectionTable() core.Widget {
	rows := []task{
		{"Design review", "Aiko", "Done", 1},
		{"Release notes", "Ben", "In progress", 0.6},
		{"Fix the build", "Chiara", "Blocked", 0.2},
		{"Translate docs", "Daichi", "In progress", 0.4},
		{"Plan the sprint", "Aiko", "Done", 1},
	}
	tone := func(t task) collection.CellTone {
		switch t.status {
		case "Done":
			return collection.CellToneSuccess
		case "Blocked":
			return collection.CellToneDanger
		}
		return collection.CellToneAccent
	}
	return basic.SizedBox{Width: 520, Height: 220, Child: collection.Table[task]{
		Label: "Tasks", Rows: rows,
		Sort: collection.TableSort{Column: 0, Order: collection.SortAscending}, OnSort: func(collection.TableSort) {},
		Columns: []collection.TableColumn[task]{
			{Title: "Task", Width: 160, Text: func(t task) string { return t.name }, Sortable: true},
			{Title: "Owner", Width: 100, Text: func(t task) string { return t.owner }, Sortable: true},
			{Title: "Status", Width: 120, Text: func(t task) string { return t.status }, Kind: collection.CellBadge, Tone: tone},
			{Title: "Done", Width: 120, Kind: collection.CellProgress, Value: func(t task) float32 { return t.done }},
		},
	}}
}

func sceneCollectionTree() core.Widget {
	folders := map[string][]string{
		"":         {"Projects", "Documents", "notes.txt"},
		"Projects": {"gowaku", "website"},
		"gowaku":   {"widgets", "go.mod", "README.md"},
		"widgets":  {"tree.go", "table.go"},
		"website":  {"index.html"},
	}
	var ctrl collection.TreeController[string]
	ctrl.Expand("Projects")
	ctrl.Expand("gowaku")
	ctrl.Select("README.md")
	// catalog:begin
	isFolder := func(k string) bool { return len(folders[k]) > 0 || k == "Documents" }
	return basic.SizedBox{Width: 280, Height: 260, Child: collection.Tree[string]{
		Label: "Files", Roots: folders[""], Controller: &ctrl,
		ChildrenOf:  func(k string) []string { return folders[k] },
		HasChildren: isFolder,
		IconOf: func(k string) core.Widget {
			if isFolder(k) {
				return basic.Icon{Path: icons.Folder(), Size: 16, Color: geom.Hex(0x2f5fd0)}
			}
			return basic.Icon{Path: icons.Document(), Size: 16, Color: geom.Hex(0x8a8a8e)}
		},
	}}
	// catalog:end
}

func sceneCollectionPropertyGrid() core.Widget {
	type fit int
	return basic.SizedBox{Width: 340, Child: collection.PropertyGrid{Label: "Inspector", Categories: []collection.PropertyCategory{
		{Name: "General", Properties: []collection.Property{
			{Name: "Name", Description: "The name of the layer.", Editor: collection.PropertyText{Value: "Header", OnChange: func(string) {}}},
			{Name: "Visible", Editor: collection.PropertyBool{Value: true, OnChange: func(bool) {}}},
			{Name: "Fit", Editor: collection.PropertyChoice[fit]{Value: 1, OnChange: func(fit) {}, Options: []field.DropdownOption[fit]{
				{Value: 0, Label: "Fill"}, {Value: 1, Label: "Contain"},
			}}},
		}},
		{Name: "Layout", Properties: []collection.Property{
			{Name: "Width", Editor: collection.PropertyInt{Value: 320, Max: 4096, OnChange: func(int) {}}},
			{Name: "Opacity", Editor: collection.PropertyNumber{Value: 0.85, Max: 1, OnChange: func(float64) {}}},
			{Name: "Tint", Editor: collection.PropertyColor{Value: geom.Hex(0x2f5fd0), OnPress: func() {}}},
		}},
		{Name: "Advanced", Collapsed: true, Properties: []collection.Property{
			{Name: "Cache", Editor: collection.PropertyBool{OnChange: func(bool) {}}},
		}},
	}}}
}

func sceneCollectionGroup() core.Widget {
	return basic.SizedBox{Width: 420, Child: collection.Group{Title: "Playback", Children: []core.Widget{
		collection.GroupRow{Label: "Shuffle", Child: control.Switch{Value: true, OnChange: func(bool) {}}},
		collection.GroupRow{Label: "Repeat", Child: control.SegmentedControl{
			Options: []string{"Off", "All", "One"}, Selected: 1, OnChange: func(int) {},
		}},
		collection.GroupRow{Label: "Crossfade", Detail: "Seconds between songs", Child: control.NumberStepper{
			Value: 4, Max: 12, OnChange: func(int) {},
		}},
	}}}
}

// Desktop.

// editItems are the items of the menus of the desktop scenes: shortcuts, a
// check mark, a disabled item, a separator and a submenu.
func editItems() []overlay.MenuItem {
	run := func() {}
	return []overlay.MenuItem{
		{Action: interact.Action{Label: "&Undo", Shortcut: interact.Shortcut{Key: input.KeyZ, Modifiers: input.ModControl}, OnInvoke: run}},
		{Action: interact.Action{Label: "&Redo"}},
		{Separator: true},
		{Action: interact.Action{Label: "&Copy", Shortcut: interact.Shortcut{Key: input.KeyC, Modifiers: input.ModControl}, OnInvoke: run}},
		{Action: interact.Action{Label: "Word &wrap", Checkable: true, Checked: true, OnInvoke: run}},
		{Action: interact.Action{Label: "&Transform"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "Upper case", OnInvoke: run}},
			{Action: interact.Action{Label: "Lower case", OnInvoke: run}},
		}},
	}
}

func sceneOverlayMenuBar() core.Widget {
	run := func() {}
	return basic.SizedBox{Width: 420, Child: overlay.MenuBar{Items: []overlay.MenuItem{
		{Action: interact.Action{Label: "&File"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "&Save", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: run}},
		}},
		{Action: interact.Action{Label: "&Edit"}, Items: editItems()},
		{Action: interact.Action{Label: "&View"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "Zoom &in", OnInvoke: run}},
		}},
	}}}
}

func sceneOverlayContextMenu() core.Widget {
	// catalog:begin
	return overlay.ContextMenu{Items: editItems(), Child: themed(func(th *theme.Theme) core.Widget {
		return basic.Container{
			Width: 240, Height: 160, Radius: th.Radius.Group, Color: th.Color.Card,
			Border: paint.Stroke{Width: 1, Color: th.Color.Hair}, Padding: geom.All(theme.Space12),
			Child: basic.Text{Content: "Right-click here", Style: colored(th.Text.Body, th.Color.Ink2)},
		}
	})}
	// catalog:end
}

func sceneNavToolbar() core.Widget {
	run := func() {}
	return basic.SizedBox{Width: 460, Child: nav.Toolbar{Label: "Format", Items: []nav.ToolbarItem{
		{Action: interact.Action{Label: "Bold", OnInvoke: run, Checkable: true, Checked: true}, Icon: squareIcon()},
		{Action: interact.Action{Label: "Italic", OnInvoke: run, Checkable: true}, Icon: circleIcon()},
		{Action: interact.Action{Label: "Shape"}, Icon: triangleIcon()},
		{Separator: true},
		{Action: interact.Action{Label: "Share", OnInvoke: run}, Icon: icons.Star(), ShowLabel: true},
		{Spacer: true},
		{Action: interact.Action{Label: "Export", OnInvoke: run}},
	}}}
}

func sceneNavTabBar() core.Widget {
	icon := icons.Document()
	return basic.SizedBox{Width: 520, Child: nav.TabBar{
		Label: "Files", Selected: 1,
		Tabs: []nav.TabItem{
			{Key: "readme", Label: "README.md", Icon: icon, Modified: true},
			{Key: "main", Label: "main.go", Icon: icon},
			{Key: "long", Label: "A long name that does not fit.txt", Icon: icon},
		},
		OnSelect: func(int) {}, OnClose: func(int) {}, OnNew: func() {}, OnMove: func(int, int) {},
	}}
}

func sceneControlSplitPane() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{
			Width: 480, Height: 200, Color: th.Color.Card, Radius: th.Radius.Group,
			Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			Child: control.SplitPane{
				Position: 160, Pixels: true, MinFirst: 120, MinSecond: 200, Collapsible: control.SplitCollapseFirst,
				First: basic.Container{Color: th.Color.Sidebar, Radius: th.Radius.Group, Padding: geom.All(theme.Space12),
					Child: basic.Text{Content: "Sidebar"}},
				Second: basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Text{Content: "Drag the divider, or press Enter on it to fold the sidebar."}},
			},
		}
	})
	// catalog:end
}

// Layout.

func sceneBasicRow() core.Widget {
	return basic.SizedBox{Width: 360, Child: basic.Row{Spacing: theme.Space8, Main: layout.MainSpaceBetween, Children: []core.Widget{
		tile("A", 64, 40), tile("B", 96, 40), tile("C", 64, 40),
	}}}
}

func sceneBasicColumn() core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
		tile("First", 160, 32), tile("Second", 120, 32), tile("Third", 200, 32),
	}}
}

func sceneBasicFlex() core.Widget {
	axis := layout.Vertical // chosen at run time, as by the width of the window
	return basic.Flex{Axis: axis, Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
		tile("Flex", 120, 32), tile("picks its axis", 160, 32), tile("at run time", 140, 32),
	}}
}

func sceneBasicExpanded() core.Widget {
	return basic.SizedBox{Width: 520, Child: basic.Row{Spacing: theme.Space8, Children: []core.Widget{
		tile("Fixed", 80, 40),
		basic.Expanded{Child: tile("Expanded takes the rest", 0, 40)},
		basic.Expanded{Flex: 2, Child: tile("Flex 2", 0, 40)},
	}}}
}

func sceneBasicFlexible() core.Widget {
	return basic.SizedBox{Width: 420, Child: basic.Row{Spacing: theme.Space8, Children: []core.Widget{
		basic.Flexible{Child: tile("Flexible stays small", 140, 40)},
		basic.Flexible{Fit: layout.FitTight, Child: tile("Tight fills its share", 60, 40)},
	}}}
}

func sceneBasicStack() core.Widget {
	return basic.SizedBox{Width: 240, Height: 140, Child: basic.Stack{Children: []core.Widget{
		tile("Below", 200, 120),
		basic.Positioned{Left: 120, Top: 70, Child: tile("On top", 110, 60)},
		basic.Positioned{AnchorRight: true, Right: 0, Top: 0, Child: control.Badge{Count: 2}},
	}}}
}

func sceneBasicPositionedFill() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Stack{Children: []core.Widget{
			basic.Image{Source: photo(), Width: 240},
			basic.PositionedFill{Insets: geom.All(theme.Space8), Child: basic.Container{
				Radius: th.Radius.Control, Border: paint.Stroke{Width: 2, Color: geom.RGBA(1, 1, 1, 0.9)},
			}},
		}}
	})
	// catalog:end
}

func sceneBasicWrap() core.Widget {
	words := []string{"Go", "GPU", "desktop", "cgo-free", "Flutter-style", "widgets", "Japanese text", "accessibility"}
	chips := make([]core.Widget, len(words))
	for i, w := range words {
		chips[i] = control.Chip{Label: w}
	}
	return basic.SizedBox{Width: 300, Child: basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}}
}

func sceneBasicGrid() core.Widget {
	tiles := make([]core.Widget, 6)
	for i := range tiles {
		tiles[i] = tile("Tile "+strconv.Itoa(i+1), 0, 0)
	}
	return basic.SizedBox{Width: 320, Child: basic.Grid{Columns: 3, Spacing: theme.Space8, RunSpacing: theme.Space8, TileHeight: 48, Children: tiles}}
}

func sceneBasicPadding() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{Color: th.Color.Fill, Radius: th.Radius.Control, Child: basic.Padding{
			Insets: geom.Insets{Left: 24, Top: 12, Right: 24, Bottom: 12},
			Child:  tile("Child", 120, 40),
		}}
	})
	// catalog:end
}

func sceneBasicAlign() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		box := func(a layout.Alignment, label string) core.Widget {
			return basic.Container{Width: 120, Height: 80, Radius: th.Radius.Control, Color: th.Color.Fill,
				Child: basic.Align{Alignment: a, Child: tile(label, 48, 24)}}
		}
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
			box(layout.AlignTopLeft, "top left"), box(layout.AlignCenter, "center"), box(layout.AlignBottomRight, "end"),
		}}
	})
	// catalog:end
}

func sceneBasicCenter() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{Width: 240, Height: 100, Radius: th.Radius.Control, Color: th.Color.Fill,
			Child: basic.Center{Child: tile("Centered", 100, 32)}}
	})
	// catalog:end
}

func sceneBasicSizedBox() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Cross: layout.CrossStart, Children: []core.Widget{
		basic.SizedBox{Width: 140, Height: 48, Child: tile("140 by 48", 0, 0)},
		basic.SizedBox{Width: 24}, // an empty gap
		basic.SizedBox{Width: 80, Height: 80, Child: tile("80 by 80", 0, 0)},
	}}
}

func sceneBasicConstrainedBox() core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
		basic.ConstrainedBox{MinWidth: 160, Child: control.Button{Label: "OK", OnPress: func() {}}},
		basic.ConstrainedBox{MaxWidth: 200, Child: basic.Text{Content: "MaxWidth makes a long line of text wrap inside 200 pixels."}},
	}}
}

func sceneBasicContainer() core.Widget {
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
		basic.Container{Width: 120, Height: 72, Radius: 12, Color: geom.Hex(0x2f5fd0),
			Shadows: []paint.BoxShadow{{Color: geom.RGBA(0, 0, 0, 0.25), Blur: 12, Offset: geom.Offset{Y: 4}}}},
		basic.Container{Width: 120, Height: 72, Radius: 12, Border: paint.Stroke{Width: 1.5, Color: geom.Hex(0x2f5fd0)},
			Padding: geom.All(12), Child: basic.Text{Content: "Border"}},
		basic.Container{Width: 120, Height: 72, Radius: 36, Gradient: &paint.Gradient{
			End: geom.Offset{X: 120, Y: 72}, StartColor: geom.Hex(0x7b8cff), EndColor: geom.Hex(0x2f5fd0),
		}},
	}}
}

func sceneBasicDecoratedBox() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.DecoratedBox{
			Decoration: layout.Decoration{Color: th.Color.Card, Radius: th.Radius.Group, Border: &paint.Stroke{Width: 1, Color: th.Color.Hair}},
			Child:      basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Text{Content: "DecoratedBox paints behind its child"}},
		}
	})
	// catalog:end
}

func sceneBasicScroll() core.Widget {
	lines := make([]core.Widget, 12)
	for i := range lines {
		lines[i] = basic.Text{Content: "Line " + strconv.Itoa(i+1) + " of a column taller than its box"}
	}
	return basic.SizedBox{Width: 300, Height: 140, Child: basic.Scroll{Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space4, Children: lines,
	}}}
}

// diagonal is the layout.Delegate of the CustomLayout scene: it puts each
// child below and to the right of the one before.
type diagonal struct{ step float32 }

// Layout implements layout.Delegate.
func (d diagonal) Layout(c layout.Constraints, children []core.RenderObject) geom.Size {
	var size geom.Size
	for i, child := range children {
		sz := core.LayoutChild(child, c.Loosen())
		off := geom.Offset{X: float32(i) * d.step, Y: float32(i) * d.step * 0.6}
		child.SetOffset(off)
		size.W = max(size.W, off.X+sz.W)
		size.H = max(size.H, off.Y+sz.H)
	}
	return c.Constrain(size)
}

func sceneBasicCustomLayout() core.Widget {
	return basic.CustomLayout{Delegate: diagonal{step: 56}, Children: []core.Widget{
		tile("One", 96, 32), tile("Two", 96, 32), tile("Three", 96, 32), tile("Four", 96, 32),
	}}
}

func sceneBasicLayoutBuilder() core.Widget {
	// catalog:begin
	panes := basic.LayoutBuilder{Content: func(_ core.BuildContext, c layout.Constraints) core.Widget {
		if c.MaxW < 300 { // narrow: one above the other
			return basic.Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
				tile("List", c.MaxW, 40), tile("Detail", c.MaxW, 40),
			}}
		}
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Children: []core.Widget{
			tile("List", 120, 88), tile("Detail", c.MaxW-120-theme.Space8, 88),
		}}
	}}
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossStart, Children: []core.Widget{
		basic.SizedBox{Width: 160, Child: panes},
		basic.SizedBox{Width: 320, Child: panes},
	}}
	// catalog:end
}

func sceneBasicAspectRatio() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		// catalog:begin
		// box is a 200 by 120 frame with the largest rectangle of ratio in it.
		box := func(ratio float32, label string) core.Widget {
			return basic.Container{
				Width: 200, Height: 120, Padding: geom.All(theme.Space8),
				Radius: th.Radius.Control, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
				Child: basic.Center{Child: basic.AspectRatio{Ratio: ratio, Child: tile(label, 0, 0)}},
			}
		}
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
			box(16.0/9, "16:9"), // as wide as the frame
			box(1, "1:1"),       // as tall as the frame
		}}
		// catalog:end
	})
}
