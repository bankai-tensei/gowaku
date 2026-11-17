package wayland

import (
	"encoding/binary"
	"errors"
	"image"
	"math"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
)

const (
	defaultWidth  = 800
	defaultHeight = 600
	// configureRoundtrips bounds the wait for the first configure. Compositors
	// answer in one.
	configureRoundtrips = 4
	// defaultAppID is the app_id of an application that names none.
	defaultAppID = "gowaku"
)

// Window is the Wayland implementation of platform.Window.
type Window struct {
	wlSurfaceNop
	xdgToplevelNop

	p  *Platform
	id platform.WindowID
	// dropHandler answers drags over the window.
	dropHandler func(e *platform.DropEvent) input.DropEffect

	surface    wlSurface
	xdgSurface xdgSurface
	toplevel   xdgToplevel
	decoration zxdgToplevelDecorationV1
	fractional wpFractionalScaleV1
	viewport   wpViewport

	// width and height are the size in logical pixels, and pending what the
	// compositor configures until the xdg_surface configure applies it.
	width, height               int32
	pendingWidth, pendingHeight int32
	pendingStates               []uint32
	configured                  bool
	maximized, fullscreen       bool
	suspended                   bool
	csd                         bool

	// scale is what one logical pixel is in physical ones. preferredFraction
	// and preferredBuffer are what the compositor asked for, zero until it does.
	scale             float64
	preferredFraction float64
	preferredBuffer   int32
	bufferScale       int32
	outputs           []wlOutput

	cursor input.Cursor
	// edge is the resize edge under the pointer, for windows that draw their
	// own frame.
	edge edge
	// pointerInside means that the application knows the pointer is over the
	// window, as opposed to over its resize border.
	pointerInside bool

	// imeEnabled says whether the application wants conversion, and imeRect
	// where its caret is, in logical pixels.
	imeEnabled bool
	imeRect    [4]int32
	// imeSecure keeps the text input disabled while a password field has the
	// keyboard, whatever imeEnabled says.
	imeSecure bool

	// export is the window as xdg-foreign shows it to the portal.
	export *exported

	// a11y is the frame of the window in the AT-SPI tree.
	a11y *dbus.A11yWindow

	// icons are what SetIcon was given, and icon what the toplevel shows of
	// them. minSize and maxSize bound the window geometry in logical pixels.
	// transparent means that the window asked for its alpha to show.
	icons            []*image.NRGBA
	icon             toplevelIcon
	minSize, maxSize geom.Size
	transparent      bool
}

var _ platform.Window = (*Window)(nil)

var _ platform.SecureInputWindow = (*Window)(nil)

// Accessibility implements platform.Window: the AT-SPI bridge (ADR 0043).
func (w *Window) Accessibility() platform.Accessibility { return w.a11y }

// a11yGeometry tells the bridge where the window is. Wayland does not say
// where a window is on the screen, so extents are within the window.
func (w *Window) a11yGeometry() dbus.WindowGeometry {
	width, height := w.PhysicalSize()
	return dbus.WindowGeometry{Width: int32(width), Height: int32(height), Scale: float32(w.scale)}
}

