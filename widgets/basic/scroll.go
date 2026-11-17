package basic

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// overscroll is how far the wheel can pull the content past its end.
	overscroll = 60
	// scrollStiffness sets the speed of the spring that brings content pulled
	// past an end back to it.
	scrollStiffness = 300
	// glideTime is the time constant of a wheel motion: the content covers 63%
	// of the way in that time, 95% in three times that, and then rests.
	glideTime = 40 * time.Millisecond
	// glideLead is how long a wheel motion has run by its first frame. The
	// wheel turns between frames, so the first frame already moves.
	glideLead = time.Second / 60
)

// Scroller is the physics of a scrolling view. The wheel moves a target, and
// the content glides to it fastest at first and slower as it arrives, the way
// browsers scroll: it answers on the next frame, and each notch adds to the
// target from where the motion is. A target past the end pulls the content
// out, and then a spring returns it to the end.
//
// A spring for the wheel itself was tried first and felt late: starting from
// rest, it covered 3% of a notch in the first frame and half in 100 ms.
type Scroller struct {
	position *layout.ScrollOffset
	// limit is the largest offset.
	limit float64

	// gliding is a wheel motion from from to to; otherwise a moving scroller
	// runs spring.
	gliding  bool
	from, to float64
	spring   motion.SpringSimulation
	// started is when the motion began. A new one takes it from the first
	// tick, because the wheel turns between frames and has no clock.
	started  time.Duration
	fresh    bool
	moving   bool
	velocity float64
}

// wheel moves the target by distance, from the current motion.
func (s *Scroller) wheel(distance float32) {
	target := float64(distance)
	switch {
	case s.moving && s.gliding:
		target += s.to
	case s.moving:
		target += s.spring.Target()
	default:
		target += s.position.Value()
	}
	lo, hi := -float64(overscroll), s.limit+overscroll
	s.glide(min(max(target, lo), hi))
}

// nudge moves the view by distance at once, for a touchpad or a smooth wheel,
// whose stream of small steps is already a motion: animating each step would
// trail behind the fingers and then rush to catch up. A glide under way ends
// at its target first, so that a wheel notch is not lost.
func (s *Scroller) nudge(distance float32) {
	base := s.position.Value()
	if s.moving && s.gliding {
		base = s.to
	}
	s.moving = false
	s.velocity = 0
	s.position.Set(s.clamped(base + float64(distance)))
}

// glide starts a wheel motion toward target from where the view is now.
func (s *Scroller) glide(target float64) {
	s.from, s.to = s.position.Value(), target
	s.gliding = true
	s.fresh = true
	s.moving = true
}

// aim starts the spring toward target from where the view is now.
func (s *Scroller) aim(target float64) {
	s.spring = motion.NewSpringSimulation(motion.CriticalSpring(scrollStiffness),
		s.position.Value(), s.velocity, target)
	s.gliding = false
	s.fresh = true
	s.moving = true
}

// Jump goes to offset at once, ending any motion.
func (s *Scroller) Jump(offset float64) {
	s.moving = false
	s.velocity = 0
	s.position.Set(offset)
}

// Shift follows a move of the content by d under a motion under way, as when
// rows above the view take their measured heights, so that the motion goes on
// from where the view now is and ends as far past its old target.
func (s *Scroller) Shift(d float64) {
	if !s.moving {
		return
	}
	if s.gliding {
		s.from += d
		s.to += d
		return
	}
	s.aim(s.spring.Target() + d)
}

// ShiftGlide moves a wheel motion under way by d, as when the content under
// it moved because estimates of the lines above the view became exact. Unlike
// Shift, it leaves a spring alone.
func (s *Scroller) ShiftGlide(d float64) {
	if s.moving && s.gliding {
		s.from += d
		s.to += d
	}
}

