package main

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of InteractiveViewer: a large drawing with buttons on it, which
// pans and zooms. See docs/ADR/0092-interactive-viewer.md.

const (
	// boardWidth and boardHeight are the size of the drawing, and boardGrid
	// the step of its grid.
	boardWidth  = 1600
	boardHeight = 1200
	boardGrid   = 40
)

// pageViewer returns the page of InteractiveViewer.
func pageViewer() core.Widget { return viewerPage{} }

type viewerPage struct{ core.StatefulBase }

type viewerPageState struct {
	core.StateBase
	view    interact.ViewerController
	scale   scaleHub
	pressed string
}

func (viewerPage) CreateState() core.State { return &viewerPageState{} }

// InitState shows the whole drawing once the viewer knows its size.
func (s *viewerPageState) InitState() { s.view.Fit(false) }

func (s *viewerPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	note := th.Text.Body
	note.Color = th.Color.Ink2
	status := "ドラッグで移動、Ctrl+ホイールかピンチで拡大縮小。図の上のボタンも押せます。"
	if s.pressed != "" {
		status = "「" + s.pressed + "」を押しました。"
	}
	press := func(name string) func() {
		return func() { s.SetState(func() { s.pressed = name }) }
	}
	return basic.Pad(theme.Space28, basic.Column{Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
		heading(th, "パンとズーム", "InteractiveViewer は大きな図を動かして、拡大して見せます。"),
		basic.Row{Size: layout.MainMin, Spacing: theme.Space8, Cross: layout.CrossCenter, Children: []core.Widget{
			control.Button{Label: "全体を表示", OnPress: func() { s.view.Fit(true) }},
			control.Button{Label: "等倍に戻す", OnPress: func() { s.view.Reset(true) }},
			control.Button{Label: "中央の円へ", OnPress: func() {
				s.view.ZoomToRect(geom.Rect{X: 650, Y: 450, W: 300, H: 300}, true)
			}},
			basic.SizedBox{Width: theme.Space8},
			scaleLabel{hub: &s.scale},
		}},
		basic.Text{Content: status, Style: note},
		basic.Expanded{Child: basic.Container{
			Radius: th.Radius.Control, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			Child: interact.InteractiveViewer{
				Controller: &s.view, MinScale: 0.2, MaxScale: 6, BoundaryMargin: 24,
				DoubleTapZoom: true, Label: "図面",
				OnChange: s.scale.set,
				Child:    board(th, press),
			},
		}},
	}})
}

// board is the drawing: a grid, a few shapes and buttons placed on it.
func board(th *theme.Theme, press func(string) func()) core.Widget {
	c := &th.Color
	shape := func(x, y, w, h, r float32, color geom.Color) core.Widget {
		return basic.Positioned{Left: x, Top: y, Width: w, Height: h, Child: basic.Container{Color: color, Radius: r}}
	}
	button := func(x, y float32, label string) core.Widget {
		return basic.Positioned{Left: x, Top: y, Child: control.Button{Label: label, OnPress: press(label)}}
	}
	caption := th.Text.Title
	caption.Color = c.Ink
	return basic.Stack{Children: []core.Widget{
		control.CustomPaint{Width: boardWidth, Height: boardHeight, Painter: gridPainter{
			ground: c.Card, line: c.Hair, major: c.FillStrong,
		}},
		shape(160, 160, 360, 220, th.Radius.Dialog, c.AccentTint),
		shape(650, 450, 300, 300, 150, c.Accent),
		shape(1080, 200, 280, 280, 24, c.Fill),
		shape(240, 760, 520, 120, 60, c.FillStrong),
		shape(1100, 820, 320, 200, th.Radius.Control, c.AccentTint),
		basic.Positioned{Left: 180, Top: 120, Child: basic.Text{Content: "左上の区画", Style: caption}},
		basic.Positioned{Left: 690, Top: 400, Child: basic.Text{Content: "中央の円", Style: caption}},
		basic.Positioned{Left: 1100, Top: 780, Child: basic.Text{Content: "右下の区画", Style: caption}},
		button(200, 300, "開く"),
		button(1120, 420, "詳細"),
		button(1140, 940, "保存"),
		button(300, 790, "共有"),
	}}
}

// gridPainter draws the ground of the drawing with a grid, every fifth line
// stronger.
type gridPainter struct {
	ground, line, major geom.Color
}

func (g gridPainter) Paint(cv *paint.Canvas, sz geom.Size) {
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{W: sz.W, H: sz.H}}, paint.Paint{Color: g.ground})
	for i := 0; float32(i*boardGrid) <= sz.W; i++ {
		color := g.line
		if i%5 == 0 {
			color = g.major
		}
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: float32(i * boardGrid), W: 1, H: sz.H}}, paint.Paint{Color: color})
	}
	for i := 0; float32(i*boardGrid) <= sz.H; i++ {
		color := g.line
		if i%5 == 0 {
			color = g.major
		}
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{Y: float32(i * boardGrid), W: sz.W, H: 1}}, paint.Paint{Color: color})
	}
}

// scaleHub carries the scale from the viewer's OnChange to the label, which
// alone builds again: the drawing does not.
type scaleHub struct {
	scale float32
	label *scaleLabelState
}

func (h *scaleHub) set(scale float32, _ geom.Offset) {
	h.scale = scale
	if h.label != nil {
		h.label.SetState(func() {})
	}
}

// scaleLabel shows the scale of the viewer as a percentage.
type scaleLabel struct {
	core.StatefulBase
	hub *scaleHub
}

type scaleLabelState struct {
	core.StateOf[scaleLabel]
}

func (scaleLabel) CreateState() core.State { return &scaleLabelState{} }

func (s *scaleLabelState) InitState() { s.Widget().hub.label = s }

func (s *scaleLabelState) Dispose() {
	if h := s.Widget().hub; h.label == s {
		h.label = nil
	}
}

func (s *scaleLabelState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	style := th.Text.Body
	style.Color = th.Color.Ink
	scale := s.Widget().hub.scale
	if scale == 0 {
		scale = 1
	}
	return basic.Text{Content: "倍率 " + strconv.Itoa(int(scale*100+0.5)) + "%", Style: style}
}
