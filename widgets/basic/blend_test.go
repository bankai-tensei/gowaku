package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestBlend_RecordsALayerInAGroupWithTheMode(t *testing.T) {
	tree := core.NewTree(Blend{Mode: paint.BlendMultiply, Child: Container{Width: 20, Height: 20, Color: geom.Hex(0x336699)}})
	defer tree.Dispose()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	ops := cv.DisplayList().Ops
	if len(ops) != 3 || ops[0].Kind != paint.OpGroupPush || ops[0].Blend != paint.BlendMultiply || ops[1].Kind != paint.OpLayer {
		t.Fatalf("got %v, want one layer in a multiplied group", ops)
	}
}

func TestBlend_UpdateChangesTheMode(t *testing.T) {
	ro := Blend{Mode: paint.BlendHue}.CreateRenderObject().(*core.RenderRepaintBoundary)
	if ro.Blend() != paint.BlendHue {
		t.Fatalf("got %v", ro.Blend())
	}
	Blend{Mode: paint.BlendScreen}.UpdateRenderObject(ro)
	if ro.Blend() != paint.BlendScreen {
		t.Errorf("got %v, want screen", ro.Blend())
	}
}
