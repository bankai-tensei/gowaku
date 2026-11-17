package collection

import (
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The objects below a renderTable draw nothing: the table paints. They place
// the rows and cells in view for the focus, the hover and the accessibility
// tree, and the header for its resize grips and filter buttons.

// tableHeaderWidget is the widget of renderTableHeader. Its children are the
// header cells, then the resize grips, in the order the columns show.
type tableHeaderWidget struct {
	c     *tableCore
	cells []core.Widget
}

// Key implements core.Widget.
func (tableHeaderWidget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (h tableHeaderWidget) CreateRenderObject() core.RenderObject { return &renderTableHeader{c: h.c} }

// UpdateRenderObject implements core.RenderWidget.
func (h tableHeaderWidget) UpdateRenderObject(ro core.RenderObject) { ro.(*renderTableHeader).c = h.c }

// ChildWidgets implements core.MultiChildWidget.
func (h tableHeaderWidget) ChildWidgets() []core.Widget { return h.cells }

type renderTableHeader struct {
	core.RenderBase
	c        *tableCore
	children []core.RenderObject
	cells    []*renderTableCell
	grips    []*renderTableZone
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTableHeader) SetChildren(children []core.RenderObject) {
	r.children = children
	r.cells, r.grips = r.cells[:0], r.grips[:0]
	for _, ch := range children {
		switch ch := ch.(type) {
		case *renderTableCell:
			r.cells = append(r.cells, ch)
		case *renderTableZone:
			r.grips = append(r.grips, ch)
		}
	}
}

// ChildCount implements core.RenderObject.
func (r *renderTableHeader) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderTableHeader) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderTableHeader) Layout(cs layout.Constraints) geom.Size {
	c := r.c
	for j, cell := range r.cells {
		if j >= len(c.cols) {
			break
		}
		core.LayoutChild(cell, geom.Tight(geom.Size{W: c.cols[j].width, H: tableHeaderHeight}))
		cell.SetOffset(geom.Offset{X: c.colX(j)})
	}
	for j, grip := range r.grips {
		if j >= len(c.cols) {
			break
		}
		core.LayoutChild(grip, geom.Tight(geom.Size{W: 2 * tableGrip, H: tableHeaderHeight}))
		grip.SetOffset(geom.Offset{X: c.colX(j) + c.cols[j].width - tableGrip})
	}
	return cs.Constrain(geom.Size{W: cs.MaxW, H: tableHeaderHeight})
}

// Paint implements core.RenderObject. The table paints the header.
func (r *renderTableHeader) Paint(*paint.Canvas, geom.Offset) {}

// ChildClip implements core.Clipper. The table paints the header cut to its
// box, so the cells of columns scrolled past either end do not show.
func (r *renderTableHeader) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// HitTest implements core.RenderObject. A grip wins over the cells it lies
// on, and a frozen cell over the cells that scroll under it.
func (r *renderTableHeader) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	c := r.c
	if j := c.gripAt(pos.X); j >= 0 && j < len(r.grips) {
		res.Path = append(res.Path, r.grips[j])
	} else if j := c.colAt(pos.X); j >= 0 && j < len(r.cells) {
		cell := r.cells[j]
		cell.HitTest(pos.Sub(cell.Offset()), res)
	}
	res.Path = append(res.Path, r)
	return true
}

// Semantics implements core.RenderObject.
func (r *renderTableHeader) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleRow, RowIndex: 1}, r.Size())
	for _, cell := range r.cells {
		sb.Visit(cell)
	}
	sb.Close()
}

// tableRowWidget is the widget of renderTableRow, the row object slot.
type tableRowWidget struct {
	c     *tableCore
	slot  int
	cells []core.Widget
}

// Key implements core.Widget.
func (tableRowWidget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (w tableRowWidget) CreateRenderObject() core.RenderObject {
	return &renderTableRow{c: w.c, slot: w.slot, r: -1}
}

// UpdateRenderObject implements core.RenderWidget.
func (w tableRowWidget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTableRow)
	r.c, r.slot = w.c, w.slot
}

// ChildWidgets implements core.MultiChildWidget.
func (w tableRowWidget) ChildWidgets() []core.Widget { return w.cells }

