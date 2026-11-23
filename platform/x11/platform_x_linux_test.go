//go:build gpu

package x11

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/keysym"
)

// These need an X server, but no GPU. They run with the GPU tests because CI has
// neither.

func newTestPlatform(t *testing.T) *Platform {
	t.Helper()
	p, err := New()
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(p.Destroy)
	return p
}

// drain handles everything the server has sent so far.
func drain(p *Platform, want func(*platform.Event) bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ev := p.WaitEvent(50 * time.Millisecond)
		if want != nil && want(&ev) {
			return true
		}
	}
	return false
}

func TestKeySym_UsesNumLockFromXKeyState(t *testing.T) {
	p := newTestPlatform(t)
	keycode := p.hot(p.x.XKeysymToKeycode, uintptr(p.display), uintptr(keysym.XK_KP_Left))
	if keycode == 0 {
		t.Skip("the X server has no keypad 4 key")
	}
	w := &Window{platform: p}
	e := &XKeyEvent{Display: p.display, Keycode: uint32(keycode)}
	if got := w.keySym(e); got != keysym.XK_KP_Left {
		t.Skipf("the X server maps keypad 4 to %#x without modifiers", got)
	}
	e.State = 1 << 4 // Num Lock is Mod2 in the core X keyboard state.
	if got := w.keySym(e); got != keysym.XK_KP_0+4 {
		t.Errorf("keySym with Num Lock = %#x, want %#x", got, keysym.XK_KP_0+4)
	}
	qKeycode := p.hot(p.x.XKeysymToKeycode, uintptr(p.display), uintptr('q'))
	if qKeycode != 0 {
		e = &XKeyEvent{Display: p.display, Keycode: uint32(qKeycode), State: ShiftMask | ControlMask}
		if got := w.keySym(e); got != 'q' {
			t.Errorf("keySym for Ctrl+Shift+Q = %#x, want 'q'", got)
		}
	}
}

func TestWaitEvent_DoesNotAllocateWhenIdle(t *testing.T) {
	p := newTestPlatform(t)
	if _, err := p.CreateWindow(platform.WindowOptions{}); err != nil {
		t.Fatal(err)
	}
	drain(p, nil)
	allocs := testing.AllocsPerRun(100, func() { p.WaitEvent(0) })
	if allocs != 0 {
		t.Errorf("WaitEvent allocates %v times", allocs)
	}
}

func TestWake_UnblocksWaitEventFromAnotherGoroutine(t *testing.T) {
	p := newTestPlatform(t)
	var woke atomic.Bool
	go func() {
		time.Sleep(20 * time.Millisecond)
		woke.Store(true)
		p.Wake()
		p.Wake() // merged with the first
	}()
	if ev := p.WaitEvent(-1); ev.Kind != platform.EventWake || !woke.Load() {
		t.Fatalf("got %v", ev.Kind)
	}
	if ev := p.WaitEvent(10 * time.Millisecond); ev.Kind != platform.EventNone {
		t.Errorf("the second wake was not merged: %v", ev.Kind)
	}
	// A wake after the first was delivered must work again.
	p.Wake()
	if ev := p.WaitEvent(time.Second); ev.Kind != platform.EventWake {
		t.Errorf("got %v", ev.Kind)
	}
}

func TestWaitEvent_TimesOut(t *testing.T) {
	p := newTestPlatform(t)
	start := time.Now()
	if ev := p.WaitEvent(30 * time.Millisecond); ev.Kind != platform.EventNone {
		t.Fatalf("got %v", ev.Kind)
	}
	if d := time.Since(start); d < 25*time.Millisecond || d > 500*time.Millisecond {
		t.Errorf("waited %v", d)
	}
}

// The watchers of the desktop interrupt the wait so that the loop reads their
// state. That is not a wake of the application.
func TestInterrupt_IsNotAWake(t *testing.T) {
	p := newTestPlatform(t)
	p.interrupt()
	if ev := p.WaitEvent(20 * time.Millisecond); ev.Kind != platform.EventNone {
		t.Errorf("got %v", ev.Kind)
	}
}

// The watchers of the desktop, and goroutines of the application, may wake a
// Platform that is gone.
func TestWake_AfterDestroyDoesNothing(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Skip(err)
	}
	p.Destroy()
	p.Wake()
	p.interrupt()
}

func TestCreateWindow_ReportsSizeAndExposure(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{Title: "テスト", Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	if width, height := w.PhysicalSize(); width != int(300*w.Scale()+0.5) || height != int(200*w.Scale()+0.5) {
		t.Errorf("size %d x %d at scale %v", width, height, w.Scale())
	}
	if !drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventRedraw && ev.Window == w.ID() }) {
		t.Error("no redraw after mapping")
	}
}

func TestClipboard_OwnTextComesBack(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w.SetClipboardText("日本語のテキスト")
	if got := w.ClipboardText(); got != "日本語のテキスト" {
		t.Errorf("got %q", got)
	}
}

// The second Platform is another client of the server, so that the text goes
// through the real selection protocol. The owner answers on a goroutine of its
// own, which is the UI goroutine of its Platform.
func TestClipboard_CrossesConnections(t *testing.T) {
	ready := make(chan *Platform)
	done := make(chan struct{})
	go func() {
		owner, err := New()
		if err != nil {
			close(ready)
			return
		}
		defer owner.Destroy()
		w, err := owner.CreateWindow(platform.WindowOptions{})
		if err != nil {
			close(ready)
			return
		}
		w.SetClipboardText("across")
		ready <- owner
		for {
			select {
			case <-done:
				return
			default:
				owner.WaitEvent(5 * time.Millisecond)
			}
		}
	}()
	if owner := <-ready; owner == nil {
		t.Skip("no X server")
	}
	defer close(done)
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.ClipboardText(); got != "across" {
		t.Errorf("got %q", got)
	}
}

func TestClose_WindowManagerRequestBecomesAnEvent(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	xid := w.(*Window).xid
	message := XClientMessageEvent{MessageType: p.atoms.WM_PROTOCOLS, Format: 32}
	message.Type, message.Display, message.Window = ClientMessage, p.display, xid
	message.Data[0] = uintptr(p.atoms.WM_DELETE_WINDOW)
	call(p.x.XSendEvent, uintptr(p.display), uintptr(xid), 0, 0, ptr(&message))
	if !drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventClose && ev.Window == w.ID() }) {
		t.Error("no close event")
	}
}

func TestClose_DestroyedFromOutsideBecomesAnEvent(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	call(p.x.XDestroyWindow, uintptr(p.display), uintptr(w.(*Window).xid))
	if !drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventDestroyed && ev.Window == w.ID() }) {
		t.Error("no destroyed event")
	}
}

func TestClose_IsIdempotentAndEventsStopMattering(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	w.Close()
	drain(p, nil)
}
