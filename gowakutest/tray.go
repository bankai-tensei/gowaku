package gowakutest

import (
	"slices"

	"github.com/bankai-tensei/gowaku/platform"
)

// Notification is a notification that gowaku.Notify showed under a Tester.
type Notification struct {
	Title, Body string
}

// Tray is a tray icon that gowaku.ShowTray showed under a Tester. It stands
// for the tray of the desktop: the test reads what it shows, and clicks it or
// chooses from its menu as the user would.
type Tray struct {
	tt     *Tester
	opts   platform.TrayOptions
	on     func(platform.TrayEvent)
	closed bool
}

var _ platform.Trays = (*host)(nil)

// CreateTray implements platform.Trays, for gowaku.ShowTray.
func (h *host) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	t := &Tray{tt: h.tt, opts: opts, on: on}
	h.trays = append(h.trays, t)
	return t, nil
}

// Notify implements platform.Trays, for gowaku.Notify. It records the
// notification, unless SetNotificationsAllowed refused them.
func (h *host) Notify(title, body string) error {
	if h.noNotifications {
		return platform.ErrNoNotifications
	}
	h.notifications = append(h.notifications, Notification{Title: title, Body: body})
	return nil
}

// RequestNotifications implements platform.Trays, for
// gowaku.RequestNotifications: done hears what SetNotificationsAllowed says
// at the next Pump, as the answer of the desktop comes after the call.
func (h *host) RequestNotifications(done func(error)) {
	h.answers = append(h.answers, done)
}

// answer calls the done functions of RequestNotifications.
func (h *host) answer() {
	var err error
	if h.noNotifications {
		err = platform.ErrNoNotifications
	}
	for len(h.answers) > 0 {
		done := h.answers[0]
		h.answers = h.answers[1:]
		done(err)
	}
}

// SetNotificationsAllowed stands for the user letting the application show
// notifications, or not, which they do by default: refused, gowaku.Notify
// returns gowaku.ErrNoNotifications and records nothing, and
// gowaku.RequestNotifications answers with it.
func (tt *Tester) SetNotificationsAllowed(allowed bool) { tt.host.noNotifications = !allowed }

// Update implements platform.Tray.
func (t *Tray) Update(opts platform.TrayOptions) { t.opts = opts }

// Close implements platform.Tray. The tray leaves Tester.Trays.
func (t *Tray) Close() {
	if t.closed {
		return
	}
	t.closed = true
	h := t.tt.host
	for i, o := range h.trays {
		if o == t {
			h.trays = append(h.trays[:i], h.trays[i+1:]...)
			break
		}
	}
}

// Options returns what the icon shows: its picture, its tooltip and the items
// of its menu, with their labels as the application gave them, mnemonics
// included ("Open(&O)"), and whether each is enabled or checked.
func (t *Tray) Options() platform.TrayOptions { return t.opts }

// Activate clicks the icon, which runs gowaku.TrayOptions.OnActivate, then
// settles.
func (t *Tray) Activate() {
	t.tt.t.Helper()
	t.send(platform.TrayEvent{Kind: platform.TrayActivate})
}

// Choose chooses the enabled item of the menu labeled label, as given in the
// menu, mnemonic included, in a submenu or not, which runs its action, then
// settles. It fails the test if there is no such item.
func (t *Tray) Choose(label string) {
	t.tt.t.Helper()
	it := findTrayItem(t.opts.Items, label)
	if it == nil {
		t.tt.t.Fatalf("gowakutest: no enabled item %q in the menu of the tray", label)
	}
	t.send(platform.TrayEvent{Kind: platform.TrayChoose, Item: it.ID})
}

// send hands e to the tray, as the desktop would, then settles.
func (t *Tray) send(e platform.TrayEvent) {
	t.tt.t.Helper()
	if t.closed {
		t.tt.t.Fatal("gowakutest: the tray is closed")
	}
	t.on(e)
	t.tt.Pump()
	t.tt.Settle()
}

// findTrayItem returns the first enabled item labeled label at or below
// items, but for those that open a submenu.
func findTrayItem(items []platform.TrayItem, label string) *platform.TrayItem {
	for i := range items {
		it := &items[i]
		if len(it.Items) > 0 {
			if found := findTrayItem(it.Items, label); found != nil {
				return found
			}
			continue
		}
		if it.Label == label && it.Enabled && !it.Separator {
			return it
		}
	}
	return nil
}

// Trays returns the tray icons that gowaku.ShowTray shows in the tree, oldest
// first, but for those closed since. The tray is the Tester's own, never that
// of the desktop.
func (tt *Tester) Trays() []*Tray { return slices.Clone(tt.host.trays) }

// Notifications returns the notifications that gowaku.Notify showed in the
// tree, oldest first. Under a Tester, Notify only records them, and returns
// nil.
func (tt *Tester) Notifications() []Notification { return slices.Clone(tt.host.notifications) }

// ClickNotification clicks the last notification, which runs the
// OnNotificationClick of the first tray, as on the desktop, then settles. It
// fails the test if there is no notification, or no tray to hear it.
func (tt *Tester) ClickNotification() {
	tt.t.Helper()
	if len(tt.host.notifications) == 0 {
		tt.t.Fatal("gowakutest: ClickNotification without a notification")
	}
	if len(tt.host.trays) == 0 {
		tt.t.Fatal("gowakutest: ClickNotification without a tray to hear it")
	}
	tt.host.trays[0].send(platform.TrayEvent{Kind: platform.TrayNotificationClick})
}