// tick moves the view to its place at now, and reports whether it keeps moving.
func (s *Scroller) tick(now time.Duration) bool {
	if !s.moving {
		return false
	}
	if s.fresh {
		s.fresh = false
		s.started = now
		if s.gliding {
			s.started -= glideLead
		}
	}
	elapsed := now - s.started
	if s.gliding {
		return s.glideAt(elapsed)
	}
	p, v := s.spring.At(elapsed)
	s.velocity = v
	s.position.Set(p)
	if !s.spring.Settled(elapsed) {
		// The pull past an end lasts only until the content gets there.
		if t := s.spring.Target(); s.pastEnd(t) && s.pastEnd(p) && absNear(p, t) {
			s.aim(s.clamped(t))
		}
		return true
	}
	target := s.spring.Target()
	if s.pastEnd(target) {
		s.aim(s.clamped(target))
		return true
	}
	s.position.Set(target)
	s.velocity = 0
	s.moving = false
	return false
}

// glideAt moves a wheel motion to its place after elapsed.
func (s *Scroller) glideAt(elapsed time.Duration) bool {
	tau := glideTime.Seconds()
	left := (s.from - s.to) * math.Exp(-elapsed.Seconds()/tau)
	p := s.to + left
	s.velocity = -left / tau
	if left > 0.25 || left < -0.25 {
		s.position.Set(p)
		// The pull past an end lasts only until the content gets there.
		if s.pastEnd(s.to) && s.pastEnd(p) && absNear(p, s.to) {
			s.aim(s.clamped(s.to))
		}
		return true
	}
	s.position.Set(s.to)
	if s.pastEnd(s.to) {
		s.aim(s.clamped(s.to))
		return true
	}
	s.velocity = 0
	s.moving = false
	return false
}

func absNear(a, b float64) bool {
	d := a - b
	return d > -overscroll/3 && d < overscroll/3
}

func (s *Scroller) pastEnd(v float64) bool { return v < 0 || v > s.limit }

func (s *Scroller) clamped(v float64) float64 { return min(max(v, 0), s.limit) }

// SetLimit sets the largest offset, following a change of size, keeping the content in place if it can.
func (s *Scroller) SetLimit(limit float64) {
	s.limit = max(limit, 0)
	if !s.moving && s.position.Value() > s.limit {
		s.position.Set(s.limit)
	}
}

// ScrollState is the State that Scroll, List, Tree, Table, PopupList and the
// editor share: the offset of a scrolling view, the physics that move it and
// what the scroll bar shows. A scrolling widget embeds it in its State, shows
// its content in a Viewport or lays it out itself, and wraps it in a
// ScrollFrame for the bar.
type ScrollState struct {
	core.StateBase
	// Offset is the position of the view, and Scroller the physics that move it.
	Offset   layout.ScrollOffset
	Scroller Scroller
	// Ticker runs the motion. InitState creates it.
	Ticker *core.Ticker
	// OnMove runs after each step of the motion.
	OnMove func()
	// OnUser runs when the user scrolls, by the wheel or the scroll bar.
	OnUser func()
	// OnScroll, when not nil, runs with the position after it changed: on
	// each step of a motion, after a jump, after a layout that changed the
	// largest offset, and after the first layout.
	OnScroll func(ScrollPosition)
	// StickToEnd keeps the view at the end of the scroll range while it rests
	// there: Measure moves it to the new end when the content grows, until
	// the user scrolls away, and again from when the view comes to rest at
	// the end.
	StickToEnd bool
	// Metrics feeds the scroll bar, and bar is its state once it exists.
	Metrics layout.ScrollMetrics
	bar     *scrollBarState
	// told is the position OnScroll was last given, once known is set.
	told  ScrollPosition
	known bool
	// stuck is set while the view rests at the end and the user has not
	// scrolled away from it.
	stuck bool
	// replay is a jump of code to replayAt made again after the next layout,
	// which takes the content of a rebuild that may not have run yet, and
	// view is the render object to lay out for it.
	replay   bool
	replayAt float64
	view     core.RenderObject
}

// InitState implements core.State.
func (s *ScrollState) InitState() {
	s.Scroller.position = &s.Offset
	// Content not laid out yet is at its end: a log that starts empty sticks.
	s.stuck = true
	s.Ticker = s.NewTicker(func(now time.Duration) {
		if !s.Scroller.tick(now) {
			s.Ticker.Stop()
		}
		s.Moved()
	})
}

