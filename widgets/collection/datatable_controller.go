package collection

import "slices"

// RowRange is the rows of a Table from Start up to End, which it does not
// include.
type RowRange struct {
	Start, End int
}

// Len returns the number of rows of r.
func (r RowRange) Len() int { return max(r.End-r.Start, 0) }

// TableController holds what a Table lets the user change about how it
// shows the rows: which rows are selected, the cell with the keyboard focus,
// and the width and order of the columns. The zero value is ready to use: no
// selection, the focus on the first cell, and the columns as the table lists
// them.
//
// A table without a Controller keeps one of its own. Give it one to read or
// set the selection from outside, or to keep the widths and order of the
// columns across rebuilds that replace the table, as a settings file would.
//
// The selection holds indexes into Table.Rows (GridView items). With
// Table.RowKey (GridView.ItemKey) it follows the rows: when the table builds
// with rows in another order, or with rows taken out, the selected rows, the
// cell in focus and the start of the Shift range move to where the same keys
// now are, and rows that are gone leave the selection. Without a key the
// indexes keep pointing at the same places, not at the same rows: clear the
// selection, or set it again, when the rows change. The methods that change
// the selection or the focus take indexes into the rows the table builds with
// next. The selection is a set of ranges, so that selecting all of a million
// rows costs one range.
//
// A controller belongs to one table at a time. Its methods run on the UI
// goroutine, and those that change it repaint the table.
type TableController struct {
	// ranges is the selection, sorted, apart and never touching.
	ranges []RowRange
	// anchor is where the range of a Shift click or Shift arrow starts.
	anchor int
	// row and col are the cell in focus; row -1 is the header. col counts
	// the columns of Table.Columns, whatever their order on screen.
	row, col int
	// widths holds the width the user gave each column, or zero.
	widths []float32
	// order lists the columns of Table.Columns in the order they show.
	order []int
	// rows is the number of rows at the last build of the table.
	rows int
	// view is the table that shows the controller.
	view tableListener
	// keys follow the selected rows, for a table with keys.
	keys rowKeys
}

// tableListener is what a controller tells about its changes.
type tableListener interface {
	controllerChanged()
}

func (c *TableController) changed() {
	if c.view != nil {
		c.view.controllerChanged()
	}
}

// IsSelected reports whether row is selected.
func (c *TableController) IsSelected(row int) bool {
	i, found := c.search(row)
	return found || i > 0 && c.ranges[i-1].End > row
}

// search returns the index of the first range that starts at or after row, and
// whether it starts at row.
func (c *TableController) search(row int) (int, bool) {
	return slices.BinarySearchFunc(c.ranges, row, func(r RowRange, row int) int { return r.Start - row })
}

// Selection returns the selected rows as sorted ranges that neither overlap
// nor touch. The slice belongs to the controller: read it, do not change it,
// and do not keep it past the next change.
func (c *TableController) Selection() []RowRange { return c.ranges }

// SelectedCount returns the number of selected rows.
func (c *TableController) SelectedCount() int {
	n := 0
	for _, r := range c.ranges {
		n += r.Len()
	}
	return n
}

// SelectedRows calls fn for each selected row, in order, until fn returns
// false.
func (c *TableController) SelectedRows(fn func(row int) bool) {
	for _, r := range c.ranges {
		for i := r.Start; i < r.End; i++ {
			if !fn(i) {
				return
			}
		}
	}
}

// Select selects row alone, and starts the range of the next Shift click there.
func (c *TableController) Select(row int) {
	c.ranges = append(c.ranges[:0], RowRange{row, row + 1})
	c.anchor = row
	c.dropKeys()
	c.changed()
}

// SetSelection selects the rows of r and no others.
func (c *TableController) SetSelection(r ...RowRange) {
	c.ranges = c.ranges[:0]
	for _, x := range r {
		c.add(x)
	}
	c.dropKeys()
	c.changed()
}

// AddRange adds the rows of r to the selection.
func (c *TableController) AddRange(r RowRange) {
	c.add(r)
	c.dropKeys()
	c.changed()
}

// RemoveRange takes the rows of r out of the selection.
func (c *TableController) RemoveRange(r RowRange) {
	c.remove(r)
	c.dropKeys()
	c.changed()
}

// Toggle selects row if it is not, and the other way around. It moves the
// start of the next Shift range to row.
func (c *TableController) Toggle(row int) {
	if c.IsSelected(row) {
		c.remove(RowRange{row, row + 1})
	} else {
		c.add(RowRange{row, row + 1})
	}
	c.anchor = row
	c.dropKeys()
	c.changed()
}

