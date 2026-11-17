package soft

import (
	"image/color"
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// halfBlack is black at alpha 0.5 over white: alpha rounds up to 128, and white
// keeps 127/255 of itself.
var halfBlack = color.NRGBA{R: 127, G: 127, B: 127, A: 255}

func TestRRect_InteriorAndOutside(t *testing.T) {
	img := render(t, frame{width: 40, height: 30}, quads(solid(10, 5, 20, 10, geom.Hex(0x0000ff))))
	checkPixel(t, img, 10, 5, blue)
	checkPixel(t, img, 29, 14, blue)
	checkPixel(t, img, 9, 5, white)
	checkPixel(t, img, 30, 14, white)
	checkPixel(t, img, 10, 15, white)
}

func TestRRect_FractionalEdgeIsHalfCovered(t *testing.T) {
	// The pixel center lies on the edge at x = 20.5: distance 0 gives coverage 0.5.
	// The vertex shader's margin is what lets the quad reach that far.
	img := render(t, frame{width: 40, height: 20}, quads(solid(20.5, 2, 10, 10, geom.Hex(0x000000))))
	checkPixel(t, img, 19, 6, white)
	checkPixel(t, img, 20, 6, halfBlack)
	checkPixel(t, img, 21, 6, black)
	checkPixel(t, img, 30, 6, halfBlack)
	checkPixel(t, img, 31, 6, white)
}

func TestRRect_EdgeIsOnePhysicalPixelWide(t *testing.T) {
	// At scale 2 logical 10.25 is physical 20.5, which is the same half covered
	// pixel: the edge does not get wider with the scale.
	img := render(t, frame{width: 60, height: 40, scale: 2}, quads(solid(10.25, 2, 10, 10, geom.Hex(0x000000))))
	checkPixel(t, img, 19, 12, white)
	checkPixel(t, img, 20, 12, halfBlack)
	checkPixel(t, img, 21, 12, black)
}

func TestRRect_Corner(t *testing.T) {
	in := solid(0, 0, 40, 40, geom.Hex(0x000000))
	in.Radii = [4]float32{16, 0, 8, 0}
	img := render(t, frame{width: 40, height: 40}, quads(in))
	checkPixel(t, img, 0, 0, white)
	checkPixel(t, img, 39, 0, black)
	checkPixel(t, img, 0, 39, black)
	checkPixel(t, img, 39, 39, white)
	// A pixel on the arc of the top left corner.
	x, y := 4, 6
	d := math.Hypot(16-(float64(x)+0.5), 16-(float64(y)+0.5)) - 16
	checkNear(t, img, x, y, over(white, black, math.Min(math.Max(0.5-d, 0), 1)), 1)
}

func TestRRect_Border(t *testing.T) {
	in := solid(4, 4, 20, 20, geom.Hex(0x00ff00))
	in.BorderWidth = 2
	in.BorderColor = geom.Hex(0xff0000)
	img := render(t, frame{width: 28, height: 28}, quads(in))
	checkPixel(t, img, 4, 10, red)
	checkPixel(t, img, 5, 10, red)
	checkPixel(t, img, 6, 10, green)
	checkPixel(t, img, 23, 10, red)
	checkPixel(t, img, 22, 10, red)
	checkPixel(t, img, 21, 10, green)
}

func TestRRect_TranslucentBlendsPremultiplied(t *testing.T) {
	img := render(t, frame{width: 10, height: 10}, quads(solid(0, 0, 10, 10, geom.Color{R: 1, A: 0.5})))
	checkPixel(t, img, 5, 5, color.NRGBA{R: 255, G: 127, B: 127, A: 255})
}

// A blend reads the 8 bits that the last one stored, not what it computed.
func TestBlend_ReadsStoredValues(t *testing.T) {
	const layers = 200
	const alpha = 0.03
	instances := make([]gpu.Instance, layers)
	keep := 255 - math.Floor(alpha*255+0.5)
	stored, exact := 255.0, 255.0
	for i := range instances {
		instances[i] = solid(0, 0, 4, 4, geom.Color{A: alpha})
		stored = math.Floor(stored*keep/255 + 0.5)
		exact *= keep / 255
	}
	if stored == math.Floor(exact+0.5) {
		t.Fatalf("the test cannot tell the two apart: both give %v", stored)
	}
	img := render(t, frame{width: 4, height: 4}, quads(instances...))
	v := uint8(stored)
	checkPixel(t, img, 1, 1, color.NRGBA{R: v, G: v, B: v, A: 255})
}

func TestGradient(t *testing.T) {
	start, end := geom.Hex(0xff0000), geom.Hex(0x0000ff)
	tests := []struct {
		name     string
		mode     gpu.GradientMode
		to       geom.Offset
		x, y     int
		progress float64
	}{
		{"linear: start", gpu.GradientLinear, geom.Offset{X: 64}, 0, 3, 0.5 / 64},
		{"linear: middle", gpu.GradientLinear, geom.Offset{X: 64}, 31, 3, 31.5 / 64},
		{"linear: end", gpu.GradientLinear, geom.Offset{X: 64}, 63, 3, 63.5 / 64},
		{"linear: along y", gpu.GradientLinear, geom.Offset{Y: 64}, 3, 40, 40.5 / 64},
		{"linear: diagonal", gpu.GradientLinear, geom.Offset{X: 64, Y: 64}, 10, 20, (10.5 + 20.5) / 128},
		{"radial", gpu.GradientRadial, geom.Offset{X: 40}, 12, 9, math.Hypot(12.5, 9.5) / 40},
		{"radial: past the end", gpu.GradientRadial, geom.Offset{X: 10}, 40, 40, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := solid(0, 0, 64, 64, start)
			in.Gradient, in.GradientEnd = tt.mode, tt.to
			twoStops(&in, end)
			img := render(t, frame{width: 64, height: 64}, quads(in))
			checkNear(t, img, tt.x, tt.y, over(red, blue, tt.progress), 1)
		})
	}
}

