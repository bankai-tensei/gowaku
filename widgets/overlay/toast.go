package overlay

import (
	"slices"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
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

// The timing and measures of a toast.
const (
	toastDuration = 4 * time.Second
	toastEnter    = 280 * time.Millisecond
	toastExit     = 160 * time.Millisecond
	// toastLimit is how many toasts show at once; a new one beyond it sends
	// the oldest away.
	toastLimit    = 3
	toastMaxWidth = 420
	// toastBottom is the room under the newest toast, and toastSide the room a
	// toast keeps from the sides of the window.
	toastBottom = theme.Space24
	toastSide   = theme.Space16
	// toastHeight is the height of a toast of one line.
	toastHeight = 44
)

// Toast is a short message shown at the bottom of the window for a few
// seconds, with an optional action, such as "Moved to trash" with "Undo". Show
// it with ShowToast.
type Toast struct {
	Message string
	// Action is the label of a button on the toast, and OnAction what it
	// does. Using it closes the toast. An empty Action shows no button.
	Action   string
	OnAction func()
	// Duration is how long the toast stays, counted while the pointer is not
	// on it and the keyboard not in it. Zero selects four seconds.
	Duration time.Duration
}

// ShowToast shows t at the bottom center of the window of ctx, below any
// toasts already there, and returns a function that closes it early. Toasts
// rise and fade in, close on their own after their Duration or on Escape while
// the keyboard is in one, and at most three show at once. Screen readers
// announce the message without moving the focus.
//
// Each toast takes the theme, MediaQuery and Localizations above ctx, as
// ShowDialog does, though toasts of different places share one stack.
//
// It must not be called during a Build.
func ShowToast(ctx core.BuildContext, t Toast) (dismiss func()) {
	o := Of(ctx)
	h := o.toasts
	if h == nil {
		h = &toastHost{overlay: o}
		o.toasts = h
		h.entry = o.Insert(func() core.Widget { return toastLayer{host: h} })
	}
	it := &toastItem{toast: t, host: h, from: ctx}
	h.items = append(h.items, it)
	live := 0
	for i := len(h.items) - 1; i >= 0; i-- {
		if h.items[i].closing {
			continue
		}
		live++
		if live > toastLimit {
			h.items[i].close()
		}
	}
	h.changed()
	return it.close
}

// toastHost is the stack of toasts of an overlay, in one entry that stays
// once the first toast showed.
type toastHost struct {
	overlay *Host
	entry   *Entry
	layer   *toastLayerState
	// items run from the oldest, at the top, to the newest.
	items []*toastItem
}

func (h *toastHost) changed() {
	if h.layer != nil {
		kit.Rebuild(&h.layer.StateBase)
	}
}

func (h *toastHost) remove(it *toastItem) {
	if i := slices.Index(h.items, it); i >= 0 {
		h.items = slices.Delete(h.items, i, i+1)
		h.changed()
	}
}

// toastItem is one toast in the stack.
type toastItem struct {
	toast Toast
	host  *toastHost
	// from is where ShowToast was called, which gives the toast its theme.
	from    core.BuildContext
	state   *toastCardState
	closing bool
}

// close sends the toast away. It does nothing to a toast already leaving.
func (it *toastItem) close() {
	if it.closing {
		return
	}
	it.closing = true
	if it.state == nil {
		it.host.remove(it)
		return
	}
	it.state.leave()
}

// toastLayer shows the stack of an overlay.
type toastLayer struct {
	core.StatefulBase
	host *toastHost
}

type toastLayerState struct {
	core.StateBase
	host *toastHost
}

// CreateState implements core.StatefulWidget.
func (l toastLayer) CreateState() core.State { return &toastLayerState{host: l.host} }

// InitState implements core.State.
func (s *toastLayerState) InitState() { s.host.layer = s }

// Build implements core.State.
func (s *toastLayerState) Build(core.BuildContext) core.Widget {
	items := s.host.items
	if len(items) == 0 {
		return toastDock{}
	}
	cards := make([]core.Widget, len(items))
	for i, it := range items {
		cards[i] = core.Keyed(it, callerScope{at: it.from, child: toastCard{item: it}})
	}
	return toastDock{child: basic.Column{Size: layout.MainMin, Cross: layout.CrossCenter, Children: cards}}
}

// toastCard is one toast.
type toastCard struct {
	core.StatefulBase
	item *toastItem
}

type toastCardState struct {
	core.StateBase
	item       *toastItem
	controller motion.AnimationController
	timer      *core.Ticker
	// remaining is the time the toast has left, counted from since while
	// counting is set. hovered and focused hold the count.
	remaining        time.Duration
	since            time.Duration
	counting         bool
	hovered, focused bool
	// announced puts the message in the live node a frame after the node
	// appears, since readers announce changes of a live node rather than new
	// ones.
	announced bool
}

// CreateState implements core.StatefulWidget.
func (c toastCard) CreateState() core.State { return &toastCardState{item: c.item} }

// InitState implements core.State.
func (s *toastCardState) InitState() {
	s.item.state = s
	s.remaining = s.item.toast.Duration
	if s.remaining <= 0 {
		s.remaining = toastDuration
	}
	c := &s.controller
	c.Duration, c.ReverseDuration = toastEnter, toastExit
	c.Curve, c.ReverseCurve = motion.SpringOut, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { kit.Rebuild(&s.StateBase) }
	c.OnStatus = func(status motion.Status) {
		if status == motion.Dismissed {
			s.item.host.remove(s.item)
		}
	}
	c.Forward()
	s.timer = s.NewTicker(s.tick)
	s.timer.Start()
}

// Dispose implements core.State.
func (s *toastCardState) Dispose() { s.controller.Dispose() }

// leave fades the toast out and takes it away.
func (s *toastCardState) leave() {
	s.timer.Stop()
	if s.controller.Progress() == 0 {
		s.controller.Stop()
		s.item.host.remove(s.item)
		return
	}
	s.controller.Reverse()
}

// hold stops or restarts the count as the pointer and the keyboard come and go.
func (s *toastCardState) hold(hovered, focused bool) {
	s.hovered, s.focused = hovered, focused
	if !s.item.closing {
		s.timer.Start()
	}
}

func (s *toastCardState) tick(now time.Duration) {
	if !s.announced {
		s.announced = true
		kit.Rebuild(&s.StateBase)
	}
	if s.hovered || s.focused {
		if s.counting {
			s.remaining -= now - s.since
			s.counting = false
		}
		s.timer.Stop()
		return
	}
	if !s.counting {
		s.counting, s.since = true, now
	}
	if now-s.since >= s.remaining {
		s.item.close()
		return
	}
	s.timer.SleepUntil(s.since + s.remaining)
}

func (s *toastCardState) key(e *core.KeyEvent) bool {
	if e.Key != input.KeyEscape || e.Modifiers != 0 {
		return false
	}
	s.item.close()
	return true
}

// Build implements core.State.
func (s *toastCardState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	t := s.item.toast
	v := s.controller.Value()
	fade := geom.Clamp32(v, 0, 1)
	rise := fade
	if basic.ReduceMotionOf(s.Context()) {
		rise = 1
	}
	parts := []core.Widget{basic.Flexible{Child: basic.Text{Content: t.Message, Style: kit.Colored(th.Text.Body, th.Color.Ink)}}}
	right := float32(toastSide)
	if t.Action != "" {
		parts = append(parts, control.Button{Label: t.Action, Style: control.ButtonQuiet, OnPress: func() {
			s.item.close()
			if t.OnAction != nil {
				t.OnAction()
			}
		}})
		// The quiet button brings room of its own.
		right = theme.Space8
	}
	label := ""
	if s.announced {
		label = t.Message
	}
	card := basic.Container{
		Color:   th.Color.Raised,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.Insets{Left: toastSide, Right: right, Top: theme.Space6, Bottom: theme.Space6},
		Child: basic.ConstrainedBox{MinHeight: toastHeight - 2*theme.Space6, Child: basic.Row{
			Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space12, Children: parts,
		}},
	}
	// The live node stays outside the fade, which hides what it covers while
	// fully transparent: the node must be there, empty, before the message.
	return toastSlot{grow: rise, child: basic.Padding{Insets: geom.Insets{Top: theme.Space8}, Child: interact.Semantics{
		Role: semantics.RoleGroup, Label: label, Live: semantics.LivePolite,
		Child: basic.Opacity{Opacity: fade, Child: interact.Focus{OnKey: s.key, Child: toastHold{
			onHold: s.hold,
			child:  basic.ConstrainedBox{MaxWidth: toastMaxWidth, Child: card},
		}}},
	}}}
}

// toastDock is the widget of renderToastDock.
type toastDock struct {
	child core.Widget
}

// Key implements core.Widget.
func (toastDock) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (toastDock) CreateRenderObject() core.RenderObject { return &renderToastDock{} }

// UpdateRenderObject implements core.RenderWidget.
func (toastDock) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (d toastDock) ChildWidget() core.Widget { return d.child }

// renderToastDock fills the overlay and holds its child at the bottom center.
// Only the child takes the pointer, so that the window stays usable around it.
type renderToastDock struct {
	layout.RenderProxy
}

// Layout implements core.RenderObject.
func (r *renderToastDock) Layout(c layout.Constraints) geom.Size {
	sz := c.Biggest()
	child := r.ChildAt(0)
	if child == nil {
		return sz
	}
	csz := core.LayoutChild(child, geom.Constraints{
		MaxW: max(sz.W-2*toastSide, 0),
		MaxH: max(sz.H-toastBottom, 0),
	})
	child.SetOffset(geom.Offset{X: geom.Floor32((sz.W-csz.W)/2 + 0.5), Y: sz.H - toastBottom - csz.H})
	return sz
}

// HitTest implements core.RenderObject.
func (r *renderToastDock) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	child := r.ChildAt(0)
	return child != nil && child.HitTest(pos.Sub(child.Offset()), res)
}

