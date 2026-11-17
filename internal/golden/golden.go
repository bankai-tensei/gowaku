// Package golden draws fixed scenes on any gpu.Backend and compares the result
// with stored PNGs. The CPU backend does so everywhere, CI included, and the GPU
// backends check against the same files. See docs/ADR/0020-pixel-goldens.md.
package golden

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// Tolerance is the difference of a channel that rounding can reach between
	// backends.
	Tolerance = 2
	// edgeTolerance holds for antialiased pixels, where the backends can differ
	// in which side of an edge owns a sample.
	edgeTolerance = 12
	// edgeShare is how many pixels may go past Tolerance.
	edgeShare = 0.002
	// strayPixels is how many pixels may go past edgeTolerance.
	strayPixels = 8

	// frameStep is the time between the frames of a scene.
	frameStep = 16 * time.Millisecond
	// maxFrames bounds how long a scene may take to come to rest.
	maxFrames = 2000
)

// Backend is what a scene needs of a backend.
type Backend interface {
	gpu.Backend
	ReadPixels(gpu.Surface) (*image.NRGBA, error)
}

// Scene is a picture to compare.
type Scene struct {
	Name string
	// Width and Height are in physical pixels.
	Width, Height int
	Scale         float32
	Dark          bool
	Root          func() core.Widget
	// Act runs after the first frame, with the context of the root.
	Act func(tree *core.Tree, ctx core.BuildContext)
	// Frames, if above zero, takes the picture that many frames after Act,
	// in the middle of a motion, instead of once the scene comes to rest.
	Frames int
}

// context hands out the BuildContext below the Overlay.
type context struct {
	got   *core.BuildContext
	child core.Widget
}

// Key implements core.Widget.
func (context) Key() core.Key { return nil }

// Build implements core.StatelessWidget.
func (c context) Build(ctx core.BuildContext) core.Widget {
	*c.got = ctx
	return c.child
}

func controls() core.Widget {
	nothing := func(bool) {}
	return basic.Padding{Insets: geom.All(16), Child: basic.Column{Spacing: 12, Children: []core.Widget{
		basic.Row{Spacing: 8, Children: []core.Widget{
			control.Button{Label: "Save", Style: control.ButtonPrimary, OnPress: func() {}},
			control.Button{Label: "Draft", OnPress: func() {}},
			control.Button{Label: "Delete", Style: control.ButtonDanger, OnPress: func() {}},
			control.Button{Label: "Off"},
		}},
		basic.Row{Spacing: 16, Children: []core.Widget{
			control.Switch{Value: true, Label: "on", OnChange: nothing},
			control.Switch{Label: "off", OnChange: nothing},
			control.Checkbox{Value: true, Label: "Open at start", OnChange: nothing},
		}},
		basic.SizedBox{Width: 240, Child: control.Slider{Value: 0.35, Label: "volume", OnChange: func(float32) {}}},
		basic.SizedBox{Width: 300, Child: field.Input{Label: "Name", Value: "gowaku", OnChange: func(string) {}}},
		basic.Container{
			Width: 300, Height: 90, Radius: 12, Color: geom.Hex(0x2f5fd0),
			Shadows: []paint.BoxShadow{{Color: geom.Hex(0x000000).WithAlpha(0.3), Blur: 16, Offset: geom.Offset{Y: 4}}},
		},
	}}}
}

func listRow(i int) core.Widget {
	return basic.Padding{Insets: geom.Symmetric(12, 4), Child: basic.Row{Spacing: 8, Children: []core.Widget{
		basic.Expanded{Child: basic.Text{Content: fmt.Sprintf("Row number %d", i)}},
		control.Button{Label: "Open", OnPress: func() {}},
	}}}
}

func list() core.Widget {
	return collection.List{ItemCount: 10000, ItemExtent: 44, ItemBuilder: listRow}
}

func paragraph() core.Widget {
	const s = "A quick brown fox jumps over the lazy dog, again and again, until the line has to break. "
	return basic.Padding{Insets: geom.All(16), Child: basic.Text{Content: s + s + s}}
}

// vertical sets Latin text in columns, which turns every glyph sideways, next
// to the same text justified.
func vertical() core.Widget {
	const s = "Columns run from right to left, and every glyph of Latin text turns sideways. "
	return basic.Padding{Insets: geom.All(16), Child: basic.Row{Spacing: 24, Children: []core.Widget{
		basic.SizedBox{Width: 200, Child: basic.Text{Content: s + s, Style: text.Style{Align: text.AlignJustify, Indent: 16}}},
		basic.Text{Content: s + s, Style: text.Style{Orientation: text.Vertical}},
	}}}
}

