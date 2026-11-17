package collection

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// reorderBody is the widget of the items of a ReorderableList.
type reorderBody struct {
	host *reorderState
	// slots is the item of each child.
	slots []int
	items []core.Widget
	look  reorderLook
	ref   **renderReorderBody
}

// Key implements core.Widget.
func (reorderBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b reorderBody) CreateRenderObject() core.RenderObject {
	r := &renderReorderBody{}
	b.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b reorderBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderReorderBody)
	r.host, r.look = b.host, b.look
	r.slots = append(r.slots[:0], b.slots...)
	r.MarkNeedsLayout()
	if b.ref != nil {
		*b.ref = r
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (b reorderBody) ChildWidgets() []core.Widget { return b.items }

// renderReorderBody places the items that exist at their places, moved by
// the scroll offset and by how far each slides, and draws the dragged item
// lifted over the others, under the pointer. Only the items near the view
// exist, and the item of the focus, which is also the one dragged. Layout,
// Paint and the slides do not allocate.
type renderReorderBody struct {
	core.RenderBase
	host     *reorderState
	slots    []int
	children []core.RenderObject
	look     reorderLook
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderReorderBody) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderReorderBody) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderReorderBody) ChildAt(i int) core.RenderObject { return r.children[i] }

// scrolled moves the items to the scroll offset and the drag.
func (r *renderReorderBody) scrolled() {
	r.place()
	r.MarkNeedsPaint()
}

// dragged reports whether child k is the item being dragged.
func (r *renderReorderBody) dragged(k int) bool {
	d := &r.host.drag
	return d.dragging && k < len(r.slots) && r.slots[k] == d.index
}

func (r *renderReorderBody) place() {
	if r.host == nil {
		return
	}
	top := r.host.Offset.Value()
	for k, c := range r.children {
		if k >= len(r.slots) {
			continue
		}
		y := float32(r.host.top(r.slots[k]) - top)
		if it, ok := c.(*renderReorderItem); ok {
			y += it.shift
		}
		if r.dragged(k) {
			y = r.host.dragTop()
		}
		c.SetOffset(geom.Offset{Y: y})
	}
}

// startAt makes item i slide to its place from d away.
func (r *renderReorderBody) startAt(i int, d float32) {
	if r.host.still {
		return
	}
	for k, c := range r.children {
		if it, ok := c.(*renderReorderItem); ok && k < len(r.slots) && r.slots[k] == i {
			it.shift, it.placed = d, true
		}
	}
}

// retarget sends each item to where it stands now: its place, or out of the
// way of a dragged item. An item seen for the first time, or any with
// ReduceMotion, goes there at once; the others slide on the clock.
func (r *renderReorderBody) retarget() {
	s := r.host
	moving := false
	for k, c := range r.children {
		it, ok := c.(*renderReorderItem)
		if !ok || k >= len(r.slots) {
			continue
		}
		it.shiftTo = s.shiftTarget(r.slots[k])
		if !it.placed || s.still {
			it.shift, it.placed = it.shiftTo, true
		}
		moving = moving || it.shift != it.shiftTo
	}
	if moving {
		s.clock.Start()
	}
}

// slide moves each item toward where it goes, after dt, and reports whether
// any is still on its way.
func (r *renderReorderBody) slide(dt time.Duration) bool {
	k := float32(0)
	if dt > 0 {
		k = 1 - float32(math.Exp(-dt.Seconds()/reorderSlideTime.Seconds()))
	}
	moving, changed := false, false
	for _, c := range r.children {
		it, ok := c.(*renderReorderItem)
		if !ok || it.shift == it.shiftTo {
			continue
		}
		it.shift += (it.shiftTo - it.shift) * k
		if geom.Abs32(it.shiftTo-it.shift) < 0.5 {
			it.shift = it.shiftTo
		}
		changed = changed || k > 0
		moving = moving || it.shift != it.shiftTo
	}
	if changed {
		r.place()
		r.MarkNeedsPaint()
	}
	return moving
}

