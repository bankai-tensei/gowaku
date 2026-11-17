package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

const (
	// defaultItemExtent is the height of a row of a List without an ItemExtent.
	defaultItemExtent = 48
	// cachedItems is how many rows exist past each edge of the view, so that a
	// fast wheel does not show gaps before the next build.
	cachedItems = 3
)

// List shows ItemCount rows, of which only those near the view are built.
// It suits any number of rows. Like Scroll, it takes all the room it is given
// and panics where that is unbounded.
//
// The rows have the same height, ItemExtent, unless ItemExtentOf is set. That
// lets a table header, section titles and ordinary rows share one list. A row
// of text is as tall as its padding plus Style.LinePixels for each line.
//
// When the heights are not known in advance, as for rows of wrapping text, set
// MeasureItems: each row is then as tall as it lays itself out at the width of
// the list.
//
// StickToEnd suits a log or a chat whose newest row is the last: while the
// view is at the end, rows added at the end, and a last row that grows, keep
// it at the end; once the user scrolls away, the view stays where it is, and
// coming back to the end sticks again. A list with StickToEnd opens at its
// end. OnScroll, when not nil, is told the position after it changed: on each
// frame of a motion, after a jump, after a layout that changed the length of
// the rows or of the view, and after the first layout. It may call SetState,
// as to show a button that jumps to the newest row while the view is away from
// the end (basic.ScrollPosition.AtEnd). ListController reads the same
// position. See docs/ADR/0139-scroll-position-stick-to-end-span-background.md.
//
// A header inside the list scrolls away with the rows. To keep one in place,
// put it above the list instead, with the list in an Expanded:
//
//	Column{Children: []core.Widget{header, Expanded{Child: List{...}}}}
type List struct {
	core.StatefulBase
	ItemCount int
	// ItemExtent is the height of a row. Zero selects 48.
	ItemExtent float32
	// ItemExtentOf, when not nil, returns the height of row i in place of
	// ItemExtent. A height of zero hides the row; a negative one panics.
	//
	// It is called for every row, once when the list first appears and again
	// each time a parent rebuild gives the list a new List value, so it
	// must be cheap and should not build widgets. Scrolling does not call it.
	// The heights are kept in a table of 4 bytes per row; the row at an
	// offset is found by binary search. See docs/ADR/0034-varying-list-extents.md.
	ItemExtentOf func(i int) float32
	// MeasureItems, when set and ItemExtentOf is nil, lets each row be as tall
	// as it lays itself out at the width of the list, with an unbounded
	// height. Only the rows near the view are ever laid out: the others count
	// as EstimatedItemExtent until they come near, and the list keeps the
	// height of each row it has laid out, 16 bytes per row.
	//
	// When a row takes a height other than its estimate, the rows in view do
	// not move: the scroll offset does. The heights stay with the indexes
	// when ItemCount changes and when a parent rebuild gives the list a new
	// value; a row whose content changes while it is out of view keeps its old
	// height until ListController.Remeasure. A new width forgets them all.
	// See docs/ADR/0078-measured-list-extents.md.
	MeasureItems bool
	// EstimatedItemExtent is the height of a row not laid out yet, with
	// MeasureItems. Zero selects the mean height of the rows laid out so far,
	// and ItemExtent until there are any. A good estimate keeps the scroll bar
	// still as rows are measured.
	EstimatedItemExtent float32
	StickToEnd          bool
	// Controller, when not nil, lets code move the list to a row and read
	// where it is. A list without one needs none.
	Controller *ListController
	OnScroll   func(basic.ScrollPosition)
	// ItemBuilder returns the widget of row i.
	ItemBuilder func(i int) core.Widget
}

func (l List) extent() float32 {
	if l.ItemExtent > 0 {
		return l.ItemExtent
	}
	return defaultItemExtent
}

// measures reports whether the rows take the heights they lay themselves out at.
func (l List) measures() bool { return l.MeasureItems && l.ItemExtentOf == nil }

type listState struct {
	basic.ScrollState
	view        float32
	first, last int
	// extents holds the heights from ItemExtentOf, read again when stale.
	extents layout.ExtentTable
	// measured holds the heights the rows laid themselves out at, with
	// MeasureItems; when stale, its length and estimate follow the widget.
	heights layout.MeasuredExtents
	stale   bool
	// controller is the controller that shows this list, and ro the render
	// object of the rows.
	controller *ListController
	ro         *layout.RenderList
	// jump is a jump to row jumpIndex at jumpAlign, the index counted in the
	// rows of the time it lands: waiting for the size of the view and for the
	// next layout, which may bring the rows of a rebuild that had not run when
	// it was asked for, or held by the render object while measured rows
	// settle.
	jump      bool
	jumpIndex int
	jumpAlign float32
}

