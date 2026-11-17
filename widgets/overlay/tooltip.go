package overlay

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The timing and measures of a tooltip.
const (
	// tooltipDelay is how long the pointer rests on a control before its
	// tooltip shows.
	tooltipDelay = 500 * time.Millisecond
	// tooltipWarm is how long after a tooltip hides the next one shows without
	// the delay, so that running the pointer along a toolbar reads each one.
	tooltipWarm  = time.Second
	tooltipEnter = 120 * time.Millisecond
	tooltipExit  = 80 * time.Millisecond
	// tooltipGap is the room between the control and its tooltip, and
	// tooltipEdge the room a tooltip keeps from the edges of the window.
	tooltipGap      = theme.Space6
	tooltipEdge     = theme.Space4
	tooltipMaxWidth = 280
)

// Tooltip shows a short label for its child when the pointer rests on it for
// half a second, and at once when the keyboard moves to it. Moving the pointer
// on to another control within a second of a tooltip shows the next one without
// the wait. The label sits below the child, or above it near the bottom of the
// window, and goes away when the pointer leaves, on a press or a turn of the
// wheel, and on Escape.
//
// Assistive technology gets Message as the description of the child's first
// node, and as a tooltip while the label shows. A Tooltip with no Message shows
// nothing.
//
//	Tooltip{Message: "太字 (Ctrl+B)", Child: boldButton}
type Tooltip struct {
	core.StatefulBase
	Message string
	Child   core.Widget
}

// tooltipWarmth is what the tooltips of a window share, to know whether one
// was open a moment ago. It lives in the Overlay.
type tooltipWarmth struct {
	// open counts the tooltips showing, and until is when the warmth after the
	// last one ends. pending says that one hid and has yet to set until,
	// which takes a tick to learn the time.
	open    int
	until   time.Duration
	pending bool
}

// warm reports whether a tooltip may show without the delay at now.
func (w *tooltipWarmth) warm(now time.Duration) bool {
	return w.open > 0 || w.pending || now < w.until
}

type tooltipState struct {
	core.StateBase
	overlay *Host
	timer   *core.Ticker
	target  *renderTooltipTarget
	// waiting runs the delay, which ends at wake once armed has learned the
	// time. stamp asks the next tick to record when this tooltip hid.
	waiting, armed, stamp bool
	wake                  time.Duration
	// hovered and focused say why the label may show. muted keeps it away
	// after a press or Escape, until the pointer leaves.
	hovered, focused, muted bool
	// shown says that the label should show; entry and bubble are its overlay
	// entry and state, which stay while it fades out.
	shown  bool
	gone   bool
	entry  *Entry
	bubble *tooltipBubbleState
	// anchor is the box of the child in the overlay when the label opened,
	// and message the text of the last build, which outlives the State.
	anchor  geom.Rect
	message string
}

// CreateState implements core.StatefulWidget.
func (Tooltip) CreateState() core.State { return &tooltipState{} }

// InitState implements core.State.
func (s *tooltipState) InitState() { s.timer = s.NewTicker(s.tick) }

// Dispose implements core.State. The label cannot leave the overlay during a
// build, so it fades out on its own.
func (s *tooltipState) Dispose() {
	s.gone = true
	if s.overlay != nil {
		w := &s.overlay.tooltips
		if s.shown {
			w.open--
		}
		if s.stamp {
			w.pending = false
		}
	}
	s.shown = false
	if s.bubble != nil {
		s.bubble.close()
	}
}

// Build implements core.State.
func (s *tooltipState) Build(ctx core.BuildContext) core.Widget {
	t := s.Widget().(Tooltip)
	if o, ok := core.Lookup[*Host](ctx); ok {
		s.overlay = o
	}
	s.message = t.Message
	return tooltipTarget{
		ref:      &s.target,
		message:  t.Message,
		onHover:  s.hover,
		onPress:  s.press,
		onFocus:  s.focus,
		onEscape: s.escape,
		child:    t.Child,
	}
}

func (s *tooltipState) hover(inside bool) {
	s.hovered = inside
	if inside {
		if !s.muted && !s.shown {
			s.waiting, s.armed = true, false
			s.timer.Start()
		}
		return
	}
	s.muted, s.waiting = false, false
	if !s.focused {
		s.hide()
	}
}

func (s *tooltipState) press() {
	s.muted = s.hovered
	s.waiting = false
	s.hide()
}

// focus follows the keyboard into and out of the child. A press that gives the
// child the focus is a press, and the keyboard is anything else.
func (s *tooltipState) focus(within, keyboard bool) {
	switch {
	case within && keyboard:
		s.focused, s.muted, s.waiting = true, false, false
		s.show()
	case within:
		s.focused = false
		s.press()
	default:
		s.focused = false
		if !s.hovered {
			s.waiting = false
			s.hide()
		}
	}
}

func (s *tooltipState) escape() bool {
	if !s.shown {
		return false
	}
	s.muted = true
	s.hide()
	return true
}

