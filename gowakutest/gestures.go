package gowakutest

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// dragSteps is the number of moves between the press and the release of a
// drag, one a frame, so that a drag takes about 160 ms of the clock.
const dragSteps = 10

// dropEffects is what a drop from a file manager allows.
const dropEffects = input.DropCopy | input.DropMove | input.DropLink

// Hover moves the pointer over the middle of the node labeled label, found as
// Tap finds it, without pressing, then settles: what highlights a row, pauses
// a carousel or arms a tooltip. A tooltip opens only after its delay, which
// Settle does not wait for: Advance past it.
func (tt *Tester) Hover(label string) {
	tt.t.Helper()
	tt.HoverAt(tt.target("Hover", label))
}

// HoverAt moves the pointer to pos, in logical pixels from the top left,
// without pressing, then settles. A pos away from what was hovered moves the
// pointer off it; Unhover takes the pointer out of the window.
func (tt *Tester) HoverAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("HoverAt", pos)
	tt.pointerTime += pressTime
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Modifiers: tt.held, Time: tt.pointerTime})
	tt.Pump()
	tt.Settle()
}

// Unhover takes the pointer out of the window, as when the mouse leaves it, so
// that nothing is hovered any more, then settles.
func (tt *Tester) Unhover() {
	tt.t.Helper()
	tt.tree.PointerLeft()
	tt.Pump()
	tt.Settle()
}

// Scroll turns the wheel over the middle of the node labeled label, found as
// Tap finds it, then settles. dx and dy are notches with the signs of
// core.PointerEvent.Scroll: positive dy turns the wheel away from the user,
// which moves a list toward its top, and negative dy toward the user, toward
// the bottom; positive dx moves the view to the right. Scroll(label, 0, -3, 0)
// scrolls a list down by three notches. mods are the keys held while the
// wheel turns, on top of what Hold keeps down: with input.ModControl the wheel
// zooms what takes a pinch, in where dy is positive, as core.PointerPinch
// says.
func (tt *Tester) Scroll(label string, dx, dy float32, mods input.Modifiers) {
	tt.t.Helper()
	tt.ScrollAt(tt.target("Scroll", label), dx, dy, mods)
}

// ScrollAt turns the wheel at pos, in logical pixels from the top left, as
// Scroll does, with the same signs, then settles. The pointer moves to pos
// first, as it would be there for the wheel to reach what is under it.
func (tt *Tester) ScrollAt(pos geom.Offset, dx, dy float32, mods input.Modifiers) {
	tt.t.Helper()
	tt.atWindow("ScrollAt", pos)
	mods |= tt.held
	tt.pointerTime += pressTime
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Modifiers: mods, Time: tt.pointerTime})
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: pos, Modifiers: mods,
		Scroll: geom.Offset{X: dx, Y: dy}, Time: tt.pointerTime})
	tt.Pump()
	tt.Settle()
}

// Drag presses the primary button over the middle of the node labeled label,
// found as Tap finds it, moves the pointer by by, and releases it there, then
// settles: what moves the thumb of a slider, a divider of a split view, or a
// row to another place.
func (tt *Tester) Drag(label string, by geom.Offset) {
	tt.t.Helper()
	from := tt.target("Drag", label)
	tt.DragAt(from, from.Add(by))
}

// DragAt presses the primary button at from, moves the pointer to to in even
// steps, one a frame, and releases it there, then settles. The clock moves on
// with the frames, by about 160 ms in all, as a quick drag by hand takes.
// What Hold keeps down stays down the whole way, as in a Ctrl-drag. It is
// PressAt, ten MoveTo and Release; call those to look at the drag on the way.
func (tt *Tester) DragAt(from, to geom.Offset) {
	tt.t.Helper()
	tt.atWindow("DragAt", from)
	tt.PressAt(from)
	for i := 1; i <= dragSteps; i++ {
		f := float32(i) / dragSteps
		tt.MoveTo(geom.Offset{X: from.X + (to.X-from.X)*f, Y: from.Y + (to.Y-from.Y)*f})
	}
	tt.Release()
}

// Press presses the primary button over the middle of the node labeled label,
// found as Tap finds it, and keeps it down, to hold a drag in progress: MoveTo
// moves the pointer, a Screenshot, Find or Warnings looks at the drag on the
// way, and Release lets go.
//
//	tt.Press("Divider")
//	tt.MoveTo(geom.Offset{X: 300, Y: 200})
//	img := tt.Screenshot() // the divider while it is dragged
//	tt.Release()
//
// It fails the test if the button is down already.
func (tt *Tester) Press(label string) {
	tt.t.Helper()
	tt.PressAt(tt.target("Press", label))
}

