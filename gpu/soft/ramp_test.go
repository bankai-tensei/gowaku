package soft

import (
	"image"
	"math"
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// rampsWith returns a texture of rows ramps, whose row holds the given stops.
func rampsWith(t *testing.T, d *Device, rows, row int, offsets []float32, colors ...geom.Color) gpu.Texture {
	t.Helper()
	tex, err := d.CreateTexture(gpu.RampWidth, rows, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	pix := make([]byte, gpu.RampWidth*4*rows)
	stride := gpu.RampWidth * 4
	gpu.FillRamp(pix[row*stride:(row+1)*stride], colors, offsets)
	if err := d.UpdateTexture(tex, image.Rect(0, 0, gpu.RampWidth, rows), pix, stride); err != nil {
		t.Fatal(err)
	}
	return tex
}

// fourStops are stops on texels of the ramps, where a ramp agrees with the
// instance up to rounding. Between texels the ramp is straight, so a stop off
// a texel bends there and nowhere else.
var (
	fourOffsets = []float32{0, 85.0 / 255, 170.0 / 255, 1}
	fourColors  = []geom.Color{geom.Hex(0x7b2ff7), geom.Hex(0xf107a3), {R: 1, G: 0.54, A: 0.6}, geom.Hex(0x06d6a0)}
)

// A gradient of four stops read from the ramps shades within one step of the
// same gradient carried by the instance.
func TestRamp_ShadesAsInstanceStops(t *testing.T) {
	const width = 701
	d := New()
	defer d.Destroy()
	ramps := rampsWith(t, d, 4, 2, fourOffsets, fourColors...).(*Texture)
	inline := cover(0, 0, width, 1, geom.Color{})
	inline.Gradient, inline.GradientEnd = gpu.GradientLinear, geom.Offset{X: width}
	stops(&inline, fourOffsets, fourColors...)
	ramped := inline
	ramped.Gradient |= gpu.GradientRamp
	ramped.StopOffsets = [4]float32{2}

	worst := 0
	for x := range width * 4 {
		at := geom.Offset{X: (float32(x) + 0.5) / 4, Y: 0.5}
		wantColor, _ := (&fragment{in: &inline, scale: 1, logical: at}).shade()
		gotColor, _ := (&fragment{in: &ramped, scale: 1, ramps: ramps, logical: at}).shade()
		want, got := wantColor.stored(), gotColor.stored()
		for c := range got {
			worst = max(worst, distance(got[c], want[c]))
		}
	}
	if worst > 1 {
		t.Errorf("worst difference %d, want at most 1", worst)
	}
}

// Drawn opaque, the frames agree within one step too.
func TestRamp_DrawsAsInstanceStops(t *testing.T) {
	const width, height = 701, 3
	colors := slices.Clone(fourColors)
	colors[2].A = 1
	inline := cover(0, 0, width, height, geom.Color{})
	inline.Gradient, inline.GradientEnd = gpu.GradientLinear, geom.Offset{X: width}
	stops(&inline, fourOffsets, colors...)

	d := New()
	defer d.Destroy()
	ramped := inline
	ramped.Gradient |= gpu.GradientRamp
	ramped.StopOffsets = [4]float32{2}
	batch := quads(ramped)
	batch.Ramps = rampsWith(t, d, 4, 2, fourOffsets, colors...)

	want := renderOn(t, d, frame{width: width, height: height}, quads(inline))
	got := renderOn(t, d, frame{width: width, height: height}, batch)
	worst := 0
	for x := range width {
		a, b := got.NRGBAAt(x, 1), want.NRGBAAt(x, 1)
		worst = max(worst, distance(a.R, b.R), distance(a.G, b.G), distance(a.B, b.B), distance(a.A, b.A))
	}
	if worst > 1 {
		t.Errorf("worst difference %d, want at most 1", worst)
	}
}

func TestRamp_SixStops(t *testing.T) {
	d := New()
	defer d.Destroy()
	r, g, b := geom.Hex(0xff0000), geom.Hex(0x00ff00), geom.Hex(0x0000ff)
	in := cover(0, 0, 255, 2, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear|gpu.GradientRamp, geom.Offset{X: 255}
	batch := quads(in)
	batch.Ramps = rampsWith(t, d, 1, 0, []float32{0, 0.2, 0.2, 0.6, 0.6, 1}, r, r, g, g, b, b)
	img := renderOn(t, d, frame{width: 255, height: 2}, batch)
	checkPixel(t, img, 10, 0, red)
	checkPixel(t, img, 100, 0, green)
	checkPixel(t, img, 200, 0, blue)
}

func TestRamp_PastTheEnds(t *testing.T) {
	d := New()
	defer d.Destroy()
	in := cover(0, 0, 30, 2, geom.Color{})
	// The axis covers only the middle third.
	in.Gradient = gpu.GradientLinear | gpu.GradientRamp
	in.GradientStart, in.GradientEnd = geom.Offset{X: 10}, geom.Offset{X: 20}
	batch := quads(in)
	batch.Ramps = rampsWith(t, d, 1, 0, []float32{0, 1}, geom.Hex(0xff0000), geom.Hex(0x0000ff))
	img := renderOn(t, d, frame{width: 30, height: 2}, batch)
	checkPixel(t, img, 2, 0, red)
	checkPixel(t, img, 27, 0, blue)
}

func TestRamp_MissingTextureIsTransparent(t *testing.T) {
	in := cover(0, 0, 10, 2, geom.Color{})
	in.Gradient, in.GradientEnd = gpu.GradientLinear|gpu.GradientRamp, geom.Offset{X: 10}
	img := render(t, frame{width: 10, height: 2}, quads(in))
	checkPixel(t, img, 5, 0, white)
}

func TestGradient_Sweep(t *testing.T) {
	in := cover(0, 0, 64, 64, geom.Color{})
	in.Gradient = gpu.GradientSweep
	in.GradientStart, in.GradientEnd = geom.Offset{X: 32, Y: 32}, geom.Offset{X: 64, Y: 32}
	r, b := geom.Hex(0xff0000), geom.Hex(0x0000ff)
	// The first half turn, clockwise from the right through the bottom, is red.
	stops(&in, []float32{0, 0.5, 0.5, 1}, r, r, b, b)
	img := render(t, frame{width: 64, height: 64}, quads(in))
	checkPixel(t, img, 32, 50, red)
	checkPixel(t, img, 10, 40, red)
	checkPixel(t, img, 32, 10, blue)
	checkPixel(t, img, 50, 20, blue)
}

func TestGradient_SweepTurnsSmoothly(t *testing.T) {
	in := cover(0, 0, 64, 64, geom.Color{})
	in.Gradient = gpu.GradientSweep
	// The axis points down, so a quarter turn is to the left.
	in.GradientStart, in.GradientEnd = geom.Offset{X: 32, Y: 32}, geom.Offset{X: 32, Y: 40}
	in.Color = geom.Hex(0x000000)
	twoStops(&in, geom.Hex(0xffffff))
	img := render(t, frame{width: 64, height: 64}, quads(in))
	// Pixel (2, 32) has its center at (-29.5, 0.5) from the start.
	turn := 0.25 - math.Atan2(0.5, 29.5)/(2*math.Pi)
	checkNear(t, img, 2, 32, over(black, white, turn), 1)
	// Pixel (32, 2), above the start, is just past half a turn.
	turn = 0.5 + math.Atan2(0.5, 29.5)/(2*math.Pi)
	checkNear(t, img, 32, 2, over(black, white, turn), 1)
}

func TestGradient_Matrix(t *testing.T) {
	in := cover(0, 0, 64, 64, geom.Color{})
	in.Gradient = gpu.GradientRadial
	// A circle of radius 10 stretched three times wide, centered at 32, 32.
	in.GradientStart, in.GradientEnd = geom.Offset{X: 32, Y: 32}, geom.Offset{X: 42, Y: 32}
	in.GradientMatrix = [4]float32{1.0 / 3, 0, 0, 1}
	in.Color = geom.Hex(0x000000)
	twoStops(&in, geom.Hex(0xffffff))
	img := render(t, frame{width: 64, height: 64}, quads(in))
	// Pixel (56, 32) is 24.5 right and 0.5 down.
	at := math.Hypot(24.5/3, 0.5) / 10
	checkNear(t, img, 56, 32, over(black, white, at), 1)
	// Pixel (32, 40) is 0.5 right and 8.5 down.
	at = math.Hypot(0.5/3, 8.5) / 10
	checkNear(t, img, 32, 40, over(black, white, at), 1)
	checkPixel(t, img, 32, 50, white)
}
