// Command gowaku runs the scenarios of the benchmark on gowaku. See ../README.md.
package main

import (
	"log"
	"math"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/bench/frames"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// probe marks a frame whenever it paints. That is every frame in which what is
// around it changes, so that a frame without any change goes unnoticed.
type probe struct{ rec *frames.Recorder }

// Key implements core.Widget.
func (probe) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p probe) CreateRenderObject() core.RenderObject { return &renderProbe{rec: p.rec} }

// UpdateRenderObject implements core.RenderWidget.
func (probe) UpdateRenderObject(core.RenderObject) {}

type renderProbe struct {
	core.RenderBase
	rec     *frames.Recorder
	started bool
}

// viewHeight is the height of the window, which the probe learns from its
// constraints, and which autoScroll needs to place the list to the pixel.
var viewHeight float32

// Layout implements core.RenderObject.
func (r *renderProbe) Layout(c geom.Constraints) geom.Size {
	viewHeight = c.MaxH
	return c.Constrain(geom.Size{})
}

// Paint implements core.RenderObject.
func (r *renderProbe) Paint(*paint.Canvas, geom.Offset) {
	r.rec.Mark()
	if !r.started {
		r.started = true
		// Posts run after the frame in progress has been presented.
		gowaku.Post(r.rec.Ready)
	}
}

type list struct {
	gowaku.StatelessBase
	icon *paint.Path
}

func (l list) row(th *theme.Theme, i int) gowaku.Widget {
	return basic.Padding{Insets: geom.Symmetric(16, 4), Child: basic.Row{
		Spacing: 12,
		Cross:   layout.CrossCenter,
		Children: []gowaku.Widget{
			basic.Icon{Path: l.icon, Color: th.Color.Accent},
			basic.Expanded{Child: basic.Text{Content: frames.Label(i), Style: th.Text.Body}},
			control.Button{Label: "開く", OnPress: func() {}},
		},
	}}
}

// Build implements core.StatelessWidget.
func (l list) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return collection.List{
		ItemCount: frames.Rows, ItemExtent: rowExtent,
		ItemBuilder: func(i int) gowaku.Widget { return l.row(th, i) },
	}
}

// rowExtent is the height of a row of the list.
const rowExtent = 52

// autoScroll is the list of the scroll scenario moving by itself at
// frames.ScrollSpeed, for a harness that cannot send the wheel (macOS without
// the accessibility permission). It is what the Gio program always does: one
// offset a frame, to the pixel, without the glide of the wheel.
type autoScroll struct {
	gowaku.StatefulBase
	list list
}

type autoScrollState struct {
	gowaku.StateBase
	ctrl    collection.ListController
	tick    *core.Ticker
	start   time.Duration
	started bool
}

func (autoScroll) CreateState() gowaku.State { return &autoScrollState{} }

// InitState implements gowaku.State.
func (s *autoScrollState) InitState() {
	s.tick = s.NewTicker(s.advance)
	s.tick.Start()
}

// advance puts the offset frames.ScrollSpeed times the time since the first tick
// at the top of the view: row i with its top below that offset, aligned up by
// the rest.
func (s *autoScrollState) advance(now time.Duration) {
	if !s.started {
		s.start, s.started = now, true
	}
	if viewHeight <= rowExtent {
		return
	}
	y := float32(frames.ScrollSpeed * (now - s.start).Seconds())
	i := int(math.Ceil(float64(y / rowExtent)))
	s.ctrl.JumpToIndex(i, (float32(i)*rowExtent-y)/(viewHeight-rowExtent))
}

// Build implements gowaku.State.
func (s *autoScrollState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	l := s.Widget().(autoScroll).list.Build(ctx).(collection.List)
	l.Controller = &s.ctrl
	return l
}

// boxes is the animate scenario: AnimatedContainers that change color, size and
// shadow at once. It only ever says where they should go.
type boxes struct {
	gowaku.StatefulBase
	// side is the length of a side of the grid. Zero selects frames.BoxesSide.
	side int
}