// CreateWindow implements platform.Platform.
func (p *Platform) CreateWindow(opts platform.WindowOptions) (platform.Window, error) {
	if strings.IndexByte(opts.Title, 0) >= 0 {
		panic("gowaku: window title contains a NUL byte")
	}
	if strings.IndexByte(opts.AppID, 0) >= 0 {
		panic("gowaku: app ID contains a NUL byte")
	}
	c := p.c
	width, height := int32(opts.Width), int32(opts.Height)
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	p.lastID++
	// A custom frame is drawn by the application, as where the compositor
	// offers no decorations: without a decoration object, the client draws.
	serverSide := p.decorations != 0 && !p.clientDecorations && !opts.CustomFrame
	w := &Window{p: p, id: p.lastID, width: width, height: height, scale: 1, bufferScale: 1, csd: !serverSide,
		minSize: opts.MinSize, maxSize: opts.MaxSize, transparent: opts.Transparent, icons: opts.Icon}
	w.surface = p.compositor.createSurface(c)
	p.windows[w.surface] = w
	w.a11y = p.a11y.NewWindow(w.id, opts.Title, w.a11yGeometry)
	c.listenWlSurface(w.surface, w)
	if p.fractional != 0 && p.viewporter != 0 {
		w.fractional = p.fractional.getFractionalScale(c, w.surface)
		c.listenWpFractionalScaleV1(w.fractional, w)
		w.viewport = p.viewporter.getViewport(c, w.surface)
	}
	w.xdgSurface = p.wmBase.getXdgSurface(c, w.surface)
	c.listenXdgSurface(w.xdgSurface, w)
	w.toplevel = w.xdgSurface.getToplevel(c)
	c.listenXdgToplevel(w.toplevel, w)
	w.toplevel.setTitle(c, opts.Title)
	id := opts.AppID
	if id == "" {
		id = defaultAppID
	}
	w.toplevel.setAppId(c, id)
	if opts.MinSize != (geom.Size{}) || opts.MaxSize != (geom.Size{}) {
		w.applySizeLimits()
	}
	if opts.Fullscreen {
		w.toplevel.setFullscreen(c, 0)
	}
	if w.icons != nil && p.iconManager != nil {
		w.applyIcon()
	}
	if owner, ok := opts.Owner.(*Window); ok && owner != nil && owner.toplevel != 0 {
		// The compositor keeps the window over its owner. Modality is the
		// application's to enforce: xdg-dialog-v1 is not bound yet
		// (docs/ADR/0047-modal-windows.md).
		w.toplevel.setParent(c, owner.toplevel)
	}
	if serverSide {
		w.decoration = p.decorations.getToplevelDecoration(c, w.toplevel)
		c.listenZxdgToplevelDecorationV1(w.decoration, w)
		w.decoration.setMode(c, zxdgToplevelDecorationV1ModeServerSide)
	}
	// A surface without a buffer asks for the first configure, which must come
	// before anything is drawn.
	w.surface.commit(c)
	for range configureRoundtrips {
		if w.configured {
			break
		}
		if err := c.roundtrip(); err != nil {
			w.Close()
			return nil, err
		}
	}
	if !w.configured {
		w.Close()
		return nil, errors.New("wayland: the compositor did not configure the window")
	}
	// The events so far describe the window as created.
	return w, nil
}

// ID implements platform.Window.
func (w *Window) ID() platform.WindowID { return w.id }

// Activate implements platform.Window. Wayland lets a window take the keyboard
// only with a token of xdg-activation, which gowaku does not bind yet, so it
// does nothing.
func (w *Window) Activate() {}

// PhysicalSize implements platform.Window.
func (w *Window) PhysicalSize() (width, height int) {
	return physicalSize(w.width, w.scale), physicalSize(w.height, w.scale)
}

// Scale implements platform.Window.
func (w *Window) Scale() float32 { return float32(w.scale) }

// SetTitle implements platform.Window.
func (w *Window) SetTitle(title string) {
	if strings.IndexByte(title, 0) >= 0 {
		panic("gowaku: window title contains a NUL byte")
	}
	if w.toplevel != 0 {
		w.toplevel.setTitle(w.p.c, title)
		w.a11y.SetTitle(title)
	}
}

// SetDarkTitleBar implements platform.Window. Wayland has no common way to ask;
// a title bar that the application draws follows its theme anyway.
func (w *Window) SetDarkTitleBar(bool) {}

// NativeHandle implements platform.Window.
func (w *Window) NativeHandle() platform.NativeHandle {
	return platform.WaylandHandle{Display: w.p.c.display, Surface: uintptr(w.surface), Transparent: w.transparent}
}