// renderTableRow stands for the row r, or for none while r is -1.
type renderTableRow struct {
	core.RenderBase
	c     *tableCore
	slot  int
	r     int
	cells []*renderTableCell
	kids  []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTableRow) SetChildren(children []core.RenderObject) {
	r.kids = children
	r.cells = r.cells[:0]
	for _, ch := range children {
		cell := ch.(*renderTableCell)
		cell.row = r
		r.cells = append(r.cells, cell)
	}
}

// ChildCount implements core.RenderObject.
func (r *renderTableRow) ChildCount() int { return len(r.kids) }

// ChildAt implements core.RenderObject.
func (r *renderTableRow) ChildAt(i int) core.RenderObject { return r.kids[i] }

// Layout implements core.RenderObject.
func (r *renderTableRow) Layout(cs layout.Constraints) geom.Size {
	c := r.c
	for j, cell := range r.cells {
		if j >= len(c.cols) {
			break
		}
		core.LayoutChild(cell, geom.Tight(geom.Size{W: c.cols[j].width, H: c.rowH}))
		cell.SetOffset(geom.Offset{X: c.colX(j)})
	}
	return cs.Constrain(geom.Size{W: cs.MaxW, H: c.rowH})
}

// Paint implements core.RenderObject. The table paints the rows.
func (r *renderTableRow) Paint(*paint.Canvas, geom.Offset) {}

// ChildClip implements core.Clipper. The table paints the rows cut to its
// width and to the part of it below the header, so the cells of columns
// scrolled past either end, and the top of a row scrolled under the header,
// do not show.
func (r *renderTableRow) ChildClip() (geom.Rect, bool) {
	sz := r.Size()
	top := max(tableHeaderHeight-r.Offset().Y, 0)
	return geom.Rect{Y: top, W: sz.W, H: max(sz.H-top, 0)}, true
}

// Hidden implements core.Hider. A row object that shows no row keeps the
// widgets of its cells, so that the next row it shows updates them, but they
// are out of the Tab order, as they are out of painting, hit testing and the
// semantics tree.
func (r *renderTableRow) Hidden() bool { return r.r < 0 }

// HitTest implements core.RenderObject.
func (r *renderTableRow) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if r.r < 0 || pos.Y < 0 || pos.Y >= sz.H || pos.X < 0 || pos.X >= sz.W {
		return false
	}
	if j := r.c.colAt(pos.X); j >= 0 && j < len(r.cells) {
		if cell := r.cells[j]; cell.isWidget() {
			// The widget of the cell comes before the table, so that a button
			// in it takes its press.
			cell.HitTest(pos.Sub(cell.Offset()), res)
		} else {
			res.Path = append(res.Path, cell)
		}
	}
	res.Path = append(res.Path, r)
	return true
}

// HandleHover implements core.HoverTarget.
func (r *renderTableRow) HandleHover(inside bool) {
	c := r.c
	switch {
	case inside:
		c.hoverSlot = r.slot
	case c.hoverSlot == r.slot:
		c.hoverSlot = -1
	default:
		return
	}
	if c.ro != nil {
		c.ro.MarkNeedsPaint()
	}
}

