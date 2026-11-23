package x11

import (
	"encoding/binary"
	"os"
	"strings"
	"unsafe"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
	"github.com/bankai-tensei/gowaku/platform/internal/frame"
	"github.com/bankai-tensei/gowaku/platform/internal/keysym"
)

const (
	defaultWidth  = 800
	defaultHeight = 600

	windowEvents = KeyPressMask | KeyReleaseMask | ButtonPressMask | ButtonReleaseMask |
		EnterWindowMask | LeaveWindowMask | PointerMotionMask | ExposureMask |
		StructureNotifyMask | FocusChangeMask | PropertyChangeMask

	// lookupBuffer holds the text of one key press. An input method can commit a
	// longer text, and gets a bigger buffer then.
	lookupBuffer = 256
)

// Window is the X11 implementation of platform.Window.
type Window struct {
	platform *Platform
	// id is zero once the window is gone.
	xid XID
	id  platform.WindowID

	physicalWidth, physicalHeight int
	mapped                        bool

	ic          uintptr
	imeEnabled  bool
	focused     bool
	keysDown    [256]bool
	buttonsDown uint8
	// leftWhileDown means that the pointer left during a press, which only counts
	// when the press ends.
	leftWhileDown bool

	lookup [lookupBuffer]byte
	keysym uintptr
	status int32

	// a11y is the frame of the window in the AT-SPI tree, and originX and
	// originY where the window is on the root, for its extents. They are
	// read on the UI goroutine.
	a11y             *dbus.A11yWindow
	originX, originY int32

	// customFrame means the application draws the title bar, maximized is
	// what the window manager last said, and edge is the edge under the
	// pointer. cursor is what the application asked for, and pressRootX and
	// pressRootY where the last press was on the root, for moving.
	// drop is the drag over the window from some source, with the place and
	// effects of its last position, and dropHandler what answers it.
	drop         dropState
	dropX, dropY int32
	dropAllowed  input.DropEffect
	dropHandler  func(e *platform.DropEvent) input.DropEffect

	// pinchScale is the scale of the pinch in progress, or zero.
	pinchScale float64

	// imeSecure keeps the input method away while a password field has the
	// keyboard, whatever imeEnabled says.
	imeSecure bool

	// state is the icon, bounds and states of the window.
	state windowState

	customFrame            bool
	maximized              bool
	edge                   frame.Edge
	cursor                 input.Cursor
	pressRootX, pressRootY int32
}

var _ platform.Window = (*Window)(nil)

var _ platform.SecureInputWindow = (*Window)(nil)

// Accessibility implements platform.Window: the AT-SPI bridge (ADR 0043).
func (w *Window) Accessibility() platform.Accessibility { return w.a11y }

// a11yGeometry tells the bridge where the window is.
func (w *Window) a11yGeometry() dbus.WindowGeometry {
	return dbus.WindowGeometry{X: w.originX, Y: w.originY, Width: int32(w.physicalWidth), Height: int32(w.physicalHeight),
		Scale: w.platform.scale}
}

// updateOrigin finds where the window is on the root, while a reader may ask.
// A synthetic ConfigureNotify from the window manager already says so
// (ICCCM 4.1.5); a real one is relative to the frame of the window manager,
// so the server is asked. c is nil when there is no event.
func (w *Window) updateOrigin(c *XConfigureEvent) {
	if w.xid == 0 || !w.a11y.Listening() {
		return
	}
	if c != nil && c.SendEvent != 0 {
		w.originX, w.originY = c.X, c.Y
	} else {
		p := w.platform
		var x, y int32
		var child XID
		if call(p.x.XTranslateCoordinates, uintptr(p.display), uintptr(w.xid), uintptr(p.root), 0, 0,
			ptr(&x), ptr(&y), ptr(&child)) != 0 {
			w.originX, w.originY = x, y
		}
	}
	w.a11y.Moved()
}

