package dbus

import (
	"os"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"
)

// fakeSettings is the Settings interface of a portal that keeps the color
// scheme in the namespace of an older backend, as a string.
type fakeSettings struct{ scheme string }

func (f *fakeSettings) ReadOne(namespace, key string) (godbus.Variant, *godbus.Error) {
	if namespace == gnomeInterface && key == colorSchemeKey {
		return godbus.MakeVariant(f.scheme), nil
	}
	return godbus.Variant{}, godbus.NewError("org.freedesktop.portal.Error.NotFound", nil)
}

// TestColorScheme_FakePortal stands in for xdg-desktop-portal on the session
// bus. It needs a bus of its own: run it under dbus-run-session with
// GOWAKU_DBUS_TEST=1.
func TestColorScheme_FakePortal(t *testing.T) {
	if os.Getenv("GOWAKU_DBUS_TEST") == "" {
		t.Skip("GOWAKU_DBUS_TEST is not set")
	}
	server, err := godbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Log(err)
		}
	}()
	settings := &fakeSettings{scheme: "prefer-dark"}
	if err := server.Export(settings, portalPath, settingsInterface); err != nil {
		t.Fatal(err)
	}
	if reply, err := server.RequestName(portalName, godbus.NameFlagDoNotQueue); err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v %v", portalName, reply, err)
	}

	if s, ok := ReadColorScheme(); !ok || s != PreferDark {
		t.Errorf("ReadColorScheme() = %v, %v; want dark", s, ok)
	}
	changes := make(chan ColorScheme, 4)
	if !WatchColorScheme(func(s ColorScheme) { changes <- s }) {
		t.Fatal("WatchColorScheme found no bus")
	}
	// Other settings do not matter.
	emit := func(namespace string, value any) {
		t.Helper()
		if err := server.Emit(portalPath, settingsInterface+".SettingChanged", namespace, colorSchemeKey, godbus.MakeVariant(value)); err != nil {
			t.Fatal(err)
		}
	}
	emit("org.example", uint32(1))
	emit(gnomeInterface, "prefer-light")
	emit(appearance, uint32(PreferDark))
	for _, want := range []ColorScheme{PreferLight, PreferDark} {
		select {
		case got := <-changes:
			if got != want {
				t.Errorf("change %v, want %v", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("no change to %v", want)
		}
	}
}
