//go:build gpu

package gpu_test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
)

const (
	sceneWidth, sceneHeight = 480, 360

	// channelTolerance is what rounding differences between drivers can reach.
	channelTolerance = 2
	// edgeTolerance holds for antialiased pixels, the only ones where the sample
	// patterns of MSAA can differ.
	edgeTolerance = 12
	// edgeShare is how many pixels may go past channelTolerance.
	edgeShare = 0.002
	// strayPixels is how many pixels may go past edgeTolerance. Which side of a
	// triangle edge owns a sample that lies exactly on it is up to the driver in
	// Vulkan, where Direct3D fixes it, so that a corner can flip one sample.
	strayPixels = 8
)

// driverMiss: on Iris Xe, Vulkan draws a frame with shaped clips differently about
// once in 50, in the same few dozen pixels of a clip corner. D3D11 on the same GPU
// and Vulkan on the RTX 3050 Ti never did in 1500 frames. A frame that differs gets
// a second try, so that the tests fail on what repeats. See
// docs/ADR/0013-vulkan-backend.md.

// reader is what all backends offer beyond gpu.Backend.
type reader interface {
	gpu.Backend
	ReadPixels(s gpu.Surface) (*image.NRGBA, error)
	AdapterName() string
}

type side struct {
	backend   reader
	window    platform.Window
	surface   gpu.Surface
	resources *render.Resources
	// onDemand multisamples only the frames that ask for it, as an application
	// does. Otherwise every frame is multisampled.
	onDemand bool
}

func open(t *testing.T, p platform.Platform, b reader, shaper *text.Shaper) *side {
	t.Helper()
	w, err := p.CreateWindow(platform.WindowOptions{Title: "parity", Width: sceneWidth, Height: sceneHeight})
	if err != nil {
		t.Fatal(err)
	}
	width, height := w.PhysicalSize()
	surface, err := b.CreateSurface(w.NativeHandle(), width, height, w.Scale())
	if err != nil {
		t.Fatal(err)
	}
	s := &side{backend: b, window: w, surface: surface, resources: render.NewResources(text.NewAtlas(shaper), b)}
	t.Cleanup(func() {
		s.resources.Destroy()
		s.surface.Destroy()
		s.window.Close()
	})
	return s
}

func (s *side) render(t *testing.T, scale float32, draw func(cv *paint.Canvas)) *image.NRGBA {
	t.Helper()
	width, height := s.window.PhysicalSize()
	if err := s.surface.Resize(width, height, scale); err != nil {
		t.Fatal(err)
	}
	var cv paint.Canvas
	cv.Reset()
	draw(&cv)
	var batch gpu.Batch
	render.Encode(cv.DisplayList(), s.resources, scale, &batch)
	if err := s.resources.Finish(&batch); err != nil {
		t.Fatal(err)
	}
	s.surface.SetMultisample(!s.onDemand || batch.Multisampled())
	if err := s.backend.BeginFrame(s.surface, geom.Hex(0xf5f5f7)); err != nil {
		t.Fatal(err)
	}
	if err := s.backend.Draw(s.surface, &batch); err != nil {
		t.Fatal(err)
	}
	img, err := s.backend.ReadPixels(s.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.backend.EndFrame(s.surface); err != nil {
		t.Fatal(err)
	}
	return img
}

// result says how two frames differ.
type result struct {
	worst        int
	worstX       int
	worstY       int
	differing    int // pixels that differ at all
	small, large int // pixels past channelTolerance and edgeTolerance
	pixels       int
	diff         *image.NRGBA
}

func (r result) String() string {
	return fmt.Sprintf("worst %d at (%d, %d), %.3f%% of pixels differ, %d past %d, %d past %d",
		r.worst, r.worstX, r.worstY, 100*float64(r.differing)/float64(r.pixels), r.small, channelTolerance, r.large, edgeTolerance)
}

func (r result) failed() bool {
	return float64(r.small) > float64(r.pixels)*edgeShare || r.large > strayPixels
}

func compare(a, b *image.NRGBA) result {
	r := result{diff: image.NewNRGBA(a.Bounds()), pixels: len(a.Pix) / 4}
	for i := 0; i < len(a.Pix); i += 4 {
		pixel := 0
		for c := range 3 {
			pixel = max(pixel, max(int(a.Pix[i+c])-int(b.Pix[i+c]), int(b.Pix[i+c])-int(a.Pix[i+c])))
		}
		if pixel > r.worst {
			r.worst, r.worstX, r.worstY = pixel, i/4%a.Rect.Dx(), i/4/a.Rect.Dx()
		}
		if pixel > 0 {
			r.differing++
		}
		if pixel > channelTolerance {
			r.small++
		}
		if pixel > edgeTolerance {
			r.large++
		}
		v := uint8(min(pixel*16, 255))
		r.diff.Pix[i], r.diff.Pix[i+1], r.diff.Pix[i+2], r.diff.Pix[i+3] = v, v, v, 255
	}
	return r
}

func save(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func polygon(points ...float32) *paint.Path {
	var p paint.Path
	p.MoveTo(points[0], points[1])
	for i := 2; i < len(points); i += 2 {
		p.LineTo(points[i], points[i+1])
	}
	p.Close()
	return &p
}

func testImage() *paint.Image {
	m := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 36), G: uint8(y * 36), B: 128, A: 255 - uint8(x*16)})
		}
	}
	return paint.NewImage(m)
}

