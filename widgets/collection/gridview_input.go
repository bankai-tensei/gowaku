package collection

import (
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file holds what a GridView does with the keyboard and the pointer:
// moving the focus between the tiles and selecting them.

// selectItem changes the selection for item i, reached with mods held, as a
// Table does for a row: a pointer toggles with Ctrl, the arrows with Ctrl only
// move the focus, and Shift selects the range from the anchor in the order of
// the items. A change rebuilds the grid and calls OnSelectionChange.
func (s *gridState) selectItem(i int, mods input.Modifiers, pointer bool) {
	mode := s.Widget().(GridView).Select
	if mode == TableSelectNone || i < 0 || i >= s.count {
		return
	}
	ctrl, shift := mods&input.ModControl != 0, mods&input.ModShift != 0
	c := s.ctrl
	switch {
	case shift && mode == TableSelectMultiple:
		if !ctrl || !pointer {
			c.ranges = c.ranges[:0]
		}
		a := min(max(c.anchor, 0), s.count-1)
		c.add(RowRange{min(a, i), max(a, i) + 1})
	case ctrl && !pointer:
		return
	case ctrl && mode == TableSelectMultiple:
		if c.IsSelected(i) {
			c.remove(RowRange{i, i + 1})
		} else {
			c.add(RowRange{i, i + 1})
		}
		c.anchor = i
	case ctrl && c.IsSelected(i):
		c.ranges = c.ranges[:0]
		c.anchor = i
	default:
		if len(c.ranges) == 1 && c.ranges[0] == (RowRange{i, i + 1}) {
			c.anchor = i
			return
		}
		c.ranges = append(c.ranges[:0], RowRange{i, i + 1})
		c.anchor = i
	}
	s.selectionChanged()
}

// selectionChanged rebuilds the grid and tells the caller.
func (s *gridState) selectionChanged() {
	s.ctrl.remember(s.count, s.Widget().(GridView).ItemKey)
	kit.Rebuild(&s.StateBase)
	if f := s.Widget().(GridView).OnSelectionChange; f != nil {
		f()
	}
}

// setFocus moves the focus of the grid to item i. With claim, its tile takes
// the keyboard and scrolls into view.
func (s *gridState) setFocus(i int, claim bool) {
	s.ctrl.row = i
	if claim {
		s.claim = true
		s.revealItem(i)
	}
	kit.Rebuild(&s.StateBase)
}

// step returns the item the key k moves to from item i, or i where it stops.
// The arrows follow the tiles as they show: Left goes back unless the rows
// run right to left. Left and Right go on to the next row at its ends; Up and
// Down stay in the column, but Down from the row above a short last row stops
// on its last item.
func (s *gridState) step(i int, k input.KeyCode) int {
	n, cols := s.count, s.geo.cols
	back := -1
	if s.geo.rtl {
		back = 1
	}
	switch k {
	case input.KeyLeft:
		i += back
	case input.KeyRight:
		i -= back
	case input.KeyUp:
		if i-cols >= 0 {
			i -= cols
		}
	case input.KeyDown:
		switch {
		case i+cols < n:
			i += cols
		case i/cols < (n-1)/cols:
			i = n - 1
		}
	case input.KeyHome:
		i = 0
	case input.KeyEnd:
		i = n - 1
	case input.KeyPageUp, input.KeyPageDown:
		page := 1
		if s.geo.stride > 0 {
			page = max(int(s.view/s.geo.stride)-1, 1)
		}
		if k == input.KeyPageUp {
			i = max(i-page*cols, i%cols)
		} else {
			last := (n - 1) / cols
			r := min(i/cols+page, last)
			i = min(r*cols+i%cols, n-1)
		}
	}
	return min(max(i, 0), n-1)
}

// key handles a key pressed while the tile of the focus has the keyboard.
func (s *gridState) key(e *core.KeyEvent) bool {
	i := s.focusIndex()
	if i < 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	w := s.Widget().(GridView)
	switch e.Key {
	case input.KeyLeft, input.KeyRight, input.KeyUp, input.KeyDown,
		input.KeyHome, input.KeyEnd, input.KeyPageUp, input.KeyPageDown:
		j := s.step(i, e.Key)
		if j != i {
			mods := e.Modifiers
			if e.Key == input.KeyHome || e.Key == input.KeyEnd {
				// Ctrl+Home and Ctrl+End are the same keys; Ctrl does not hold
				// the selection there.
				mods &^= input.ModControl
			}
			s.selectItem(j, mods, false)
		}
		s.setFocus(j, true)
	case input.KeySpace:
		if !e.Repeat {
			mods := input.ModControl
			if e.Modifiers&input.ModShift != 0 {
				mods = input.ModShift
			}
			s.selectItem(i, mods, true)
		}
	case input.KeyA:
		if e.Modifiers&input.ModControl == 0 || w.Select != TableSelectMultiple {
			return false
		}
		s.ctrl.ranges = s.ctrl.ranges[:0]
		s.ctrl.add(RowRange{0, s.count})
		s.selectionChanged()
	case input.KeyEnter:
		if e.Modifiers != 0 || e.Repeat {
			return false
		}
		if w.OnActivate != nil {
			w.OnActivate(i)
		}
	default:
		return false
	}
	return true
}

// cellPress handles a press on the tile of item i.
func (s *gridState) cellPress(i int, e *core.PointerEvent) {
	if i < 0 || i >= s.count {
		return
	}
	s.selectItem(i, e.Modifiers, true)
	s.setFocus(i, false)
	if e.Clicks == 2 && e.Modifiers&(input.ModControl|input.ModShift) == 0 {
		if f := s.Widget().(GridView).OnActivate; f != nil {
			f(i)
		}
	}
}

// cellSecondaryPress handles a press of the secondary button on the tile of
// item i: it selects the item alone unless it is selected already, and moves
// the focus there. It reports whether the grid has OnContextMenu.
func (s *gridState) cellSecondaryPress(i int) bool {
	if i < 0 || i >= s.count {
		return false
	}
	if !s.ctrl.IsSelected(i) {
		s.selectItem(i, 0, true)
	}
	s.setFocus(i, false)
	return s.Widget().(GridView).OnContextMenu != nil
}

// cellContextMenu asks for the context menu of item i at the place at in the
// window, and reports whether the grid has OnContextMenu.
func (s *gridState) cellContextMenu(i int, at geom.Offset) bool {
	f := s.Widget().(GridView).OnContextMenu
	if f == nil || i < 0 || i >= s.count {
		return false
	}
	f(i, toLocal(s.Context(), at))
	return true
}

// toLocal turns a place in the window into one from the top left of the
// widget of ctx, as OnContextMenu reports it.
func toLocal(ctx core.BuildContext, at geom.Offset) geom.Offset {
	if ro := ctx.RenderObject(); ro != nil {
		if p, ok := core.GlobalToLocal(ro, at); ok {
			return p
		}
	}
	return at
}

// emptyPress handles a press between the tiles, which clears the selection.
func (s *gridState) emptyPress() {
	if len(s.ctrl.ranges) > 0 {
		s.ctrl.ranges = s.ctrl.ranges[:0]
		s.selectionChanged()
	}
}

// cellFocused follows the tile of item i taking the keyboard, by Tab, a press
// or a claim.
func (s *gridState) cellFocused(i int) {
	if i < 0 || i >= s.count {
		return
	}
	if s.ctrl.row != i {
		s.ctrl.row = i
		kit.Rebuild(&s.StateBase)
	}
	s.revealItem(i)
}

// cellAction does what assistive technology asks of the tile of item i.
func (s *gridState) cellAction(i int, a semantics.Actions) bool {
	if i < 0 || i >= s.count {
		return false
	}
	switch a {
	case semantics.ActionTap:
		s.selectItem(i, 0, true)
		s.setFocus(i, false)
	case semantics.ActionScrollIntoView:
		s.revealItem(i)
	default:
		return false
	}
	return true
}
