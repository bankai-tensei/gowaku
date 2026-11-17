package layout

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// Tap is a press or a click of a pointer button: where it was, relative to
// the object that reports it, and the modifier keys held at that moment, so
// that Shift or Ctrl with a click can extend or add to a selection.
type Tap struct {
	Pos       geom.Offset
	Modifiers input.Modifiers
}

// RenderGesture turns pointer events over its child into taps, double taps,
// long presses and drags.
type RenderGesture struct {
	RenderProxy
	// OnDown runs when the primary button goes down.
	OnDown func()
	// OnDownAt is OnDown for those who want the place of the press and the
	// modifier keys.
	OnDownAt func(t Tap)
	// OnTap runs when the button comes up over the object, unless the press
	// became a drag or a long press, or OnDoubleTap takes it. OnTapAt runs
	// then too, with the place of the release and the modifier keys held.
	OnTap   func()
	OnTapAt func(t Tap)
	// OnDoubleTap runs in place of OnTap when the button comes up over the
	// object after the second press of a double click (core.PointerEvent.Clicks
	// 2, 4 and so on). The first press is a tap of its own, at once.
	OnDoubleTap func()
	// OnCancel runs when the button comes up elsewhere, when the press is
	// canceled, and when it becomes a drag.
	OnCancel func()
	// OnSecondaryTapAt runs when the secondary button goes down and comes up
	// over the object, with the place of the release and the modifier keys:
	// the gesture of a context menu.
	OnSecondaryTapAt func(t Tap)
	// OnPinch runs when two fingers pinch on a touchpad over the object, or
	// Ctrl and the wheel turn, with the change of scale and the place.
	OnPinch func(factor float32, focus geom.Offset)
	// OnLongPress runs when the primary button or a finger stays down for
	// longPressDelay without moving, with the place and what pressed. The
	// release is then no tap, and the press no drag.
	OnLongPress func(pos geom.Offset, device input.PointerDevice)
	// OnDragStart runs when the pointer moves more than dragSlop from where the
	// primary button went down, with that place. OnDragUpdate follows at once
	// with the move so far, then with each move: the place and the change since
	// the last call. OnDragEnd runs with the last place when the button comes
	// up or the press is canceled.
	OnDragStart  func(pos geom.Offset)
	OnDragUpdate func(pos, delta geom.Offset)
	OnDragEnd    func(pos geom.Offset)

	pressed, pressedSecondary bool
	// The press in progress: where it went down, where the pointer was last,
	// whether it is the second of a double click, and whether it became a drag.
	downPos, lastPos geom.Offset
	double, dragging bool
	// The long press in progress: with what it went down, when, and whether it
	// fired.
	longTicker  *core.Ticker
	longDevice  input.PointerDevice
	longStart   time.Duration
	longStarted bool
	longFired   bool
}

// The long press: how long it takes, and how far the pointer may move. A
// press that moves further becomes a drag, if the object takes drags.
const (
	longPressDelay = 500 * time.Millisecond
	longPressSlop  = 8
	dragSlop       = longPressSlop
)

// active reports whether the object has any use for the primary button.
func (r *RenderGesture) active() bool {
	return r.OnDown != nil || r.OnDownAt != nil || r.OnTap != nil || r.OnTapAt != nil || r.OnDoubleTap != nil ||
		r.OnCancel != nil || r.OnLongPress != nil || r.drags()
}

// drags reports whether the object takes drags.
func (r *RenderGesture) drags() bool {
	return r.OnDragStart != nil || r.OnDragUpdate != nil || r.OnDragEnd != nil
}

// YieldsToPan implements core.PanYielder: a finger that pressed a tap target
// may scroll instead, unless the object takes drags.
func (r *RenderGesture) YieldsToPan() bool { return !r.drags() }

// startLongPress waits for a long press from a press.
func (r *RenderGesture) startLongPress(device input.PointerDevice) {
	if r.OnLongPress == nil {
		return
	}
	if r.longTicker == nil {
		r.longTicker = r.NewTicker(r.longTick)
	}
	r.longDevice, r.longStarted, r.longFired = device, false, false
	r.longTicker.Start()
}

func (r *RenderGesture) stopLongPress() {
	if r.longTicker != nil {
		r.longTicker.Stop()
	}
}

// longTick fires the long press once the delay has passed.
func (r *RenderGesture) longTick(now time.Duration) {
	if !r.longStarted {
		r.longStarted, r.longStart = true, now
		r.longTicker.SleepUntil(now + longPressDelay)
		return
	}
	if now-r.longStart < longPressDelay {
		r.longTicker.SleepUntil(r.longStart + longPressDelay)
		return
	}
	r.longTicker.Stop()
	if r.pressed && !r.dragging && r.OnLongPress != nil {
		r.longFired = true
		r.OnLongPress(r.downPos, r.longDevice)
	}
}

