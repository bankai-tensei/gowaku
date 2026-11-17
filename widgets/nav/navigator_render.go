package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// navigatorLayout is the widget of renderNavigator.
type navigatorLayout struct {
	state    *navigatorState
	children []core.Widget
}

// Key implements core.Widget.
func (navigatorLayout) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l navigatorLayout) CreateRenderObject() core.RenderObject {
	r := &renderNavigator{}
	l.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (l navigatorLayout) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderNavigator)
	r.nav = l.state
	l.state.render = r
	for _, e := range l.state.entries {
		if e.arriving {
			// The new page is laid out below, but the Navigator has to hear
			// of it to hand it the focus.
			r.MarkNeedsLayout()
			break
		}
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (l navigatorLayout) ChildWidgets() []core.Widget { return l.children }

// renderNavigator lays its pages on top of each other at its own size, paints
// those that show and the flights of the heroes over them, and goes back and
// forward on request.
type renderNavigator struct {
	core.RenderBase
	nav      *navigatorState
	children []core.RenderObject
}

var _ core.NavigationTarget = (*renderNavigator)(nil)

// SetChildren implements core.MultiChildRenderObject.
func (r *renderNavigator) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderNavigator) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderNavigator) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderNavigator) Layout(c geom.Constraints) geom.Size {
	sz := c.Biggest()
	if sz.W >= geom.Inf || sz.H >= geom.Inf {
		panic("gowaku: Navigator has an unbounded size, as inside a Scroll, a Row or a Column; give it one with a SizedBox or Expanded")
	}
	inner := geom.Tight(sz)
	for _, child := range r.children {
		core.LayoutChild(child, inner)
		child.SetOffset(geom.Offset{})
	}
	r.nav.laidOut()
	return sz
}

// Paint implements core.RenderObject.
func (r *renderNavigator) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
	r.nav.paintFlights(cv, off)
}

// HandleNavigation implements core.NavigationTarget.
func (r *renderNavigator) HandleNavigation(forward bool) bool {
	if forward {
		return r.nav.ctl.Forward()
	}
	return r.nav.pop(true)
}

// pageSlot holds one page of a Navigator.
type pageSlot struct {
	entry *navEntry
	child core.Widget
}

// Key implements core.Widget.
func (pageSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p pageSlot) CreateRenderObject() core.RenderObject {
	r := &renderPageSlot{}
	p.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p pageSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderPageSlot)
	r.entry = p.entry
	p.entry.slot = r
}

// ChildWidget implements core.SingleChildWidget.
func (p pageSlot) ChildWidget() core.Widget { return p.child }

// renderPageSlot shows a page where its entry says, and hides it from the
// pointer, the keyboard and assistive technology unless it is the page on top.
// Its child is the RepaintBoundary of the page, which fades as one picture.
type renderPageSlot struct {
	layout.RenderProxy
	entry *navEntry
	// shift is the offset painted last, to see whether it moved.
	shift float32
}

// Hidden implements core.Hider.
func (r *renderPageSlot) Hidden() bool { return !r.entry.active }

// moved takes the shift and the opacity of a frame of the standard transition.
func (r *renderPageSlot) moved(opacity float32) {
	if b, ok := r.ChildAt(0).(*core.RenderRepaintBoundary); ok {
		if r.entry.spec.route.Transition != nil {
			opacity = 1
		}
		b.SetOpacity(opacity)
	}
	if r.shift != r.entry.shift {
		r.MarkNeedsPaint()
	}
}

// ChildClip implements core.Clipper.
func (r *renderPageSlot) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), r.entry.shift != 0
}

// Paint implements core.RenderObject.
func (r *renderPageSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	r.shift = r.entry.shift
	if !r.entry.painted {
		return
	}
	if r.shift == 0 {
		r.RenderProxy.Paint(cv, off)
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off.Add(geom.Offset{X: r.shift}))
	cv.Restore()
}
