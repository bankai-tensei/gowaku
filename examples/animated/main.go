// Command animated plays animated pictures side by side with AnimatedImage: two
// GIF files that it makes itself, and any GIF, APNG, WebP, PNG or JPEG files
// named on the command line. See docs/ADR/0045-shaders-svg-animated-images.md.
//
//	animated                                     the two made-up GIF files
//	animated paint/testdata/anim/movie.webp      and an animated WebP file
//	animated -png out.png [files]                draws a frame and exits
//
// The switch hides the pictures with a fully transparent Opacity, which pauses
// them; they go on where they were when they show again.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// clip is an animation with a caption.
type clip struct {
	name string
	anim *paint.Animation
}

// spinnerGIF returns twelve frames of a ring of dots, one of them dark, that
// goes around on a transparent background.
func spinnerGIF() []byte {
	const size, dots = 48, 12
	pal := color.Palette{color.RGBA{}, color.RGBA{0x2f, 0x6f, 0xeb, 0xff}, color.RGBA{0xb8, 0xcc, 0xf4, 0xff}}
	g := &gif.GIF{Config: image.Config{ColorModel: pal, Width: size, Height: size}}
	for f := range dots {
		m := image.NewPaletted(image.Rect(0, 0, size, size), pal)
		for d := range dots {
			a := 2 * math.Pi * float64(d) / dots
			cx, cy := size/2+17*math.Cos(a), size/2+17*math.Sin(a)
			index := uint8(2)
			if d == f {
				index = 1
			}
			disc(m, cx, cy, 4, index)
		}
		g.Image = append(g.Image, m)
		g.Delay = append(g.Delay, 8)
		g.Disposal = append(g.Disposal, gif.DisposalBackground)
	}
	return encode(g)
}

// ballGIF returns a ball that bounces on a floor. Each frame after the first
// is only the square around the ball, which disposes to the picture before it.
func ballGIF() []byte {
	const w, h, frames, r = 96, 64, 24, 7
	pal := color.Palette{color.RGBA{}, color.RGBA{0xf4, 0xf1, 0xea, 0xff}, color.RGBA{0x6b, 0x70, 0x7a, 0xff}, color.RGBA{0xe8, 0x50, 0x30, 0xff}}
	g := &gif.GIF{Config: image.Config{ColorModel: pal, Width: w, Height: h}}
	floor := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for i := range floor.Pix {
		floor.Pix[i] = 1
	}
	for x := range w {
		floor.SetColorIndex(x, h-4, 2)
	}
	g.Image = append(g.Image, floor)
	g.Delay = append(g.Delay, 4)
	g.Disposal = append(g.Disposal, gif.DisposalNone)
	for f := range frames {
		t := float64(f) / frames
		cx := 12 + t*(w-24)
		cy := float64(h-4-r) - 40*math.Abs(math.Sin(math.Pi*2*t))
		box := image.Rect(int(cx)-r-1, int(cy)-r-1, int(cx)+r+2, int(cy)+r+2)
		m := image.NewPaletted(box, pal)
		disc(m, cx, cy, r, 3)
		g.Image = append(g.Image, m)
		g.Delay = append(g.Delay, 4)
		g.Disposal = append(g.Disposal, gif.DisposalPrevious)
	}
	return encode(g)
}

// disc paints a disc of radius r around (cx, cy) with a palette index.
func disc(m *image.Paletted, cx, cy, r float64, index uint8) {
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy; dx*dx+dy*dy <= r*r {
				m.SetColorIndex(x, y, index)
			}
		}
	}
}

func encode(g *gif.GIF) []byte {
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

func decode(name string, data []byte) clip {
	a, err := paint.DecodeAnimation(bytes.NewReader(data))
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	return clip{name: fmt.Sprintf("%s, %d frames", name, len(a.Frames)), anim: a}
}

type page struct {
	gowaku.StatefulBase
	clips []clip
}

type pageState struct {
	gowaku.StateBase
	hidden bool
}

func (page) CreateState() gowaku.State { return &pageState{} }

func (s *pageState) Build(gowaku.BuildContext) gowaku.Widget {
	clips := s.Widget().(page).clips
	cells := make([]gowaku.Widget, 0, len(clips))
	for _, c := range clips {
		cells = append(cells, basic.Column{Spacing: 8, Children: []gowaku.Widget{
			basic.Container{Color: geom.Hex(0xe4e7ec), Radius: 8, Padding: geom.All(8), Child: basic.AnimatedImage{
				Source: c.anim, Width: 160, Height: 120, Label: c.name,
			}},
			basic.Text{Content: c.name},
		}})
	}
	opacity := float32(1)
	if s.hidden {
		opacity = 0
	}
	return basic.Padding{Insets: geom.All(24), Child: basic.Column{Spacing: 20, Children: []gowaku.Widget{
		basic.Row{Spacing: 12, Children: []gowaku.Widget{
			control.Switch{Value: !s.hidden, Label: "Show", OnChange: func(v bool) { s.SetState(func() { s.hidden = !v }) }},
			basic.Text{Content: "Hidden pictures pause and go on where they were."},
		}},
		basic.Opacity{Opacity: opacity, Child: basic.Wrap{Spacing: 16, RunSpacing: 16, Children: cells}},
	}}}
}

func screenshot(root gowaku.Widget, path string) error {
	h, err := shot.Open(root, shot.Options{Width: 840, Height: 300, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	// Let the animations play for a moment after the first frame, which can
	// take long, so that they are not on their first frames.
	if _, err := h.Frame(); err != nil {
		return err
	}
	for start := time.Now(); time.Since(start) < 1100*time.Millisecond; {
		if _, err := h.Frame(); err != nil {
			return err
		}
		h.Pump()
	}
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	flag.Parse()
	clips := make([]clip, 0, 2+flag.NArg())
	clips = append(clips, decode("spinner.gif", spinnerGIF()), decode("ball.gif", ballGIF()))
	for _, path := range flag.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		clips = append(clips, decode(filepath.Base(path), data))
	}
	root := page{clips: clips}
	var err error
	if *pngPath != "" {
		err = screenshot(root, *pngPath)
	} else {
		err = gowaku.Run(gowaku.App{Title: "gowaku: animated", Width: 840, Height: 300, Home: root})
	}
	if err != nil {
		log.Fatal(err)
	}
}
