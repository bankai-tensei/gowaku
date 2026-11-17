package core

import (
	"fmt"
	"reflect"
	"slices"
)

// SameWidget reports whether b would update a and keep its state, rather than
// replace it.
func SameWidget(a, b Widget) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b) && a.Key() == b.Key()
}

// canUpdate reports whether el can take on w: same dynamic type, same key.
func canUpdate(el *element, w Widget) bool {
	return el.typ == reflect.TypeOf(w) && el.key == w.Key()
}

// updateChild reconciles one child slot. It returns the element for w
// in the place of el, either of which can be nil.
func (t *Tree) updateChild(parent, el *element, w Widget) *element {
	switch {
	case w == nil:
		if el != nil {
			el.unmount()
		}
		return nil
	case el == nil:
		return t.inflate(w, parent)
	case canUpdate(el, w):
		el.update(w)
		return el
	}
	el.unmount()
	return t.inflate(w, parent)
}

// updateChildren reconciles a list of children: match from the top, then
// from the bottom, and look up what is left in the middle by key. It is O(n).
// An unkeyed child in the middle cannot be followed and starts over.
func (el *element) updateChildren(widgets []Widget) {
	t := el.tree
	old := el.children
	out := slices.Grow(el.spare[:0], len(widgets))[:len(widgets)]

	newTop, oldTop := 0, 0
	newBottom, oldBottom := len(widgets)-1, len(old)-1

	// Update what matches at the top.
	for newTop <= newBottom && oldTop <= oldBottom {
		if !canUpdate(old[oldTop], widgets[newTop]) {
			break
		}
		old[oldTop].update(widgets[newTop])
		out[newTop] = old[oldTop]
		newTop++
		oldTop++
	}

	// Only find what matches at the bottom. It is updated last, in order.
	for newTop <= newBottom && oldTop <= oldBottom {
		if !canUpdate(old[oldBottom], widgets[newBottom]) {
			break
		}
		newBottom--
		oldBottom--
	}

	// Index the keyed children of the old middle, and drop the rest.
	var byKey map[Key]*element
	if oldTop <= oldBottom {
		byKey = t.acquireKeyMap()
		for _, c := range old[oldTop : oldBottom+1] {
			if c.key == nil {
				c.unmount()
				continue
			}
			if _, dup := byKey[c.key]; dup {
				panic(fmt.Sprintf("gowaku: two siblings share the key %v", c.key))
			}
			byKey[c.key] = c
		}
	}

	// Fill the new middle from the index, or with new elements.
	for ; newTop <= newBottom; newTop++ {
		w := widgets[newTop]
		var match *element
		if k := w.Key(); k != nil && byKey != nil {
			if c := byKey[k]; c != nil && canUpdate(c, w) {
				delete(byKey, k)
				match = c
			}
		}
		out[newTop] = t.updateChild(el, match, w)
	}

	// Update the matches of the bottom.
	for oldBottom++; newTop < len(widgets); newTop++ {
		old[oldBottom].update(widgets[newTop])
		out[newTop] = old[oldBottom]
		oldBottom++
	}

	// Whatever the index still holds did not come back.
	if byKey != nil {
		for _, c := range byKey {
			c.unmount()
		}
		t.releaseKeyMap(byKey)
	}

	clear(old)
	el.children, el.spare = out, old[:0]
}
