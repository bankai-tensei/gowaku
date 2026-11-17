//go:build gpu

package d3d11

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
)

var (
	white     = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	pureRed   = color.NRGBA{R: 255, A: 255}
	pureGreen = color.NRGBA{G: 255, A: 255}
	pureBlue  = color.NRGBA{B: 255, A: 255}
)

// drawShapes renders what fill records and returns the frame. Positions in the
// result are in logical pixels, because the fixture window is read at its scale.
func drawShapes(t *testing.T, f *fixture, tg *target, fill func(cv *paint.Canvas)) (at func(x, y float32) color.NRGBA, img *image.NRGBA) {
	t.Helper()
	res := render.NewResources(text.NewAtlas(testutil.NewShaper(t)), f.device)
	t.Cleanup(res.Destroy)
	var cv paint.Canvas
	cv.Reset()
	fill(&cv)
	scale := tg.window.Scale()
	var batch gpu.Batch
	render.Encode(cv.DisplayList(), res, scale, &batch)
	if err := res.Finish(&batch); err != nil {
		t.Fatal(err)
	}
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, &batch); err != nil {
		t.Fatal(err)
	}
	frame, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GOWAKU_SHAPES_PNG"); path != "" {
		if out, err := os.Create(path); err == nil {
			_ = png.Encode(out, frame) // a debugging aid
			_ = out.Close()            // a debugging aid
		}
	}
	return func(x, y float32) color.NRGBA { return frame.NRGBAAt(int(x*scale), int(y*scale)) }, frame
}

func expect(t *testing.T, name string, got, want color.NRGBA) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func big(points ...float32) *paint.Path {
	p := &paint.Path{}
	p.MoveTo(points[0], points[1])
	for i := 2; i < len(points); i += 2 {
		p.LineTo(points[i], points[i+1])
	}
	p.Close()
	return p
}

func TestPath_ConcaveFill(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	// An arrowhead that is 100 wide, too large for the atlas.
	arrow := big(10, 10, 110, 60, 10, 110, 40, 60)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.DrawPath(arrow, paint.Paint{Color: geom.Hex(0xff0000)})
	})
	expect(t, "inside", at(60, 60), pureRed)
	expect(t, "in the notch", at(20, 60), white)
	expect(t, "outside", at(150, 60), white)
}

func TestPath_FillRules(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	square := func(p *paint.Path, x, y, s float32) {
		p.MoveTo(x, y)
		p.LineTo(x+s, y)
		p.LineTo(x+s, y+s)
		p.LineTo(x, y+s)
		p.Close()
	}
	nonZero := &paint.Path{}
	square(nonZero, 10, 10, 100)
	square(nonZero, 40, 40, 40)
	evenOdd := &paint.Path{Rule: paint.FillEvenOdd}
	square(evenOdd, 120, 10, 100)
	square(evenOdd, 150, 40, 40)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.DrawPath(nonZero, paint.Paint{Color: geom.Hex(0x00ff00)})
		cv.DrawPath(evenOdd, paint.Paint{Color: geom.Hex(0x0000ff)})
	})
	expect(t, "non-zero keeps the middle", at(60, 60), pureGreen)
	expect(t, "even-odd opens the middle", at(170, 60), white)
	expect(t, "even-odd ring", at(130, 20), pureBlue)
}

func TestPath_Stroke(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	line := &paint.Path{}
	line.MoveTo(20, 50)
	line.LineTo(120, 50)
	line.LineTo(120, 120)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.DrawPath(line, paint.Paint{Stroke: &paint.Stroke{Width: 10, Color: geom.Hex(0xff0000)}})
	})
	expect(t, "on the line", at(60, 50), pureRed)
	expect(t, "round join", at(122, 48), pureRed)
	expect(t, "off the line", at(60, 60), white)
	expect(t, "open paths stay open", at(60, 90), white)
}

func TestClip_RRectAndNesting(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	full := geom.RRect{Rect: geom.Rect{W: 320, H: 240}}
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.Save()
		cv.ClipRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 100, H: 100}, 30))
		cv.DrawRRect(full, paint.Paint{Color: geom.Hex(0xff0000)})
		cv.Save()
		cv.ClipRRect(geom.RRectAll(geom.Rect{X: 60, Y: 60, W: 100, H: 100}, 10))
		cv.DrawRRect(full, paint.Paint{Color: geom.Hex(0x00ff00)})
		cv.Restore()
		// Back at the first clip, which must not have lost anything.
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 30, Y: 60, W: 5, H: 5}}, paint.Paint{Color: geom.Hex(0x0000ff)})
		cv.Restore()
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 200, Y: 200, W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0x0000ff)})
	})
	expect(t, "inside the first clip", at(40, 40), pureRed)
	expect(t, "rounded corner cut off", at(22, 22), white)
	expect(t, "inside both clips", at(100, 100), pureGreen)
	expect(t, "second clip beyond the first", at(130, 130), white)
	expect(t, "after popping the inner clip", at(32, 62), pureBlue)
	expect(t, "after popping everything", at(205, 205), pureBlue)
}

