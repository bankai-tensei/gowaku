//go:build darwin && arm64

package cocoa

import (
	"errors"
	"math"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// Default size of a window whose options leave it out, in points.
const (
	defaultWidth  = 800
	defaultHeight = 600
)

// Window is the AppKit implementation of platform.Window.
type Window struct {
	p      *Platform
	id     platform.WindowID
	window objc.ID
	view   objc.ID
	layer  objc.ID
	owner  *Window

	width, height int
	scale         float32
	closed        bool
	// customFrame means that the application draws the title bar.
	customFrame bool
	// transparent means that the window and its layer are not opaque.
	transparent bool

	// The IME: whether it converts, where the caret is in physical pixels,
	// and the marked text it holds.
	imeEnabled bool
	caret      [4]int
	vertical   bool
	marked     string

	// secure is what SetSecureInput asked, key whether the window is the key
	// window, and secureOn whether it holds secure event input, which it
	// does only while both are set.
	secure, key, secureOn bool

	cursor input.Cursor
	// buttons are the buttons down, so that a press with Control, which is a
	// secondary press on the Mac, lifts as one.
	secondaryByControl bool
	lastMouseDown      objc.ID

	drop func(e *platform.DropEvent) input.DropEffect

	// menu is what SetMenuBar gave, and onMenu hears its items.
	menu   []platform.MenuItem
	onMenu func(id int)
}

var _ platform.Window = (*Window)(nil)

var _ platform.SecureInputWindow = (*Window)(nil)

// CreateWindow implements platform.Platform.
func (p *Platform) CreateWindow(opts platform.WindowOptions) (platform.Window, error) {
	var w *Window
	var err error
	p.libs.pool(func() { w, err = p.createWindow(opts) })
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (p *Platform) createWindow(opts platform.WindowOptions) (*Window, error) {
	width, height := float64(opts.Width), float64(opts.Height)
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	rect := NSRect{Size: NSSize{width, height}}
	style := uint64(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable)
	if opts.CustomFrame {
		style |= NSWindowStyleMaskFullSizeContentView
	}
	nsWindow := class("NSWindow").Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"),
		rect, style, uint64(NSBackingStoreBuffered), false)
	if nsWindow == 0 {
		return nil, errors.New("cocoa: create window")
	}
	nsWindow.Send(sel("setReleasedWhenClosed:"), false)
	nsWindow.Send(sel("setTitle:"), nsString(opts.Title))
	nsWindow.Send(sel("setAcceptsMouseMovedEvents:"), true)
	nsWindow.Send(sel("setTabbingMode:"), int64(2)) // NSWindowTabbingModeDisallowed
	if opts.CustomFrame {
		// The content reaches the top edge, and the application draws the
		// title bar and its buttons, as on the other systems (ADR 0048). The
		// edges still resize the window.
		nsWindow.Send(sel("setTitlebarAppearsTransparent:"), true)
		nsWindow.Send(sel("setTitleVisibility:"), int64(1)) // NSWindowTitleHidden
		// The buttons are close, miniaturize and zoom.
		for button := range int64(3) {
			nsWindow.Send(sel("standardWindowButton:"), button).Send(sel("setHidden:"), true)
		}
	}

	view := objc.ID(p.viewClass).Send(sel("alloc")).Send(sel("initWithFrame:"), rect)
	// A layer-hosting view: the layer is set before wantsLayer, and AppKit
	// leaves its contents to Metal.
	layer := class("CAMetalLayer").Send(sel("new"))
	layer.Send(sel("setOpaque:"), !opts.Transparent)
	view.Send(sel("setLayer:"), layer)
	view.Send(sel("setWantsLayer:"), true)
	options := uint64(NSTrackingMouseEnteredAndExited | NSTrackingMouseMoved | NSTrackingCursorUpdate |
		NSTrackingActiveAlways | NSTrackingInVisibleRect)
	tracking := class("NSTrackingArea").Send(sel("alloc")).Send(sel("initWithRect:options:owner:userInfo:"),
		NSRect{}, options, view, objc.ID(0)).Send(sel("autorelease"))
	view.Send(sel("addTrackingArea:"), tracking)
	registerDropTypes(view)
	nsWindow.Send(sel("setContentView:"), view)
	nsWindow.Send(sel("setDelegate:"), p.appDelegate)
	nsWindow.Send(sel("makeFirstResponder:"), view)

	w := &Window{p: p, id: p.nextID(), window: nsWindow, view: view, layer: layer, customFrame: opts.CustomFrame}
	if owner, ok := opts.Owner.(*Window); ok && owner != nil && !owner.closed {
		w.owner = owner
	}
	if opts.Transparent {
		w.transparent = true
		w.makeTransparent()
	}
	if w.owner == nil {
		nsWindow.Send(sel("setCollectionBehavior:"), uint64(NSWindowCollectionBehaviorFullScreenPrimary))
	}
	if opts.MinSize != (geom.Size{}) || opts.MaxSize != (geom.Size{}) {
		w.applySizeLimits(opts.MinSize, opts.MaxSize)
	}
	if opts.AlwaysOnTop {
		nsWindow.Send(sel("setLevel:"), int64(NSFloatingWindowLevel))
	}
	w.SetDarkTitleBar(opts.DarkTitleBar)
	w.updateSize()
	p.windows[w.id] = w
	p.byView[view] = w
	p.byWindow[nsWindow] = w

	switch {
	case opts.Placed:
		w.MoveClientTo(opts.ClientX, opts.ClientY)
	case w.owner != nil:
		w.centerOver(w.owner)
	default:
		nsWindow.Send(sel("center"))
	}
	if w.owner != nil {
		w.owner.window.Send(sel("addChildWindow:ordered:"), nsWindow, int64(NSWindowAbove))
	}
	nsWindow.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	p.activate()
	if opts.Icon != nil {
		w.SetIcon(opts.Icon)
	}
	if opts.Fullscreen {
		nsWindow.Send(sel("toggleFullScreen:"), objc.ID(0))
	}
	return w, nil
}

// centerOver places w in the middle of owner.
func (w *Window) centerOver(owner *Window) {
	of := objc.Send[NSRect](owner.window, sel("frame"))
	mine := objc.Send[NSRect](w.window, sel("frame"))
	origin := NSPoint{
		X: of.Origin.X + (of.Size.Width-mine.Size.Width)/2,
		Y: of.Origin.Y + (of.Size.Height-mine.Size.Height)/2,
	}
	w.window.Send(sel("setFrameOrigin:"), origin)
}

// updateSize reads the size and scale of the content view, and the layer
// takes the scale. It reports whether either changed.
func (w *Window) updateSize() bool {
	bounds := objc.Send[NSRect](w.view, sel("bounds"))
	scale := float32(objc.Send[float64](w.window, sel("backingScaleFactor")))
	if scale <= 0 {
		scale = 1
	}
	width := int(math.Round(bounds.Size.Width * float64(scale)))
	height := int(math.Round(bounds.Size.Height * float64(scale)))
	if width == w.width && height == w.height && scale == w.scale {
		return false
	}
	if scale != w.scale {
		w.layer.Send(sel("setContentsScale:"), float64(scale))
	}
	w.width, w.height, w.scale = width, height, scale
	return true
}

// resized reports a change of size to the application.
func (w *Window) resized() {
	if !w.updateSize() {
		return
	}
	w.pushSize()
	w.p.runModalHandler()
}

// hidden reports whether nothing of w can be seen: minimized, covered by
// other windows, or on a display that sleeps. Metal hands out drawables
// without waiting for vertical blank then, so drawing would spin.
func (w *Window) hidden() bool {
	const visible = 1 << 1 // NSWindowOcclusionStateVisible
	return objc.Send[bool](w.window, sel("isMiniaturized")) ||
		objc.Send[uint64](w.window, sel("occlusionState"))&visible == 0
}

// pushSize reports the size of w, zero while it is hidden, as the application
// stops drawing a minimized window.
func (w *Window) pushSize() {
	width, height := w.width, w.height
	if w.hidden() {
		width, height = 0, 0
	}
	w.p.enqueue(platform.Event{Kind: platform.EventResize, Window: w.id, PhysicalWidth: width, PhysicalHeight: height, Scale: w.scale})
}

// ID implements platform.Window.
func (w *Window) ID() platform.WindowID { return w.id }

// PhysicalSize implements platform.Window.
func (w *Window) PhysicalSize() (width, height int) { return w.width, w.height }

// Scale implements platform.Window.
func (w *Window) Scale() float32 { return w.scale }

// SetTitle implements platform.Window.
func (w *Window) SetTitle(title string) {
	w.p.libs.pool(func() { w.window.Send(sel("setTitle:"), nsString(title)) })
}

// SetDarkTitleBar implements platform.Window. The window takes the appearance
// of the application's theme, which may differ from that of the system.
func (w *Window) SetDarkTitleBar(dark bool) {
	name := "NSAppearanceNameAqua"
	if dark {
		name = "NSAppearanceNameDarkAqua"
	}
	w.p.libs.pool(func() {
		w.window.Send(sel("setAppearance:"), class("NSAppearance").Send(sel("appearanceNamed:"), nsString(name)))
	})
}

// NativeHandle implements platform.Window.
func (w *Window) NativeHandle() platform.NativeHandle {
	return platform.CocoaHandle{Layer: uintptr(w.layer)}
}

// SetIMEEnabled implements platform.Window. Without the IME, keys still type
// through the key bindings of AppKit.
func (w *Window) SetIMEEnabled(enabled bool) {
	if w.imeEnabled == enabled {
		return
	}
	if !enabled {
		w.CancelComposition()
	}
	w.imeEnabled = enabled
}

// SetSecureInput implements platform.SecureInputWindow. The view has no input
// context, so the IME sees nothing, and while the window is the key window
// it holds secure event input, which keeps other programs from reading the
// keys and makes the system switch to a Roman keyboard, as an
// NSSecureTextField does.
func (w *Window) SetSecureInput(secure bool) {
	if secure == w.secure {
		return
	}
	if secure {
		w.CancelComposition()
	}
	w.secure = secure
	w.syncSecureInput()
}

// converts reports whether the view gives AppKit an input context.
func (w *Window) converts() bool { return w.imeEnabled && !w.secure }

// syncSecureInput takes or gives back secure event input. The calls are
// counted by the system across the process, so each window keeps its own
// balanced.
func (w *Window) syncSecureInput() {
	want := w.secure && w.key && !w.closed
	if want == w.secureOn {
		return
	}
	w.secureOn = want
	if want {
		purego.SyscallN(w.p.libs.enableSecureInput)
	} else {
		purego.SyscallN(w.p.libs.disableSecureInput)
	}
}

// SetIMECursorRect implements platform.Window.
func (w *Window) SetIMECursorRect(physicalX, physicalY, physicalWidth, physicalHeight int) {
	w.caret = [4]int{physicalX, physicalY, physicalWidth, physicalHeight}
}

// SetIMEVertical implements platform.Window.
func (w *Window) SetIMEVertical(vertical bool) { w.vertical = vertical }

// CancelComposition implements platform.Window.
func (w *Window) CancelComposition() {
	if w.marked == "" {
		return
	}
	w.p.libs.pool(func() {
		if ctx := w.view.Send(sel("inputContext")); ctx != 0 {
			ctx.Send(sel("discardMarkedText"))
		}
	})
	w.marked = ""
	w.p.queue.Push(platform.Event{Kind: platform.EventComposition, Window: w.id})
}

// SetCursor implements platform.Window.
func (w *Window) SetCursor(c input.Cursor) {
	if w.cursor == c {
		return
	}
	w.cursor = c
	w.p.libs.pool(w.applyCursor)
}

// applyCursor sets the cursor of the window, while the pointer is over it.
func (w *Window) applyCursor() {
	name := "arrowCursor"
	switch w.cursor {
	case input.CursorIBeam:
		name = "IBeamCursor"
	case input.CursorHand:
		name = "pointingHandCursor"
	case input.CursorColResize:
		name = resizeCursor("columnResizeCursor", "resizeLeftRightCursor")
	case input.CursorRowResize:
		name = resizeCursor("rowResizeCursor", "resizeUpDownCursor")
	}
	class("NSCursor").Send(sel(name)).Send(sel("set"))
}

// resizeCursor returns the name of a cursor that resizes: the one of macOS
// 15, or before it the older one, which macOS 15 marks for deprecation.
func resizeCursor(name, older string) string {
	if responds(class("NSCursor"), name) {
		return name
	}
	return older
}

// activate brings the application to the front. macOS 14 has activate for it,
// and marks activateIgnoringOtherApps:, all there was before, for deprecation.
func (p *Platform) activate() {
	if responds(p.app, "activate") {
		p.app.Send(sel("activate"))
		return
	}
	p.app.Send(sel("activateIgnoringOtherApps:"), true)
}

// primaryHeight is the height of the screen with the menu bar, whose bottom
// left is the origin of the coordinates of AppKit.
func primaryHeight() float64 {
	screens := class("NSScreen").Send(sel("screens"))
	if count(screens) == 0 {
		return 0
	}
	return objc.Send[NSRect](at(screens, 0), sel("frame")).Size.Height
}

// ClientOrigin implements platform.Placement, in physical pixels from the top
// left of the main screen.
func (w *Window) ClientOrigin() (physicalX, physicalY int, ok bool) {
	w.p.libs.pool(func() {
		bounds := objc.Send[NSRect](w.view, sel("bounds"))
		inWindow := objc.Send[NSRect](w.view, sel("convertRect:toView:"), bounds, objc.ID(0))
		screen := objc.Send[NSRect](w.window, sel("convertRectToScreen:"), inWindow)
		top := primaryHeight() - (screen.Origin.Y + screen.Size.Height)
		physicalX = int(math.Round(screen.Origin.X * float64(w.scale)))
		physicalY = int(math.Round(top * float64(w.scale)))
	})
	return physicalX, physicalY, true
}

// MoveClientTo implements platform.Placement.
func (w *Window) MoveClientTo(physicalX, physicalY int) {
	w.p.libs.pool(func() {
		scale := float64(w.scale)
		if scale <= 0 {
			scale = 1
		}
		bounds := objc.Send[NSRect](w.view, sel("bounds"))
		content := NSRect{Size: bounds.Size}
		content.Origin.X = float64(physicalX) / scale
		content.Origin.Y = primaryHeight() - float64(physicalY)/scale - bounds.Size.Height
		frame := objc.Send[NSRect](w.window, sel("frameRectForContentRect:"), content)
		w.window.Send(sel("setFrameOrigin:"), frame.Origin)
	})
}

// ShowsScreenPoint implements platform.Placement.
func (w *Window) ShowsScreenPoint(physicalX, physicalY int) bool {
	shows := false
	w.p.libs.pool(func() {
		scale := float64(w.scale)
		point := NSPoint{X: float64(physicalX) / scale, Y: primaryHeight() - float64(physicalY)/scale}
		number := objc.Send[int64](class("NSWindow"), sel("windowNumberAtPoint:belowWindowWithWindowNumber:"), point, int64(0))
		if number != objc.Send[int64](w.window, sel("windowNumber")) {
			return
		}
		inWindow := objc.Send[NSPoint](w.window, sel("convertPointFromScreen:"), point)
		local := objc.Send[NSPoint](w.view, sel("convertPoint:fromView:"), inWindow, objc.ID(0))
		bounds := objc.Send[NSRect](w.view, sel("bounds"))
		shows = local.X >= 0 && local.Y >= 0 && local.X < bounds.Size.Width && local.Y < bounds.Size.Height
	})
	return shows
}

// ClientDecorated implements platform.Window: a window of CustomFrame, whose
// title bar is transparent and whose buttons are hidden.
func (w *Window) ClientDecorated() bool { return w.customFrame }

// SetHitTest implements platform.Window. AppKit does not ask; the application
// moves the window with BeginMove.
func (w *Window) SetHitTest(func(physicalX, physicalY int) platform.HitArea) {}

// Maximized implements platform.Window: zoomed, for a ClientDecorated window.
func (w *Window) Maximized() bool {
	if !w.customFrame {
		return false
	}
	zoomed := false
	w.p.libs.pool(func() { zoomed = objc.Send[bool](w.window, sel("isZoomed")) })
	return zoomed
}

// BeginMove implements platform.Window.
func (w *Window) BeginMove() {
	if w.lastMouseDown == 0 {
		return
	}
	w.p.libs.pool(func() { w.window.Send(sel("performWindowDragWithEvent:"), w.lastMouseDown) })
}

// ToggleMaximize implements platform.Window.
func (w *Window) ToggleMaximize() {
	w.p.libs.pool(func() { w.window.Send(sel("zoom:"), objc.ID(0)) })
}

// Minimize implements platform.Window.
func (w *Window) Minimize() {
	w.p.libs.pool(func() { w.window.Send(sel("miniaturize:"), objc.ID(0)) })
}

// ShowWindowMenu implements platform.Window.
func (w *Window) ShowWindowMenu(int, int) {}

// Activate implements platform.Window.
func (w *Window) Activate() {
	w.p.libs.pool(func() {
		w.window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
		w.p.activate()
	})
}

// Close implements platform.Window.
func (w *Window) Close() {
	if w.closed {
		return
	}
	// The elements of the screen reader go with the window, at once.
	if b, ok := w.Accessibility().(*bridge); ok {
		b.release()
	}
	w.closed = true
	w.syncSecureInput()
	p := w.p
	if p.menuOwner == w {
		p.menuOwner = nil
	}
	delete(p.windows, w.id)
	delete(p.byView, w.view)
	delete(p.byWindow, w.window)
	p.libs.pool(func() {
		if w.owner != nil && !w.owner.closed {
			w.owner.window.Send(sel("removeChildWindow:"), w.window)
		}
		w.window.Send(sel("setDelegate:"), objc.ID(0))
		w.window.Send(sel("orderOut:"), objc.ID(0))
		w.window.Send(sel("close"))
		if w.lastMouseDown != 0 {
			w.lastMouseDown.Send(sel("release"))
		}
		w.layer.Send(sel("release"))
		w.view.Send(sel("release"))
		w.window.Send(sel("release"))
	})
}
