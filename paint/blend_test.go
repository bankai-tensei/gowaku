package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestBlendMode_String(t *testing.T) {
	tests := map[BlendMode]string{
		BlendSrcOver:    "src-over",
		BlendModulate:   "modulate",
		BlendColorDodge: "color-dodge",
		BlendLuminosity: "luminosity",
		BlendMode(200):  "unknown",
	}
	for m, want := range tests {
		if got := m.String(); got != want {
			t.Errorf("%d: got %q, want %q", m, got, want)
		}
	}
	if BlendSrcOver != 0 {
		t.Error("SrcOver must stay the zero value")
	}
}

func TestCanvas_BlendGoesIntoTheCommand(t *testing.T) {
	c := newCanvas()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed, Blend: BlendMultiply})
	c.Save()
	c.PushBlend(BlendScreen)
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed, Blend: BlendDarken})
	c.DrawShadow(unitRect, 4, opaqueRed, geom.Offset{})
	c.Restore()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	want := []BlendMode{BlendMultiply, BlendScreen, BlendDarken, BlendScreen, BlendSrcOver}
	ops := c.DisplayList().Ops
	if len(ops) != len(want) {
		t.Fatalf("got %d ops", len(ops))
	}
	for i, m := range want {
		if ops[i].Blend != m {
			t.Errorf("op %d: got %v, want %v", i, ops[i].Blend, m)
		}
	}
}

func TestCanvas_BlendStopsALayer(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.PushBlend(BlendHue)
	if c.CanDrawLayer() {
		t.Error("each command of a layer blends on its own under PushBlend, which its pixels cannot")
	}
	c.Restore()
	if !c.CanDrawLayer() {
		t.Error("CanDrawLayer after the blend ended")
	}
}
