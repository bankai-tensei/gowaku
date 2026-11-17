package basic

import (
	"slices"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
)

// defaultDuration is the length of an implicit animation that names none.
const defaultDuration = 200 * time.Millisecond

// durationOf returns the length of an implicit animation.
func durationOf(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return defaultDuration
}

// curveOf returns the curve of an implicit animation.
func curveOf(c motion.Curve) motion.Curve {
	if c != nil {
		return c
	}
	return motion.Standard
}

// AnimatedContainer is a Container that moves to new values over time instead of
// jumping. It animates the color, padding, size, corners, border and shadows.
// A Width or Height of zero, which means "as the child wants", does not animate.
type AnimatedContainer struct {
	core.StatefulBase
	// Duration is the length of the motion. Zero selects 200 milliseconds.
	Duration time.Duration
	// Curve shapes the motion. Nil selects motion.Standard.
	Curve         motion.Curve
	Width, Height float32
	Padding       layout.Insets
	Color         geom.Color
	Radius        float32
	Border        paint.Stroke
	Shadows       []paint.BoxShadow
	Child         core.Widget
}

type animatedContainerState struct {
	core.StateBase
	motion  motion.Implicit
	size    *motion.Value[geom.Size]
	padding *motion.Value[geom.Insets]
	color   *motion.Value[geom.Color]
	radius  *motion.Value[float32]
	border  *motion.Value[paint.Stroke]
	shadows *motion.Value[[]paint.BoxShadow]
}

// CreateState implements core.StatefulWidget.
func (AnimatedContainer) CreateState() core.State { return &animatedContainerState{} }

// InitState implements core.State.
func (s *animatedContainerState) InitState() {
	w := s.Widget().(AnimatedContainer)
	s.motion.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.size = motion.Track(&s.motion, geom.Size{W: w.Width, H: w.Height}, motion.LerpSize)
	s.padding = motion.Track(&s.motion, w.Padding, motion.LerpInsets)
	s.color = motion.Track(&s.motion, w.Color, motion.LerpColor)
	s.radius = motion.Track(&s.motion, w.Radius, motion.LerpFloat)
	s.border = motion.Track(&s.motion, w.Border, lerpStroke)
	s.shadows = motion.TrackFunc(&s.motion, w.Shadows, layout.LerpShadows, slices.Equal[[]paint.BoxShadow])
}

func lerpStroke(a, b paint.Stroke, t float32) paint.Stroke {
	return paint.Stroke{Width: motion.LerpFloat(a.Width, b.Width, t), Color: motion.LerpColor(a.Color, b.Color, t)}
}

// DidUpdateWidget implements core.State.
func (s *animatedContainerState) DidUpdateWidget(old core.Widget) {
	w := s.Widget().(AnimatedContainer)
	o := old.(AnimatedContainer)
	moves := w.Width != o.Width || w.Height != o.Height || w.Padding != o.Padding || w.Radius != o.Radius
	s.size.Set(geom.Size{W: w.Width, H: w.Height})
	s.padding.Set(w.Padding)
	s.color.Set(w.Color)
	s.radius.Set(w.Radius)
	s.border.Set(w.Border)
	s.shadows.Set(w.Shadows)
	s.motion.Commit(motionDuration(&s.StateBase, moves, w.Duration), curveOf(w.Curve))
}

// Dispose implements core.State.
func (s *animatedContainerState) Dispose() { s.motion.Dispose() }

// Build implements core.State.
func (s *animatedContainerState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(AnimatedContainer)
	size := s.size.Get()
	c := Container{
		Width:   size.W,
		Height:  size.H,
		Padding: s.padding.Get(),
		Color:   s.color.Get(),
		Radius:  s.radius.Get(),
		Shadows: s.shadows.Get(),
		Border:  s.border.Get(),
		Child:   w.Child,
	}
	return c
}

// AnimatedOpacity is an Opacity that moves to a new value over time. While its
// value is 0 the child is out of hit testing, semantics and focus traversal,
// as with Opacity; during a fade it is present.
type AnimatedOpacity struct {
	core.StatefulBase
	// Duration is the length of the motion. Zero selects 200 milliseconds.
	Duration time.Duration
	// Curve shapes the motion. Nil selects motion.Standard.
	Curve   motion.Curve
	Opacity float32
	Child   core.Widget
}

type animatedOpacityState struct {
	core.StateOf[AnimatedOpacity]
	motion  motion.Implicit
	opacity *motion.Value[float32]
}

// CreateState implements core.StatefulWidget.
func (AnimatedOpacity) CreateState() core.State { return &animatedOpacityState{} }

// InitState implements core.State.
func (s *animatedOpacityState) InitState() {
	s.motion.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.opacity = motion.Track(&s.motion, s.Widget().Opacity, motion.LerpFloat)
}

// DidUpdateWidget implements core.State.
func (s *animatedOpacityState) DidUpdateWidget(core.Widget) {
	w := s.Widget()
	s.opacity.Set(w.Opacity)
	s.motion.Commit(durationOf(w.Duration), curveOf(w.Curve))
}

// Dispose implements core.State.
func (s *animatedOpacityState) Dispose() { s.motion.Dispose() }

