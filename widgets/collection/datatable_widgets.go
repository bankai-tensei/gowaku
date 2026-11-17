package collection

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The cells of a CellWidget column hold the widgets of the caller. Every
// other cell is drawn by the table itself; these are real subtrees, one per
// row object and widget column, so that a widget lays itself out, paints,
// takes the pointer and the keyboard and describes itself as it does
// anywhere. Each subtree sits under a tableHost, whose state builds the
// widget of the row its row object shows. A build of the table builds every
// host again (DidUpdateWidget). When a scroll brings another row into a row
// object, only the hosts of that row object build again, before the layout
// of the frame. See docs/ADR/0100-table-widget-cells.md.

// tableWidgetInset is the room above and below the widget of a cell, so that
// a widget that takes all the room it is given does not touch the next row.
// It is small enough that a line of Body in a Container with 4 above and
// below, 28 high, fits a row of the default 32.
const tableWidgetInset = theme.Space2

// cellUnbuilt marks a widget cell whose host has not built since the last
// build of the table.
const cellUnbuilt = -2

// tableHost is the widget of the subtree of a widget cell: the column col, in
// the order they show, of the row object slot.
type tableHost struct {
	core.StatefulBase
	c         *tableCore
	slot, col int
}

// CreateState implements core.StatefulWidget.
func (tableHost) CreateState() core.State { return &tableHostState{row: cellUnbuilt} }

// tableHostState builds the widget of the row that its row object shows. row
// is the row it built for, -1 for none, and last the widget it built.
type tableHostState struct {
	core.StateOf[tableHost]
	c       *tableCore
	row     int
	last    core.Widget
	stale   bool
	nothing func()
}

// InitState implements core.State.
func (s *tableHostState) InitState() {
	s.c = s.Widget().c
	s.nothing = func() {}
	s.c.hosts = append(s.c.hosts, s)
}

// DidUpdateWidget implements core.State. A table that rebuilt may show other
// rows, columns or widgets, so the host builds again with it, which core does
// after this.
func (s *tableHostState) DidUpdateWidget(core.Widget) {}

// Dispose implements core.State.
func (s *tableHostState) Dispose() {
	hosts := s.c.hosts
	if i := slices.Index(hosts, s); i >= 0 {
		last := len(hosts) - 1
		hosts[i] = hosts[last]
		hosts[last] = nil
		s.c.hosts = hosts[:last]
	}
}

// Build implements core.State.
func (s *tableHostState) Build(core.BuildContext) core.Widget {
	w := s.Widget()
	c := w.c
	s.stale = false
	s.row = c.slotRow(w.slot)
	c.setCellRow(w.slot, w.col, s.row)
	if s.row < 0 || w.col >= len(c.cols) {
		// A row object that shows no row keeps its widget, hidden, so that
		// the next row it shows updates the widget rather than make one.
		s.row = -1
		if s.last != nil {
			return s.last
		}
		return c.empty
	}
	s.last = c.empty
	if child := c.src.cellWidget(s.row, c.cols[w.col].model); child != nil {
		s.last = child
	}
	return s.last
}

// refresh asks the host to build again if its row object shows another row
// than the one it built for. One that shows no row keeps its widget hidden
// until it shows one.
func (s *tableHostState) refresh() {
	if s.stale || !s.Mounted() {
		return
	}
	if row := s.c.slotRow(s.Widget().slot); row < 0 || row == s.row {
		return
	}
	s.stale = true
	s.SetState(s.nothing)
}

// slotRow returns the row that row object k shows at the current offsets, or
// -1: the layout gives row i the object i modulo their number.
func (c *tableCore) slotRow(k int) int {
	n := c.built
	if k < 0 || k >= n {
		return -1
	}
	first, last := c.visibleRows()
	i := first + ((k-first)%n+n)%n
	if i >= last {
		return -1
	}
	return i
}

// syncHosts builds again the hosts whose row objects show other rows than
// they built for. It runs on every move of the scroll and after a layout, and
// allocates nothing when no row changed.
func (c *tableCore) syncHosts() {
	for _, h := range c.hosts {
		h.refresh()
	}
}

// setCellRow records that the widget cell of row object slot and column col
// was built for row.
func (c *tableCore) setCellRow(slot, col, row int) {
	if k := slot*len(c.cols) + col; col < len(c.cols) && k >= 0 && k < len(c.cellRows) {
		c.cellRows[k] = row
	}
}

// cellRow returns the row that the widget cell of row object slot and
// column col was built for, cellUnbuilt if it has not built since the table
// did.
func (c *tableCore) cellRow(slot, col int) int {
	if k := slot*len(c.cols) + col; col < len(c.cols) && k >= 0 && k < len(c.cellRows) {
		return c.cellRows[k]
	}
	return cellUnbuilt
}