func TestGradient_MixesPremultiplied(t *testing.T) {
	in := solid(0, 0, 2, 2, geom.Color{R: 1, A: 1})
	twoStops(&in, geom.Color{})
	in.Gradient = gpu.GradientLinear
	in.GradientStart, in.GradientEnd = geom.Offset{X: -100}, geom.Offset{X: 102}
	img := render(t, frame{width: 2, height: 2}, quads(in))
	// Halfway to transparent: alpha 0.5 and red 0.5, which keeps a pure red over
	// white instead of darkening it.
	checkNear(t, img, 0, 0, color.NRGBA{R: 255, G: 128, B: 128, A: 255}, 1)
}

// shadowAt is the exact blur of a square corner box.
func shadowAt(x, y, left, top, right, bottom, sigma float64) float64 {
	axis := func(v, low, high float64) float64 {
		return 0.5 * (math.Erf((high-v)/(sigma*math.Sqrt2)) - math.Erf((low-v)/(sigma*math.Sqrt2)))
	}
	return axis(x, left, right) * axis(y, top, bottom)
}

func TestShadow_FollowsGaussianBlur(t *testing.T) {
	const blur = 16
	in := solid(40, 40, 60, 40, geom.Hex(0x000000))
	in.Kind, in.Blur = gpu.InstanceShadow, blur
	img := render(t, frame{width: 140, height: 120}, quads(in))
	for _, p := range [][2]int{{70, 60}, {40, 60}, {30, 60}, {100, 80}, {36, 36}, {105, 60}, {70, 85}} {
		coverage := shadowAt(float64(p[0])+0.5, float64(p[1])+0.5, 40, 40, 100, 80, blur/2)
		// Six slices are an approximation of the integral, and so is the erf.
		checkNear(t, img, p[0], p[1], over(white, black, coverage), 4)
	}
}

func TestShadow_StopsAtItsMargin(t *testing.T) {
	const blur = 10
	in := solid(40, 40, 20, 20, geom.Hex(0x000000))
	in.Kind, in.Blur = gpu.InstanceShadow, blur
	img := render(t, frame{width: 100, height: 100}, quads(in))
	// The quad ends 1 + 1.5 * blur = 16 pixels out.
	checkPixel(t, img, 23, 50, white)
	if got := img.NRGBAAt(26, 50); got.R == 255 {
		t.Errorf("no shadow near the edge of the quad: %v", got)
	}
}

func TestShadow_RoundedCornerIsLighter(t *testing.T) {
	box := solid(20, 20, 60, 60, geom.Hex(0x000000))
	box.Kind, box.Blur = gpu.InstanceShadow, 8
	round := box
	round.Radii = [4]float32{20, 20, 20, 20}
	square := render(t, frame{width: 100, height: 100}, quads(box))
	rounded := render(t, frame{width: 100, height: 100}, quads(round))
	if a, b := square.NRGBAAt(22, 22).R, rounded.NRGBAAt(22, 22).R; a >= b {
		t.Errorf("corner: square %d, rounded %d", a, b)
	}
	checkPixel(t, rounded, 50, 50, square.NRGBAAt(50, 50))
}

func atlasWith(t *testing.T, d *Device, width, height int, pix []byte) gpu.Texture {
	t.Helper()
	tx, err := d.CreateTexture(width, height, gpu.TextureR8)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTexture(tx, rectOf(width, height), pix, width); err != nil {
		t.Fatal(err)
	}
	return tx
}

func glyphBatch(in gpu.Instance, atlas gpu.Texture) *gpu.Batch {
	in.Kind = gpu.InstanceGlyph
	return &gpu.Batch{
		Instances: []gpu.Instance{in},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Texture: atlas}},
	}
}

