package dbus

import (
	"fmt"
	"image"
	"log/slog"
	"os"
	"strings"
	"sync"

	godbus "github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"github.com/bankai-tensei/gowaku/platform"
)

// Tray icons are StatusNotifierItems that a watcher (KDE Plasma, waybar, the
// AppIndicator extension of GNOME) shows, each with a com.canonical.dbusmenu
// menu that the panel draws. Notifications go to org.freedesktop.Notifications.
// Without a watcher or a notification service, CreateTray and Notify return
// ErrNoTray and ErrNoNotifications, and log the reason at Debug. See
// docs/ADR/0051-tray-and-notifications.md.

const (
	sniIface       = "org.kde.StatusNotifierItem"
	sniPath        = "/StatusNotifierItem"
	menuIface      = "com.canonical.dbusmenu"
	menuPath       = "/MenuBar"
	watcherName    = "org.kde.StatusNotifierWatcher"
	watcherPath    = "/StatusNotifierWatcher"
	notifyName     = "org.freedesktop.Notifications"
	notifyPath     = "/org/freedesktop/Notifications"
	notifyIface    = "org.freedesktop.Notifications"
	defaultAction  = "default"
	notifyExpireOS = -1
)

// Trays are the tray icons and notifications of one Platform. It is safe from
// any goroutine: D-Bus calls the icons on goroutines of its own.
type Trays struct {
	mu   sync.Mutex
	conn *godbus.Conn
	// trays are the icons, and next the number of the next one.
	trays []*Tray
	next  int
	// notifications holds the IDs of the notifications shown, which clicks
	// report.
	notifications map[uint32]bool
	watching      bool
}

// Tray is one icon, with its own connection: each StatusNotifierItem needs a
// bus name and paths of its own.
type Tray struct {
	parent   *Trays
	conn     *godbus.Conn
	props    *prop.Properties
	mu       sync.Mutex
	opts     platform.TrayOptions
	on       func(platform.TrayEvent)
	revision uint32
	closed   bool
}

// iconPixmap is an image of StatusNotifierItem: ARGB32 in network order.
type iconPixmap struct {
	W, H int32
	Data []byte
}

func pixmapsOf(img *image.NRGBA) []iconPixmap {
	if img == nil || img.Rect.Dx() == 0 {
		return []iconPixmap{}
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	data := make([]byte, 0, 4*w*h)
	for y := range h {
		row := img.Pix[y*img.Stride:]
		for x := range w {
			p := row[4*x:]
			data = append(data, p[3], p[0], p[1], p[2])
		}
	}
	return []iconPixmap{{W: int32(w), H: int32(h), Data: data}}
}

// toolTip is the ToolTip of StatusNotifierItem: icon name, icon, title, text.
type toolTip struct {
	Icon   string
	Pixmap []iconPixmap
	Title  string
	Text   string
}

// CreateTray implements platform.Trays.
func (t *Trays) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	conn, err := godbus.ConnectSessionBus()
	if err != nil {
		slog.Debug("dbus: no session bus for a tray", "err", err)
		return nil, platform.ErrNoTray
	}
	tr := &Tray{parent: t, conn: conn, opts: opts, on: on, revision: 1}
	t.mu.Lock()
	t.next++
	n := t.next
	t.mu.Unlock()
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-%d", os.Getpid(), n)
	if err := tr.export(); err != nil {
		slog.Debug("dbus: export a tray", "err", err)
		conn.Close() //nolint:errcheck,gosec // nothing to do about it
		return nil, platform.ErrNoTray
	}
	if reply, err := conn.RequestName(name, godbus.NameFlagDoNotQueue); err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
		slog.Debug("dbus: take the name of a tray", "err", err)
		conn.Close() //nolint:errcheck,gosec // nothing to do about it
		return nil, platform.ErrNoTray
	}
	call := conn.Object(watcherName, watcherPath).Call(watcherName+".RegisterStatusNotifierItem", 0, name)
	if call.Err != nil {
		slog.Debug("dbus: no StatusNotifierWatcher", "err", call.Err)
		conn.Close() //nolint:errcheck,gosec // nothing to do about it
		return nil, platform.ErrNoTray
	}
	t.mu.Lock()
	t.trays = append(t.trays, tr)
	t.mu.Unlock()
	return tr, nil
}