// CreateWindow implements platform.Platform.
func (p *Platform) CreateWindow(opts platform.WindowOptions) (platform.Window, error) {
	width, height := opts.Width, opts.Height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	physicalWidth, physicalHeight := int(width*p.scale+0.5), int(height*p.scale+0.5)
	var id, colormap XID
	if opts.Transparent {
		id, colormap, _ = p.createWindowWithAlpha(physicalWidth, physicalHeight, windowEvents)
	}
	if id == 0 {
		// Without a visual with alpha the window is opaque, which
		// Transparent reports.
		var err error
		if id, err = p.createWindow(physicalWidth, physicalHeight, windowEvents); err != nil {
			return nil, err
		}
	}
	p.lastID++
	w := &Window{
		platform: p, xid: id, id: p.lastID, physicalWidth: physicalWidth, physicalHeight: physicalHeight,
		customFrame: opts.CustomFrame,
	}
	w.state.colormap = colormap
	w.state.minSize, w.state.maxSize = opts.MinSize, opts.MaxSize
	p.windows[id] = w

	protocols := [2]Atom{p.atoms.WM_DELETE_WINDOW, p.atoms.NET_WM_PING}
	call(p.x.XSetWMProtocols, uintptr(p.display), uintptr(id), ptr(&protocols[0]), uintptr(len(protocols)))
	pid := [1]uintptr{uintptr(os.Getpid())}
	w.setProperty(p.atoms.NET_WM_PID, XA_CARDINAL, 32, unsafe.Pointer(&pid[0]), 1)
	class := wmClass(opts.AppID)
	w.setProperty(XA_WM_CLASS, XA_STRING, 8, unsafe.Pointer(unsafe.StringData(class)), len(class))
	w.a11y = p.a11y.NewWindow(w.id, opts.Title, w.a11yGeometry)
	w.SetTitle(opts.Title)
	w.own(&opts)
	w.initialState(&opts)
	if opts.Icon != nil {
		w.SetIcon(opts.Icon)
	}
	w.xdndAware()
	w.selectPinches()
	if w.customFrame {
		w.undecorate()
	}
	w.createIC()
	if opts.Placed && opts.Owner == nil {
		w.placeHints(opts.ClientX, opts.ClientY)
		call(p.x.XMoveWindow, uintptr(p.display), uintptr(id), uintptr(int32(opts.ClientX)), uintptr(int32(opts.ClientY)))
	} else if opts.MinSize != (geom.Size{}) || opts.MaxSize != (geom.Size{}) {
		w.writeNormalHints()
	}
	call(p.x.XMapWindow, uintptr(p.display), uintptr(id))
	call(p.x.XFlush, uintptr(p.display))
	return w, nil
}

// setProperty replaces a property. data is only read during the call.
func (w *Window) setProperty(property, kind Atom, format int, data unsafe.Pointer, items int) {
	p := w.platform
	call(p.x.XChangeProperty, uintptr(p.display), uintptr(w.xid), uintptr(property), uintptr(kind), uintptr(format),
		PropModeReplace, uintptr(data), uintptr(items))
}

// ID implements platform.Window.
func (w *Window) ID() platform.WindowID { return w.id }

// PhysicalSize implements platform.Window.
func (w *Window) PhysicalSize() (width, height int) { return w.physicalWidth, w.physicalHeight }

// Scale implements platform.Window.
func (w *Window) Scale() float32 { return w.platform.scale }

// SetTitle implements platform.Window.
func (w *Window) SetTitle(title string) {
	if strings.IndexByte(title, 0) >= 0 {
		panic("gowaku: window title contains a NUL byte")
	}
	a := &w.platform.atoms
	data := unsafe.Pointer(cstring(title))
	w.setProperty(a.NET_WM_NAME, a.UTF8_STRING, 8, data, len(title))
	w.setProperty(XA_WM_NAME, a.UTF8_STRING, 8, data, len(title))
	w.a11y.SetTitle(title)
}

// SetDarkTitleBar implements platform.Window. The hint of GTK is what window
// managers read; those that do not keep their own look.
func (w *Window) SetDarkTitleBar(dark bool) {
	variant := "light"
	if dark {
		variant = "dark"
	}
	a := &w.platform.atoms
	w.setProperty(a.GTK_THEME, a.UTF8_STRING, 8, unsafe.Pointer(cstring(variant)), len(variant))
}

// NativeHandle implements platform.Window.
func (w *Window) NativeHandle() platform.NativeHandle {
	return platform.X11Handle{Display: uintptr(w.platform.display), Window: uintptr(w.xid), Transparent: w.Transparent()}
}

// SetCursor implements platform.Window.
func (w *Window) SetCursor(c input.Cursor) {
	w.cursor = c
	if w.edge != frame.None {
		// The edge keeps its cursor until the pointer leaves it.
		return
	}
	p := w.platform
	call(p.x.XDefineCursor, uintptr(p.display), uintptr(w.xid), uintptr(p.cursors[cursorIndex(c)]))
}

func cursorIndex(c input.Cursor) int {
	switch c {
	case input.CursorIBeam:
		return 1
	case input.CursorHand:
		return 2
	case input.CursorColResize:
		return 3
	case input.CursorRowResize:
		return 4
	}
	return 0
}

