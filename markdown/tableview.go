package markdown

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// ruleWidth is the width of the lines of a table.
const ruleWidth = 1

// minSlack is added to the natural and the least width of a cell: adding the
// padding to the width of the text and taking it off again in float32 can
// round down, which would leave the text a hair short of its line or of its
// widest word, and break it there.
const minSlack = 1.0 / 64

// tableView lays out the rows of a table in columns as wide as their widest
// cell. When the columns do not fit, they shrink toward the width of their
// widest word and their text wraps between words; only when even the words do
// not fit do they shrink further and break inside words. With fill, the first
// row is the header, on a filled ground.
type tableView struct {
	cols         int
	rule, header geom.Color
	fill         bool
	rows         []core.Widget
}

// Key implements core.Widget.
func (tableView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tableView) CreateRenderObject() core.RenderObject {
	return &renderTable{cols: t.cols, rule: t.rule, header: t.header, fill: t.fill}
}

// UpdateRenderObject implements core.RenderWidget.
func (t tableView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTable)
	if r.cols != t.cols {
		r.cols = t.cols
		r.MarkNeedsLayout()
	}
	if r.rule != t.rule || r.header != t.header || r.fill != t.fill {
		r.rule, r.header, r.fill = t.rule, t.header, t.fill
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (t tableView) ChildWidgets() []core.Widget { return t.rows }

// tableRow is a row of cells, which the table around it lays out.
type tableRow struct {
	cells []core.Widget
}

// Key implements core.Widget.
func (tableRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (tableRow) CreateRenderObject() core.RenderObject { return &renderTableRow{} }

// UpdateRenderObject implements core.RenderWidget.
func (tableRow) UpdateRenderObject(core.RenderObject) {}

// ChildWidgets implements core.MultiChildWidget.
func (t tableRow) ChildWidgets() []core.Widget { return t.cells }

// renderTable lays out the cells of all its rows itself, since the width of
// a column depends on every row. A row only holds its cells: it takes the
// size the table gives it and paints them where the table put them.
type renderTable struct {
	core.RenderBase
	cols         int
	rule, header geom.Color
	fill         bool
	rows         []core.RenderObject
	// widths are the widths of the columns at the last layout, and ys the top
	// of each row followed by the bottom of the table. mins are the widths
	// below which a column would have to break a word.
	widths, ys, mins []float32
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTable) SetChildren(children []core.RenderObject) { r.rows = children }

// ChildCount implements core.RenderObject.
func (r *renderTable) ChildCount() int { return len(r.rows) }

// ChildAt implements core.RenderObject.
func (r *renderTable) ChildAt(i int) core.RenderObject { return r.rows[i] }

// Layout implements core.RenderObject.
func (r *renderTable) Layout(c geom.Constraints) geom.Size {
	r.widths, r.mins = r.widths[:0], r.mins[:0]
	for range r.cols {
		r.widths = append(r.widths, 0)
		r.mins = append(r.mins, 0)
	}
	// The natural width of each column is its widest cell on one line, and
	// its least width the widest word of its cells, both with minSlack.
	unbounded := geom.Constraints{MaxW: geom.Inf, MaxH: geom.Inf}
	for _, row := range r.rows {
		for j := range min(row.ChildCount(), r.cols) {
			cell := row.ChildAt(j)
			w := core.LayoutChild(cell, unbounded).W
			r.widths[j] = max(r.widths[j], w+minSlack)
			r.mins[j] = max(r.mins[j], cellMinWidth(cell, w))
		}
	}
	var natural, least float32
	for j, w := range r.widths {
		natural += w
		least += r.mins[j]
	}
	room := c.MaxW - float32(r.cols+1)*ruleWidth
	switch {
	case natural <= room || room <= 0:
	case least < room:
		// Every column keeps its widest word, and the room left over goes to
		// each column by how much it has to give, as in the automatic layout
		// of an HTML table.
		share := (room - least) / (natural - least)
		for j := range r.widths {
			r.widths[j] = r.mins[j] + (r.widths[j]-r.mins[j])*share
		}
	default:
		// Not even the words fit: they shrink in proportion and break.
		for j := range r.widths {
			r.widths[j] = r.mins[j] * room / least
		}
	}
	var width float32 = ruleWidth
	for _, w := range r.widths {
		width += w + ruleWidth
	}
	r.ys = r.ys[:0]
	var y float32 = ruleWidth
	for _, row := range r.rows {
		var h float32
		x := float32(ruleWidth)
		for j := range min(row.ChildCount(), r.cols) {
			cell := row.ChildAt(j)
			w := r.widths[j]
			sz := core.LayoutChild(cell, geom.Constraints{MinW: w, MaxW: w, MaxH: geom.Inf})
			cell.SetOffset(geom.Offset{X: x})
			x += w + ruleWidth
			h = max(h, sz.H)
		}
		// The height is a lower bound, not a tight size, so that the row
		// is no relayout boundary: a cell that changes lays the table out
		// again.
		core.LayoutChild(row, geom.Constraints{MinW: width, MaxW: width, MinH: h, MaxH: geom.Inf})
		row.SetOffset(geom.Offset{Y: y})
		r.ys = append(r.ys, y)
		y += h + ruleWidth
	}
	r.ys = append(r.ys, y)
	return c.Constrain(geom.Size{W: width, H: y})
}

// Paint implements core.RenderObject.
func (r *renderTable) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if r.fill && len(r.rows) > 0 {
		head := geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: r.ys[1]}
		cv.DrawRRect(geom.RRect{Rect: head}, paint.Paint{Color: r.header})
	}
	for _, row := range r.rows {
		row.Paint(cv, off.Add(row.Offset()))
	}
	line := paint.Paint{Color: r.rule}
	for _, y := range r.ys {
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: off.Y + y - ruleWidth, W: sz.W, H: ruleWidth}}, line)
	}
	x := off.X
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: x, Y: off.Y, W: ruleWidth, H: sz.H}}, line)
	for _, w := range r.widths {
		x += w + ruleWidth
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: x, Y: off.Y, W: ruleWidth, H: sz.H}}, line)
	}
}