// HandlePointer implements core.PointerTarget.
func (r *RenderGesture) HandlePointer(e *core.PointerEvent) {
	if e.Kind == core.PointerPinch {
		if r.OnPinch != nil {
			e.Handled = true
			r.OnPinch(e.Pinch, e.Pos)
		}
		return
	}
	if e.Button == input.ButtonSecondary && r.OnSecondaryTapAt != nil {
		r.secondary(e)
		return
	}
	if !r.active() || e.Button != input.ButtonPrimary {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		r.pressed, r.dragging, r.longFired = true, false, false
		r.downPos, r.lastPos = e.Pos, e.Pos
		r.double = r.OnDoubleTap != nil && e.Clicks > 1 && e.Clicks%2 == 0
		e.Handled = true
		if r.OnDown != nil {
			r.OnDown()
		}
		if r.OnDownAt != nil {
			r.OnDownAt(Tap{Pos: e.Pos, Modifiers: e.Modifiers})
		}
		r.startLongPress(e.Device)
	case core.PointerMove:
		if r.pressed {
			r.move(e.Pos)
		}
	case core.PointerUp:
		if !r.pressed {
			return
		}
		r.pressed = false
		r.stopLongPress()
		if r.dragging {
			r.move(e.Pos)
			r.endDrag(e.Pos)
			return
		}
		if r.longFired {
			r.longFired = false
			return
		}
		sz := r.Size()
		inside := e.Pos.X >= 0 && e.Pos.Y >= 0 && e.Pos.X < sz.W && e.Pos.Y < sz.H
		switch {
		case inside && r.double:
			r.OnDoubleTap()
		case inside:
			if r.OnTap != nil {
				r.OnTap()
			}
			if r.OnTapAt != nil {
				r.OnTapAt(Tap{Pos: e.Pos, Modifiers: e.Modifiers})
			}
		case r.OnCancel != nil:
			r.OnCancel()
		}
	case core.PointerCancel:
		r.stopLongPress()
		if !r.pressed {
			return
		}
		r.pressed = false
		if r.dragging {
			r.endDrag(r.lastPos)
			return
		}
		if r.OnCancel != nil {
			r.OnCancel()
		}
	}
}

// move follows the pointer while the primary button is down: past the slop,
// the press is no long press, and becomes a drag if the object takes drags.
func (r *RenderGesture) move(pos geom.Offset) {
	if r.dragging {
		delta := pos.Sub(r.lastPos)
		r.lastPos = pos
		if r.OnDragUpdate != nil && delta != (geom.Offset{}) {
			r.OnDragUpdate(pos, delta)
		}
		return
	}
	r.lastPos = pos
	if r.longFired || pos.Sub(r.downPos).Length() <= dragSlop {
		return
	}
	r.stopLongPress()
	if !r.drags() {
		return
	}
	r.dragging = true
	if r.OnCancel != nil {
		r.OnCancel()
	}
	if r.OnDragStart != nil {
		r.OnDragStart(r.downPos)
	}
	if r.OnDragUpdate != nil {
		r.OnDragUpdate(pos, pos.Sub(r.downPos))
	}
}

// endDrag ends the drag in progress at pos.
func (r *RenderGesture) endDrag(pos geom.Offset) {
	r.dragging = false
	if r.OnDragEnd != nil {
		r.OnDragEnd(pos)
	}
}

// secondary turns presses of the secondary button into OnSecondaryTapAt.
func (r *RenderGesture) secondary(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		r.pressedSecondary = true
		e.Handled = true
	case core.PointerUp:
		if !r.pressedSecondary {
			return
		}
		r.pressedSecondary = false
		sz := r.Size()
		if e.Pos.X >= 0 && e.Pos.Y >= 0 && e.Pos.X < sz.W && e.Pos.Y < sz.H {
			r.OnSecondaryTapAt(Tap{Pos: e.Pos, Modifiers: e.Modifiers})
		}
	case core.PointerCancel:
		r.pressedSecondary = false
	}
}

// RenderMouseRegion reports the pointer entering, moving over and leaving, and
// sets the cursor over its child.
type RenderMouseRegion struct {
	RenderProxy
	OnEnter func()
	OnExit  func()
	// OnHover gets the position of the pointer, relative to the object, on
	// every move over it.
	OnHover func(pos geom.Offset)
	Cursor  input.Cursor
}