// Close implements platform.Window.
func (w *Window) Close() {
	if w.xid == 0 {
		return
	}
	p := w.platform
	if p.dragging != nil && p.dragging.w == w {
		p.dragCancel()
	}
	if w.ic != 0 {
		call(p.x.XDestroyIC, w.ic)
		w.ic = 0
	}
	w.a11y.Destroy()
	delete(p.windows, w.xid)
	call(p.x.XDestroyWindow, uintptr(p.display), uintptr(w.xid))
	if w.state.colormap != 0 {
		call(p.x.XFreeColormap, uintptr(p.display), uintptr(w.state.colormap))
	}
	call(p.x.XFlush, uintptr(p.display))
	w.xid = 0
}

func (w *Window) resizeEvent() platform.Event {
	ev := platform.Event{
		Kind:           platform.EventResize,
		Window:         w.id,
		PhysicalWidth:  w.physicalWidth,
		PhysicalHeight: w.physicalHeight,
		Scale:          w.platform.scale,
	}
	if !w.mapped {
		// The application stops drawing, as for a minimized window on Win32.
		ev.PhysicalWidth, ev.PhysicalHeight = 0, 0
	}
	return ev
}

func (w *Window) dispatch(ev *XEvent) {
	p := w.platform
	if w.dragEvent(ev) {
		return
	}
	switch ev.Type {
	case ConfigureNotify:
		c := as[XConfigureEvent](ev)
		if int(c.Width) != w.physicalWidth || int(c.Height) != w.physicalHeight {
			w.physicalWidth, w.physicalHeight = int(c.Width), int(c.Height)
			p.queue.Push(w.resizeEvent())
		}
		w.updateOrigin(c)
		if w.readState() {
			// Maximized shows on the title bar, and fullscreen hides it.
			p.queue.Push(w.resizeEvent())
		}
	case DestroyNotify:
		// Only someone else destroys a window that is still known here. The
		// application must hear of it, or it would wait for a window that is gone,
		// and it cannot refuse.
		p.queue.Push(platform.Event{Kind: platform.EventDestroyed, Window: w.id})
	case MapNotify:
		w.mapped = true
		w.updateOrigin(nil)
		p.queue.Push(w.resizeEvent())
	case UnmapNotify:
		w.mapped = false
		p.queue.Push(w.resizeEvent())
	case Expose:
		p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	case ClientMessage:
		w.clientMessage(as[XClientMessageEvent](ev))
	case MotionNotify:
		if !w.hoverEdge(as[XKeyEvent](ev)) {
			w.pushPointer(platform.EventPointerMove, input.ButtonPrimary, as[XKeyEvent](ev))
		}
	case ButtonPress:
		e := as[XKeyEvent](ev)
		w.pressRootX, w.pressRootY = e.XRoot, e.YRoot
		if w.pressEdge(e) {
			return
		}
		w.button(e, true)
	case ButtonRelease:
		w.button(as[XKeyEvent](ev), false)
	case EnterNotify:
		w.leftWhileDown = false
	case LeaveNotify:
		if c := as[XCrossingEvent](ev); c.Mode == NotifyNormal {
			if w.buttonsDown != 0 {
				w.leftWhileDown = true
			} else {
				p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
			}
		}
	case KeyPress:
		w.keyPress(ev)
	case KeyRelease:
		w.keyRelease(ev)
	case FocusIn, FocusOut:
		w.focus(as[XFocusChangeEvent](ev), ev.Type == FocusIn)
	}
}

func (w *Window) clientMessage(m *XClientMessageEvent) {
	p := w.platform
	a := &p.atoms
	if w.xdnd(m) || m.MessageType != a.WM_PROTOCOLS {
		return
	}
	switch Atom(m.Data[0]) {
	case a.WM_DELETE_WINDOW:
		p.queue.Push(platform.Event{Kind: platform.EventClose, Window: w.id})
	case a.NET_WM_PING:
		// Answering shows the window manager that the application lives.
		reply := *m
		reply.Window = p.root
		call(p.x.XSendEvent, uintptr(p.display), uintptr(p.root), 0, SubstructureNotifyMask|SubstructureRedirectMask, ptr(&reply))
	}
}

func (w *Window) pushPointer(kind platform.EventKind, button input.Button, e *XKeyEvent) {
	w.platform.queue.Push(platform.Event{
		Kind: kind, Window: w.id, X: float32(e.X), Y: float32(e.Y), Button: button,
		Modifiers: modifiersOf(e.State), Time: uint32(e.Time),
	})
}

