package soft

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// twoStops makes in a gradient from its Color to end, as render encodes one.
func twoStops(in *gpu.Instance, end geom.Color) {
	colors := [gpu.MaxStops]geom.Color{in.Color, end}
	offsets := [gpu.MaxStops]float32{0, 1}
	in.SetStops(&colors, &offsets, 2)
}

// stops makes in a gradient of the given stops.
func stops(in *gpu.Instance, offsets []float32, colors ...geom.Color) {
	var c [gpu.MaxStops]geom.Color
	var o [gpu.MaxStops]float32
	copy(c[:], colors)
	copy(o[:], offsets)
	in.SetStops(&c, &o, len(colors))
}

func TestGradient_ThreeStops(t *testing.T) {
	in := solid(0, 0, 64, 4, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear, geom.Offset{X: 64}
	stops(&in, []float32{0, 0.5, 1}, geom.Hex(0xff0000), geom.Hex(0x00ff00), geom.Hex(0x0000ff))
	img := render(t, frame{width: 64, height: 4}, quads(in))
	tests := []struct {
		name string
		x    int
		want float64
		// first says the pixel lies in the red to green half.
		first bool
	}{
		{"first half", 15, 15.5 / 32, true},
		{"middle", 31, 31.5 / 32, true},
		{"second half", 47, (47.5 - 32) / 32, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := over(green, blue, tt.want)
			if tt.first {
				want = over(red, green, tt.want)
			}
			checkNear(t, img, tt.x, 1, want, 1)
		})
	}
}

func TestGradient_StopOffsets(t *testing.T) {
	in := cover(0, 0, 100, 2, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear, geom.Offset{X: 100}
	stops(&in, []float32{0.25, 0.75}, geom.Hex(0xff0000), geom.Hex(0x0000ff))
	img := render(t, frame{width: 100, height: 2}, quads(in))
	checkPixel(t, img, 10, 0, red)
	checkNear(t, img, 49, 0, over(red, blue, (49.5-25)/50), 1)
	checkPixel(t, img, 90, 0, blue)
}

func TestGradient_HardStop(t *testing.T) {
	in := cover(0, 0, 10, 2, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear, geom.Offset{X: 10}
	r, b := geom.Hex(0xff0000), geom.Hex(0x0000ff)
	stops(&in, []float32{0, 0.5, 0.5, 1}, r, r, b, b)
	img := render(t, frame{width: 10, height: 2}, quads(in))
	checkPixel(t, img, 4, 0, red)
	checkPixel(t, img, 5, 0, blue)
}

func TestGradient_RadialFourStops(t *testing.T) {
	in := solid(0, 0, 64, 64, geom.Color{})
	in.Gradient = gpu.GradientRadial
	in.GradientStart, in.GradientEnd = geom.Offset{X: 32, Y: 32}, geom.Offset{X: 62, Y: 32}
	stops(&in, []float32{0, 0.3, 0.6, 1},
		geom.Hex(0xffffff), geom.Hex(0xff0000), geom.Hex(0x00ff00), geom.Hex(0x0000ff))
	img := render(t, frame{width: 64, height: 64}, quads(in))
	// The pixel centers lie 6.5 and 13.5 to the right of the center, with y 0.5
	// off the axis.
	t1 := math.Hypot(6.5, 0.5) / 30
	checkNear(t, img, 38, 32, over(white, red, t1/0.3), 1)
	t2 := math.Hypot(13.5, 0.5) / 30
	checkNear(t, img, 45, 32, over(red, green, (t2-0.3)/0.3), 1)
	// Past the last stop the last color goes on.
	checkPixel(t, img, 1, 1, blue)
}

func TestGradient_OneStopIsSolid(t *testing.T) {
	in := cover(0, 0, 10, 2, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear, geom.Offset{X: 10}
	stops(&in, []float32{0.5}, geom.Hex(0x00ff00))
	img := render(t, frame{width: 10, height: 2}, quads(in))
	checkPixel(t, img, 0, 0, green)
	checkPixel(t, img, 9, 0, green)
}
