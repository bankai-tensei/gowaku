package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// gridBody is the widget of the rows of a grid.
type gridBody struct {
	host *gridState
	// position is the scroll offset, slots the row of each child.
	position *layout.ScrollOffset
	slots    []int
	rows     []core.Widget
	widget   GridView
	// cols is the number of columns the rows were built with, and count that
	// of the items.
	cols, count int
	label       string
	state       semantics.State
	ref         **renderGridBody
}

// Key implements core.Widget.
func (gridBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b gridBody) CreateRenderObject() core.RenderObject {
	r := &renderGridBody{position: b.position}
	b.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b gridBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderGridBody)
	r.host, r.widget = b.host, b.widget
	r.cols, r.count, r.label, r.state = b.cols, b.count, b.label, b.state
	r.slots = append(r.slots[:0], b.slots...)
	r.MarkNeedsLayout()
	if b.ref != nil {
		*b.ref = r
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (b gridBody) ChildWidgets() []core.Widget { return b.rows }

// renderGridBody places the rows that exist at their rows, moved by the scroll
// offset. Only the rows near the view exist, and the row of the focus, which
// keeps the keyboard when it scrolls away. Layout and Paint do not allocate.
type renderGridBody struct {
	core.RenderBase
	host     *gridState
	position *layout.ScrollOffset
	slots    []int
	children []core.RenderObject
	widget   GridView
	cols     int
	count    int
	label    string
	state    semantics.State
	// geo is the geometry at the width of the last layout.
	geo gridGeometry
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderGridBody) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderGridBody) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderGridBody) ChildAt(i int) core.RenderObject { return r.children[i] }

// scrolled moves the rows to the scroll offset.
func (r *renderGridBody) scrolled() {
	r.place()
	r.MarkNeedsPaint()
}

func (r *renderGridBody) place() {
	top := r.position.Value()
	for k, c := range r.children {
		if k < len(r.slots) {
			c.SetOffset(geom.Offset{Y: float32(r.geo.rowTop(r.slots[k]) - top)})
		}
	}
}

// Layout implements core.RenderObject. Like List, it takes all the room it is
// given.
func (r *renderGridBody) Layout(c geom.Constraints) geom.Size {
	if c.MaxH >= geom.Inf {
		panic("gowaku: collection.GridView has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or an Expanded")
	}
	if c.MaxW >= geom.Inf {
		panic("gowaku: collection.GridView has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded")
	}
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	geo := r.widget.geometry(sz.W)
	changed := geo != r.geo
	r.geo = geo
	row := geom.Tight(geom.Size{W: sz.W, H: geo.cellH})
	for _, child := range r.children {
		if gr, ok := child.(*renderGridRow); ok {
			if changed || gr.geo != &r.geo {
				gr.geo = &r.geo
				gr.MarkNeedsLayout()
			}
		}
		core.LayoutChild(child, row)
	}
	if r.host != nil {
		r.host.measured(sz.W, sz.H)
		if r.Attached() && r.Focused() == nil {
			r.host.lostFocus()
		}
	}
	r.place()
	return sz
}

// inView reports whether a row at offset y shows.
func (r *renderGridBody) inView(y float32) bool { return y+r.geo.cellH > 0 && y < r.Size().H }

// ChildClip implements core.Clipper.
func (r *renderGridBody) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderGridBody) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	for _, c := range r.children {
		if o := c.Offset(); r.inView(o.Y) {
			c.Paint(cv, off.Add(o))
		}
	}
	cv.Restore()
}

// HitTest implements core.RenderObject. A row scrolled out of the view is not
// hit, though it exists.
func (r *renderGridBody) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	for i := len(r.children) - 1; i >= 0; i-- {
		c := r.children[i]
		if r.inView(c.Offset().Y) && c.HitTest(pos.Sub(c.Offset()), res) {
			break
		}
	}
	res.Path = append(res.Path, r)
	return true
}

// HandlePointer implements core.PointerTarget: the wheel scrolls, and a press
// between the tiles clears the selection.
func (r *renderGridBody) HandlePointer(e *core.PointerEvent) {
	if r.host == nil {
		return
	}
	switch {
	case e.Kind == core.PointerScroll && e.Scroll.Y != 0:
		r.host.Wheel(-e.Scroll.Y)
		e.Handled = true
	case e.Kind == core.PointerDown && e.Button == input.ButtonPrimary:
		r.host.emptyPress()
		e.Handled = true
	}
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderGridBody) FocusWithinChanged(within bool) {
	if r.host != nil {
		r.host.setActive(within)
	}
}