func (w *Window) button(e *XKeyEvent, pressed bool) {
	if isWheel(e.Keycode) {
		// A wheel presses and releases a button for each notch.
		if pressed {
			w.wheel(e)
		}
		return
	}
	button, ok := buttonOf(e.Keycode)
	if !ok {
		return
	}
	if pressed {
		w.buttonsDown |= 1 << button
		w.pushPointer(platform.EventPointerDown, button, e)
		return
	}
	w.buttonsDown &^= 1 << button
	w.pushPointer(platform.EventPointerUp, button, e)
	if w.buttonsDown == 0 && w.leftWhileDown {
		w.leftWhileDown = false
		w.platform.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
	}
}

// wheel pushes a scroll of one notch, positive away from the user for the
// vertical wheel.
func (w *Window) wheel(e *XKeyEvent) {
	ev := platform.Event{
		Kind: platform.EventScroll, Window: w.id, X: float32(e.X), Y: float32(e.Y),
		Modifiers: modifiersOf(e.State), Time: uint32(e.Time),
	}
	switch e.Keycode {
	case wheelUp:
		ev.ScrollY = 1
	case wheelDown:
		ev.ScrollY = -1
	case wheelLeft:
		ev.ScrollX = -1
	case wheelRight:
		ev.ScrollX = 1
	}
	w.platform.queue.Push(ev)
}

func (w *Window) keyRelease(ev *XEvent) {
	p := w.platform
	e := as[XKeyEvent](ev)
	w.keysDown[e.Keycode&0xFF] = false
	key := keysym.KeyCode(w.keySym(e))
	if key != input.KeyNone {
		p.queue.Push(platform.Event{
			Kind: platform.EventKeyUp, Window: w.id, Key: key, Modifiers: modifiersOf(e.State), Time: uint32(e.Time),
		})
	}
}

// keyPress pushes the key and then its text. An input method commits through a
// press of keycode zero, which has only text.
func (w *Window) keyPress(ev *XEvent) {
	p := w.platform
	e := as[XKeyEvent](ev)
	if e.Keycode != 0 {
		// Ordinary keys ignore modifiers here, so Ctrl+Shift+Z stays a Z.
		// The keypad uses the effective modifier state to select its meaning.
		key := keysym.KeyCode(w.keySym(e))
		repeat := w.keysDown[e.Keycode&0xFF]
		w.keysDown[e.Keycode&0xFF] = true
		if key != input.KeyNone {
			p.queue.Push(platform.Event{
				Kind: platform.EventKeyDown, Window: w.id, Key: key, Modifiers: modifiersOf(e.State),
				Repeat: repeat, Time: uint32(e.Time),
			})
		}
	}
	text := w.lookupText(ev)
	if e.Keycode != 0 && e.State&(ControlMask|Mod1Mask) != 0 {
		// A shortcut, not typing.
		return
	}
	if s := keysym.TypedText(text); s != "" {
		p.queue.Push(platform.Event{Kind: platform.EventText, Window: w.id, Text: s})
	}
}

// keySym returns the unmodified symbol, except for keypad keys whose meaning
// depends on Num Lock and Shift.
func (w *Window) keySym(e *XKeyEvent) uint64 {
	p := w.platform
	plain := uint64(p.hot(p.x.XLookupKeysym, ptr(e), 0))
	if !keysym.IsKeypadNumLockKey(plain) {
		return plain
	}
	var consumed uint32
	var sym uintptr
	if p.hot(p.x.XkbLookupKeySym, uintptr(p.display), uintptr(e.Keycode), uintptr(e.State), ptr(&consumed), ptr(&sym)) != 0 {
		return uint64(sym)
	}
	return plain
}

func (w *Window) focus(e *XFocusChangeEvent, in bool) {
	// Grabs of the window manager and moves of the pointer say nothing about
	// where the keyboard goes.
	if (e.Mode != NotifyNormal && e.Mode != NotifyWhileGrabbed) || e.Detail == NotifyPointer {
		return
	}
	if w.focused == in {
		return
	}
	w.focused = in
	if !in {
		w.keysDown = [256]bool{}
	}
	w.syncICFocus()
	w.a11y.SetActive(in)
	kind := platform.EventBlur
	if in {
		kind = platform.EventFocus
	}
	w.platform.queue.Push(platform.Event{Kind: kind, Window: w.id})
}

// atomBytes encodes atoms for a property of format 32, where Xlib wants longs.
func atomBytes(atoms ...Atom) []byte {
	out := make([]byte, 0, len(atoms)*8)
	for _, a := range atoms {
		out = binary.NativeEndian.AppendUint64(out, uint64(a))
	}
	return out
}
