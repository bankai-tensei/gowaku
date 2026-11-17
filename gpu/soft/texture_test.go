package soft

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// imageOf uploads premultiplied RGBA pixels.
func imageOf(t *testing.T, d *Device, width, height int, pix []byte) gpu.Texture {
	t.Helper()
	tx, err := d.CreateTexture(width, height, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTexture(tx, rectOf(width, height), pix, width*4); err != nil {
		t.Fatal(err)
	}
	return tx
}

// imageBatch draws all of img into a rectangle.
func imageBatch(img gpu.Texture, x, y, w, h, alpha float32) *gpu.Batch {
	return &gpu.Batch{
		Instances: []gpu.Instance{{
			Rect:          geom.Rect{X: x, Y: y, W: w, H: h},
			Color:         geom.Color{A: alpha},
			GradientStart: geom.Offset{},
			GradientEnd:   geom.Offset{X: 1, Y: 1},
			Kind:          gpu.InstanceImage,
		}},
		Draws: []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Image: img}},
	}
}

func TestImage_OneToOneReadsTexelsExactly(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := imageOf(t, d, 2, 1, []byte{255, 0, 0, 255, 0, 0, 255, 255})
	defer tx.Destroy()
	img := renderOn(t, d, frame{width: 4, height: 2}, imageBatch(tx, 1, 0, 2, 1, 1))
	checkPixel(t, img, 1, 0, red)
	checkPixel(t, img, 2, 0, blue)
	checkPixel(t, img, 0, 0, white)
	checkPixel(t, img, 3, 0, white)
}

func TestImage_BilinearAndClamp(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := imageOf(t, d, 2, 1, []byte{0, 0, 0, 255, 200, 200, 200, 255})
	defer tx.Destroy()
	// Eight pixels over two texels: u = 1/16, 3/16, ... The centers of the texels
	// are at u = 1/4 and 3/4, and outside them the edge texel repeats.
	img := renderOn(t, d, frame{width: 8, height: 1}, imageBatch(tx, 0, 0, 8, 1, 1))
	want := []uint8{0, 0, 25, 75, 125, 175, 200, 200}
	for x, v := range want {
		checkNear(t, img, x, 0, color.NRGBA{R: v, G: v, B: v, A: 255}, 1)
	}
}

func TestImage_BilinearInBothAxes(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := imageOf(t, d, 2, 2, []byte{
		0, 0, 0, 255, 100, 0, 0, 255,
		0, 100, 0, 255, 100, 100, 0, 255,
	})
	defer tx.Destroy()
	// At 4x4 the middle pixels lie a quarter of a texel from the center of the image.
	img := renderOn(t, d, frame{width: 4, height: 4}, imageBatch(tx, 0, 0, 4, 4, 1))
	checkNear(t, img, 1, 1, color.NRGBA{R: 25, G: 25, A: 255}, 1)
	checkNear(t, img, 2, 1, color.NRGBA{R: 75, G: 25, A: 255}, 1)
	checkNear(t, img, 2, 2, color.NRGBA{R: 75, G: 75, A: 255}, 1)
}

func TestImage_TextureCoordinatesSelectARegion(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := imageOf(t, d, 2, 1, []byte{255, 0, 0, 255, 0, 255, 0, 255})
	defer tx.Destroy()
	b := imageBatch(tx, 0, 0, 2, 2, 1)
	b.Instances[0].GradientStart = geom.Offset{X: 0.5}
	img := renderOn(t, d, frame{width: 2, height: 2}, b)
	// u runs from 0.5 to 1: the left pixel is at 0.625, a quarter past the middle.
	checkNear(t, img, 1, 1, green, 1)
}

func TestImage_AlphaIsPremultiplied(t *testing.T) {
	d := New()
	defer d.Destroy()
	// Red at half alpha, premultiplied.
	tx := imageOf(t, d, 1, 1, []byte{128, 0, 0, 128})
	defer tx.Destroy()
	img := renderOn(t, d, frame{width: 1, height: 1}, imageBatch(tx, 0, 0, 1, 1, 1))
	checkNear(t, img, 0, 0, color.NRGBA{R: 255, G: 127, B: 127, A: 255}, 1)
}