// SelectAll selects every row the table showed at its last build.
func (c *TableController) SelectAll() {
	c.ranges = c.ranges[:0]
	c.add(RowRange{0, c.rows})
	c.dropKeys()
	c.changed()
}

// ClearSelection selects nothing.
func (c *TableController) ClearSelection() {
	c.ranges = c.ranges[:0]
	c.dropKeys()
	c.changed()
}

// FocusedCell returns the cell that has, or will have, the keyboard focus. Row
// -1 is the header; column is an index into Table.Columns.
func (c *TableController) FocusedCell() (row, column int) { return c.row, c.col }

// SetFocusedCell moves the focus to a cell. The table brings it into view at
// its next layout only when the user moves it; call it before the table
// appears, or scroll by moving it with the keyboard.
func (c *TableController) SetFocusedCell(row, column int) {
	c.row, c.col = row, column
	c.dropKeys()
	c.changed()
}

// ColumnWidth returns the width the user gave a column of Table.Columns
// by dragging its edge, or zero if they did not.
func (c *TableController) ColumnWidth(column int) float32 {
	if column < 0 || column >= len(c.widths) {
		return 0
	}
	return c.widths[column]
}

// SetColumnWidth sets the width of a column of Table.Columns. Zero goes
// back to TableColumn.Width.
func (c *TableController) SetColumnWidth(column int, width float32) {
	c.setWidth(column, width)
	c.changed()
}

// setWidth is SetColumnWidth without the rebuild, for a drag that moves the
// columns itself.
func (c *TableController) setWidth(column int, width float32) {
	if column < 0 {
		return
	}
	for len(c.widths) <= column {
		c.widths = append(c.widths, 0)
	}
	c.widths[column] = max(width, 0)
}

// ColumnOrder returns the indexes of Table.Columns in the order they
// show, or nil if they show in their own order. The slice belongs to the
// controller.
func (c *TableController) ColumnOrder() []int { return c.order }

// SetColumnOrder shows the columns of Table.Columns in the order of the
// indexes. Nil, or a list that is not an ordering of the columns, shows them in
// their own order.
func (c *TableController) SetColumnOrder(order []int) {
	c.order = append(c.order[:0], order...)
	if len(order) == 0 {
		c.order = nil
	}
	c.changed()
}

// add puts r into the selection, joining the ranges it touches.
func (c *TableController) add(r RowRange) {
	if r.Len() == 0 {
		return
	}
	// lo is the first range that ends at or after r starts; hi the first that
	// starts after r ends. Both touch r when they meet it.
	lo := 0
	for lo < len(c.ranges) && c.ranges[lo].End < r.Start {
		lo++
	}
	hi := lo
	for hi < len(c.ranges) && c.ranges[hi].Start <= r.End {
		r.Start = min(r.Start, c.ranges[hi].Start)
		r.End = max(r.End, c.ranges[hi].End)
		hi++
	}
	c.ranges = slices.Replace(c.ranges, lo, hi, r)
}

// remove takes r out of the selection, splitting a range that holds it.
func (c *TableController) remove(r RowRange) {
	if r.Len() == 0 {
		return
	}
	var out []RowRange
	for _, x := range c.ranges {
		if x.End <= r.Start || x.Start >= r.End {
			out = append(out, x)
			continue
		}
		if x.Start < r.Start {
			out = append(out, RowRange{x.Start, r.Start})
		}
		if x.End > r.End {
			out = append(out, RowRange{r.End, x.End})
		}
	}
	c.ranges = append(c.ranges[:0], out...)
}

// clip drops the selection past the last of rows, after the rows shrank.
func (c *TableController) clip(rows int) {
	c.rows = rows
	for len(c.ranges) > 0 && c.ranges[len(c.ranges)-1].Start >= rows {
		c.ranges = c.ranges[:len(c.ranges)-1]
	}
	if n := len(c.ranges); n > 0 && c.ranges[n-1].End > rows {
		c.ranges[n-1].End = rows
	}
}

// displayOrder writes into dst the indexes of n columns in the order they
// show: the stored order if it is one of the n columns, else their own.
func (c *TableController) displayOrder(dst []int, n int) []int {
	dst = dst[:0]
	if len(c.order) == n && isOrdering(c.order) {
		return append(dst, c.order...)
	}
	for i := range n {
		dst = append(dst, i)
	}
	return dst
}

// isOrdering reports whether order holds each of 0 to len(order)-1 once.
func isOrdering(order []int) bool {
	for i, m := range order {
		if m < 0 || m >= len(order) || slices.Index(order[:i], m) >= 0 {
			return false
		}
	}
	return true
}
