package catalog

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// Scale is the number of physical pixels per logical one in the pictures.
	// Two keeps text crisp on the high density screens most readers have; the
	// pages show the pictures at half their size.
	Scale = 2
	// inset is the room, in logical pixels, between the edge of the canvas and
	// the scene, and margin the room kept around what the scene draws when the
	// picture is cropped.
	inset  = 24
	margin = 16

	// defaultWidth and defaultHeight are the canvas of an entry that sets none,
	// in logical pixels.
	defaultWidth  = 560
	defaultHeight = 320

	frameStep = 16 * time.Millisecond
	maxFrames = 2000
)

// Fonts returns the fonts the pictures are set in: the Go fonts, which every
// machine has through the module cache, so that the pictures do not depend on
// the fonts of the system. They have no Japanese glyphs, so the scenes are
// written in English for both languages. See docs/catalog/README.md.
func Fonts() [][]byte {
	return [][]byte{goregular.TTF, gomedium.TTF, gobold.TTF, gomono.TTF}
}

// Shot is one rendered picture of an entry, in light and in dark, cropped to
// the same rectangle.
type Shot struct {
	Light, Dark *image.NRGBA
}

// picture is one file of a Shot.
type picture struct {
	name string
	img  *image.NRGBA
}

// files returns the pictures of s with their file names.
func (s Shot) files(e *Entry) [2]picture {
	return [2]picture{{e.File() + "-light.png", s.Light}, {e.File() + "-dark.png", s.Dark}}
}

// Render draws the scene of e in light and in dark on the CPU and crops both
// to what either draws, plus a margin.
func Render(e *Entry) (Shot, error) {
	light, bgLight, err := renderOne(e, false)
	if err != nil {
		return Shot{}, err
	}
	dark, bgDark, err := renderOne(e, true)
	if err != nil {
		return Shot{}, err
	}
	r := contentBounds(light, bgLight).Union(contentBounds(dark, bgDark))
	if r.Empty() {
		return Shot{}, fmt.Errorf("catalog: %s draws nothing", e.Name)
	}
	r = r.Inset(-margin * Scale).Intersect(light.Rect)
	return Shot{Light: crop(light, r), Dark: crop(dark, r)}, nil
}

// canvas returns the size of the canvas of e in logical pixels.
func (e *Entry) canvas() geom.Size {
	sz := geom.Size{W: e.Width, H: e.Height}
	if sz.W <= 0 {
		sz.W = defaultWidth
	}
	if sz.H <= 0 {
		sz.H = defaultHeight
	}
	return sz
}

// context hands out the BuildContext below the Overlay, for Act.
type context struct {
	core.StatelessBase
	got   *core.BuildContext
	child core.Widget
}

// Build implements core.StatelessWidget.
func (c context) Build(ctx core.BuildContext) core.Widget {
	*c.got = ctx
	return c.child
}

func renderOne(e *Entry, dark bool) (*image.NRGBA, geom.Color, error) {
	shaper, err := text.NewShaper(Fonts()...)
	if err != nil {
		return nil, geom.Color{}, err
	}
	device := soft.New()
	defer device.Destroy()
	resources := render.NewResources(text.NewAtlas(shaper), device)
	defer resources.Destroy()

	b := theme.BrightnessLight
	if dark {
		b = theme.BrightnessDark
	}
	th := theme.Set{}.For(b)
	var ctx core.BuildContext
	scene := e.Scene
	if e.Show != nil {
		scene = e.Show
	}
	content := basic.Padding{Insets: geom.All(inset), Child: basic.Align{Child: scene()}}
	tree := core.NewTree(core.Inherited[*text.Shaper]{Value: shaper, Child: basic.MediaQuery{
		// Less motion holds spinners, shaders and animated images still.
		Data: basic.MediaQueryData{Brightness: b, Scale: Scale, ReduceMotion: true, Locale: language.English},
		Child: basic.Theme{Data: th, Child: overlay.Overlay{
			Child: context{got: &ctx, child: content},
		}},
	}})
	defer tree.Dispose()
	tree.SetWindowFocused(true)

	sz := e.canvas()
	surface, err := device.CreateSurface(nil, int(sz.W*Scale), int(sz.H*Scale), Scale)
	if err != nil {
		return nil, geom.Color{}, err
	}
	defer surface.Destroy()
	f := frame{tree: tree, size: sz, device: device, resources: resources, surface: surface, background: th.Color.Canvas}
	if _, err := f.run(false); err != nil {
		return nil, geom.Color{}, err
	}
	if e.Act != nil {
		e.Act(tree, ctx)
	}
	now := time.Second
	for range maxFrames {
		tree.Tick(now)
		now += frameStep
		if !tree.NeedsFrame() && !tree.Ticking() {
			img, err := f.run(true)
			return img, th.Color.Canvas, err
		}
		if _, err := f.run(false); err != nil {
			return nil, geom.Color{}, err
		}
	}
	return nil, geom.Color{}, fmt.Errorf("catalog: %s never comes to rest", e.Name)
}

// frame runs the pipeline as gowaku.Run does.
type frame struct {
	tree       *core.Tree
	size       geom.Size
	device     *soft.Device
	resources  *render.Resources
	surface    gpu.Surface
	background geom.Color
	canvas     paint.Canvas
	batch      gpu.Batch
}

func (f *frame) run(read bool) (*image.NRGBA, error) {
	c := geom.Tight(f.size)
	f.tree.Build()
	f.tree.Layout(c)
	for f.tree.NeedsBuild() {
		f.tree.Build()
		f.tree.Layout(c)
	}
	if read {
		f.tree.MarkNeedsPaint()
	}
	f.tree.Repaint(&f.canvas)
	render.Encode(f.canvas.DisplayList(), f.resources, Scale, &f.batch)
	if err := f.resources.Finish(&f.batch); err != nil {
		return nil, err
	}
	f.surface.SetMultisample(f.batch.Multisampled())
	if err := f.device.BeginFrame(f.surface, f.background); err != nil {
		return nil, err
	}
	if err := f.device.Draw(f.surface, &f.batch); err != nil {
		return nil, err
	}
	var img *image.NRGBA
	if read {
		var err error
		if img, err = f.device.ReadPixels(f.surface); err != nil {
			return nil, err
		}
	}
	return img, f.device.EndFrame(f.surface)
}

// contentBounds returns the smallest rectangle that holds every pixel of img
// that is not the background.
func contentBounds(img *image.NRGBA, background geom.Color) image.Rectangle {
	bg := [3]uint8{unorm(background.R), unorm(background.G), unorm(background.B)}
	r := image.Rectangle{}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride:]
		for x := range w {
			p := row[x*4:]
			if p[0] != bg[0] || p[1] != bg[1] || p[2] != bg[2] {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func unorm(v float32) uint8 {
	return uint8(min(max(v, 0), 1)*255 + 0.5)
}

// crop copies r of img into a new picture.
func crop(img *image.NRGBA, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := range r.Dy() {
		src := img.Pix[(r.Min.Y+y)*img.Stride+r.Min.X*4:]
		copy(out.Pix[y*out.Stride:y*out.Stride+r.Dx()*4], src)
	}
	// The pictures are opaque: drop any alpha the CPU left below one, so that
	// the encoder writes RGB without a fourth channel.
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 0xff
	}
	return out
}

// EncodePNG compresses img as tightly as the standard encoder can.
func EncodePNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
