package x11

import (
	"fmt"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
	"github.com/bankai-tensei/gowaku/platform/internal/frame"
)

// A window with CustomFrame asks the window manager for no decorations
// (_MOTIF_WM_HINTS), and asks it to move, resize, maximize and minimize the
// window with the messages of EWMH, as GTK does for its client-side
// decorations. See docs/ADR/0048-custom-title-bar.md.

// The values of _NET_WM_MOVERESIZE.
const (
	moveResizeTopLeft     = 0
	moveResizeTop         = 1
	moveResizeTopRight    = 2
	moveResizeRight       = 3
	moveResizeBottomRight = 4
	moveResizeBottom      = 5
	moveResizeBottomLeft  = 6
	moveResizeLeft        = 7
	moveResizeMove        = 8

	// netWMStateToggle is the action of a _NET_WM_STATE message.
	netWMStateToggle = 2
	// iconicState is the state of WM_CHANGE_STATE that minimizes (ICCCM).
	iconicState = 3
	// motifDecorations is the flag of _MOTIF_WM_HINTS that sets decorations.
	motifDecorations = 2
)

// The font cursors of the edges, from cursorfont.h.
const (
	XC_bottom_left_corner  = 12
	XC_bottom_right_corner = 14
	XC_bottom_side         = 16
	XC_left_side           = 70
	XC_right_side          = 96
	XC_top_left_corner     = 134
	XC_top_right_corner    = 136
	XC_top_side            = 138
)

// edgeCursors are the font cursors of the edges, by frame.Edge.
var edgeCursors = map[frame.Edge]uintptr{
	frame.Top: XC_top_side, frame.Bottom: XC_bottom_side, frame.Left: XC_left_side, frame.Right: XC_right_side,
	frame.TopLeft: XC_top_left_corner, frame.TopRight: XC_top_right_corner,
	frame.BottomLeft: XC_bottom_left_corner, frame.BottomRight: XC_bottom_right_corner,
}

// ShowFileDialog implements platform.Window, through xdg-desktop-portal.
func (w *Window) ShowFileDialog(opts platform.FileDialogOptions, done func([]string, error)) {
	parent := fmt.Sprintf("x11:%x", uint64(w.xid))
	go func() { done(dbus.ShowFileDialog(parent, opts)) }()
}

// ClientDecorated implements platform.Window. The window manager draws the
// title bar unless the window has a custom frame.
func (w *Window) ClientDecorated() bool { return w.customFrame }

// Maximized implements platform.Window. It is what _NET_WM_STATE said at the
// last configure.
func (w *Window) Maximized() bool { return w.maximized }

// SetHitTest implements platform.Window. The application moves the window
// with BeginMove; the window manager does not ask.
func (w *Window) SetHitTest(func(physicalX, physicalY int) platform.HitArea) {}

// undecorate asks the window manager for no title bar or borders.
func (w *Window) undecorate() {
	a := &w.platform.atoms
	hints := [5]uintptr{motifDecorations, 0, 0, 0, 0}
	w.setProperty(a.MOTIF_WM_HINTS, a.MOTIF_WM_HINTS, 32, unsafe.Pointer(&hints[0]), len(hints))
}

// rootMessage sends a client message about w to the window manager.
func (w *Window) rootMessage(kind Atom, data ...uintptr) {
	p := w.platform
	m := XClientMessageEvent{MessageType: kind, Format: 32}
	m.Type, m.Display, m.Window = ClientMessage, p.display, w.xid
	copy(m.Data[:], data)
	call(p.x.XSendEvent, uintptr(p.display), uintptr(p.root), 0, SubstructureNotifyMask|SubstructureRedirectMask, ptr(&m))
	call(p.x.XFlush, uintptr(p.display))
}