// ClientDecorated implements platform.Window.
func (w *Window) ClientDecorated() bool { return w.csd }

// SetHitTest implements platform.Window. The application moves the window with
// BeginMove; the compositor does not ask.
func (w *Window) SetHitTest(func(physicalX, physicalY int) platform.HitArea) {}

// Maximized implements platform.Window.
func (w *Window) Maximized() bool { return w.maximized }

// BeginMove implements platform.Window.
func (w *Window) BeginMove() {
	if s := &w.p.seat; s.proxy != 0 && w.toplevel != 0 {
		w.toplevel.move(w.p.c, s.proxy, s.serial)
	}
}

// ToggleMaximize implements platform.Window.
func (w *Window) ToggleMaximize() {
	if w.toplevel == 0 {
		return
	}
	if w.maximized {
		w.toplevel.unsetMaximized(w.p.c)
	} else {
		w.toplevel.setMaximized(w.p.c)
	}
}

// Minimize implements platform.Window.
func (w *Window) Minimize() {
	if w.toplevel != 0 {
		w.toplevel.setMinimized(w.p.c)
	}
}

// ShowWindowMenu implements platform.Window.
func (w *Window) ShowWindowMenu(physicalX, physicalY int) {
	if s := &w.p.seat; s.proxy != 0 && w.toplevel != 0 {
		w.toplevel.showWindowMenu(w.p.c, s.proxy, s.serial, int32(float64(physicalX)/w.scale), int32(float64(physicalY)/w.scale))
	}
}

// Close implements platform.Window.
func (w *Window) Close() {
	if w.surface == 0 {
		return
	}
	p, c := w.p, w.p.c
	p.seat.forget(w)
	if w.export != nil {
		w.export.proxy.destroy(c)
	}
	if w.decoration != 0 {
		w.decoration.destroy(c)
	}
	w.icon.destroy(c)
	if w.toplevel != 0 {
		w.toplevel.destroy(c)
	}
	if w.xdgSurface != 0 {
		w.xdgSurface.destroy(c)
	}
	if w.viewport != 0 {
		w.viewport.destroy(c)
	}
	if w.fractional != 0 {
		w.fractional.destroy(c)
	}
	w.a11y.Destroy()
	delete(p.windows, w.surface)
	w.surface.destroy(c)
	c.hot(c.lib.flush, c.display)
	*w = Window{p: p, id: w.id, a11y: w.a11y}
}

func (w *Window) resizeEvent() platform.Event {
	width, height := w.PhysicalSize()
	if w.suspended {
		// The window cannot be seen, and the application stops drawing, as for
		// a minimized window on Win32.
		width, height = 0, 0
	}
	return platform.Event{
		Kind: platform.EventResize, Window: w.id,
		PhysicalWidth: width, PhysicalHeight: height, Scale: float32(w.scale),
	}
}

// xdgToplevelConfigure implements xdgToplevelListener.
func (w *Window) xdgToplevelConfigure(width, height int32, states []byte) {
	w.pendingWidth, w.pendingHeight = width, height
	w.pendingStates = w.pendingStates[:0]
	for i := 0; i+4 <= len(states); i += 4 {
		w.pendingStates = append(w.pendingStates, binary.NativeEndian.Uint32(states[i:]))
	}
}

// xdgToplevelClose implements xdgToplevelListener.
func (w *Window) xdgToplevelClose() {
	w.p.queue.Push(platform.Event{Kind: platform.EventClose, Window: w.id})
}

