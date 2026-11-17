package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// paneScenes returns the scenes of SplitPane, PropertyGrid and Breadcrumb: a
// folded trail above a split whose divider has the focus, with an inspector in
// its right pane that shows the description of the property last in focus.
func paneScenes() []Scene {
	return []Scene{
		{Name: "pane", Width: 720, Height: 540, Scale: 1, Root: panes, Act: focusDivider},
		{Name: "pane-dark", Width: 1080, Height: 810, Scale: 1.5, Dark: true, Root: panes, Act: focusDivider},
	}
}

// focusDivider tabs to the editor of Name, whose description stays in the
// inspector, and back to the divider, which shows its focus band.
func focusDivider(tree *core.Tree, _ core.BuildContext) {
	focused := func() *semantics.Node {
		var out *semantics.Node
		tree.Semantics().Walk(func(n *semantics.Node) bool {
			if n.State&semantics.Focused != 0 {
				out = n
			}
			return out == nil
		})
		return out
	}
	size := tree.RootRenderObject().Size()
	settle := func() {
		for tree.NeedsBuild() {
			tree.Build()
			tree.Layout(geom.Tight(size))
		}
	}
	for range 20 {
		tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		settle()
		if f := focused(); f != nil && f.Label == "Name" && f.Role == semantics.RoleTextField {
			break
		}
	}
	for range 20 {
		tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
		settle()
		if f := focused(); f != nil && f.Role == semantics.RoleSeparator {
			break
		}
	}
}

// paneIcon returns a 24 unit folder.
func paneIcon() *paint.Path {
	var p paint.Path
	p.AddRRect(geom.RRectAll(geom.Rect{X: 3, Y: 6, W: 18, H: 13}, 2.5))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 3, Y: 4, W: 8, H: 4}, 1.5))
	return &p
}

func panes() core.Widget {
	folder := paneIcon()
	trail := nav.Breadcrumb{OnSelect: func(int) {}, Items: []nav.BreadcrumbItem{
		{Label: "Home", Icon: folder}, {Label: "Documents"}, {Label: "Projects"}, {Label: "gowaku"}, {Label: "widgets"},
	}}
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		names := []string{"Overview", "Layers", "Assets", "History"}
		side := make([]core.Widget, 0, len(names))
		for _, n := range names {
			side = append(side, basic.Padding{Insets: geom.Symmetric(theme.Space16, theme.Space6), Child: basic.Text{Content: n}})
		}
		return basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{
			Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
				basic.Row{Spacing: theme.Space24, Children: []core.Widget{
					basic.SizedBox{Width: 300, Child: trail},
					basic.Expanded{Child: trail},
				}},
				basic.Expanded{Child: basic.Container{
					Color: th.Color.Card, Radius: th.Radius.Group, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
					Child: control.SplitPane{
						Position: 200, Pixels: true, MinFirst: 120, MinSecond: 240, Collapsible: control.SplitCollapseFirst,
						First: basic.Container{Color: th.Color.Sidebar, Radius: th.Radius.Group, Padding: geom.Insets{Top: theme.Space12},
							Child: basic.Column{Cross: layout.CrossStretch, Children: side}},
						Second: inspector(),
					},
				}},
			},
		}}
	}}
}

func inspector() core.Widget {
	type fit int
	return collection.PropertyGrid{Label: "Inspector", Categories: []collection.PropertyCategory{
		{Name: "General", Properties: []collection.Property{
			{Name: "Name", Description: "The name of the layer, as the list of layers shows it.",
				Editor: collection.PropertyText{Value: "Header", OnChange: func(string) {}}},
			{Name: "Kind", Editor: collection.PropertyText{Value: "Rectangle"}},
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
	}}
}
