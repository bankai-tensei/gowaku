package collection

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// pointer handles the pointer events that reach the table.
func (c *tableCore) pointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerScroll:
		dx, dy := e.Scroll.X, e.Scroll.Y
		if e.Modifiers&input.ModShift != 0 && dx == 0 {
			// Shift turns the wheel sideways, as in browsers.
			dx, dy = -dy, 0
		}
		if dy != 0 && c.vert.Metrics.Content > c.vert.Metrics.View {
			c.vert.Wheel(-dy)
			e.Handled = true
		}
		if dx != 0 && c.horz.Metrics.Content > c.horz.Metrics.View {
			c.horz.Wheel(dx)
			e.Handled = true
		}
	case core.PointerDown:
		c.menuPress = false
		e.Handled = c.press(e)
	case core.PointerMove:
		c.dragTo(e.Pos.X)
	case core.PointerUp:
		if c.menuPress {
			c.menuPress = false
			if row := c.rowAt(e.Pos.Y); row == c.menuRow && e.Pos.X >= 0 && e.Pos.X < c.viewW && c.cfg.onContextMenu != nil {
				c.cfg.onContextMenu(row, c.toTable(e.Pos))
			}
			return
		}
		c.release()
	case core.PointerCancel:
		c.menuPress = false
		c.drag = tableDrag{}
		c.repaint()
	}
}

// toTable turns pos, from the top left of the render object of the table, into
// a place from the top left of the widget, as OnContextMenu reports it.
func (c *tableCore) toTable(pos geom.Offset) geom.Offset {
	if c.ro == nil || c.base == nil {
		return pos
	}
	return toLocal(c.base.Context(), core.LocalToGlobal(c.ro, pos))
}

// secondaryPress is a press of the secondary button on the cell of row and
// column j: it selects the row alone unless it is selected already, and
// moves the focus there, as a desktop does before it shows a context menu. It
// reports whether the table keeps the press for OnContextMenu; otherwise the
// press goes on to an overlay.ContextMenu around the table.
func (c *tableCore) secondaryPress(row, j int, e *core.PointerEvent) bool {
	if !c.ctrl.IsSelected(row) {
		c.click(row, j, 0, 1)
	} else {
		if c.editing && !c.commitEdit(true) {
			return true
		}
		c.ctrl.row, c.ctrl.col = row, c.cols[j].model
		c.reveal(row, j)
		c.focusNow()
		c.changed()
	}
	if c.cfg.onContextMenu == nil || e.Button != input.ButtonSecondary {
		// The keyboard comes back at the next layout if nothing took the
		// press, which clears the focus; a menu that opened keeps it.
		c.refocus = true
		c.relayout()
		return e.Button != input.ButtonSecondary
	}
	c.menuPress, c.menuRow = true, row
	return true
}

func (c *tableCore) repaint() {
	if c.ro != nil {
		c.ro.MarkNeedsPaint()
	}
}

func (c *tableCore) relayout() {
	if c.ro != nil {
		c.ro.MarkNeedsLayout()
	}
}

// contextMenuKey asks for the context menu of the row in focus below its cell
// in focus, for the menu key or Shift+F10, and reports whether the table has
// OnContextMenu.
func (c *tableCore) contextMenuKey() bool {
	row, j := c.focusRow(), c.focusCol()
	if c.cfg.onContextMenu == nil || row < 0 || row >= c.cfg.rows || j < 0 {
		return false
	}
	at := geom.Offset{X: max(c.colX(j), 0), Y: c.rowY(row) + c.rowH}
	if j >= c.frozen {
		at.X = max(at.X, c.frozenW)
	}
	c.cfg.onContextMenu(row, c.toTable(at))
	return true
}

// press starts what a press on the table does: resize or move a column, open
// a filter, or select a row. It reports whether the table keeps the press,
// which it does but for a secondary press on a row without OnContextMenu.
func (c *tableCore) press(e *core.PointerEvent) bool {
	pos := e.Pos
	if pos.Y < tableHeaderHeight {
		if e.Button != input.ButtonPrimary {
			return true
		}
		if j := c.gripAt(pos.X); j >= 0 {
			c.drag = tableDrag{kind: tableDragResize, col: j, model: c.cols[j].model, start: pos.X, x: pos.X, width: c.cols[j].width}
			c.repaint()
			return true
		}
		j := c.colAt(pos.X)
		if j < 0 {
			return true
		}
		if c.inFilterButton(j, pos) {
			c.openFilter(j)
			return true
		}
		c.drag = tableDrag{kind: tableDragHeader, col: j, model: c.cols[j].model, start: pos.X, x: pos.X}
		c.refocus = true
		c.relayout()
		return true
	}
	row, j := c.rowAt(pos.Y), c.colAt(pos.X)
	if row < 0 {
		if c.editing {
			c.commitEdit(true)
		}
		return true
	}
	if j >= 0 && e.Button == input.ButtonPrimary && e.Modifiers == 0 && c.onCheckBox(row, j, pos) {
		c.toggleCheck(row, j)
		return true
	}
	if j < 0 {
		j = c.focusCol()
	}
	if j < 0 {
		return true
	}
	if e.Button != input.ButtonPrimary {
		return c.secondaryPress(row, j, e)
	}
	c.click(row, j, e.Modifiers, e.Clicks)
	return true
}

