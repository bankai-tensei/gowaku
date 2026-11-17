package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// ClipRect cuts what its child paints to its own box: a child larger than the
// box, a picture moved by a Transform or a row that overflows shows only the
// part inside. The pointer reaches the child only inside the box, and
// gowakutest.Tester.Warnings leaves out what lies wholly outside it. A
// rectangle costs nothing to clip.
//
// The box is the child's size within the constraints, so put the size
// outside, as in SizedBox{Width: 120, Height: 80, Child: ClipRect{Child: ...}}.
// Like Align or Padding, ClipRect draws nothing itself: where the child is not
// hit, the pointer goes on to what is below.
type ClipRect struct {
	Child core.Widget
}

// Key implements core.Widget.
func (ClipRect) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (ClipRect) CreateRenderObject() core.RenderObject { return &layout.RenderClip{} }

// UpdateRenderObject implements core.RenderWidget.
func (ClipRect) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderClip)
	if r.Radius != 0 {
		r.Radius = 0
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (c ClipRect) ChildWidget() core.Widget { return c.Child }

// ClipRRect cuts what its child paints to its box with rounded corners, as an
// image or a list in a card is cut to the card. A press on a cut-off corner
// does not reach the child, and gowakutest.Tester.Warnings leaves out what lies
// wholly outside the box. Rounded corners cost a stencil pass; ClipRect is
// free.
//
// Radius rounds the four corners. Zero takes the corners of a card,
// theme.Radii.Group, from the theme, so that the clip matches a Container with
// that Radius.
type ClipRRect struct {
	core.StatelessBase
	Radius float32
	Child  core.Widget
}

// Build implements core.StatelessWidget.
func (c ClipRRect) Build(ctx core.BuildContext) core.Widget {
	radius := c.Radius
	if radius == 0 {
		radius = ThemeOf(ctx).Radius.Group
	}
	return clipBox{radius: radius, child: c.Child}
}

// clipBox is the render widget of ClipRRect.
type clipBox struct {
	radius float32
	child  core.Widget
}

// Key implements core.Widget.
func (clipBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (c clipBox) CreateRenderObject() core.RenderObject {
	return &layout.RenderClip{Radius: c.radius}
}

// UpdateRenderObject implements core.RenderWidget.
func (c clipBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderClip)
	if r.Radius != c.radius {
		r.Radius = c.radius
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (c clipBox) ChildWidget() core.Widget { return c.child }
