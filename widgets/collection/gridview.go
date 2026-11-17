package collection

import (
	"math"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// gridMinColumnWidth is the narrowest column of a GridView that sets
	// neither Columns nor MinColumnWidth: a thumbnail.
	gridMinColumnWidth = 120
	// gridCellInset keeps the content of a cell off its edges, so that the
	// ground of a selected cell shows around it.
	gridCellInset = theme.Space4
	gridRing      = 2
)

// GridView shows ItemCount items as tiles in rows, of which only the rows near
// the view are built: thumbnails, or the icons of a file view. It suits any
// number of items. Like List, it takes all the room it is given and panics
// where that is unbounded.
//
// The columns share the width, and their number follows it as Grid does:
// Columns fixes it, and otherwise as many columns fit as MinColumnWidth
// allows, at least one; zero for both selects columns of at least 120. Every
// tile has the same height, TileHeight, or else the column width over
// AspectRatio plus ExtraHeight, square without either; the same height is
// what lets the grid find the rows in view of a hundred thousand items at
// once. Spacing parts the columns, RunSpacing the rows, and Padding keeps the
// tiles off the edges. When the width changes the number of columns, the item
// at the top of the view stays at the top.
//
// The items fill the rows from the top left, or from the top right when
// Direction is text.DirectionRTL.
//
// Items are selected as rows of a Table are: a click selects one, Ctrl+click
// adds or removes one, Shift+click selects a range in the order of the items,
// Ctrl+A all, Space selects or clears the item in focus, and Select limits how
// many. The selection and the item in focus live in a TableController, whose
// rows are the items here and whose column is unused; the grid keeps one
// itself when Controller is nil. With ItemKey the selection follows the items
// when they move.
//
// The grid is one stop of Tab. Inside, the arrows move the focus between the
// tiles as they show, Left and Right on to the next row at its ends, Home and
// End to the first and the last item, Page Up and Page Down by a view of rows.
// Shift extends the selection, Ctrl moves the focus alone, and Enter or a
// double click calls OnActivate.
//
// A screen reader sees a grid named by Label with the number of all its rows
// and columns, and a row of cells for each row in view, each with its place.
// See docs/ADR/0090-gridview.md.
type GridView struct {
	core.StatefulBase
	ItemCount int
	// Columns fixes the number of columns. Zero fits as many as
	// MinColumnWidth allows.
	Columns int
	// MinColumnWidth is the narrowest a column may be when Columns is zero.
	// Zero selects 120.
	MinColumnWidth float32
	// Spacing is the gap between columns, and RunSpacing the gap between rows.
	Spacing    float32
	RunSpacing float32
	// TileHeight fixes the height of every tile. Zero gives the column width
	// over AspectRatio, plus ExtraHeight; AspectRatio zero is a square.
	TileHeight  float32
	AspectRatio float32
	ExtraHeight float32
	// Padding is the room between the tiles and the edges of the view. It
	// scrolls with the tiles.
	Padding geom.Insets
	// Direction text.DirectionRTL fills the rows from the right. Auto and LTR
	// fill them from the left.
	Direction text.Direction
	// Label names the grid to a screen reader.
	Label string
	// LabelOf, when not nil, names item i to a screen reader. Nil names an item
	// by the text it shows, if any.
	LabelOf func(i int) string
	Select  TableSelectMode
	// Controller holds the selection and the item in focus. Nil keeps them
	// in the grid.
	Controller *TableController
	// ItemKey, when not nil, names item i by a key that stays with it, so
	// that the selection and the focus follow the items when they change
	// order or some are taken out, as Table.RowKey does for rows.
	ItemKey func(i int) core.Key
	// OnActivate runs on Enter or a double click on an item.
	OnActivate func(i int)
	// OnContextMenu asks for the context menu of item i, after a click of the
	// secondary button on its tile or the menu key or Shift+F10 on the tile in
	// focus, as Tree.OnContextMenu does: pos is from the top left of the grid,
	// at the release of the button or the bottom left of the tile, and the
	// press has selected the item unless it was selected already, and given it
	// the focus.
	OnContextMenu func(i int, pos geom.Offset)
	// OnSelectionChange runs after the user changes the selection.
	OnSelectionChange func()
	// ItemBuilder returns the widget of item i, which fills the tile but for a
	// small inset where the ground of the selection shows.
	ItemBuilder func(i int) core.Widget
}

