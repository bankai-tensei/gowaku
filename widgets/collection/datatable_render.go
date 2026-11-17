package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The parts of a header.
const (
	tableSortGap = theme.Space4
	// tableFilterButton is the side of the filter button, tableFilterInset
	// its distance from the end of the header, and tableFilterGlyph the
	// size of the funnel in it.
	tableFilterButton = 20
	tableFilterInset  = theme.Space6
	tableFilterGlyph  = 12
	// tableSeparator is the height of the line between two headers.
	tableSeparator = 16
	// tableTextCache is how many shaped texts a table keeps at least, and
	// tableTextAge how many paints a text stays after its last.
	tableTextCache = 1024
	tableTextAge   = 120
)

// tableTextKey names a shaped text of a table: the string, in the style of a
// cell, a header or a badge.
type tableTextKey struct {
	s     string
	style tableStyle
}

// tableStyle is one of the text styles of a table.
type tableStyle uint8

const (
	styleCell tableStyle = iota
	styleHead
	styleBadge
)

type tableText struct {
	layout *text.Layout
	// frame is the last paint that drew the text.
	frame uint32
}

// tableBody is the widget of renderTable. Its children are the header, the
// row objects and the editor of a cell, if one is open.
type tableBody struct {
	c        *tableCore
	children []core.Widget
}

// Key implements core.Widget.
func (tableBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b tableBody) CreateRenderObject() core.RenderObject {
	r := &renderTable{c: b.c, texts: make(map[tableTextKey]*tableText)}
	r.sortUp, r.sortDown, r.funnel, r.tick = sortChevron(true), sortChevron(false), funnelPath(), tickPath()
	b.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b tableBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTable)
	b.c.ro = r
	th := b.c.th
	head := th.Text.Caption
	head.Weight = th.Text.Headline.Weight
	badge := th.Text.Section
	if r.shaper != b.c.shaper || !r.body.SameLayout(th.Text.Body) || !r.head.SameLayout(head) || !r.badge.SameLayout(badge) {
		clear(r.texts)
	}
	r.shaper, r.body, r.head, r.badge = b.c.shaper, th.Text.Body, head, badge
	r.MarkNeedsLayout()
}

// ChildWidgets implements core.MultiChildWidget.
func (b tableBody) ChildWidgets() []core.Widget { return b.children }

// renderTable lays out, paints and hit tests a table. It draws every cell
// itself, from the rows in view, so that scrolling builds nothing; its
// children draw nothing and exist for the focus, the hover and the
// accessibility tree, which need one object per row and cell in view.
type renderTable struct {
	core.RenderBase
	c        *tableCore
	children []core.RenderObject
	header   *renderTableHeader
	rows     []*renderTableRow
	editor   core.RenderObject

	shaper            *text.Shaper
	body, head, badge text.Style
	texts             map[tableTextKey]*tableText
	frame             uint32

	sortUp, sortDown, funnel, tick *paint.Path
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTable) SetChildren(children []core.RenderObject) {
	r.children = children
	r.header, r.editor = nil, nil
	r.rows = r.rows[:0]
	for _, ch := range children {
		switch ch := ch.(type) {
		case *renderTableHeader:
			r.header = ch
		case *renderTableRow:
			r.rows = append(r.rows, ch)
		default:
			r.editor = ch
		}
	}
}

// ChildCount implements core.RenderObject.
func (r *renderTable) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject. With widget cells, the row objects
// come in the order of the rows they show rather than of their slots, so that
// Tab visits the widgets of the rows from the top down.
func (r *renderTable) ChildAt(i int) core.RenderObject {
	if n := len(r.rows); r.c.widgets && n > 1 && i >= 1 && i <= n && r.header != nil && r.children[0] == core.RenderObject(r.header) {
		first, _ := r.c.visibleRows()
		return r.rows[(first+i-1)%n]
	}
	return r.children[i]
}