func TestGlyph_ReadsTexelsOneToOne(t *testing.T) {
	d := New()
	defer d.Destroy()
	// The glyph is the 2x2 block at (1, 1).
	atlas := atlasWith(t, d, 4, 3, []byte{
		9, 9, 9, 9,
		9, 255, 0, 9,
		9, 128, 64, 9,
	})
	defer atlas.Destroy()
	in := solid(5, 6, 2, 2, geom.Hex(0x000000))
	in.Radii[0], in.Radii[1] = 1, 1
	img := renderOn(t, d, frame{width: 12, height: 12}, glyphBatch(in, atlas))
	checkPixel(t, img, 4, 6, white)
	checkPixel(t, img, 5, 6, black)
	checkPixel(t, img, 6, 6, white)
	checkNear(t, img, 5, 7, over(white, black, 128.0/255), 1)
	checkNear(t, img, 6, 7, over(white, black, 64.0/255), 1)
	checkPixel(t, img, 7, 7, white)
}

func TestGlyph_ColorAndAlpha(t *testing.T) {
	d := New()
	defer d.Destroy()
	atlas := atlasWith(t, d, 1, 1, []byte{255})
	defer atlas.Destroy()
	img := renderOn(t, d, frame{width: 4, height: 4}, glyphBatch(solid(1, 1, 1, 1, geom.Color{B: 1, A: 0.5}), atlas))
	checkPixel(t, img, 1, 1, color.NRGBA{R: 127, G: 127, B: 255, A: 255})
}

func TestGlyph_OriginRoundsHalvesToEven(t *testing.T) {
	d := New()
	defer d.Destroy()
	atlas := atlasWith(t, d, 3, 1, []byte{10, 255, 20})
	defer atlas.Destroy()
	// At scale 1.5 the logical x of 3 is physical 4.5, which rounds to 4: texel 1
	// lands on pixel 5 with the first texel at 0 and origin 4. Pixel 4 is outside
	// the quad, which starts at the 4.5 sample of pixel 4 and takes in its center.
	in := solid(3, 0, 2, 2, geom.Hex(0x000000))
	img := renderOn(t, d, frame{width: 10, height: 4, scale: 1.5, plain: true}, glyphBatch(in, atlas))
	checkPixel(t, img, 5, 0, black)
	checkNear(t, img, 4, 0, over(white, black, 10.0/255), 1)
	checkNear(t, img, 6, 0, over(white, black, 20.0/255), 1)
}

func TestGlyph_ReadsZeroOutsideTheAtlas(t *testing.T) {
	d := New()
	defer d.Destroy()
	atlas := atlasWith(t, d, 1, 1, []byte{255})
	defer atlas.Destroy()
	img := renderOn(t, d, frame{width: 4, height: 4}, glyphBatch(solid(0, 0, 3, 3, geom.Hex(0x000000)), atlas))
	checkPixel(t, img, 0, 0, black)
	checkPixel(t, img, 1, 0, white)
	checkPixel(t, img, 0, 1, white)
}

func TestCover_HasNoEdgeOfItsOwn(t *testing.T) {
	// No margin, no SDF: a center on the left edge is in, on the right edge out.
	in := cover(2.5, 0, 3, 4, geom.Hex(0x000000))
	plain := render(t, frame{width: 8, height: 4, plain: true}, quads(in))
	checkPixel(t, plain, 1, 1, white)
	checkPixel(t, plain, 2, 1, black)
	checkPixel(t, plain, 4, 1, black)
	checkPixel(t, plain, 5, 1, white)

	// With samples, two of the four in the edge pixels lie right of x.5.
	multi := render(t, frame{width: 8, height: 4}, quads(in))
	checkPixel(t, multi, 2, 1, mix(white, black, 2))
	checkPixel(t, multi, 3, 1, black)
	checkPixel(t, multi, 5, 1, mix(white, black, 2))
}

func TestCover_Gradient(t *testing.T) {
	in := cover(0, 0, 10, 2, geom.Hex(0xff0000))
	in.Gradient, in.GradientEnd = gpu.GradientLinear, geom.Offset{X: 10}
	twoStops(&in, geom.Hex(0x0000ff))
	img := render(t, frame{width: 10, height: 2}, quads(in))
	checkNear(t, img, 7, 0, over(red, blue, 0.75), 1)
}

func TestFragment_ShaderRunsAtThePixelCenterOnly(t *testing.T) {
	// A sharp gradient that steps inside the edge pixel: with one run per pixel,
	// all samples of the pixel get the color of its center.
	in := cover(0, 0, 8, 2, geom.Hex(0xff0000))
	in.Gradient = gpu.GradientLinear
	twoStops(&in, geom.Hex(0x0000ff))
	in.GradientStart, in.GradientEnd = geom.Offset{X: 3.4}, geom.Offset{X: 3.45}
	img := render(t, frame{width: 8, height: 2}, quads(in))
	checkPixel(t, img, 3, 0, blue)
	checkPixel(t, img, 2, 0, red)
}
