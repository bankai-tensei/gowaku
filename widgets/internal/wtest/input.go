package wtest

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// Center returns the middle of r.
func Center(r geom.Rect) geom.Offset { return geom.Offset{X: r.X + r.W/2, Y: r.Y + r.H/2} }

// Tap moves the pointer to at and clicks there.
func Tap(tree *core.Tree, at geom.Offset) {
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at})
}

// Wheel scrolls the wheel by notches at the top left of the window.
func Wheel(tree *core.Tree, notches float32) {
	tree.DispatchPointer(core.PointerEvent{
		Kind: core.PointerScroll, Pos: geom.Offset{X: 10, Y: 10}, Scroll: geom.Offset{Y: notches},
	})
}

// FakeHost records what the widgets ask of the platform.
type FakeHost struct {
	Ime       []bool
	Rect      geom.Rect // the last IME cursor rectangle
	Rects     int       // how often the IME cursor rectangle was set
	Cancels   int
	Clipboard string
}

// SetIMEEnabled implements core.Host.
func (h *FakeHost) SetIMEEnabled(enabled bool) { h.Ime = append(h.Ime, enabled) }

// SetIMECursorRect implements core.Host.
func (h *FakeHost) SetIMECursorRect(r geom.Rect) { h.Rect, h.Rects = r, h.Rects+1 }

// CancelComposition implements core.Host.
func (h *FakeHost) CancelComposition() { h.Cancels++ }

// SetCursor implements core.Host.
func (h *FakeHost) SetCursor(input.Cursor) {}

// ClipboardText implements core.Host.
func (h *FakeHost) ClipboardText() string { return h.Clipboard }

// SetClipboardText implements core.Host.
func (h *FakeHost) SetClipboardText(s string) { h.Clipboard = s }