func TestImage_OpacityScalesEverything(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := imageOf(t, d, 1, 1, []byte{0, 0, 0, 255})
	defer tx.Destroy()
	img := renderOn(t, d, frame{width: 1, height: 1}, imageBatch(tx, 0, 0, 1, 1, 0.5))
	checkPixel(t, img, 0, 0, halfBlack)
}

func TestUpdateTexture_RegionOfALargerImage(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx, err := d.CreateTexture(4, 4, gpu.TextureR8)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Destroy()
	src := make([]byte, 6*4)
	src[2*6+1] = 77 // (1, 2) with a stride of 6
	if err := d.UpdateTexture(tx, image.Rect(1, 2, 3, 3), src, 6); err != nil {
		t.Fatal(err)
	}
	got := tx.(*Texture)
	if v := got.pix[2*4+1]; v != 77 {
		t.Errorf("got %d, want 77", v)
	}
}

func TestCreateTexture_Errors(t *testing.T) {
	d := New()
	defer d.Destroy()
	if _, err := d.CreateTexture(0, 4, gpu.TextureR8); err == nil {
		t.Error("empty texture: no error")
	}
	if _, err := d.CreateTexture(4, 4, gpu.TextureFormat(9)); err == nil {
		t.Error("unknown format: no error")
	}
	if _, err := d.CreateTarget(4, 0); err == nil {
		t.Error("empty target: no error")
	}
}

func newTarget(t *testing.T, d *Device, width, height int) gpu.Texture {
	t.Helper()
	tx, err := d.CreateTarget(width, height)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tx.Destroy)
	return tx
}

func targetPixel(t gpu.Texture, x, y int) color.RGBA {
	tx := t.(*Texture)
	p := tx.pix[(y*tx.width+x)*4:]
	return color.RGBA{R: p[0], G: p[1], B: p[2], A: p[3]}
}

func checkTarget(t *testing.T, tx gpu.Texture, x, y int, want color.RGBA) {
	t.Helper()
	if got := targetPixel(tx, x, y); got != want {
		t.Errorf("target pixel (%d, %d): got %v, want %v", x, y, got, want)
	}
}

func TestTarget_StartsTransparent(t *testing.T) {
	d := New()
	defer d.Destroy()
	checkTarget(t, newTarget(t, d, 2, 2), 1, 1, color.RGBA{})
}

func TestTarget_HoldsPremultipliedColor(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 4, 4)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 4, 4, geom.Color{R: 1, A: 0.5})), 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 2, 2, color.RGBA{R: 128, A: 128})
}

func TestTarget_DrawsOverWhatItHolds(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 4, 1)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 2, 1, geom.Hex(0xff0000))), 1); err != nil {
		t.Fatal(err)
	}
	if err := d.DrawToTarget(tx, quads(solid(2, 0, 2, 1, geom.Hex(0x0000ff))), 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 1, 0, color.RGBA{R: 255, A: 255})
	checkTarget(t, tx, 2, 0, color.RGBA{B: 255, A: 255})
}

func TestTarget_ClearsComeFirst(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 6, 2)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 6, 2, geom.Hex(0xff0000))), 1); err != nil {
		t.Fatal(err)
	}
	b := quads(solid(3, 0, 1, 1, geom.Hex(0x00ff00)))
	b.Clears = []image.Rectangle{image.Rect(2, 0, 4, 1), image.Rect(5, 1, 100, 100)}
	if err := d.DrawToTarget(tx, b, 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 1, 0, color.RGBA{R: 255, A: 255})
	checkTarget(t, tx, 2, 0, color.RGBA{})
	checkTarget(t, tx, 3, 0, color.RGBA{G: 255, A: 255})
	checkTarget(t, tx, 4, 0, color.RGBA{R: 255, A: 255})
	checkTarget(t, tx, 5, 1, color.RGBA{})
	checkTarget(t, tx, 2, 1, color.RGBA{R: 255, A: 255})
}