// gridGeometry places the tiles of a grid at a width.
type gridGeometry struct {
	cols         int
	cellW, cellH float32
	// stride is the height of a row and the gap under it.
	stride       float32
	spacing, run float32
	padding      geom.Insets
	width        float32
	rtl          bool
}

// geometry returns where the tiles of g go in a view of the given width.
func (g GridView) geometry(width float32) gridGeometry {
	geo := gridGeometry{
		spacing: max(g.Spacing, 0), run: max(g.RunSpacing, 0), padding: g.Padding,
		width: width, rtl: g.Direction == text.DirectionRTL,
	}
	inner := max(width-g.Padding.Left-g.Padding.Right, 0)
	switch {
	case g.Columns > 0:
		geo.cols = g.Columns
	default:
		least := g.MinColumnWidth
		if least <= 0 {
			least = gridMinColumnWidth
		}
		geo.cols = max(1, int((inner+geo.spacing)/(least+geo.spacing)))
	}
	geo.cellW = max((inner-geo.spacing*float32(geo.cols-1))/float32(geo.cols), 0)
	switch {
	case g.TileHeight > 0:
		geo.cellH = g.TileHeight
	case g.AspectRatio > 0:
		geo.cellH = geo.cellW/g.AspectRatio + g.ExtraHeight
	default:
		geo.cellH = geo.cellW + g.ExtraHeight
	}
	geo.cellH = max(geo.cellH, 0)
	geo.stride = geo.cellH + geo.run
	return geo
}

// rows returns the number of rows of n items.
func (g *gridGeometry) rows(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + g.cols - 1) / g.cols
}

// rowTop returns the top of row r, from the top of the content.
func (g *gridGeometry) rowTop(r int) float64 {
	return float64(g.padding.Top) + float64(r)*float64(g.stride)
}

// rowAt returns the row whose band, the row and the gap under it, holds y.
func (g *gridGeometry) rowAt(y float64) int {
	if g.stride <= 0 {
		return 0
	}
	return max(int(math.Floor((y-float64(g.padding.Top))/float64(g.stride))), 0)
}

// content returns the height of the content of n items.
func (g *gridGeometry) content(n int) float64 {
	rows := g.rows(n)
	h := float64(g.padding.Top + g.padding.Bottom)
	if rows > 0 {
		h += float64(rows)*float64(g.cellH) + float64(rows-1)*float64(g.run)
	}
	return h
}

// cellX returns the left edge of column c.
func (g *gridGeometry) cellX(c int) float32 {
	x := float32(c) * (g.cellW + g.spacing)
	if g.rtl {
		return g.width - g.padding.Right - x - g.cellW
	}
	return g.padding.Left + x
}

type gridState struct {
	basic.ScrollState
	own  TableController
	ctrl *TableController
	// count is the number of items at the last build, geo the places of the
	// tiles at width, and view the height of the view.
	count int
	geo   gridGeometry
	width float32
	view  float32
	// first and last are the rows built.
	first, last int
	body        *renderGridBody
	// active means the keyboard is in the grid, which colors the selection.
	active bool
	// claim asks the cell of the focus to take the keyboard when it is built,
	// and reveal to scroll it into view once the view has a height.
	claim, reveal bool
}

// CreateState implements core.StatefulWidget.
func (GridView) CreateState() core.State { return &gridState{} }

// InitState implements core.State.
func (s *gridState) InitState() {
	s.ScrollState.InitState()
	s.OnMove = s.scrolled
	s.attach(s.Widget().(GridView).Controller)
}

// DidUpdateWidget implements core.State.
func (s *gridState) DidUpdateWidget(core.Widget) { s.attach(s.Widget().(GridView).Controller) }