// fineImage returns a picture of single texels in alternating colors, some of
// them translucent, which aliases when shrunk unless it is mipmapped.
func fineImage() *paint.Image {
	const n = 96
	m := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			c := color.NRGBA{R: uint8(x * 2), G: 40, B: uint8(255 - y*2), A: 255}
			if (x+y)%2 == 1 {
				c = color.NRGBA{R: 250, G: 250, B: 240, A: uint8(80 + x)}
			}
			if (x/3+y/5)%4 == 0 {
				c = color.NRGBA{R: 20, G: 20, B: 20, A: 255}
			}
			m.SetNRGBA(x, y, c)
		}
	}
	return paint.NewImage(m)
}

func scenes(shaper *text.Shaper) map[string]func(cv *paint.Canvas) {
	picture := testImage()
	fine := fineImage()
	paragraph := shaper.Shape("吾輩は猫である。名前はまだ無い。gowaku renders 日本語 and English.", text.Style{Size: 18}).Layout(420)
	full := geom.RRect{Rect: geom.Rect{W: sceneWidth, H: sceneHeight}}
	star := polygon(120, 20, 170, 170, 40, 75, 200, 75, 70, 170)
	evenOdd := polygon(340, 20, 390, 170, 260, 75, 420, 75, 290, 170)
	evenOdd.Rule = paint.FillEvenOdd
	triangle := polygon(120, 40, 300, 120, 120, 300)
	return map[string]func(cv *paint.Canvas){
		"rrect": func(cv *paint.Canvas) {
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 200, H: 120}, 16), paint.Paint{
				Color: geom.Hex(0x0a84ff), Stroke: &paint.Stroke{Width: 2, Color: geom.Hex(0x1d1d1f)},
			})
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 240, Y: 20, W: 200, H: 120}, 60), paint.Paint{Color: geom.Color{R: 1, A: 0.5}})
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 20.5, Y: 160.5, W: 100, H: 50}}, paint.Paint{Color: geom.Hex(0x30d158)})
		},
		"gradient": func(cv *paint.Canvas) {
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 400, H: 100}, 12), paint.Paint{Gradient: &paint.Gradient{
				Kind: paint.GradientLinear, Start: geom.Offset{X: 20, Y: 20}, End: geom.Offset{X: 420, Y: 120},
				StartColor: geom.Hex(0xff0000), EndColor: geom.Hex(0x0000ff),
			}})
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 140, W: 200, H: 200}, 100), paint.Paint{Gradient: &paint.Gradient{
				Kind: paint.GradientRadial, Start: geom.Offset{X: 120, Y: 240}, End: geom.Offset{X: 220, Y: 240},
				StartColor: geom.Hex(0xffffff), EndColor: geom.Hex(0x000000),
			}})
		},
		"spread": func(cv *paint.Canvas) {
			stops := []paint.GradientStop{
				{Offset: 0, Color: geom.Hex(0xff453a)}, {Offset: 0.5, Color: geom.Hex(0xffd60a)}, {Offset: 1, Color: geom.Hex(0x0a84ff)},
			}
			for i, spread := range []paint.SpreadMode{paint.SpreadPad, paint.SpreadReflect, paint.SpreadRepeat} {
				y := float32(20 + i*70)
				cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 20, Y: y, W: 400, H: 60}}, paint.Paint{Gradient: &paint.Gradient{
					Kind: paint.GradientLinear, Spread: spread, Start: geom.Offset{X: 150, Y: y}, End: geom.Offset{X: 230, Y: y + 20},
					MoreStops: &stops,
				}})
			}
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 230, W: 120, H: 120}, 60), paint.Paint{Gradient: &paint.Gradient{
				Kind: paint.GradientFocal, Start: geom.Offset{X: 80, Y: 290}, End: geom.Offset{X: 140, Y: 290},
				Focus: geom.Offset{X: 60, Y: 270}, MoreStops: &stops,
			}})
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 160, Y: 230, W: 260, H: 120}}, paint.Paint{Gradient: &paint.Gradient{
				Kind: paint.GradientFocal, Spread: paint.SpreadReflect, Start: geom.Offset{X: 290, Y: 290}, End: geom.Offset{X: 320, Y: 290},
				Focus: geom.Offset{X: 300, Y: 280}, MoreStops: &stops, Transform: geom.Affine{A: 1, D: 0.6, C: 0.4, E: -116, F: 116},
			}})
		},
		"shadow": func(cv *paint.Canvas) {
			box := geom.RRectAll(geom.Rect{X: 60, Y: 60, W: 200, H: 120}, 16)
			cv.DrawShadow(box, 24, geom.Color{A: 0.4}, geom.Offset{Y: 6})
			cv.DrawRRect(box, paint.Paint{Color: geom.Hex(0xffffff)})
		},
		"clips": func(cv *paint.Canvas) {
			cv.Save()
			cv.ClipRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 200, H: 200}, 40))
			cv.DrawRRect(full, paint.Paint{Color: geom.Hex(0xff453a)})
			cv.Save()
			cv.ClipPath(triangle)
			cv.DrawRRect(full, paint.Paint{Color: geom.Hex(0x30d158)})
			cv.Restore()
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 40, Y: 100, W: 30, H: 30}}, paint.Paint{Color: geom.Hex(0x0a84ff)})
			cv.Restore()
			cv.Save()
			cv.ClipRect(geom.Rect{X: 240, Y: 240, W: 100, H: 60})
			cv.DrawRRect(full, paint.Paint{Color: geom.Hex(0xffd60a)})
			cv.Restore()
		},
		"paths": func(cv *paint.Canvas) {
			cv.DrawPath(star, paint.Paint{Color: geom.Hex(0x0a84ff)})
			cv.DrawPath(evenOdd, paint.Paint{Color: geom.Hex(0xff453a)})
		},
		"image": func(cv *paint.Canvas) {
			cv.DrawImage(picture, geom.Rect{X: 20, Y: 20, W: 200, H: 200}, 1)
			cv.DrawImage(picture, geom.Rect{X: 240, Y: 20, W: 100, H: 100}, 0.5)
		},
		"mipmaps": func(cv *paint.Canvas) {
			qualities := [...]paint.FilterQuality{paint.FilterNearest, paint.FilterLinear, paint.FilterMipmap}
			for i, q := range qualities {
				y := 10 + float32(i)*100
				cv.DrawImageQuality(fine, geom.Rect{X: 10, Y: y, W: 48, H: 48}, 1, q)
				cv.DrawImageQuality(fine, geom.Rect{X: 70, Y: y, W: 32, H: 32}, 1, q)
				// Edges on whole pixels at both scales, which a multisampled
				// frame would otherwise smooth.
				cv.DrawImageQuality(fine, geom.Rect{X: 116, Y: y, W: 20, H: 20}, 0.8, q)
				cv.DrawImageQuality(fine, geom.Rect{X: 150, Y: y, W: 90, H: 30}, 1, q)
				cv.Save()
				about := geom.Offset{X: 300, Y: y + 40}
				cv.Transform(geom.Translation(about).Mul(geom.Rotation(0.4)).Mul(geom.Translation(geom.Offset{X: -about.X, Y: -about.Y})))
				cv.DrawImageQuality(fine, geom.Rect{X: 270, Y: y + 10, W: 60, H: 60}, 1, q)
				cv.Restore()
			}
		},
		"text": func(cv *paint.Canvas) {
			runs := paragraph.Runs()
			for i := range runs {
				cv.DrawGlyphs(&runs[i], geom.Offset{X: 20, Y: 30}, geom.Hex(0x1d1d1f))
			}
		},
		"backdrop": func(cv *paint.Canvas) {
			stripes(cv)
			runs := paragraph.Runs()
			for i := range runs {
				cv.DrawGlyphs(&runs[i], geom.Offset{X: 20, Y: 30}, geom.Hex(0x1d1d1f))
			}
			for i, sigma := range []float32{2, 8, 30} {
				card := geom.RRectAll(geom.Rect{X: 20 + float32(i)*150, Y: 60, W: 130, H: 200}, 18)
				cv.DrawBackdrop(card, sigma)
				cv.DrawRRect(card, paint.Paint{Color: geom.Color{R: 1, G: 1, B: 1, A: 0.4}})
			}
			// At the edge of the surface, the blur repeats the edge.
			cv.DrawBackdrop(geom.RRect{Rect: geom.Rect{X: -10, Y: 300, W: 200, H: 80}}, 6)
		},
		"backdrop-clipped": func(cv *paint.Canvas) {
			stripes(cv)
			cv.DrawPath(star, paint.Paint{Color: geom.Hex(0x1d1d1f)})
			cv.Save()
			cv.ClipRRect(geom.RRectAll(geom.Rect{X: 60, Y: 40, W: 300, H: 260}, 60))
			cv.PushOpacity(0.8)
			cv.DrawBackdrop(geom.RRect{Rect: geom.Rect{X: 40, Y: 20, W: 400, H: 200}}, 10)
			cv.Restore()
		},
		"shaders": func(cv *paint.Canvas) {
			stripes(cv)
			params := paint.ShaderParams{0.13, 0.30, 0.72, 1, 0.62, 0.22, 0.66, 1, 0.2, 2.4, 0.35, 0}
			cv.DrawShader(geom.Rect{X: 10, Y: 10, W: 220, H: 120}, paint.ShaderCall{Shader: shaders.Aurora, Params: params, Time: 1500 * time.Millisecond})
			// Turned, faded and blended, under a rounded clip.
			cv.Save()
			cv.ClipRRect(geom.RRectAll(geom.Rect{X: 250, Y: 10, W: 220, H: 150}, 40))
			about := geom.Offset{X: 360, Y: 85}
			cv.Transform(geom.Translation(about).Mul(geom.Rotation(0.35)).Mul(geom.Translation(geom.Offset{X: -about.X, Y: -about.Y})))
			cv.PushOpacity(0.8)
			cv.PushBlend(paint.BlendMultiply)
			cv.DrawShader(geom.Rect{X: 270, Y: 30, W: 180, H: 110}, paint.ShaderCall{Shader: shaders.Aurora, Params: params, Time: 400 * time.Millisecond})
			cv.Restore()
			// Two textures, stretched.
			tint := paint.ShaderParams{0.35, 0, 0, 0, 1, 0.8, 0.2, 0.3}
			cv.DrawShader(geom.Rect{X: 10, Y: 150, W: 150, H: 90}, paint.ShaderCall{
				Shader: shaders.Crossfade, Params: tint, Tex0: paint.ShaderTexture{Image: picture}, Tex1: paint.ShaderTexture{Image: fine},
			})
			// A layer of what is under the rectangle, bent.
			if cv.CanShadeLayer() {
				var l paint.Layer
				lc := l.Begin()
				for i := range paragraph.Runs() {
					lc.DrawGlyphs(&paragraph.Runs()[i], geom.Offset{X: 0, Y: 20}, geom.Hex(0x1d1d1f))
				}
				lc.DrawImage(picture, geom.Rect{X: 20, Y: 40, W: 80, H: 60}, 1)
				l.End()
				ripple := paint.ShaderParams{0.5, 0.5, 4, 0.3, 3}
				cv.DrawShader(geom.Rect{X: 180, Y: 170, W: 290, H: 180}, paint.ShaderCall{
					Shader: shaders.Ripple, Params: ripple, Time: 700 * time.Millisecond, Tex0: paint.ShaderTexture{Layer: &l, Origin: geom.Offset{X: 180, Y: 170}},
				})
			}
		},
		"filtered": func(cv *paint.Canvas) {
			stripes(cv)
			shapes := func(c *paint.Canvas, at geom.Offset) {
				c.DrawRRect(geom.RRectAll(geom.Rect{X: at.X + 20, Y: at.Y + 20, W: 160, H: 100}, 20), paint.Paint{Color: geom.Hex(0x0a84ff)})
				c.DrawRRect(geom.RRect{Rect: geom.Rect{X: at.X + 100, Y: at.Y + 60, W: 120, H: 120}}, paint.Paint{Color: geom.Color{R: 1, A: 0.6}})
			}
			// The shapes blurred as one picture: a group with a filter, which
			// draws the same inside the layers that the layer tests replay the
			// scenes in.
			blurred := func(at geom.Offset, sigma float32, mode paint.BlendMode) {
				m := 3*sigma + 1
				f := &paint.Filter{
					Region:     geom.Rect{X: at.X + 20 - m, Y: at.Y + 20 - m, W: 200 + 2*m, H: 160 + 2*m},
					Primitives: []paint.FilterPrimitive{{Kind: paint.FilterBlur, SRGB: true, StdDev: geom.Offset{X: sigma, Y: sigma}}},
				}
				cv.PushGroup(paint.GroupStyle{Blend: mode, Filter: f})
				shapes(cv, at)
				cv.PopGroup()
			}
			blurred(geom.Offset{X: 10, Y: 10}, 5, paint.BlendSrcOver)
			cv.Save()
			about := geom.Offset{X: 340, Y: 240}
			cv.Transform(geom.Translation(about).Mul(geom.Rotation(0.3)).Mul(geom.Translation(geom.Offset{X: -about.X, Y: -about.Y})))
			cv.PushOpacity(0.7)
			blurred(geom.Offset{X: 220, Y: 150}, 12, paint.BlendMultiply)
			cv.Restore()
		},
	}
}