// Semantics implements core.RenderObject. The table is a grid that cannot be
// changed, the first row holding the column headers.
func (r *renderTable) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{
		Role: semantics.RoleGrid, State: semantics.ReadOnly, RowCount: len(r.rows), ColumnCount: r.cols,
	}, r.Size())
	for _, row := range r.rows {
		sb.Visit(row)
	}
	sb.Close()
}

type renderTableRow struct {
	core.RenderBase
	cells []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTableRow) SetChildren(children []core.RenderObject) { r.cells = children }

// ChildCount implements core.RenderObject.
func (r *renderTableRow) ChildCount() int { return len(r.cells) }

// ChildAt implements core.RenderObject.
func (r *renderTableRow) ChildAt(i int) core.RenderObject { return r.cells[i] }

// Layout implements core.RenderObject. The table has laid out the cells
// already.
func (r *renderTableRow) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: c.MinW, H: c.MinH})
}

// Paint implements core.RenderObject.
func (r *renderTableRow) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, cell := range r.cells {
		cell.Paint(cv, off.Add(cell.Offset()))
	}
}

// Semantics implements core.RenderObject.
func (r *renderTableRow) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleRow}, r.Size())
	for _, cell := range r.cells {
		sb.Visit(cell)
	}
	sb.Close()
}

// cellMinWidth returns the width below which the cell, whose natural width is
// natural, would break a word: the least width of its text plus the padding
// around it, and minSlack. It is at most natural plus minSlack.
func cellMinWidth(cell core.RenderObject, natural float32) float32 {
	ro := cell
	for {
		if r, ok := ro.(*renderRich); ok {
			return min(natural, r.minWidth()+natural-r.Size().W) + minSlack
		}
		if ro.ChildCount() != 1 {
			return natural + minSlack
		}
		ro = ro.ChildAt(0)
	}
}
