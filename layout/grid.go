package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// gridUnbounded is the panic of a grid without a width. It is a constant so
// that Layout does not build it.
const gridUnbounded = "gowaku: Grid has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded"

// RenderGrid lays out its children in columns of the same width, row by row
// from the top left. It takes all the width it is allowed, which must be
// bounded, and is as tall as its rows. Inside a vertical Scroll it works as is.
type RenderGrid struct {
	core.RenderBase
	// Columns fixes the number of columns. Zero fits as many columns as
	// MinColumnWidth allows, and at least one.
	Columns int
	// MinColumnWidth is the narrowest a column may be when Columns is zero.
	// Zero means a single column.
	MinColumnWidth float32
	// Spacing is the gap between columns, and RunSpacing the gap between rows.
	Spacing    float32
	RunSpacing float32
	// TileHeight fixes the height of every tile. Zero falls back on
	// AspectRatio.
	TileHeight float32
	// AspectRatio is the width over the height of the part of a tile that
	// scales with the column, such as a cover image. ExtraHeight is added to
	// that, such as for a caption under the cover. Zero leaves each row as tall
	// as its tallest child, and every child of the row is stretched to that.
	AspectRatio float32
	ExtraHeight float32
	children    []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderGrid) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderGrid) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderGrid) ChildAt(i int) core.RenderObject { return r.children[i] }

// ColumnsFor returns the number of columns in a grid of the given width.
func (r *RenderGrid) ColumnsFor(width float32) int {
	if r.Columns > 0 {
		return r.Columns
	}
	if r.MinColumnWidth <= 0 {
		return 1
	}
	return max(1, int((width+r.Spacing)/(r.MinColumnWidth+r.Spacing)))
}

// tileHeight returns the fixed height of a tile of the given width, or zero if
// the children decide it.
func (r *RenderGrid) tileHeight(width float32) float32 {
	switch {
	case r.TileHeight > 0:
		return r.TileHeight
	case r.AspectRatio > 0:
		return width/r.AspectRatio + r.ExtraHeight
	}
	return 0
}

// Layout implements core.RenderObject.
func (r *RenderGrid) Layout(c Constraints) geom.Size {
	if c.MaxW >= geom.Inf {
		panic(gridUnbounded)
	}
	width := c.MaxW
	cols := r.ColumnsFor(width)
	tile := max((width-r.Spacing*float32(cols-1))/float32(cols), 0)
	fixed := r.tileHeight(tile)

	var y float32
	for start := 0; start < len(r.children); start += cols {
		row := r.children[start:min(start+cols, len(r.children))]
		height := fixed
		if height <= 0 {
			for _, child := range row {
				height = max(height, core.LayoutChild(child, Constraints{MinW: tile, MaxW: tile, MaxH: geom.Inf}).H)
			}
		}
		if start > 0 {
			y += r.RunSpacing
		}
		tight := geom.Tight(geom.Size{W: tile, H: height})
		for i, child := range row {
			core.LayoutChild(child, tight)
			child.SetOffset(geom.Offset{X: float32(i) * (tile + r.Spacing), Y: y})
		}
		y += height
	}
	return c.Constrain(geom.Size{W: width, H: y})
}

// Paint implements core.RenderObject.
func (r *RenderGrid) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