// CreateState implements core.StatefulWidget.
func (List) CreateState() core.State { return &listState{stale: true} }

// InitState implements core.State.
func (s *listState) InitState() {
	s.ScrollState.InitState()
	s.OnMove = s.refresh
	s.OnUser = s.stopJump
	s.follow()
}

// DidUpdateWidget implements core.State. The heights are read again because a
// new ItemExtentOf, or the same one over new data, may give others, and
// functions cannot be compared. A jump waiting for the next layout, and a view
// stuck to the end, move to the new rows before they are built.
func (s *listState) DidUpdateWidget(core.Widget) {
	s.stale = true
	s.follow()
	if s.jump {
		s.placeJump()
	} else if s.StickToEnd && s.StuckToEnd() && s.view > 0 {
		s.Scroller.Jump(max(s.content()-float64(s.view), 0))
	}
}

// follow takes the settings of the widget.
func (s *listState) follow() {
	w := s.Widget().(List)
	s.StickToEnd, s.OnScroll = w.StickToEnd, w.OnScroll
	s.attach(w.Controller)
}

// Dispose implements core.State.
func (s *listState) Dispose() {
	s.attach(nil)
	s.ScrollState.Dispose()
}

// attach makes c the controller of the list, and takes a jump it holds.
func (s *listState) attach(c *ListController) {
	if s.controller == c {
		return
	}
	if s.controller != nil && s.controller.view == s {
		s.controller.view = nil
	}
	s.controller = c
	if c == nil {
		return
	}
	if c.view != nil && c.view != s {
		c.view.controller = nil
	}
	c.view = s
	if c.pending {
		c.pending = false
		s.jumpTo(c.index, c.align)
	}
}

// table returns the heights of the rows, or nil for rows of the same height.
func (s *listState) table() *layout.ExtentTable {
	w := s.Widget().(List)
	if w.ItemExtentOf == nil {
		return nil
	}
	if s.stale {
		s.extents.Reset(max(w.ItemCount, 0), w.ItemExtentOf)
		s.stale = false
	}
	return &s.extents
}

// measure returns the measured heights of the rows, or nil unless the rows
// measure themselves.
func (s *listState) measure() *layout.MeasuredExtents {
	w := s.Widget().(List)
	if !w.measures() {
		return nil
	}
	if s.stale {
		s.stale = false
		m := &s.heights
		n, total, est := m.Len(), m.Total(), m.Estimate()
		m.Resize(w.ItemCount)
		if w.EstimatedItemExtent > 0 {
			m.SetEstimate(w.EstimatedItemExtent, false)
		} else {
			m.SetEstimate(w.extent(), true)
		}
		if s.ro != nil && (m.Len() != n || m.Total() != total || m.Estimate() != est) {
			s.ro.MarkNeedsLayout()
		}
	}
	return &s.heights
}

// content returns the height of all the rows.
func (s *listState) content() float64 {
	if t := s.table(); t != nil {
		return t.Total()
	}
	if m := s.measure(); m != nil {
		return m.Total()
	}
	w := s.Widget().(List)
	return float64(w.ItemCount) * float64(w.extent())
}

// visible returns the rows to build at the current offset.
func (s *listState) visible() (first, last int) {
	w := s.Widget().(List)
	top, view := s.Offset.Value(), float64(s.view)
	if t := s.table(); t != nil {
		first = t.IndexAt(top) - cachedItems
		last = t.IndexAt(top+view) + 1 + cachedItems
	} else if m := s.measure(); m != nil {
		first = m.IndexAt(top) - cachedItems
		last = m.IndexAt(top+view) + 1 + cachedItems
	} else {
		extent := float64(w.extent())
		first = int(top/extent) - cachedItems
		last = int((top+view)/extent) + 1 + cachedItems
	}
	last = max(min(last, w.ItemCount), 0)
	return min(max(first, 0), last), last
}