// HandleHover implements core.HoverTarget.
func (r *RenderMouseRegion) HandleHover(inside bool) {
	switch {
	case inside && r.OnEnter != nil:
		r.OnEnter()
	case !inside && r.OnExit != nil:
		r.OnExit()
	}
}

// HandleHoverMove implements core.HoverMoveTarget.
func (r *RenderMouseRegion) HandleHoverMove(pos geom.Offset) {
	if r.OnHover != nil {
		r.OnHover(pos)
	}
}

// MouseCursor implements core.CursorSource.
func (r *RenderMouseRegion) MouseCursor() input.Cursor { return r.Cursor }

// RenderFocus lets its child take part in keyboard focus.
type RenderFocus struct {
	RenderProxy
	// Enabled says whether the object takes focus.
	Enabled bool
	// OnKey reacts to key presses. It returns false to pass them on.
	OnKey func(e *core.KeyEvent) bool
	// OnFocusChange reports gaining and losing focus.
	OnFocusChange func(focused bool)
	// Autofocus takes the focus at the first layout, unless an object below
	// took it while it was laid out: a text field whose controller asked for
	// the keyboard before it was built, or one with its own Autofocus, keeps
	// it, as the more specific request. See
	// docs/ADR/0126-dialog-focus-submit-on-enter-after.md.
	Autofocus bool
	// OnAutofocus reports what had the focus before Autofocus took it, or nil.
	// It also runs when an object below took the focus instead.
	OnAutofocus func(previous core.FocusTarget)
	// OnShowFocus reports whether the focus should show now: the object has it,
	// and the keyboard rather than the pointer gave it (core.Tree.FocusVisible).
	OnShowFocus func(show bool)

	autofocused bool
	shown       bool
}

// Layout implements core.RenderObject.
func (r *RenderFocus) Layout(c Constraints) geom.Size {
	first := r.Autofocus && !r.autofocused
	var previous core.FocusTarget
	if first {
		// What had the focus before the children were laid out, as one of
		// them may take it.
		previous = r.Focused()
	}
	sz := r.RenderProxy.Layout(c)
	if first {
		r.autofocused = true
		if now := r.Focused(); now == previous || !holds(r, now) {
			r.RequestFocus()
		}
		if r.OnAutofocus != nil {
			r.OnAutofocus(previous)
		}
	}
	return sz
}

// holds reports whether target is ro or below it.
func holds(ro core.RenderObject, target core.FocusTarget) bool {
	if target == nil {
		return false
	}
	for p := core.RenderObject(target); p != nil; p = p.Parent() {
		if p == ro {
			return true
		}
	}
	return false
}

// CanFocus implements core.FocusTarget.
func (r *RenderFocus) CanFocus() bool { return r.Enabled }

// HandleKey implements core.FocusTarget.
func (r *RenderFocus) HandleKey(e *core.KeyEvent) bool { return r.OnKey != nil && r.OnKey(e) }

// HandleText implements core.FocusTarget. Only text fields use text.
func (r *RenderFocus) HandleText(string) {}

// HandleComposition implements core.FocusTarget.
func (r *RenderFocus) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *RenderFocus) FocusChanged(focused bool) {
	if r.OnFocusChange != nil {
		r.OnFocusChange(focused)
	}
	r.showFocus(focused && r.FocusVisible())
}

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *RenderFocus) FocusVisibleChanged(visible bool) { r.showFocus(visible) }

func (r *RenderFocus) showFocus(show bool) {
	if r.shown == show {
		return
	}
	r.shown = show
	if r.OnShowFocus != nil {
		r.OnShowFocus(show)
	}
}

// RenderFocusOrder sets the place of the objects below it in the order of Tab.
// See core.FocusOrderer.
type RenderFocusOrder struct {
	RenderProxy
	Order int
}

// FocusOrder implements core.FocusOrderer.
func (r *RenderFocusOrder) FocusOrder() int { return r.Order }

// RenderListener hands every pointer event over its child to OnPointer, with
// the device and the pressure of a pen: what a drawing needs. It takes the
// presses of every button, and the moves and release that follow: it marks a
// PointerDown handled before OnPointer runs, which can clear e.Handled to pass
// the press on to the objects around it.
type RenderListener struct {
	RenderProxy
	OnPointer func(e *core.PointerEvent)
}

// HandlePointer implements core.PointerTarget.
func (r *RenderListener) HandlePointer(e *core.PointerEvent) {
	if r.OnPointer == nil {
		return
	}
	if e.Kind == core.PointerDown {
		e.Handled = true
	}
	r.OnPointer(e)
}