// click is a press on the cell of row and column j, in the order they show,
// with the keys mods held, the clicks-th in a row.
func (c *tableCore) click(row, j int, mods input.Modifiers, clicks int) {
	h := c.cols[j]
	if c.editing {
		if row == c.editRow && h.model == c.editCol {
			return
		}
		if !c.commitEdit(true) {
			return
		}
	}
	c.ctrl.row, c.ctrl.col = row, h.model
	c.choose(row, mods, true)
	c.reveal(row, j)
	c.focusNow()
	c.changed()
	if clicks == 2 {
		switch {
		case h.editable:
			c.startEdit(row, j)
		case c.cfg.onActivate != nil:
			c.cfg.onActivate(row)
		}
	}
}

// tapCell is what a screen reader's tap on a cell does: a plain click.
func (c *tableCore) tapCell(row, j int) {
	if j >= 0 && j < len(c.cols) && row >= 0 && row < c.cfg.rows {
		c.click(row, j, 0, 1)
	}
}

// choose changes the selection for row, reached with mods held. A pointer
// toggles with Ctrl; the arrows with Ctrl only move the focus.
func (c *tableCore) choose(row int, mods input.Modifiers, pointer bool) {
	mode := c.cfg.mode
	if mode == TableSelectNone || row < 0 {
		return
	}
	ctrl, shift := mods&input.ModControl != 0, mods&input.ModShift != 0
	s := c.ctrl
	switch {
	case shift && mode == TableSelectMultiple:
		if !ctrl || !pointer {
			s.ranges = s.ranges[:0]
		}
		a := min(max(s.anchor, 0), c.cfg.rows-1)
		s.add(RowRange{min(a, row), max(a, row) + 1})
	case ctrl && !pointer:
		return
	case ctrl && mode == TableSelectMultiple:
		if s.IsSelected(row) {
			s.remove(RowRange{row, row + 1})
		} else {
			s.add(RowRange{row, row + 1})
		}
		s.anchor = row
	case ctrl && s.IsSelected(row):
		s.ranges = s.ranges[:0]
		s.anchor = row
	default:
		s.ranges = append(s.ranges[:0], RowRange{row, row + 1})
		s.anchor = row
	}
	c.selectionChanged()
}

// gripAt returns the column whose right edge is within reach of x, or -1.
// An edge hidden under the frozen columns cannot be reached.
func (c *tableCore) gripAt(x float32) int {
	best, dist := -1, float32(tableGrip+1)
	for j := range c.cols {
		edge := c.colX(j) + c.cols[j].width
		if j >= c.frozen && edge < c.frozenW {
			continue
		}
		if d := geom.Abs32(x - edge); d <= tableGrip && d < dist {
			best, dist = j, d
		}
	}
	return best
}

// inFilterButton reports whether pos, from the top left of the table, is on
// the filter button of column j.
func (c *tableCore) inFilterButton(j int, pos geom.Offset) bool {
	if !c.cols[j].filterable {
		return false
	}
	right := c.colX(j) + c.cols[j].width - tableFilterInset
	top := float32(tableHeaderHeight-tableFilterButton) / 2
	return pos.X >= right-tableFilterButton && pos.X < right && pos.Y >= top && pos.Y < top+tableFilterButton
}

// dragTo follows the pointer of a press on the header.
func (c *tableCore) dragTo(x float32) {
	d := &c.drag
	switch d.kind {
	case tableDragResize:
		col := &c.cols[d.col]
		w := max(d.width+x-d.start, col.min)
		if w == col.width {
			return
		}
		c.ctrl.setWidth(col.model, w)
		col.width = w
		c.placeColumns()
		c.relayout()
	case tableDragHeader:
		d.x = x
		if geom.Abs32(x-d.start) > tableDragSlop {
			d.moved = true
		}
		c.repaint()
	}
}

// placeColumns sets the starts of the columns after a width changed.
func (c *tableCore) placeColumns() {
	var left float32
	c.frozenW = 0
	for j := range c.cols {
		c.cols[j].left = left
		left += c.cols[j].width
		if j < c.frozen {
			c.frozenW = left
		}
	}
	c.totalW = left
}