// Dispose implements core.State.
func (s *gridState) Dispose() {
	s.ScrollState.Dispose()
	if s.ctrl != nil && s.ctrl.view == tableListener(s) {
		s.ctrl.view = nil
	}
}

// attach makes c, or the grid's own controller for nil, the one it shows.
func (s *gridState) attach(c *TableController) {
	if c == nil {
		c = &s.own
	}
	if s.ctrl == c {
		return
	}
	if s.ctrl != nil && s.ctrl.view == tableListener(s) {
		s.ctrl.view = nil
	}
	s.ctrl = c
	c.view = s
}

// controllerChanged implements tableListener.
func (s *gridState) controllerChanged() { kit.Rebuild(&s.StateBase) }

// focusIndex returns the item in focus, or -1 for an empty grid.
func (s *gridState) focusIndex() int {
	if s.count <= 0 {
		return -1
	}
	return min(max(s.ctrl.row, 0), s.count-1)
}

// visible returns the rows to build at the current offset.
func (s *gridState) visible() (first, last int) {
	top := s.Offset.Value()
	g := &s.geo
	first = g.rowAt(top) - cachedItems
	last = max(min(g.rowAt(top+float64(s.view))+1+cachedItems, g.rows(s.count)), 0)
	// The offset may lie past the rows until the next layout clamps it, as
	// when the items shrink.
	return min(max(first, 0), last), last
}

// scrolled follows a move of the view: the rows move at once, and other rows
// are built when they come into range.
func (s *gridState) scrolled() {
	if s.body != nil {
		s.body.scrolled()
	}
	if first, last := s.visible(); first != s.first || last != s.last {
		kit.Rebuild(&s.StateBase)
	}
}

// measured follows the size of the view at a layout. A new number of columns
// keeps the item at the top of the view, or the item in focus if it showed,
// where it was.
func (s *gridState) measured(width, view float32) {
	w := s.Widget().(GridView)
	if width != s.width {
		old := s.geo
		next := w.geometry(width)
		if old.cols != next.cols && s.count > 0 && s.view > 0 && old.stride > 0 {
			s.keepAnchor(&old, &next)
		}
		s.width, s.geo = width, next
	}
	s.Metrics = layout.ScrollMetrics{View: float64(view), Content: s.geo.content(s.count)}
	s.Scroller.SetLimit(s.Metrics.Content - float64(view))
	changed := view != s.view
	s.view = view
	if s.reveal && view > 0 {
		s.reveal = false
		s.revealItem(s.focusIndex())
	}
	if first, last := s.visible(); changed || first != s.first || last != s.last || s.body == nil || s.body.cols != s.geo.cols {
		kit.Rebuild(&s.StateBase)
	}
}

// keepAnchor moves the offset so that the item that led the view under old
// leads it under next, at the same distance from the top of the view.
func (s *gridState) keepAnchor(old, next *gridGeometry) {
	off, view := s.Offset.Value(), float64(s.view)
	item := old.rowAt(off) * old.cols
	if f := s.focusIndex(); f >= 0 {
		top := old.rowTop(f / old.cols)
		if top+float64(old.cellH) > off && top < off+view {
			item = f
		}
	}
	item = min(item, s.count-1)
	from := old.rowTop(item/old.cols) - off
	to := next.rowTop(item/next.cols) - from
	limit := max(next.content(s.count)-view, 0)
	s.Scroller.Jump(min(max(to, 0), limit))
	s.Ticker.Stop()
}

// revealItem scrolls item i into view, at once.
func (s *gridState) revealItem(i int) {
	if i < 0 || s.view <= 0 {
		s.reveal = i >= 0
		return
	}
	g := &s.geo
	top := g.rowTop(i / g.cols)
	off, view, cellH := s.Offset.Value(), float64(s.view), float64(g.cellH)
	switch {
	case top < off:
		off = top - min(float64(g.padding.Top), top)
	case top+cellH > off+view:
		off = top + cellH - view
		if i/g.cols == g.rows(s.count)-1 {
			off += float64(g.padding.Bottom)
		}
	default:
		return
	}
	limit := max(g.content(s.count)-view, 0)
	s.Scroller.Jump(min(max(off, 0), limit))
	s.Ticker.Stop()
	if s.body != nil {
		s.body.scrolled()
	}
	s.ShowBar()
	if first, last := s.visible(); first != s.first || last != s.last {
		kit.Rebuild(&s.StateBase)
	}
}