// moveResize hands the pointer that is down to the window manager, which moves
// or resizes the window from there. The press stays the window manager's, so
// the release never comes.
func (w *Window) moveResize(direction uintptr) {
	if w.xid == 0 || w.buttonsDown&(1<<input.ButtonPrimary) == 0 {
		return
	}
	p := w.platform
	call(p.x.XUngrabPointer, uintptr(p.display), CurrentTime)
	w.buttonsDown = 0
	// Source 1 is an application.
	w.rootMessage(p.atoms.NET_WM_MOVERESIZE, uintptr(w.pressRootX), uintptr(w.pressRootY), direction, 1, 1)
}

// BeginMove implements platform.Window.
func (w *Window) BeginMove() {
	if w.customFrame {
		w.moveResize(moveResizeMove)
	}
}

// ToggleMaximize implements platform.Window.
func (w *Window) ToggleMaximize() {
	if w.customFrame && w.xid != 0 {
		a := &w.platform.atoms
		w.rootMessage(a.NET_WM_STATE, netWMStateToggle, uintptr(a.NET_WM_STATE_MAXIMIZED_VERT),
			uintptr(a.NET_WM_STATE_MAXIMIZED_HORZ), 1)
	}
}

// Minimize implements platform.Window.
func (w *Window) Minimize() {
	if w.customFrame && w.xid != 0 {
		w.rootMessage(w.platform.atoms.WM_CHANGE_STATE, iconicState)
	}
}

// ShowWindowMenu implements platform.Window, at the place of the last press,
// with the message of GTK that Mutter and KWin answer.
func (w *Window) ShowWindowMenu(_, _ int) {
	if w.customFrame && w.xid != 0 {
		p := w.platform
		call(p.x.XUngrabPointer, uintptr(p.display), CurrentTime)
		w.buttonsDown = 0
		w.rootMessage(p.atoms.GTK_SHOW_WINDOW_MENU, 0, uintptr(w.pressRootX), uintptr(w.pressRootY))
	}
}

// edgeAt returns the edge that the pointer at (x, y), physical, resizes.
func (w *Window) edgeAt(x, y int32) frame.Edge {
	if !w.customFrame || w.maximized {
		return frame.None
	}
	s := float64(w.platform.scale)
	return frame.At(float64(x)/s, float64(y)/s, float64(w.physicalWidth)/s, float64(w.physicalHeight)/s)
}

// hoverEdge shows the cursor of the edge under the pointer. It reports whether
// the pointer is on an edge, where the application does not get it.
func (w *Window) hoverEdge(e *XKeyEvent) bool {
	edge := w.edgeAt(e.X, e.Y)
	if edge != w.edge {
		w.edge = edge
		p := w.platform
		cursor := p.cursors[cursorIndex(w.cursor)]
		if edge != frame.None {
			cursor = p.edgeCursor(edge)
		}
		call(p.x.XDefineCursor, uintptr(p.display), uintptr(w.xid), uintptr(cursor))
	}
	return edge != frame.None && w.buttonsDown == 0
}

// edgeCursor returns the cursor of edge, made on first use.
func (p *Platform) edgeCursor(edge frame.Edge) XID {
	if c, ok := p.edgeCursors[edge]; ok {
		return c
	}
	if p.edgeCursors == nil {
		p.edgeCursors = make(map[frame.Edge]XID)
	}
	c := XID(call(p.x.XCreateFontCursor, uintptr(p.display), edgeCursors[edge]))
	p.edgeCursors[edge] = c
	return c
}

// pressEdge starts a resize for a primary press on an edge, and reports
// whether it did.
func (w *Window) pressEdge(e *XKeyEvent) bool {
	direction := map[frame.Edge]uintptr{
		frame.Top: moveResizeTop, frame.Bottom: moveResizeBottom, frame.Left: moveResizeLeft,
		frame.Right: moveResizeRight, frame.TopLeft: moveResizeTopLeft, frame.TopRight: moveResizeTopRight,
		frame.BottomLeft: moveResizeBottomLeft, frame.BottomRight: moveResizeBottomRight,
	}
	d, ok := direction[w.edgeAt(e.X, e.Y)]
	if !ok || e.Keycode != 1 {
		return false
	}
	w.buttonsDown |= 1 << input.ButtonPrimary
	w.moveResize(d)
	return true
}
