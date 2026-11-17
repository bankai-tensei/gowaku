package gowakutest_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// heard is what a Listener heard: the modifiers of each kind of event.
type heard map[core.PointerKind][]input.Modifiers

func listening(h heard) gowaku.Widget {
	return interact.Listener{
		OnPointer: func(e *core.PointerEvent) { h[e.Kind] = append(h[e.Kind], e.Modifiers) },
		Child:     interact.Semantics{Role: semantics.RoleGroup, Label: "Pad", Child: basic.SizedBox{Width: 200, Height: 200}},
	}
}

// all reports whether every one of ms is want, and there is at least one.
func all(ms []input.Modifiers, want input.Modifiers) bool {
	return len(ms) > 0 && !slices.ContainsFunc(ms, func(m input.Modifiers) bool { return m != want })
}

// TestHold_PointerActionsCarryTheKeys: what Hold keeps down reaches every
// event of a tap, a drag, a hover and a scroll, until Hold(0).
func TestHold_PointerActionsCarryTheKeys(t *testing.T) {
	h := heard{}
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: listening(h)}, goFonts)
	both := input.ModShift | input.ModControl

	tt.Hold(input.ModShift)
	tt.Tap("Pad")
	if !all(h[core.PointerDown], input.ModShift) || !all(h[core.PointerUp], input.ModShift) {
		t.Errorf("Shift-tap: got %v", h)
	}

	clear(h)
	tt.Hold(input.ModControl)
	tt.DragAt(geom.Offset{X: 20, Y: 20}, geom.Offset{X: 150, Y: 120})
	if !all(h[core.PointerDown], input.ModControl) || !all(h[core.PointerMove], input.ModControl) || !all(h[core.PointerUp], input.ModControl) {
		t.Errorf("Ctrl-drag: got %v", h)
	}

	clear(h)
	tt.ScrollAt(geom.Offset{X: 50, Y: 50}, 0, -1, input.ModShift)
	if !all(h[core.PointerScroll], both) {
		t.Errorf("scroll adds the held keys to its own: got %v", h)
	}

	clear(h)
	tt.Hold(0)
	tt.DoubleTap("Pad")
	if !all(h[core.PointerDown], 0) {
		t.Errorf("after Hold(0): got %v", h)
	}
}

// TestHold_KeyAddsTheKeys: Key sends what Hold keeps down with its own, so
// that Ctrl held and Key(S, Shift) is Ctrl+Shift+S.
func TestHold_KeyAddsTheKeys(t *testing.T) {
	saved := 0
	tt := gowakutest.New(t, interact.Actions{
		Actions: []interact.Action{{Label: "Save as", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl | input.ModShift}, OnInvoke: func() { saved++ }}},
		Child:   basic.SizedBox{Width: 10, Height: 10},
	}, goFonts)
	tt.Key(input.KeyS, input.ModShift)
	if saved != 0 {
		t.Fatal("Shift+S ran Ctrl+Shift+S")
	}
	tt.Hold(input.ModControl)
	tt.Key(input.KeyS, input.ModShift)
	if saved != 1 {
		t.Errorf("Ctrl held and Shift+S ran the action %d times, want once", saved)
	}
}

// TestHold_ShiftClickSelectsARange: a Shift-click in a GridView that selects
// many takes in every item from the one clicked before, and a Ctrl-click
// adds one more.
func TestHold_ShiftClickSelectsARange(t *testing.T) {
	ctrl := &collection.TableController{}
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 400, Height: 300, Child: collection.GridView{
		ItemCount: 12, MinColumnWidth: 100, TileHeight: 80, Label: "Photos", Controller: ctrl,
		Select:      collection.TableSelectMultiple,
		LabelOf:     func(i int) string { return "item " + strconv.Itoa(i) },
		ItemBuilder: func(int) gowaku.Widget { return basic.SizedBox{} },
	}}}, goFonts)
	selected := func() []int {
		var out []int
		ctrl.SelectedRows(func(i int) bool { out = append(out, i); return true })
		return out
	}
	tt.Tap("item 1")
	tt.Hold(input.ModShift)
	tt.Tap("item 4")
	if got := selected(); !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Errorf("Shift-click: selected %v, want 1 to 4", got)
	}
	tt.Hold(input.ModControl)
	tt.Tap("item 7")
	if got := selected(); !slices.Equal(got, []int{1, 2, 3, 4, 7}) {
		t.Errorf("Ctrl-click: selected %v, want 1 to 4 and 7", got)
	}
	tt.Hold(0)
	tt.Tap("item 9")
	if got := selected(); !slices.Equal(got, []int{9}) {
		t.Errorf("plain click: selected %v, want 9 alone", got)
	}
}
