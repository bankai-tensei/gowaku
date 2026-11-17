package catalog

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// The scenes of the widgets that the other files leave out:
// Disclosure and Link (ADR 0087), GridView (0090), ReorderableList (0091),
// InteractiveViewer (0092), NumberField, RangeSlider and TokenField (0093),
// and DockLayout.

func sceneControlDisclosure() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		note := colored(th.Text.Body, th.Color.Ink2)
		return basic.SizedBox{Width: 380, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
			// catalog:begin
			control.Disclosure{Title: "Advanced", Child: basic.Text{Content: "Closed, the content is not laid out.", Style: note}},
			control.Disclosure{Title: "Sync", Open: true, Child: basic.Text{
				Content: "Open starts it open. Without OnChange it keeps its own state.", Style: note,
			}},
			control.Disclosure{Title: "Notifications", Header: basic.Row{Size: layout.MainMin, Spacing: theme.Space6, Children: []core.Widget{
				basic.Text{Content: "Notifications", Style: th.Text.Headline}, control.Badge{Count: 3},
			}}, Child: basic.Text{Content: "A header can hold a badge.", Style: note}},
			// catalog:end
		}}}
	})
}

func sceneControlLink() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		note := colored(th.Text.Body, th.Color.Ink2)
		return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
			// catalog:begin
			basic.Row{Size: layout.MainMin, Spacing: theme.Space4, Children: []core.Widget{
				basic.Text{Content: "Sync is off.", Style: note},
				control.Link{Label: "Turn it on", OnPress: func() {}},
			}},
			control.Link{Label: "Release notes", Style: th.Text.Caption, OnPress: func() {}},
			control.Link{Label: "Disabled without OnPress"},
			// catalog:end
		}}
	})
}

func sceneControlRangeSlider() core.Widget {
	set := func(_, _ float32) {}
	return basic.SizedBox{Width: 320, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		// catalog:begin
		control.RangeSlider{
			Label: "Price", Low: 20000, High: 60000, Min: 0, Max: 100000,
			Step: 1000, Snap: true, MinGap: 5000, OnChange: set,
		},
		control.RangeSlider{Label: "Disabled", Low: 0.3, High: 0.6},
		// catalog:end
	}}}
}

func sceneFieldNumberField() core.Widget {
	set := func(float64) {}
	return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Children: []core.Widget{
		// catalog:begin
		basic.SizedBox{Width: 140, Child: field.NumberField{Label: "Quantity", Value: 3, Min: 0, Max: 99, OnChange: set}},
		basic.SizedBox{Width: 180, Child: field.NumberField{Label: "Price", Value: 128000, Step: 100, Grouping: true, OnChange: set}},
		basic.SizedBox{Width: 140, Child: field.NumberField{Label: "Ratio", Value: 0.25, Min: 0, Max: 1, Step: 0.05, OnChange: set}},
		// catalog:end
	}}
}

func sceneFieldTokenField() core.Widget {
	set := func([]string) {}
	return basic.SizedBox{Width: 380, Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
		// catalog:begin
		field.TokenField{Label: "Tags", Tokens: []string{"design", "go", "typography"}, Placeholder: "Add a tag", OnChange: set},
		field.TokenField{Label: "Disabled", Tokens: []string{"read only"}},
		// catalog:end
	}}}
}

func sceneInteractInteractiveViewer() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		c := &th.Color
		shape := func(x, y, w, h, r float32, color geom.Color) core.Widget {
			return basic.Positioned{Left: x, Top: y, Width: w, Height: h, Child: basic.Container{Color: color, Radius: r}}
		}
		drawing := basic.Container{Width: 900, Height: 560, Color: c.Card, Child: basic.Stack{Children: []core.Widget{
			shape(80, 80, 300, 180, th.Radius.Dialog, c.AccentTint),
			shape(420, 220, 220, 220, 110, c.Accent),
			shape(680, 60, 160, 160, 24, c.FillStrong),
			shape(120, 360, 260, 120, 60, c.Fill),
			basic.Positioned{Left: 110, Top: 180, Child: control.Button{Label: "Open", OnPress: func() {}}},
			basic.Positioned{Left: 690, Top: 240, Child: control.Button{Label: "Details", OnPress: func() {}}},
		}}}
		// catalog:begin
		var view interact.ViewerController
		view.Fit(false) // once the viewer knows its size
		return basic.SizedBox{Width: 440, Height: 280, Child: basic.Container{
			Radius: th.Radius.Control, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			Child: interact.InteractiveViewer{
				Controller: &view, MinScale: 0.2, MaxScale: 6, BoundaryMargin: 16,
				DoubleTapZoom: true, Label: "Drawing", Child: drawing,
			},
		}}
		// catalog:end
	})
}

