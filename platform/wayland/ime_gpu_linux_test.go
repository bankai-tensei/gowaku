//go:build gpu

package wayland

import (
	"os"
	"strings"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/platform"
)

const remoteDesktop = "org.gnome.Mutter.RemoteDesktop"

// startMutterSession starts a session of the RemoteDesktop interface of
// mutter, which gives the seat a keyboard that needs no device: headless,
// mutter has none, and no window gets the focus. Mutter ends the session when
// the client that made it goes, so it lives as long as its connection.
func startMutterSession(t *testing.T) godbus.BusObject {
	t.Helper()
	c, err := godbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	var path godbus.ObjectPath
	if err := c.Object(remoteDesktop, "/org/gnome/Mutter/RemoteDesktop").Call(remoteDesktop+".CreateSession", 0).Store(&path); err != nil {
		t.Fatal(err)
	}
	session := c.Object(remoteDesktop, path)
	if err := session.Call(remoteDesktop+".Session.Start", 0).Err; err != nil {
		t.Fatal(err)
	}
	// The virtual keyboard comes with its first key: a Shift.
	for _, down := range []bool{true, false} {
		if err := session.Call(remoteDesktop+".Session.NotifyKeyboardKeysym", 0, uint32(0xffe1), down).Err; err != nil {
			t.Fatal(err)
		}
	}
	return session
}

// typeWithMutter presses keys through a session of startMutterSession. Zero
// waits a moment.
func typeWithMutter(t *testing.T, session godbus.BusObject, keysyms []uint32) {
	name := remoteDesktop
	time.Sleep(500 * time.Millisecond)
	for _, k := range keysyms {
		if k == 0 {
			time.Sleep(800 * time.Millisecond)
			continue
		}
		for _, down := range []bool{true, false} {
			// Below 0x100 these are evdev key codes, which go through the
			// keymap as a real keyboard would; above, keysyms.
			method := ".Session.NotifyKeyboardKeysym"
			if k < 0x100 {
				method = ".Session.NotifyKeyboardKeycode"
			}
			if err := session.Call(name+method, 0, k, down).Err; err != nil {
				t.Error(err)
				return
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// TestIME_ThroughTheCompositor turns the IME on in a window and waits for an
// input method to compose にほんご and commit 日本語. It needs a compositor
// with text-input-v3, an input method with Mozc, and someone typing
// "nihongo", Space and Enter once the file named by GOWAKU_IME_TEST says
// "ready", for example ibus with Mozc on GNOME.
func TestIME_ThroughTheCompositor(t *testing.T) {
	flag := os.Getenv("GOWAKU_IME_TEST")
	if flag == "" {
		t.Skip("GOWAKU_IME_TEST is not set")
	}
	p := newTestPlatform(t)
	if p.textInputs == 0 {
		t.Fatal("the compositor has no zwp_text_input_manager_v3")
	}
	w, err := p.CreateWindow(platform.WindowOptions{Title: "gowaku ime test", Width: 400, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("client decorated: %v, fractional scale: %v", w.ClientDecorated(), p.fractional != 0)
	var session godbus.BusObject
	if os.Getenv("GOWAKU_IME_KEYS") == "mutter" {
		session = startMutterSession(t)
	}
	mapWindow(t, w)
	focused := false
	for deadline := time.Now().Add(10 * time.Second); !focused && time.Now().Before(deadline); {
		focused = p.WaitEvent(50*time.Millisecond).Kind == platform.EventFocus
	}
	if !focused {
		t.Fatal("the window got no keyboard focus")
	}
	w.SetIMEEnabled(true)
	w.SetIMECursorRect(20, 40, 2, 24)
	if err := os.WriteFile(flag, []byte("ready\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if session != nil {
		// Henkan turns Mozc on; then n i h o n g o, Space and Enter as evdev codes.
		go typeWithMutter(t, session, []uint32{0xff23, 0, 49, 23, 35, 24, 49, 34, 24, 0, 57, 0, 28})
	}

	var compositions, texts []string
	composing := false
	for deadline := time.Now().Add(40 * time.Second); time.Now().Before(deadline); {
		ev := p.WaitEvent(50 * time.Millisecond)
		switch ev.Kind {
		case platform.EventComposition:
			if ev.Composition == nil {
				composing = false
				continue
			}
			composing = true
			if n := len(compositions); n == 0 || compositions[n-1] != ev.Composition.Text {
				compositions = append(compositions, ev.Composition.Text)
			}
		case platform.EventText:
			texts = append(texts, ev.Text)
		}
		if strings.Contains(strings.Join(texts, ""), "日本語") && !composing {
			break
		}
	}
	t.Logf("compositions %q", compositions)
	t.Logf("commits %q", texts)
	found := false
	for _, c := range compositions {
		if c == "にほんご" {
			found = true
		}
	}
	if !found {
		t.Error("never showed the composition にほんご")
	}
	if got := strings.Join(texts, ""); !strings.Contains(got, "日本語") {
		t.Errorf("committed %q, want 日本語", got)
	}
	if composing {
		t.Error("a composition is still shown after the commit")
	}
}
