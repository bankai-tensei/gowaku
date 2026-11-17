package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// ringWidth is the width of the focus ring, which touches the shape.
const ringWidth = 3

// RenderOpacity paints its child faded as one picture, as opacity does in CSS:
// where two shapes of the child overlap, the lower one does not show through. The
// child draws into a group of the canvas (paint.Canvas.PushGroup), a picture on
// the GPU each frame it is drawn; one that draws a single shape skips the
// picture, as fading the shape alone looks the same.
//
// At 0 and below it is a hidden core.Hider: the child is kept and laid out,
// but the pointer, the semantics tree and Tab pass it by, since nobody can see
// it.
type RenderOpacity struct {
	RenderProxy
	// Opacity runs from 0, which paints nothing, to 1.
	Opacity float32
}

// Hidden implements core.Hider.
func (r *RenderOpacity) Hidden() bool { return r.Opacity <= 0 }

// Paint implements core.RenderObject.
func (r *RenderOpacity) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Opacity <= 0 {
		return
	}
	if r.Opacity >= 1 {
		r.RenderProxy.Paint(cv, off)
		return
	}
	cv.Save()
	cv.PushOpacity(r.Opacity)
	cv.PushGroup(paint.GroupStyle{})
	r.RenderProxy.Paint(cv, off)
	cv.PopGroup()
	cv.Restore()
}

// RenderScale paints its child enlarged around a point. Layout and hit testing
// stay those of the unscaled child: it is made for transitions, which end at 1.
type RenderScale struct {
	RenderProxy
	Scale float32
	// Origin is the point that stays in place.
	Origin Alignment
}

// PaintTransform implements core.PaintTransformer.
func (r *RenderScale) PaintTransform() geom.Affine {
	if r.Scale == 1 {
		return geom.Affine{}
	}
	sz := r.Size()
	pivot := geom.Offset{X: sz.W * r.Origin.X, Y: sz.H * r.Origin.Y}
	return geom.Translation(pivot).Mul(geom.Scaling(r.Scale, r.Scale)).Mul(geom.Translation(pivot.Scale(-1)))
}

// Paint implements core.RenderObject.
func (r *RenderScale) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Scale == 1 {
		r.RenderProxy.Paint(cv, off)
		return
	}
	sz := r.Size()
	pivot := off.Add(geom.Offset{X: sz.W * r.Origin.X, Y: sz.H * r.Origin.Y})
	cv.Save()
	cv.Translate(pivot.X, pivot.Y)
	cv.Scale(r.Scale)
	cv.Translate(-pivot.X, -pivot.Y)
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// RenderIgnorePointer lets the pointer pass through its child.
type RenderIgnorePointer struct {
	RenderProxy
}

// HitTest implements core.RenderObject.
func (r *RenderIgnorePointer) HitTest(geom.Offset, *core.HitTestResult) bool { return false }

// RenderModal keeps Tab inside its child while Active is set.
type RenderModal struct {
	RenderProxy
	Active bool
	// BlockSemantics also takes what is painted before it out of the
	// semantics tree while Active is set, as a dialog hides the page under it
	// from a screen reader. A popup anchored to a control leaves it unset, so
	// that the control stays in the tree and says that it is expanded.
	BlockSemantics bool
}

// Modal implements core.FocusScope.
func (r *RenderModal) Modal() bool { return r.Active }

// BlocksSemantics implements core.SemanticsBlocker.
func (r *RenderModal) BlocksSemantics() bool { return r.Active && r.BlockSemantics }

// RenderRing draws the focus ring just outside its child. It is the one loud thing
// on a page: it says where the keys go.
type RenderRing struct {
	RenderProxy
	// Radius is that of the child.
	Radius float32
	// Color is the ring. Transparent means that there is none.
	Color geom.Color
}

// Paint implements core.RenderObject.
func (r *RenderRing) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Color.A > 0 {
		sz := r.Size()
		ring := geom.Rect{X: off.X - ringWidth, Y: off.Y - ringWidth, W: sz.W + 2*ringWidth, H: sz.H + 2*ringWidth}
		radius := min(r.Radius, sz.W/2, sz.H/2) + ringWidth
		cv.DrawRRect(geom.RRectAll(ring, radius), paint.Paint{Stroke: &paint.Stroke{Width: ringWidth, Color: r.Color}})
	}
	r.RenderProxy.Paint(cv, off)
}

// LerpShadows blends two lists of shadows. Past the end of the shorter list the
// shadows fade in from nothing.
func LerpShadows(a, b []paint.BoxShadow, t float32) []paint.BoxShadow {
	out := make([]paint.BoxShadow, max(len(a), len(b)))
	for i := range out {
		var from, to paint.BoxShadow
		if i < len(a) {
			from = a[i]
		}
		if i < len(b) {
			to = b[i]
		}
		if i >= len(a) {
			from = to
			from.Color = to.Color.WithAlpha(0)
		}
		if i >= len(b) {
			to = from
			to.Color = from.Color.WithAlpha(0)
		}
		out[i] = paint.BoxShadow{
			Color:  lerpColorStraight(from.Color, to.Color, t),
			Offset: geom.Offset{X: from.Offset.X + (to.Offset.X-from.Offset.X)*t, Y: from.Offset.Y + (to.Offset.Y-from.Offset.Y)*t},
			Blur:   from.Blur + (to.Blur-from.Blur)*t,
			Spread: from.Spread + (to.Spread-from.Spread)*t,
		}
	}
	return out
}

func lerpColorStraight(a, b geom.Color, t float32) geom.Color {
	return geom.Color{R: a.R + (b.R-a.R)*t, G: a.G + (b.G-a.G)*t, B: a.B + (b.B-a.B)*t, A: a.A + (b.A-a.A)*t}
}

// RenderBackdropFilter blurs what is painted below its box before it paints its
// child, as backdrop-filter does in CSS. The blur stays inside the box, with
// rounded corners, and inside the clip in effect. A RepaintBoundary that holds it
// is drawn as part of the frame each time, since it reads the surface. See
// docs/ADR/0044-drawing-extensions.md.
type RenderBackdropFilter struct {
	RenderProxy
	// Sigma is the standard deviation of the blur in logical pixels. Zero and
	// below paint the child alone.
	Sigma float32
	// Radius rounds the corners of the blurred box.
	Radius float32
}

// Paint implements core.RenderObject.
func (r *RenderBackdropFilter) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Sigma > 0 {
		sz := r.Size()
		radius := max(min(r.Radius, sz.W/2, sz.H/2), 0)
		cv.DrawBackdrop(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, radius), r.Sigma)
	}
	r.RenderProxy.Paint(cv, off)
}
