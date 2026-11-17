package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Grid lays out its children as tiles in columns of the same width, row by row
// from the top left. It suits album covers, photos and cards.
//
// Columns fixes the number of columns. When it is zero, the Grid fits as many
// columns as MinColumnWidth allows, so the tiles grow with the window until
// another column fits. Spacing is the gap between columns and RunSpacing the
// gap between rows.
//
// The height of a tile is TileHeight if set. Otherwise it is the column width
// over AspectRatio plus ExtraHeight, so AspectRatio: 1, ExtraHeight: 40 makes a
// square cover with room for a caption. With neither, each row is as tall as
// its tallest child and stretches the others to match.
//
// A Grid takes all the width it is allowed and is as tall as its rows, so it
// works inside a vertical Scroll. Its width must be bounded: inside a Row or a
// horizontal Scroll it panics unless a SizedBox or an Expanded gives it one.
// Every child is built and laid out; for thousands of tiles, page them.
//
// The zero value lays out its children in a single column.
type Grid struct {
	Columns        int
	MinColumnWidth float32
	Spacing        float32
	RunSpacing     float32
	TileHeight     float32
	AspectRatio    float32
	ExtraHeight    float32
	Children       []core.Widget
}

// Key implements core.Widget.
func (Grid) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (g Grid) CreateRenderObject() core.RenderObject {
	r := &layout.RenderGrid{}
	g.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (g Grid) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderGrid)
	if r.Columns != g.Columns || r.MinColumnWidth != g.MinColumnWidth || r.Spacing != g.Spacing ||
		r.RunSpacing != g.RunSpacing || r.TileHeight != g.TileHeight || r.AspectRatio != g.AspectRatio ||
		r.ExtraHeight != g.ExtraHeight {
		g.apply(r)
		r.MarkNeedsLayout()
	}
}

func (g Grid) apply(r *layout.RenderGrid) {
	r.Columns, r.MinColumnWidth, r.Spacing, r.RunSpacing = g.Columns, g.MinColumnWidth, g.Spacing, g.RunSpacing
	r.TileHeight, r.AspectRatio, r.ExtraHeight = g.TileHeight, g.AspectRatio, g.ExtraHeight
}

// ChildWidgets implements core.MultiChildWidget.
func (g Grid) ChildWidgets() []core.Widget { return g.Children }
