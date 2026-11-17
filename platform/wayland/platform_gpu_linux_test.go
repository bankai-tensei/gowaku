//go:build gpu

package wayland

import (
	"image"
	"os"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/platform"
)

// These need a Wayland compositor, but no GPU. They run with the GPU tests
// because CI has neither. A headless sway (WLR_BACKENDS=headless) is enough.

func newTestPlatform(t *testing.T) *Platform {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("WAYLAND_DISPLAY is not set")
	}
	p, err := New()
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(p.Destroy)
	return p
}

// mapWindow gives w a buffer of one color, which is what makes a compositor
// show it and give it the keyboard. Applications get theirs from the GPU.
func mapWindow(t *testing.T, w platform.Window) {
	t.Helper()
	ww := w.(*Window)
	c := ww.p.c
	width, height := w.PhysicalSize()
	size := width * height * 4
	fd, err := unix.MemfdCreate("gowaku-test", unix.MFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := unix.Ftruncate(fd, int64(size)); err != nil {
		t.Fatal(err)
	}
	pool := ww.p.shm.createPool(c, int32(fd), int32(size))
	buffer := pool.createBuffer(c, 0, int32(width), int32(height), int32(width*4), wlShmFormatXrgb8888)
	pool.destroy(c)
	ww.surface.attach(c, buffer, 0, 0)
	ww.surface.commit(c)
	t.Cleanup(func() { c.destroy(proxy(buffer)) })
}

// drain handles what the compositor sends for a while, and reports whether
// want took an event.
func drain(p *Platform, want func(*platform.Event) bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ev := p.WaitEvent(50 * time.Millisecond)
		if want != nil && want(&ev) {
			return true
		}
	}
	return false
}

func TestCreateWindow_IsConfiguredWithItsSize(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{Title: "テスト", Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	ww := w.(*Window)
	if !ww.configured {
		t.Fatal("CreateWindow returned before the first configure")
	}
	// A tiling compositor may choose another size; the first resize says which.
	var got platform.Event
	if !drain(p, func(ev *platform.Event) bool {
		got = *ev
		return ev.Kind == platform.EventResize && ev.Window == w.ID()
	}) {
		t.Fatal("no resize after the first configure")
	}
	width, height := w.PhysicalSize()
	if got.PhysicalWidth != width || got.PhysicalHeight != height || got.Scale != w.Scale() {
		t.Errorf("resize %dx%d at %v, window %dx%d at %v", got.PhysicalWidth, got.PhysicalHeight, got.Scale, width, height, w.Scale())
	}
	if _, ok := w.NativeHandle().(platform.WaylandHandle); !ok {
		t.Errorf("handle %T", w.NativeHandle())
	}
	t.Logf("%dx%d at scale %v, client decorated %v", width, height, w.Scale(), w.ClientDecorated())
}

func TestCreateWindow_DecorationsFollowTheCompositor(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := p.decorations == 0; w.ClientDecorated() != want {
		t.Errorf("ClientDecorated %v with decoration manager %v", w.ClientDecorated(), p.decorations != 0)
	}
	p.clientDecorations = true
	w2, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !w2.ClientDecorated() {
		t.Error("GOWAKU_WAYLAND_DECORATIONS=client must draw the title bar in the application")
	}
}

func TestWaitEvent_DoesNotAllocateWhenIdle(t *testing.T) {
	p := newTestPlatform(t)
	if _, err := p.CreateWindow(platform.WindowOptions{}); err != nil {
		t.Fatal(err)
	}
	drain(p, nil)
	if allocs := testing.AllocsPerRun(100, func() { p.WaitEvent(0) }); allocs != 0 {
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
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("WAYLAND_DISPLAY is not set")
	}
	p, err := New()
	if err != nil {
		t.Skip(err)
	}
	p.Destroy()
	p.Wake()
	p.interrupt()
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

// The owner is another client, so that the text goes through the compositor.
// A compositor gives the selection only to a client with keyboard focus, so
// this needs one that focuses new windows, as sway does.
func TestClipboard_CrossesConnections(t *testing.T) {
	ready := make(chan bool, 1)
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
		mapWindow(t, w)
		// Taking the selection needs the serial of an input event.
		if !drain(owner, func(ev *platform.Event) bool { return ev.Kind == platform.EventFocus }) {
			ready <- false
			return
		}
		w.SetClipboardText("across")
		owner.WaitEvent(50 * time.Millisecond)
		ready <- true
		for {
			select {
			case <-done:
				return
			default:
				owner.WaitEvent(5 * time.Millisecond)
			}
		}
	}()
	if !<-ready {
		t.Skip("no compositor, or it gives no keyboard focus (a headless one needs a keyboard)")
	}
	defer close(done)
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, w)
	drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventFocus })
	if got := w.ClipboardText(); got != "across" {
		t.Errorf("got %q", got)
	}
}

// The dispatcher is a purego callback, and each event goes through reflection.
// This measures what that costs a pointer motion, from the socket to the
// platform.Event. It needs a wlroots compositor (sway.sh).
func TestPointerMotion_Allocations(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{Width: 400, Height: 300})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, w)
	drain(p, nil)
	v := newVirtualPointer(t)
	const layoutW, layoutH, moves = 4000, 4000, 200
	var got, mallocs uint64
	var stats runtime.MemStats
	for i := range moves {
		// The window is somewhere on the output: sweep a small square in the
		// middle of the layout, which a floating window covers.
		v.moveTo(t, 1990+uint32(i%20), 1990+uint32(i/20), layoutW, layoutH)
		runtime.ReadMemStats(&stats)
		before := stats.Mallocs
		for {
			ev := p.WaitEvent(20 * time.Millisecond)
			if ev.Kind == platform.EventNone {
				break
			}
			if ev.Kind == platform.EventPointerMove {
				got++
			}
		}
		runtime.ReadMemStats(&stats)
		mallocs += stats.Mallocs - before
	}
	if got == 0 {
		t.Skip("the pointer never reached the window")
	}
	t.Logf("%d motions reached the window, %.1f allocations each", got, float64(mallocs)/float64(got))
}

func TestClose_IsIdempotent(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	w.Close()
	drain(p, nil)
}

// The same as above with every kind, and an image of half a megabyte.
func TestClipboard_EveryKindCrossesConnections(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7919 >> 3)
	}
	in := platform.ClipboardContent{Text: "和文", HTML: "<b>太字</b>", Image: img, Files: []string{"/tmp/メモ.txt"}}
	ready := make(chan bool, 1)
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
		mapWindow(t, w)
		if !drain(owner, func(ev *platform.Event) bool { return ev.Kind == platform.EventFocus }) {
			ready <- false
			return
		}
		w.WriteClipboard(in)
		owner.WaitEvent(50 * time.Millisecond)
		ready <- true
		for {
			select {
			case <-done:
				return
			default:
				owner.WaitEvent(5 * time.Millisecond)
			}
		}
	}()
	if !<-ready {
		t.Skip("no compositor, or it gives no keyboard focus (a headless one needs a keyboard)")
	}
	defer close(done)
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, w)
	drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventFocus })
	if k := w.ClipboardKinds(); k != in.Kinds() {
		t.Errorf("kinds %b, want %b", k, in.Kinds())
	}
	out := w.ReadClipboard(in.Kinds())
	if out.Text != in.Text || out.HTML != in.HTML || !slices.Equal(out.Files, in.Files) {
		t.Errorf("got %q %q %q", out.Text, out.HTML, out.Files)
	}
	if out.Image == nil || !slices.Equal(out.Image.Pix, in.Image.Pix) {
		t.Error("the image must cross whole")
	}
}
