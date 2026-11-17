package platform

import "github.com/bankai-tensei/gowaku/input"

// DropPhase says where a drag over a window is.
type DropPhase uint8

// The drop phases.
const (
	// DropOver is the pointer moving over the window with data, the first
	// time included.
	DropOver DropPhase = iota
	// DropLeave is the drag leaving the window, or being canceled.
	DropLeave
	// DropDrop is the data being dropped.
	DropDrop
)

// DropEvent is a drag over a window. See docs/ADR/0050-drag-and-drop.md.
type DropEvent struct {
	Phase DropPhase
	// X and Y are the pointer in physical pixels of the client area.
	X, Y float32
	// Kinds is what the drag carries, and Data the data itself, for DropDrop
	// only.
	Kinds input.DataKinds
	Data  input.DragData
	// Allowed is what the source lets the drop do.
	Allowed   input.DropEffect
	Modifiers input.Modifiers
}

// Dragging is drag and drop with other windows and programs. A Window
// implements it.
type Dragging interface {
	// SetDropHandler gives the window the function that answers drags over
	// it with what a drop would do. The window system asks it while the drag
	// goes on, on the UI goroutine, inside WaitEvent; it must answer at once.
	SetDropHandler(fn func(e *DropEvent) input.DropEffect)
	// StartDrag drags d from the press in progress, allowing the effects in
	// allowed. The drag starts at the next WaitEvent, which runs it as a
	// modal loop on Windows, and calls done with what the drop did, DropNone
	// if it was canceled or refused.
	StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect))
}