// release ends a press on the header: a resize, a move, or a click that sorts.
func (c *tableCore) release() {
	d := c.drag
	c.drag = tableDrag{}
	switch d.kind {
	case tableDragResize:
		c.changed()
		if c.cfg.onColumns != nil {
			c.cfg.onColumns()
		}
	case tableDragHeader:
		if !d.moved {
			c.sortBy(d.col)
			c.repaint()
			return
		}
		c.drag = d
		at := c.dropIndex()
		c.drag = tableDrag{}
		if at != d.col && at != d.col+1 {
			order := slices.Clone(c.order)
			m := order[d.col]
			order = slices.Delete(order, d.col, d.col+1)
			if at > d.col {
				at--
			}
			order = slices.Insert(order, at, m)
			c.ctrl.order = order
			if c.cfg.onColumns != nil {
				c.cfg.onColumns()
			}
		}
		c.changed()
	}
}

// dropIndex returns where the header being moved would land: before the
// column at the index, or after them all for their number.
func (c *tableCore) dropIndex() int {
	x := c.drag.x
	for j := range c.cols {
		if x < c.colX(j)+c.cols[j].width/2 {
			return j
		}
	}
	return len(c.cols)
}

// sortBy asks for the next order of the column at j, in the order they show.
func (c *tableCore) sortBy(j int) bool {
	if j < 0 || j >= len(c.cols) || !c.cols[j].sortable {
		return false
	}
	h := c.cols[j]
	order := SortAscending
	if c.cfg.sort.Column == h.model {
		order = c.cfg.sort.Order.next()
	}
	c.cfg.onSort(TableSort{Column: h.model, Order: order})
	return true
}

// key handles a key pressed while the table, one of its cells or its editor
// has the keyboard.
func (c *tableCore) key(e *core.KeyEvent) bool {
	if c.editing {
		return c.editKey(e)
	}
	if len(c.cols) == 0 {
		return false
	}
	if c.widgets && c.widgetKey(e) {
		return true
	}
	if kit.IsContextMenuKey(e) {
		return c.contextMenuKey()
	}
	ctrl := e.Modifiers&input.ModControl != 0
	shift := e.Modifiers&input.ModShift != 0
	alt := e.Modifiers&input.ModAlt != 0
	row, j := c.focusRow(), c.focusCol()
	last := c.cfg.rows - 1
	page := max(int(c.bodyH()/c.rowH)-1, 1)
	switch e.Key {
	case input.KeyUp:
		if alt {
			return false
		}
		row--
	case input.KeyDown:
		if alt {
			if !c.cols[j].filterable {
				return false
			}
			c.openFilter(j)
			return true
		}
		row++
	case input.KeyLeft:
		j--
	case input.KeyRight:
		j++
	case input.KeyHome:
		if ctrl {
			row = min(0, last)
		}
		j = 0
	case input.KeyEnd:
		if ctrl {
			row = last
		}
		j = len(c.cols) - 1
	case input.KeyPageUp:
		row = max(row-page, min(0, last))
	case input.KeyPageDown:
		row += page
	case input.KeySpace:
		if row >= 0 && !e.Repeat && e.Modifiers == 0 && c.cols[j].kind == CellCheck && c.cols[j].onCheck != nil {
			// Space ticks the box of the cell in focus; with Shift or Ctrl
			// it still selects.
			c.toggleCheck(row, j)
			return true
		}
		if row >= 0 && !e.Repeat {
			mods := input.ModControl
			if shift {
				mods = input.ModShift
			}
			c.choose(row, mods, true)
		}
		return true
	case input.KeyA:
		if !ctrl || alt || c.cfg.mode != TableSelectMultiple {
			return false
		}
		c.ctrl.ranges = c.ctrl.ranges[:0]
		c.ctrl.add(RowRange{0, c.cfg.rows})
		c.selectionChanged()
		return true
	case input.KeyEnter, input.KeyF2:
		if e.Modifiers != 0 || e.Repeat {
			return false
		}
		switch {
		case row < 0 && e.Key == input.KeyEnter:
			c.sortBy(j)
		case row < 0:
		case c.cols[j].editable:
			c.startEdit(row, j)
		case e.Key == input.KeyEnter && c.cfg.onActivate != nil:
			c.cfg.onActivate(row)
		}
		return true
	default:
		return false
	}
	if alt {
		return false
	}
	row = min(max(row, -1), last)
	j = min(max(j, 0), len(c.cols)-1)
	moved := row != c.focusRow()
	c.ctrl.row, c.ctrl.col = row, c.cols[j].model
	if moved && row >= 0 {
		mods := e.Modifiers
		if e.Key == input.KeyHome || e.Key == input.KeyEnd {
			// Ctrl is part of these keys: it does not hold the selection.
			mods &^= input.ModControl
		}
		c.choose(row, mods, false)
	}
	c.reveal(row, j)
	c.focusNow()
	c.changed()
	return true
}

// focusNow moves the keyboard to the object of the cell in focus at once,
// or to the table while the cell has none, so that keys pressed before the
// next layout still reach the table. The layout moves it on to the cell.
func (c *tableCore) focusNow() {
	if c.ro == nil || !c.ro.Attached() {
		return
	}
	c.refocus = true
	c.ro.syncFocus()
}