// Build implements core.State.
func (s *animatedOpacityState) Build(core.BuildContext) core.Widget {
	return Opacity{Opacity: s.opacity.Get(), Child: s.Widget().Child}
}

// AnimatedAlign is an Align that moves to a new alignment over time.
type AnimatedAlign struct {
	core.StatefulBase
	// Duration is the length of the motion. Zero selects 200 milliseconds.
	Duration time.Duration
	// Curve shapes the motion. Nil selects motion.Standard.
	Curve motion.Curve
	// Alignment is the target, from 0 to 1 on each axis (not -1..1 as in
	// Flutter). An overshooting Curve carries the child a little past it.
	Alignment layout.Alignment
	Child     core.Widget
}

type animatedAlignState struct {
	core.StateOf[AnimatedAlign]
	motion    motion.Implicit
	alignment *motion.Value[layout.Alignment]
}

// CreateState implements core.StatefulWidget.
func (AnimatedAlign) CreateState() core.State { return &animatedAlignState{} }

func lerpAlignment(a, b layout.Alignment, t float32) layout.Alignment {
	return layout.Alignment{X: motion.LerpFloat(a.X, b.X, t), Y: motion.LerpFloat(a.Y, b.Y, t)}
}

// InitState implements core.State.
func (s *animatedAlignState) InitState() {
	s.motion.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.alignment = motion.Track(&s.motion, s.Widget().Alignment, lerpAlignment)
}

// DidUpdateWidget implements core.State.
func (s *animatedAlignState) DidUpdateWidget(core.Widget) {
	w := s.Widget()
	s.alignment.Set(w.Alignment)
	s.motion.Commit(motionDuration(&s.StateBase, true, w.Duration), curveOf(w.Curve))
}

// Dispose implements core.State.
func (s *animatedAlignState) Dispose() { s.motion.Dispose() }

// Build implements core.State.
func (s *animatedAlignState) Build(core.BuildContext) core.Widget {
	return Align{Alignment: s.alignment.Get(), Child: s.Widget().Child}
}

// AnimatedPositioned is a Positioned that moves to a new place over time. Like
// Positioned, it must be a child of a Stack. The anchors do not animate.
type AnimatedPositioned struct {
	core.StatefulBase
	// Duration is the length of the motion. Zero selects 200 milliseconds.
	Duration time.Duration
	// Curve shapes the motion. Nil selects motion.Standard.
	Curve                     motion.Curve
	Left, Top, Right, Bottom  float32
	AnchorRight, AnchorBottom bool
	Width, Height             float32
	Child                     core.Widget
}

type animatedPositionedState struct {
	core.StateBase
	motion motion.Implicit
	// edges holds left, top, width and height, and far right and bottom.
	edges *motion.Value[geom.Rect]
	far   *motion.Value[geom.Offset]
}

// CreateState implements core.StatefulWidget.
func (AnimatedPositioned) CreateState() core.State { return &animatedPositionedState{} }

func (p AnimatedPositioned) box() geom.Rect {
	return geom.Rect{X: p.Left, Y: p.Top, W: p.Width, H: p.Height}
}

// InitState implements core.State.
func (s *animatedPositionedState) InitState() {
	w := s.Widget().(AnimatedPositioned)
	s.motion.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.edges = motion.Track(&s.motion, w.box(), motion.LerpRect)
	s.far = motion.Track(&s.motion, geom.Offset{X: w.Right, Y: w.Bottom}, motion.LerpOffset)
}

// DidUpdateWidget implements core.State.
func (s *animatedPositionedState) DidUpdateWidget(core.Widget) {
	w := s.Widget().(AnimatedPositioned)
	s.edges.Set(w.box())
	s.far.Set(geom.Offset{X: w.Right, Y: w.Bottom})
	s.motion.Commit(motionDuration(&s.StateBase, true, w.Duration), curveOf(w.Curve))
}

// Dispose implements core.State.
func (s *animatedPositionedState) Dispose() { s.motion.Dispose() }

// Build implements core.State.
func (s *animatedPositionedState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(AnimatedPositioned)
	box, far := s.edges.Get(), s.far.Get()
	return Positioned{
		Left: box.X, Top: box.Y, Right: far.X, Bottom: far.Y,
		AnchorRight: w.AnchorRight, AnchorBottom: w.AnchorBottom,
		Width: box.W, Height: box.H,
		Child: w.Child,
	}
}

// AnimatedSwitcher moves from its old child to a new one: a crossfade, or the
// motion of Transition, such as a page sliding in. A child counts as new when
// it has another type or Key, so give children of the same type a Key with
// core.Keyed:
//
//	AnimatedSwitcher{
//		Duration:   250 * time.Millisecond,
//		Transition: SlideTransition(0.04*float32(dir), 0),
//		Child:      core.Keyed(page, body),
//	}
type AnimatedSwitcher struct {
	core.StatefulBase
	// Duration is the length of the motion. Zero selects 200 milliseconds.
	Duration time.Duration
	// Curve shapes the motion. Nil selects motion.Standard.
	Curve motion.Curve
	// Alignment places the children, which are on top of each other for a while.
	Alignment layout.Alignment
	// Transition wraps each child for where it is in the motion. Nil selects
	// FadeTransition. The children use the Transition of the newest widget, so
	// one chosen at the switch, such as a slide toward the next tab, moves the
	// old child and the new one together.
	Transition SwitcherTransition
	Child      core.Widget
}