// Semantics implements core.RenderObject. The grid states the number of all
// its rows and columns; only the rows in view speak, and the row of the focus
// wherever it is.
func (r *renderGridBody) Semantics(sb *core.SemanticsBuilder) {
	cols := min(r.cols, r.count)
	sb.Open(semantics.Node{
		Role: semantics.RoleGrid, Label: r.label, State: r.state,
		RowCount: r.geo.rows(r.count), ColumnCount: cols,
	}, r.Size())
	for _, c := range r.children {
		if row, ok := c.(*renderGridRow); ok && (r.inView(c.Offset().Y) || row.hasFocus()) {
			sb.Visit(c)
		}
	}
	sb.Close()
}

// gridRowView is the widget of one row of tiles.
type gridRowView struct {
	row   int
	cells []core.Widget
}

// Key implements core.Widget.
func (gridRowView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v gridRowView) CreateRenderObject() core.RenderObject { return &renderGridRow{row: v.row} }

// UpdateRenderObject implements core.RenderWidget.
func (v gridRowView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderGridRow)
	if r.row != v.row {
		r.row = v.row
		r.MarkNeedsLayout()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (v gridRowView) ChildWidgets() []core.Widget { return v.cells }

// renderGridRow places the tiles of one row in their columns, and is a row of
// the grid to a screen reader.
type renderGridRow struct {
	core.RenderBase
	row   int
	geo   *gridGeometry
	cells []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderGridRow) SetChildren(children []core.RenderObject) {
	r.cells = children
	r.MarkNeedsLayout()
}

// ChildCount implements core.RenderObject.
func (r *renderGridRow) ChildCount() int { return len(r.cells) }

// ChildAt implements core.RenderObject.
func (r *renderGridRow) ChildAt(i int) core.RenderObject { return r.cells[i] }

// Layout implements core.RenderObject.
func (r *renderGridRow) Layout(c geom.Constraints) geom.Size {
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	if r.geo == nil {
		return sz
	}
	g := r.geo
	cell := geom.Tight(geom.Size{W: g.cellW, H: g.cellH})
	for k, child := range r.cells {
		core.LayoutChild(child, cell)
		child.SetOffset(geom.Offset{X: g.cellX(k)})
		if gc, ok := child.(*renderGridCell); ok {
			gc.row, gc.col = r.row, k
		}
	}
	return sz
}

// Paint implements core.RenderObject.
func (r *renderGridRow) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, c := range r.cells {
		c.Paint(cv, off.Add(c.Offset()))
	}
}

// hasFocus reports whether a tile of the row is the one in focus.
func (r *renderGridRow) hasFocus() bool {
	for _, c := range r.cells {
		if gc, ok := c.(*renderGridCell); ok && gc.focusable {
			return true
		}
	}
	return false
}

// Semantics implements core.RenderObject.
func (r *renderGridRow) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleRow, RowIndex: r.row + 1}, r.Size())
	for _, c := range r.cells {
		sb.Visit(c)
	}
	sb.Close()
}

// gridCellView is the widget of one tile.
type gridCellView struct {
	host                       *gridState
	index                      int
	look                       *gridLook
	label                      string
	selected, focusable, claim bool
	child                      core.Widget
}

