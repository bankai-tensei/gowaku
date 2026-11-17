package core

import "github.com/bankai-tensei/gowaku/geom"

// Transformer is a RenderObject that paints its children through an affine
// transform, as layout.RenderTransform does. The pointer, hit testing, the IME
// caret and the semantics tree follow it through GlobalToLocal and
// LocalToGlobal. See docs/ADR/0044-drawing-extensions.md.
//
// A Transformer hit tests its children through the inverse of ChildTransform
// itself; core only uses ChildTransform to convert positions.
type Transformer interface {
	RenderObject
	// ChildTransform returns the transform from the coordinates of the
	// children, after their Offset, to those of the object. The zero value is
	// the identity.
	ChildTransform() geom.Affine
}

// LocalToGlobal returns where p, relative to ro, lies in the window.
func LocalToGlobal(ro RenderObject, p geom.Offset) geom.Offset {
	if !underTransform(ro) {
		return p.Add(GlobalOffset(ro))
	}
	return toGlobal(ro).Apply(p)
}

// GlobalToLocal returns where p, relative to the window, lies relative to ro. It
// reports false if a transform on the way squashes ro flat, so that no point of
// the window maps back to it.
func GlobalToLocal(ro RenderObject, p geom.Offset) (geom.Offset, bool) {
	if !underTransform(ro) {
		return p.Sub(GlobalOffset(ro)), true
	}
	inverse, ok := toGlobal(ro).Invert()
	if !ok {
		return geom.Offset{}, false
	}
	return inverse.Apply(p), true
}

// LocalRectToGlobal returns the upright bounds in the window of r, relative to
// ro: r moved, or the box around it once turned.
func LocalRectToGlobal(ro RenderObject, r geom.Rect) geom.Rect {
	if !underTransform(ro) {
		return r.Translate(GlobalOffset(ro))
	}
	return toGlobal(ro).ApplyRect(r)
}

// underTransform reports whether a Transformer above ro turns or scales it. The
// common case keeps the plain sums of GlobalOffset, to the last bit.
func underTransform(ro RenderObject) bool {
	for p := ro.Parent(); p != nil; p = p.Parent() {
		if t, ok := p.(Transformer); ok && !t.ChildTransform().IsIdentity() {
			return true
		}
	}
	return false
}

// toGlobal returns the transform from the coordinates of ro to the window.
func toGlobal(ro RenderObject) geom.Affine {
	m := geom.Identity()
	for cur := ro; cur != nil; cur = cur.Parent() {
		m = geom.Translation(cur.Offset()).Mul(m)
		if t, ok := cur.Parent().(Transformer); ok {
			if c := t.ChildTransform(); !c.IsIdentity() {
				m = c.Mul(m)
			}
		}
	}
	return m
}

// PaintTransformer is a RenderObject that paints its children moved or scaled
// away from where they are laid out, while layout, the pointer and the
// semantics tree keep them in place, as layout.RenderTranslate and
// layout.RenderScale do for transitions. core does not use it; what reads the
// painted frame, such as the Warnings of gowakutest, follows it to find where
// a child shows.
type PaintTransformer interface {
	RenderObject
	// PaintTransform returns the transform from the coordinates of the
	// children, after their Offset, to those of the object, as painted. The
	// zero value is the identity.
	PaintTransform() geom.Affine
}
