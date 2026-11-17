// Command zoom has a list that zooms with a pinch of the touchpad, or Ctrl and
// the wheel, and a pad that draws with a finger, a pen or the mouse, as thick
// as the pen presses. A finger drags the list to scroll it, and two fingers
// zoom it. See docs/ADR/0052-pinch-touch-and-pen.md.
//
//	zoom             opens the window
//	zoom -dark       the same in the dark theme
//	zoom -zoom 1.8   starts zoomed in, for screenshots
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	minZoom = 0.5
	maxZoom = 3
	// rowHeight and textSize are those of a row at a zoom of 1.
	rowHeight = 40
	textSize  = 14
)

type page struct {
	core.StatefulBase
	zoom float32
}

type pageState struct {
	core.StateBase
	zoom   float32
	status string
	pad    *drawing
	signal layout.RepaintSignal
}

func (p page) CreateState() core.State {
	return &pageState{zoom: p.zoom, status: "一覧の上でピンチ（または Ctrl+ホイール）", pad: &drawing{}}
}

func (s *pageState) pinch(factor float32, _ geom.Offset) {
	s.SetState(func() {
		s.zoom = geom.Clamp32(s.zoom*factor, minZoom, maxZoom)
		s.status = fmt.Sprintf("ズーム %.0f%%", s.zoom*100)
	})
}

func (s *pageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	zoom := s.zoom
	list := interact.GestureDetector{OnPinch: s.pinch, Child: collection.List{
		ItemCount: 1000, ItemExtent: rowHeight * zoom,
		ItemBuilder: func(i int) core.Widget {
			return basic.Container{
				Padding: geom.Symmetric(theme.Space12*zoom, 0),
				Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{
					Content: fmt.Sprintf("%d 行目　ピンチで大きく", i+1),
					Style:   text.Style{Size: textSize * zoom, Color: th.Color.Ink},
				}},
			}
		},
	}}
	pad := interact.Listener{
		OnPointer: func(e *core.PointerEvent) {
			if s.pad.add(e) {
				s.signal.Notify()
				if e.Kind == core.PointerDown {
					s.SetState(func() { s.status = deviceName(e.Device) + "で描いている" })
				}
			}
		},
		Child: basic.Container{
			Color: th.Color.Card, Radius: th.Radius.Group, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			Child: control.CustomPaint{Painter: s.pad, Repaint: &s.signal},
		},
	}
	return basic.Padding{Insets: geom.All(theme.Space16), Child: basic.Column{
		Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
			basic.Expanded{Child: basic.Row{Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
				basic.Expanded{Child: basic.Container{
					Color: th.Color.Card, Radius: th.Radius.Group, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
					Child: list,
				}},
				basic.Expanded{Child: pad},
			}}},
			basic.Row{Spacing: theme.Space12, Children: []core.Widget{
				basic.Expanded{Child: basic.Text{Content: s.status, Style: th.Text.Caption.WithColor(th.Color.Ink2)}},
				control.Button{Label: "消す", Size: control.ButtonSmall, OnPress: func() { s.pad.clear(); s.signal.Notify() }},
			}},
		},
	}}
}

func deviceName(d input.PointerDevice) string {
	switch d {
	case input.DeviceTouch:
		return "指"
	case input.DevicePen:
		return "ペン"
	}
	return "マウス"
}

// drawing is the strokes of the pad: dots as thick as the pen pressed.
type drawing struct {
	dots []dot
	down bool
	// device is what pressed: the moves of others, such as the mouse while
	// a pen draws, are not the stroke.
	device input.PointerDevice
}

type dot struct {
	pos    geom.Offset
	radius float32
	// join means the dot continues the stroke of the one before.
	join bool
}

// add takes a pointer event, and reports whether the drawing changed.
func (d *drawing) add(e *core.PointerEvent) bool {
	radius := float32(2)
	if e.Device == input.DevicePen && e.Pressure > 0 {
		radius = 1 + 7*e.Pressure
	}
	switch e.Kind {
	case core.PointerDown:
		d.down, d.device = true, e.Device
		d.dots = append(d.dots, dot{pos: e.Pos, radius: radius})
	case core.PointerMove:
		if !d.down || e.Device != d.device {
			return false
		}
		d.dots = append(d.dots, dot{pos: e.Pos, radius: radius, join: true})
	case core.PointerUp, core.PointerCancel:
		if e.Device == d.device {
			d.down = false
		}
		return false
	default:
		return false
	}
	return true
}

func (d *drawing) clear() { d.dots, d.down = d.dots[:0], false }

// Paint implements layout.Painter.
func (d *drawing) Paint(cv *paint.Canvas, sz geom.Size) {
	cv.Save()
	cv.ClipRect(geom.Rect{W: sz.W, H: sz.H})
	ink := paint.Paint{Color: geom.Hex(0x2f5fd0)}
	for i, p := range d.dots {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: p.pos.X - p.radius, Y: p.pos.Y - p.radius, W: 2 * p.radius, H: 2 * p.radius}, p.radius), ink)
		if p.join && i > 0 {
			// Dots between two far apart ones, so that a fast stroke stays whole.
			q := d.dots[i-1]
			v := p.pos.Sub(q.pos)
			steps := int(v.Length() / max(p.radius, 1))
			for k := 1; k < steps; k++ {
				f := float32(k) / float32(steps)
				c := q.pos.Add(v.Scale(f))
				r := q.radius + (p.radius-q.radius)*f
				cv.DrawRRect(geom.RRectAll(geom.Rect{X: c.X - r, Y: c.Y - r, W: 2 * r, H: 2 * r}, r), ink)
			}
		}
	}
	cv.Restore()
}

func main() {
	dark := flag.Bool("dark", false, "use the dark theme")
	zoom := flag.Float64("zoom", 1, "the zoom to start at, for screenshots")
	flag.Parse()
	mode := gowaku.ThemeLight
	if *dark {
		mode = gowaku.ThemeDark
	}
	if err := gowaku.Run(gowaku.App{Title: "gowaku: zoom", Width: 860, Height: 520, ThemeMode: mode, Home: page{zoom: float32(*zoom)}}); err != nil {
		log.Fatal(err)
	}
}