func sceneCollectionGridView() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		small := colored(th.Text.Caption, th.Color.Ink2)
		// catalog:begin
		var photos collection.TableController
		photos.SetSelection(collection.RowRange{Start: 1, End: 3})
		return basic.SizedBox{Width: 460, Height: 280, Child: collection.GridView{
			ItemCount: 10000, MinColumnWidth: 96, AspectRatio: 4.0 / 3, ExtraHeight: 20,
			Spacing: theme.Space8, RunSpacing: theme.Space8, Padding: geom.All(theme.Space8),
			Label: "Photos", Controller: &photos,
			LabelOf: func(i int) string { return "Photo " + strconv.Itoa(i+1) },
			ItemBuilder: func(i int) core.Widget {
				tone := theme.TonalPalette{Hue: float64(i*47%360) + 0.5, Chroma: 36}.Tone(66)
				return basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space4, Children: []core.Widget{
					basic.Expanded{Child: basic.Container{Color: tone, Radius: th.Radius.Control}},
					basic.Text{Content: "Photo " + strconv.Itoa(i+1), Style: small},
				}}
			},
		}}
		// catalog:end
	})
}

func sceneCollectionReorderableList() core.Widget {
	todos := []string{"Buy milk", "Pay the bills", "Book a dentist", "Return the books", "Water the plants"}
	// catalog:begin
	return basic.SizedBox{Width: 300, Height: 220, Child: collection.ReorderableList{
		ItemCount: len(todos), ItemExtent: 44, Label: "To do",
		LabelOf: func(i int) string { return todos[i] },
		// Move the item in your data, then build again.
		OnReorder: func(_, _ int) {},
		ItemBuilder: func(i int) core.Widget {
			return basic.Padding{Insets: geom.Symmetric(theme.Space12, 0), Child: basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
				basic.Expanded{Child: basic.Text{Content: todos[i]}},
				basic.Text{Content: "≡"},
			}}}
		},
	}}
	// catalog:end
}

func sceneNavDockLayout() core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		note := func(s string) func(core.BuildContext) core.Widget {
			return func(core.BuildContext) core.Widget {
				return basic.Padding{Insets: geom.All(theme.Space12), Child: basic.Text{Content: s, Style: colored(th.Text.Body, th.Color.Ink2)}}
			}
		}
		// catalog:begin
		panels := []nav.DockPanel{
			{ID: "files", Title: "Files", Build: note("go.mod\nmain.go\ndock.go")},
			{ID: "main", Title: "main.go", Closable: true, Build: note("package main")},
			{ID: "dock", Title: "dock.go", Closable: true, Build: note("package main")},
			{ID: "output", Title: "Output", Closable: true, Build: note("$ go build ./...")},
		}
		start := nav.DockArrangement{Root: nav.DockNode{
			Weights: []float32{1, 3},
			Children: []nav.DockNode{
				{Panels: []string{"files"}},
				{Axis: layout.Vertical, Weights: []float32{2, 1}, Children: []nav.DockNode{
					{Panels: []string{"main", "dock"}, Active: "main"},
					{Panels: []string{"output"}},
				}},
			},
		}}
		return basic.SizedBox{Width: 560, Height: 300, Child: nav.DockLayout{Panels: panels, Default: start, Label: "Panels"}}
		// catalog:end
	})
}