// Layout implements core.RenderObject. The table takes all the room it is
// given, which must be bounded both ways, and gives each row in view one of
// its row objects: row i takes object i modulo their number, so that a row
// keeps its object, and its node in the accessibility tree, while it stays in
// view.
func (r *renderTable) Layout(cs layout.Constraints) geom.Size {
	if cs.MaxH >= geom.Inf {
		panic("gowaku: collection.Table has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or an Expanded")
	}
	if cs.MaxW >= geom.Inf {
		panic("gowaku: collection.Table has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded")
	}
	c := r.c
	sz := cs.Constrain(geom.Size{W: cs.MaxW, H: cs.MaxH})
	c.measured(sz.W, sz.H)
	if r.header != nil {
		// The header and the rows place their cells from the scroll offsets,
		// which their own constraints do not show.
		r.header.MarkNeedsLayout()
		core.LayoutChild(r.header, geom.Tight(geom.Size{W: sz.W, H: tableHeaderHeight}))
	}
	for _, row := range r.rows {
		row.r = -1
	}
	if n := len(r.rows); n > 0 {
		first, last := c.visibleRows()
		for i := first; i < last && i-first < n; i++ {
			row := r.rows[i%n]
			row.r = i
			row.MarkNeedsLayout()
			core.LayoutChild(row, geom.Tight(geom.Size{W: sz.W, H: c.rowH}))
			row.SetOffset(geom.Offset{Y: c.rowY(i)})
		}
	}
	if r.editor != nil {
		j := c.displayOf(c.editCol)
		w := c.cols[max(j, 0)].width
		esz := core.LayoutChild(r.editor, geom.Constraints{MinW: w, MaxW: w, MaxH: c.rowH})
		r.editor.SetOffset(geom.Offset{X: c.colX(max(j, 0)), Y: c.rowY(c.editRow) + geom.Floor32((c.rowH-esz.H)/2)})
	}
	if len(c.hosts) > 0 {
		// The size or the number of rows may have moved the rows in view
		// without a scroll.
		c.syncHosts()
	}
	r.syncFocus()
	return sz
}

// syncFocus keeps the keyboard on the object that shows the cell in focus,
// when the table has it: the cell, or the table itself while the cell is out
// of view. After a click, or once an editor closed and took the keyboard with
// it, refocus also takes the keyboard when nothing has it; the table never
// takes it from elsewhere, such as a filter that opened since.
func (r *renderTable) syncFocus() {
	c := r.c
	if c.editing {
		return
	}
	focus := r.Focused()
	mine := core.RenderObject(focus) == r
	if cell, ok := focus.(*renderTableCell); ok && cell.c == c {
		mine = true
	}
	if c.widgets {
		if around := r.cellAround(focus); around != nil {
			// The keyboard stays in the widget of the cell in focus, and
			// leaves one whose cell is not, or no longer shows its row.
			if around == r.focusCell() && around.widgetShown() {
				c.refocus = false
				return
			}
			mine = true
		}
	}
	refocus := c.refocus
	c.refocus = false
	if !mine && (focus != nil || !refocus) {
		return
	}
	if cell := r.focusCell(); cell != nil {
		if core.FocusTarget(cell) != focus {
			cell.RequestFocus()
		}
		return
	}
	if core.FocusTarget(r) != focus {
		r.RequestFocus()
	}
}

// focusCell returns the object of the cell in focus, or nil if it is out of
// view.
func (r *renderTable) focusCell() *renderTableCell {
	c := r.c
	j := c.focusCol()
	if j < 0 {
		return nil
	}
	row := c.focusRow()
	if row < 0 {
		if r.header == nil || j >= len(r.header.cells) {
			return nil
		}
		return r.header.cells[j]
	}
	if n := len(r.rows); n > 0 {
		if slot := r.rows[row%n]; slot.r == row && j < len(slot.cells) {
			return slot.cells[j]
		}
	}
	return nil
}

// slotRow returns the row that row object k shows, or -1.
func (r *renderTable) slotRow(k int) int {
	if k < 0 || k >= len(r.rows) {
		return -1
	}
	return r.rows[k].r
}

// HitTest implements core.RenderObject. The header lies over the rows that
// scroll under it, so a point decides which of the two it tests.
func (r *renderTable) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	switch {
	case pos.Y < tableHeaderHeight:
		if r.header != nil {
			r.header.HitTest(pos.Sub(r.header.Offset()), res)
		}
	case r.editor != nil && r.editor.HitTest(pos.Sub(r.editor.Offset()), res):
	default:
		for _, row := range r.rows {
			if row.r >= 0 && row.HitTest(pos.Sub(row.Offset()), res) {
				break
			}
		}
	}
	res.Path = append(res.Path, r)
	return true
}

