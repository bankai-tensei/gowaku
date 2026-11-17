package core

import "testing"

func TestSemanticsIDOf_MatchesTheNodeItOpened(t *testing.T) {
	h := newHarness(list{children: []Widget{leaf{w: 1, h: 7}, labeled{leaf{name: "ok", w: 30, h: 10}}}})
	h.layout()
	silent, speaker := h.rootRO().ChildAt(0), h.rootRO().ChildAt(1)
	if id := SemanticsIDOf(speaker); id != 0 {
		t.Errorf("before any walk: got %d, want 0", id)
	}
	root := h.tree.Semantics()
	if got, want := SemanticsIDOf(speaker), root.Children[0].ID; got == 0 || got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if id := SemanticsIDOf(silent); id != 0 {
		t.Errorf("an object that opens no node: got %d, want 0", id)
	}
}