// Key implements core.Widget.
func (gridCellView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v gridCellView) CreateRenderObject() core.RenderObject {
	r := &renderGridCell{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v gridCellView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderGridCell)
	r.host, r.index, r.label = v.host, v.index, v.label
	if v.claim {
		r.claim = true
		r.MarkNeedsLayout()
	}
	if r.look != *v.look || r.selected != v.selected || r.focusable != v.focusable {
		r.look, r.selected, r.focusable = *v.look, v.selected, v.focusable
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (v gridCellView) ChildWidgets() []core.Widget {
	if v.child == nil {
		return nil
	}
	return []core.Widget{v.child}
}

// renderGridCell draws the ground of a tile under its content. It is the
// keyboard focus of the grid while it is the tile of the focus, and a cell of
// the grid to a screen reader. Paint does not allocate.
type renderGridCell struct {
	core.RenderBase
	host                *gridState
	index               int
	row, col            int
	look                gridLook
	label               string
	selected, focusable bool
	claim, hovered      bool
	// menuPress and reclaim are as in renderTreeRow.
	menuPress, reclaim bool
	kids               []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderGridCell) SetChildren(children []core.RenderObject) { r.kids = children }

// ChildCount implements core.RenderObject.
func (r *renderGridCell) ChildCount() int { return len(r.kids) }

// ChildAt implements core.RenderObject.
func (r *renderGridCell) ChildAt(i int) core.RenderObject { return r.kids[i] }

// Layout implements core.RenderObject.
func (r *renderGridCell) Layout(c geom.Constraints) geom.Size {
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	inner := geom.Tight(geom.Size{W: max(sz.W-2*gridCellInset, 0), H: max(sz.H-2*gridCellInset, 0)})
	for _, k := range r.kids {
		core.LayoutChild(k, inner)
		k.SetOffset(geom.Offset{X: gridCellInset, Y: gridCellInset})
	}
	if r.claim {
		r.claim = false
		if r.focusable {
			r.RequestFocus()
		}
	}
	if r.reclaim {
		r.reclaim = false
		if r.focusable && kit.FocusFrom(r) == nil {
			r.RequestFocus()
		}
	}
	return sz
}

// Paint implements core.RenderObject.
func (r *renderGridCell) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	l := &r.look
	rr := geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, l.radius)
	switch {
	case r.selected:
		cv.DrawRRect(rr, paint.Paint{Color: l.selected})
	case r.hovered:
		cv.DrawRRect(rr, paint.Paint{Color: l.hover})
	}
	for _, k := range r.kids {
		k.Paint(cv, off.Add(k.Offset()))
	}
	if r.focusable && r.Focused() == core.FocusTarget(r) && r.FocusVisible() {
		ring := geom.RRectAll(geom.Rect{X: off.X + gridRing/2, Y: off.Y + gridRing/2, W: sz.W - gridRing, H: sz.H - gridRing}, l.radius)
		cv.DrawRRect(ring, paint.Paint{Stroke: &paint.Stroke{Width: gridRing, Color: l.ring}})
	}
}

// HandlePointer implements core.PointerTarget.
func (r *renderGridCell) HandlePointer(e *core.PointerEvent) {
	if r.host == nil {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		r.menuPress = false
		switch e.Button {
		case input.ButtonPrimary:
			e.Handled = true
			r.host.cellPress(r.index, e)
			// The grid moves the focus here; the press gives the keyboard to
			// the nearest object that takes it, which is now this tile.
			r.focusable = true
		case input.ButtonSecondary:
			// As a row of a Tree does.
			owns := r.host.cellSecondaryPress(r.index)
			r.focusable = true
			r.RequestFocus()
			if owns {
				e.Handled, r.menuPress = true, true
				return
			}
			r.reclaim = true
			r.MarkNeedsLayout()
		}
	case core.PointerUp:
		if r.menuPress {
			r.menuPress = false
			if sz := r.Size(); e.Pos.X >= 0 && e.Pos.Y >= 0 && e.Pos.X < sz.W && e.Pos.Y < sz.H {
				r.host.cellContextMenu(r.index, core.LocalToGlobal(r, e.Pos))
			}
		}
	case core.PointerCancel:
		r.menuPress = false
	}
}

// HandleHover implements core.HoverTarget.
func (r *renderGridCell) HandleHover(inside bool) {
	if r.hovered != inside {
		r.hovered = inside
		r.MarkNeedsPaint()
	}
}

// CanFocus implements core.FocusTarget. Only the tile of the focus takes it,
// so that Tab stops once in the grid.
func (r *renderGridCell) CanFocus() bool { return r.focusable }

// HandleKey implements core.FocusTarget.
func (r *renderGridCell) HandleKey(e *core.KeyEvent) bool {
	if r.host == nil {
		return false
	}
	if kit.IsContextMenuKey(e) {
		return r.host.cellContextMenu(r.index, core.LocalToGlobal(r, geom.Offset{Y: r.Size().H}))
	}
	return r.host.key(e)
}

// HandleText implements core.FocusTarget.
func (r *renderGridCell) HandleText(string) {}

// HandleComposition implements core.FocusTarget.
func (r *renderGridCell) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderGridCell) FocusChanged(focused bool) {
	if focused && r.host != nil {
		r.host.cellFocused(r.index)
	}
	r.MarkNeedsPaint()
}

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderGridCell) FocusVisibleChanged(bool) { r.MarkNeedsPaint() }

// Semantics implements core.RenderObject.
func (r *renderGridCell) Semantics(sb *core.SemanticsBuilder) {
	n := semantics.Node{
		Role: semantics.RoleGridCell, Label: r.label, RowIndex: r.row + 1, ColumnIndex: r.col + 1,
		Actions: semantics.ActionTap | semantics.ActionScrollIntoView,
	}
	if r.selected {
		n.State |= semantics.Selected
	}
	sb.Open(n, r.Size())
	for _, k := range r.kids {
		sb.Visit(k)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderGridCell) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	return r.host != nil && r.host.cellAction(r.index, a)
}
