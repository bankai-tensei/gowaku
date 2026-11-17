package core

import "github.com/bankai-tensei/gowaku/geom"

// Clipper is a RenderObject that cuts what its children paint to a rectangle,
// such as the viewport of a scroll view. Tools that judge what a user sees, as
// gowakutest.Tester.Warnings does, leave out what lies wholly outside it: a row
// scrolled out of view, or a page slid out of its slot. See
// docs/ADR/0132-tester-clip-and-held-modifiers.md.
//
// ChildClip is asked outside the frame only, so it need not be fast.
type Clipper interface {
	RenderObject
	// ChildClip returns the rectangle that the children are cut to now, in
	// the coordinates of the object, and false while it does not cut them.
	ChildClip() (geom.Rect, bool)
}