// Layout implements core.RenderObject. Like List, it takes all the room it is
// given.
func (r *renderReorderBody) Layout(c geom.Constraints) geom.Size {
	if c.MaxH >= geom.Inf {
		panic("gowaku: collection.ReorderableList has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or an Expanded")
	}
	if c.MaxW >= geom.Inf {
		panic("gowaku: collection.ReorderableList has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded")
	}
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	s := r.host
	if s == nil {
		return sz
	}
	for k, child := range r.children {
		if k < len(r.slots) {
			core.LayoutChild(child, geom.Tight(geom.Size{W: sz.W, H: s.extentOf(r.slots[k])}))
		}
	}
	s.measured(sz.H)
	if r.Attached() && r.Focused() == nil {
		s.lostFocus()
	}
	if s.settle.pending {
		s.settle.pending = false
		for k, child := range r.children {
			if it, ok := child.(*renderReorderItem); ok && k < len(r.slots) {
				it.shift, it.shiftTo, it.placed = s.settleStart(r.slots[k]), 0, true
			}
		}
	}
	r.retarget()
	r.place()
	return sz
}

// inView reports whether child k at offset y shows.
func (r *renderReorderBody) inView(k int, y float32) bool {
	return y+r.children[k].Size().H > 0 && y < r.Size().H
}

// ChildClip implements core.Clipper.
func (r *renderReorderBody) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject. The dragged item comes last, over the
// others, on a ground with a shadow.
func (r *renderReorderBody) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	lifted := -1
	for k, c := range r.children {
		if r.dragged(k) {
			lifted = k
			continue
		}
		if o := c.Offset(); r.inView(k, o.Y) {
			c.Paint(cv, off.Add(o))
		}
	}
	if lifted >= 0 {
		c := r.children[lifted]
		o := off.Add(c.Offset())
		csz := c.Size()
		rr := geom.RRectAll(geom.Rect{X: o.X, Y: o.Y, W: csz.W, H: csz.H}, r.look.radius)
		cv.DrawShadow(rr, reorderShadowBlur, r.look.shadow, geom.Offset{Y: reorderShadowDrop})
		cv.DrawRRect(rr, paint.Paint{Color: r.look.ground})
		c.Paint(cv, o)
	}
	cv.Restore()
}

// HitTest implements core.RenderObject. An item scrolled out of the view is
// not hit, though it exists.
func (r *renderReorderBody) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	for k := len(r.children) - 1; k >= 0; k-- {
		c := r.children[k]
		if r.inView(k, c.Offset().Y) && c.HitTest(pos.Sub(c.Offset()), res) {
			break
		}
	}
	res.Path = append(res.Path, r)
	return true
}

// HandlePointer implements core.PointerTarget: the wheel scrolls.
func (r *renderReorderBody) HandlePointer(e *core.PointerEvent) {
	if r.host != nil && e.Kind == core.PointerScroll && e.Scroll.Y != 0 {
		r.host.Wheel(-e.Scroll.Y)
		e.Handled = true
	}
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderReorderBody) FocusWithinChanged(within bool) {
	if r.host != nil {
		r.host.setActive(within)
	}
}

// Semantics implements core.RenderObject. Only the items in view speak, and
// the item of the focus wherever it is.
func (r *renderReorderBody) Semantics(sb *core.SemanticsBuilder) {
	for k, c := range r.children {
		if it, ok := c.(*renderReorderItem); ok && (r.inView(k, c.Offset().Y) || it.focusable) {
			sb.Visit(c)
		}
	}
}

// reorderItemView is the widget of one item.
type reorderItemView struct {
	host             *reorderState
	index, count     int
	look             *reorderLook
	label            string
	focusable, claim bool
	child            core.Widget
}