// Build implements core.State.
func (s *gridState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(GridView)
	n := max(w.ItemCount, 0)
	s.count = n
	c := s.ctrl
	was := c.row
	c.follow(n, w.ItemKey)
	if c.row != was && s.active {
		// The focus followed its item to another tile, which takes the
		// keyboard.
		s.claim = true
	}
	c.clip(n)
	if n > 0 {
		c.row = min(max(c.row, 0), n-1)
		c.anchor = min(max(c.anchor, 0), n-1)
	}
	c.remember(n, w.ItemKey)
	s.geo = w.geometry(s.width)
	focus := s.focusIndex()
	s.first, s.last = s.visible()
	look := newGridLook(basic.ThemeOf(ctx), s.active)
	cols := s.geo.cols

	count := s.last - s.first
	rows := make([]core.Widget, 0, count+1)
	slots := make([]int, 0, count+1)
	add := func(r int) {
		cells := make([]core.Widget, 0, cols)
		for i := r * cols; i < min((r+1)*cols, n); i++ {
			cells = append(cells, core.Keyed(i, s.cell(&w, i, i == focus, &look)))
		}
		rows = append(rows, core.Keyed(r, gridRowView{row: r, cells: cells}))
		slots = append(slots, r)
	}
	focusRow := -1
	if focus >= 0 {
		focusRow = focus / cols
	}
	if focusRow >= 0 && focusRow < s.first {
		add(focusRow)
	}
	for r := s.first; r < s.last; r++ {
		add(r)
	}
	if focusRow >= s.last {
		add(focusRow)
	}
	s.claim = false

	var state semantics.State
	if w.Select == TableSelectMultiple {
		state = semantics.Multiselectable
	}
	body := gridBody{
		host: s, position: &s.Offset, slots: slots, rows: rows, widget: w, cols: cols,
		count: n, label: w.Label, state: state, ref: &s.body,
	}
	return basic.ScrollFrame{Axis: layout.Vertical, State: &s.ScrollState, View: basic.RepaintBoundary{Child: body}}
}

// cell builds the tile of item i.
func (s *gridState) cell(w *GridView, i int, focus bool, look *gridLook) core.Widget {
	v := gridCellView{
		host: s, index: i, look: look,
		selected:  w.Select != TableSelectNone && s.ctrl.IsSelected(i),
		focusable: focus, claim: focus && s.claim,
	}
	if w.LabelOf != nil {
		v.label = w.LabelOf(i)
	}
	if w.ItemBuilder != nil {
		if child := w.ItemBuilder(i); child != nil {
			v.child = basic.RepaintBoundary{Child: child}
		}
	}
	return v
}

// lostFocus follows the cell of the focus leaving the grid with the keyboard,
// as when the items shrink: the cell of the new focus takes it back.
func (s *gridState) lostFocus() {
	if s.active && !s.claim && s.count > 0 {
		s.claim = true
		kit.Rebuild(&s.StateBase)
	}
}

// setActive follows the keyboard entering and leaving the grid.
func (s *gridState) setActive(active bool) {
	if s.active != active {
		s.active = active
		kit.Rebuild(&s.StateBase)
	}
}

// gridLook is what the cells take from the theme.
type gridLook struct {
	hover, selected, ring geom.Color
	radius                float32
}

// newGridLook returns the colors of the cells. As in a Tree, the selection is
// the accent tint while the keyboard is in the grid and a quiet grey
// otherwise.
func newGridLook(th *theme.Theme, active bool) gridLook {
	c := &th.Color
	l := gridLook{hover: c.Fill, selected: c.FillStrong, ring: c.AccentInk.WithAlpha(treeRingOpacity), radius: th.Radius.Control}
	if active {
		l.selected = c.AccentTint
	}
	return l
}
