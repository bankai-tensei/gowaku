package overlay

import "github.com/bankai-tensei/gowaku/core"

// Contains reports whether ro lies within region: it is region or below it,
// or it is in what an overlay entry shows that was opened from within region
// with Host.InsertFrom, as a menu, a popup, a dialog or a tooltip opened from
// a widget there, and so on through the entries those open in turn.
//
// A surface that hides when the focus or a press goes elsewhere, as a panel
// that slid out or a popover, asks it about the focus, so that a menu opened
// from inside the surface counts as inside. An entry inserted without a place,
// as a toast, lies within nothing but what holds the Overlay. Both may be nil.
// See docs/ADR/0118-dock-close-guard-and-flyout-overlays.md.
func Contains(region, ro core.RenderObject) bool {
	if region == nil {
		return false
	}
	for ; ro != nil; ro = ownerParent(ro) {
		if ro == region {
			return true
		}
	}
	return false
}

// EntryFocusListener is a render object that wants to know when the focus
// leaves an overlay entry opened from within it, as Contains counts them. The
// focus going from the surface into such an entry leaves the surface, which
// core.FocusWithinListener reports; the focus then leaving the entry for
// somewhere else touches nothing of the surface, so this tells it to look
// again. Focused already answers the new target when it runs.
type EntryFocusListener interface {
	core.RenderObject
	EntryFocusLeft()
}

// ownerParent returns what holds ro for Contains: its parent, but above the
// top of what an entry with a place shows, the render object of that place.
func ownerParent(ro core.RenderObject) core.RenderObject {
	if from, ok := entryPlace(ro); ok {
		return from
	}
	return ro.Parent()
}

// entryPlace returns the render object an entry was opened from, if ro is
// the top of what one shows and it has a place.
func entryPlace(ro core.RenderObject) (core.RenderObject, bool) {
	r, ok := ro.(*renderEntryHits)
	if !ok || r.entry == nil || r.entry.from == (core.BuildContext{}) {
		return nil, false
	}
	return r.entry.from.RenderObject(), true
}

// entryFocusLeft tells the EntryFocusListeners above the place of the entry
// shown below r that the focus left it.
func entryFocusLeft(r *renderEntryHits) {
	from, ok := entryPlace(r)
	if !ok {
		return
	}
	for ro := from; ro != nil; ro = ownerParent(ro) {
		if l, ok := ro.(EntryFocusListener); ok {
			l.EntryFocusLeft()
		}
	}
}
