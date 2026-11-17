package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// RenderClip cuts what its child paints to its own box, with corners rounded
// by Radius. The pointer reaches the child only inside that shape: a press on
// a cut-off corner, or outside the box, is not a press on the child. Like the
// other objects that only paint around their child, it is hit only where the
// child is (ADR 0142).
type RenderClip struct {
	RenderProxy
	// Radius rounds the four corners. Zero clips to the rectangle, which costs
	// nothing; rounded corners cost a stencil pass (paint.Canvas.ClipRRect).
	Radius float32
}

// ChildClip implements core.Clipper. It reports the box, also for rounded
// corners: the tools that ask (gowakutest.Tester.Warnings) only need to know
// what lies wholly outside.
func (r *RenderClip) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// HitTest implements core.RenderObject. It leaves out the cut-off corners.
func (r *RenderClip) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	if r.Radius > 0 && !insideRRect(geom.RRectAll(geom.RectFromSize(r.Size()), r.Radius), pos) {
		return false
	}
	return r.RenderProxy.HitTest(pos, res)
}

// Paint implements core.RenderObject.
func (r *RenderClip) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child == nil {
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRRect(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, r.Radius))
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// insideRRect reports whether pos is inside rr, corners included.
func insideRRect(rr geom.RRect, pos geom.Offset) bool {
	r := rr.Rect
	if !r.Contains(pos) {
		return false
	}
	limit := min(r.W, r.H) / 2
	return cornerOK(pos, r.X, r.Y, min(rr.TopLeft, limit), 1, 1) &&
		cornerOK(pos, r.X+r.W, r.Y, min(rr.TopRight, limit), -1, 1) &&
		cornerOK(pos, r.X+r.W, r.Y+r.H, min(rr.BottomRight, limit), -1, -1) &&
		cornerOK(pos, r.X, r.Y+r.H, min(rr.BottomLeft, limit), 1, -1)
}

// cornerOK reports whether pos is not cut off by the corner of radius at
// (x, y), whose box runs in the directions sx and sy.
func cornerOK(pos geom.Offset, x, y, radius, sx, sy float32) bool {
	if radius <= 0 {
		return true
	}
	cx, cy := x+sx*radius, y+sy*radius
	dx, dy := (pos.X-cx)*sx, (pos.Y-cy)*sy
	if dx >= 0 || dy >= 0 {
		// Not in the corner's square.
		return true
	}
	return dx*dx+dy*dy <= radius*radius
}
