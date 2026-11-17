package platform

import (
	"image"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// Brightness is the light or dark mode of the OS.
type Brightness uint8

// The brightnesses.
const (
	BrightnessLight Brightness = iota
	BrightnessDark
)

// WindowID identifies a window within a Platform. IDs are never reused and
// zero is never a valid ID.
type WindowID uint32

// WindowOptions configures a new window. The zero value is a usable untitled
// window of a default size.
type WindowOptions struct {
	// DarkTitleBar starts the window with the title bar of the dark mode, so that
	// it does not show up light first.
	DarkTitleBar bool
	Title        string
	// Width and Height are the client area in logical pixels.
	// Zero selects a default.
	Width, Height float32
	// Owner, if set, is the window the new one belongs to: it stays in front
	// of it, opens centered over it, and has no button of its own in the task
	// bar. Modal also keeps the user from Owner until the new window closes,
	// as far as the OS does that; the application stops the rest.
	// See docs/ADR/0047-modal-windows.md.
	Owner Window
	Modal bool
	// CustomFrame asks for a window without the title bar of the OS: the
	// application draws one, and the window is ClientDecorated. The edges
	// still resize the window. See docs/ADR/0048-custom-title-bar.md.
	CustomFrame bool
	// Placed opens the window with the top-left corner of its client area at
	// ClientX, ClientY, physical pixels of the screen, where the window
	// system lets applications place windows. Wayland does not, and puts the
	// window where it likes.
	Placed           bool
	ClientX, ClientY int
	// AppID names the application to the desktop: the app_id of Wayland and
	// the class of WM_CLASS on X11, which match it with its desktop file.
	// Empty is "gowaku". Windows and macOS ignore it.
	AppID string
	// Icon is the picture of the window, in one or more sizes; nil leaves
	// the default of the OS. See Window.SetIcon.
	Icon []*image.NRGBA
	// MinSize and MaxSize bound the client area in logical pixels. A zero
	// axis has no bound. See Window.SetSizeLimits.
	MinSize, MaxSize geom.Size
	// AlwaysOnTop keeps the window above the windows of other applications.
	// See Window.SetAlwaysOnTop.
	AlwaysOnTop bool
	// Fullscreen opens the window over the whole of its monitor.
	Fullscreen bool
	// Transparent asks for a window whose pixels keep the alpha that the GPU
	// draws, so that the desktop shows through where it is below one. The
	// gpu surface then takes premultiplied colors. Where the system cannot,
	// the window is opaque; Window.Transparent tells.
	// See docs/ADR/0111-window-decorations.md.
	Transparent bool
}

// Placement is where a window is on the screen, in physical pixels, which every
// window of the desktop shares. A tab torn off a TabBar uses it to find the
// window under the pointer and to open its own window there. A Window
// implements it.
type Placement interface {
	// ClientOrigin returns where the top-left corner of the client area is on
	// the screen. ok is false where the window system does not tell, as on
	// Wayland.
	ClientOrigin() (physicalX, physicalY int, ok bool)
	// MoveClientTo moves the window so that the top-left corner of its client
	// area is at a point of the screen. Wayland ignores it.
	MoveClientTo(physicalX, physicalY int)
	// ShowsScreenPoint reports whether the window is what the user sees at a
	// point of the screen: inside its client area, and under no other window.
	// It is false where the window system does not tell.
	ShowsScreenPoint(physicalX, physicalY int) bool
}

// HitArea is what a point of a ClientDecorated window is to the window system.
type HitArea uint8

// The hit areas.
const (
	// HitClient is the application's: the pointer goes to it.
	HitClient HitArea = iota
	// HitCaption moves the window when dragged, maximizes it on a double
	// click, and opens the window menu on a secondary click.
	HitCaption
	// HitMaximize is the maximize button, which Windows 11 needs to know to
	// offer its snap layouts. The pointer still goes to the application.
	HitMaximize
)

// Window is a top-level OS window.
//
// All methods must be called on the UI goroutine.
type Window interface {
	// ID returns the identifier used in events for this window.
	ID() WindowID
	// PhysicalSize returns the client area in physical pixels.
	PhysicalSize() (w, h int)
	// Scale returns physical pixels per logical pixel.
	Scale() float32
	// SetTitle changes the title bar text.
	SetTitle(title string)
	// SetDarkTitleBar asks the OS for a title bar that suits dark contents.
	SetDarkTitleBar(dark bool)
	// NativeHandle returns what gpu needs to create a surface.
	NativeHandle() NativeHandle
	// SetIMEEnabled turns text conversion on or off for the window. New windows
	// have it off.
	SetIMEEnabled(enabled bool)
	// SetIMECursorRect tells the IME where the caret is, in physical pixels of
	// the client area, so that it can place its candidate list.
	SetIMECursorRect(physicalX, physicalY, physicalWidth, physicalHeight int)
	// SetIMEVertical tells the IME that the text runs in vertical lines, so
	// that its candidate list opens beside the line instead of over it.
	SetIMEVertical(vertical bool)
	// CancelComposition makes the IME drop what it holds, without committing it.
	CancelComposition()
	// SetCursor changes the mouse cursor over the window.
	SetCursor(c input.Cursor)
	// ClipboardText returns the text on the clipboard, or "".
	ClipboardText() string
	// SetClipboardText replaces the clipboard with s.
	SetClipboardText(s string)
	// Clipboard reads and writes the other kinds of data.
	Clipboard
	// Dragging drags data in and out of the window.
	Dragging
	// Placement says where the window is on the screen.
	Placement
	// ClientDecorated reports whether the application must draw the title bar
	// itself, because the OS does not: a window with CustomFrame, or any window
	// on a Wayland compositor without xdg-decoration, such as GNOME. A change
	// comes with an EventResize. Such a window takes the pointer near its
	// edges for resizing on its own.
	ClientDecorated() bool
	// SetHitTest gives a ClientDecorated window the function that says what a
	// point of it is, in physical pixels of the client area. Windows asks it
	// while the pointer moves, on the UI goroutine; other systems ignore it,
	// and the application moves the window with BeginMove.
	SetHitTest(fn func(physicalX, physicalY int) HitArea)
	// Maximized reports whether the window is maximized. It is only known for
	// ClientDecorated windows, which show it on their title bar, and false for
	// the others.
	Maximized() bool
	// BeginMove lets the user move the window with the pointer that is down on
	// the title bar of a ClientDecorated window. Other windows ignore it.
	BeginMove()
	// ToggleMaximize maximizes the window, or restores it if it is maximized.
	// Only ClientDecorated windows need it; others may ignore it.
	ToggleMaximize()
	// Minimize hides the window in the task bar. Only ClientDecorated windows
	// need it; others may ignore it.
	Minimize()
	// ShowWindowMenu opens the menu of the window system at a point in physical
	// pixels of the client area, after a secondary press on the title bar of a
	// ClientDecorated window. Other windows ignore it.
	ShowWindowMenu(physicalX, physicalY int)
	// ShowFileDialog shows the file dialog of the system in front of the window,
	// and calls done with the chosen paths, nil if the user canceled. done may
	// run on any goroutine, and before ShowFileDialog returns. err is
	// ErrNoFileDialog where the system has none.
	ShowFileDialog(opts FileDialogOptions, done func(paths []string, err error))
	// Accessibility returns the bridge to the screen readers of the OS, or nil
	// where there is none yet.
	Accessibility() Accessibility
	// SetIcon replaces the picture of the window in the title bar, the task
	// bar and the window switcher. images are the same picture in one or more
	// sizes; each size the system wants is scaled down from the smallest one
	// that is at least as large. Nil goes back to the default. macOS has no
	// icon per window and shows it in the Dock, for the whole application;
	// Wayland shows it only where the compositor has xdg-toplevel-icon-v1.
	SetIcon(images []*image.NRGBA)
	// SetSizeLimits bounds the client area, in logical pixels, which the user
	// cannot resize the window past. A zero axis has no bound. A window
	// outside the new bounds is resized into them where the system lets
	// applications size windows.
	SetSizeLimits(minSize, maxSize geom.Size)
	// SetAlwaysOnTop keeps the window above the windows of other applications,
	// or stops. Wayland has no way to ask, and ignores it.
	SetAlwaysOnTop(on bool)
	// SetFullscreen puts the window over the whole of its monitor without a
	// frame, or back where it was. The change arrives with an EventResize,
	// as do those that the user makes through the system.
	SetFullscreen(on bool)
	// Fullscreen reports whether the window is fullscreen, as far as the
	// system has said.
	Fullscreen() bool
	// Transparent reports whether the window shows the alpha of its frames:
	// it was created with WindowOptions.Transparent and the system can.
	Transparent() bool
	// Activate brings the window to the front and gives it the keyboard, as
	// far as the OS lets an application do that: a press on the owner of a
	// modal window activates the modal one.
	Activate()
	// Close destroys the OS window. Destroy any gpu surface first.
	// Closing twice is a no-op.
	Close()
}

// Platform owns the event loop and every window.
//
// All methods except Wake must be called on the UI goroutine.
type Platform interface {
	// CreateWindow opens and shows a new window.
	CreateWindow(opts WindowOptions) (Window, error)
	// WaitEvent blocks until an event arrives or timeout passes, and returns
	// EventNone in the latter case. A negative timeout blocks forever and zero
	// polls.
	WaitEvent(timeout time.Duration) Event
	// Brightness returns the current mode of the OS. EventBrightness reports
	// changes.
	Brightness() Brightness
	// ReduceMotion reports whether the user asked the OS for less animation.
	// EventSettings reports changes.
	ReduceMotion() bool
	// Locale returns the locale of the user as a BCP 47 tag, such as "ja-JP",
	// or "" when the OS does not say. EventSettings reports changes where the
	// OS announces them.
	Locale() string
	// Wake makes a blocked WaitEvent return EventWake. Wakes that arrive before
	// the first one is delivered are merged. It is safe to call from any goroutine.
	Wake()
	// SetModalHandler registers fn to run while the OS holds the UI goroutine
	// in a modal loop, such as a window drag-resize, where WaitEvent cannot
	// return. fn runs on the UI goroutine, nested inside the OS call. It should
	// drain WaitEvent(0) and draw, as the main loop would.
	// See docs/ADR/0001-win32-modal-loop.md.
	SetModalHandler(fn func())
	// Trays shows icons in the tray of the desktop, and notifications.
	Trays
	// Destroy closes every window and releases the platform.
	Destroy()
}
