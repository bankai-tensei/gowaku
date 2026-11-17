//go:build gowakudebug

package inspect

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// framesDetailsWidth is the width of the details of the selected frame.
const framesDetailsWidth = 380

// FramesPanel shows the frames as a graph, with the details of one.
//
// The graph stacks the stages of each frame in a bar, the newest at the right,
// with the GPU time as a dash and the allocations in a band below. Hovering a
// bar reads its numbers out above the graph; clicking it, or the arrow keys,
// select it and show its details at the right.
type FramesPanel struct {
	core.StatefulBase
	Session *Session
}

// CreateState implements core.StatefulWidget.
func (FramesPanel) CreateState() core.State { return &framesState{hoverSlot: -1} }

type framesState struct {
	core.StateOf[FramesPanel]
	// hoverSlot is the slot of the ring under the pointer, or -1. The frame in
	// it is found again at each build, as the bars move left with new frames.
	hoverSlot int
	// slots are a region over each slot that tells when the pointer enters
	// it: the tree reports the pointer entering and leaving objects, not its
	// moves. They are made once, so that a rebuild allocates none of them.
	slots []core.Widget
	graph framesGraph
}

// InitState implements core.State.
func (s *framesState) InitState() {
	s.slots = make([]core.Widget, FrameRing)
	for k := range s.slots {
		s.slots[k] = basic.Expanded{Child: interact.MouseRegion{
			OnEnter: func() { s.hover(k) },
			OnExit: func() {
				if s.hoverSlot == k {
					s.hover(-1)
				}
			},
			Child: basic.SizedBox{Height: graphHeight},
		}}
	}
}

// Build implements core.State.
func (s *framesState) Build(ctx core.BuildContext) core.Widget {
	ses := s.Widget().Session
	str, fs := stringsOf(ctx), framesStringsOf(ctx)
	th := basic.ThemeOf(ctx)
	n := ses.Frames()

	g := &s.graph
	g.ses = ses
	g.colors = graphColors(th)
	g.top, g.maxAllocs = scaleOf(ses)
	g.selected = ses.SelectedFrame
	if g.selected >= n {
		g.selected = -1
	}
	g.hover = -1
	if i := s.hoverSlot - (FrameRing - n); s.hoverSlot >= 0 && i >= 0 {
		g.hover = i
	}

	caption := th.Text.Caption.WithColor(th.Color.Ink2)
	var readout string
	switch {
	case g.hover >= 0:
		readout = frameReadout(ses.Frame(g.hover), str, fs)
	case n > 0:
		readout = summaryOf(ses, fs)
	}
	// The readout of a frame fills the line; the pause shows with the summary.
	if ses.Paused && g.hover < 0 {
		if readout != "" {
			readout += "  ·  "
		}
		readout += fs.paused
	}
	header := basic.Row{
		Spacing: theme.Space12,
		Children: []core.Widget{
			basic.Text{Content: str.frames, Style: th.Text.Headline},
			basic.Expanded{Child: basic.Text{Content: readout, Style: caption}},
		},
	}

	var graph core.Widget
	if n == 0 {
		graph = basic.SizedBox{Height: graphHeight, Child: basic.Center{
			Child: basic.Text{Content: fs.noFrames, Style: caption},
		}}
	} else {
		graph = s.graphView(th, fs, summaryOf(ses, fs))
	}

	left := basic.Padding{
		Insets: geom.Symmetric(theme.Space12, theme.Space8),
		Child: basic.Column{
			Cross:   layout.CrossStretch,
			Spacing: theme.Space6,
			Children: []core.Widget{
				header,
				graph,
				legend(th, str, fs),
			},
		},
	}
	return basic.Row{
		Cross: layout.CrossStretch,
		Children: []core.Widget{
			basic.Expanded{Child: left},
			basic.Divider{Axis: layout.Vertical},
			basic.SizedBox{Width: framesDetailsWidth, Child: frameDetails{Session: ses}},
		},
	}
}