func (s *tooltipState) tick(now time.Duration) {
	if s.overlay == nil {
		s.timer.Stop()
		return
	}
	w := &s.overlay.tooltips
	if s.stamp {
		s.stamp, w.pending = false, false
		w.until = now + tooltipWarm
	}
	if s.waiting {
		if !s.armed {
			s.armed, s.wake = true, now+tooltipDelay
			if w.warm(now) {
				s.wake = now
			}
		}
		if now < s.wake {
			s.timer.SleepUntil(s.wake)
			return
		}
		s.waiting = false
		s.show()
	}
	s.timer.Stop()
}

// show opens the label below the child.
func (s *tooltipState) show() {
	if s.shown || s.gone || s.overlay == nil || s.target == nil || !s.target.Attached() {
		return
	}
	if s.message == "" {
		return
	}
	s.shown = true
	s.overlay.tooltips.open++
	s.anchor = kit.GlobalRect(s.target)
	if ro := s.overlay.Context().RenderObject(); ro != nil {
		o := core.LocalToGlobal(ro, geom.Offset{})
		s.anchor.X, s.anchor.Y = s.anchor.X-o.X, s.anchor.Y-o.Y
	}
	if s.bubble != nil {
		s.bubble.open()
		return
	}
	if s.entry == nil {
		s.entry = s.overlay.InsertFrom(s.Context(), func() core.Widget { return tooltipBubble{owner: s} })
	}
}

// hide fades the label out.
func (s *tooltipState) hide() {
	if !s.shown {
		return
	}
	s.shown = false
	w := &s.overlay.tooltips
	w.open--
	w.pending, s.stamp = true, true
	s.timer.Start()
	if s.bubble != nil {
		s.bubble.close()
	}
}

// removed forgets the label once its entry left the overlay.
func (s *tooltipState) removed() { s.entry, s.bubble = nil, nil }

// tooltipBubble is the label of a Tooltip in the overlay.
type tooltipBubble struct {
	core.StatefulBase
	owner *tooltipState
}

type tooltipBubbleState struct {
	core.StateBase
	owner      *tooltipState
	controller motion.AnimationController
	// reaper takes the entry out of the overlay on the tick after the label
	// faded out, since that can happen during a build.
	reaper *core.Ticker
}

// CreateState implements core.StatefulWidget.
func (b tooltipBubble) CreateState() core.State { return &tooltipBubbleState{owner: b.owner} }

// InitState implements core.State.
func (s *tooltipBubbleState) InitState() {
	s.owner.bubble = s
	c := &s.controller
	c.Duration, c.ReverseDuration = tooltipEnter, tooltipExit
	c.Curve, c.ReverseCurve = motion.EaseOutCubic, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { kit.Rebuild(&s.StateBase) }
	c.OnStatus = func(status motion.Status) {
		if status == motion.Dismissed {
			s.reaper.Start()
		}
	}
	s.reaper = s.NewTicker(s.reap)
	if s.owner.shown {
		c.Forward()
	} else {
		s.reaper.Start()
	}
}

// Dispose implements core.State.
func (s *tooltipBubbleState) Dispose() { s.controller.Dispose() }

func (s *tooltipBubbleState) open() {
	s.reaper.Stop()
	s.controller.Forward()
	kit.Rebuild(&s.StateBase)
}

func (s *tooltipBubbleState) close() {
	if s.controller.Progress() == 0 {
		// Reverse would end at once and rebuild, which a Dispose during a
		// build must not do.
		s.controller.Stop()
		s.reaper.Start()
		return
	}
	s.controller.Reverse()
}

func (s *tooltipBubbleState) reap(time.Duration) {
	s.reaper.Stop()
	if s.owner.shown {
		return
	}
	if s.owner.entry != nil {
		s.owner.entry.Remove()
	}
	s.owner.removed()
}

// Build implements core.State.
func (s *tooltipBubbleState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	message := s.owner.message
	if message == "" {
		return basic.SizedBox{}
	}
	return tooltipPlace{anchor: s.owner.anchor, child: basic.Opacity{
		Opacity: geom.Clamp32(s.controller.Value(), 0, 1),
		Child: tooltipLabel{message: message, child: basic.Container{
			Color:   th.Color.Raised,
			Radius:  th.Radius.Control,
			Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
			Shadows: th.Color.Shadows(theme.LiftKnob),
			Padding: geom.Insets{Left: theme.Space8, Right: theme.Space8, Top: theme.Space4, Bottom: theme.Space4},
			Child:   basic.Text{Content: message, Style: kit.Colored(th.Text.Caption, th.Color.Ink)},
		}},
	}}
}

// tooltipTarget is the widget of renderTooltipTarget.
type tooltipTarget struct {
	ref      **renderTooltipTarget
	message  string
	onHover  func(inside bool)
	onPress  func()
	onFocus  func(within, keyboard bool)
	onEscape func() bool
	child    core.Widget
}

// Key implements core.Widget.
func (tooltipTarget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tooltipTarget) CreateRenderObject() core.RenderObject {
	r := &renderTooltipTarget{}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tooltipTarget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTooltipTarget)
	r.message, r.onHover, r.onPress, r.onFocus, r.onEscape = t.message, t.onHover, t.onPress, t.onFocus, t.onEscape
	*t.ref = r
}

