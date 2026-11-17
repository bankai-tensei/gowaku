// Command gio runs the scenarios of the benchmark on Gio. See ../README.md.
package main

import (
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/bankai-tensei/gowaku/examples/bench/frames"
)

type (
	// C and D are the customary shorthands of Gio.
	C = layout.Context
	D = layout.Dimensions
)

// page draws one scenario.
type page struct {
	th      *material.Theme
	list    widget.List
	icon    *widget.Icon
	buttons [128]widget.Clickable
	start   time.Time
	last    time.Time
}

func (p *page) row(gtx C, i int) D {
	gtx.Constraints.Min.Y = gtx.Dp(52)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return layout.Inset{Left: 16, Right: 16, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(24)
				return p.icon.Layout(gtx, p.th.ContrastBg)
			}),
			layout.Rigid(layout.Spacer{Width: 12}.Layout),
			layout.Flexed(1, material.Body2(p.th, frames.Label(i)).Layout),
			layout.Rigid(material.Button(p.th, &p.buttons[i%len(p.buttons)], "開く").Layout),
		)
	})
}

func (p *page) list10k(gtx C) D {
	return material.List(p.th, &p.list).Layout(gtx, frames.Rows, p.row)
}

// scroll moves the list by itself, as fast as the wheel of the harness moves the
// lists of the other programs: Gio reads the wheel through WM_POINTERWHEEL, which
// another process cannot post.
func (p *page) scroll(gtx C) D {
	now := time.Now()
	if !p.last.IsZero() {
		p.list.Position.Offset += gtx.Dp(unit.Dp(frames.ScrollSpeed * now.Sub(p.last).Seconds()))
	}
	p.last = now
	gtx.Execute(op.InvalidateCmd{})
	return p.list10k(gtx)
}

// lerp8 mixes two channels.
func lerp8(a, b uint8, t float32) uint8 { return uint8(float32(a) + (float32(b)-float32(a))*t) }

// ease is the curve of the other programs.
func ease(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// rrect fills a rounded rectangle.
func rrect(gtx C, r image.Rectangle, radius int, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

// animate draws the boxes. Gio has no animation and no blurred shadow: every
// frame works out the values by hand, and rings of translucent rectangles stand
// in for the blur.
func (p *page) animate(gtx C) D {
	t := ease(frames.Wave(time.Since(p.start)))
	c := color.NRGBA{R: lerp8(0x2f, 0xd0, t), G: lerp8(0x5f, 0x50, t), B: lerp8(0xd0, 0x2f, t), A: 0xff}
	cell := gtx.Dp(64)
	size := gtx.Dp(unit.Dp(24 + 32*t))
	blur := gtx.Dp(unit.Dp(2 + 14*t))
	for i := range frames.Boxes {
		x := (i%frames.BoxesSide)*cell + (cell-size)/2
		y := (i/frames.BoxesSide)*cell + (cell-size)/2
		box := image.Rect(x, y, x+size, y+size)
		for ring := 4; ring > 0; ring-- {
			grown := box.Inset(-blur * ring / 4).Add(image.Pt(0, gtx.Dp(2)))
			rrect(gtx, grown, gtx.Dp(8)+blur*ring/4, color.NRGBA{A: 0x12})
		}
		rrect(gtx, box, gtx.Dp(8), c)
	}
	gtx.Execute(op.InvalidateCmd{})
	return D{Size: gtx.Constraints.Max}
}

func (p *page) hello(gtx C) D {
	return layout.Center.Layout(gtx, material.Body1(p.th, "こんにちは").Layout)
}

func run(f frames.Flags, rec *frames.Recorder) {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	icon, err := widget.NewIcon(icons.ToggleStar)
	if err != nil {
		log.Fatal(err)
	}
	p := &page{th: th, icon: icon, start: time.Now()}
	p.list.Axis = layout.Vertical
	draw := p.list10k
	switch f.Scenario {
	case "scroll":
		draw = p.scroll
	case "animate":
		draw = p.animate
	case "startup":
		draw = p.hello
	}
	w := new(app.Window)
	w.Option(app.Title("bench"), app.Size(800, 700))
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			os.Exit(0)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			draw(gtx)
			e.Frame(gtx.Ops)
			rec.Mark()
			rec.Ready()
		}
	}
}

func main() {
	f := frames.Parse()
	rec := frames.New(f)
	go run(f, rec)
	app.Main()
}