// Moved tells those who follow the offset: OnMove, the scroll bar and
// OnScroll.
func (s *ScrollState) Moved() {
	s.rest()
	if s.OnMove != nil {
		s.OnMove()
	}
	if s.bar != nil {
		s.bar.activity()
	}
	s.report()
}

// Position returns where the view is, as of the last layout for MaxOffset.
func (s *ScrollState) Position() ScrollPosition {
	return ScrollPosition{Offset: s.Offset.Value(), MaxOffset: s.Scroller.limit}
}

// StuckToEnd reports whether the view rests at the end of the scroll range
// and the user has not scrolled away since: what StickToEnd holds to.
func (s *ScrollState) StuckToEnd() bool { return s.stuck }

// Measure takes the length of the view and of the content after a layout. It
// sets Metrics and the largest offset, makes again a jump of code that the
// layout may have changed, keeps the view at the end with StickToEnd, and
// tells OnScroll.
func (s *ScrollState) Measure(view, content float64) {
	s.Metrics = layout.ScrollMetrics{View: view, Content: content}
	s.Scroller.SetLimit(content - view)
	limit := s.Scroller.limit
	switch {
	case s.replay:
		s.replay = false
		s.jumpTo(min(s.replayAt, limit))
	case s.StickToEnd && s.stuck && !s.Scroller.moving && s.Offset.Value() != limit:
		s.jumpTo(limit)
	default:
		s.rest()
		s.report()
	}
}

// rest notes whether a view at rest is at the end. A motion leaves it as the
// user left it.
func (s *ScrollState) rest() {
	if !s.Scroller.moving {
		s.stuck = s.Position().AtEnd()
	}
}

// report tells OnScroll a position it has not been told. It allocates nothing.
func (s *ScrollState) report() {
	if s.OnScroll == nil {
		return
	}
	if p := s.Position(); !s.known || p != s.told {
		s.told, s.known = p, true
		s.OnScroll(p)
	}
}

// Jump moves the view at once, as dragging the scroll bar does.
func (s *ScrollState) Jump(offset float64) {
	s.user()
	s.Scroller.Jump(offset)
	s.Ticker.Stop()
	s.Moved()
}

// ShowBar shows the scroll bar and starts its stay over, as a move of the view
// does, without telling OnMove.
func (s *ScrollState) ShowBar() {
	if s.bar != nil {
		s.bar.activity()
	}
}

// jumpTo moves the view at once for code, not the user: it does not run
// OnUser.
func (s *ScrollState) jumpTo(offset float64) {
	s.Scroller.Jump(offset)
	s.Ticker.Stop()
	s.Moved()
}

func (s *ScrollState) user() {
	s.stuck, s.replay = false, false
	if s.OnUser != nil {
		s.OnUser()
	}
}

// Dispose implements core.State.
func (s *ScrollState) Dispose() { s.Ticker.Stop() }

// Wheel scrolls by notches of the wheel. Whole notches come from a wheel that
// clicks, and glide. A fraction of a notch comes from a touchpad or a wheel
// that turns freely, which send many small steps; those move at once, as in
// browsers.
func (s *ScrollState) Wheel(notches float32) {
	s.user()
	if notches != float32(math.Round(float64(notches))) {
		s.Scroller.nudge(notches * kit.NotchDistance)
		s.Ticker.Stop()
		s.Moved()
		return
	}
	s.Scroller.wheel(notches * kit.NotchDistance)
	s.Ticker.Start()
}

