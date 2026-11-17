package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// NavigationTarget is a RenderObject that goes back and forward through pages,
// as widgets.Navigator does.
//
// The tree offers it Alt+Left and Alt+Right before the focus sees them, since
// a list or a text field would take the arrow for itself: first to the targets
// above the focus, nearest first, then to the others in the window, the last
// painted first, as with ShortcutTarget. A press that no target takes goes on
// to the focus as usual. The thumb buttons of a mouse, input.ButtonBack and
// input.ButtonForward, go to the targets under the pointer, deepest first, when
// they come up, and never arrive as presses. A modal FocusScope keeps the keys
// to the targets inside it, and a dialog over the targets keeps the pointer
// from them. See docs/ADR/0063-navigator.md.
type NavigationTarget interface {
	RenderObject
	// HandleNavigation goes back, or forward if forward is set. It returns
	// false when there is nowhere to go, which passes the request on to the
	// targets further out.
	HandleNavigation(forward bool) bool
}

// navigationKey reports whether e asks to go back or forward: Alt+Left or
// Alt+Right with no other modifier. A held key does not repeat the request, so
// that holding it cannot empty a stack of pages.
func navigationKey(e *KeyEvent) (forward, ok bool) {
	if e.Modifiers != input.ModAlt || e.Repeat {
		return false, false
	}
	switch e.Key {
	case input.KeyLeft:
		return false, true
	case input.KeyRight:
		return true, true
	}
	return false, false
}

// navigationButton reports whether b is a thumb button, and which.
func navigationButton(b input.Button) (forward, ok bool) {
	switch b {
	case input.ButtonBack:
		return false, true
	case input.ButtonForward:
		return true, true
	}
	return false, false
}

// navigateFromFocus offers a request to go back or forward to the targets above
// the focus, then to the rest of the scope.
func (t *Tree) navigateFromFocus(forward bool) bool {
	if t.focus != nil {
		for ro := RenderObject(t.focus); ro != nil; ro = ro.Parent() {
			if n, ok := ro.(NavigationTarget); ok && n.HandleNavigation(forward) {
				return true
			}
		}
	}
	root := t.rootRO
	if root == nil {
		return false
	}
	if scope := modalScope(root); scope != nil {
		root = scope
	}
	return t.navigateBelow(root, forward)
}

// navigateBelow offers the request to the targets below ro, the last painted
// first and the inner before the outer, skipping hidden subtrees and those
// above the focus, which had their turn.
func (t *Tree) navigateBelow(ro RenderObject, forward bool) bool {
	if isHidden(ro) {
		return false
	}
	for i := ro.ChildCount() - 1; i >= 0; i-- {
		if t.navigateBelow(ro.ChildAt(i), forward) {
			return true
		}
	}
	n, ok := ro.(NavigationTarget)
	return ok && !t.aboveFocus(ro) && n.HandleNavigation(forward)
}

// navigateAt offers the request to the targets under pos, deepest first.
func (t *Tree) navigateAt(pos geom.Offset, forward bool) bool {
	t.hit(pos)
	for _, ro := range t.hits.Path {
		if n, ok := ro.(NavigationTarget); ok && n.HandleNavigation(forward) {
			return true
		}
	}
	return false
}
