package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
)

// GestureDetector reports taps, double taps, long presses, drags and pinches
// on its child.
//
// It takes the primary button when any of its tap, long press or drag
// callbacks is set, and the secondary one with OnSecondaryTapAt. The
// callbacks that end in At also tell the modifier keys held, so that a
// Shift+click needs no Listener. A press that
// something inside takes first, such as a button, does not reach it, nor does
// the drag that follows. The press it takes gives the keyboard to the nearest
// Focus around it, as a click on a button does.
//
// A finger that presses a detector without drag callbacks may still scroll
// the Scroll around it; with OnDragStart, OnDragUpdate or OnDragEnd set, the
// detector keeps the finger and drags instead.
//
// The places the callbacks get are relative to the child. To turn one into
// window coordinates, keep the BuildContext of the widget that builds the
// detector and, in the callback, call core.LocalToGlobal(ctx.RenderObject(),
// pos): the first render object below that widget is the detector's, of the
// size and place of the child. core.GlobalToLocal with the render object of
// another widget turns the result into its coordinates, as for a drag that
// ends over a widget beside the one dragged, and core.LocalRectToGlobal does
// the same for a rectangle. All three follow the transforms on the way.
type GestureDetector struct {
	// OnTapDown runs when the primary button goes down on the child.
	OnTapDown func()
	// OnTapDownAt is OnTapDown with the place of the press, relative to the
	// child, and the modifier keys held.
	OnTapDownAt func(t layout.Tap)
	// OnTap runs when the button comes up on the child again, unless the press
	// became a drag or a long press.
	OnTap func()
	// OnTapAt is OnTap with the place of the release, relative to the child,
	// and the modifier keys held, as for a list where Shift with a click
	// selects a range and Ctrl with a click adds a row. When both are set,
	// OnTap runs first.
	OnTapAt func(t layout.Tap)
	// OnDoubleTap runs in place of OnTap on the release of the second press of
	// a double click. OnTap does not wait for it: the first press of a double
	// click is a tap at once, so a double click runs OnTap, then OnDoubleTap,
	// as a list selects a file and then opens it. Without OnDoubleTap, every
	// press is a tap.
	OnDoubleTap func()
	// OnTapCancel runs when the button comes up somewhere else, or the press
	// becomes a drag.
	OnTapCancel func()
	// OnSecondaryTapAt runs when the secondary button goes down and comes up
	// on the child, with the place of the release relative to the child and
	// the modifier keys held. A ContextMenu uses it.
	OnSecondaryTapAt func(t layout.Tap)
	// OnPinch runs when two fingers pinch on a touchpad over the child, or Ctrl
	// and the wheel turn, with the change of scale since the last call, above
	// 1 for apart, and the place relative to the child.
	OnPinch func(factor float32, focus geom.Offset)
	// OnLongPress runs when the primary button or a finger stays down on the
	// child for half a second without moving, with the place and what
	// pressed; the release is then no tap, and the press no drag.
	OnLongPress func(pos geom.Offset, device input.PointerDevice)
	// OnDragStart runs when the pointer, with the primary button down, moves
	// more than 8 pixels from where it went down, with that place: a press
	// that moves less is a tap. OnDragUpdate follows at once with the move so
	// far, then with each move: the place, and the change since the last call.
	// OnDragEnd runs with the last place when the button comes up or the press
	// is canceled. Places are relative to the child, and the pointer may leave
	// it. This is a drag inside the app; Draggable drags data to other places
	// through the system.
	OnDragStart  func(pos geom.Offset)
	OnDragUpdate func(pos, delta geom.Offset)
	OnDragEnd    func(pos geom.Offset)
	Child        core.Widget
}

// Key implements core.Widget.
func (GestureDetector) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (g GestureDetector) CreateRenderObject() core.RenderObject {
	r := &layout.RenderGesture{}
	g.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (g GestureDetector) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderGesture)
	r.OnDown, r.OnDownAt, r.OnTap, r.OnTapAt = g.OnTapDown, g.OnTapDownAt, g.OnTap, g.OnTapAt
	r.OnDoubleTap, r.OnCancel = g.OnDoubleTap, g.OnTapCancel
	r.OnSecondaryTapAt, r.OnPinch, r.OnLongPress = g.OnSecondaryTapAt, g.OnPinch, g.OnLongPress
	r.OnDragStart, r.OnDragUpdate, r.OnDragEnd = g.OnDragStart, g.OnDragUpdate, g.OnDragEnd
}

// ChildWidget implements core.SingleChildWidget.
func (g GestureDetector) ChildWidget() core.Widget { return g.Child }

