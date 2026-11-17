package core

import "github.com/bankai-tensei/gowaku/input"

// ShortcutTarget is a RenderObject that reacts to the key presses that nothing
// in focus used, such as Ctrl+S or the Alt of a menu bar.
//
// DispatchKey offers a press to those above the focus first, nearest first, and
// then to every other one in the window, the last painted first, so that a
// shortcut works whether or not anything has the focus. A modal FocusScope,
// such as a dialog, keeps the presses to the targets inside it.
// See docs/ADR/0046-menus-and-shortcuts.md.
type ShortcutTarget interface {
	RenderObject
	// HandleShortcut reacts to a key press. It returns false to pass it on.
	HandleShortcut(e *KeyEvent) bool
}

// DispatchKeyUp reports that a key came up. Only Alt matters: pressed and
// released with nothing in between, it arrives as a press of input.KeyAlt
// without ModAlt.
func (t *Tree) DispatchKeyUp(e KeyEvent) {
	if e.Key != input.KeyAlt {
		return
	}
	tapped := t.altDown
	t.altDown = false
	if tapped {
		t.dispatchKey(KeyEvent{Key: input.KeyAlt, Modifiers: e.Modifiers &^ input.ModAlt})
	}
}

// altPress tracks a lone Alt. It returns true for the press of Alt itself,
// which nothing should see: Alt alone means something only once it comes up.
func (t *Tree) altPress(e *KeyEvent) bool {
	if e.Key == input.KeyAlt && e.Modifiers&^input.ModAlt == 0 && !e.Repeat {
		t.altDown = true
		return true
	}
	if e.Key == input.KeyAlt {
		// A held Alt repeats, and stays alone. With another modifier it is
		// part of a chord.
		t.altDown = t.altDown && e.Repeat
		return true
	}
	t.altDown = false
	return false
}

// dispatchShortcut offers e to the shortcut targets: those above the focus,
// nearest first, then the rest of the scope.
func (t *Tree) dispatchShortcut(e *KeyEvent) bool {
	if t.focus != nil {
		for ro := RenderObject(t.focus); ro != nil; ro = ro.Parent() {
			if s, ok := ro.(ShortcutTarget); ok && s.HandleShortcut(e) {
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
	return t.shortcutBelow(root, e)
}

// shortcutBelow offers e to the targets below ro, the last painted first,
// skipping hidden subtrees and those above the focus, which had their turn.
func (t *Tree) shortcutBelow(ro RenderObject, e *KeyEvent) bool {
	if isHidden(ro) {
		return false
	}
	for i := ro.ChildCount() - 1; i >= 0; i-- {
		if t.shortcutBelow(ro.ChildAt(i), e) {
			return true
		}
	}
	s, ok := ro.(ShortcutTarget)
	return ok && !t.aboveFocus(ro) && s.HandleShortcut(e)
}

// aboveFocus reports whether ro is the focus or one of its ancestors.
func (t *Tree) aboveFocus(ro RenderObject) bool {
	if t.focus == nil {
		return false
	}
	for f := RenderObject(t.focus); f != nil; f = f.Parent() {
		if f == ro {
			return true
		}
	}
	return false
}
