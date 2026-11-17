package collection

import (
	"math"

	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// ListController moves a List from code, to a row or to the end, and reads
// where it is. It
// works for rows of any height, and for rows that measure themselves it is
// what reaches a row that was never laid out, at a cost that does not grow
// with the number of rows. The zero value is ready; keep it where it outlives
// a build, such as in a State, and put it in List.Controller.
//
// A jump asked for while no list shows the controller waits for the next list
// that does, so that a list can open at its end:
//
//	func (s *chatState) InitState() { s.list.JumpToEnd() }
//
// A jump lands at once, on the rows the list has, and again at the next
// layout, on the rows of a rebuild that has not run yet: JumpToEnd right after
// the SetState that appended a row reaches that row. An index counts in the
// rows of the time the jump lands.
//
// A controller belongs to one list at a time. Its methods run on the UI
// goroutine.
type ListController struct {
	// view is the list that shows the controller, or nil.
	view *listState
	// pending is a jump to row index at align that waits for a list.
	pending bool
	index   int
	align   float32
}

// Position returns where the view is, or the zero position without a list.
// The offset counts from the top of the first row; with List.MeasureItems the
// rows not laid out yet count as their estimate.
func (c *ListController) Position() basic.ScrollPosition {
	if c.view == nil {
		return basic.ScrollPosition{}
	}
	return c.view.Position()
}

// Offset returns how far the view is from the top of the first row, or 0
// without a list.
func (c *ListController) Offset() float64 { return c.Position().Offset }

// MaxOffset returns the largest offset as of the last layout: how much taller
// the rows are than the view. It is 0 without a list and for rows that fit.
func (c *ListController) MaxOffset() float64 { return c.Position().MaxOffset }

// AtEnd reports whether a list shows the controller and its view is at the
// bottom, as basic.ScrollPosition.AtEnd tells.
func (c *ListController) AtEnd() bool { return c.view != nil && c.view.Position().AtEnd() }

// JumpToIndex moves the list at once so that row i is at align in the view:
// 0 puts the top of the row at the top of the view, 1 its bottom at the bottom,
// and 0.5 its middle in the middle. The view stays within the rows, so the
// last rows cannot come to the top. An index past either end goes to that end.
//
// With List.MeasureItems, the list first goes to the row by the estimated
// heights, then lays out the row and its neighbors and corrects the offset,
// keeping the row at align until the user scrolls. Only the rows near the
// view are laid out.
func (c *ListController) JumpToIndex(i int, align float32) {
	if c.view != nil {
		c.view.jumpTo(i, align)
		return
	}
	c.pending, c.index, c.align = true, i, align
}

// JumpToEnd moves the list at once to its end, with the bottom of the last row
// at the bottom of the view. The last row is the last of the time it lands, as
// the type tells.
func (c *ListController) JumpToEnd() { c.JumpToIndex(math.MaxInt, 1) }

// Remeasure tells a list with List.MeasureItems that the content of row i
// changed while it may be out of view, so that the list lays it out again when
// it comes near. Until then it counts as the estimate. A row in view needs no
// call: it lays itself out again when it changes. Rows in view do not move.
func (c *ListController) Remeasure(i int) {
	s := c.view
	if s == nil {
		return
	}
	if m := s.measure(); m != nil && i >= 0 && i < m.Len() {
		m.Forget(i)
		if s.ro != nil {
			s.ro.MarkNeedsLayout()
		}
	}
}

// RemeasureAll is Remeasure for every row, as when all the rows changed in
// place. It costs O(n).
func (c *ListController) RemeasureAll() {
	s := c.view
	if s == nil {
		return
	}
	if m := s.measure(); m != nil {
		m.ForgetAll()
		if s.ro != nil {
			s.ro.MarkNeedsLayout()
		}
	}
}
