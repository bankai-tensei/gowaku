package control

import (
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The timing and measures of an InfoBar.
const (
	infoBarEnter = 240 * time.Millisecond
	infoBarExit  = 160 * time.Millisecond
	// infoBarIcon is the side of the icon of the severity.
	infoBarIcon = 18
	// infoBarClose is the square of the close button, and infoBarCross the
	// side its × is drawn at.
	infoBarClose = 24
	infoBarCross = 16
	// infoBarBorder is the opacity of the color of the severity in the line
	// around the bar.
	infoBarBorder = 0.45
)

// InfoSeverity is how much an InfoBar matters, which picks its icon and color.
//
// It is SeverityInfo (the zero value), SeveritySuccess, SeverityWarning or
// SeverityError.
type InfoSeverity uint8

// The severities.
const (
	// SeverityInfo tells something worth knowing, in the accent color.
	SeverityInfo InfoSeverity = iota
	// SeveritySuccess says something went well, in the Success color.
	SeveritySuccess
	// SeverityWarning asks for attention before something goes wrong, in the
	// Warning color.
	SeverityWarning
	// SeverityError says something went wrong, in the Danger color. Screen
	// readers interrupt what they are saying to announce it.
	SeverityError
)

// String returns the name of s.
func (s InfoSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeveritySuccess:
		return "success"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	}
	return "InfoSeverity(" + strconv.Itoa(int(s)) + ")"
}

// InfoBar is a notice that stays in the page until the app takes it away, such
// as "You are offline" above a form, unlike a Toast, which leaves on its own.
// The parent decides whether it shows: set Open, and clear it in OnClose.
//
//	control.InfoBar{
//		Open: s.offline, Severity: control.SeverityWarning,
//		Title: "オフラインです", Message: "接続が戻ると同期します。",
//		OnClose: func() { s.SetState(func() { s.offline = false }) },
//	}
//
// The bar shows the icon of its Severity, the Title in bold and the Message
// after it, wrapping as the width asks, then at most one action button and a
// close button. It slides open and fades in when Open is set, and closes the
// same way; with ReduceMotion it only fades. A closed bar takes no room.
//
// Screen readers announce the bar once when it opens, politely, or at once for
// SeverityError, without moving the focus. Rebuilding an open bar does not
// announce it again; changing its words does.
//
// The zero value is a closed bar.
type InfoBar struct {
	core.StatefulBase
	// Open shows the bar.
	Open     bool
	Severity InfoSeverity
	Title    string
	Message  string
	// Action is the label of a button on the bar, and OnAction what it does.
	// An empty Action shows no button. The button does not close the bar; do
	// that in OnAction if it should.
	Action   string
	OnAction func()
	// OnClose adds a close button, which runs it. The bar closes once the
	// parent clears Open.
	OnClose func()
}

type infoBarState struct {
	core.StateOf[InfoBar]
	controller motion.AnimationController
	// announce puts the words in the live node a frame after the node
	// appears, since readers announce changes of a live node rather than new
	// ones; announced says the words are there.
	announce  *core.Ticker
	announced bool
	glyphs    infoGlyphs
}

// infoGlyphs are the paths of an InfoBar, made once per State.
type infoGlyphs struct {
	disc, triangle, info, mark, check, cross *paint.Path
}

// CreateState implements core.StatefulWidget.
func (InfoBar) CreateState() core.State { return &infoBarState{} }

// InitState implements core.State.
func (s *infoBarState) InitState() {
	c := &s.controller
	c.Duration, c.ReverseDuration = infoBarEnter, infoBarExit
	c.Curve, c.ReverseCurve = motion.SpringOut, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { kit.Rebuild(&s.StateBase) }
	c.OnStatus = func(motion.Status) { kit.Rebuild(&s.StateBase) }
	s.announce = s.NewTicker(s.tick)
	s.glyphs = newInfoGlyphs()
	// A bar that is open from the start shows at once, and is announced.
	if s.Widget().Open {
		c.SetValue(1)
		s.announce.Start()
	}
}

// DidUpdateWidget implements core.State.
func (s *infoBarState) DidUpdateWidget(old core.Widget) {
	open := s.Widget().Open
	if open == old.(InfoBar).Open {
		return
	}
	if open {
		s.announced = false
		s.announce.Start()
		s.controller.Forward()
		return
	}
	s.announce.Stop()
	s.controller.Reverse()
}

// Dispose implements core.State.
func (s *infoBarState) Dispose() { s.controller.Dispose() }

