package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/semantics"
)

// dockFlyoutHandle is the handle on the inner edge of the panel that slid
// out, which sizes it as the divider of a SplitPane does: the pointer drags
// it, a double click goes back to the first size, and the arrow keys move it.
type dockFlyoutHandle struct {
	core.StatefulBase
	dock  *dockState
	side  DockZone
	label string
}

type dockFlyoutHandleState struct {
	core.StateOf[dockFlyoutHandle]
	behavior.Control
	dragging bool
	// grip is the size of the panel when a drag started.
	grip float32
}

// CreateState implements core.StatefulWidget.
func (dockFlyoutHandle) CreateState() core.State { return &dockFlyoutHandleState{} }

// InitState implements core.State.
func (s *dockFlyoutHandleState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *dockFlyoutHandleState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *dockFlyoutHandleState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	c := &basic.ThemeOf(ctx).Color
	active := max(s.Hot.Get(), kit.OnOff(s.dragging))
	lit := s.Lit.Get()
	line := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, active), c.AccentInk, lit)
	sem := interact.Semantics{
		Role: semantics.RoleSeparator, Label: w.label,
		OnIncrease: func() { s.moveBy(dockKeyStep) },
		OnDecrease: func() { s.moveBy(-dockKeyStep) },
	}
	// The room is that of the last layout, which the panel coming out now
	// has not had yet.
	if r, h := w.dock.view, w.dock.ctrl.hiddenEntry(w.dock.ctrl.flyout); r != nil && h != nil {
		if length, _, _, total := r.flyoutSize(h); total > 0 {
			sem.Range = semantics.Range{Max: 100, Value: float64(length / total * 100), Step: float64(dockKeyStep / total * 100)}
		}
	}
	axis := layout.Horizontal
	if w.side == DockTop || w.side == DockBottom {
		axis = layout.Vertical
	}
	sem.Child = interact.Focus{
		Enabled:       true,
		OnKey:         s.key,
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: kit.SplitHandle{
			Axis: axis, Line: line, Ring: c.AccentInk.WithAlpha(kit.RingOpacity * lit),
			OnHover: s.Hover,
			OnStart: s.dragStart,
			OnDrag:  s.drag,
			OnEnd:   s.dragEnd,
			OnReset: s.reset,
		},
	}
	return sem
}

// sign is +1 where a move toward the larger coordinate grows the panel, at
// the left and top, and -1 at the right and bottom.
func (s *dockFlyoutHandleState) sign() float32 {
	if side := s.Widget().side; side == DockRight || side == DockBottom {
		return -1
	}
	return 1
}

// length returns the size of the panel at the last layout.
func (s *dockFlyoutHandleState) length() float32 {
	if r := s.Widget().dock.view; r != nil {
		return r.flyLen
	}
	return 0
}

func (s *dockFlyoutHandleState) dragStart() {
	s.dragging = true
	s.grip = s.length()
	kit.Rebuild(&s.StateBase)
}

// drag sizes the panel without a build: only the layout runs again.
func (s *dockFlyoutHandleState) drag(delta float32) {
	s.Widget().dock.resizeFlyout(s.grip + s.sign()*delta)
}

func (s *dockFlyoutHandleState) dragEnd() {
	s.dragging = false
	s.Widget().dock.userChanged()
	kit.Rebuild(&s.StateBase)
}

// moveBy grows the panel by d logical pixels.
func (s *dockFlyoutHandleState) moveBy(d float32) {
	w := s.Widget()
	w.dock.resizeFlyout(s.length() + d)
	w.dock.userChanged()
	kit.Rebuild(&s.StateBase)
}

// reset gives the panel back the size the layout chooses.
func (s *dockFlyoutHandleState) reset() {
	w := s.Widget()
	if h := w.dock.ctrl.hiddenEntry(w.dock.ctrl.flyout); h != nil && h.size != 0 {
		h.size = 0
		w.dock.relayout()
		w.dock.userChanged()
	}
	kit.Rebuild(&s.StateBase)
}

// key moves the edge with the arrow keys along the axis of the handle.
func (s *dockFlyoutHandleState) key(e *core.KeyEvent) bool {
	if e.Modifiers != 0 {
		return false
	}
	back, forth := input.KeyLeft, input.KeyRight
	if side := s.Widget().side; side == DockTop || side == DockBottom {
		back, forth = input.KeyUp, input.KeyDown
	}
	switch e.Key {
	case back:
		s.moveBy(-s.sign() * dockKeyStep)
	case forth:
		s.moveBy(s.sign() * dockKeyStep)
	default:
		return false
	}
	return true
}