// Scenes returns every scene.
func Scenes() []Scene {
	extra := append(append(controlScenes(), desktopScenes()...), appScenes()...)
	scenes := make([]Scene, 0, 32+len(extra))
	scenes = append(scenes, []Scene{
		{Name: "controls", Width: 640, Height: 420, Scale: 1, Root: controls},
		{Name: "controls-dark", Width: 960, Height: 630, Scale: 1.5, Dark: true, Root: controls},
		{Name: "paragraph", Width: 640, Height: 320, Scale: 2, Root: paragraph},
		{Name: "list", Width: 600, Height: 450, Scale: 1.5, Root: list},
		{Name: "vertical", Width: 600, Height: 360, Scale: 1.5, Root: vertical},
		{Name: "gradients", Width: 648, Height: 228, Scale: 1.5, Root: gradients},
		{Name: "gradients2", Width: 900, Height: 450, Scale: 1.5, Root: gradients2},
		{Name: "transforms", Width: 960, Height: 330, Scale: 1.5, Root: transforms},
		{
			Name: "list-scrolled", Width: 600, Height: 450, Scale: 1.5, Root: list,
			Act: func(tree *core.Tree, _ core.BuildContext) {
				tree.DispatchPointer(core.PointerEvent{
					Kind: core.PointerScroll, Pos: geom.Offset{X: 20, Y: 20}, Scroll: geom.Offset{Y: -5},
				})
			},
		},
		{
			Name: "dialog", Width: 640, Height: 420, Scale: 1, Root: controls,
			Act: func(_ *core.Tree, ctx core.BuildContext) {
				overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
					return overlay.Dialog{
						Title:   "Delete this?",
						Content: basic.Text{Content: "It cannot be undone."},
						Actions: []core.Widget{control.Button{Label: "Keep", OnPress: dismiss}},
					}
				})
			},
		},
		// New scenes go last: the scenes share one Renderer, whose atlas and
		// pages they leave behind for the next.
		{Name: "blends", Width: 996, Height: 612, Scale: 1.5, Root: blends},
		{Name: "blends2", Width: 1146, Height: 201, Scale: 1.5, Root: blends2},
		{Name: "blurs", Width: 960, Height: 540, Scale: 1.5, Root: blurs},
		{Name: "glass-dialog", Width: 720, Height: 480, Scale: 1, Root: glassPage, Act: showGlassDialog},
		{Name: "glass-dialog-dark", Width: 900, Height: 600, Scale: 1.5, Dark: true, Root: glassPage, Act: showGlassDialog},
		{Name: "svg-icons", Width: 675, Height: 495, Scale: 1.5, Root: svgScene},
		{Name: "mipmaps", Width: 580, Height: 430, Scale: 1, Root: mipmaps},
		{Name: "animated", Width: 400, Height: 150, Scale: 1, Root: animated},
		{Name: "shader", Width: 1320, Height: 780, Scale: 1.5, Root: shaderScene},
		{Name: "svg-features", Width: 1200, Height: 765, Scale: svgFeaturesScale, Root: svgFeatures},
	}...)
	scenes = append(scenes, extra...)
	scenes = append(scenes, rubyScenes()...)
	scenes = append(scenes, bareScenes()...)
	scenes = append(scenes, italicScenes()...)
	// New scenes go last, as above.
	return append(scenes, Scene{Name: "groups", Width: 1281, Height: 351, Scale: 1.5, Root: groups})
}

// Renderer draws scenes on one backend.
type Renderer struct {
	backend   Backend
	resources *render.Resources
	shaper    *text.Shaper
}

