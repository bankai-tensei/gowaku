package collection

import "github.com/bankai-tensei/gowaku/core"

// keyedRow is a row that a TableController picks out, at the last time the
// controller remembered it: its index then, and its key.
type keyedRow struct {
	row int
	key core.Key
}

// rowKeys lets the selection, the focus and the anchor of a TableController
// follow their rows by key when the rows change order or lose some, for a
// Table with RowKey or a GridView with ItemKey. See
// docs/ADR/0127-sort-semantics-row-keys-sidebar-count-icons.md.
type rowKeys struct {
	// valid means the keys below describe the selection. A change made from
	// code drops them: its indexes name the rows the view builds with next.
	valid bool
	// rows is the number of rows when the keys were taken.
	rows int
	// all means every row was selected, which stays so without a key each.
	all bool
	// sel holds the selected rows, in order.
	sel                 []keyedRow
	focus, anchor       keyedRow
	hasFocus, hasAnchor bool
	// want is the keys of sel, reused by follow.
	want map[core.Key]struct{}
}

// dropKeys forgets the keys, after a change from code.
func (c *TableController) dropKeys() { c.keys.valid = false }

// remember takes the keys of the selected rows, the row in focus and the
// anchor among n rows, which key names. A nil key forgets them. It costs one
// call of key for each selected row, unless every row is selected.
func (c *TableController) remember(n int, key func(i int) core.Key) {
	k := &c.keys
	clear(k.sel)
	k.sel = k.sel[:0]
	k.focus, k.anchor = keyedRow{}, keyedRow{}
	k.valid, k.rows = key != nil, n
	if !k.valid {
		return
	}
	k.all = n > 0 && len(c.ranges) == 1 && c.ranges[0] == RowRange{0, n}
	if !k.all {
		for _, r := range c.ranges {
			for i := r.Start; i < min(r.End, n); i++ {
				k.sel = append(k.sel, keyedRow{i, key(i)})
			}
		}
	}
	k.hasFocus = c.row >= 0 && c.row < n
	if k.hasFocus {
		k.focus = keyedRow{c.row, key(c.row)}
	}
	k.hasAnchor = c.anchor >= 0 && c.anchor < n
	if k.hasAnchor {
		k.anchor = keyedRow{c.anchor, key(c.anchor)}
	}
}

// follow finds the remembered rows among the n rows that key now names, and
// moves the selection, the focus and the anchor to them. Rows that are gone
// leave the selection; a focus whose row is gone stays at its index, on the
// row that took its place. When every remembered row is where it was, as on
// most builds, it costs a call of key for each of them; otherwise one for
// each of the n rows.
func (c *TableController) follow(n int, key func(i int) core.Key) {
	k := &c.keys
	if !k.valid || key == nil || k.still(n, key) {
		return
	}
	if k.want == nil {
		k.want = make(map[core.Key]struct{}, len(k.sel))
	}
	clear(k.want)
	for _, r := range k.sel {
		k.want[r.key] = struct{}{}
	}
	c.ranges = c.ranges[:0]
	if k.all && n > 0 {
		c.ranges = append(c.ranges, RowRange{0, n})
	}
	// The keys are unique, so the scan stops once it has found them all.
	focus, anchor, left := -1, -1, len(k.want)
	for i := range n {
		if left == 0 && (!k.hasFocus || focus >= 0) && (!k.hasAnchor || anchor >= 0) {
			break
		}
		id := key(i)
		if _, ok := k.want[id]; ok {
			c.appendRow(i)
			left--
		}
		if k.hasFocus && focus < 0 && id == k.focus.key {
			focus = i
		}
		if k.hasAnchor && anchor < 0 && id == k.anchor.key {
			anchor = i
		}
	}
	if focus >= 0 {
		c.row = focus
	}
	switch {
	case anchor >= 0:
		c.anchor = anchor
	case k.hasAnchor:
		c.anchor = c.row
	}
}

// still reports whether each remembered row is still at its index among the n
// rows that key names.
func (k *rowKeys) still(n int, key func(i int) core.Key) bool {
	if n != k.rows {
		return false
	}
	for _, r := range k.sel {
		if key(r.row) != r.key {
			return false
		}
	}
	return (!k.hasFocus || key(k.focus.row) == k.focus.key) && (!k.hasAnchor || key(k.anchor.row) == k.anchor.key)
}

// appendRow adds row to the selection, after every row already in it.
func (c *TableController) appendRow(row int) {
	if n := len(c.ranges); n > 0 && c.ranges[n-1].End == row {
		c.ranges[n-1].End++
		return
	}
	c.ranges = append(c.ranges, RowRange{row, row + 1})
}