// export puts the item and its menu on the connection.
func (tr *Tray) export() error {
	c := tr.conn
	if err := c.Export(sniMethods{tr}, sniPath, sniIface); err != nil {
		return err
	}
	if err := c.Export(menuMethods{tr}, menuPath, menuIface); err != nil {
		return err
	}
	props, err := prop.Export(c, sniPath, prop.Map{sniIface: tr.sniProps()})
	if err != nil {
		return err
	}
	tr.props = props
	_, err = prop.Export(c, menuPath, prop.Map{menuIface: {
		"Version":       {Value: uint32(3)},
		"TextDirection": {Value: "ltr"},
		"Status":        {Value: "normal"},
		"IconThemePath": {Value: []string{}},
	}})
	return err
}

func (tr *Tray) sniProps() map[string]*prop.Prop {
	o := tr.opts
	return map[string]*prop.Prop{
		"Category":   {Value: "ApplicationStatus"},
		"Id":         {Value: "gowaku"},
		"Title":      {Value: o.Tooltip},
		"Status":     {Value: "Active"},
		"WindowId":   {Value: int32(0)},
		"IconName":   {Value: ""},
		"IconPixmap": {Value: pixmapsOf(o.Icon)},
		"ToolTip":    {Value: toolTip{Pixmap: []iconPixmap{}, Title: o.Tooltip}},
		"ItemIsMenu": {Value: false},
		"Menu":       {Value: godbus.ObjectPath(menuPath)},
	}
}

// Update implements platform.Tray.
func (tr *Tray) Update(opts platform.TrayOptions) {
	tr.mu.Lock()
	if tr.closed {
		tr.mu.Unlock()
		return
	}
	tr.opts = opts
	tr.revision++
	revision := tr.revision
	tr.mu.Unlock()
	tr.props.SetMust(sniIface, "Title", opts.Tooltip)
	tr.props.SetMust(sniIface, "IconPixmap", pixmapsOf(opts.Icon))
	tr.props.SetMust(sniIface, "ToolTip", toolTip{Pixmap: []iconPixmap{}, Title: opts.Tooltip})
	for _, signal := range []string{"NewIcon", "NewTitle", "NewToolTip"} {
		_ = tr.conn.Emit(sniPath, sniIface+"."+signal) //nolint:errcheck // a panel that misses it reads again
	}
	_ = tr.conn.Emit(menuPath, menuIface+".LayoutUpdated", revision, int32(0)) //nolint:errcheck // see above
}

// Close implements platform.Tray.
func (tr *Tray) Close() {
	tr.mu.Lock()
	if tr.closed {
		tr.mu.Unlock()
		return
	}
	tr.closed = true
	tr.mu.Unlock()
	t := tr.parent
	t.mu.Lock()
	for i, o := range t.trays {
		if o == tr {
			t.trays = append(t.trays[:i], t.trays[i+1:]...)
			break
		}
	}
	t.mu.Unlock()
	// The watcher forgets the item when its name leaves the bus.
	tr.conn.Close() //nolint:errcheck,gosec // nothing to do about it
}

func (tr *Tray) emit(e platform.TrayEvent) {
	tr.mu.Lock()
	on, closed := tr.on, tr.closed
	tr.mu.Unlock()
	if on != nil && !closed {
		on(e)
	}
}

// sniMethods are the methods of org.kde.StatusNotifierItem.
type sniMethods struct{ tr *Tray }

// Activate is a click on the icon.
func (m sniMethods) Activate(int32, int32) *godbus.Error {
	m.tr.emit(platform.TrayEvent{Kind: platform.TrayActivate})
	return nil
}

