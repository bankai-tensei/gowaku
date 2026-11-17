package gpu

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestFillRamp_Ends(t *testing.T) {
	row := make([]byte, RampWidth*4)
	FillRamp(row, []geom.Color{geom.Hex(0xff0000), geom.Hex(0x0000ff)}, []float32{0, 1})
	if got := [4]byte(row[:4]); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("first texel: got %v", got)
	}
	if got := [4]byte(row[len(row)-4:]); got != [4]byte{0, 0, 255, 255} {
		t.Errorf("last texel: got %v", got)
	}
	// Texel 51 lies at 51/255 = 0.2 of the way.
	if got := [4]byte(row[51*4 : 52*4]); got != [4]byte{204, 0, 51, 255} {
		t.Errorf("texel 51: got %v", got)
	}
}

func TestFillRamp_Premultiplied(t *testing.T) {
	row := make([]byte, RampWidth*4)
	half := geom.Color{R: 1, G: 1, B: 1, A: 0.5}
	FillRamp(row, []geom.Color{half}, []float32{0.3})
	for i := range RampWidth {
		if got := [4]byte(row[i*4 : i*4+4]); got != [4]byte{128, 128, 128, 128} {
			t.Fatalf("texel %d: got %v, want 128s", i, got)
		}
	}
}

func TestFillRamp_HardStopAndClampedEnds(t *testing.T) {
	row := make([]byte, RampWidth*4)
	r, g, b := geom.Hex(0xff0000), geom.Hex(0x00ff00), geom.Hex(0x0000ff)
	// Offsets of 0.2 and 0.8 leave room at both ends, and two stops at 0.5
	// meet in a hard edge.
	FillRamp(row, []geom.Color{r, g, b, r}, []float32{0.2, 0.5, 0.5, 0.8})
	texel := func(i int) [4]byte { return [4]byte(row[i*4 : i*4+4]) }
	if got := texel(10); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("before the first stop: got %v", got)
	}
	if got := texel(127); got[1] < 250 {
		t.Errorf("just before the edge: got %v, want green", got)
	}
	if got := texel(128); got[2] < 250 {
		t.Errorf("just after the edge: got %v, want blue", got)
	}
	if got := texel(250); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("after the last stop: got %v", got)
	}
}

func TestFillRamp_MatchesInstanceStops(t *testing.T) {
	colors := [MaxStops]geom.Color{geom.Hex(0x7b2ff7), geom.Hex(0xf107a3), geom.Hex(0xff8a00), geom.Hex(0x06d6a0)}
	offsets := [MaxStops]float32{0.1, 0.35, 0.6, 0.9}
	row := make([]byte, RampWidth*4)
	FillRamp(row, colors[:], offsets[:])
	var in Instance
	in.SetStops(&colors, &offsets, MaxStops)
	// The instance blends its padded stops the same way, so a row of up to
	// four stops is what the shader would compute at each texel.
	for i := range RampWidth {
		at := float32(i) / (RampWidth - 1)
		c := premultiplied(in.Color)
		mids := [...]geom.Color{in.MidColors[0], in.MidColors[1], in.EndColor}
		for k, m := range mids {
			w := stopWeight(at, in.StopOffsets[k], in.StopOffsets[k+1])
			n := premultiplied(m)
			for j := range c {
				c[j] += w * (n[j] - c[j])
			}
		}
		want := [4]byte{unorm8(c[0]), unorm8(c[1]), unorm8(c[2]), unorm8(c[3])}
		if got := [4]byte(row[i*4 : i*4+4]); got != want {
			t.Fatalf("texel %d: got %v, want %v", i, got, want)
		}
	}
}

func TestFillRamp_PanicsWithoutColors(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	FillRamp(make([]byte, RampWidth*4), nil, nil)
}

func TestGradientMode_Shape(t *testing.T) {
	if got := (GradientSweep | GradientRamp).Shape(); got != GradientSweep {
		t.Errorf("got %d, want GradientSweep", got)
	}
	if GradientRamp <= GradientSweep {
		t.Error("GradientRamp overlaps the shapes")
	}
}