// PressAt presses the primary button at pos, in logical pixels from the top
// left, and keeps it down, as Press does. It draws one frame and does not
// settle or move the clock, so that how long the button is held is up to the
// test: Advance holds it still, as for a long press. What Hold keeps down
// goes with the press, the moves and the release.
func (tt *Tester) PressAt(pos geom.Offset) {
	tt.t.Helper()
	tt.atWindow("PressAt", pos)
	if tt.pressed {
		tt.t.Fatal("gowakutest: PressAt while the button is down; Release it first")
	}
	tt.pressed = true
	tt.pointer = pos
	tt.pointerTime += clickGap
	mods := tt.held
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Modifiers: mods, Time: tt.pointerTime})
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: input.ButtonPrimary, Modifiers: mods, Time: tt.pointerTime})
	tt.Pump()
}

// MoveTo moves the pointer to pos, in logical pixels from the top left, in
// one step, with the button down that Press or PressAt holds, and moves the
// clock on by one frame, FrameTime, drawing it. Call it as many times as the
// drag needs steps: a drag that a widget must see cross a threshold or pass
// over a target needs a step there. It fails the test if no button is down.
func (tt *Tester) MoveTo(pos geom.Offset) {
	tt.t.Helper()
	if !tt.pressed {
		tt.t.Fatal("gowakutest: MoveTo without a button down; call Press or PressAt first")
	}
	tt.pointer = pos
	tt.pointerTime += uint32(FrameTime.Milliseconds())
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Button: input.ButtonPrimary, Modifiers: tt.held, Time: tt.pointerTime})
	tt.Advance(FrameTime)
}

// Release lets go of the button that Press or PressAt holds, where the
// pointer is, then settles. It fails the test if no button is down.
func (tt *Tester) Release() {
	tt.t.Helper()
	if !tt.pressed {
		tt.t.Fatal("gowakutest: Release without a button down; call Press or PressAt first")
	}
	tt.pressed = false
	tt.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: tt.pointer, Button: input.ButtonPrimary, Modifiers: tt.held, Time: tt.pointerTime})
	tt.Pump()
	tt.Settle()
}

// DropFiles drops files, by their paths, on the middle of the node labeled
// label, found as Tap finds it, as a file manager drops them, then settles. It
// returns what the drop did: input.DropNone if nothing there took it.
//
// An interact.Draggable of the tree itself starts a drag through the OS, which
// a Tester has no host for, so dragging it does nothing: a test drops its Data
// on the interact.DragTarget with DropFiles or DropText instead.
func (tt *Tester) DropFiles(label string, paths ...string) input.DropEffect {
	tt.t.Helper()
	return tt.DropFilesAt(tt.target("DropFiles", label), paths...)
}

// DropFilesAt drops files at pos, in logical pixels from the top left, as
// DropFiles does.
func (tt *Tester) DropFilesAt(pos geom.Offset, paths ...string) input.DropEffect {
	tt.t.Helper()
	tt.atWindow("DropFilesAt", pos)
	return tt.drop(pos, input.DragData{Files: paths})
}

// DropText drops text on the middle of the node labeled label, found as Tap
// finds it, as another program drops a selection, then settles. It returns
// what the drop did.
func (tt *Tester) DropText(label, s string) input.DropEffect {
	tt.t.Helper()
	return tt.DropTextAt(tt.target("DropText", label), s)
}

// DropTextAt drops text at pos, in logical pixels from the top left, as
// DropText does.
func (tt *Tester) DropTextAt(pos geom.Offset, s string) input.DropEffect {
	tt.t.Helper()
	tt.atWindow("DropTextAt", pos)
	return tt.drop(pos, input.DragData{Text: s})
}

// drop brings data over pos from outside the window and drops it, as the OS
// delivers a drag from another program: the drag first passes over the place
// with only the kinds of data, then drops with the data. The source allows
// every effect, as a file manager does.
func (tt *Tester) drop(pos geom.Offset, data input.DragData) input.DropEffect {
	tt.t.Helper()
	e := core.DragEvent{Pos: pos, Kinds: data.Kinds(), Allowed: dropEffects, Modifiers: tt.held}
	tt.tree.DispatchDrag(core.DragOver, e)
	tt.Pump()
	e.Data = data
	effect := tt.tree.DispatchDrag(core.DragDrop, e)
	tt.Pump()
	tt.Settle()
	return effect
}