// stripes fills the scene with colored bars, a busy backdrop for blurs.
func stripes(cv *paint.Canvas) {
	colors := []uint32{0xff453a, 0xffd60a, 0x30d158, 0x0a84ff, 0xbf5af2, 0x1d1d1f}
	for i := range 24 {
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: float32(i) * 20, W: 10, H: sceneHeight}},
			paint.Paint{Color: geom.Hex(colors[i%len(colors)])})
	}
}

// Whether a frame is multisampled must not show. Going through the scenes twice
// also switches a surface between its targets, in both directions.
func TestParity_OnDemandMultisamplingLooksTheSame(t *testing.T) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Fatal(err)
	}
	p, err := win32.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	direct, err := d3d11.New(d3d11.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(direct.Destroy)
	vk, err := vulkan.New(vulkan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Destroy)
	reference := soft.New()
	t.Cleanup(reference.Destroy)
	for _, s := range []*side{open(t, p, direct, shaper), open(t, p, vk, shaper), open(t, p, reference, shaper)} {
		for name, scene := range scenes(shaper) {
			for _, scale := range []float32{1, 1.5} {
				s.onDemand = false
				always := s.render(t, scale, scene)
				s.onDemand = true
				sometimes := s.render(t, scale, scene)
				r := compare(always, sometimes)
				if r.failed() {
					// See driverMiss. Either frame can be the one that missed.
					s.onDemand = false
					always = s.render(t, scale, scene)
					s.onDemand = true
					r = compare(always, s.render(t, scale, scene))
				}
				t.Logf("%s %s@%v: %s", s.backend.AdapterName(), name, scale, r)
				if r.failed() {
					t.Errorf("%s@%v: multisampling shows: %s", name, scale, r)
				}
			}
		}
		// A resize drops the multisampled targets, and the next frame that needs
		// them must make them again.
		width, height := s.window.PhysicalSize()
		for _, resize := range []int{width - 40, width} {
			if err := s.surface.Resize(resize, height, 1); err != nil {
				t.Fatal(err)
			}
			s.onDemand = false
			always := s.render(t, 1, scenes(shaper)["paths"])
			s.onDemand = true
			if r := compare(always, s.render(t, 1, scenes(shaper)["paths"])); r.failed() {
				t.Errorf("after a resize: %s", r)
			}
		}
	}
}