// rowAt returns the top and the height of row i, measured or estimated.
func (s *listState) rowAt(i int) (top float64, height float32) {
	if t := s.table(); t != nil {
		return t.Offset(i), t.Extent(i)
	}
	if m := s.measure(); m != nil {
		return m.Offset(i), m.Extent(i)
	}
	extent := s.Widget().(List).extent()
	return float64(i) * float64(extent), extent
}

// jumpTo moves the view at once so that row i is at align in it. Measured rows
// go to their estimated place first, and the render object keeps row i there
// while the rows around it take their heights.
func (s *listState) jumpTo(i int, align float32) {
	s.jump, s.jumpIndex, s.jumpAlign = true, max(i, 0), geom.Clamp32(align, 0, 1)
	s.applyJump()
	if s.ro != nil {
		s.ro.MarkNeedsLayout()
	}
}

// applyJump moves the view to the jump and tells those who follow the offset.
func (s *listState) applyJump() {
	if s.placeJump() {
		s.Moved()
	}
}

// placeJump moves the view to the jump, once the view has a size or the rows
// measure themselves, and reports whether it did. The jump stays.
func (s *listState) placeJump() bool {
	n := s.Widget().(List).ItemCount
	m := s.measure()
	if !s.jump || n <= 0 || (s.view == 0 && m == nil) {
		return false
	}
	i := min(s.jumpIndex, n-1)
	top, height := s.rowAt(i)
	off := min(max(top-float64(s.jumpAlign*(s.view-height)), 0), max(s.content()-float64(s.view), 0))
	s.Scroller.Jump(off)
	s.Ticker.Stop()
	if m != nil && s.ro != nil {
		s.ro.Reveal(i, s.jumpAlign)
	}
	return true
}

// stopJump ends a jump when the user scrolls.
func (s *listState) stopJump() {
	s.jump = false
	if s.ro != nil {
		s.ro.StopRevealing()
	}
}

// refresh rebuilds only when other rows come into range.
func (s *listState) refresh() {
	if first, last := s.visible(); first != s.first || last != s.last {
		s.SetState(func() {})
	}
}

func (s *listState) measured(view float32) {
	resized := view != s.view
	s.view = view
	measures := s.measure() != nil
	if s.jump && (!measures || s.Widget().(List).ItemCount <= 0) {
		// A jump of rows of one height lands once more, on the rows of this
		// layout, and is done.
		s.applyJump()
		s.jump = false
	}
	s.Measure(float64(view), s.content())
	if resized || measures {
		// Rows that took their heights may leave the view short of rows.
		s.refresh()
	}
}

// shifted follows a move of the offset by the render object, which keeps the
// rows in view still while rows take their measured heights.
func (s *listState) shifted(delta float64) {
	s.Scroller.Shift(delta)
}

// Build implements core.State.
func (s *listState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(List)
	s.first, s.last = s.visible()
	rows := make([]core.Widget, 0, max(s.last-s.first, 0))
	for i := s.first; i < s.last; i++ {
		rows = append(rows, core.Keyed(i, basic.RepaintBoundary{Child: w.ItemBuilder(i)}))
	}
	view := list{state: s, extent: w.extent(), extents: s.table(), measured: s.measure(), first: s.first, rows: rows}
	return basic.ScrollFrame{Axis: layout.Vertical, State: &s.ScrollState, View: view}
}

type list struct {
	state    *listState
	extent   float32
	extents  *layout.ExtentTable
	measured *layout.MeasuredExtents
	first    int
	rows     []core.Widget
}

// Key implements core.Widget.
func (list) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l list) CreateRenderObject() core.RenderObject {
	r := &layout.RenderList{
		Position:   &l.state.Offset,
		Extent:     l.extent,
		Measured:   l.measured,
		First:      l.first,
		OnScroll:   l.state.Wheel,
		OnMeasured: l.state.measured,
		OnShift:    l.state.shifted,
	}
	r.SetExtents(l.extents)
	r.Attach()
	l.state.ro = r
	if n := l.state.Widget().(List).ItemCount; l.state.jump && l.measured != nil && n > 0 {
		r.Reveal(min(l.state.jumpIndex, n-1), l.state.jumpAlign)
	}
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (l list) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderList)
	if r.Extent != l.extent || r.First != l.first || r.Measured != l.measured {
		r.Extent, r.First, r.Measured = l.extent, l.first, l.measured
		r.MarkNeedsLayout()
	}
	r.SetExtents(l.extents)
}

// ChildWidgets implements core.MultiChildWidget.
func (l list) ChildWidgets() []core.Widget { return l.rows }