// ChildWidget implements core.SingleChildWidget.
func (t tooltipTarget) ChildWidget() core.Widget { return t.child }

// renderTooltipTarget watches the pointer and the focus over the child of a
// Tooltip, and describes the child with the message.
type renderTooltipTarget struct {
	layout.RenderProxy
	message  string
	onHover  func(inside bool)
	onPress  func()
	onFocus  func(within, keyboard bool)
	onEscape func() bool
}

// HitTest implements core.RenderObject. The object goes on the path before its
// child, so that it sees the presses the child takes; it never takes one.
func (r *renderTooltipTarget) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	n := len(res.Path)
	if !r.RenderProxy.HitTest(pos, res) {
		return false
	}
	last := len(res.Path) - 1
	copy(res.Path[n+1:], res.Path[n:last])
	res.Path[n] = r
	return true
}

// HandleHover implements core.HoverTarget.
func (r *renderTooltipTarget) HandleHover(inside bool) { r.onHover(inside) }

// HandlePointer implements core.PointerTarget. It only looks.
func (r *renderTooltipTarget) HandlePointer(e *core.PointerEvent) {
	if e.Kind == core.PointerDown || e.Kind == core.PointerScroll {
		r.onPress()
	}
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderTooltipTarget) FocusWithinChanged(within bool) {
	r.onFocus(within, r.FocusVisible())
}

// HandleShortcut implements core.ShortcutTarget: Escape closes the label
// wherever the focus is.
func (r *renderTooltipTarget) HandleShortcut(e *core.KeyEvent) bool {
	return e.Key == input.KeyEscape && e.Modifiers == 0 && r.onEscape()
}

// Semantics implements core.RenderObject.
func (r *renderTooltipTarget) Semantics(sb *core.SemanticsBuilder) {
	sb.Describe(r.message)
	r.RenderProxy.Semantics(sb)
	sb.Describe("")
}

// tooltipPlace is the widget of renderTooltipPlace.
type tooltipPlace struct {
	anchor geom.Rect
	child  core.Widget
}

// Key implements core.Widget.
func (tooltipPlace) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p tooltipPlace) CreateRenderObject() core.RenderObject {
	return &renderTooltipPlace{anchor: p.anchor}
}

// UpdateRenderObject implements core.RenderWidget.
func (p tooltipPlace) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTooltipPlace)
	if r.anchor != p.anchor {
		r.anchor = p.anchor
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p tooltipPlace) ChildWidget() core.Widget { return p.child }

// renderTooltipPlace fills the overlay and centers its child below anchor, or
// above it when there is no room below, inside the window. The pointer goes
// through it.
type renderTooltipPlace struct {
	layout.RenderProxy
	anchor geom.Rect
}

// Layout implements core.RenderObject.
func (r *renderTooltipPlace) Layout(c layout.Constraints) geom.Size {
	sz := c.Biggest()
	child := r.ChildAt(0)
	if child == nil {
		return sz
	}
	room := geom.Constraints{
		MaxW: max(min(tooltipMaxWidth, sz.W-2*tooltipEdge), 0),
		MaxH: max(sz.H-2*tooltipEdge, 0),
	}
	csz := core.LayoutChild(child, room)
	a := r.anchor
	x := a.X + (a.W-csz.W)/2
	y := a.Y + a.H + tooltipGap
	if y+csz.H > sz.H-tooltipEdge && a.Y-tooltipGap-csz.H >= tooltipEdge {
		y = a.Y - tooltipGap - csz.H
	}
	x = geom.Clamp32(x, tooltipEdge, max(sz.W-tooltipEdge-csz.W, tooltipEdge))
	y = geom.Clamp32(y, tooltipEdge, max(sz.H-tooltipEdge-csz.H, tooltipEdge))
	child.SetOffset(geom.Offset{X: geom.Floor32(x + 0.5), Y: geom.Floor32(y + 0.5)})
	return sz
}

// HitTest implements core.RenderObject. A tooltip never takes the pointer, so
// that it cannot cover what the pointer moves to.
func (r *renderTooltipPlace) HitTest(geom.Offset, *core.HitTestResult) bool { return false }

// tooltipLabel is the widget of renderTooltipLabel.
type tooltipLabel struct {
	message string
	child   core.Widget
}

// Key implements core.Widget.
func (tooltipLabel) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l tooltipLabel) CreateRenderObject() core.RenderObject {
	return &renderTooltipLabel{message: l.message}
}

// UpdateRenderObject implements core.RenderWidget.
func (l tooltipLabel) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderTooltipLabel).message = l.message
}

// ChildWidget implements core.SingleChildWidget.
func (l tooltipLabel) ChildWidget() core.Widget { return l.child }

// renderTooltipLabel is the tooltip node of the label. It speaks for the text
// inside, which would otherwise be read a second time.
type renderTooltipLabel struct {
	layout.RenderProxy
	message string
}

// Semantics implements core.RenderObject.
func (r *renderTooltipLabel) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleTooltip, Label: r.message}, r.Size())
	sb.Close()
}
