package soft

import (
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// mipmappedOf uploads premultiplied RGBA pixels with their mip levels.
func mipmappedOf(t *testing.T, d *Device, width, height int, pix []byte) gpu.Texture {
	t.Helper()
	tx, err := d.CreateTexture(width, height, gpu.TextureRGBA8Mipmapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTexture(tx, rectOf(width, height), pix, width*4); err != nil {
		t.Fatal(err)
	}
	return tx
}

func filtered(img gpu.Texture, x, y, w, h float32, f gpu.ImageFilter) *gpu.Batch {
	b := imageBatch(img, x, y, w, h, 1)
	b.Instances[0].SetImageFilter(f)
	return b
}

func gray(v uint8) color.NRGBA { return color.NRGBA{R: v, G: v, B: v, A: 255} }

func TestMipmap_EnlargedLooksLinear(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := mipmappedOf(t, d, 2, 2, []byte{
		0, 0, 0, 255, 100, 0, 0, 255,
		0, 100, 0, 255, 100, 100, 0, 255,
	})
	defer tx.Destroy()
	for _, size := range []float32{2, 4, 7} {
		linear := renderOn(t, d, frame{width: 8, height: 8}, filtered(tx, 0, 0, size, size, gpu.FilterLinear))
		mipmap := renderOn(t, d, frame{width: 8, height: 8}, filtered(tx, 0, 0, size, size, gpu.FilterMipmap))
		for i := range linear.Pix {
			if linear.Pix[i] != mipmap.Pix[i] {
				t.Fatalf("size %v: byte %d differs", size, i)
			}
		}
	}
}

func TestMipmap_ShrunkReadsTheLevels(t *testing.T) {
	d := New()
	defer d.Destroy()
	// One white texel among black: level 1 is 0, 128, 0, 0 and level 2 is 64, 0.
	pix := make([]byte, 8*4)
	for i := 3; i < len(pix); i += 4 {
		pix[i] = 255
	}
	pix[12], pix[13], pix[14] = 255, 255, 255
	tx := mipmappedOf(t, d, 8, 1, pix)
	defer tx.Destroy()
	img := renderOn(t, d, frame{width: 2, height: 1}, filtered(tx, 0, 0, 2, 1, gpu.FilterMipmap))
	checkNear(t, img, 0, 0, gray(64), 1)
	checkNear(t, img, 1, 0, gray(0), 0)
	// Linear blends texels 0 and 1, and misses the white one.
	img = renderOn(t, d, frame{width: 2, height: 1}, filtered(tx, 0, 0, 2, 1, gpu.FilterLinear))
	checkNear(t, img, 0, 0, gray(0), 0)
}

func TestNearest_TakesTheTexelUnderThePixel(t *testing.T) {
	d := New()
	defer d.Destroy()
	tx := mipmappedOf(t, d, 4, 1, []byte{10, 10, 10, 255, 20, 20, 20, 255, 30, 30, 30, 255, 40, 40, 40, 255})
	defer tx.Destroy()
	// At a half, the center of a pixel lies on the edge between two texels,
	// and takes the one after it.
	img := renderOn(t, d, frame{width: 2, height: 1}, filtered(tx, 0, 0, 2, 1, gpu.FilterNearest))
	checkPixel(t, img, 0, 0, gray(20))
	checkPixel(t, img, 1, 0, gray(40))
	img = renderOn(t, d, frame{width: 8, height: 1}, filtered(tx, 0, 0, 8, 1, gpu.FilterNearest))
	for x, v := range []uint8{10, 10, 20, 20, 30, 30, 40, 40} {
		checkPixel(t, img, x, 0, gray(v))
	}
}

func TestLevelOfDetail_TakesTheLongerAxisAndIgnoresRotation(t *testing.T) {
	for _, c := range []struct {
		name  string
		rect  geom.Rect
		turn  geom.Affine
		scale float32
		want  float32
	}{
		{"quarter", geom.Rect{W: 16, H: 16}, geom.Identity(), 1, 2},
		{"quarter at scale 2", geom.Rect{W: 16, H: 16}, geom.Identity(), 2, 1},
		{"squeezed", geom.Rect{W: 16, H: 64}, geom.Identity(), 1, 2},
		{"turned", geom.Rect{W: 16, H: 16}, geom.Rotation(0.7), 1, 2},
		{"turned and squeezed", geom.Rect{W: 64, H: 16}, geom.Rotation(1.1), 1, 2},
		{"scaled by the transform", geom.Rect{W: 64, H: 64}, geom.Rotation(0.3).Mul(geom.Scaling(0.5, 0.5)), 1, 1},
	} {
		in := gpu.Instance{Rect: c.rect, GradientEnd: geom.Offset{X: 1, Y: 1}}
		in.SetTransform(c.turn)
		f := fragment{in: &in, scale: c.scale}
		if got := f.levelOfDetail(64, 64); geom.Abs32(got-c.want) > 1e-4 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
