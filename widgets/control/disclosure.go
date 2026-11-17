package control

import (
	"math"
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
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// disclosureChevron is the size of the chevron before the title, and
	// disclosureGap the gap between them.
	disclosureChevron = 12
	disclosureGap     = theme.Space6
	// disclosureInset is the room at the ends of the header.
	disclosureInset = theme.Space8
	// disclosureIndent puts the content under the title, past the chevron.
	disclosureIndent = disclosureInset + disclosureChevron + disclosureGap
	// disclosureDuration is the length of the opening and closing.
	disclosureDuration = 200 * time.Millisecond
)

// Disclosure is a header that opens and closes the content below it when
// pressed, for the sections of settings, properties and questions that are
// answered on demand:
//
//	Disclosure{Title: "Advanced", Child: advanced}
//
// The header is a chevron and Title, as tall as a control, and takes the whole
// width it is given; inside a horizontal Scroll it panics unless a SizedBox
// gives it one. The content shows under the title, indented past the chevron,
// and slides open and shut; under MediaQuery.ReduceMotion it jumps. The
// content is laid out at the height it wants, so it must not need a bounded
// height, as an Expanded does.
//
// A Disclosure keeps its own state unless OnChange is set, which is all a
// single section needs. With OnChange the parent keeps the state, in Open, and
// the disclosure only asks for changes, as a Switch does. That is how several
// in a row show one open at a time, an accordion:
//
//	for i, sec := range sections {
//		out[i] = Disclosure{Title: sec.Title, Child: sec.Body, Open: s.open == i,
//			OnChange: func(open bool) {
//				s.SetState(func() { s.open = -1; if open { s.open = i } })
//			}}
//	}
//
// See docs/ADR/0087-disclosure-link.md.
//
// The header is a stop of Tab with a focus ring, and Enter or Space opens or
// closes it. A screen reader finds a button named Title that is expanded or
// collapsed, with the actions to expand and collapse it. The closed content
// keeps its state, but the pointer, Tab and assistive technology pass it by,
// and it is not laid out.
type Disclosure struct {
	core.StatefulBase
	// Title names the section, in the header and to assistive technology.
	Title string
	// Header replaces the text of Title in the header, for a title with a
	// count or a badge beside it. Title still names the header to assistive
	// technology.
	Header core.Widget
	// Open says whether the content shows. With OnChange it is the state,
	// which the disclosure shows and never changes itself. Without, the
	// disclosure starts in it, keeps its own state from then on, and follows
	// the parent when the parent changes Open.
	Open bool
	// OnChange makes the parent keep the state. It receives the state asked
	// for.
	OnChange func(open bool)
	Child    core.Widget
}

type disclosureState struct {
	core.StateOf[Disclosure]
	behavior.Control
	// open is the state of a disclosure without OnChange.
	open bool
	// reveal moves shown, which is 0 closed and 1 open.
	reveal  motion.Implicit
	shown   *motion.Value[float32]
	chevron *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Disclosure) CreateState() core.State { return &disclosureState{} }

// InitState implements core.State.
func (s *disclosureState) InitState() {
	s.Attach(&s.StateBase)
	s.open = s.Widget().Open
	s.reveal.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.shown = motion.Track(&s.reveal, kit.OnOff(s.isOpen()), motion.LerpFloat)
	s.chevron = kit.ChevronPath(true)
}

// DidUpdateWidget implements core.State.
func (s *disclosureState) DidUpdateWidget(old core.Widget) {
	was, w := old.(Disclosure), s.Widget()
	if w.OnChange == nil && (was.Open != w.Open || was.OnChange != nil) {
		// The parent changed the state it starts in, or gave the state back:
		// either way Open is the state now.
		s.open = w.Open
	}
	s.retarget()
}

// Dispose implements core.State.
func (s *disclosureState) Dispose() {
	s.Control.Dispose()
	s.reveal.Dispose()
}

// isOpen reports whether the content shows, or is opening.
func (s *disclosureState) isOpen() bool {
	if w := s.Widget(); w.OnChange != nil {
		return w.Open
	}
	return s.open
}

// retarget moves the content toward the state.
func (s *disclosureState) retarget() {
	s.shown.Set(kit.OnOff(s.isOpen()))
	d := disclosureDuration
	if basic.ReduceMotionOf(s.Context()) {
		d = 0
	}
	s.reveal.Commit(d, motion.Standard)
}

