package gowaku

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// windowHost gives a tree the services of its OS window, in logical pixels.
type windowHost struct{ w *window }

// SetIMEEnabled implements core.Host.
func (h windowHost) SetIMEEnabled(enabled bool) { h.w.native.SetIMEEnabled(enabled) }

// SetIMECursorRect implements core.Host.
func (h windowHost) SetIMECursorRect(r geom.Rect) {
	s := h.w.scale
	h.w.native.SetIMECursorRect(int(r.X*s), int(r.Y*s), int(r.W*s), int(r.H*s))
}

// SetIMEVertical implements core.VerticalIMEHost.
func (h windowHost) SetIMEVertical(vertical bool) { h.w.native.SetIMEVertical(vertical) }

// SetSecureInput implements core.SecureInputHost. A window system without it
// still has the IME turned off by the field.
func (h windowHost) SetSecureInput(secure bool) {
	if native, ok := h.w.native.(platform.SecureInputWindow); ok {
		native.SetSecureInput(secure)
	}
}

// CancelComposition implements core.Host.
func (h windowHost) CancelComposition() { h.w.native.CancelComposition() }

// SetCursor implements core.Host.
func (h windowHost) SetCursor(c input.Cursor) { h.w.native.SetCursor(c) }

// ClipboardText implements core.Host.
func (h windowHost) ClipboardText() string { return h.w.native.ClipboardText() }

// SetClipboardText implements core.Host.
func (h windowHost) SetClipboardText(s string) { h.w.native.SetClipboardText(s) }

// ClipboardKinds implements platform.Clipboard, for ClipboardHas.
func (h windowHost) ClipboardKinds() platform.ClipboardKinds { return h.w.native.ClipboardKinds() }

// ReadClipboard implements platform.Clipboard, for ReadClipboard.
func (h windowHost) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	return h.w.native.ReadClipboard(kinds)
}

// WriteClipboard implements platform.Clipboard, for WriteClipboard.
func (h windowHost) WriteClipboard(c platform.ClipboardContent) { h.w.native.WriteClipboard(c) }

// CreateTray implements platform.Trays, for ShowTray. What the user does to
// the icon reaches on from the post queue, on the UI goroutine, and the icon
// goes when Run ends.
func (h windowHost) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	ap := h.w.app
	native, err := ap.platform.CreateTray(opts, func(e platform.TrayEvent) { ap.post(func() { on(e) }) })
	if err != nil {
		return nil, err
	}
	t := &appTray{Tray: native, app: ap}
	ap.trays = append(ap.trays, t)
	return t, nil
}

// Notify implements platform.Trays, for Notify.
func (h windowHost) Notify(title, body string) error { return h.w.app.platform.Notify(title, body) }

// RequestNotifications implements platform.Trays, for RequestNotifications:
// done runs from the post queue, on the UI goroutine.
func (h windowHost) RequestNotifications(done func(error)) {
	ap := h.w.app
	ap.platform.RequestNotifications(func(err error) { ap.post(func() { done(err) }) })
}

// appTray is a tray icon of the platform that the app closes at the end of
// Run, unless it is closed before.
type appTray struct {
	platform.Tray
	app    *app
	closed bool
}

// Close implements platform.Tray.
func (t *appTray) Close() {
	if t.closed {
		return
	}
	t.closed = true
	t.Tray.Close()
	for i, o := range t.app.trays {
		if o == t {
			t.app.trays = append(t.app.trays[:i], t.app.trays[i+1:]...)
			break
		}
	}
}

// ScreenOrigin implements core.ScreenHost.
func (h windowHost) ScreenOrigin() (geom.Offset, float32, bool) {
	x, y, ok := h.w.native.ClientOrigin()
	return geom.Offset{X: float32(x), Y: float32(y)}, h.w.scale, ok
}

// ShowsScreenPoint implements core.ScreenHost.
func (h windowHost) ShowsScreenPoint(p geom.Offset) bool {
	return h.w.native.ShowsScreenPoint(int(geom.Floor32(p.X)), int(geom.Floor32(p.Y)))
}

// Activate implements core.ScreenHost.
func (h windowHost) Activate() { h.w.native.Activate() }

// StartDrag implements core.DragHost. done runs from the post queue, outside
// the event that ended the drag.
func (h windowHost) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	ap := h.w.app
	h.w.native.StartDrag(d, allowed, func(effect input.DropEffect) {
		ap.post(func() { done(effect) })
	})
}

// SetNativeMenu implements core.NativeMenuHost on window systems with a menu
// bar of their own. The items run from the post queue, outside the event of
// the menu.
func (h windowHost) SetNativeMenu(menu []core.NativeMenuItem) bool {
	native, ok := h.w.native.(platform.MenuBarWindow)
	if !ok {
		return false
	}
	// A new slice, because the handler of the menu before may still hold the old one.
	h.w.menuHandlers = nil
	items := h.w.nativeMenuItems(menu)
	ap := h.w.app
	handlers := h.w.menuHandlers
	native.SetMenuBar(items, func(id int) {
		if id >= 0 && id < len(handlers) && handlers[id] != nil {
			ap.post(handlers[id])
		}
	})
	return true
}

// nativeMenuItems converts menu, numbering the items by their handlers.
func (w *window) nativeMenuItems(menu []core.NativeMenuItem) []platform.MenuItem {
	if len(menu) == 0 {
		return nil
	}
	items := make([]platform.MenuItem, len(menu))
	for i := range menu {
		m := &menu[i]
		items[i] = platform.MenuItem{
			ID: -1, Label: m.Label, Key: m.Key, Modifiers: m.Modifiers, Enabled: m.Enabled,
			Checkable: m.Checkable, Checked: m.Checked, Separator: m.Separator,
			Items: w.nativeMenuItems(m.Items),
		}
		if m.OnInvoke != nil && len(m.Items) == 0 {
			items[i].ID = len(w.menuHandlers)
			w.menuHandlers = append(w.menuHandlers, m.OnInvoke)
		}
	}
	return items
}
