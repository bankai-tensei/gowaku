package platform

import (
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// EventKind identifies what happened.
type EventKind uint8

// The event kinds. EventNone means WaitEvent timed out.
const (
	EventNone EventKind = iota
	EventWake
	EventRedraw
	EventResize
	// EventClose asks for the window to close, from the user through the
	// window system. The application may refuse: the window stays until the
	// application closes it.
	EventClose
	EventPointerMove
	EventPointerDown
	EventPointerUp
	EventPointerLeave
	EventScroll
	EventKeyDown
	EventKeyUp
	EventText
	EventComposition
	EventFocus
	EventBlur
	// EventBrightness means that the OS changed between light and dark. It
	// concerns every window, and Window is zero.
	EventBrightness
	// EventSettings means that an OS setting other than the brightness changed,
	// such as ReduceMotion. It concerns every window, and Window is zero.
	EventSettings
	// EventAccessibilityAction asks for Action to be done to Node, from a
	// screen reader. Text, Number and Selection come with ActionSetValue and
	// ActionSetSelection.
	EventAccessibilityAction
	// EventPinch is a pinch on a touchpad at X and Y, by the factor Pinch.
	EventPinch
	// EventDestroyed means that someone other than the application destroyed
	// the window. It cannot be refused: the application forgets the window,
	// and still calls Close.
	EventDestroyed
)

// String returns the name of the kind.
func (k EventKind) String() string {
	switch k {
	case EventNone:
		return "none"
	case EventWake:
		return "wake"
	case EventRedraw:
		return "redraw"
	case EventResize:
		return "resize"
	case EventClose:
		return "close"
	case EventPointerMove:
		return "pointer-move"
	case EventPointerDown:
		return "pointer-down"
	case EventPointerUp:
		return "pointer-up"
	case EventPointerLeave:
		return "pointer-leave"
	case EventScroll:
		return "scroll"
	case EventKeyDown:
		return "key-down"
	case EventKeyUp:
		return "key-up"
	case EventText:
		return "text"
	case EventComposition:
		return "composition"
	case EventFocus:
		return "focus"
	case EventBlur:
		return "blur"
	case EventBrightness:
		return "brightness"
	case EventSettings:
		return "settings"
	case EventAccessibilityAction:
		return "accessibility-action"
	case EventPinch:
		return "pinch"
	case EventDestroyed:
		return "destroyed"
	}
	return "unknown"
}

// Event is one window system notification. It is a plain struct so that
// delivering it does not allocate.
//
// Events can still be queued for a window that was closed in the meantime;
// ignore IDs that are no longer known.
type Event struct {
	Kind EventKind
	// Window is the target. It is zero for EventNone and EventWake.
	Window WindowID
	// PhysicalWidth and PhysicalHeight are set for EventResize. Both are zero
	// while the window is minimized.
	PhysicalWidth, PhysicalHeight int
	// Scale is set for EventResize. It is physical pixels per logical pixel.
	Scale float32

	// X and Y are the pointer in physical pixels of the client area.
	X, Y float32
	// Button is set for EventPointerDown and EventPointerUp.
	Button input.Button
	// Device is what moves the pointer, PointerID tells the fingers of a touch
	// screen apart, and Pressure is that of a pen, from 0 to 1, or 0 where it
	// is not known.
	Device    input.PointerDevice
	PointerID uint32
	Pressure  float32
	// ScrollX and ScrollY are set for EventScroll, in wheel notches.
	ScrollX, ScrollY float32
	// Pinch is set for EventPinch: the change of scale since the last one.
	Pinch float32
	// Key and Repeat are set for EventKeyDown and EventKeyUp.
	Key    input.KeyCode
	Repeat bool
	// Modifiers is set for pointer and key events.
	Modifiers input.Modifiers
	// Time is a millisecond clock for pointer and key events. It wraps around.
	Time uint32
	// Text is what EventText adds to the document: typed or committed by an IME.
	Text string
	// Composition is set for EventComposition. Nil means the composition ended.
	Composition *input.Composition

	// Node and Action are set for EventAccessibilityAction.
	Node   semantics.ID
	Action semantics.Actions
	// Number is the value of an ActionSetValue on a range.
	Number float64
	// SelectionStart and SelectionEnd are the range of an ActionSetSelection,
	// in UTF-16 code units.
	SelectionStart, SelectionEnd int
}
