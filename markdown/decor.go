package markdown

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// quoteBarWidth is the width of the bar at the left of a block quote.
const quoteBarWidth = 3

// quoteBox moves its child in by indent and draws a bar the height of the
// child at its left, as a block quote shows.
type quoteBox struct {
	indent float32
	color  geom.Color
	child  core.Widget
}

// Key implements core.Widget.
func (quoteBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (q quoteBox) CreateRenderObject() core.RenderObject {
	return &renderQuote{indent: q.indent, color: q.color}
}

// UpdateRenderObject implements core.RenderWidget.
func (q quoteBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderQuote)
	if r.indent != q.indent {
		r.indent = q.indent
		r.MarkNeedsLayout()
	}
	if r.color != q.color {
		r.color = q.color
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (q quoteBox) ChildWidget() core.Widget { return q.child }

type renderQuote struct {
	core.RenderBase
	indent float32
	color  geom.Color
	child  core.RenderObject
}

// SetChild implements core.SingleChildRenderObject.
func (r *renderQuote) SetChild(child core.RenderObject) { r.child = child }

// ChildCount implements core.RenderObject.
func (r *renderQuote) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements core.RenderObject.
func (r *renderQuote) ChildAt(int) core.RenderObject { return r.child }

// Layout implements core.RenderObject.
func (r *renderQuote) Layout(c geom.Constraints) geom.Size {
	if r.child == nil {
		return c.Constrain(geom.Size{W: r.indent})
	}
	sz := core.LayoutChild(r.child, c.Deflate(geom.Insets{Left: r.indent}))
	r.child.SetOffset(geom.Offset{X: r.indent})
	return c.Constrain(geom.Size{W: sz.W + r.indent, H: sz.H})
}

// Paint implements core.RenderObject.
func (r *renderQuote) Paint(cv *paint.Canvas, off geom.Offset) {
	bar := geom.Rect{X: off.X, Y: off.Y, W: quoteBarWidth, H: r.Size().H}
	cv.DrawRRect(geom.RRectAll(bar, quoteBarWidth/2), paint.Paint{Color: r.color})
	if r.child != nil {
		r.child.Paint(cv, off.Add(r.child.Offset()))
	}
}

// bullet is the marker of an item of a bullet list: a dot, a ring or a
// square by the depth of the list, in the middle of the first line.
type bullet struct {
	depth             int
	width, line, size float32
	color             geom.Color
}

// Key implements core.Widget.
func (bullet) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b bullet) CreateRenderObject() core.RenderObject { return &renderBullet{b: b} }

// UpdateRenderObject implements core.RenderWidget.
func (b bullet) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderBullet)
	if r.b != b {
		r.b = b
		r.MarkNeedsLayout()
	}
}

type renderBullet struct {
	core.RenderBase
	b    bullet
	ring paint.Stroke
}

// Layout implements core.RenderObject.
func (r *renderBullet) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: r.b.width, H: r.b.line})
}

// Paint implements core.RenderObject.
func (r *renderBullet) Paint(cv *paint.Canvas, off geom.Offset) {
	em := r.b.size
	if em <= 0 {
		em = defaultTextSize
	}
	d := evenPixels(em * 0.36)
	// The marker sits just before the text, where a reader looks for it.
	x := off.X + r.Size().W - d - em*0.6
	y := off.Y + r.b.line/2 - d/2
	box := geom.Rect{X: x, Y: y, W: d, H: d}
	switch r.b.depth % 3 {
	case 0:
		cv.DrawRRect(geom.RRectAll(box, d/2), paint.Paint{Color: r.b.color})
	case 1:
		r.ring = paint.Stroke{Width: max(1, em/12), Color: r.b.color}
		cv.DrawRRect(geom.RRectAll(box, d/2), paint.Paint{Stroke: &r.ring})
	default:
		cv.DrawRRect(geom.RRect{Rect: box}, paint.Paint{Color: r.b.color})
	}
}

// checkMark is the read-only checkbox of a task item.
type checkMark struct {
	checked           bool
	label             string
	width, line, size float32
	box, fill, mark   geom.Color
	radius            float32
}

// Key implements core.Widget.
func (checkMark) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (m checkMark) CreateRenderObject() core.RenderObject { return &renderCheck{m: m} }

// UpdateRenderObject implements core.RenderWidget.
func (m checkMark) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderCheck)
	if r.m != m {
		r.m = m
		r.tick.Reset()
		r.MarkNeedsLayout()
	}
}

type renderCheck struct {
	core.RenderBase
	m      checkMark
	tick   paint.Path
	stroke paint.Stroke
}

// Layout implements core.RenderObject.
func (r *renderCheck) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: r.m.width, H: r.m.line})
}

// boxRect returns the square of the checkbox, relative to the object.
func (r *renderCheck) boxRect() geom.Rect {
	s := r.m.size
	return geom.Rect{X: r.Size().W - s - s*0.5, Y: (r.m.line - s) / 2, W: s, H: s}
}

// Paint implements core.RenderObject.
func (r *renderCheck) Paint(cv *paint.Canvas, off geom.Offset) {
	box := r.boxRect().Translate(off)
	if !r.m.checked {
		r.stroke = paint.Stroke{Width: max(1, r.m.size/10), Color: r.m.box}
		cv.DrawRRect(geom.RRectAll(box, r.m.radius), paint.Paint{Stroke: &r.stroke})
		return
	}
	cv.DrawRRect(geom.RRectAll(box, r.m.radius), paint.Paint{Color: r.m.fill})
	if r.tick.Empty() {
		// The tick is made once, relative to the box, and moved with the
		// canvas.
		s := r.m.size
		r.tick.MoveTo(s*0.24, s*0.52)
		r.tick.LineTo(s*0.42, s*0.70)
		r.tick.LineTo(s*0.76, s*0.32)
	}
	r.stroke = paint.Stroke{Width: max(1.5, r.m.size/8), Color: r.m.mark}
	cv.Save()
	cv.Translate(box.X, box.Y)
	cv.DrawPath(&r.tick, paint.Paint{Stroke: &r.stroke})
	cv.Restore()
}

// Semantics implements core.RenderObject. The item is a checkbox that the
// reader cannot change.
func (r *renderCheck) Semantics(sb *core.SemanticsBuilder) {
	state := semantics.ReadOnly
	if r.m.checked {
		state |= semantics.Checked
	}
	sb.Open(semantics.Node{Role: semantics.RoleCheckbox, Label: r.m.label, State: state}, r.Size())
	sb.Close()
}
