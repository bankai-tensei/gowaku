package gowakutest_test

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// TestTransform_RotatedButtonIsHitInsideItsShape turns a 160x40 button by 45
// degrees. The pointer must reach it along its turned length, beyond its upright
// box, and miss it in the corners of the box around it.
func TestTransform_RotatedButtonIsHitInsideItsShape(t *testing.T) {
	presses := 0
	root := basic.Center{Child: basic.Rotate(math.Pi/4, basic.SizedBox{
		Width: 160, Height: 40,
		Child: control.Button{Label: "Turned", OnPress: func() { presses++ }},
	})}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: 400, Height: 400, Fonts: goFonts.Fonts})

	n, ok := tt.Find("Turned")
	if !ok {
		t.Fatal("no button")
	}
	// Semantics reports the upright box around the turned one: about 141 wide.
	if n.Rect.W < 130 || n.Rect.H < 130 {
		t.Fatalf("semantics rect %+v, want the bounds of the turned button", n.Rect)
	}
	center := geom.Offset{X: 200, Y: 200}

	tt.TapAt(center)
	if presses != 1 {
		t.Fatalf("a tap in the middle pressed %d times, want 1", presses)
	}
	// 60 pixels along the turned length: outside the upright 160x40 box, which
	// ends 20 pixels below the middle, but inside the turned button.
	along := geom.Offset{X: center.X + 60*math.Sqrt2/2, Y: center.Y + 60*math.Sqrt2/2}
	tt.TapAt(along)
	if presses != 2 {
		t.Errorf("a tap along the turned button pressed %d times in all, want 2", presses)
	}
	// The top right corner of the box around it is far from the turned shape.
	corner := geom.Offset{X: n.Rect.X + n.Rect.W - 6, Y: n.Rect.Y + 6}
	tt.TapAt(corner)
	if presses != 2 {
		t.Errorf("a tap in the corner of the bounds pressed the button")
	}
	// Tap through semantics, as a test usually does, lands in the middle.
	tt.Tap("Turned")
	if presses != 3 {
		t.Errorf("Tap pressed %d times in all, want 3", presses)
	}
}