// SecondaryActivate is a click of the middle button.
func (m sniMethods) SecondaryActivate(int32, int32) *godbus.Error { return nil }

// ContextMenu is never asked for: the item has a Menu.
func (m sniMethods) ContextMenu(int32, int32) *godbus.Error { return nil }

// Scroll is the wheel over the icon.
func (m sniMethods) Scroll(int32, string) *godbus.Error { return nil }

// menuLayout is an item of com.canonical.dbusmenu with its children.
type menuLayout struct {
	ID       int32
	Props    map[string]godbus.Variant
	Children []godbus.Variant
}

// menuProps is an item without its children, for GetGroupProperties.
type menuProps struct {
	ID    int32
	Props map[string]godbus.Variant
}

// menuEvent is an entry of EventGroup.
type menuEvent struct {
	ID        int32
	EventID   string
	Data      godbus.Variant
	Timestamp uint32
}

// dbusmenuLabel turns the & of a mnemonic into the _ of dbusmenu.
func dbusmenuLabel(s string) string {
	s = strings.ReplaceAll(s, "_", "__")
	s = strings.ReplaceAll(s, "&&", "\x00")
	s = strings.ReplaceAll(s, "&", "_")
	return strings.ReplaceAll(s, "\x00", "&")
}

// itemProps are the properties of item.
func itemProps(it *platform.TrayItem) map[string]godbus.Variant {
	if it.Separator {
		return map[string]godbus.Variant{"type": godbus.MakeVariant("separator")}
	}
	p := map[string]godbus.Variant{
		"label":   godbus.MakeVariant(dbusmenuLabel(it.Label)),
		"enabled": godbus.MakeVariant(it.Enabled),
		"visible": godbus.MakeVariant(true),
	}
	if it.Checkable {
		state := int32(0)
		if it.Checked {
			state = 1
		}
		p["toggle-type"] = godbus.MakeVariant("checkmark")
		p["toggle-state"] = godbus.MakeVariant(state)
	}
	if len(it.Items) > 0 {
		p["children-display"] = godbus.MakeVariant("submenu")
	}
	return p
}

// layout builds the tree of items below the item with id, which is the ID of
// a TrayItem plus one, or zero for the root.
func layout(items []platform.TrayItem, id int32, props map[string]godbus.Variant, depth int32) menuLayout {
	l := menuLayout{ID: id, Props: props, Children: []godbus.Variant{}}
	if depth == 0 {
		return l
	}
	for i := range items {
		it := &items[i]
		l.Children = append(l.Children, godbus.MakeVariant(layout(it.Items, int32(it.ID+1), itemProps(it), depth-1)))
	}
	return l
}

// find returns the item with the dbusmenu id below items.
func find(items []platform.TrayItem, id int32) *platform.TrayItem {
	for i := range items {
		if int32(items[i].ID+1) == id {
			return &items[i]
		}
		if it := find(items[i].Items, id); it != nil {
			return it
		}
	}
	return nil
}

// menuMethods are the methods of com.canonical.dbusmenu.
type menuMethods struct{ tr *Tray }

func (m menuMethods) items() ([]platform.TrayItem, uint32) {
	m.tr.mu.Lock()
	defer m.tr.mu.Unlock()
	return m.tr.opts.Items, m.tr.revision
}

// GetLayout returns the tree below parentID.
func (m menuMethods) GetLayout(parentID, depth int32, _ []string) (uint32, menuLayout, *godbus.Error) {
	items, revision := m.items()
	root := map[string]godbus.Variant{"children-display": godbus.MakeVariant("submenu")}
	if parentID == 0 {
		return revision, layout(items, 0, root, depth), nil
	}
	it := find(items, parentID)
	if it == nil {
		return revision, menuLayout{ID: parentID, Props: map[string]godbus.Variant{}, Children: []godbus.Variant{}}, nil
	}
	return revision, layout(it.Items, parentID, itemProps(it), depth), nil
}

