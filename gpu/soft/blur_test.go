package soft

import (
	"image"
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// backdrop adds a backdrop blur of sigma over r.
func backdrop(b *gpu.Batch, r geom.Rect, sigma float32) {
	in := gpu.Instance{Rect: r, Color: geom.Color{R: 1, G: 1, B: 1, A: 1}, Blur: sigma, Kind: gpu.InstanceBackdrop}
	addCall(b, in, gpu.DrawCall{Kind: gpu.DrawQuads, Bounds: geom.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}, Blur: sigma})
}

// edge draws the left half of a 100 by 60 frame black.
func edge() *gpu.Batch {
	return quads(solid(0, 0, 50, 60, geom.Color{A: 1}))
}

func TestBlur_SmoothsAnEdgeSymmetrically(t *testing.T) {
	b := edge()
	backdrop(b, geom.Rect{X: 20, Y: 10, W: 60, H: 40}, 6)
	img := render(t, frame{width: 100, height: 60, plain: true}, b)
	// Outside the box nothing changes.
	checkPixel(t, img, 10, 5, black)
	checkPixel(t, img, 90, 5, white)
	// Across the edge the gray rises steadily, and is symmetric about it.
	prev := -1
	for x := 30; x < 70; x++ {
		v := int(img.NRGBAAt(x, 30).R)
		if v < prev {
			t.Errorf("x %d: %d after %d, want no darker", x, v, prev)
		}
		prev = v
		mirror := int(img.NRGBAAt(99-x, 30).R)
		if math.Abs(float64(v+mirror-255)) > 3 {
			t.Errorf("x %d: %d and mirrored %d do not add up to white", x, v, mirror)
		}
	}
	if v := img.NRGBAAt(50, 30).R; v < 110 || v > 145 {
		t.Errorf("on the edge: %d, want about half", v)
	}
	if v := img.NRGBAAt(30, 30).R; v > 20 {
		t.Errorf("3 sigma inside the black: %d, want almost black", v)
	}
}

func TestBlur_KeepsAUniformColor(t *testing.T) {
	c := geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}
	b := quads(solid(0, 0, 100, 60, c))
	for _, sigma := range []float32{1, 6, 20, 60} {
		backdrop(b, geom.Rect{X: 10, Y: 10, W: 80, H: 40}, sigma)
	}
	img := render(t, frame{width: 100, height: 60, scale: 1.5}, b)
	want := img.NRGBAAt(2, 2)
	for _, p := range []image.Point{{20, 20}, {50, 30}, {85, 45}} {
		checkNear(t, img, p.X, p.Y, want, 1)
	}
}

func TestBlur_RoundedCornerAndOpacity(t *testing.T) {
	b := edge()
	in := gpu.Instance{
		Rect: geom.Rect{X: 30, Y: 10, W: 40, H: 40}, Radii: [4]float32{20, 20, 20, 20},
		Color: geom.Color{R: 1, G: 1, B: 1, A: 0.5}, Blur: 8, Kind: gpu.InstanceBackdrop,
	}
	addCall(b, in, gpu.DrawCall{Kind: gpu.DrawQuads, Bounds: geom.Rect{X: 29, Y: 9, W: 42, H: 42}, Blur: 8})
	img := render(t, frame{width: 100, height: 60, plain: true}, b)
	// The corner of the box is outside the circle.
	checkPixel(t, img, 31, 11, black)
	// Half the blurred gray over the black.
	if v := img.NRGBAAt(48, 30).R; v < 40 || v > 80 {
		t.Errorf("faded blur over black: %d", v)
	}
}

func TestBlur_FilteredImage(t *testing.T) {
	d := New()
	defer d.Destroy()
	target, err := d.CreateTarget(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	// A red square inside transparent room.
	if err := d.DrawToTarget(target, quads(solid(24, 24, 16, 16, geom.Color{R: 1, A: 1})), 1); err != nil {
		t.Fatal(err)
	}
	b := &gpu.Batch{}
	in := gpu.Instance{
		Rect: geom.Rect{X: 10, Y: 10, W: 64, H: 64}, Color: geom.Color{A: 1}, Blur: 4,
		GradientEnd: geom.Offset{X: 1, Y: 1}, Kind: gpu.InstanceFiltered,
	}
	addCall(b, in, gpu.DrawCall{Kind: gpu.DrawQuads, Image: target, Blur: 4})
	img := renderOn(t, d, frame{width: 100, height: 100, plain: true}, b)
	center := img.NRGBAAt(42, 42)
	if center.R != 255 || center.G > 150 {
		t.Errorf("center %v, want a red that the blur lightened only a little", center)
	}
	// The blur spreads past the square, and fades to nothing.
	if v := img.NRGBAAt(42, 31).G; v >= 250 {
		t.Errorf("past the square: %d, want some red", v)
	}
	checkPixel(t, img, 42, 15, white)
}

func TestBlur_InATarget(t *testing.T) {
	d := New()
	defer d.Destroy()
	target, err := d.CreateTarget(100, 60)
	if err != nil {
		t.Fatal(err)
	}
	b := edge()
	backdrop(b, geom.Rect{X: 20, Y: 10, W: 60, H: 40}, 6)
	if err := d.DrawToTarget(target, b, 1); err != nil {
		t.Fatal(err)
	}
	tx := target.(*Texture)
	got := tx.texel(50, 30)
	if got[0] != 0 || got[3] < 0.4 || got[3] > 0.6 {
		t.Errorf("on the edge %v, want half of opaque black", got)
	}
}