func TestClip_PathThenFillInside(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	clip := big(10, 10, 110, 10, 60, 110)
	square := big(0, 40, 200, 40, 200, 60, 0, 60)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.Save()
		cv.ClipPath(clip)
		cv.DrawPath(square, paint.Paint{Color: geom.Hex(0xff0000)})
		cv.Restore()
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 150, Y: 100, W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0x00ff00)})
	})
	expect(t, "fill inside the clip", at(60, 50), pureRed)
	expect(t, "fill outside the clip", at(150, 50), white)
	expect(t, "stencil left clean", at(155, 105), pureGreen)
}

func TestImage_Draws(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	m := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	m.SetNRGBA(0, 0, pureRed)
	m.SetNRGBA(1, 0, pureBlue)
	picture := paint.NewImage(m)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.DrawImage(picture, geom.Rect{X: 10, Y: 10, W: 100, H: 50}, 1)
		cv.DrawImage(picture, geom.Rect{X: 10, Y: 100, W: 100, H: 50}, 0.5)
	})
	expect(t, "left", at(15, 30), pureRed)
	expect(t, "right", at(105, 30), pureBlue)
	got := at(15, 120)
	if got.R != 255 || got.G < 120 || got.G > 135 {
		t.Errorf("opacity: got %v", got)
	}
}

func TestPath_SmallGoesThroughTheAtlas(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	dot := big(0, 0, 20, 0, 20, 20, 0, 20)
	at, _ := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.Translate(40, 40)
		cv.DrawPath(dot, paint.Paint{Color: geom.Hex(0x00ff00)})
	})
	expect(t, "inside", at(50, 50), pureGreen)
	expect(t, "outside", at(70, 50), white)
}

func TestMSAA_SmoothsADiagonal(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	wedge := big(0, 0, 200, 90, 0, 90)
	_, frame := drawShapes(t, f, tg, func(cv *paint.Canvas) {
		cv.DrawPath(wedge, paint.Paint{Color: geom.Hex(0)})
	})
	soft := 0
	b := frame.Bounds()
	for y := range b.Dy() {
		for x := range b.Dx() {
			if v := frame.NRGBAAt(x, y).R; v != 0 && v != 255 {
				soft++
			}
		}
	}
	if soft < 50 {
		t.Errorf("got %d antialiased pixels", soft)
	}
}

// checkerImage returns n by n texels of black and white.
func checkerImage(n int) *paint.Image {
	m := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			if (x+y)%2 == 1 {
				m.SetNRGBA(x, y, white)
			} else {
				m.SetNRGBA(x, y, color.NRGBA{A: 255})
			}
		}
	}
	return paint.NewImage(m)
}

// A shrunk checkerboard of single texels turns gray with mipmaps, stays black or
// white when nearest, and drawing it every frame allocates nothing once it is on
// the GPU.
func TestImage_MipmapsDoNotAllocate(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	picture := checkerImage(64)
	res := render.NewResources(text.NewAtlas(testutil.NewShaper(t)), f.device)
	t.Cleanup(res.Destroy)
	scale := tg.window.Scale()
	var cv paint.Canvas
	var batch gpu.Batch
	frame := func() {
		cv.Reset()
		// A quarter at scale 1, which is past level 1, where the checkerboard
		// is uniformly gray, at any scale up to 2.
		cv.DrawImage(picture, geom.Rect{X: 16, Y: 16, W: 16, H: 16}, 1)
		cv.DrawImageQuality(picture, geom.Rect{X: 64, Y: 16, W: 16, H: 16}, 1, paint.FilterNearest)
		cv.Save()
		cv.Transform(geom.Translation(geom.Offset{X: 150, Y: 40}).Mul(geom.Rotation(0.5)))
		cv.DrawImage(picture, geom.Rect{W: 20, H: 20}, 1)
		cv.Restore()
		render.Encode(cv.DisplayList(), res, scale, &batch)
		if err := res.Finish(&batch); err != nil {
			t.Fatal(err)
		}
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, &batch); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		frame()
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	}
	allocs := testing.AllocsPerRun(20, func() {
		frame()
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs per frame, want 0", allocs)
	}
	frame()
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}
	at := func(x, y float32) color.NRGBA { return img.NRGBAAt(int(x*scale), int(y*scale)) }
	if got := at(24, 24); got.R < 120 || got.R > 136 || got.R != got.G {
		t.Errorf("mipmapped: got %v, want gray", got)
	}
	if got := at(72, 24); got != white && got != (color.NRGBA{A: 255}) {
		t.Errorf("nearest: got %v, want black or white", got)
	}
}