// GetGroupProperties returns the properties of the items with ids.
func (m menuMethods) GetGroupProperties(ids []int32, _ []string) ([]menuProps, *godbus.Error) {
	items, _ := m.items()
	out := []menuProps{}
	for _, id := range ids {
		if it := find(items, id); it != nil {
			out = append(out, menuProps{ID: id, Props: itemProps(it)})
		}
	}
	return out, nil
}

// GetProperty returns one property of an item.
func (m menuMethods) GetProperty(id int32, name string) (godbus.Variant, *godbus.Error) {
	items, _ := m.items()
	if it := find(items, id); it != nil {
		if v, ok := itemProps(it)[name]; ok {
			return v, nil
		}
	}
	return godbus.MakeVariant(""), nil
}

// Event reports a click on an item.
func (m menuMethods) Event(id int32, eventID string, _ godbus.Variant, _ uint32) *godbus.Error {
	if eventID == "clicked" && id > 0 {
		m.tr.emit(platform.TrayEvent{Kind: platform.TrayChoose, Item: int(id - 1)})
	}
	return nil
}

// EventGroup is Event for several items.
func (m menuMethods) EventGroup(events []menuEvent) ([]int32, *godbus.Error) {
	for _, e := range events {
		_ = m.Event(e.ID, e.EventID, e.Data, e.Timestamp) //nolint:errcheck // Event never fails
	}
	return []int32{}, nil
}

// AboutToShow says that nothing changes when a submenu opens.
func (m menuMethods) AboutToShow(int32) (bool, *godbus.Error) { return false, nil }

// AboutToShowGroup is AboutToShow for several items.
func (m menuMethods) AboutToShowGroup([]int32) ([]int32, []int32, *godbus.Error) {
	return []int32{}, []int32{}, nil
}

// Notify implements platform.Trays.
func (t *Trays) Notify(title, body string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil {
		conn, err := godbus.ConnectSessionBus()
		if err != nil {
			slog.Debug("dbus: no session bus for notifications", "err", err)
			return platform.ErrNoNotifications
		}
		t.conn = conn
	}
	var id uint32
	err := t.conn.Object(notifyName, notifyPath).Call(notifyIface+".Notify", 0,
		"gowaku", uint32(0), "", title, body, []string{defaultAction, "Open"},
		map[string]godbus.Variant{}, int32(notifyExpireOS)).Store(&id)
	if err != nil {
		slog.Debug("dbus: no notification service", "err", err)
		return platform.ErrNoNotifications
	}
	if t.notifications == nil {
		t.notifications = map[uint32]bool{}
	}
	t.notifications[id] = true
	if !t.watching {
		t.watching = true
		t.watchClicks()
	}
	return nil
}

// watchClicks reports clicks on the notifications of the application to the
// first tray.
func (t *Trays) watchClicks() {
	if err := t.conn.AddMatchSignal(godbus.WithMatchInterface(notifyIface), godbus.WithMatchMember("ActionInvoked")); err != nil {
		return
	}
	signals := make(chan *godbus.Signal, 8)
	t.conn.Signal(signals)
	go func() {
		for s := range signals {
			if len(s.Body) < 2 {
				continue
			}
			id, _ := s.Body[0].(uint32)
			t.mu.Lock()
			ours := t.notifications[id]
			var first *Tray
			if len(t.trays) > 0 {
				first = t.trays[0]
			}
			t.mu.Unlock()
			if ours && first != nil {
				first.emit(platform.TrayEvent{Kind: platform.TrayNotificationClick})
			}
		}
	}()
}

// Close takes every icon away, when the platform goes.
func (t *Trays) Close() {
	t.mu.Lock()
	trays := append([]*Tray(nil), t.trays...)
	conn := t.conn
	t.conn = nil
	t.mu.Unlock()
	for _, tr := range trays {
		tr.Close()
	}
	if conn != nil {
		conn.Close() //nolint:errcheck,gosec // nothing to do about it
	}
}
