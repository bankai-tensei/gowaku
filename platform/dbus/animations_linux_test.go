package dbus

import (
	"os"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"
)

// fakeAnimations is the Settings interface of a portal that only knows
// enable-animations, and only through the deprecated Read, which wraps the
// value twice.
type fakeAnimations struct{ enabled bool }

func (f *fakeAnimations) Read(namespace, key string) (godbus.Variant, *godbus.Error) {
	if namespace == gnomeInterface && key == enableAnimationsKey {
		return godbus.MakeVariant(godbus.MakeVariant(f.enabled)), nil
	}
	return godbus.Variant{}, godbus.NewError("org.freedesktop.portal.Error.NotFound", nil)
}

// TestEnableAnimations_FakePortal stands in for xdg-desktop-portal. Like
// TestColorScheme_FakePortal, it needs a bus of its own: run it under
// dbus-run-session with GOWAKU_DBUS_TEST=1.
func TestEnableAnimations_FakePortal(t *testing.T) {
	if os.Getenv("GOWAKU_DBUS_TEST") == "" {
		t.Skip("GOWAKU_DBUS_TEST is not set")
	}
	server, err := godbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Close() //nolint:errcheck // the test is over
	}()
	if err := server.Export(&fakeAnimations{enabled: false}, portalPath, settingsInterface); err != nil {
		t.Fatal(err)
	}
	if reply, err := server.RequestName(portalName, godbus.NameFlagDoNotQueue); err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v %v", portalName, reply, err)
	}
	defer func() {
		_, _ = server.ReleaseName(portalName) //nolint:errcheck // the test is over
	}()

	if enabled, ok := ReadEnableAnimations(); !ok || enabled {
		t.Errorf("ReadEnableAnimations() = %v, %v; want false, true", enabled, ok)
	}
	changes := make(chan bool, 4)
	if !WatchEnableAnimations(func(enabled bool) { changes <- enabled }) {
		t.Fatal("WatchEnableAnimations found no bus")
	}
	emit := func(namespace, key string, value any) {
		t.Helper()
		if err := server.Emit(portalPath, settingsInterface+".SettingChanged", namespace, key, godbus.MakeVariant(value)); err != nil {
			t.Fatal(err)
		}
	}
	emit(gnomeInterface, "color-scheme", "prefer-dark")
	emit("org.example", enableAnimationsKey, false)
	emit(gnomeInterface, enableAnimationsKey, true)
	select {
	case got := <-changes:
		if !got {
			t.Errorf("got %v, want true", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no change arrived")
	}
	select {
	case got := <-changes:
		t.Errorf("unexpected change %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}