func (s *infoBarState) tick(time.Duration) {
	s.announce.Stop()
	if !s.announced {
		s.announced = true
		kit.Rebuild(&s.StateBase)
	}
}

// Build implements core.State.
func (s *infoBarState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	if !w.Open && s.controller.Status() == motion.Dismissed {
		return basic.SizedBox{}
	}
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	c := &th.Color
	tint := severityColor(th, w.Severity)

	v := s.controller.Value()
	fade := geom.Clamp32(v, 0, 1)
	grow := fade
	if basic.ReduceMotionOf(ctx) {
		grow = 1
	}

	texts := make([]core.Widget, 0, 2)
	if w.Title != "" {
		texts = append(texts, basic.Text{Content: w.Title, Style: kit.Colored(th.Text.Headline, c.Ink)})
	}
	if w.Message != "" {
		texts = append(texts, basic.Text{Content: w.Message, Style: kit.Colored(th.Text.Body, c.Ink2)})
	}
	row := make([]core.Widget, 0, 4)
	row = append(row,
		basic.Padding{Insets: geom.Insets{Top: 1}, Child: s.icon(th, w.Severity, tint)},
		basic.Expanded{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space2, Children: texts}},
	)
	right := float32(theme.Space12)
	if w.Action != "" {
		var press func()
		if w.OnAction != nil {
			press = func() {
				if f := s.Widget().OnAction; f != nil {
					f()
				}
			}
		}
		row = append(row, Button{Label: w.Action, Size: ButtonSmall, OnPress: press})
	}
	if w.OnClose != nil {
		row = append(row, s.closeButton(th, words.Close))
		right = theme.Space8
	}
	card := basic.Container{
		Color:   c.Card,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: tint.WithAlpha(tint.A * infoBarBorder)},
		Padding: geom.Insets{Left: theme.Space12, Right: right, Top: theme.Space10, Bottom: theme.Space10},
		Child:   basic.Row{Cross: layout.CrossStart, Spacing: theme.Space10, Children: row},
	}

	live := semantics.LivePolite
	if w.Severity == SeverityError {
		live = semantics.LiveAssertive
	}
	label := ""
	if s.announced {
		label = infoBarLabel(words, w)
	}
	// The live node stays outside the fade, which hides what it covers while
	// fully transparent: the node must be there, empty, before the words.
	return interact.Semantics{Role: semantics.RoleGroup, Label: label, Live: live, Child: infoBarSlot{
		grow:  grow,
		child: basic.Opacity{Opacity: fade, Child: card},
	}}
}

// icon returns the icon of severity: a colored shape with a mark cut out of
// it, drawn in the color of the ground so that it reads as a hole.
func (s *infoBarState) icon(th *theme.Theme, severity InfoSeverity, tint geom.Color) core.Widget {
	g := &s.glyphs
	shape, mark := g.disc, g.info
	switch severity {
	case SeveritySuccess:
		mark = g.check
	case SeverityWarning:
		shape, mark = g.triangle, g.mark
	case SeverityError:
		mark = g.mark
	}
	return basic.SizedBox{Width: infoBarIcon, Height: infoBarIcon, Child: basic.Stack{Children: []core.Widget{
		basic.Icon{Path: shape, Size: infoBarIcon, Color: tint},
		basic.Icon{Path: mark, Size: infoBarIcon, Color: th.Color.Card},
	}}}
}

// closeButton returns the × that runs OnClose.
func (s *infoBarState) closeButton(th *theme.Theme, label string) core.Widget {
	c := &th.Color
	return interact.Pressable{
		Label:   label,
		OnPress: func() { s.Widget().OnClose() },
		Build: func(p interact.PressState) core.Widget {
			var ground geom.Color
			ink := c.Ink2
			switch {
			case p.Pressed:
				ground, ink = c.FillStrong, c.Ink
			case p.Hovered:
				ground, ink = c.Fill, c.Ink
			}
			return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
				Width: infoBarClose, Height: infoBarClose, Radius: th.Radius.Control, Color: ground,
				Child: basic.Center{Child: basic.Icon{Path: s.glyphs.cross, Size: infoBarCross, Color: ink}},
			}}
		},
	}
}

// infoBarLabel is what a screen reader says for w: the severity, the title and
// the message.
func infoBarLabel(words *basic.Strings, w InfoBar) string {
	label := words.Information
	switch w.Severity {
	case SeveritySuccess:
		label = words.Success
	case SeverityWarning:
		label = words.Warning
	case SeverityError:
		label = words.Error
	}
	text := w.Title
	if w.Message != "" {
		if text != "" {
			text += " "
		}
		text += w.Message
	}
	if text == "" {
		return label
	}
	return label + ": " + text
}

