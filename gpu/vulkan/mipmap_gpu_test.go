//go:build gpu && (linux || windows)

package vulkan

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
)

// A shrunk checkerboard of single texels turns gray with mipmaps, stays black or
// white when nearest, and drawing it every frame allocates nothing once it is on
// the GPU.
func TestImage_MipmapsDoNotAllocate(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	const n = 64
	m := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			m.SetNRGBA(x, y, color.NRGBA{A: 255})
			if (x+y)%2 == 1 {
				m.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	picture := paint.NewImage(m)
	res := render.NewResources(text.NewAtlas(testutil.NewShaper(t)), f.device)
	t.Cleanup(res.Destroy)
	var cv paint.Canvas
	var batch gpu.Batch
	encode := func() {
		cv.Reset()
		cv.DrawImage(picture, geom.Rect{X: 16, Y: 16, W: 16, H: 16}, 1)
		cv.DrawImageQuality(picture, geom.Rect{X: 64, Y: 16, W: 16, H: 16}, 1, paint.FilterNearest)
		cv.Save()
		cv.Transform(geom.Translation(geom.Offset{X: 150, Y: 40}).Mul(geom.Rotation(0.5)))
		cv.DrawImage(picture, geom.Rect{W: 20, H: 20}, 1)
		cv.Restore()
		render.Encode(cv.DisplayList(), res, 1, &batch)
		if err := res.Finish(&batch); err != nil {
			t.Fatal(err)
		}
	}
	for range 4 {
		encode()
		f.frame(tg, &batch)
	}
	allocs := testing.AllocsPerRun(50, func() {
		encode()
		f.frame(tg, &batch)
	})
	if allocs != 0 {
		t.Errorf("a frame with images allocates %v times", allocs)
	}
	img := f.read(tg, &batch)
	if got := img.NRGBAAt(24, 24); got.R < 120 || got.R > 136 || got.R != got.G {
		t.Errorf("mipmapped: got %v, want gray", got)
	}
	if got := img.NRGBAAt(72, 24); got.R != 0 && got.R != 255 {
		t.Errorf("nearest: got %v, want black or white", got)
	}
}
