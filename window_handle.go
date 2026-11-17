package gowaku

import (
	"image"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// WindowOptions configures a new window.
type WindowOptions struct {
	Title string
	// Width and Height are the client area in logical pixels. Zero selects a
	// default.
	Width, Height float32
	// Background fills the window. The zero value is the canvas of the theme.
	Background geom.Color
	// OnClose runs on the UI goroutine when the window closes, whether the user
	// closed it, the program called Close or Quit, or the last window went. The
	// tree is still whole, so it is the place to save what the window holds.
	OnClose func()
	// OnCloseRequest runs on the UI goroutine when the user asks to close the
	// window: with the close button of the OS or of a TitleBar, Alt+F4, the
	// window menu, the taskbar, the window manager or a screen reader, or when
	// the program calls WindowHandle.RequestClose. Returning false keeps the
	// window open, so that the application can ask about unsaved work, with a
	// Dialog or ShowModal, and call WindowHandle.Close once it has the answer.
	// It must not block. It runs from the event loop, outside any Build and
	// outside the callbacks of the widgets, and not while a modal window is
	// open over the window: a request then brings the modal window to the
	// front instead. Nil closes the window at once.
	//
	// Closing from the program asks nothing: WindowHandle.Close, Quit, closing
	// the owner of a modal window, and the window system destroying the
	// window. See docs/ADR/0076-close-request.md.
	OnCloseRequest func() bool
	// OnDrawError runs on the UI goroutine, after a frame, when the GPU could
	// not draw part of what the window shows, such as a custom shader it
	// refused; the rest is drawn, and the part stays out from then on. Each
	// failure is reported once. Without it, the error goes to the default
	// slog logger.
	OnDrawError func(error)
	// CustomFrame takes the title bar of the OS off the window, so that the
	// application draws its own: put a TitleBar at the top of the root. The
	// edges still resize the window. See docs/ADR/0048-custom-title-bar.md.
	CustomFrame bool
	// Icon is the picture of the window in its title bar, the task bar and the
	// window switcher: one image, best 256 pixels square, which each size the
	// system wants is scaled down from, or the same picture in several sizes,
	// of which the smallest that covers each size is scaled. Nil leaves the
	// default of the OS. macOS has no icon per window and shows the last one
	// given in the Dock, for the whole application. Wayland shows it only
	// where the compositor has xdg-toplevel-icon-v1; elsewhere it takes the
	// icon of the desktop file that App.AppID names.
	Icon []image.Image
	// MinSize and MaxSize bound the client area in logical pixels, which the
	// user cannot resize the window past; they follow the scale of the
	// monitor. A zero axis has no bound, so the zero values bound nothing.
	MinSize, MaxSize geom.Size
	// AlwaysOnTop keeps the window above the windows of other applications.
	// Wayland has no way to ask for it, and ignores it.
	AlwaysOnTop bool
	// Fullscreen opens the window over the whole of its monitor, without a
	// frame. A TitleBar takes no room while the window is fullscreen, and the
	// title bar that gowaku draws on some Wayland compositors goes too.
	Fullscreen bool
	// Transparent opens a window that shows the desktop through the alpha of
	// what it draws, for shaped or glass-like windows: the zero Background is
	// then clear instead of the canvas of the theme. It is fixed when the
	// window opens. Where the system cannot, the window is opaque and the zero
	// Background is the canvas again; WindowHandle.Transparent tells. On X11
	// it needs a compositing manager. See docs/ADR/0111-window-decorations.md.
	Transparent bool
}

// WindowHandle refers to an open window.
type WindowHandle struct {
	app    *app
	window *window
}

// OpenWindow opens another window with its own tree. It must be called on the
// UI goroutine, during Run.
func OpenWindow(opts WindowOptions, root Widget) (*WindowHandle, error) {
	ap := currentApp.Load()
	if ap == nil {
		panic("gowaku: OpenWindow called outside Run")
	}
	w, err := ap.open(opts, root)
	if err != nil {
		return nil, err
	}
	return &WindowHandle{ap, w}, nil
}

// Close closes the window, also from a callback of its own widgets: it goes once
// the callback has returned. Run returns once the last window is gone. It must
// be called on the UI goroutine.
func (h *WindowHandle) Close() {
	if h.app.windows[h.window.native.ID()] == h.window {
		h.app.requestClose(h.window)
	}
}

// RequestClose asks to close the window as the close button of the OS does:
// OnCloseRequest decides, once the callback that called RequestClose has
// returned. While a modal window is open over the window, that window comes to
// the front instead. It must be called on the UI goroutine.
func (h *WindowHandle) RequestClose() {
	if h.app.windows[h.window.native.ID()] == h.window {
		h.app.requestCloseAsking(h.window)
	}
}

// SetOnCloseRequest puts fn in place of WindowOptions.OnCloseRequest, so that
// the widgets of the window, which know what it holds, can decide whether it
// closes: a State calls it with the handle of WindowOf. Nil closes the window
// without asking. It must be called on the UI goroutine.
func (h *WindowHandle) SetOnCloseRequest(fn func() bool) {
	h.window.onCloseRequest = fn
}

// SetTitle changes the title of the window. It must be called on the UI
// goroutine.
func (h *WindowHandle) SetTitle(title string) {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.SetTitle(title)
		w.title = title
		if w.decorated {
			w.root.refresh()
		}
	}
}

// WindowOf returns the window of ctx. It must be called on the UI goroutine,
// during Run.
func WindowOf(ctx BuildContext) *WindowHandle {
	w, ok := core.Lookup[*window](ctx)
	if !ok {
		panic("gowaku: WindowOf found no window; call it below the root of a window of Run")
	}
	return &WindowHandle{w.app, w}
}

// ShowModal opens a window that belongs to the window of ctx and keeps the user
// from that window until the new one closes, as a dialog of the OS does: the
// owner takes no input, a press on it brings the modal window to the front,
// and it opens centered over the owner where the OS allows. opts.OnClose runs
// when the window closes, which is where its result goes; closing the owner
// closes it first. The event loop stays one: ShowModal returns at once.
//
//	var h *gowaku.WindowHandle
//	h, err := gowaku.ShowModal(ctx, gowaku.WindowOptions{Title: "設定", OnClose: apply},
//		settings{onDone: func() { h.Close() }})
//
// It must be called on the UI goroutine, during Run, outside a Build. See
// docs/ADR/0047-modal-windows.md.
func ShowModal(ctx BuildContext, opts WindowOptions, root Widget) (*WindowHandle, error) {
	owner := WindowOf(ctx).window
	w, err := owner.app.openOwned(opts, root, owner)
	if err != nil {
		return nil, err
	}
	return &WindowHandle{owner.app, w}, nil
}