// severityColor returns the color of severity in th.
func severityColor(th *theme.Theme, severity InfoSeverity) geom.Color {
	switch severity {
	case SeveritySuccess:
		return th.Color.Success
	case SeverityWarning:
		return th.Color.Warning
	case SeverityError:
		return th.Color.Danger
	}
	return th.Color.AccentInk
}

// newInfoGlyphs draws the glyphs of an InfoBar on the 24 unit grid of Icon.
func newInfoGlyphs() infoGlyphs {
	var g infoGlyphs
	g.disc = &paint.Path{}
	infoCircle(g.disc, 12, 12, 10)

	g.triangle = &paint.Path{}
	// A triangle with its corners cut a little, which reads as rounded at 18.
	g.triangle.MoveTo(10.4, 3.2)
	g.triangle.LineTo(13.6, 3.2)
	g.triangle.LineTo(22.6, 19.2)
	g.triangle.LineTo(21, 21.2)
	g.triangle.LineTo(3, 21.2)
	g.triangle.LineTo(1.4, 19.2)
	g.triangle.Close()

	// The i of SeverityInfo: a dot over a stem.
	g.info = &paint.Path{}
	infoCircle(g.info, 12, 7.4, 1.4)
	g.info.AddRRect(geom.RRectAll(geom.Rect{X: 10.8, Y: 10.2, W: 2.4, H: 7.6}, 1.2))

	// The ! of SeverityWarning and SeverityError: a stem over a dot. On the
	// triangle it sits lower, where the triangle is wide, which the same path
	// gives since the triangle's middle is below its center.
	g.mark = &paint.Path{}
	g.mark.AddRRect(geom.RRectAll(geom.Rect{X: 10.8, Y: 6.6, W: 2.4, H: 7.8}, 1.2))
	infoCircle(g.mark, 12, 17, 1.4)

	var check paint.Path
	check.MoveTo(7.4, 12.4)
	check.LineTo(10.6, 15.6)
	check.LineTo(16.8, 8.8)
	g.check = check.Stroke(paint.StrokeStyle{Width: 2.2, Cap: paint.CapRound, Join: paint.JoinRound})

	g.cross = kit.CrossPath(24, 10, 1.75)
	return g
}

// infoCircle adds a circle to p.
func infoCircle(p *paint.Path, cx, cy, r float32) {
	p.AddRRect(geom.RRectAll(geom.Rect{X: cx - r, Y: cy - r, W: 2 * r, H: 2 * r}, r))
}

// infoBarSlot is the widget of renderInfoBarSlot.
type infoBarSlot struct {
	grow  float32
	child core.Widget
}

// Key implements core.Widget.
func (infoBarSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t infoBarSlot) CreateRenderObject() core.RenderObject { return &renderInfoBarSlot{grow: t.grow} }

// UpdateRenderObject implements core.RenderWidget.
func (t infoBarSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderInfoBarSlot)
	if r.grow != t.grow {
		r.grow = t.grow
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t infoBarSlot) ChildWidget() core.Widget { return t.child }

// renderInfoBarSlot takes grow times the height of its child, which hangs from
// its top and is cut off below: the bar opens downward and pushes what follows
// it, and closing lets it back up.
type renderInfoBarSlot struct {
	layout.RenderProxy
	grow float32
}

// Layout implements core.RenderObject.
func (r *renderInfoBarSlot) Layout(c layout.Constraints) geom.Size {
	child := r.ChildAt(0)
	if child == nil {
		return c.Constrain(geom.Size{})
	}
	csz := core.LayoutChild(child, geom.Constraints{MinW: c.MinW, MaxW: c.MaxW, MaxH: geom.Inf})
	return c.Constrain(geom.Size{W: csz.W, H: geom.Floor32(csz.H*r.grow + 0.5)})
}

// ChildClip implements core.Clipper.
func (r *renderInfoBarSlot) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), r.grow < 1
}

// Paint implements core.RenderObject.
func (r *renderInfoBarSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	child := r.ChildAt(0)
	if child == nil {
		return
	}
	if r.grow >= 1 {
		child.Paint(cv, off.Add(child.Offset()))
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	child.Paint(cv, off.Add(child.Offset()))
	cv.Restore()
}

// HitTest implements core.RenderObject.
func (r *renderInfoBarSlot) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	return r.RenderProxy.HitTest(pos, res)
}