// resetCellRows makes room for the rows of the widget cells of a build with
// slots row objects.
func (c *tableCore) resetCellRows(slots int) {
	n := slots * len(c.cols)
	if !c.widgets {
		n = 0
	}
	if cap(c.cellRows) < n {
		c.cellRows = make([]int, n)
	}
	c.cellRows = c.cellRows[:n]
	for i := range c.cellRows {
		c.cellRows[i] = cellUnbuilt
	}
}

// isWidget reports whether the cell holds the widget of a row: a cell of a
// CellWidget column in a row object.
func (r *renderTableCell) isWidget() bool {
	return r.row != nil && r.col < len(r.c.cols) && r.c.cols[r.col].kind == CellWidget
}

// widgetShown reports whether the cell holds a widget built for the row its
// row object shows. A widget built for another row, while its host waits to
// build again, is neither painted, hit nor described.
func (r *renderTableCell) widgetShown() bool {
	return r.child != nil && r.row.r >= 0 && r.c.cellRow(r.row.slot, r.col) == r.row.r
}

// layoutWidget lays out the widget of the cell, of size sz: as wide as the
// cell and as tall as the row allow, less the padding, at the start of the
// cell or at its end for a trailing column, in the middle of the height. The
// room is the same for every widget and every row, so that the widgets of a
// column line up.
func (r *renderTableCell) layoutWidget(sz geom.Size) {
	room := geom.Size{W: max(sz.W-2*tablePadding, 0), H: max(sz.H-2*tableWidgetInset, 0)}
	csz := core.LayoutChild(r.child, geom.Constraints{MaxW: room.W, MaxH: room.H})
	x := float32(tablePadding)
	if r.c.cols[r.col].trailing {
		x = sz.W - tablePadding - csz.W
	}
	r.child.SetOffset(geom.Offset{X: x, Y: geom.Floor32((sz.H - csz.H) / 2)})
}

// HitTest implements core.RenderObject. A widget that waits to build again
// for the row of its row object is not hit; any other child is, as by
// default.
func (r *renderTableCell) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	if r.isWidget() && !r.widgetShown() {
		sz := r.Size()
		if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
			return false
		}
		res.Path = append(res.Path, r)
		return true
	}
	return r.RenderBase.HitTest(pos, res)
}

// FocusWithinChanged implements core.FocusWithinListener. When the keyboard
// goes into the widget of the cell, by a click or by Tab, the cell becomes the
// cell in focus, so that the arrows go on from it. The selection stays, as for
// a check box.
func (r *renderTableCell) FocusWithinChanged(within bool) {
	c := r.c
	if !within || !r.isWidget() || core.RenderObject(r.Focused()) == r {
		return
	}
	row := r.dataRow()
	if row < 0 || c.editing {
		return
	}
	model := c.cols[r.col].model
	if c.ctrl.row == row && c.ctrl.col == model {
		return
	}
	c.ctrl.row, c.ctrl.col = row, model
	c.changed()
}

// paintWidget paints the widget of the cell of row i and the column at j, in
// box, cut to the cell.
func (r *renderTable) paintWidget(cv *paint.Canvas, off geom.Offset, i, j int, box geom.Rect) {
	n := len(r.rows)
	if n == 0 {
		return
	}
	slot := r.rows[i%n]
	if slot.r != i || j >= len(slot.cells) {
		return
	}
	cell := slot.cells[j]
	if !cell.widgetShown() {
		return
	}
	child := cell.child
	cv.Save()
	cv.ClipRect(box)
	child.Paint(cv, off.Add(slot.Offset()).Add(cell.Offset()).Add(child.Offset()))
	cv.Restore()
}

// cellAround returns the cell of the table that holds f in its widget, or
// nil.
func (r *renderTable) cellAround(f core.FocusTarget) *renderTableCell {
	if f == nil {
		return nil
	}
	for ro := core.RenderObject(f).Parent(); ro != nil && ro != core.RenderObject(r); ro = ro.Parent() {
		if cell, ok := ro.(*renderTableCell); ok && cell.c == r.c {
			return cell
		}
	}
	return nil
}

// widgetKey handles a key that the widget in focus inside a cell, or the cell
// of a CellWidget column, left to the table: Escape takes the keyboard back to
// the cell, and Enter or F2 on the cell gives it to the widget. It reports
// whether it used the key.
func (c *tableCore) widgetKey(e *core.KeyEvent) bool {
	r := c.ro
	if r == nil || e.Repeat || e.Modifiers != 0 {
		return false
	}
	switch e.Key {
	case input.KeyEscape:
		cell := r.cellAround(r.Focused())
		if cell == nil {
			return false
		}
		cell.RequestFocus()
		return true
	case input.KeyEnter, input.KeyF2:
		cell := r.focusCell()
		if cell == nil || core.FocusTarget(cell) != r.Focused() || !cell.isWidget() || !cell.widgetShown() {
			return false
		}
		f := kit.FirstFocusable(cell.child)
		if f == nil {
			return false
		}
		core.RestoreFocus(f)
		return true
	}
	return false
}