// Semantics implements core.RenderObject.
func (r *renderTableRow) Semantics(sb *core.SemanticsBuilder) {
	c := r.c
	if r.r < 0 {
		return
	}
	// A row is named by its cells, in the order they show, as Chromium names
	// a row from its contents: a reader that lists the selection says which
	// rows they are.
	var name strings.Builder
	for i := range c.cols {
		if t := c.cellLabel(r.r, &c.cols[i]); t != "" {
			if name.Len() > 0 {
				name.WriteByte(' ')
			}
			name.WriteString(t)
		}
	}
	node := semantics.Node{Role: semantics.RoleRow, Label: name.String(), RowIndex: r.r + 2}
	if c.cfg.mode != TableSelectNone {
		node.Actions |= semantics.ActionTap
		if c.ctrl.IsSelected(r.r) {
			node.State |= semantics.Selected
		}
	}
	sb.Open(node, r.Size())
	for _, cell := range r.cells {
		sb.Visit(cell)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor: a tap selects the row.
func (r *renderTableRow) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	if a != semantics.ActionTap || r.r < 0 {
		return false
	}
	r.c.tapCell(r.r, r.c.focusCol())
	return true
}

// tableCellWidget is the widget of renderTableCell: column col, in the order
// they show, of the header or of the row object slot.
type tableCellWidget struct {
	c      *tableCore
	col    int
	slot   int
	header bool
}

// Key implements core.Widget.
func (tableCellWidget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (w tableCellWidget) CreateRenderObject() core.RenderObject {
	return &renderTableCell{c: w.c, col: w.col}
}

// UpdateRenderObject implements core.RenderWidget.
func (w tableCellWidget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTableCell)
	r.c, r.col = w.c, w.col
}

// ChildWidgets implements core.MultiChildWidget. A header cell of a column
// that filters holds the filter button, a cell of a CellCheck column its
// check box, and a cell of a CellWidget column the host of its widget.
func (w tableCellWidget) ChildWidgets() []core.Widget {
	if w.col >= len(w.c.cols) {
		return nil
	}
	h := &w.c.cols[w.col]
	switch {
	case w.header && h.filterable:
		return []core.Widget{tableZoneWidget{c: w.c, zone: tableZone{kind: zoneFilter, col: w.col}}}
	case !w.header && h.kind == CellCheck:
		return []core.Widget{tableCheckWidget{c: w.c, col: w.col}}
	case !w.header && h.kind == CellWidget:
		return []core.Widget{tableHost{c: w.c, slot: w.slot, col: w.col}}
	}
	return nil
}

// renderTableCell stands for a cell: of the header when row is nil, else of
// the row its row object shows. The cell in focus takes the keyboard, so that
// a screen reader reads the cell as the arrows move.
type renderTableCell struct {
	core.RenderBase
	c     *tableCore
	col   int
	row   *renderTableRow
	child core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTableCell) SetChildren(children []core.RenderObject) {
	r.child = nil
	if len(children) > 0 {
		r.child = children[0]
		if box, ok := r.child.(*renderTableCheck); ok {
			box.cell = r
		}
	}
}

// ChildCount implements core.RenderObject.
func (r *renderTableCell) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements core.RenderObject.
func (r *renderTableCell) ChildAt(int) core.RenderObject { return r.child }

// dataRow returns the row of the cell, -1 for the header and for a row object
// that shows none.
func (r *renderTableCell) dataRow() int {
	if r.row == nil {
		return -1
	}
	return r.row.r
}

// Layout implements core.RenderObject.
func (r *renderTableCell) Layout(cs layout.Constraints) geom.Size {
	sz := cs.Constrain(geom.Size{W: cs.MaxW, H: cs.MaxH})
	switch ch := r.child.(type) {
	case nil:
	case *renderTableCheck:
		trailing := ch.col < len(r.c.cols) && r.c.cols[ch.col].trailing
		core.LayoutChild(ch, geom.Tight(geom.Size{W: kit.CheckboxBox, H: kit.CheckboxBox}))
		ch.SetOffset(geom.Offset{X: checkLeft(geom.Rect{W: sz.W}, trailing), Y: geom.Floor32((sz.H - kit.CheckboxBox) / 2)})
	default:
		if r.isWidget() {
			r.layoutWidget(sz)
			break
		}
		core.LayoutChild(ch, geom.Tight(geom.Size{W: tableFilterButton, H: tableFilterButton}))
		ch.SetOffset(geom.Offset{X: sz.W - tableFilterInset - tableFilterButton, Y: (sz.H - tableFilterButton) / 2})
	}
	return sz
}

// Paint implements core.RenderObject. The table paints the cells.
func (r *renderTableCell) Paint(*paint.Canvas, geom.Offset) {}

// ChildClip implements core.Clipper. The table paints the widget of a cell
// cut to the cell.
func (r *renderTableCell) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), r.isWidget()
}

// HandleHover implements core.HoverTarget.
func (r *renderTableCell) HandleHover(inside bool) {
	c := r.c
	if r.row != nil {
		return
	}
	switch {
	case inside:
		c.hoverHeader = r.col
	case c.hoverHeader == r.col:
		c.hoverHeader = -1
	default:
		return
	}
	if c.ro != nil {
		c.ro.MarkNeedsPaint()
	}
}

// CanFocus implements core.FocusTarget.
func (r *renderTableCell) CanFocus() bool {
	c := r.c
	return !c.editing && c.ro != nil && c.ro.focusCell() == r
}

// HandleKey implements core.FocusTarget. The keys go on to the table.
func (r *renderTableCell) HandleKey(*core.KeyEvent) bool { return false }

// HandleText implements core.FocusTarget.
func (r *renderTableCell) HandleText(s string) { r.c.typed(s) }

