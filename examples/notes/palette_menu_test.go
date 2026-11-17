package main

import "testing"

// The palette opened from the menu takes the keyboard, as from Ctrl+K.
func TestPalette_FromTheMenuTakesTheKeyboard(t *testing.T) {
	h := newHarness(t, false, nil)
	h.tap("表示(V)")
	h.tap("コマンドパレット(P)…")
	if _, ok := h.tt.Find("新規"); !ok {
		t.Fatalf("the palette did not open:\n%s", h.tt.Dump())
	}
	h.typeText("縦")
	if _, ok := h.tt.Find("新規"); ok {
		t.Errorf("typing did not reach the palette:\n%s", h.tt.Dump())
	}
}