// MouseRegion reports the pointer entering, moving over and leaving its child,
// and sets the mouse cursor over it. It takes no press: what is inside still
// gets the clicks.
type MouseRegion struct {
	OnEnter func()
	OnExit  func()
	// OnHover gets the position of the pointer, relative to the region, on
	// every move over it, with a button held or not, starting with the move
	// that enters, after OnEnter. It follows the pointer as a tooltip over a
	// chart does; a Listener gets the moves only of a press it took.
	OnHover func(pos geom.Offset)
	Cursor  input.Cursor
	Child   core.Widget
}

// Key implements core.Widget.
func (MouseRegion) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (m MouseRegion) CreateRenderObject() core.RenderObject {
	return &layout.RenderMouseRegion{OnEnter: m.OnEnter, OnExit: m.OnExit, OnHover: m.OnHover, Cursor: m.Cursor}
}

// UpdateRenderObject implements core.RenderWidget.
func (m MouseRegion) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderMouseRegion)
	r.OnEnter, r.OnExit, r.OnHover, r.Cursor = m.OnEnter, m.OnExit, m.OnHover, m.Cursor
}

// ChildWidget implements core.SingleChildWidget.
func (m MouseRegion) ChildWidget() core.Widget { return m.Child }

// Focus lets its child take the keyboard, which Tab moves between Focus widgets
// and text fields in the order of painting.
//
// A click gives it the keyboard only when something inside takes the press,
// as a GestureDetector or a Listener does: the press focuses the nearest Focus
// around what took it. A click on content that takes no press, such as a
// Container, leaves the focus where it was or, on nothing at all, takes it
// away. To focus on a click, put a GestureDetector with OnTapDown inside, even
// one that does nothing else.
type Focus struct {
	// Enabled says whether the child takes focus.
	Enabled bool
	// OnKey reacts to key presses. It returns false to pass them on.
	OnKey func(e *core.KeyEvent) bool
	// OnFocusChange reports gaining and losing focus.
	OnFocusChange func(focused bool)
	// Autofocus takes the focus when the widget first appears.
	Autofocus bool
	// OnAutofocus reports what had the focus before Autofocus took it, or nil.
	OnAutofocus func(previous core.FocusTarget)
	// OnShowFocus reports whether the focus should show: the child has it, and
	// the keyboard gave it rather than the pointer. Show a focus ring with it.
	OnShowFocus func(show bool)
	Child       core.Widget
}

// Key implements core.Widget.
func (Focus) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f Focus) CreateRenderObject() core.RenderObject {
	return &layout.RenderFocus{
		Enabled:       f.Enabled,
		OnKey:         f.OnKey,
		OnFocusChange: f.OnFocusChange,
		Autofocus:     f.Autofocus,
		OnAutofocus:   f.OnAutofocus,
		OnShowFocus:   f.OnShowFocus,
	}
}

// UpdateRenderObject implements core.RenderWidget.
func (f Focus) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFocus)
	r.Enabled, r.OnKey, r.OnFocusChange, r.OnShowFocus = f.Enabled, f.OnKey, f.OnFocusChange, f.OnShowFocus
}

// ChildWidget implements core.SingleChildWidget.
func (f Focus) ChildWidget() core.Widget { return f.Child }

// Listener hands the pointer events over its child to OnPointer, relative to
// the child, with the device and the pressure of a pen, for drawing.
//
// It takes a press of any button that nothing inside takes first, and then
// gets the moves and the release of that press wherever the pointer goes;
// what is around it, such as the drag of an InteractiveViewer, does not get
// them. A press that a child takes, as a button inside does, never reaches it,
// nor do the moves and the release that follow: a press goes to the deepest
// object that takes it, and to that one alone. Wheel and pinch events reach it
// unless something inside handles them, and moves without a press never do;
// MouseRegion reports the pointer coming and going.
//
// To pass a press on, OnPointer clears e.Handled on the PointerDown; the moves
// and the release of that press then go to whoever takes it, such as the item
// of a ReorderableList around it, and OnPointer sees no more of that press.
type Listener struct {
	OnPointer func(e *core.PointerEvent)
	Child     core.Widget
}

// Key implements core.Widget.
func (Listener) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l Listener) CreateRenderObject() core.RenderObject {
	return &layout.RenderListener{OnPointer: l.OnPointer}
}

// UpdateRenderObject implements core.RenderWidget.
func (l Listener) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderListener).OnPointer = l.OnPointer
}

// ChildWidget implements core.SingleChildWidget.
func (l Listener) ChildWidget() core.Widget { return l.Child }
