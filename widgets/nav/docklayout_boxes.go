package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// The small render objects of a DockLayout: a quarter turn, the row of a tab
// strip and its button, a line of text that keeps its layout, and the target
// of the keys.

// dockTurn is the widget of renderDockTurn.
type dockTurn struct {
	clockwise bool
	child     core.Widget
}

// Key implements core.Widget.
func (dockTurn) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t dockTurn) CreateRenderObject() core.RenderObject {
	return &renderDockTurn{clockwise: t.clockwise}
}

// UpdateRenderObject implements core.RenderWidget.
func (t dockTurn) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDockTurn)
	if r.clockwise != t.clockwise {
		r.clockwise = t.clockwise
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t dockTurn) ChildWidget() core.Widget { return t.child }

// renderDockTurn lays its child out on its side and draws it turned a
// quarter, clockwise or not, taking the turned size; the pointer and
// assistive technology follow the turn.
type renderDockTurn struct {
	layout.RenderProxy
	clockwise bool
}

// Layout implements core.RenderObject.
func (r *renderDockTurn) Layout(c geom.Constraints) geom.Size {
	if r.ChildCount() == 0 {
		return c.Constrain(geom.Size{})
	}
	child := r.ChildAt(0)
	sz := core.LayoutChild(child, geom.Constraints{MinW: c.MinH, MaxW: c.MaxH, MinH: c.MinW, MaxH: c.MaxW})
	child.SetOffset(geom.Offset{})
	return c.Constrain(geom.Size{W: sz.H, H: sz.W})
}

// ChildTransform implements core.Transformer.
func (r *renderDockTurn) ChildTransform() geom.Affine {
	if r.ChildCount() == 0 {
		return geom.Affine{}
	}
	sz := r.ChildAt(0).Size()
	if r.clockwise {
		return geom.Affine{B: 1, C: -1, E: sz.H}
	}
	return geom.Affine{B: -1, C: 1, F: sz.W}
}

// Paint implements core.RenderObject.
func (r *renderDockTurn) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.ChildCount() == 0 {
		return
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.Transform(r.ChildTransform())
	cv.Translate(-off.X, -off.Y)
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// HitTest implements core.RenderObject. The position turns back, so that the
// child is hit where it shows.
func (r *renderDockTurn) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	if r.ChildCount() == 0 {
		return false
	}
	inverse, ok := r.ChildTransform().Invert()
	if !ok {
		return false
	}
	child := r.ChildAt(0)
	local := inverse.Apply(pos)
	sz := child.Size()
	if local.X < 0 || local.Y < 0 || local.X >= sz.W || local.Y >= sz.H {
		return false
	}
	child.HitTest(local, res)
	res.Path = append(res.Path, r)
	return true
}

// dockStripRow is the widget of renderDockStripRow.
type dockStripRow struct {
	line geom.Color
	// children are the tab bar and the button after it.
	children []core.Widget
}

// Key implements core.Widget.
func (dockStripRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (d dockStripRow) CreateRenderObject() core.RenderObject {
	r := &renderDockStripRow{}
	d.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (d dockStripRow) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDockStripRow)
	if r.line != d.line {
		r.line = d.line
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (d dockStripRow) ChildWidgets() []core.Widget { return d.children }

// renderDockStripRow puts a button at the end of the tab strip of a group,
// over the hairline the strip draws along its bottom.
type renderDockStripRow struct {
	core.RenderBase
	line     geom.Color
	children []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderDockStripRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderDockStripRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderDockStripRow) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderDockStripRow) Layout(c geom.Constraints) geom.Size {
	sz := c.Constrain(geom.Size{W: c.MaxW, H: tabBarHeight})
	if len(r.children) < 2 {
		return sz
	}
	button := core.LayoutChild(r.children[1], geom.Constraints{MaxW: sz.W, MaxH: sz.H})
	bar := max(sz.W-button.W, 0)
	core.LayoutChild(r.children[0], geom.Tight(geom.Size{W: bar, H: sz.H}))
	r.children[0].SetOffset(geom.Offset{})
	r.children[1].SetOffset(geom.Offset{X: bar})
	return sz
}

// Paint implements core.RenderObject.
func (r *renderDockStripRow) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if len(r.children) == 2 {
		x := r.children[1].Offset().X
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X + x, Y: off.Y + sz.H - 1, W: sz.W - x, H: 1}}, paint.Paint{Color: r.line})
	}
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// dockKeys takes the keys of a DockLayout from anywhere in the window, and
// tells the layout whether the focus is within it.
type dockKeys struct {
	onShortcut func(e *core.KeyEvent) bool
	onFocus    func(within bool)
	child      core.Widget
}

// Key implements core.Widget.
func (dockKeys) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (k dockKeys) CreateRenderObject() core.RenderObject {
	r := &renderDockKeys{}
	k.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (k dockKeys) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDockKeys)
	r.OnShortcut, r.onFocus = k.onShortcut, k.onFocus
}

// ChildWidget implements core.SingleChildWidget.
func (k dockKeys) ChildWidget() core.Widget { return k.child }

// renderDockKeys is a shortcut target that hears the focus come and go.
type renderDockKeys struct {
	layout.RenderShortcuts
	onFocus func(within bool)
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderDockKeys) FocusWithinChanged(within bool) {
	if r.onFocus != nil {
		r.onFocus(within)
	}
}

// dockLine is the widget of renderDockLine.
type dockLine struct{ child core.Widget }

// Key implements core.Widget.
func (dockLine) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (dockLine) CreateRenderObject() core.RenderObject { return &renderDockLine{} }

// UpdateRenderObject implements core.RenderWidget.
func (dockLine) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (d dockLine) ChildWidget() core.Widget { return d.child }

// renderDockLine lays out a line of text at its own width, whatever the
// width it is given, and cuts it there: the title of a flyout keeps its layout
// while the flyout is sized, which would shape it again at every width.
type renderDockLine struct{ layout.RenderProxy }

// Layout implements core.RenderObject.
func (r *renderDockLine) Layout(c geom.Constraints) geom.Size {
	if r.ChildCount() == 0 {
		return c.Constrain(geom.Size{})
	}
	child := r.ChildAt(0)
	sz := core.LayoutChild(child, geom.Constraints{MaxW: geom.Inf, MaxH: c.MaxH})
	out := c.Constrain(geom.Size{W: kit.FiniteOr(c.MaxW, sz.W), H: sz.H})
	child.SetOffset(geom.Offset{Y: geom.Floor32((out.H - sz.H) / 2)})
	return out
}

// ChildClip implements core.Clipper.
func (r *renderDockLine) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderDockLine) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}
