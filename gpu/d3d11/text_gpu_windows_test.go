//go:build gpu

package d3d11

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
)

func TestText_GlyphsReachThePixels(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Skip(err)
	}
	glyphs := render.NewResources(text.NewAtlas(shaper), f.device)
	defer glyphs.Destroy()

	layout := shaper.Shape("■", text.Style{Size: 40}).Layout(1000)
	var cv paint.Canvas
	cv.Reset()
	runs := layout.Runs()
	for i := range runs {
		cv.DrawGlyphs(&runs[i], geom.Offset{X: 10, Y: 10}, geom.Hex(0xff0000))
	}
	var batch gpu.Batch
	render.Encode(cv.DisplayList(), glyphs, tg.window.Scale(), &batch)
	if err := glyphs.Finish(&batch); err != nil {
		t.Fatal(err)
	}
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, &batch); err != nil {
		t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}

	in := batch.Instances[0].Rect
	scale := tg.window.Scale()
	cx, cy := int((in.X+in.W/2)*scale), int((in.Y+in.H/2)*scale)
	if got, want := img.NRGBAAt(cx, cy), (color.NRGBA{R: 255, A: 255}); got != want {
		t.Errorf("inside the glyph: got %v, want %v", got, want)
	}
	if got, want := img.NRGBAAt(2, 2), (color.NRGBA{R: 255, G: 255, B: 255, A: 255}); got != want {
		t.Errorf("outside the glyph: got %v, want %v", got, want)
	}
}

// TestText_1000FrameTime measures a scene of 1000 different short strings from
// strings to present. The first frame shapes the text and rasterizes every glyph; the
// later ones are what a repaint costs.
func TestText_1000FrameTime(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Skip(err)
	}
	// The first use of a font loads it from disk, which happens once per process.
	shaper.Shape("字a", text.Style{Size: 14}).Layout(100)

	glyphs := render.NewResources(text.NewAtlas(shaper), f.device)
	defer glyphs.Destroy()
	var layouts []*text.Layout
	var cv paint.Canvas
	var batch gpu.Batch
	frame := func() {
		cv.Reset()
		for i, l := range layouts {
			runs := l.Runs()
			off := geom.Offset{X: float32(i%4) * 200, Y: float32(i/4) * 2}
			for k := range runs {
				cv.DrawGlyphs(&runs[k], off, geom.Hex(0))
			}
		}
		render.Encode(cv.DisplayList(), glyphs, tg.window.Scale(), &batch)
		if err := glyphs.Finish(&batch); err != nil {
			t.Fatal(err)
		}
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, &batch); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	for i := range 1000 {
		content := fmt.Sprintf("項目 %d の説明文 Item", i)
		layouts = append(layouts, shaper.Shape(content, text.Style{Size: 14}).Layout(190))
	}
	shaped := time.Since(start)
	frame()
	cold := time.Since(start)
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}

	const frames = 30
	start = time.Now()
	var busy time.Duration
	for range frames {
		begin := time.Now()
		frame()
		busy += time.Since(begin)
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("shape+layout %v, first frame in total %v (%d glyphs), repaint %v of CPU per frame",
		shaped, cold, len(batch.Instances), busy/frames)
}