// NewRenderer returns a Renderer. Call Destroy before destroying b.
func NewRenderer(b Backend) (*Renderer, error) {
	shaper, err := text.NewShaper(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return &Renderer{backend: b, shaper: shaper, resources: render.NewResources(text.NewAtlas(shaper), b)}, nil
}

// Destroy releases what the Renderer holds on the GPU.
func (r *Renderer) Destroy() { r.resources.Destroy() }

// Render draws s on surface, which has the size of s, once it has come to rest.
func (r *Renderer) Render(s *Scene, surface gpu.Surface) (*image.NRGBA, error) {
	brightness := theme.BrightnessLight
	if s.Dark {
		brightness = theme.BrightnessDark
	}
	th := theme.Set{}.For(brightness)
	var ctx core.BuildContext
	tree := core.NewTree(core.Inherited[*text.Shaper]{Value: r.shaper, Child: basic.Theme{
		Data:  th,
		Child: overlay.Overlay{Child: context{got: &ctx, child: s.Root()}},
	}})
	defer tree.Dispose()

	size := geom.Size{W: float32(s.Width) / s.Scale, H: float32(s.Height) / s.Scale}
	f := frame{tree: tree, size: size, scale: s.Scale, renderer: r, surface: surface, background: th.Color.Canvas}
	if _, err := f.run(false); err != nil {
		return nil, err
	}
	if s.Act != nil {
		s.Act(tree, ctx)
	}
	now := time.Second
	for i := range maxFrames {
		tree.Tick(now)
		now += frameStep
		if i+1 == s.Frames || !tree.NeedsFrame() && !tree.Ticking() {
			return f.run(true)
		}
		if _, err := f.run(false); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("golden: %s never comes to rest", s.Name)
}

// frame runs the pipeline as gowaku.Run does.
type frame struct {
	tree       *core.Tree
	size       geom.Size
	scale      float32
	renderer   *Renderer
	surface    gpu.Surface
	background geom.Color
	canvas     paint.Canvas
	batch      gpu.Batch
}

func (f *frame) run(read bool) (*image.NRGBA, error) {
	f.tree.Build()
	f.tree.Layout(geom.Tight(f.size))
	for f.tree.NeedsBuild() {
		f.tree.Build()
		f.tree.Layout(geom.Tight(f.size))
	}
	if read {
		f.tree.MarkNeedsPaint()
	}
	f.tree.Repaint(&f.canvas)
	r := f.renderer
	render.Encode(f.canvas.DisplayList(), r.resources, f.scale, &f.batch)
	if err := r.resources.Finish(&f.batch); err != nil {
		return nil, err
	}
	f.surface.SetMultisample(f.batch.Multisampled())
	if err := r.backend.BeginFrame(f.surface, f.background); err != nil {
		return nil, err
	}
	if err := r.backend.Draw(f.surface, &f.batch); err != nil {
		return nil, err
	}
	var img *image.NRGBA
	if read {
		var err error
		if img, err = r.backend.ReadPixels(f.surface); err != nil {
			return nil, err
		}
	}
	return img, r.backend.EndFrame(f.surface)
}

// Difference says how two pictures differ.
type Difference struct {
	Worst int
	// Past and Stray count the pixels beyond Tolerance and beyond the tolerance
	// of edges.
	Past, Stray int
	pixels      int
}

// String returns a line for a test log.
func (d Difference) String() string {
	return fmt.Sprintf("worst %d, %d pixels past %d, %d past %d", d.Worst, d.Past, Tolerance, d.Stray, edgeTolerance)
}

// Failed reports whether the difference is more than backends can explain.
func (d Difference) Failed() bool {
	return float64(d.Past) > float64(d.pixels)*edgeShare || d.Stray > strayPixels
}

// Compare measures how far got is from want. It returns an error if their sizes
// differ.
func Compare(got, want *image.NRGBA) (Difference, error) {
	if got.Rect.Size() != want.Rect.Size() {
		return Difference{}, fmt.Errorf("golden: size %v, want %v", got.Rect.Size(), want.Rect.Size())
	}
	d := Difference{pixels: len(got.Pix) / 4}
	for i := 0; i < len(got.Pix); i += 4 {
		pixel := 0
		for c := range 3 {
			pixel = max(pixel, max(int(got.Pix[i+c])-int(want.Pix[i+c]), int(want.Pix[i+c])-int(got.Pix[i+c])))
		}
		d.Worst = max(d.Worst, pixel)
		if pixel > Tolerance {
			d.Past++
		}
		if pixel > edgeTolerance {
			d.Stray++
		}
	}
	return d, nil
}

// Path returns the file of the scene, relative to this package.
func Path(name string) string { return filepath.Join("testdata", name+".png") }

// Load reads the stored picture of the scene.
func Load(path string) (*image.NRGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// An opaque picture decodes to another type.
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Rect, img, img.Bounds().Min, draw.Src)
	return out, nil
}

// Save stores img at path.
func Save(path string, img image.Image) error {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}
