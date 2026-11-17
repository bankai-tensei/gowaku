package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/semantics"
)

// dockDivider is the handle between child index and the next one of a split
// of a DockLayout, which looks and works as the divider of a SplitPane: the
// pointer drags it, a double click shares the room evenly, and the arrow keys,
// Home and End move it.
type dockDivider struct {
	core.StatefulBase
	dock  *dockState
	split *dockNode
	index int
	label string
}

type dockDividerState struct {
	core.StateOf[dockDivider]
	behavior.Control
	dragging bool
	// grip are the lengths of the two children when a drag started.
	grip [2]float32
}

// CreateState implements core.StatefulWidget.
func (dockDivider) CreateState() core.State { return &dockDividerState{} }

// InitState implements core.State.
func (s *dockDividerState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *dockDividerState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *dockDividerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	c := &basic.ThemeOf(ctx).Color
	active := max(s.Hot.Get(), kit.OnOff(s.dragging))
	lit := s.Lit.Get()
	line := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, active), c.AccentInk, lit)
	sem := interact.Semantics{
		Role: semantics.RoleSeparator, Label: w.label,
		OnIncrease: func() { s.moveBy(dockKeyStep) },
		OnDecrease: func() { s.moveBy(-dockKeyStep) },
		OnSetValue: func(_ string, number float64) { s.setPercent(float32(number)) },
	}
	if i := w.index; i+1 < len(w.split.weights) {
		a, b := w.split.weights[i], w.split.weights[i+1]
		la, lb := pairLengths(w.split, i)
		if a+b > 0 && la+lb > 0 {
			sem.Range = semantics.Range{
				Max: 100, Value: float64(a / (a + b) * 100), Step: float64(dockKeyStep / (la + lb) * 100),
			}
		}
	}
	sem.Child = interact.Focus{
		Enabled:       true,
		OnKey:         s.key,
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: kit.SplitHandle{
			Axis: w.split.axis, Line: line, Ring: c.AccentInk.WithAlpha(kit.RingOpacity * lit),
			OnHover: s.Hover,
			OnStart: s.dragStart,
			OnDrag:  s.drag,
			OnEnd:   s.dragEnd,
			OnReset: s.reset,
		},
	}
	return sem
}

func (s *dockDividerState) dragStart() {
	w := s.Widget()
	s.dragging = true
	s.grip[0], s.grip[1] = pairLengths(w.split, w.index)
	kit.Rebuild(&s.StateBase)
}

// drag gives the first child its length at the start of the drag and delta,
// without a build: only the layout runs again.
func (s *dockDividerState) drag(delta float32) {
	w := s.Widget()
	w.dock.resize(w.split, w.index, s.grip, s.grip[0]+delta)
}

func (s *dockDividerState) dragEnd() {
	s.dragging = false
	s.Widget().dock.userChanged()
	kit.Rebuild(&s.StateBase)
}

// moveBy moves the divider by d logical pixels.
func (s *dockDividerState) moveBy(d float32) {
	w := s.Widget()
	a, b := pairLengths(w.split, w.index)
	s.moveTo(a+d, [2]float32{a, b})
}

// setPercent gives the first child p percent of the room of the two.
func (s *dockDividerState) setPercent(p float32) {
	w := s.Widget()
	a, b := pairLengths(w.split, w.index)
	s.moveTo(geom.Clamp32(p, 0, 100)/100*(a+b), [2]float32{a, b})
}

func (s *dockDividerState) moveTo(first float32, grip [2]float32) {
	w := s.Widget()
	w.dock.resize(w.split, w.index, grip, first)
	w.dock.userChanged()
	kit.Rebuild(&s.StateBase)
}

// reset shares the room of the two children evenly.
func (s *dockDividerState) reset() {
	w := s.Widget()
	if i := w.index; i+1 < len(w.split.weights) {
		sum := w.split.weights[i] + w.split.weights[i+1]
		w.split.weights[i], w.split.weights[i+1] = sum/2, sum/2
		w.dock.relayout()
		w.dock.userChanged()
	}
	kit.Rebuild(&s.StateBase)
}

// key moves the divider with the arrow keys along its axis, and to the limits
// with Home and End.
func (s *dockDividerState) key(e *core.KeyEvent) bool {
	w := s.Widget()
	if e.Modifiers != 0 {
		return false
	}
	back, forth := input.KeyLeft, input.KeyRight
	if w.split.axis == layout.Vertical {
		back, forth = input.KeyUp, input.KeyDown
	}
	switch e.Key {
	case back:
		s.moveBy(-dockKeyStep)
	case forth:
		s.moveBy(dockKeyStep)
	case input.KeyHome, input.KeyEnd:
		a, b := pairLengths(w.split, w.index)
		to := float32(0)
		if e.Key == input.KeyEnd {
			to = a + b
		}
		s.moveTo(to, [2]float32{a, b})
	default:
		return false
	}
	return true
}

// pairLengths returns the lengths of child i and the next one of split along
// its axis, at the last layout.
func pairLengths(split *dockNode, i int) (float32, float32) {
	if i+1 >= len(split.children) {
		return 0, 0
	}
	a, b := split.children[i].rect, split.children[i+1].rect
	if split.axis == layout.Vertical {
		return a.H, b.H
	}
	return a.W, b.W
}
