package interact_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
)

// TestMouseRegion_OnHoverFollowsThePointer: a region of 200 by 100, 50 from
// the left and 20 from the top of the window, hears where the pointer moves
// over it, in its own coordinates, and nothing once the pointer is off it.
func TestMouseRegion_OnHoverFollowsThePointer(t *testing.T) {
	var log []string
	var seen []geom.Offset
	region := interact.MouseRegion{
		OnEnter: func() { log = append(log, "enter") },
		OnExit:  func() { log = append(log, "exit") },
		OnHover: func(pos geom.Offset) { log = append(log, "hover"); seen = append(seen, pos) },
		Child:   basic.SizedBox{Width: 200, Height: 100},
	}
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Padding{
		Insets: geom.Insets{Left: 50, Top: 20}, Child: region,
	}}, pressableFonts)

	tt.HoverAt(geom.Offset{X: 60, Y: 30})
	tt.HoverAt(geom.Offset{X: 230, Y: 100})
	want := []geom.Offset{{X: 10, Y: 10}, {X: 180, Y: 80}}
	if len(seen) != len(want) || seen[0] != want[0] || seen[1] != want[1] {
		t.Fatalf("OnHover saw %v, want %v", seen, want)
	}
	if len(log) != 3 || log[0] != "enter" || log[1] != "hover" || log[2] != "hover" {
		t.Errorf("events %v, want enter, hover, hover", log)
	}

	tt.HoverAt(geom.Offset{X: 400, Y: 300})
	if len(seen) != 2 || log[len(log)-1] != "exit" {
		t.Errorf("off the region: OnHover saw %v, events %v", seen, log)
	}
}

// TestMouseRegion_OnHoverBesideAButtonInside: the moves reach the region with a
// button inside it that takes the press, and the button still gets the tap.
func TestMouseRegion_OnHoverBesideAButtonInside(t *testing.T) {
	var seen []geom.Offset
	taps := 0
	tt := gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: interact.MouseRegion{
		OnHover: func(pos geom.Offset) { seen = append(seen, pos) },
		Child: interact.GestureDetector{
			OnTap: func() { taps++ },
			Child: basic.SizedBox{Width: 100, Height: 100},
		},
	}}, pressableFonts)
	tt.HoverAt(geom.Offset{X: 5, Y: 6})
	tt.TapAt(geom.Offset{X: 40, Y: 50})
	if taps != 1 {
		t.Errorf("the detector inside got %d taps, want 1", taps)
	}
	if len(seen) == 0 || seen[0] != (geom.Offset{X: 5, Y: 6}) {
		t.Errorf("OnHover saw %v, want (5, 6) first", seen)
	}
}
