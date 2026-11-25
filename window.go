package gowaku

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// window is one OS window with its tree and GPU surface.
type window struct {
	app        *app
	native     platform.Window
	surface    gpu.Surface
	tree       *core.Tree
	root       *rootState
	canvas     paint.Canvas
	batch      gpu.Batch
	background geom.Color
	// themed means that the background follows the theme.
	themed bool
	// fullscreen is what the window system last said, and hides the title
	// bar that gowaku draws or TitleBar. transparent means that the window
	// shows the alpha of its frames.
	fullscreen  bool
	transparent bool
	// menuHandlers run the items of the menu that the window system shows,
	// by their IDs.
	menuHandlers []func()

	size      geom.Size
	scale     float32
	minimized bool
	// hidden means that WindowHandle.Hide took the window off the screen.
	hidden bool

	// title, focused and maximized are what a title bar that gowaku draws
	// shows, and decorated says whether it draws one.
	title     string
	focused   bool
	maximized bool
	decorated bool
	// customFrame is WindowOptions.CustomFrame: the application draws the
	// title bar with TitleBar, and gowaku draws none of its own.
	customFrame bool
	onClose     func()
	// onCloseRequest is WindowOptions.OnCloseRequest, or what
	// WindowHandle.SetOnCloseRequest put in its place.
	onCloseRequest func() bool
	// onDrawError is WindowOptions.OnDrawError.
	onDrawError func(error)
	// owner is the window a modal window belongs to, or nil, and modals the
	// modal windows that belong to this one, the newest last. While there are
	// any, the window takes no input. See docs/ADR/0047-modal-windows.md.
	owner  *window
	modals []*window
	// closing means that Close was asked for. The window stops taking events
	// and goes at the next safe point, because the request may come from a
	// callback of the very tree that it ends.
	closing bool

	// a11y is the bridge to the screen readers, or nil. spoken is the tree it
	// was last given, with its focus, and diff the storage of the comparison.
	a11y        platform.Accessibility
	spoken      *semantics.Node
	spokenFocus semantics.ID
	diff        semantics.Diff
}

// applyTheme gives what the OS draws for the window the look of the theme.
func (w *window) applyTheme() {
	th := w.app.theme()
	if w.themed {
		w.background = th.Color.Canvas
		w.tree.MarkNeedsPaint()
	}
	w.native.SetDarkTitleBar(th.Brightness == theme.BrightnessDark)
}

// open creates a window. It is called on the UI goroutine.
func (ap *app) open(opts WindowOptions, root Widget) (*window, error) {
	return ap.openOwned(opts, root, nil)
}

// openOwned creates a window, modal over owner if owner is not nil.
func (ap *app) openOwned(opts WindowOptions, root Widget, owner *window) (*window, error) {
	return ap.openPlaced(opts, root, owner, nil)
}

// openPlaced creates a window, modal over owner if owner is not nil, with the
// top-left corner of its client area at place, physical pixels of the screen,
// if place is not nil.
func (ap *app) openPlaced(opts WindowOptions, root Widget, owner *window, place *geom.Offset) (*window, error) {
	if root == nil {
		panic("gowaku: a window needs a root widget")
	}
	native, err := ap.platform.CreateWindow(nativeOptions(opts, ap.theme(), ap.appID, owner, place))
	if err != nil {
		return nil, err
	}
	width, height := native.PhysicalSize()
	scale := native.Scale()
	surface, err := ap.backend.CreateSurface(native.NativeHandle(), width, height, scale)
	if err != nil {
		native.Close()
		return nil, err
	}
	return ap.attach(opts, root, owner, native, surface, width, height, scale), nil
}

// nativeOptions returns what the platform needs to create the window of opts
// under th, for the application appID.
func nativeOptions(opts WindowOptions, th *theme.Theme, appID string, owner *window, place *geom.Offset) platform.WindowOptions {
	popts := platform.WindowOptions{
		Title: opts.Title, Width: opts.Width, Height: opts.Height,
		DarkTitleBar: th.Brightness == theme.BrightnessDark,
		CustomFrame:  opts.CustomFrame,
		AppID:        appID,
		Icon:         nrgbas(opts.Icon),
		MinSize:      opts.MinSize, MaxSize: opts.MaxSize,
		AlwaysOnTop: opts.AlwaysOnTop, Fullscreen: opts.Fullscreen, Transparent: opts.Transparent,
	}
	if owner != nil {
		popts.Owner, popts.Modal = owner.native, true
	}
	if place != nil {
		popts.Placed, popts.ClientX, popts.ClientY = true, int(geom.Floor32(place.X+0.5)), int(geom.Floor32(place.Y+0.5))
	}
	return popts
}

// attach makes a window of native, whose surface was created at the given size
// and scale, and mounts root in it.
func (ap *app) attach(opts WindowOptions, root Widget, owner *window, native platform.Window, surface gpu.Surface, width, height int, scale float32) *window {
	background, themed := windowBackground(opts.Background, ap.theme(), native.Transparent())
	w := &window{
		app:            ap,
		native:         native,
		surface:        surface,
		scale:          scale,
		background:     background,
		onClose:        opts.OnClose,
		onDrawError:    opts.OnDrawError,
		onCloseRequest: opts.OnCloseRequest,
		themed:         themed,
		title:          opts.Title,
		decorated:      native.ClientDecorated(),
		maximized:      native.Maximized(),
		fullscreen:     native.Fullscreen(),
		transparent:    native.Transparent(),
		a11y:           native.Accessibility(),
		owner:          owner,
		customFrame:    opts.CustomFrame,
	}
	native.SetHitTest(w.hitArea)
	native.SetDropHandler(w.drop)
	if owner != nil {
		owner.modals = append(owner.modals, w)
	}
	// The host comes first, so that the InitState of the first build reaches
	// the tray, the notifications and the clipboard.
	w.tree = core.NewTreeWithHost(rootWidget{window: w, home: root}, windowHost{w})
	// Work posted from other goroutines wakes the event loop, which ticks.
	w.tree.SetWake(ap.platform.Wake)
	w.applyTheme()
	w.resize(width, height, scale)
	ap.windows[native.ID()] = w
	return w
}

