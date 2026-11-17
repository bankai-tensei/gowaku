package interact

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// viewerBox is the render widget of an InteractiveViewer.
type viewerBox struct {
	s           *viewerState
	constrained bool
	child       core.Widget
}

// Key implements core.Widget.
func (viewerBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v viewerBox) CreateRenderObject() core.RenderObject {
	r := &renderViewer{s: v.s, constrained: v.constrained}
	v.s.ro = r
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v viewerBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderViewer)
	r.s = v.s
	v.s.ro = r
	if r.constrained != v.constrained {
		r.constrained = v.constrained
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (v viewerBox) ChildWidget() core.Widget { return v.child }

// renderViewer lays its child out once and paints, hit tests and describes it
// through the transform of the controller, which its state moves. As a
// core.Transformer, it lets the pointer, the IME and the semantics tree follow
// the picture.
type renderViewer struct {
	core.RenderBase
	s           *viewerState
	constrained bool
	child       core.RenderObject
}

var (
	_ core.Transformer    = (*renderViewer)(nil)
	_ core.PointerTarget  = (*renderViewer)(nil)
	_ core.FocusTarget    = (*renderViewer)(nil)
	_ core.SemanticsActor = (*renderViewer)(nil)
)

// SetChild implements core.SingleChildRenderObject.
func (r *renderViewer) SetChild(child core.RenderObject) { r.child = child }

// ChildCount implements core.RenderObject.
func (r *renderViewer) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements core.RenderObject.
func (r *renderViewer) ChildAt(int) core.RenderObject { return r.child }

// ChildTransform implements core.Transformer.
func (r *renderViewer) ChildTransform() geom.Affine {
	c := r.s.ctl
	if c.Scale() == 1 && c.offset == (geom.Offset{}) {
		return geom.Affine{}
	}
	return c.Transform()
}

// Layout implements core.RenderObject. The viewer takes all the room it is
// given; the child takes the room of the viewer if constrained, and any it
// wants otherwise.
func (r *renderViewer) Layout(c layout.Constraints) geom.Size {
	if c.MaxW >= geom.Inf || c.MaxH >= geom.Inf {
		panic("gowaku: InteractiveViewer has an unbounded width or height, as inside a Scroll, Row or Column; give it a size with a SizedBox or an Expanded")
	}
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	var content geom.Size
	if r.child != nil {
		inner := layout.Constraints{MaxW: geom.Inf, MaxH: geom.Inf}
		if r.constrained {
			inner = layout.Constraints{MaxW: sz.W, MaxH: sz.H}
		}
		content = core.LayoutChild(r.child, inner)
		if content.W >= geom.Inf || content.H >= geom.Inf {
			panic("gowaku: the child of an InteractiveViewer took an unbounded size; give it one with a SizedBox, or set Constrained")
		}
	}
	r.s.laidOutAt(sz, content)
	return sz
}

// ChildClip implements core.Clipper.
func (r *renderViewer) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderViewer) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child == nil {
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	if m := r.ChildTransform(); !m.IsIdentity() {
		cv.Translate(off.X, off.Y)
		cv.Transform(m)
		cv.Translate(-off.X, -off.Y)
	}
	r.child.Paint(cv, off.Add(r.child.Offset()))
	cv.Restore()
}

// HitTest implements core.RenderObject. The position goes back through the
// transform to reach the child where it shows; the viewer itself takes the
// rest of its box, to pan.
func (r *renderViewer) HitTest(pos geom.Offset, result *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	if c := r.child; c != nil {
		if h, hides := c.(core.Hider); !hides || !h.Hidden() {
			// The scale is never zero, so the transform always inverts.
			inverse, _ := r.s.ctl.Transform().Invert()
			c.HitTest(inverse.Apply(pos).Sub(c.Offset()), result)
		}
	}
	result.Path = append(result.Path, r)
	return true
}

// HandlePointer implements core.PointerTarget.
func (r *renderViewer) HandlePointer(e *core.PointerEvent) { r.s.pointer(e) }

// CanFocus implements core.FocusTarget.
func (r *renderViewer) CanFocus() bool { return r.s.Widget().Keys != ViewerKeysNone }

// HandleKey implements core.FocusTarget.
func (r *renderViewer) HandleKey(e *core.KeyEvent) bool { return r.s.key(e) }

// HandleText implements core.FocusTarget.
func (r *renderViewer) HandleText(t string) { r.s.text(t) }

// HandleComposition implements core.FocusTarget.
func (r *renderViewer) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderViewer) FocusChanged(focused bool) { r.s.showFocus(focused && r.FocusVisible()) }

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderViewer) FocusVisibleChanged(visible bool) { r.s.showFocus(visible) }

// Semantics implements core.RenderObject: a group named by Label whose value
// is the zoom as a percentage, which a screen reader increases and decreases.
func (r *renderViewer) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{
		Role:    semantics.RoleGroup,
		Label:   r.s.Widget().Label,
		Value:   strconv.Itoa(int(r.s.ctl.Scale()*100+0.5)) + "%",
		Actions: semantics.ActionIncrease | semantics.ActionDecrease,
	}, r.Size())
	if r.child != nil {
		sb.Visit(r.child)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderViewer) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	switch a {
	case semantics.ActionIncrease:
		r.s.zoomCenter(viewerKeyZoom)
	case semantics.ActionDecrease:
		r.s.zoomCenter(1 / viewerKeyZoom)
	default:
		return false
	}
	return true
}