// Scroll shows a child taller than itself, moved with the wheel or the scroll bar
// that appears over its edge. Horizontal scrolls sideways instead.
//
// Reverse starts the scroll range at the other end: at the right for a
// horizontal Scroll, at the bottom for a vertical one. The view opens there,
// a child shorter than the view sits against that end, and when the view or
// the child changes size, that end stays where it is. This is the Scroll for
// vertical Japanese text, whose columns run from right to left; the Row holds
// the paragraphs last first, so that the first is on the right:
//
//	basic.Scroll{Horizontal: true, Reverse: true, Child: basic.Row{
//		Cross: layout.CrossStart, Children: lastFirst,
//	}}
//
// It also suits a log or a chat whose newest line is at the bottom; a short
// log then sits at the bottom too, so show an empty state outside the Scroll
// or leave Reverse off until the lines fill the view. The wheel and the scroll
// bar move the content the same way as without Reverse.
//
// StickToEnd suits a log: while the view is at the end of the scroll range,
// content that grows keeps it at the end; once the user scrolls away, the
// view stays where it is, and coming back to the end sticks again. A Scroll
// with StickToEnd opens at its end. With Reverse, the end of the range is the
// top or the left: a chat whose newest line is at the bottom with Reverse
// stays at the bottom without StickToEnd, as long as the view is there.
//
// OnScroll, when not nil, is told the position after it changed: on each
// frame of a motion, after a jump, after a layout that changed the length of
// the content or of the view, and after the first layout. It may call
// SetState, as to show a button that jumps to the end while the view is away
// from it (ScrollPosition.AtEnd). A ScrollController reads the same position.
//
// A Scroll takes all the room it is given, so its parent must bound both of its
// lengths. A horizontal Scroll inside a vertical one, or a Scroll inside a
// Column, panics unless a SizedBox or an Expanded gives it a size.
type Scroll struct {
	core.StatefulBase
	Horizontal bool
	Reverse    bool
	StickToEnd bool
	// Controller, when not nil, lets code read and set the offset. A Scroll
	// without one needs none.
	Controller *ScrollController
	OnScroll   func(ScrollPosition)
	Child      core.Widget
}

type scrollWidgetState struct {
	ScrollState
	controller *ScrollController
}

// CreateState implements core.StatefulWidget.
func (Scroll) CreateState() core.State { return &scrollWidgetState{} }

// InitState implements core.State.
func (s *scrollWidgetState) InitState() {
	s.ScrollState.InitState()
	s.follow()
}

// DidUpdateWidget implements core.State.
func (s *scrollWidgetState) DidUpdateWidget(core.Widget) { s.follow() }

// follow takes the settings of the widget.
func (s *scrollWidgetState) follow() {
	w := s.Widget().(Scroll)
	s.StickToEnd, s.OnScroll = w.StickToEnd, w.OnScroll
	s.attach(w.Controller)
}

// Dispose implements core.State.
func (s *scrollWidgetState) Dispose() {
	s.attach(nil)
	s.ScrollState.Dispose()
}

// Build implements core.State.
func (s *scrollWidgetState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(Scroll)
	view := Viewport{State: &s.ScrollState, horizontal: w.Horizontal, reverse: w.Reverse, Child: w.Child}
	return ScrollFrame{Axis: view.axis(), Reverse: w.Reverse, State: &s.ScrollState, View: view}
}

// Viewport shows Child scrolled by the offset of State, and feeds State the
// sizes of the view and the content. It scrolls vertically.
type Viewport struct {
	State      *ScrollState
	horizontal bool
	reverse    bool
	Child      core.Widget
}

// Key implements core.Widget.
func (Viewport) Key() core.Key { return nil }

func (v Viewport) axis() layout.Axis {
	if v.horizontal {
		return layout.Horizontal
	}
	return layout.Vertical
}

// CreateRenderObject implements core.RenderWidget.
func (v Viewport) CreateRenderObject() core.RenderObject {
	r := &layout.RenderViewport{
		Axis:       v.axis(),
		Reverse:    v.reverse,
		Position:   &v.State.Offset,
		OnScroll:   v.State.Wheel,
		OnJump:     v.State.jumpTo,
		OnMeasured: v.State.Measure,
	}
	r.Attach()
	v.State.view = r
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v Viewport) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderViewport)
	if r.Axis != v.axis() || r.Reverse != v.reverse {
		r.Axis, r.Reverse = v.axis(), v.reverse
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (v Viewport) ChildWidget() core.Widget { return v.Child }
