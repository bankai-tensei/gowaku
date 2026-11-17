package behavior

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Control is the behavior that the controls share: hover and focus fade, and a
// press moves on a spring, which is what gives them their feel (DESIGN §7).
type Control struct {
	base *core.StateBase
	Hovered, pressed,
	Focused bool
	// shown means the focus should show: the keyboard gave it.
	Shown bool

	Tint     motion.Implicit
	Hot, Lit *motion.Value[float32]
	Sink     motion.SpringValue
}

// Attach belongs in InitState.
func (c *Control) Attach(base *core.StateBase) {
	c.base = base
	refresh := func() { kit.Rebuild(base) }
	c.Tint.Attach(base, refresh)
	c.Hot = motion.Track(&c.Tint, 0, motion.LerpFloat)
	c.Lit = motion.Track(&c.Tint, 0, motion.LerpFloat)
	c.Sink.OnChange = refresh
	c.Sink.Attach(base, 0)
}

// Dispose belongs in the Dispose of the State.
func (c *Control) Dispose() {
	c.Tint.Dispose()
	c.Sink.Dispose()
}

func (c *Control) sync() {
	c.Hot.Set(kit.OnOff(c.Hovered))
	c.Lit.Set(kit.OnOff(c.Focused && c.Shown))
	c.Tint.Commit(kit.HoverDuration, motion.Linear)
	kit.Rebuild(c.base)
}

// Hover follows the pointer into and out of the control.
func (c *Control) Hover(inside bool) {
	if c.Hovered != inside {
		c.Hovered = inside
		c.sync()
	}
}

// Focus follows the keyboard focus.
func (c *Control) Focus(focused bool) {
	if c.Focused != focused {
		c.Focused = focused
		c.sync()
	}
}

// ShowFocus follows whether the focus should show.
func (c *Control) ShowFocus(show bool) {
	if c.Shown != show {
		c.Shown = show
		c.sync()
	}
}

// Press sinks the control under a press.
func (c *Control) Press() {
	c.pressed = true
	c.Sink.Set(1)
	kit.Rebuild(c.base)
}

// Release lets the control rise again.
func (c *Control) Release() {
	c.pressed = false
	c.Sink.Set(0)
	kit.Rebuild(c.base)
}

// Wrap puts child inside what makes it a control, and sem around that, which
// presses the control for a screen reader. Disabled, the child stays bare and
// sem says so.
func (c *Control) Wrap(sem interact.Semantics, enabled bool, activate func(), child core.Widget) core.Widget {
	if !enabled {
		sem.State |= semantics.Disabled
		sem.Child = child
		return sem
	}
	sem.OnTap = activate
	sem.Child = c.Focusable(activate, child)
	return sem
}

// Focusable puts child inside the focus, hover and press handling of a control.
func (c *Control) Focusable(activate func(), child core.Widget) core.Widget {
	return interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			if !kit.Activates(e) {
				return false
			}
			if !e.Repeat {
				c.Sink.Kick(kit.KeyKick)
				activate()
			}
			return true
		},
		OnFocusChange: c.Focus,
		OnShowFocus:   c.ShowFocus,
		Child: interact.MouseRegion{
			OnEnter: func() { c.Hover(true) },
			OnExit:  func() { c.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   c.Press,
				OnTapCancel: c.Release,
				OnTap: func() {
					c.Release()
					activate()
				},
				Child: child,
			},
		},
	}
}

// Ring returns child inside the focus ring of the control, in color.
func (c *Control) Ring(radius float32, color geom.Color, child core.Widget) core.Widget {
	return kit.Ring{Radius: radius, Color: color.WithAlpha(kit.RingOpacity * c.Lit.Get()), Child: child}
}