// Key implements core.Widget.
func (reorderItemView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v reorderItemView) CreateRenderObject() core.RenderObject {
	r := &renderReorderItem{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v reorderItemView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderReorderItem)
	r.host, r.index, r.count, r.label = v.host, v.index, v.count, v.label
	if v.claim {
		r.claim = true
		r.MarkNeedsLayout()
	}
	if r.look != *v.look || r.focusable != v.focusable {
		r.look, r.focusable = *v.look, v.focusable
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (v reorderItemView) ChildWidgets() []core.Widget {
	if v.child == nil {
		return nil
	}
	return []core.Widget{v.child}
}

// renderReorderItem holds the content of an item. It takes the press that
// starts a drag, is the keyboard focus of the list while it is the item of
// the focus, and is an item of the list to a screen reader. shift is how far
// it stands from its place at rest, sliding toward shiftTo.
type renderReorderItem struct {
	core.RenderBase
	host             *reorderState
	index, count     int
	look             reorderLook
	label            string
	focusable, claim bool
	shift, shiftTo   float32
	placed           bool
	kids             []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderReorderItem) SetChildren(children []core.RenderObject) { r.kids = children }

// ChildCount implements core.RenderObject.
func (r *renderReorderItem) ChildCount() int { return len(r.kids) }

// ChildAt implements core.RenderObject.
func (r *renderReorderItem) ChildAt(i int) core.RenderObject { return r.kids[i] }

// Layout implements core.RenderObject.
func (r *renderReorderItem) Layout(c geom.Constraints) geom.Size {
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	for _, k := range r.kids {
		core.LayoutChild(k, geom.Tight(sz))
	}
	if r.claim {
		r.claim = false
		if r.focusable {
			r.RequestFocus()
		}
	}
	return sz
}

// Paint implements core.RenderObject.
func (r *renderReorderItem) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, k := range r.kids {
		k.Paint(cv, off.Add(k.Offset()))
	}
	if r.focusable && r.Focused() == core.FocusTarget(r) && r.FocusVisible() {
		sz := r.Size()
		ring := geom.RRectAll(geom.Rect{X: off.X + reorderRing/2, Y: off.Y + reorderRing/2, W: sz.W - reorderRing, H: sz.H - reorderRing}, r.look.radius)
		cv.DrawRRect(ring, paint.Paint{Stroke: &paint.Stroke{Width: reorderRing, Color: r.look.ring}})
	}
}

// HandlePointer implements core.PointerTarget. The press goes to the list,
// and so does the drag that may follow.
func (r *renderReorderItem) HandlePointer(e *core.PointerEvent) {
	if r.host == nil {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		r.host.press(r.index, core.GlobalOffset(r).Add(e.Pos))
		r.focusable = true
	case core.PointerMove, core.PointerUp, core.PointerCancel:
		r.host.pointer(e.Kind, core.GlobalOffset(r).Add(e.Pos))
	}
}

// CanFocus implements core.FocusTarget. Only the item of the focus takes it,
// so that Tab stops once in the list.
func (r *renderReorderItem) CanFocus() bool { return r.focusable }

// HandleKey implements core.FocusTarget.
func (r *renderReorderItem) HandleKey(e *core.KeyEvent) bool {
	return r.host != nil && r.host.key(e)
}

// HandleText implements core.FocusTarget.
func (r *renderReorderItem) HandleText(string) {}

// HandleComposition implements core.FocusTarget.
func (r *renderReorderItem) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderReorderItem) FocusChanged(focused bool) {
	if focused && r.host != nil {
		r.host.itemFocused(r.index)
	}
	r.MarkNeedsPaint()
}

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderReorderItem) FocusVisibleChanged(bool) { r.MarkNeedsPaint() }

// Semantics implements core.RenderObject.
func (r *renderReorderItem) Semantics(sb *core.SemanticsBuilder) {
	actions := semantics.ActionScrollIntoView
	if r.host != nil && r.host.Widget().(ReorderableList).OnTap != nil {
		actions |= semantics.ActionTap
	}
	sb.Open(semantics.Node{
		Role: semantics.RoleListItem, Label: r.label, PosInSet: r.index + 1, SetSize: r.count,
		Actions: actions,
	}, r.Size())
	for _, k := range r.kids {
		sb.Visit(k)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderReorderItem) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	return r.host != nil && r.host.itemAction(r.index, a)
}