// xdgSurfaceConfigure implements xdgSurfaceListener: what the configures before
// it described applies now.
func (w *Window) xdgSurfaceConfigure(serial uint32) {
	changed := !w.configured
	if w.pendingWidth > 0 && w.pendingHeight > 0 && (w.pendingWidth != w.width || w.pendingHeight != w.height) {
		w.width, w.height = w.pendingWidth, w.pendingHeight
		changed = true
	}
	maximized := slices.Contains(w.pendingStates, xdgToplevelStateMaximized)
	fullscreen := slices.Contains(w.pendingStates, xdgToplevelStateFullscreen)
	suspended := slices.Contains(w.pendingStates, xdgToplevelStateSuspended)
	if maximized != w.maximized || fullscreen != w.fullscreen || suspended != w.suspended {
		w.maximized, w.fullscreen, w.suspended = maximized, fullscreen, suspended
		changed = true
	}
	w.xdgSurface.ackConfigure(w.p.c, serial)
	w.configured = true
	if changed {
		w.applyScale()
		w.p.queue.Push(w.resizeEvent())
	}
}

// zxdgToplevelDecorationV1Configure implements zxdgToplevelDecorationV1Listener.
// A compositor may refuse to draw the title bar, and then the application must.
func (w *Window) zxdgToplevelDecorationV1Configure(mode uint32) {
	csd := mode != zxdgToplevelDecorationV1ModeServerSide
	if csd != w.csd {
		w.csd = csd
		if w.configured {
			w.p.queue.Push(w.resizeEvent())
		}
	}
}

// wpFractionalScaleV1PreferredScale implements wpFractionalScaleV1Listener.
func (w *Window) wpFractionalScaleV1PreferredScale(scale uint32) {
	w.preferredFraction = fractionalScale(scale)
	w.updateScale()
}

// wlSurfacePreferredBufferScale implements wlSurfaceListener.
func (w *Window) wlSurfacePreferredBufferScale(factor int32) {
	w.preferredBuffer = factor
	w.updateScale()
}

// wlSurfaceEnter implements wlSurfaceListener.
func (w *Window) wlSurfaceEnter(o wlOutput) {
	if !slices.Contains(w.outputs, o) {
		w.outputs = append(w.outputs, o)
	}
	w.updateScale()
}

// wlSurfaceLeave implements wlSurfaceListener.
func (w *Window) wlSurfaceLeave(o wlOutput) { w.leaveOutput(o) }

func (w *Window) leaveOutput(o wlOutput) {
	if i := slices.Index(w.outputs, o); i >= 0 {
		w.outputs = slices.Delete(w.outputs, i, i+1)
		w.updateScale()
	}
}

// wantedScale returns the scale that the compositor asks for, from the most
// precise way it has.
func (w *Window) wantedScale() float64 {
	switch {
	case w.fractional != 0:
		if w.preferredFraction > 0 {
			return w.preferredFraction
		}
		return 1
	case w.preferredBuffer > 0:
		return float64(w.preferredBuffer)
	}
	scale := int32(1)
	for _, o := range w.outputs {
		scale = max(scale, w.p.outputScale(o))
	}
	return float64(scale)
}

// updateScale picks up a new scale.
func (w *Window) updateScale() {
	scale := w.wantedScale()
	if scale == w.scale {
		return
	}
	w.scale = scale
	if w.configured {
		w.applyScale()
		w.p.queue.Push(w.resizeEvent())
	}
}

// applyScale tells the compositor how the buffers that come next relate to the
// logical size. It takes effect with the commit of the next frame.
func (w *Window) applyScale() {
	c := w.p.c
	if w.viewport != 0 {
		w.viewport.setDestination(c, w.width, w.height)
		return
	}
	buffer := int32(math.Round(w.scale))
	if buffer != w.bufferScale && w.p.compositorVersion >= wlSurfaceSetBufferScaleSince {
		w.bufferScale = buffer
		w.surface.setBufferScale(c, buffer)
	}
}

// focusChanged reports the keyboard focus.
func (w *Window) focusChanged(focused bool) {
	w.a11y.SetActive(focused)
	kind := platform.EventBlur
	if focused {
		kind = platform.EventFocus
	}
	w.p.queue.Push(platform.Event{Kind: kind, Window: w.id})
}

// resizable reports whether the window has a resize border of its own.
func (w *Window) resizable() bool { return w.csd && !w.maximized && !w.fullscreen }