// HandleComposition implements core.FocusTarget.
func (r *renderTableCell) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderTableCell) FocusChanged(bool) {
	if r.c.ro != nil {
		r.c.ro.MarkNeedsPaint()
	}
}

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderTableCell) FocusVisibleChanged(bool) {
	if r.c.ro != nil {
		r.c.ro.MarkNeedsPaint()
	}
}

// Semantics implements core.RenderObject.
func (r *renderTableCell) Semantics(sb *core.SemanticsBuilder) {
	c := r.c
	if r.col >= len(c.cols) {
		return
	}
	h := &c.cols[r.col]
	node := semantics.Node{ColumnIndex: r.col + 1}
	if row := r.dataRow(); r.row == nil {
		node.Role, node.Label, node.RowIndex = semantics.RoleColumnHeader, h.title, 1
		if c.cfg.sort.Column == h.model {
			// The platform says the order in the words of the reader.
			node.Sort = c.cfg.sort.Order.semantics()
		}
		if h.sortable {
			node.Actions |= semantics.ActionTap
		}
	} else {
		node.Role, node.Label, node.RowIndex = semantics.RoleGridCell, c.cellLabel(row, h), row+2
		if h.kind == CellProgress {
			node.Range = kit.PercentRange(clampShare(c.src.cellData(row, h.model).value))
		}
		node.Actions |= semantics.ActionScrollIntoView
		if c.cfg.mode != TableSelectNone {
			node.Actions |= semantics.ActionTap
			if c.ctrl.IsSelected(row) {
				node.State |= semantics.Selected
			}
		}
	}
	sb.Open(node, r.Size())
	if r.child != nil && (!r.isWidget() || r.widgetShown()) {
		sb.Visit(r.child)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderTableCell) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	c := r.c
	row := r.dataRow()
	switch {
	case r.row == nil && a == semantics.ActionTap:
		return c.sortBy(r.col)
	case r.row == nil || row < 0:
		return false
	case a == semantics.ActionTap:
		c.tapCell(row, r.col)
		return true
	case a == semantics.ActionScrollIntoView:
		c.reveal(row, r.col)
		return true
	}
	return false
}

// tableZoneWidget is the widget of renderTableZone.
type tableZoneWidget struct {
	c    *tableCore
	zone tableZone
}

// Key implements core.Widget.
func (tableZoneWidget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (w tableZoneWidget) CreateRenderObject() core.RenderObject {
	return &renderTableZone{c: w.c, zone: w.zone}
}

// UpdateRenderObject implements core.RenderWidget.
func (w tableZoneWidget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTableZone)
	r.c, r.zone = w.c, w.zone
}

// renderTableZone is a resize grip or a filter button of the header, which
// the pointer hovers. A filter button is a button to a screen reader.
type renderTableZone struct {
	core.RenderBase
	c    *tableCore
	zone tableZone
}

// Layout implements core.RenderObject.
func (r *renderTableZone) Layout(cs layout.Constraints) geom.Size {
	return cs.Constrain(geom.Size{W: cs.MaxW, H: cs.MaxH})
}

// Paint implements core.RenderObject. The table paints the header.
func (r *renderTableZone) Paint(*paint.Canvas, geom.Offset) {}

// HandleHover implements core.HoverTarget.
func (r *renderTableZone) HandleHover(inside bool) {
	c := r.c
	switch {
	case inside:
		c.hoverZone = r.zone
	case c.hoverZone == r.zone:
		c.hoverZone = tableZone{}
	default:
		return
	}
	if c.ro != nil {
		c.ro.MarkNeedsPaint()
	}
}

// MouseCursor implements core.CursorSource. A grip shows the column resize
// cursor.
func (r *renderTableZone) MouseCursor() input.Cursor {
	if r.zone.kind == zoneGrip {
		return input.CursorColResize
	}
	return input.CursorArrow
}

// Semantics implements core.RenderObject.
func (r *renderTableZone) Semantics(sb *core.SemanticsBuilder) {
	if r.zone.kind != zoneFilter {
		return
	}
	sb.Open(semantics.Node{Role: semantics.RoleButton, Label: r.c.words.Filter, Actions: semantics.ActionTap}, r.Size())
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderTableZone) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	if a != semantics.ActionTap || r.zone.kind != zoneFilter {
		return false
	}
	r.c.openFilter(r.zone.col)
	return true
}