// ChildClip implements core.Clipper. The table cuts what it paints to its
// box, so the rows it builds below its bottom edge do not show.
func (r *renderTable) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderTable) Paint(cv *paint.Canvas, off geom.Offset) {
	c := r.c
	col := &c.th.Color
	sz := r.Size()
	r.frame++
	box := geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}
	cv.Save()
	cv.ClipRect(box)
	cv.DrawRRect(geom.RRectAll(box, 0), paint.Paint{Color: col.Card})

	body := geom.Rect{X: off.X, Y: off.Y + tableHeaderHeight, W: sz.W, H: max(sz.H-tableHeaderHeight, 0)}
	cv.Save()
	cv.ClipRect(body)
	first, last := c.visibleRows()
	hover := r.slotRow(c.hoverSlot)
	selectable := c.cfg.mode != TableSelectNone
	for i := first; i < last; i++ {
		var ground geom.Color
		switch {
		case selectable && c.ctrl.IsSelected(i):
			ground = col.AccentTint
		case i == hover && c.drag.kind == tableDragNone:
			ground = col.Fill
		}
		if ground.A > 0 {
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y + c.rowY(i), W: sz.W, H: c.rowH}, 0), paint.Paint{Color: ground})
		}
	}
	if c.frozen < len(c.cols) {
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X + c.frozenW, Y: body.Y, W: max(sz.W-c.frozenW, 0), H: body.H})
		for i := first; i < last; i++ {
			r.paintRow(cv, off, i, c.frozen, len(c.cols))
		}
		cv.Restore()
	}
	for i := first; i < last; i++ {
		r.paintRow(cv, off, i, 0, c.frozen)
	}
	if c.frozen > 0 {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X + c.frozenW - 1, Y: body.Y, W: 1, H: body.H}, 0), paint.Paint{Color: col.Hair})
	}
	if r.editor != nil {
		r.editor.Paint(cv, off.Add(r.editor.Offset()))
	}
	if row := c.focusRow(); row >= 0 && r.ringShown() && c.rowVisible(row) {
		r.paintRing(cv, off, row, c.focusCol())
	}
	cv.Restore()

	r.paintHeader(cv, off)
	cv.Restore()
	r.evict(last - first)
}

// paintRow draws the cells of row i in the columns from j0 up to j1.
func (r *renderTable) paintRow(cv *paint.Canvas, off geom.Offset, i, j0, j1 int) {
	c := r.c
	y := off.Y + c.rowY(i)
	ink := c.th.Color.Ink
	w := r.Size().W
	for j := j0; j < j1; j++ {
		col := &c.cols[j]
		x := c.colX(j)
		if x >= w {
			break
		}
		if x+col.width <= 0 || j >= c.frozen && x+col.width <= c.frozenW {
			continue
		}
		box := geom.Rect{X: off.X + x, Y: y, W: col.width, H: c.rowH}
		switch col.kind {
		case CellText:
			r.paintText(cv, c.src.cellText(i, col.model), styleCell, box, col.trailing, ink)
		case CellWidget:
			r.paintWidget(cv, off, i, j, box)
		default:
			r.paintCell(cv, col, i, box)
		}
	}
}

// paintText draws s in the cell box, inset by the padding of a cell, cut at
// its end when it is too long.
func (r *renderTable) paintText(cv *paint.Canvas, s string, style tableStyle, box geom.Rect, trailing bool, color geom.Color) {
	if s == "" {
		return
	}
	l := r.layoutOf(s, style)
	ts := l.Size()
	room := box.W - 2*tablePadding
	if room <= 0 {
		return
	}
	at := geom.Offset{X: box.X + tablePadding, Y: box.Y + geom.Floor32((box.H-ts.H)/2+0.5)}
	long := ts.W > room
	if trailing && !long {
		at.X = box.X + box.W - tablePadding - ts.W
	}
	if long {
		cv.Save()
		cv.ClipRect(geom.Rect{X: box.X + tablePadding, Y: box.Y, W: room, H: box.H})
	}
	runs := l.Runs()
	for k := range runs {
		cv.DrawGlyphs(&runs[k], at, color)
	}
	if long {
		cv.Restore()
	}
}

