package layout

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// RenderTranslate paints its child moved by X and Y times its size, for
// transitions. Layout, the pointer and semantics see the child in place.
type RenderTranslate struct {
	RenderProxy
	// X and Y are the shift as fractions of the width and height: 1 is a whole
	// width to the right, -0.1 a tenth of a height up.
	X, Y float32
	// Clip limits the painting to the object's own box, so that a page sliding
	// in does not cover its neighbors. It cuts also at rest, X and Y 0, so
	// that a child larger than the box does not show more of itself when the
	// motion ends.
	Clip bool
}

// ChildClip implements core.Clipper.
func (r *RenderTranslate) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), r.Clip
}

// PaintTransform implements core.PaintTransformer.
func (r *RenderTranslate) PaintTransform() geom.Affine {
	if r.X == 0 && r.Y == 0 {
		return geom.Affine{}
	}
	sz := r.Size()
	return geom.Translation(geom.Offset{X: r.X * sz.W, Y: r.Y * sz.H})
}

// Paint implements core.RenderObject.
func (r *RenderTranslate) Paint(cv *paint.Canvas, off geom.Offset) {
	if !r.Clip && r.X == 0 && r.Y == 0 {
		r.RenderProxy.Paint(cv, off)
		return
	}
	sz := r.Size()
	cv.Save()
	if r.Clip {
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	}
	r.RenderProxy.Paint(cv, off.Add(geom.Offset{X: r.X * sz.W, Y: r.Y * sz.H}))
	cv.Restore()
}