// SwitcherTransition wraps a child of an AnimatedSwitcher for one frame of its
// motion. t runs from 0 to 1 while the child comes in and from 1 to 0 while it
// goes out, with the Curve applied; entering tells which. It must return the
// same widget types for every t, or the child loses its State mid-motion.
type SwitcherTransition func(child core.Widget, t float32, entering bool) core.Widget

// FadeTransition crossfades: the default of AnimatedSwitcher.
func FadeTransition(child core.Widget, t float32, _ bool) core.Widget {
	return Opacity{Opacity: t, Child: child}
}

// SlideTransition fades a child in while it slides from dx, dy to its place,
// and fades the old one out while it slides the other way, toward -dx, -dy.
// The distances are fractions of the child's size: 1 is a whole width, and
// 0.03 to 0.06 gives the quiet shift of a settings page. Pass a negative dx
// to go back, so that the motion follows the order of the pages. The children
// are clipped to the switcher's box while they move, and not at rest, where a
// focus ring may reach past the box.
func SlideTransition(dx, dy float32) SwitcherTransition {
	return func(child core.Widget, t float32, entering bool) core.Widget {
		away := 1 - t
		if !entering {
			away = -away
		}
		return Opacity{Opacity: t, Child: Translate{X: dx * away, Y: dy * away, Clip: away != 0, Child: child}}
	}
}

// ScaleTransition fades a child in while it grows from from to its size, and
// fades the old one out while it grows past its size by as much, the way a
// window opens. 0.96 is gentle.
func ScaleTransition(from float32) SwitcherTransition {
	return func(child core.Widget, t float32, entering bool) core.Widget {
		scale := from + (1-from)*t
		if !entering {
			scale = 1 + (1-from)*(1-t)
		}
		return Opacity{Opacity: t, Child: Scale{Scale: scale, Origin: layout.AlignCenter, Child: child}}
	}
}

// switcherEntry is a child on its way in, in place, or on its way out.
type switcherEntry struct {
	id         int
	widget     core.Widget
	controller motion.AnimationController
}

type animatedSwitcherState struct {
	core.StateBase
	entries []*switcherEntry
	nextID  int
}

// CreateState implements core.StatefulWidget.
func (AnimatedSwitcher) CreateState() core.State { return &animatedSwitcherState{} }

// InitState implements core.State.
func (s *animatedSwitcherState) InitState() {
	s.add(s.Widget().(AnimatedSwitcher)).controller.SetValue(1)
}

func (s *animatedSwitcherState) add(w AnimatedSwitcher) *switcherEntry {
	e := &switcherEntry{id: s.nextID, widget: w.Child}
	s.nextID++
	e.controller.Duration, e.controller.Curve = durationOf(w.Duration), curveOf(w.Curve)
	e.controller.Attach(&s.StateBase)
	e.controller.OnChange = func() { kit.Rebuild(&s.StateBase) }
	e.controller.OnStatus = func(status motion.Status) {
		if status == motion.Dismissed {
			s.remove(e)
		}
	}
	s.entries = append(s.entries, e)
	return e
}

func (s *animatedSwitcherState) remove(e *switcherEntry) {
	if i := slices.Index(s.entries, e); i >= 0 {
		s.entries = slices.Delete(s.entries, i, i+1)
		kit.Rebuild(&s.StateBase)
	}
}

// DidUpdateWidget implements core.State.
func (s *animatedSwitcherState) DidUpdateWidget(core.Widget) {
	w := s.Widget().(AnimatedSwitcher)
	current := s.entries[len(s.entries)-1]
	if core.SameWidget(current.widget, w.Child) {
		current.widget = w.Child
		return
	}
	current.controller.Reverse()
	s.add(w).controller.Forward()
}

// Dispose implements core.State.
func (s *animatedSwitcherState) Dispose() {
	for _, e := range s.entries {
		e.controller.Dispose()
	}
}

// Build implements core.State.
func (s *animatedSwitcherState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(AnimatedSwitcher)
	transition := w.Transition
	if transition == nil || reduceMotion(&s.StateBase) {
		// Less motion keeps the fade and drops the slide or the growth.
		transition = FadeTransition
	}
	children := make([]core.Widget, len(s.entries))
	last := len(s.entries) - 1
	for i, e := range s.entries {
		children[i] = core.Keyed(e.id, transition(RepaintBoundary{Child: e.widget}, e.controller.Value(), i == last))
	}
	return Stack{Alignment: w.Alignment, Children: children}
}

// motionDuration is the duration of an implicit animation of a State: d, or its
// default, unless the animation moves or resizes something in a window that
// asks for less motion, which makes it jump (ADR 0042).
func motionDuration(s *core.StateBase, moves bool, d time.Duration) time.Duration {
	if moves && reduceMotion(s) {
		return 0
	}
	return durationOf(d)
}