func (w *Window) pointerEntered() { w.edge = edgeNone; w.pointerInside = false }

func (w *Window) pointerLeft() {
	if w.pointerInside {
		w.pointerInside = false
		w.p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
	}
	w.edge = edgeNone
}

// pointerMoved takes a motion in logical pixels of the surface. Over the resize
// border it shows a resize cursor, and the application does not see it.
func (w *Window) pointerMoved(x, y float64, stamp uint32) {
	e := edgeNone
	if w.resizable() {
		e = edgeAt(x, y, float64(w.width), float64(w.height))
	}
	if e != w.edge {
		w.edge = e
		if e != edgeNone {
			w.p.seat.cursor.set(edgeShape(e))
		} else {
			w.p.seat.cursor.set(shapeOf(w.cursor))
		}
	}
	if e != edgeNone {
		if w.pointerInside && w.p.seat.buttonsDown == 0 {
			w.pointerInside = false
			w.p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
		}
		if !w.pointerInside {
			return
		}
	}
	w.pointerInside = true
	w.p.queue.Push(platform.Event{
		Kind: platform.EventPointerMove, Window: w.id,
		X: float32(x * w.scale), Y: float32(y * w.scale),
		Modifiers: w.p.seat.kb.modifiers(), Time: stamp,
	})
}

func (w *Window) pointerButton(b input.Button, pressed bool, stamp uint32) {
	s := &w.p.seat
	if pressed && w.edge != edgeNone && !w.pointerInside {
		if b == input.ButtonPrimary {
			w.toplevel.resize(w.p.c, s.proxy, s.serial, uint32(w.edge))
		}
		return
	}
	if !w.pointerInside {
		return
	}
	kind := platform.EventPointerUp
	if pressed {
		kind = platform.EventPointerDown
	}
	w.p.queue.Push(platform.Event{
		Kind: kind, Window: w.id, Button: b,
		X: float32(s.pointerX * w.scale), Y: float32(s.pointerY * w.scale),
		Modifiers: s.kb.modifiers(), Time: stamp,
	})
}

// SetCursor implements platform.Window.
func (w *Window) SetCursor(c input.Cursor) {
	w.cursor = c
	if w.p.seat.pointerFocus == w && w.edge == edgeNone {
		w.p.seat.cursor.set(shapeOf(c))
	}
}

// SetIMEEnabled implements platform.Window. Without text-input-v3 in the
// compositor, keys still produce their text.
func (w *Window) SetIMEEnabled(enabled bool) {
	w.imeEnabled = enabled
	w.p.seat.text.sync()
}

// SetSecureInput implements platform.SecureInputWindow. The text input stays
// disabled, so that the input method gets no keys at all, rather than enabled
// with the purpose password, which an input method is free to convert anyway.
// Wayland has no secure event input: a client cannot read the keys of another.
func (w *Window) SetSecureInput(secure bool) {
	if secure == w.imeSecure {
		return
	}
	w.imeSecure = secure
	w.p.seat.text.sync()
}

// SetIMEVertical implements platform.Window. text-input-v3 has no notion of
// it: the cursor rectangle is all the IME gets.
func (w *Window) SetIMEVertical(bool) {}

// SetIMECursorRect implements platform.Window.
func (w *Window) SetIMECursorRect(physicalX, physicalY, physicalWidth, physicalHeight int) {
	s := w.scale
	r := [4]int32{
		int32(float64(physicalX) / s), int32(float64(physicalY) / s),
		int32(math.Ceil(float64(physicalWidth) / s)), int32(math.Ceil(float64(physicalHeight) / s)),
	}
	if r != w.imeRect {
		w.imeRect = r
		w.p.seat.text.rectChanged(w)
	}
}

// CancelComposition implements platform.Window.
func (w *Window) CancelComposition() { w.p.seat.text.cancel(w) }
