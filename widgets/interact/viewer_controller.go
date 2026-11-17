package interact

import "github.com/bankai-tensei/gowaku/geom"

// ViewerController reads and moves the view of an InteractiveViewer. Keep it in
// a State and hand its address to the viewer; the zero value shows the child at
// a scale of 1 from the top left. It holds the transform, so a viewer built
// again with the same controller shows the same view, and the transform can be
// set before the viewer exists.
//
// Calls that move the view run OnChange, so make them from callbacks and
// InitState, not from Build. Fit and ZoomToRect need the size of the viewer:
// made before its first layout, they wait for it and then move at once. The
// last viewer built with a controller takes it.
type ViewerController struct {
	// scale is the zoom, where zero means 1, and offset the place of the top
	// left of the child in the viewer.
	scale  float32
	offset geom.Offset
	state  *viewerState
	// pending is a move that waits for the first layout.
	pending viewerRequest
}

// viewerRequest is a move of the controller that needs the size of the viewer.
type viewerRequest struct {
	kind viewerRequestKind
	rect geom.Rect
}

type viewerRequestKind uint8

const (
	requestNone viewerRequestKind = iota
	requestFit
	requestRect
)

// Scale returns the zoom: 2 shows the child twice as large.
func (c *ViewerController) Scale() float32 {
	if c.scale == 0 {
		return 1
	}
	return c.scale
}

// Offset returns where the top left of the child lies in the viewer, in
// logical pixels.
func (c *ViewerController) Offset() geom.Offset { return c.offset }

// Transform returns the transform from the coordinates of the child to those
// of the viewer.
func (c *ViewerController) Transform() geom.Affine {
	s := c.Scale()
	return geom.Affine{A: s, D: s, E: c.offset.X, F: c.offset.Y}
}

// ToChild returns the point of the child shown at p, a point of the viewer.
func (c *ViewerController) ToChild(p geom.Offset) geom.Offset {
	s := c.Scale()
	return geom.Offset{X: (p.X - c.offset.X) / s, Y: (p.Y - c.offset.Y) / s}
}

// FromChild returns where p, a point of the child, shows in the viewer.
func (c *ViewerController) FromChild(p geom.Offset) geom.Offset {
	s := c.Scale()
	return geom.Offset{X: c.offset.X + p.X*s, Y: c.offset.Y + p.Y*s}
}

// Set moves the view to scale, within the limits of the viewer, with the top
// left of the child at offset, within its boundary. Animate glides there,
// unless MediaQuery asks to reduce motion. A scale of zero or less panics.
func (c *ViewerController) Set(scale float32, offset geom.Offset, animate bool) {
	if scale <= 0 {
		panic("gowaku: ViewerController.Set needs a positive scale")
	}
	c.pending = viewerRequest{}
	if c.state == nil {
		c.scale, c.offset = scale, offset
		return
	}
	c.state.moveTo(scale, offset, animate)
}

// PanBy moves the child by d, in logical pixels of the viewer.
func (c *ViewerController) PanBy(d geom.Offset, animate bool) {
	c.Set(c.Scale(), c.offset.Add(d), animate)
}

// ZoomBy scales the view by factor about focus, a point of the viewer, which
// stays in place unless a limit stops it. A factor of zero or less panics.
func (c *ViewerController) ZoomBy(factor float32, focus geom.Offset, animate bool) {
	if factor <= 0 {
		panic("gowaku: ViewerController.ZoomBy needs a positive factor")
	}
	c.pending = viewerRequest{}
	if c.state == nil {
		s0 := c.Scale()
		s1 := s0 * factor
		c.offset = geom.Offset{X: focus.X - (focus.X-c.offset.X)/s0*s1, Y: focus.Y - (focus.Y-c.offset.Y)/s0*s1}
		c.scale = s1
		return
	}
	c.state.zoomAt(factor, focus, animate)
}

// Reset goes back to a scale of 1 with the child at the top left, or centered
// along an axis where a bounded child is smaller than the viewer.
func (c *ViewerController) Reset(animate bool) { c.Set(1, geom.Offset{}, animate) }

// Fit shows the whole child, as large as fits within the viewer and its
// BoundaryMargin, centered.
func (c *ViewerController) Fit(animate bool) {
	if c.state == nil || !c.state.laidOut {
		c.pending = viewerRequest{kind: requestFit}
		return
	}
	c.pending = viewerRequest{}
	c.state.fit(animate)
}

// ZoomToRect shows r, a rectangle in the coordinates of the child, as large as
// fits in the viewer, centered. A rectangle without area panics.
func (c *ViewerController) ZoomToRect(r geom.Rect, animate bool) {
	if r.W <= 0 || r.H <= 0 {
		panic("gowaku: ViewerController.ZoomToRect needs a rectangle with an area")
	}
	if c.state == nil || !c.state.laidOut {
		c.pending = viewerRequest{kind: requestRect, rect: r}
		return
	}
	c.pending = viewerRequest{}
	c.state.zoomToRect(r, animate)
}

// Stop ends a fling or an animated move where it is.
func (c *ViewerController) Stop() {
	if c.state != nil {
		c.state.stop()
	}
}

// Moving reports whether a fling or an animated move is under way.
func (c *ViewerController) Moving() bool {
	return c.state != nil && c.state.motion.kind != motionNone
}
