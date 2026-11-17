// Command decorations shows what a window can be asked for besides its
// contents: an icon, bounds on its size, always on top, fullscreen and a
// transparent background. See docs/ADR/0111-window-decorations.md.
//
//	decorations                  a window with the gowaku mark as its icon
//	decorations -custom          the same with a title bar of its own
//	decorations -transparent     a window whose background shows the desktop
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// The colors of docs/brand.
var (
	ink    = color.NRGBA{0x1d, 0x1d, 0x1f, 0xff}
	paper  = color.NRGBA{0xf5, 0xf5, 0xf7, 0xff}
	accent = color.NRGBA{0x2f, 0x6b, 0xff, 0xff}
)

// mark draws the mark of docs/brand at size pixels on a rounded tile, light
// or dark: a rounded frame with the blue dot inside.
func mark(size int, dark bool) *image.NRGBA {
	tile, frame := paper, ink
	if dark {
		tile, frame = ink, paper
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size) / 64
	for y := range size {
		for x := range size {
			// The center of the pixel, in the 64 units of the SVG.
			px, py := (float64(x)+0.5)/s, (float64(y)+0.5)/s
			c := color.NRGBA{}
			c = over(c, tile, cover(roundRect(px, py, 32, 32, 32, 14), s))
			ring := math.Abs(roundRect(px, py, 32, 32, 23, 15)) - 6.5
			c = over(c, frame, cover(ring, s))
			c = over(c, accent, cover(roundRect(px, py, 32, 32, 7, 4), s))
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// roundRect is the signed distance from (x, y) to a square around (cx, cy)
// with half side h and corner radius r.
func roundRect(x, y, cx, cy, h, r float64) float64 {
	dx, dy := math.Max(math.Abs(x-cx)-(h-r), 0), math.Max(math.Abs(y-cy)-(h-r), 0)
	inside := math.Min(math.Max(math.Abs(x-cx), math.Abs(y-cy))-(h-r), 0)
	return math.Hypot(dx, dy) + inside - r
}

// cover turns a distance in SVG units into the coverage of a pixel.
func cover(d, scale float64) float64 { return math.Max(0, math.Min(1, 0.5-d*scale)) }

// over puts src with coverage a over dst.
func over(dst, src color.NRGBA, a float64) color.NRGBA {
	sa := a * float64(src.A) / 255
	da := float64(dst.A) / 255 * (1 - sa)
	out := sa + da
	if out == 0 {
		return color.NRGBA{}
	}
	mix := func(s, d uint8) uint8 { return uint8((float64(s)*sa + float64(d)*da) / out) }
	return color.NRGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), uint8(out*255 + 0.5)}
}

type demo struct {
	gowaku.StatefulBase
	transparent bool
	custom      bool
}

type demoState struct {
	gowaku.StateBase
	onTop, dark, bounded bool
}

func (demo) CreateState() gowaku.State { return &demoState{bounded: true} }

func (s *demoState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	d := s.Widget().(demo)
	th := basic.ThemeOf(ctx)
	win := gowaku.WindowOf(ctx)
	row := func(label string, on bool, change func(bool)) gowaku.Widget {
		return basic.Row{Cross: layout.CrossCenter, Spacing: 12, Children: []gowaku.Widget{
			basic.Expanded{Child: basic.Text{Content: label, Style: th.Text.Body}},
			control.Switch{Value: on, Label: label, OnChange: change},
		}}
	}
	full := win.Fullscreen()
	body := basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 14, Children: []gowaku.Widget{
		basic.Text{Content: "ウィンドウの装飾", Style: th.Text.Title},
		basic.Text{Content: fmt.Sprintf("透過: %v　フルスクリーン: %v", win.Transparent(), full), Style: th.Text.Caption},
		row("常に手前", s.onTop, func(on bool) {
			s.SetState(func() { s.onTop = on })
			win.SetAlwaysOnTop(on)
		}),
		row("フルスクリーン", full, func(on bool) {
			win.SetFullscreen(on)
			s.SetState(func() {})
		}),
		row("アイコンを暗い版に", s.dark, func(on bool) {
			s.SetState(func() { s.dark = on })
			win.SetIcon([]image.Image{mark(256, on)})
		}),
		row("大きさを 360×260〜1000×760 に制限", s.bounded, func(on bool) {
			s.SetState(func() { s.bounded = on })
			if on {
				win.SetSizeLimits(geom.Size{W: 360, H: 260}, geom.Size{W: 1000, H: 760})
			} else {
				win.SetSizeLimits(geom.Size{}, geom.Size{})
			}
		}),
	}}
	card := basic.Container{
		Width: 380, Padding: geom.All(24), Radius: th.Radius.Group, Color: th.Color.Canvas.WithAlpha(0.92),
		Child: body,
	}
	content := gowaku.Widget(basic.Center{Child: card})
	if d.custom {
		content = basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
			gowaku.TitleBar{}, basic.Expanded{Child: content},
		}}
	}
	return content
}

func main() {
	transparent := flag.Bool("transparent", false, "open a window whose background shows the desktop")
	custom := flag.Bool("custom", false, "draw the title bar in the application")
	flag.Parse()
	err := gowaku.Run(gowaku.App{
		Title:       "gowaku: decorations",
		Width:       520,
		Height:      420,
		Icon:        []image.Image{mark(256, false)},
		MinSize:     geom.Size{W: 360, H: 260},
		MaxSize:     geom.Size{W: 1000, H: 760},
		Transparent: *transparent,
		CustomFrame: *custom,
		AppID:       "dev.gowaku.Decorations",
		Home:        demo{transparent: *transparent, custom: *custom},
	})
	if err != nil {
		log.Fatal(err)
	}
}