func (w *window) resize(physicalWidth, physicalHeight int, scale float32) {
	w.minimized = physicalWidth == 0 || physicalHeight == 0
	if !w.minimized {
		size := geom.Size{W: float32(physicalWidth) / scale, H: float32(physicalHeight) / scale}
		switch {
		case w.scale != scale:
			w.size, w.scale = size, scale
			w.root.refresh()
		case w.size != size:
			// Only what reads the size of the window rebuilds; see rootState.
			w.size = size
			w.root.resized()
		}
		w.tree.MarkNeedsPaint()
	}
}

// shown reports whether the window is on the screen to draw: neither
// minimized nor hidden.
func (w *window) shown() bool { return !w.minimized && !w.hidden }

// hide takes the window off the screen, with the modal windows over it.
func (w *window) hide() {
	for _, m := range w.modals {
		m.hide()
	}
	if !w.hidden {
		w.hidden = true
		w.native.SetVisible(false)
	}
}

// show puts the window back on the screen, with the modal windows over it,
// the newest in front.
func (w *window) show() {
	if w.hidden {
		w.hidden = false
		w.native.SetVisible(true)
		w.tree.MarkNeedsPaint()
	}
	for _, m := range w.modals {
		m.show()
	}
}

// titleBarShown reports whether gowaku draws the title bar of the window.
func (w *window) titleBarShown() bool { return w.decorated && !w.customFrame && !w.fullscreen }

// contentSize is the size of the window less the title bar that gowaku draws:
// the Size of the MediaQuery of the window.
func (w *window) contentSize() geom.Size {
	sz := w.size
	if w.titleBarShown() {
		sz.H = max(sz.H-titleBarHeight, 0)
	}
	return sz
}

func (ap *app) close(w *window) {
	if _, ok := ap.windows[w.native.ID()]; !ok {
		return
	}
	// The modal windows of w go first, the newest first, as their owner
	// cannot outlive them.
	for len(w.modals) > 0 {
		ap.close(w.modals[len(w.modals)-1])
	}
	if o := w.owner; o != nil {
		o.modals = slices.DeleteFunc(o.modals, func(m *window) bool { return m == w })
	}
	if w.onClose != nil {
		w.onClose()
	}
	if inspecting {
		ap.inspectForget(w)
	}
	delete(ap.windows, w.native.ID())
	w.tree.Dispose()
	w.surface.Destroy()
	w.native.Close()
	// Where the OS does not hand the keyboard back to the owner, ask for it.
	if o := w.owner; o != nil && len(o.modals) == 0 && ap.windows[o.native.ID()] == o && !o.closing {
		o.native.Activate()
	}
}

// requestClose closes w at the next safe point, once no callback of its tree is
// on the stack.
func (ap *app) requestClose(w *window) {
	if !w.closing {
		w.closing = true
		ap.closing = append(ap.closing, w)
	}
}

// askClose asks w whether the user may close it, and reports whether the close
// goes ahead. It must run from the event loop, not from a callback of the tree
// of w. A modal window over w answers for it: it comes to the front, and w
// stays.
func (ap *app) askClose(w *window) bool {
	if w.closing || ap.windows[w.native.ID()] != w {
		return false
	}
	if len(w.modals) > 0 {
		w.modals[len(w.modals)-1].native.Activate()
		return false
	}
	if w.onCloseRequest == nil {
		return true
	}
	ok := w.onCloseRequest()
	// The callback may have closed the window itself.
	return ok && !w.closing && ap.windows[w.native.ID()] == w
}

// requestCloseAsking asks w whether the user may close it once the callback on
// the stack has returned, and closes it if so. The close buttons that gowaku
// draws use it, as they sit in the tree of w.
func (ap *app) requestCloseAsking(w *window) {
	ap.post(func() {
		if ap.askClose(w) {
			ap.requestClose(w)
		}
	})
}

// reap closes the windows that asked for it.
func (ap *app) reap() {
	// By index, and reading the queue anew each time: an OnClose may close
	// another window, which joins the queue while it is being worked through.
	for i := 0; i < len(ap.closing); i++ {
		w := ap.closing[i]
		ap.closing[i] = nil
		ap.close(w)
	}
	ap.closing = ap.closing[:0]
}

// syncFrame picks up whether gowaku draws the title bar, and what it shows.
func (w *window) syncFrame() {
	decorated, maximized, fullscreen := w.native.ClientDecorated(), w.native.Maximized(), w.native.Fullscreen()
	if decorated != w.decorated || maximized != w.maximized || fullscreen != w.fullscreen {
		w.decorated, w.maximized, w.fullscreen = decorated, maximized, fullscreen
		w.root.refresh()
	}
}

func (w *window) setFocused(focused bool) {
	if w.focused != focused {
		w.focused = focused
		if w.decorated {
			w.root.refresh()
		}
	}
}
