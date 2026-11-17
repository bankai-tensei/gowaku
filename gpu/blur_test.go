package gpu

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestBlurFactor_Thresholds(t *testing.T) {
	for _, c := range []struct {
		sigma float32
		want  int
	}{{0.5, 2}, {7.99, 2}, {8, 4}, {31.9, 4}, {32, 8}, {200, 8}} {
		if got := BlurFactor(c.sigma); got != c.want {
			t.Errorf("BlurFactor(%v) = %d, want %d", c.sigma, got, c.want)
		}
	}
	if got := BlurRadius(5); got != 15 {
		t.Errorf("BlurRadius(5) = %d, want 15", got)
	}
	if got := BlurRadius(100); got != MaxBlurRadius {
		t.Errorf("BlurRadius(100) = %d, want the limit %d", got, MaxBlurRadius)
	}
}

func backdropCall(bounds geom.Rect, sigma float32) (*DrawCall, *Instance) {
	return &DrawCall{Count: 1, Bounds: bounds, Blur: sigma}, &Instance{Rect: bounds, Blur: sigma, Kind: InstanceBackdrop}
}

func TestPlanBlur_BackdropReadsThreeSigmasAround(t *testing.T) {
	c, in := backdropCall(geom.Rect{X: 100, Y: 100, W: 200, H: 100}, 10)
	b, ok := PlanBlur(c, in, 1.5, 1000, 800)
	if !ok {
		t.Fatal("nothing to draw")
	}
	// 15 physical pixels of sigma shrink by 4.
	if b.Factor != 4 || b.Sigma != 3.75 {
		t.Errorf("factor %d, sigma %v; want 4 and 3.75", b.Factor, b.Sigma)
	}
	// Grown by 30 logical pixels: 70 to 330, 70 to 230, at 1.5.
	if want := image.Rect(105, 105, 496, 346); b.Source != want {
		t.Errorf("source %v, want %v", b.Source, want)
	}
	if want := image.Rect(25, 25, 125, 88); b.Out != want {
		t.Errorf("out %v, want %v", b.Out, want)
	}
	var passes [3]Instance
	b.Passes(&passes, 1.5, false)
	if passes[0].Kind != InstanceBlurDown || passes[1].Kind != InstanceBlurX || passes[2].Kind != InstanceBlurY {
		t.Errorf("kinds %v %v %v", passes[0].Kind, passes[1].Kind, passes[2].Kind)
	}
	if got := passes[0].Radii; got != [4]float32{105, 105, 495, 345} {
		t.Errorf("source texels %v", got)
	}
	if got := passes[1].Radii; got != [4]float32{25, 25, 124, 87} {
		t.Errorf("shrunk texels %v", got)
	}
	if r := passes[2].Rect; geom.Floor32(r.X*1.5+0.5) != 25 || geom.Floor32((r.X+r.W)*1.5+0.5) != 125 {
		t.Errorf("rect %v does not cover the texels of out at 1.5", r)
	}
}

func TestPlanBlur_StaysInsideClipAndTarget(t *testing.T) {
	c, in := backdropCall(geom.Rect{X: -20, Y: 10, W: 100, H: 50}, 4)
	c.Clip, c.Clipped = geom.Rect{X: 0, Y: 0, W: 60, H: 300}, true
	b, ok := PlanBlur(c, in, 1, 400, 300)
	if !ok {
		t.Fatal("nothing to draw")
	}
	if want := image.Rect(0, 0, 61, 73); b.Source != want {
		t.Errorf("source %v, want %v", b.Source, want)
	}
	// Out never starts left of the texture.
	if b.Out.Min.X != 0 || b.Out.Min.Y != 0 {
		t.Errorf("out %v starts outside", b.Out)
	}
	c.Clip = geom.Rect{X: 500, Y: 500, W: 10, H: 10}
	if _, ok := PlanBlur(c, in, 1, 400, 300); ok {
		t.Error("a blur outside its clip draws")
	}
	c.Clipped, c.Blur = false, 0
	if _, ok := PlanBlur(c, in, 1, 400, 300); ok {
		t.Error("a blur of zero draws")
	}
}

func TestPlanBlur_ImageReadsItsTexels(t *testing.T) {
	c := &DrawCall{Count: 1, Blur: 2}
	in := &Instance{Kind: InstanceFiltered, Blur: 2, GradientStart: geom.Offset{X: 0.25, Y: 0.5}, GradientEnd: geom.Offset{X: 0.5, Y: 0.75}}
	b, ok := PlanBlur(c, in, 2, 2048, 2048)
	if !ok {
		t.Fatal("nothing to draw")
	}
	if want := image.Rect(512, 1024, 1024, 1536); b.Source != want || b.Factor != 2 {
		t.Errorf("source %v factor %d, want %v and 2", b.Source, b.Factor, want)
	}
	var passes [3]Instance
	b.Passes(&passes, 2, true)
	if passes[0].Blur != 1 {
		t.Error("the first pass does not read the image")
	}
}

func TestBatch_CountBlurs(t *testing.T) {
	b := Batch{Draws: []DrawCall{{Count: 1, Blur: 2}, {Count: 3}, {Count: 0, Blur: 1}, {Count: 1, Blur: 1}}}
	if got := b.CountBlurs(); got != 2 {
		t.Errorf("CountBlurs = %d, want 2", got)
	}
}

// The weights of three box blurs add up to one, and spread as far as the boxes
// reach together.
func TestBoxWeight_SumsToOne(t *testing.T) {
	for _, sigma := range []float32{2, 2.7, 5, 11.5, 40} {
		widths, before := BoxSizes(sigma)
		d := [3]float32{float32(widths[0]), float32(widths[1]), float32(widths[2])}
		span := widths[0] + widths[1] + widths[2] - 2
		var sum, mean float32
		for m := range span {
			w := BoxWeight(float32(m), d)
			sum += w
			mean += w * float32(m-before)
		}
		if geom.Abs32(sum-1) > 1e-5 || geom.Abs32(mean) > 0.51 {
			t.Errorf("sigma %v: sum %v, mean %v", sigma, sum, mean)
		}
		if BoxWeight(float32(span), d) != 0 || BoxWeight(-1, d) != 0 {
			t.Errorf("sigma %v: weight past the span", sigma)
		}
	}
}