type boxesState struct {
	gowaku.StateBase
	on bool
}

func (boxes) CreateState() gowaku.State { return &boxesState{} }

// InitState implements gowaku.State.
func (s *boxesState) InitState() {
	go func() {
		// A little ahead of the end of the motion, or frames would stop for a
		// moment at every turn and the count would say so.
		for range time.Tick(950 * time.Millisecond) {
			gowaku.Post(s.flip)
		}
	}()
	gowaku.Post(s.flip)
}

func (s *boxesState) flip() { s.SetState(func() { s.on = !s.on }) }

func (s *boxesState) box() gowaku.Widget {
	size, color, blur := float32(24), geom.Hex(0x2f5fd0), float32(2)
	if s.on {
		size, color, blur = 56, geom.Hex(0xd0502f), 16
	}
	return basic.SizedBox{Width: 64, Height: 64, Child: basic.Center{Child: basic.AnimatedContainer{
		Duration: time.Second,
		Curve:    motion.EaseInOutCubic,
		Width:    size,
		Height:   size,
		Radius:   8,
		Color:    color,
		Shadows:  []paint.BoxShadow{{Color: geom.Hex(0x000000).WithAlpha(0.3), Blur: blur, Offset: geom.Offset{Y: 2}}},
	}}}
}

// Build implements gowaku.State.
func (s *boxesState) Build(gowaku.BuildContext) gowaku.Widget {
	side := s.Widget().(boxes).side
	if side == 0 {
		side = frames.BoxesSide
	}
	rows := make([]gowaku.Widget, side)
	for i := range rows {
		cells := make([]gowaku.Widget, side)
		for j := range cells {
			cells[j] = s.box()
		}
		rows[i] = basic.Row{Children: cells}
	}
	return basic.Column{Children: rows}
}

// dense is the dense scenarios: a window full of text that stays, with one small
// box that moves. It exists to show what layers buy, so that only gowaku has it.
// With flat, nothing separates the two.
func dense(rec *frames.Recorder, flat bool) gowaku.Widget {
	rows := make([]gowaku.Widget, 34)
	for i := range rows {
		cells := make([]gowaku.Widget, 10)
		for j := range cells {
			cells[j] = basic.Expanded{Child: basic.Text{Content: frames.Label(i*10 + j)}}
		}
		rows[i] = basic.SizedBox{Height: 20, Child: basic.Row{Children: cells}}
	}
	var page gowaku.Widget = basic.Column{Children: rows}
	var box gowaku.Widget = basic.Layers(boxes{side: 1}, probe{rec})
	if !flat {
		page = basic.RepaintBoundary{Child: page}
		box = basic.RepaintBoundary{Child: box}
	}
	return basic.Layers(page, basic.Center{Child: basic.SizedBox{Width: 64, Height: 64, Child: box}})
}

func main() {
	f := frames.Parse()
	rec := frames.New(f)
	icon, err := paint.ParseSVGPath("M12 2l3 7h7l-5.5 4.5L18.5 21 12 16.8 5.5 21l2-7.5L2 9h7z")
	if err != nil {
		log.Fatal(err)
	}
	var page gowaku.Widget = list{icon: icon}
	switch f.Scenario {
	case "animate":
		page = boxes{}
	case "scroll":
		if f.AutoScroll {
			page = autoScroll{list: list{icon: icon}}
		}
	case "startup":
		page = basic.Center{Child: basic.Text{Content: "こんにちは"}}
	}
	var home gowaku.Widget = basic.Layers(page, probe{rec})
	if f.Scenario == "dense" || f.Scenario == "dense-flat" {
		home = dense(rec, f.Scenario == "dense-flat")
	}
	err = gowaku.Run(gowaku.App{Title: "bench", Width: 800, Height: 700, Home: home})
	if err != nil {
		log.Fatal(err)
	}
}
