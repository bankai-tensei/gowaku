package basic

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
)

const (
	// scrollBarLinger is how long the bar stays after the view stopped moving.
	scrollBarLinger = 900 * time.Millisecond
	scrollBarFade   = 150 * time.Millisecond
)

// ScrollFrame is View with the scroll bar of State over its edge along Axis.
// Reverse puts offset 0 at the right end of the bar, or at its bottom, for a
// view whose scroll range starts there, as a Scroll with Reverse.
type ScrollFrame struct {
	Axis    layout.Axis
	Reverse bool
	State   *ScrollState
	View    core.Widget
}

// Key implements core.Widget.
func (ScrollFrame) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f ScrollFrame) CreateRenderObject() core.RenderObject {
	return &layout.RenderScrollFrame{Axis: f.Axis}
}

// UpdateRenderObject implements core.RenderWidget.
func (f ScrollFrame) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderScrollFrame)
	if r.Axis != f.Axis {
		r.Axis = f.Axis
		r.MarkNeedsLayout()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (f ScrollFrame) ChildWidgets() []core.Widget {
	return []core.Widget{f.View, scrollBar{axis: f.Axis, reverse: f.Reverse, view: f.State}}
}

// scrollBar shows the position of a view while it moves or the pointer is near.
type scrollBar struct {
	core.StatefulBase
	axis    layout.Axis
	reverse bool
	view    *ScrollState
}

type scrollBarState struct {
	core.StateBase
	hovered bool
	// armed means that the next wake of linger ends the stay.
	armed   bool
	linger  *core.Ticker
	fade    motion.Implicit
	opacity *motion.Value[float32]
}

// CreateState implements core.StatefulWidget.
func (scrollBar) CreateState() core.State { return &scrollBarState{} }

// InitState implements core.State.
func (s *scrollBarState) InitState() {
	s.fade.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.opacity = motion.Track(&s.fade, 0, motion.LerpFloat)
	s.linger = s.NewTicker(s.wake)
	s.Widget().(scrollBar).view.bar = s
}

// Dispose implements core.State.
func (s *scrollBarState) Dispose() {
	s.fade.Dispose()
	s.linger.Stop()
	s.Widget().(scrollBar).view.bar = nil
}

// wake first sets the alarm, because what asks for a stay has no clock, and then
// ends the stay when the alarm goes off.
func (s *scrollBarState) wake(now time.Duration) {
	if !s.armed {
		s.armed = true
		s.linger.SleepUntil(now + scrollBarLinger)
		return
	}
	s.armed = false
	s.linger.Stop()
	if !s.hovered {
		s.show(0)
	}
}

func (s *scrollBarState) show(target float32) {
	s.opacity.Set(target)
	s.fade.Commit(scrollBarFade, motion.Linear)
}

// activity shows the bar and starts its stay over.
func (s *scrollBarState) activity() {
	s.show(1)
	s.armed = false
	s.linger.Start()
}

func (s *scrollBarState) hover(inside bool) {
	s.hovered = inside
	if inside {
		s.show(1)
	} else {
		s.activity()
	}
	kit.Rebuild(&s.StateBase)
}

// Build implements core.State.
func (s *scrollBarState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(scrollBar)
	return scrollBarPaint{
		axis:    w.axis,
		reverse: w.reverse,
		view:    w.view,
		color:   ThemeOf(ctx).Color.Ink3,
		opacity: s.opacity.Get(),
		wide:    s.hovered,
		onHover: s.hover,
	}
}

// scrollBarPaint is the widget of layout.RenderScrollBar.
type scrollBarPaint struct {
	axis    layout.Axis
	reverse bool
	view    *ScrollState
	color   geom.Color
	opacity float32
	wide    bool
	onHover func(inside bool)
}

// Key implements core.Widget.
func (scrollBarPaint) Key() core.Key { return nil }

func (p scrollBarPaint) apply(r *layout.RenderScrollBar) {
	r.Axis, r.Reverse, r.Position, r.Metrics = p.axis, p.reverse, &p.view.Offset, &p.view.Metrics
	r.Color, r.Opacity, r.Wide = p.color, p.opacity, p.wide
	r.OnDrag, r.OnScroll, r.OnHover = p.view.Jump, p.view.Wheel, p.onHover
}

// CreateRenderObject implements core.RenderWidget.
func (p scrollBarPaint) CreateRenderObject() core.RenderObject {
	r := &layout.RenderScrollBar{}
	p.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p scrollBarPaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderScrollBar)
	p.apply(r)
	r.MarkNeedsPaint()
}