// graphView returns the graph with the labels of its lines, which takes the
// pointer and the arrow keys.
func (s *framesState) graphView(th *theme.Theme, fs *framesStrings, summary string) core.Widget {
	g := &s.graph
	label := func(y float32, text string) core.Widget {
		return basic.Positioned{Left: 0, Top: y - 9, Width: graphGutter - theme.Space6, Child: basic.Text{
			Content: text, Style: th.Text.Caption.WithColor(th.Color.Ink2),
		}}
	}
	children := []core.Widget{
		control.CustomPaint{Painter: g, Height: graphHeight},
	}
	// The top of the scale gives way to a budget line that runs close to it.
	if g.y(budget60) >= 2*9+9 {
		children = append(children, label(9, msLabel(g.top, fs)))
	}
	// The label of 120 fps gives way to that of 60 fps when the scale puts
	// them on top of each other.
	for _, budget := range [...]time.Duration{budget60, budget120} {
		if budget >= g.top || budget == budget120 && g.y(budget120)-g.y(budget60) < 18 {
			continue
		}
		children = append(children, label(g.y(budget), msLabel(budget, fs)))
	}
	children = append(children, basic.PositionedFill{
		Insets: layout.Insets{Left: graphGutter},
		Child:  basic.Row{Children: s.slots},
	}, basic.Positioned{
		Left: 0, Top: graphHeight - allocBand, Width: graphGutter - theme.Space6,
		Child: basic.Text{Content: fs.allocsAxis, Style: th.Text.Caption.WithColor(th.Color.Ink2)},
	})

	return interact.Semantics{
		Role: semantics.RoleImage, Label: fs.graph, Value: summary,
		Child: interact.Focus{
			Enabled: true,
			OnKey:   s.key,
			Child: interact.Listener{
				OnPointer: s.pointer,
				Child:     basic.SizedBox{Height: graphHeight, Child: basic.Stack{Children: children}},
			},
		},
	}
}

// pointer selects the frame clicked.
func (s *framesState) pointer(e *core.PointerEvent) {
	if e.Kind != core.PointerDown || e.Button != input.ButtonPrimary {
		return
	}
	if i := s.graph.indexAt(e.Pos.X); i >= 0 {
		s.Widget().Session.SelectFrame(i)
	}
}

// hover moves the readout to the frame in slot k, or back to the summary
// with -1.
func (s *framesState) hover(k int) {
	if k != s.hoverSlot {
		s.SetState(func() { s.hoverSlot = k })
	}
}

// key moves the selection with the arrow keys, Home and End.
func (s *framesState) key(e *core.KeyEvent) bool {
	ses := s.Widget().Session
	n := ses.Frames()
	if n == 0 {
		return false
	}
	i := ses.SelectedFrame
	switch e.Key {
	case input.KeyLeft:
		if i < 0 {
			i = n
		}
		i = max(i-1, 0)
	case input.KeyRight:
		i = min(i+1, n-1)
	case input.KeyHome:
		i = 0
	case input.KeyEnd:
		i = n - 1
	case input.KeyEscape:
		if i < 0 {
			return false
		}
		i = -1
	default:
		return false
	}
	ses.SelectFrame(i)
	return true
}

// graphColors returns the colors of the graph in the theme th.
func graphColors(th *theme.Theme) graphPaint {
	c := &th.Color
	return graphPaint{
		stages:       stageColors(th.Brightness),
		gpu:          c.Ink,
		allocs:       c.Ink3,
		hover:        c.FillStrong,
		selected:     c.AccentTint,
		selectedEdge: c.AccentInk,
		baseline:     c.Hair,
		line120:      c.Ink3.WithAlpha(0.5),
		line60:       c.Danger.WithAlpha(0.6),
	}
}

// legend names the colors of the graph.
func legend(th *theme.Theme, str *inspectStrings, fs *framesStrings) core.Widget {
	style := th.Text.Caption.WithColor(th.Color.Ink2)
	colors := stageColors(th.Brightness)
	item := func(swatch core.Widget, label string) core.Widget {
		return basic.Row{
			Size:    layout.MainMin,
			Spacing: theme.Space6,
			Children: []core.Widget{
				swatch, basic.Text{Content: label, Style: style},
			},
		}
	}
	box := func(col geom.Color) core.Widget {
		return basic.Container{Width: 10, Height: 10, Radius: 3, Color: col}
	}
	line := func(col geom.Color, h float32) core.Widget {
		return basic.SizedBox{Width: 14, Height: 10, Child: basic.Center{
			Child: basic.Container{Width: 14, Height: h, Color: col},
		}}
	}
	items := make([]core.Widget, 0, StageCount+5)
	for st := range StageCount {
		items = append(items, item(box(colors[st]), str.stageNames[st]))
	}
	items = append(items,
		item(line(th.Color.Ink, 2), str.gpu),
		item(box(th.Color.Ink3), fs.allocsAxis),
		basic.Expanded{Child: basic.SizedBox{}},
		item(line(th.Color.Ink3.WithAlpha(0.5), 1), fs.fps120),
		item(line(th.Color.Danger.WithAlpha(0.6), 1), fs.fps60),
	)
	return basic.Row{Spacing: theme.Space16, Children: items}
}
