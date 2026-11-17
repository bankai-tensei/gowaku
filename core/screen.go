package core

import "github.com/bankai-tensei/gowaku/geom"

// ScreenHost is a Host that knows where its window is on the screen, so that a
// drag can go from one window to another, as a tab torn off a TabBar does. A
// Host need not implement it. Points of the screen are in physical pixels,
// which every window of the desktop shares; logical pixels differ between
// windows on monitors of different scales.
type ScreenHost interface {
	// ScreenOrigin returns where the top-left corner of the window's client
	// area is on the screen, and the physical pixels per logical pixel of the
	// window. ok is false where the window system does not tell, as on
	// Wayland.
	ScreenOrigin() (origin geom.Offset, scale float32, ok bool)
	// ShowsScreenPoint reports whether the window is what the user sees at a
	// point of the screen: inside it, and under no other window.
	ShowsScreenPoint(p geom.Offset) bool
	// Activate brings the window to the front and gives it the keyboard, as
	// far as the window system lets an application do that.
	Activate()
}

// ScreenOf returns where the top-left corner of ro is on the screen, in
// physical pixels, and the physical pixels per logical pixel of its window: a
// point pos of ro is at origin + pos × scale. ok is false where the host of ro
// does not know where its window is.
func ScreenOf(ro RenderObject) (origin geom.Offset, scale float32, ok bool) {
	h, isScreen := ro.base().hostOrNone().(ScreenHost)
	if !isScreen {
		return geom.Offset{}, 0, false
	}
	win, scale, ok := h.ScreenOrigin()
	if !ok || scale <= 0 {
		return geom.Offset{}, 0, false
	}
	g := GlobalOffset(ro)
	return geom.Offset{X: win.X + g.X*scale, Y: win.Y + g.Y*scale}, scale, true
}

// ScreenHostOf returns the host of the window of ro if it is a ScreenHost.
func ScreenHostOf(ro RenderObject) (ScreenHost, bool) {
	h, ok := ro.base().hostOrNone().(ScreenHost)
	return h, ok
}

// SameTree reports whether a and b are in the same tree, which is to say the
// same window.
func SameTree(a, b RenderObject) bool {
	t := a.base().tree
	return t != nil && t == b.base().tree
}

// hostOrNone returns the host of the tree of b, or nil when b is not in one.
func (b *RenderBase) hostOrNone() Host {
	if b.tree == nil {
		return nil
	}
	return b.tree.Host()
}
