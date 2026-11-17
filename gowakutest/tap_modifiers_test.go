package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// TestGestureDetector_TapsCarryTheModifiers checks that the taps of a
// GestureDetector tell the modifier keys held, with the place relative to
// the child: OnTapDownAt at the press, OnTapAt after OnTap at the release, and
// OnSecondaryTapAt, so that a Shift+click needs no Listener.
func TestGestureDetector_TapsCarryTheModifiers(t *testing.T) {
	var order []string
	var down, up, secondary layout.Tap
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Padding{
		Insets: geom.All(10),
		Child: interact.GestureDetector{
			OnTapDownAt:      func(tp layout.Tap) { down = tp; order = append(order, "down") },
			OnTap:            func() { order = append(order, "tap") },
			OnTapAt:          func(tp layout.Tap) { up = tp; order = append(order, "tapAt") },
			OnSecondaryTapAt: func(tp layout.Tap) { secondary = tp },
			Child:            interact.Semantics{Role: semantics.RoleGroup, Label: "Row", Child: basic.SizedBox{Width: 100, Height: 40}},
		},
	}}, goFonts)

	tt.Hold(input.ModShift)
	tt.TapAt(geom.Offset{X: 30, Y: 20})
	if down.Modifiers != input.ModShift || up.Modifiers != input.ModShift {
		t.Errorf("Shift+click: down %+v, up %+v, want Shift on both", down, up)
	}
	if want := (geom.Offset{X: 20, Y: 10}); down.Pos != want || up.Pos != want {
		t.Errorf("Shift+click: down at %v, up at %v, want %v relative to the child", down.Pos, up.Pos, want)
	}
	if got := len(order); got != 3 || order[0] != "down" || order[1] != "tap" || order[2] != "tapAt" {
		t.Errorf("order %v, want down, tap, tapAt", order)
	}

	tt.Hold(input.ModControl)
	tt.SecondaryTap("Row")
	if secondary.Modifiers != input.ModControl {
		t.Errorf("Ctrl+right click: %+v, want Ctrl", secondary)
	}

	tt.Hold(0)
	tt.Tap("Row")
	if down.Modifiers != 0 || up.Modifiers != 0 {
		t.Errorf("a plain click: down %+v, up %+v, want no keys", down, up)
	}
}

// TestGestureDetector_OnTapAtAloneTakesThePress checks that OnTapAt by itself
// is enough for the detector to take the primary button.
func TestGestureDetector_OnTapAtAloneTakesThePress(t *testing.T) {
	taps := 0
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: interact.GestureDetector{
		OnTapAt: func(layout.Tap) { taps++ },
		Child:   interact.Semantics{Role: semantics.RoleGroup, Label: "Pad", Child: basic.SizedBox{Width: 200, Height: 200}},
	}}, goFonts)
	tt.Tap("Pad")
	if taps != 1 {
		t.Fatalf("taps %d after a click, want 1", taps)
	}
}
