package core

// Hider is a RenderObject that can hide its subtree without taking it out of
// the tree, the way a fully transparent Opacity does. While Hidden reports
// true, the tree treats the subtree as absent for everything but layout: it is
// left out of hit testing, so the pointer reaches what is below and the object
// itself is not hit either; out of the semantics tree; and out of focus
// traversal with Tab, including the search for a modal scope. The object keeps
// its size, and its ancestors still see the pointer over that area, so a
// MouseRegion around a hidden overlay reports the pointer entering and leaving
// as before.
//
// Hidden is asked on every hit test, so it must be cheap and must not
// allocate. The tree asks again at the next pointer event; nothing is cached.
type Hider interface {
	RenderObject
	Hidden() bool
}

// isHidden reports whether ro hides its subtree at the moment.
func isHidden(ro RenderObject) bool {
	h, ok := ro.(Hider)
	return ok && h.Hidden()
}