// setOpen asks for the state open.
func (s *disclosureState) setOpen(open bool) {
	if open == s.isOpen() {
		return
	}
	if f := s.Widget().OnChange; f != nil {
		f(open)
		return
	}
	s.SetState(func() { s.open = open })
	s.retarget()
}

func (s *disclosureState) toggle() { s.setOpen(!s.isOpen()) }

// Build implements core.State.
func (s *disclosureState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	shown := geom.Clamp32(s.shown.Get(), 0, 1)

	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	fill := c.Fill.WithAlpha(c.Fill.A * s.Hot.Get())
	fill = theme.Over(fill, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	var title core.Widget = basic.Text{Content: w.Title, Style: kit.Colored(th.Text.Headline, c.Ink)}
	if w.Header != nil {
		title = w.Header
	}
	// The chevron points along the line closed and down open.
	chevron := basic.Rotate(float32(math.Pi/2)*shown, basic.Icon{Path: s.chevron, Size: disclosureChevron, Color: c.Ink2})
	face := s.Ring(th.Radius.Control, c.AccentInk, basic.Container{
		Height: theme.ControlHeight, Color: fill, Radius: th.Radius.Control,
		Padding: geom.Symmetric(disclosureInset, 0),
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: disclosureGap, Children: []core.Widget{
			chevron, basic.Expanded{Child: title},
		}},
	})
	state := semantics.Collapsed
	if s.isOpen() {
		state = semantics.Expanded
	}
	sem := interact.Semantics{
		Role: semantics.RoleButton, Label: w.Title, State: state,
		OnExpand:   func() { s.setOpen(true) },
		OnCollapse: func() { s.setOpen(false) },
	}
	header := s.Wrap(sem, true, s.toggle, face)
	body := disclosureBody{factor: shown, child: basic.Padding{
		Insets: geom.Insets{Left: disclosureIndent, Top: theme.Space4, Bottom: theme.Space8},
		Child:  w.Child,
	}}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{header, body}}
}

// disclosureBody is the widget of renderDisclosureBody.
type disclosureBody struct {
	factor float32
	child  core.Widget
}

// Key implements core.Widget.
func (disclosureBody) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b disclosureBody) CreateRenderObject() core.RenderObject {
	return &renderDisclosureBody{factor: b.factor}
}

// UpdateRenderObject implements core.RenderWidget.
func (b disclosureBody) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDisclosureBody)
	if r.factor != b.factor {
		r.factor = b.factor
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (b disclosureBody) ChildWidget() core.Widget { return b.child }

// renderDisclosureBody shows factor of the height of its child, from the top,
// as the content of a Disclosure slides open. Closed, it is hidden and does not
// lay its child out, so that the content of closed sections costs nothing but
// its build.
type renderDisclosureBody struct {
	layout.RenderProxy
	factor float32
}

// Hidden implements core.Hider.
func (r *renderDisclosureBody) Hidden() bool { return r.factor <= 0 }

// Layout implements core.RenderObject.
func (r *renderDisclosureBody) Layout(c geom.Constraints) geom.Size {
	if r.factor <= 0 || r.ChildCount() == 0 {
		return c.Constrain(geom.Size{W: kit.FiniteOr(c.MaxW, 0)})
	}
	sz := core.LayoutChild(r.ChildAt(0), geom.Constraints{MinW: c.MinW, MaxW: c.MaxW, MaxH: geom.Inf})
	h := sz.H
	if r.factor < 1 {
		// Whole pixels keep what follows the disclosure from shimmering as it
		// moves. The product is rounded on its own so that arm64 does not fuse
		// it into the add (ADR 0086).
		h = geom.Floor32(float32(sz.H*r.factor) + 0.5)
	}
	return c.Constrain(geom.Size{W: sz.W, H: h})
}

// ChildClip implements core.Clipper.
func (r *renderDisclosureBody) ChildClip() (geom.Rect, bool) {
	// Folded shut, nothing of the body shows.
	if r.factor <= 0 {
		return geom.Rect{}, true
	}
	return geom.RectFromSize(r.Size()), r.factor < 1
}

// Paint implements core.RenderObject.
func (r *renderDisclosureBody) Paint(cv *paint.Canvas, off geom.Offset) {
	switch {
	case r.factor <= 0:
	case r.factor >= 1:
		r.RenderProxy.Paint(cv, off)
	default:
		sz := r.Size()
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
		r.RenderProxy.Paint(cv, off)
		cv.Restore()
	}
}