// layoutOf returns s shaped in a style of the table. Shaping a new string
// allocates, which a cache miss may do (CODING_STYLE §7).
func (r *renderTable) layoutOf(s string, style tableStyle) *text.Layout {
	k := tableTextKey{s: s, style: style}
	if t, ok := r.texts[k]; ok {
		t.frame = r.frame
		return t.layout
	}
	ts := r.body
	switch style {
	case styleHead:
		ts = r.head
	case styleBadge:
		ts = r.badge
	}
	l := r.shaper.Shape(s, ts).Layout(geom.Inf)
	r.texts[k] = &tableText{layout: l, frame: r.frame}
	return l
}

// evict forgets texts once there are many more of them than cells in view:
// first those not drawn for a while, and if that is not enough, all that the
// last paint did not draw. A scroll back and forth keeps its texts.
func (r *renderTable) evict(rows int) {
	keep := max(tableTextCache, 4*(rows+1)*len(r.c.cols))
	if len(r.texts) <= keep {
		return
	}
	for k, t := range r.texts {
		if r.frame-t.frame > tableTextAge {
			delete(r.texts, k)
		}
	}
	if len(r.texts) <= 2*keep {
		return
	}
	for k, t := range r.texts {
		if t.frame != r.frame {
			delete(r.texts, k)
		}
	}
}

// ringShown reports whether a cell of the table has the keyboard, and the
// keyboard gave it.
func (r *renderTable) ringShown() bool {
	cell, ok := r.Focused().(*renderTableCell)
	return ok && cell.c == r.c && r.FocusVisible()
}

// paintRing draws the focus ring inside the cell of row i, -1 for the header,
// and column j.
func (r *renderTable) paintRing(cv *paint.Canvas, off geom.Offset, i, j int) {
	c := r.c
	if j < 0 || j >= len(c.cols) {
		return
	}
	y := off.Y
	if i >= 0 {
		y += c.rowY(i)
	}
	h := c.rowH
	if i < 0 {
		h = tableHeaderHeight
	}
	box := geom.Rect{X: off.X + c.colX(j) + 1, Y: y + 1, W: c.cols[j].width - 2, H: h - 2}
	cv.DrawRRect(geom.RRectAll(box, c.th.Radius.Check), paint.Paint{Stroke: &paint.Stroke{Width: 2, Color: c.th.Color.AccentInk}})
}

// Semantics implements core.RenderObject. Only the rows in view have nodes;
// the grid tells the number of all of them.
func (r *renderTable) Semantics(sb *core.SemanticsBuilder) {
	c := r.c
	node := semantics.Node{Role: semantics.RoleGrid, Label: c.cfg.label, RowCount: c.cfg.rows + 1, ColumnCount: len(c.cols)}
	if c.cfg.mode == TableSelectMultiple {
		node.State |= semantics.Multiselectable
	}
	sb.Open(node, r.Size())
	if r.header != nil {
		sb.Visit(r.header)
	}
	if n := len(r.rows); n > 0 {
		first, last := c.visibleRows()
		for i := first; i < last && i-first < n; i++ {
			if row := r.rows[i%n]; row.r == i {
				sb.Visit(row)
			}
		}
	}
	if r.editor != nil {
		sb.Visit(r.editor)
	}
	sb.Close()
}

// HandlePointer implements core.PointerTarget.
func (r *renderTable) HandlePointer(e *core.PointerEvent) { r.c.pointer(e) }

// HandleHover implements core.HoverTarget.
func (r *renderTable) HandleHover(inside bool) {
	if !inside {
		r.c.hoverSlot, r.c.hoverHeader, r.c.hoverZone = -1, -1, tableZone{}
		r.MarkNeedsPaint()
	}
}

// CanFocus implements core.FocusTarget. The table takes the keyboard only while
// the cell in focus is out of view, so that it is one stop of Tab.
func (r *renderTable) CanFocus() bool { return !r.c.editing && r.focusCell() == nil }

// HandleKey implements core.FocusTarget. Keys reach it from the cells and the
// editor below it too.
func (r *renderTable) HandleKey(e *core.KeyEvent) bool { return r.c.key(e) }

// HandleText implements core.FocusTarget.
func (r *renderTable) HandleText(s string) { r.c.typed(s) }

// HandleComposition implements core.FocusTarget.
func (r *renderTable) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderTable) FocusChanged(bool) { r.MarkNeedsPaint() }

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderTable) FocusVisibleChanged(bool) { r.MarkNeedsPaint() }
