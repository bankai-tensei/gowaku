package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// RenderTransform paints its child through an affine transform around Origin.
// Layout ignores the transform, as in Flutter: the child takes its place
// upright, and only the picture turns. The pointer, the IME and the semantics
// tree follow the picture, through core.Transformer. See
// docs/ADR/0044-drawing-extensions.md.
type RenderTransform struct {
	RenderProxy
	// Transform is applied around Origin. The zero value is the identity.
	Transform geom.Affine
	// Origin is the point of the box that the transform keeps in place, 0..1
	// on each axis like the Origin of RenderScale.
	Origin Alignment
}

// ChildTransform implements core.Transformer.
func (r *RenderTransform) ChildTransform() geom.Affine {
	if r.Transform.IsIdentity() {
		return geom.Affine{}
	}
	sz := r.Size()
	pivot := geom.Offset{X: sz.W * r.Origin.X, Y: sz.H * r.Origin.Y}
	return geom.Translation(pivot).Mul(r.Transform).Mul(geom.Translation(geom.Offset{X: -pivot.X, Y: -pivot.Y}))
}

// Paint implements core.RenderObject.
func (r *RenderTransform) Paint(cv *paint.Canvas, off geom.Offset) {
	m := r.ChildTransform()
	if m.IsIdentity() {
		r.RenderProxy.Paint(cv, off)
		return
	}
	if m.Det() == 0 {
		// Squashed onto a line, the child shows nothing.
		return
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.Transform(m)
	cv.Translate(-off.X, -off.Y)
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// HitTest implements core.RenderObject. The position goes back through the
// transform, so that the child is hit where it shows: inside its turned box, and
// not in the corners of the box around it.
func (r *RenderTransform) HitTest(pos geom.Offset, result *core.HitTestResult) bool {
	m := r.ChildTransform()
	if m.IsIdentity() {
		return r.RenderProxy.HitTest(pos, result)
	}
	inverse, ok := m.Invert()
	if !ok {
		return false
	}
	local := inverse.Apply(pos)
	sz := r.Size()
	if local.X < 0 || local.Y < 0 || local.X >= sz.W || local.Y >= sz.H {
		return false
	}
	// Like the other objects of layout, a transform is hit only over its child.
	c := r.child
	if c == nil {
		return false
	}
	if h, hides := c.(core.Hider); hides && h.Hidden() {
		return false
	}
	if !c.HitTest(local.Sub(c.Offset()), result) {
		return false
	}
	result.Path = append(result.Path, r)
	return true
}
