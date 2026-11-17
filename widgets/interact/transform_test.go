package interact

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
)

func TestTransform_PointerArrivesInLocalCoordinates(t *testing.T) {
	var at geom.Offset
	tree := core.NewTree(basic.Center{Child: basic.Rotate(math.Pi/2, basic.SizedBox{
		Width: 80, Height: 40,
		Child: GestureDetector{OnTapDownAt: func(t layout.Tap) { at = t.Pos }, Child: basic.Container{Color: geom.Hex(0)}},
	})})
	defer tree.Dispose()
	testutil.RenderTree(tree, transformPage)
	// Turned a quarter around (100, 100), the card's top left corner (60, 80)
	// lands at (120, 60), and its local x axis points down. A press 10 below
	// and 5 left of that corner is at (10, 5) on the card.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 115, Y: 70}, Button: input.ButtonPrimary})
	if geom.Abs32(at.X-10) > 1e-3 || geom.Abs32(at.Y-5) > 1e-3 {
		t.Errorf("got %v, want (10, 5)", at)
	}
}

var transformPage = geom.Size{W: 200, H: 200}