func TestParity_D3D11AndVulkanDrawTheSame(t *testing.T) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("GOWAKU_PARITY_DIR")
	if out == "" {
		out = t.TempDir()
	}
	for _, power := range []gpu.PowerPreference{gpu.LowPower, gpu.HighPerformance} {
		// Windows belong to the thread of the Platform, which is that of this
		// subtest. The scenes must not become subtests of their own.
		t.Run(power.String(), func(t *testing.T) {
			p, err := win32.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Destroy)
			direct, err := d3d11.New(d3d11.Options{Power: power})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(direct.Destroy)
			vk, err := vulkan.New(vulkan.Options{Power: power})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(vk.Destroy)
			reference := soft.New()
			t.Cleanup(reference.Destroy)
			t.Logf("d3d11: %s, vulkan: %s", direct.AdapterName(), vk.AdapterName())

			a, b, c := open(t, p, direct, shaper), open(t, p, vk, shaper), open(t, p, reference, shaper)
			for name, scene := range scenes(shaper) {
				for _, scale := range []float32{1, 1.5} {
					left, right := a.render(t, scale, scene), b.render(t, scale, scene)
					r := compare(left, right)
					if r.failed() {
						// See driverMiss.
						right = b.render(t, scale, scene)
						r = compare(left, right)
					}
					t.Logf("%s@%v: %s", name, scale, r)
					if r.failed() {
						stem := fmt.Sprintf("%s-%s-%v", power, name, scale)
						save(t, out, stem+"-d3d11.png", left)
						save(t, out, stem+"-vulkan.png", right)
						save(t, out, stem+"-diff.png", r.diff)
						t.Errorf("%s@%v: backends differ: %s; see %s", name, scale, r, out)
					}

					cpu := c.render(t, scale, scene)
					r = compare(left, cpu)
					t.Logf("%s@%v soft: %s", name, scale, r)
					if r.failed() {
						stem := fmt.Sprintf("%s-%s-%v", power, name, scale)
						save(t, out, stem+"-d3d11.png", left)
						save(t, out, stem+"-soft.png", cpu)
						save(t, out, stem+"-soft-diff.png", r.diff)
						t.Errorf("%s@%v: soft differs from d3d11: %s; see %s", name, scale, r, out)
					}
				}
			}
		})
	}
}

