package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// linkRadius rounds the focus ring around the text of a link.
	linkRadius = theme.Space4
	// linkPress is how far a press darkens the ink of a link toward Ink.
	linkPress = 0.3
)

// Link is a short text that does something when pressed, such as "Open" or
// "Learn more" beside the text it belongs to:
//
//	Row{Size: layout.MainMin, Spacing: theme.Space4, Children: []core.Widget{
//		Text{Content: "Sync is off."},
//		Link{Label: "Turn on", OnPress: s.turnOn},
//	}}
//
// It is set in the accent ink of the theme, and underlined while the pointer
// is over it, which also shows the hand. It is a stop of Tab with a focus
// ring, and Enter presses it; Space does not, as in a web page, where Space
// scrolls. A screen reader finds a link named Label. Without OnPress it is
// disabled: set in Ink3, with no underline or hand, and no stop of Tab.
//
// A Link is a widget of its own, beside the text in a Row or a Wrap; it does
// not go inside the spans of a RichText. A long label wraps, and every line is
// underlined.
type Link struct {
	core.StatefulBase
	Label string
	// Style sets the text, to match the text around the link. The zero value
	// takes the body text of the theme, and a zero Color the accent ink.
	Style text.Style
	// OnPress runs when the link is pressed. Nil disables the link.
	OnPress func()
}

type linkState struct {
	core.StateOf[Link]
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (Link) CreateState() core.State { return &linkState{} }

// InitState implements core.State.
func (s *linkState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *linkState) Dispose() { s.Control.Dispose() }

// press runs the link of the moment.
func (s *linkState) press() {
	if f := s.Widget().OnPress; f != nil {
		f()
	}
}

// Build implements core.State.
func (s *linkState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	style := w.Style
	if style.Size == 0 {
		style.Size = th.Text.Body.Size
		if style.LineHeight == 0 {
			style.LineHeight = th.Text.Body.LineHeight
		}
	}
	enabled := w.OnPress != nil
	sem := interact.Semantics{Role: semantics.RoleLink, Label: w.Label}
	if !enabled {
		style.Color = c.Ink3
		sem.State = semantics.Disabled
		sem.Child = basic.Text{Content: w.Label, Style: style}
		return sem
	}
	if style.Color == (geom.Color{}) {
		style.Color = c.AccentInk
	}
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	style.Color = theme.Over(style.Color, c.Ink, linkPress*sink)
	// The underline comes with the pointer, and stays under a press that
	// slid off.
	under := max(s.Hot.Get(), sink)
	face := s.Ring(linkRadius, c.AccentInk, linkUnderline{
		color: style.Color.WithAlpha(style.Color.A * under),
		child: basic.Text{Content: w.Label, Style: style},
	})
	sem.OnTap = s.press
	sem.Child = interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			if e.Key != input.KeyEnter {
				return false
			}
			if !e.Repeat {
				s.Sink.Kick(kit.KeyKick)
				s.press()
			}
			return true
		},
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			Cursor:  input.CursorHand,
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   s.Press,
				OnTapCancel: s.Release,
				OnTap: func() {
					s.Release()
					s.press()
				},
				Child: face,
			},
		},
	}
	return sem
}

// linkUnderline is the widget of renderLinkUnderline.
type linkUnderline struct {
	color geom.Color
	child core.Widget
}

// Key implements core.Widget.
func (linkUnderline) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (u linkUnderline) CreateRenderObject() core.RenderObject {
	return &renderLinkUnderline{color: u.color}
}

// UpdateRenderObject implements core.RenderWidget.
func (u linkUnderline) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderLinkUnderline)
	if r.color != u.color {
		r.color = u.color
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (u linkUnderline) ChildWidget() core.Widget { return u.child }

// renderLinkUnderline paints its text and a line under every line of it, a
// little below the baseline, in color.
type renderLinkUnderline struct {
	layout.RenderProxy
	color geom.Color
	// lines is kept across frames, so that painting does not allocate.
	lines []geom.Rect
}

// Paint implements core.RenderObject.
func (r *renderLinkUnderline) Paint(cv *paint.Canvas, off geom.Offset) {
	r.RenderProxy.Paint(cv, off)
	if r.color.A <= 0 || r.ChildCount() == 0 {
		return
	}
	p, ok := r.ChildAt(0).(*basic.RenderParagraph)
	if !ok {
		return
	}
	l := p.TextLayout()
	if l == nil || l.Orientation() != text.Horizontal {
		return
	}
	size := p.Style().Size
	thick := max(1, geom.Floor32(size/14+0.5))
	gap := max(1, geom.Floor32(float32(size*0.12)+0.5))
	r.lines = l.SelectionRects(0, len(p.Text()), r.lines[:0])
	runs := l.Runs()
	at := off.Add(p.Offset())
	for _, ln := range r.lines {
		base := ln.Y + ln.H*0.75
		for i := range runs {
			if y := runs[i].Origin.Y; y >= ln.Y && y <= ln.Y+ln.H {
				base = y
				break
			}
		}
		line := geom.Rect{X: at.X + ln.X, Y: geom.Floor32(at.Y + base + gap), W: ln.W, H: thick}
		cv.DrawRRect(geom.RRect{Rect: line}, paint.Paint{Color: r.color})
	}
}
