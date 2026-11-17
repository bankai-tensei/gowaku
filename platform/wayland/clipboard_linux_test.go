package wayland

import "testing"

// offersOf fills a clipboard with offers that have no proxy behind them, which
// is enough for the bookkeeping: takeUnused sends nothing.
func offersOf(ids ...wlDataOffer) *clipboard {
	cb := &clipboard{offers: map[wlDataOffer]*offer{}}
	for _, id := range ids {
		cb.offers[id] = &offer{proxy: id}
	}
	return cb
}

func taken(unused []*offer) map[wlDataOffer]bool {
	m := map[wlDataOffer]bool{}
	for _, o := range unused {
		m[o.proxy] = true
	}
	return m
}

func TestClipboard_SelectionKeepsTheOfferOfADrag(t *testing.T) {
	const drag, old, next = wlDataOffer(1), wlDataOffer(2), wlDataOffer(3)
	cb := offersOf(drag, old, next)
	cb.selection = cb.offers[old]
	cb.drop.offer = cb.offers[drag]

	// What wlDataDeviceSelection(next) does before it destroys.
	cb.selection = cb.offers[next]
	got := taken(cb.takeUnused())

	if len(got) != 1 || !got[old] {
		t.Errorf("took %v, want only the selection before", got)
	}
	if cb.offers[drag] == nil || cb.offers[next] == nil || len(cb.offers) != 2 {
		t.Errorf("offers left = %v, want the drag and the selection", cb.offers)
	}
	if cb.drop.offer != cb.offers[drag] {
		t.Error("the drag lost its offer")
	}
}

func TestClipboard_ClearedSelectionKeepsTheOfferOfADrag(t *testing.T) {
	const drag, old = wlDataOffer(1), wlDataOffer(2)
	cb := offersOf(drag, old)
	cb.selection = cb.offers[old]
	cb.drop.offer = cb.offers[drag]

	// A selection of no offer: nothing is on the clipboard.
	cb.selection = cb.offers[0]
	got := taken(cb.takeUnused())

	if len(got) != 1 || !got[old] {
		t.Errorf("took %v, want only the selection before", got)
	}
	if cb.offers[drag] == nil || len(cb.offers) != 1 {
		t.Errorf("offers left = %v, want the drag", cb.offers)
	}
}

func TestClipboard_LeaveTakesTheOfferOfTheDragOnce(t *testing.T) {
	const drag, sel = wlDataOffer(1), wlDataOffer(2)
	cb := offersOf(drag, sel)
	cb.selection = cb.offers[sel]
	cb.drop.offer = cb.offers[drag]

	// What wlDataDeviceLeave does before it destroys. The drag has no window
	// here, so nobody is asked.
	cb.dropLeft()
	got := taken(cb.takeUnused())

	if len(got) != 1 || !got[drag] {
		t.Errorf("took %v, want only the drag", got)
	}
	if cb.drop.offer != nil {
		t.Error("the drag is still there after it left")
	}
	if cb.offers[sel] == nil || len(cb.offers) != 1 {
		t.Errorf("offers left = %v, want the selection", cb.offers)
	}
	// A second leave, or a selection after it, finds nothing to destroy again.
	if again := cb.takeUnused(); len(again) != 0 {
		t.Errorf("took %v the second time, want nothing", taken(again))
	}
}

func TestClipboard_SelectionTakesAnOfferNothingUses(t *testing.T) {
	// An offer whose drag entered a surface that is not a window of this
	// client is in no drop, and goes with the next selection.
	const stray, next = wlDataOffer(1), wlDataOffer(2)
	cb := offersOf(stray, next)

	cb.selection = cb.offers[next]
	got := taken(cb.takeUnused())

	if len(got) != 1 || !got[stray] {
		t.Errorf("took %v, want only the stray offer", got)
	}
}
