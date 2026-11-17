package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// treeIndicator marks where dragged rows would go: a line between rows, or a
// frame around the row they would go into. Its places are those of the rows,
// before scrolling.
type treeIndicator struct {
	// into frames row; otherwise line is the top of the line, from x on.
	shown, into bool
	row         int
	line        float64
	x           float32
}

// treeBody is the widget of the rows of a tree.
type treeBody struct {
	state treeHost
	// position is the scroll offset, slots the row of each child.
	position *layout.ScrollOffset
	slots    []int
	rows     []core.Widget
	bar      geom.Color
	ref      **renderTreeBody
}

// Key implements core.Widget.
func (treeBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b treeBody) CreateRenderObject() core.RenderObject {
	r := &renderTreeBody{position: b.position}
	b.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b treeBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTreeBody)
	r.host, r.bar = b.state, b.bar
	r.slots = append(r.slots[:0], b.slots...)
	r.MarkNeedsLayout()
	if b.ref != nil {
		*b.ref = r
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (b treeBody) ChildWidgets() []core.Widget { return b.rows }

// renderTreeBody places the rows that exist at their rows, moved by the scroll
// offset, and draws the drop indicator over them. Only rows near the view
// exist, and the row of the focus, which keeps the keyboard when it scrolls
// away. Layout and Paint do not allocate.
type renderTreeBody struct {
	core.RenderBase
	host     treeHost
	position *layout.ScrollOffset
	slots    []int
	children []core.RenderObject
	bar      geom.Color
	drop     treeIndicator
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTreeBody) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderTreeBody) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderTreeBody) ChildAt(i int) core.RenderObject { return r.children[i] }

// scrolled moves the rows to the scroll offset.
func (r *renderTreeBody) scrolled() {
	r.place()
	r.MarkNeedsPaint()
}

func (r *renderTreeBody) place() {
	top := r.position.Value()
	for k, c := range r.children {
		if k < len(r.slots) {
			c.SetOffset(geom.Offset{Y: float32(float64(r.slots[k])*treeRowHeight - top)})
		}
	}
}

// Layout implements core.RenderObject. Like List, it takes all the room
// it is given.
func (r *renderTreeBody) Layout(c geom.Constraints) geom.Size {
	if c.MaxH >= geom.Inf {
		panic("gowaku: collection.Tree has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or an Expanded")
	}
	if c.MaxW >= geom.Inf {
		panic("gowaku: collection.Tree has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded")
	}
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	row := geom.Tight(geom.Size{W: sz.W, H: treeRowHeight})
	for _, child := range r.children {
		core.LayoutChild(child, row)
	}
	if r.host != nil {
		r.host.measured(sz.H)
		if r.Attached() && r.Focused() == nil {
			r.host.lostFocus()
		}
	}
	r.place()
	return sz
}

// inView reports whether a child at offset y shows.
func (r *renderTreeBody) inView(y float32) bool { return y+treeRowHeight > 0 && y < r.Size().H }

// ChildClip implements core.Clipper.
func (r *renderTreeBody) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderTreeBody) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	for _, c := range r.children {
		if o := c.Offset(); r.inView(o.Y) {
			c.Paint(cv, off.Add(o))
		}
	}
	if d := r.drop; d.shown {
		top := r.position.Value()
		if d.into {
			y := off.Y + float32(float64(d.row)*treeRowHeight-top)
			pill := geom.RRectAll(geom.Rect{X: off.X + treeInset, Y: y + 1, W: sz.W - 2*treeInset, H: treeRowHeight - 2}, treeRadius)
			cv.DrawRRect(pill, paint.Paint{Color: r.bar.WithAlpha(treeIntoOpacity), Stroke: &paint.Stroke{Width: treeRing, Color: r.bar}})
		} else {
			y := off.Y + float32(d.line-top)
			x := off.X + d.x
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: x, Y: y - treeRing/2, W: off.X + sz.W - treeInset - x, H: treeRing}, treeRing/2),
				paint.Paint{Color: r.bar})
			dot := geom.Rect{X: x - treeDropDot, Y: y - treeDropDot, W: 2 * treeDropDot, H: 2 * treeDropDot}
			cv.DrawRRect(geom.RRectAll(dot, treeDropDot), paint.Paint{Color: r.bar})
		}
	}
	cv.Restore()
}

// The look of the drop indicator.
const (
	treeIntoOpacity = 0.12
	treeDropDot     = 3
)

// HitTest implements core.RenderObject. A row scrolled out of the view is not
// hit, though it exists.
func (r *renderTreeBody) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
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
// below the rows clears the selection.
func (r *renderTreeBody) HandlePointer(e *core.PointerEvent) {
	if r.host == nil {
		return
	}
	switch {
	case e.Kind == core.PointerScroll && e.Scroll.Y != 0:
		r.host.Wheel(-e.Scroll.Y)
		e.Handled = true
	case e.Kind == core.PointerDown && e.Button == input.ButtonPrimary:
		r.host.bodyPress()
		e.Handled = true
	}
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderTreeBody) FocusWithinChanged(within bool) {
	if r.host != nil {
		r.host.setActive(within)
	}
}

// Semantics implements core.RenderObject. Only the rows in view speak, and
// the row of the focus wherever it is.
func (r *renderTreeBody) Semantics(sb *core.SemanticsBuilder) {
	for _, c := range r.children {
		if row, ok := c.(*renderTreeRow); ok && (r.inView(c.Offset().Y) || row.focusable) {
			sb.Visit(c)
		}
	}
}
