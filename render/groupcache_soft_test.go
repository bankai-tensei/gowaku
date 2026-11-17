package render

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// softFrames draws what draw records frames times on the CPU backend with the
// same Resources, on white, and returns each frame.
func softFrames(t *testing.T, width, height, frames int, draw func(cv *paint.Canvas, frame int)) []*image.NRGBA {
	t.Helper()
	device := soft.New()
	t.Cleanup(device.Destroy)
	res := NewResources(text.NewAtlas(testutil.NewShaper(t)), device)
	t.Cleanup(res.Destroy)
	surface, err := device.CreateSurface(nil, width, height, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Destroy)
	var cv paint.Canvas
	var b gpu.Batch
	var out []*image.NRGBA
	for i := range frames {
		cv.Reset()
		draw(&cv, i)
		Encode(cv.DisplayList(), res, 1, &b)
		if err := res.Finish(&b); err != nil {
			t.Fatal(err)
		}
		if err := device.BeginFrame(surface, geom.Color{R: 1, G: 1, B: 1, A: 1}); err != nil {
			t.Fatal(err)
		}
		if err := device.Draw(surface, &b); err != nil {
			t.Fatal(err)
		}
		img, err := device.ReadPixels(surface)
		if err != nil {
			t.Fatal(err)
		}
		if err := device.EndFrame(surface); err != nil {
			t.Fatal(err)
		}
		out = append(out, img)
	}
	return out
}

// maskedGroup draws a blurred, faded and masked group cached by cache at x,
// under a clip, over a bar.
func maskedGroup(cv *paint.Canvas, cache *paint.GroupCache, x float32) {
	blur := &paint.Filter{
		Region:     geom.Rect{X: -20, Y: -20, W: 120, H: 120},
		Primitives: []paint.FilterPrimitive{{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 3, Y: 3}}},
	}
	cv.DrawRRect(rect(0, 50, 160, 20), paint.Paint{Color: geom.Color{G: 0.6, A: 1}})
	cv.Save()
	cv.ClipRect(geom.Rect{W: 120, H: 100})
	cv.Translate(x, 10)
	cv.PushOpacity(0.8)
	cv.PushGroup(paint.GroupStyle{Cache: cache, Filter: blur, Mask: paint.MaskAlpha})
	cv.DrawRRect(rect(0, 0, 50, 50), paint.Paint{Color: geom.Color{R: 1, A: 1}})
	cv.DrawRRect(rect(25, 25, 50, 50), paint.Paint{Color: geom.Color{B: 1, A: 1}})
	cv.BeginMask()
	cv.DrawRRect(geom.RRectAll(geom.Rect{W: 70, H: 70}, 35), paint.Paint{Color: geom.Color{A: 1}})
	cv.PopGroup()
	cv.Restore()
}

// The kept picture looks as the group drawn anew: blurred, faded, masked and
// clipped, and moved by whole pixels.
func TestCachedGroup_KeptPictureLooksTheSame(t *testing.T) {
	var cache paint.GroupCache
	frames := softFrames(t, 160, 120, 4, func(cv *paint.Canvas, frame int) {
		x := float32(20)
		if frame == 3 {
			x = 40
		}
		maskedGroup(cv, &cache, x)
	})
	for i := 1; i < 3; i++ {
		if !samePixels(frames[0], frames[i]) {
			t.Errorf("frame %d differs from the first", i)
		}
	}
	var fresh paint.GroupCache
	moved := softFrames(t, 160, 120, 1, func(cv *paint.Canvas, _ int) { maskedGroup(cv, &fresh, 40) })
	if !samePixels(moved[0], frames[3]) {
		t.Error("the moved picture differs from the group drawn there")
	}
	var uncached [1]*image.NRGBA
	uncached[0] = softFrames(t, 160, 120, 1, func(cv *paint.Canvas, _ int) { maskedGroup(cv, nil, 20) })[0]
	if !samePixels(uncached[0], frames[0]) {
		t.Error("the cached group differs from the group without a cache")
	}
}

func samePixels(a, b *image.NRGBA) bool {
	if a.Rect != b.Rect {
		return false
	}
	for i := range a.Pix {
		if d := int(a.Pix[i]) - int(b.Pix[i]); d > 1 || d < -1 {
			return false
		}
	}
	return true
}
