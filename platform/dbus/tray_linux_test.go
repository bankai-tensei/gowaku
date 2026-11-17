package dbus

import (
	"errors"
	"os"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/platform"
)

// fakeWatcher is a panel: it takes the items that register.
type fakeWatcher struct{ items chan string }

func (w *fakeWatcher) RegisterStatusNotifierItem(service string) *godbus.Error {
	w.items <- service
	return nil
}

// fakeNotifications is a notification service.
type fakeNotifications struct{ shown chan [2]string }

func (n *fakeNotifications) Notify(_ string, _ uint32, _, summary, body string, _ []string,
	_ map[string]godbus.Variant, _ int32,
) (uint32, *godbus.Error) {
	n.shown <- [2]string{summary, body}
	return 42, nil
}

func TestDbusmenuLabel(t *testing.T) {
	for in, want := range map[string]string{"開く(&O)": "開く(_O)", "a_b": "a__b", "Save && Quit": "Save & Quit"} {
		if got := dbusmenuLabel(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// TestTray_FakePanel stands in for a panel and a notification service on the
// session bus, and plays the panel: it reads the item, its menu, clicks an
// item and the icon, and a notification. Run it under dbus-run-session with
// GOWAKU_DBUS_TEST=1.
func TestTray_FakePanel(t *testing.T) {
	if os.Getenv("GOWAKU_DBUS_TEST") == "" {
		t.Skip("GOWAKU_DBUS_TEST is not set")
	}
	trays := &Trays{}
	if _, err := trays.CreateTray(platform.TrayOptions{}, nil); !errors.Is(err, platform.ErrNoTray) {
		t.Errorf("without a watcher: %v, want ErrNoTray", err)
	}
	if err := trays.Notify("a", "b"); !errors.Is(err, platform.ErrNoNotifications) {
		t.Errorf("without a service: %v, want ErrNoNotifications", err)
	}

	panel, err := godbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer panel.Close() //nolint:errcheck // the test is over
	watcher := &fakeWatcher{items: make(chan string, 1)}
	notifications := &fakeNotifications{shown: make(chan [2]string, 1)}
	for _, x := range []struct {
		v          any
		path, name string
	}{{watcher, watcherPath, watcherName}, {notifications, notifyPath, notifyName}} {
		if err := panel.Export(x.v, godbus.ObjectPath(x.path), x.name); err != nil {
			t.Fatal(err)
		}
		if reply, err := panel.RequestName(x.name, godbus.NameFlagDoNotQueue); err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
			t.Fatalf("own %s: %v %v", x.name, reply, err)
		}
	}

	events := make(chan platform.TrayEvent, 4)
	tr, err := trays.CreateTray(platform.TrayOptions{Tooltip: "tip", Items: []platform.TrayItem{
		{ID: 0, Label: "開く(&O)", Enabled: true},
		{ID: 1, Separator: true},
		{ID: 2, Label: "表示", Enabled: true, Items: []platform.TrayItem{{ID: 3, Label: "手前", Enabled: true, Checkable: true, Checked: true}}},
	}}, func(e platform.TrayEvent) { events <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer trays.Close()
	var service string
	select {
	case service = <-watcher.items:
	case <-time.After(2 * time.Second):
		t.Fatal("the item did not register")
	}
	item := panel.Object(service, sniPath)
	title, err := item.GetProperty(sniIface + ".Title")
	if err != nil || title.Value() != "tip" {
		t.Errorf("Title %v %v", title, err)
	}
	menuPathV, err := item.GetProperty(sniIface + ".Menu")
	if err != nil || menuPathV.Value() != godbus.ObjectPath(menuPath) {
		t.Errorf("Menu %v %v", menuPathV, err)
	}

	var revision uint32
	var root menuLayout
	if err := panel.Object(service, menuPath).Call(menuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).
		Store(&revision, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 {
		t.Fatalf("root %+v", root)
	}
	sub, ok := root.Children[2].Value().([]any)
	if !ok || len(sub) != 3 {
		t.Fatalf("submenu %#v", root.Children[2].Value())
	}
	first, _ := root.Children[0].Value().([]any)
	if props, _ := first[1].(map[string]godbus.Variant); props["label"].Value() != "開く(_O)" {
		t.Errorf("first item %v", first)
	}
	if err := panel.Object(service, menuPath).Call(menuIface+".Event", 0, int32(4), "clicked",
		godbus.MakeVariant(""), uint32(0)).Err; err != nil {
		t.Fatal(err)
	}
	if err := item.Call(sniIface+".Activate", 0, int32(0), int32(0)).Err; err != nil {
		t.Fatal(err)
	}
	for _, want := range []platform.TrayEvent{{Kind: platform.TrayChoose, Item: 3}, {Kind: platform.TrayActivate}} {
		select {
		case e := <-events:
			if e != want {
				t.Errorf("got %+v, want %+v", e, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %+v", want)
		}
	}

	if err := trays.Notify("題", "本文"); err != nil {
		t.Fatal(err)
	}
	if shown := <-notifications.shown; shown != [2]string{"題", "本文"} {
		t.Errorf("shown %v", shown)
	}
	if err := panel.Emit(notifyPath, notifyIface+".ActionInvoked", uint32(42), "default"); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Kind != platform.TrayNotificationClick {
			t.Errorf("got %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Error("the click on the notification did not arrive")
	}
	tr.Update(platform.TrayOptions{Tooltip: "new"})
	if title, err := item.GetProperty(sniIface + ".Title"); err != nil || title.Value() != "new" {
		t.Errorf("Title after Update %v %v", title, err)
	}
}
