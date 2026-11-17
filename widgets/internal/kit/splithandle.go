package kit

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

const (
	// SplitHit is the width of the strip that takes the pointer around a
	// divider: the line is one pixel, which no hand can hit.
	SplitHit = 9
	// splitRingWidth is the width of the pale band a divider shows in focus,
	// the focus ring of DESIGN §5 laid along the line.
	splitRingWidth = 5
)

// SplitHandle is the strip that drags the divider between two panes: the
// widget of RenderSplitHandle.
type SplitHandle struct {
	// Axis is the direction the handle moves in: Horizontal for a vertical
	// line between panes side by side.
	Axis layout.Axis
	// Line is the color of the hairline, and Ring that of the band around it,
	// transparent out of focus.
	Line, Ring geom.Color
	OnHover    func(inside bool)
	OnStart    func()
	// OnDrag reports how far the pointer is from where it went down, along
	// axis, in logical pixels of the window.
	OnDrag  func(delta float32)
	OnEnd   func()
	OnReset func()
}

// Key implements core.Widget.
func (SplitHandle) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (h SplitHandle) CreateRenderObject() core.RenderObject {
	r := &RenderSplitHandle{}
	h.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (h SplitHandle) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*RenderSplitHandle)
	if r.axis != h.Axis || r.line != h.Line || r.ring != h.Ring {
		r.axis, r.line, r.ring = h.Axis, h.Line, h.Ring
		r.MarkNeedsPaint()
	}
	r.onHover, r.onStart, r.onDrag, r.onEnd, r.onReset = h.OnHover, h.OnStart, h.OnDrag, h.OnEnd, h.OnReset
}

// RenderSplitHandle is the strip around a divider: it fills the box its parent
// gives it, draws the hairline down its middle, and turns a drag into the
// distance the pointer went. Positions are taken in the window, since the
// strip itself moves under the pointer as the divider follows it.
type RenderSplitHandle struct {
	core.RenderBase
	axis       layout.Axis
	line, ring geom.Color
	onHover    func(inside bool)
	onStart    func()
	onDrag     func(delta float32)
	onEnd      func()
	onReset    func()
	// dragging says a press is held, and from is where it went down in the
	// window, along axis.
	dragging bool
	from     float32
}

// Layout implements core.RenderObject.
func (r *RenderSplitHandle) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: FiniteOr(c.MaxW, c.MinW), H: FiniteOr(c.MaxH, c.MinH)})
}

// Paint implements core.RenderObject.
func (r *RenderSplitHandle) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if r.axis == layout.Horizontal {
		mid := off.X + geom.Floor32(sz.W/2)
		if r.ring.A > 0 {
			band := geom.Rect{X: mid + 0.5 - splitRingWidth/2, Y: off.Y, W: splitRingWidth, H: sz.H}
			cv.DrawRRect(geom.RRectAll(band, splitRingWidth/2), paint.Paint{Color: r.ring})
		}
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: mid, Y: off.Y, W: DividerThickness, H: sz.H}}, paint.Paint{Color: r.line})
		return
	}
	mid := off.Y + geom.Floor32(sz.H/2)
	if r.ring.A > 0 {
		band := geom.Rect{X: off.X, Y: mid + 0.5 - splitRingWidth/2, W: sz.W, H: splitRingWidth}
		cv.DrawRRect(geom.RRectAll(band, splitRingWidth/2), paint.Paint{Color: r.ring})
	}
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: mid, W: sz.W, H: DividerThickness}}, paint.Paint{Color: r.line})
}

// along returns the coordinate of p along the axis of the handle.
func (r *RenderSplitHandle) along(p geom.Offset) float32 {
	if r.axis == layout.Horizontal {
		return p.X
	}
	return p.Y
}

// HandlePointer implements core.PointerTarget.
func (r *RenderSplitHandle) HandlePointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		if e.Clicks == 2 {
			if r.onReset != nil {
				r.onReset()
			}
			return
		}
		r.dragging, r.from = true, r.along(core.LocalToGlobal(r, e.Pos))
		if r.onStart != nil {
			r.onStart()
		}
	case core.PointerMove:
		if r.dragging && r.onDrag != nil {
			r.onDrag(r.along(core.LocalToGlobal(r, e.Pos)) - r.from)
		}
	case core.PointerUp, core.PointerCancel:
		if r.dragging {
			r.dragging = false
			if r.onEnd != nil {
				r.onEnd()
			}
		}
	}
}

// HandleHover implements core.HoverTarget.
func (r *RenderSplitHandle) HandleHover(inside bool) {
	if r.onHover != nil {
		r.onHover(inside)
	}
}

// Line returns the color of the hairline.
func (r *RenderSplitHandle) Line() geom.Color { return r.line }

// MouseCursor implements core.CursorSource.
func (r *RenderSplitHandle) MouseCursor() input.Cursor {
	if r.axis == layout.Horizontal {
		return input.CursorColResize
	}
	return input.CursorRowResize
}

// PaneSlot holds one pane of a split, which the split hides while it is
// folded: the pointer, Tab and assistive technology then pass it by.
type PaneSlot struct {
	Child core.Widget
}

// Key implements core.Widget.
func (PaneSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (PaneSlot) CreateRenderObject() core.RenderObject { return &RenderPaneSlot{} }

// UpdateRenderObject implements core.RenderWidget.
func (PaneSlot) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (p PaneSlot) ChildWidget() core.Widget { return p.Child }

// RenderPaneSlot clips its pane to its box, since a pane narrower than its
// content would otherwise draw over its neighbor.
type RenderPaneSlot struct {
	layout.RenderProxy
	// Folded hides the pane; the split that holds the slot sets it.
	Folded bool
}

// Hidden implements core.Hider.
func (r *RenderPaneSlot) Hidden() bool { return r.Folded }

// ChildClip implements core.Clipper.
func (r *RenderPaneSlot) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *RenderPaneSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Folded {
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}