// toastSlot is the widget of renderToastSlot.
type toastSlot struct {
	grow  float32
	child core.Widget
}

// Key implements core.Widget.
func (toastSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t toastSlot) CreateRenderObject() core.RenderObject { return &renderToastSlot{grow: t.grow} }

// UpdateRenderObject implements core.RenderWidget.
func (t toastSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderToastSlot)
	if r.grow != t.grow {
		r.grow = t.grow
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t toastSlot) ChildWidget() core.Widget { return t.child }

// renderToastSlot takes grow times the height of its child, which hangs from
// its top: a toast growing in pushes the older ones up and itself rises from
// below, and shrinking away lets them back down.
type renderToastSlot struct {
	layout.RenderProxy
	grow float32
}

// Layout implements core.RenderObject.
func (r *renderToastSlot) Layout(c layout.Constraints) geom.Size {
	child := r.ChildAt(0)
	if child == nil {
		return c.Constrain(geom.Size{})
	}
	csz := core.LayoutChild(child, c)
	return c.Constrain(geom.Size{W: csz.W, H: csz.H * r.grow})
}

// toastHold is the widget of renderToastHold.
type toastHold struct {
	onHold func(hovered, focused bool)
	child  core.Widget
}

// Key implements core.Widget.
func (toastHold) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t toastHold) CreateRenderObject() core.RenderObject { return &renderToastHold{onHold: t.onHold} }

// UpdateRenderObject implements core.RenderWidget.
func (t toastHold) UpdateRenderObject(ro core.RenderObject) { ro.(*renderToastHold).onHold = t.onHold }

// ChildWidget implements core.SingleChildWidget.
func (t toastHold) ChildWidget() core.Widget { return t.child }

// renderToastHold reports whether the pointer is over its child or the
// keyboard inside it.
type renderToastHold struct {
	layout.RenderProxy
	onHold           func(hovered, focused bool)
	hovered, focused bool
}

// HandleHover implements core.HoverTarget.
func (r *renderToastHold) HandleHover(inside bool) {
	r.hovered = inside
	r.onHold(r.hovered, r.focused)
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderToastHold) FocusWithinChanged(within bool) {
	r.focused = within
	r.onHold(r.hovered, r.focused)
}
