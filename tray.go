package gowaku

import (
	"image"
	"image/draw"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// ErrNoTray reports that the desktop shows no icons of applications, such as
// GNOME without the AppIndicator extension. ErrNoNotifications reports that it
// shows no notifications.
var (
	ErrNoTray          = platform.ErrNoTray
	ErrNoNotifications = platform.ErrNoNotifications
)

// TrayOptions is what a tray icon shows and does. See
// docs/ADR/0051-tray-and-notifications.md.
type TrayOptions struct {
	// Icon is drawn at the size of the tray, 16 to 32 pixels. Nil shows the
	// icon of applications of the OS.
	Icon    image.Image
	Tooltip string
	// Menu opens on a secondary click. The tray draws it, as no window of the
	// application holds it; the actions of its items are those of a MenuBar.
	Menu []overlay.MenuItem
	// OnActivate runs on a click on the icon.
	OnActivate func()
	// OnNotificationClick runs on a click on a notification of Notify. The
	// first tray hears those that no tray showed.
	OnNotificationClick func()
}

// Tray is an icon of the application in the tray of the desktop.
type Tray struct {
	native  platform.Tray
	opts    TrayOptions
	actions map[int]func()
}

// ShowTray shows an icon in the tray, which stays until Close or the end of
// Run. err is ErrNoTray where the desktop has no tray. It must be called on the
// UI goroutine, outside a Build, as from InitState or a callback.
//
// The tray, like Notify, goes through the core.Host of the tree of ctx, as the
// clipboard functions do: in a window of Run that is the tray of the OS, and
// under a gowakutest.Tester a fake tray that the test inspects and clicks. A
// host that implements no platform.Trays, as outside both, gives ErrNoTray.
func ShowTray(ctx BuildContext, opts TrayOptions) (*Tray, error) {
	trays, ok := ctx.Host().(platform.Trays)
	if !ok {
		return nil, ErrNoTray
	}
	t := &Tray{}
	native, err := trays.CreateTray(t.convert(opts), t.handle)
	if err != nil {
		return nil, err
	}
	t.native = native
	return t, nil
}

// Update replaces what the icon shows and does. It must be called on the UI
// goroutine.
func (t *Tray) Update(opts TrayOptions) {
	if t.native != nil {
		t.native.Update(t.convert(opts))
	}
}

// Close takes the icon away. It must be called on the UI goroutine.
func (t *Tray) Close() {
	if t.native == nil {
		return
	}
	t.native.Close()
	t.native = nil
}

// convert keeps opts and turns them into what the platform shows.
func (t *Tray) convert(opts TrayOptions) platform.TrayOptions {
	t.opts = opts
	t.actions = map[int]func(){}
	return platform.TrayOptions{Icon: nrgba(opts.Icon), Tooltip: opts.Tooltip, Items: t.items(opts.Menu)}
}

func (t *Tray) items(menu []overlay.MenuItem) []platform.TrayItem {
	out := make([]platform.TrayItem, 0, len(menu))
	for i := range menu {
		m := &menu[i]
		id := len(t.actions)
		t.actions[id] = m.Action.OnInvoke
		out = append(out, platform.TrayItem{
			ID: id, Label: m.Action.Label, Shortcut: m.Action.Shortcut.String(),
			Enabled: len(m.Items) > 0 || m.Action.OnInvoke != nil, Checkable: m.Action.Checkable,
			Checked: m.Action.Checked, Separator: m.Separator, Items: t.items(m.Items),
		})
	}
	return out
}

func (t *Tray) handle(e platform.TrayEvent) {
	if t.native == nil {
		return
	}
	switch e.Kind {
	case platform.TrayActivate:
		if t.opts.OnActivate != nil {
			t.opts.OnActivate()
		}
	case platform.TrayChoose:
		if fn := t.actions[e.Item]; fn != nil {
			fn()
		}
	case platform.TrayNotificationClick:
		if t.opts.OnNotificationClick != nil {
			t.opts.OnNotificationClick()
		}
	}
}

// nrgba converts img for the platform.
func nrgba(img image.Image) *image.NRGBA {
	if img == nil {
		return nil
	}
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	return out
}

// Notify shows a notification of the application: a toast on Windows, through
// the tray icon, and the notification service on Linux. A click on it runs the
// OnNotificationClick of the first tray. err is ErrNoNotifications where the
// desktop shows none. It must be called on the UI goroutine, outside a Build.
// Like ShowTray it goes through the host of the tree of ctx, so that a
// gowakutest.Tester records it, and gives ErrNoNotifications outside Run.
func Notify(ctx BuildContext, title, body string) error {
	trays, ok := ctx.Host().(platform.Trays)
	if !ok {
		return ErrNoNotifications
	}
	return trays.Notify(title, body)
}
