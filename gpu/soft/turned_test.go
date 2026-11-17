package soft

import (
	"image"
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// turnedAbout returns in turned by radians around the center of its rectangle.
func turnedAbout(in gpu.Instance, radians float32) gpu.Instance {
	r := in.Rect
	c := geom.Offset{X: r.X + r.W/2, Y: r.Y + r.H/2}
	m := geom.Translation(c).Mul(geom.Rotation(radians)).Mul(geom.Translation(geom.Offset{X: -c.X, Y: -c.Y}))
	in.SetTransform(m)
	return in
}

func TestTurned_DiamondCoversCenterNotCorners(t *testing.T) {
	in := turnedAbout(solid(10, 10, 20, 20, geom.Hex(0xff0000)), math.Pi/4)
	img := render(t, frame{width: 40, height: 40, plain: true}, quads(in))
	checkPixel(t, img, 20, 20, red)
	// The corners of the upright square are outside the diamond.
	checkPixel(t, img, 11, 11, white)
	checkPixel(t, img, 28, 28, white)
	// The tips of the diamond reach about 14 pixels from the center.
	checkPixel(t, img, 20, 7, red)
	checkPixel(t, img, 20, 4, white)
}

func TestTurned_QuarterTurnMatchesUpright(t *testing.T) {
	upright := solid(12, 8, 16, 24, geom.Hex(0x2060c0))
	upright.Radii = [4]float32{3, 3, 3, 3}
	// A box of 24x16 at (8, 12), turned a quarter around its center, covers
	// the same pixels as the upright 16x24 one.
	turned := turnedAbout(solid(8, 12, 24, 16, geom.Hex(0x2060c0)), math.Pi/2)
	turned.Radii = upright.Radii
	f := frame{width: 40, height: 40, scale: 1.5, plain: true}
	a := render(t, f, quads(upright))
	b := render(t, f, quads(turned))
	for y := range 60 {
		for x := range 60 {
			ga, gb := a.NRGBAAt(x, y), b.NRGBAAt(x, y)
			if distance(ga.R, gb.R) > 2 || distance(ga.G, gb.G) > 2 || distance(ga.B, gb.B) > 2 {
				t.Fatalf("pixel (%d, %d): upright %v, turned %v", x, y, ga, gb)
			}
		}
	}
}

func TestTurned_EdgeIsAntialiased(t *testing.T) {
	in := turnedAbout(solid(10, 10, 20, 20, geom.Hex(0x000000)), 0.3)
	img := render(t, frame{width: 40, height: 40, plain: true}, quads(in))
	partial := 0
	for y := range 40 {
		for x := range 40 {
			if c := img.NRGBAAt(x, y).R; c > 20 && c < 235 {
				partial++
			}
		}
	}
	// A 20 pixel square has a perimeter of 80 pixels, nearly all of it at an
	// angle, so most edge pixels are partly covered.
	if partial < 40 {
		t.Errorf("%d partly covered pixels, want an antialiased edge", partial)
	}
}

func TestTurned_ZeroTransformIsUpright(t *testing.T) {
	in := solid(3.3, 4.7, 20, 11, geom.Hex(0x123456))
	turned := in
	turned.SetTransform(geom.Identity())
	if turned.Transform != ([6]float32{}) {
		t.Fatal("SetTransform of the identity must clear the transform")
	}
	f := frame{width: 30, height: 20}
	a, b := render(t, f, quads(in)), render(t, f, quads(turned))
	if string(a.Pix) != string(b.Pix) {
		t.Error("the identity must draw exactly what no transform draws")
	}
}

func TestTurned_GlyphIsFilteredWithinItsTexels(t *testing.T) {
	d := New()
	defer d.Destroy()
	atlas, err := d.CreateTexture(8, 8, gpu.TextureR8)
	if err != nil {
		t.Fatal(err)
	}
	// A 2x2 glyph at (2, 2) with a full neighbor at (4, 2), which must not
	// bleed in.
	pix := make([]byte, 64)
	for _, i := range []int{2*8 + 2, 2*8 + 3, 3*8 + 2, 3*8 + 3, 2*8 + 4, 3*8 + 4} {
		pix[i] = 255
	}
	if err := d.UpdateTexture(atlas, image.Rect(0, 0, 8, 8), pix, 8); err != nil {
		t.Fatal(err)
	}
	in := gpu.Instance{Rect: geom.Rect{X: 10, Y: 10, W: 2, H: 2}, Radii: [4]float32{2, 2}, Color: geom.Hex(0), Kind: gpu.InstanceGlyph}
	// A turn of almost nothing keeps the texels where they were.
	in.SetTransform(geom.Affine{A: 1, B: 1e-7, C: -1e-7, D: 1})
	b := &gpu.Batch{}
	addCall(b, in, gpu.DrawCall{Kind: gpu.DrawQuads, Texture: atlas})
	img := renderOn(t, d, frame{width: 20, height: 20, plain: true}, b)
	checkNear(t, img, 10, 10, black, 2)
	checkNear(t, img, 11, 11, black, 2)
	// The neighbor at (4, 2) would show at x = 12.
	checkNear(t, img, 12, 10, white, 2)
	checkNear(t, img, 9, 10, white, 2)
}

func TestTurned_QuadMarginGrowsWithShrink(t *testing.T) {
	in := solid(0, 0, 10, 10, geom.Hex(0))
	if got := quadMargin(&in, 2); got != 0.5 {
		t.Errorf("upright margin %v, want 0.5", got)
	}
	in.SetTransform(geom.Scaling(0.5, 2))
	if got := quadMargin(&in, 2); geom.Abs32(got-1) > 1e-5 {
		t.Errorf("turned margin %v, want 1: one physical pixel at half size", got)
	}
}