func TestTarget_ClearsWithoutInstances(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 2, 1)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 2, 1, geom.Hex(0xff0000))), 1); err != nil {
		t.Fatal(err)
	}
	if err := d.DrawToTarget(tx, &gpu.Batch{Clears: []image.Rectangle{image.Rect(0, 0, 1, 1)}}, 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 0, 0, color.RGBA{})
	checkTarget(t, tx, 1, 0, color.RGBA{R: 255, A: 255})
}

func TestDraw_IgnoresClears(t *testing.T) {
	b := quads(solid(0, 0, 2, 2, geom.Hex(0x000000)))
	b.Clears = []image.Rectangle{image.Rect(0, 0, 2, 2)}
	checkPixel(t, render(t, frame{width: 2, height: 2}, b), 1, 1, black)
}

func TestTarget_UsesItsScale(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 8, 2)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 2, 1, geom.Hex(0x0000ff))), 2); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 3, 1, color.RGBA{B: 255, A: 255})
	checkTarget(t, tx, 4, 1, color.RGBA{})
}

func TestTarget_HasOneSamplePerPixel(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 8, 1)
	if err := d.DrawToTarget(tx, quads(cover(2.4, 0, 3, 1, geom.Hex(0x000000))), 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 2, 0, color.RGBA{A: 255})
}

func TestTarget_ClipLevelAboveZeroDrawsNothing(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 2, 1)
	b := &gpu.Batch{}
	addQuad(b, solid(0, 0, 2, 1, geom.Hex(0x000000)), 1)
	if err := d.DrawToTarget(tx, b, 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 0, 0, color.RGBA{})
}

func TestTarget_ScissorApplies(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 4, 1)
	b := quads(solid(0, 0, 4, 1, geom.Hex(0x000000)))
	b.Draws[0].Clipped, b.Draws[0].Clip = true, geom.Rect{W: 2, H: 1}
	if err := d.DrawToTarget(tx, b, 1); err != nil {
		t.Fatal(err)
	}
	checkTarget(t, tx, 1, 0, color.RGBA{A: 255})
	checkTarget(t, tx, 2, 0, color.RGBA{})
}

func TestTarget_SampledLikeAnImage(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := newTarget(t, d, 4, 4)
	if err := d.DrawToTarget(tx, quads(solid(0, 0, 4, 4, geom.Color{R: 1, A: 0.5})), 1); err != nil {
		t.Fatal(err)
	}
	img := renderOn(t, d, frame{width: 6, height: 6}, imageBatch(tx, 1, 1, 4, 4, 1))
	checkPixel(t, img, 2, 2, color.NRGBA{R: 255, G: 127, B: 127, A: 255})
	checkPixel(t, img, 0, 0, white)
}

func TestTarget_Panics(t *testing.T) {
	d := New()
	defer d.Destroy()
	target := newTarget(t, d, 4, 4)
	s, err := d.CreateSurface(nil, 4, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Destroy()
	plain, err := d.CreateTexture(4, 4, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Destroy()

	shaped := &gpu.Batch{}
	addFill(shaped, cover(0, 0, 4, 4, geom.Hex(0x000000)), square(0, 0, 4, 4), false, 0)
	mustPanic(t, "multisampled batch", func() error { return d.DrawToTarget(target, shaped, 1) })
	mustPanic(t, "not a target", func() error { return d.DrawToTarget(plain, quads(), 1) })
	mustPanic(t, "reads itself", func() error { return d.DrawToTarget(target, imageBatch(target, 0, 0, 4, 4, 1), 1) })

	if err := d.BeginFrame(s, geom.Color{}); err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "inside a frame", func() error { return d.DrawToTarget(target, quads(), 1) })
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
	if err := d.DrawToTarget(target, quads(), 1); err != nil {
		t.Errorf("after the frame: %v", err)
	}
}