// The scenes again, this time through a layer, which is the path of CreateTarget
// and DrawToTarget.
func TestParity_SoftLayersMatchD3D11(t *testing.T) {
	shaper, err := text.NewSystemShaper()
	if err != nil {
		t.Fatal(err)
	}
	for _, power := range []gpu.PowerPreference{gpu.LowPower, gpu.HighPerformance} {
		// See the note on threads in TestParity_D3D11AndVulkanDrawTheSame.
		t.Run(power.String(), func(t *testing.T) {
			p, err := win32.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Destroy)
			direct, err := d3d11.New(d3d11.Options{Power: power})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(direct.Destroy)
			reference := soft.New()
			t.Cleanup(reference.Destroy)
			t.Logf("d3d11: %s", direct.AdapterName())

			a, b := open(t, p, direct, shaper), open(t, p, reference, shaper)
			a.onDemand, b.onDemand = true, true
			throughLayer := func(s *side, scale float32, scene func(cv *paint.Canvas)) *image.NRGBA {
				var l paint.Layer
				scene(l.Begin())
				l.End()
				return s.render(t, scale, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) })
			}
			for name, scene := range scenes(shaper) {
				for _, scale := range []float32{1, 1.5} {
					r := compare(throughLayer(a, scale, scene), throughLayer(b, scale, scene))
					t.Logf("%s@%v: %s", name, scale, r)
					if r.failed() {
						t.Errorf("%s@%v: soft differs from d3d11: %s", name, scale, r)
					}
				}
			}
		})
	}
}
