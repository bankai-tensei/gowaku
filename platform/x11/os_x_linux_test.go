//go:build gpu

package x11

import (
	"image"
	"image/color"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// noise is an image whose PNG is larger than incrThreshold, so that it goes by
// INCR.
func noise() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 500, 400))
	r := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(r.Uint32())
	}
	img.SetNRGBA(3, 4, color.NRGBA{1, 2, 3, 255})
	return img
}

// owned runs a second Platform, another client of the server, that puts c on
// the clipboard and serves it on its own goroutine until the test ends.
func owned(t *testing.T, c platform.ClipboardContent) {
	t.Helper()
	ready := make(chan bool)
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
		w.WriteClipboard(c)
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
		t.Skip("no X server")
	}
	t.Cleanup(func() { close(done) })
}

func TestClipboard_EveryKindCrossesConnectionsWithINCR(t *testing.T) {
	in := platform.ClipboardContent{
		Text: "和文", HTML: "<b>太字</b>", Image: noise(), Files: []string{"/tmp/メモ.txt", "/tmp/a b"},
	}
	owned(t, in)
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if k := w.ClipboardKinds(); k != in.Kinds() {
		t.Errorf("kinds %b, want %b", k, in.Kinds())
	}
	start := time.Now()
	out := w.ReadClipboard(in.Kinds())
	t.Logf("read %d pixels in %v", len(in.Image.Pix)/4, time.Since(start))
	if out.Text != in.Text || out.HTML != in.HTML || !slices.Equal(out.Files, in.Files) {
		t.Errorf("got %q %q %q", out.Text, out.HTML, out.Files)
	}
	if out.Image == nil || !slices.Equal(out.Image.Pix, in.Image.Pix) {
		t.Error("the image must cross whole")
	}
	if got := w.ClipboardText(); got != in.Text {
		t.Errorf("ClipboardText %q", got)
	}
}

// longs reads a property of 32-bit items.
func longs(p *Platform, w XID, property Atom) []Atom {
	value, _ := p.readProperty(w, property, false)
	size := int(unsafe.Sizeof(uintptr(0)))
	var out []Atom
	for i := 0; i+size <= len(value); i += size {
		out = append(out, Atom(*(*uintptr)(unsafe.Pointer(&value[i]))))
	}
	return out
}

func TestModal_TellsTheWindowManager(t *testing.T) {
	p := newTestPlatform(t)
	owner, err := p.CreateWindow(platform.WindowOptions{Title: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	modal, err := p.CreateWindow(platform.WindowOptions{Title: "modal", Owner: owner, Modal: true})
	if err != nil {
		t.Fatal(err)
	}
	a := &p.atoms
	m := modal.(*Window)
	if got := longs(p, m.xid, XA_WM_TRANSIENT_FOR); len(got) != 1 || XID(got[0]) != owner.(*Window).xid {
		t.Errorf("WM_TRANSIENT_FOR %v", got)
	}
	if got := longs(p, m.xid, a.NET_WM_WINDOW_TYPE); !slices.Contains(got, a.NET_WM_WINDOW_TYPE_DIALOG) {
		t.Errorf("_NET_WM_WINDOW_TYPE %v", got)
	}
	// The window manager owns _NET_WM_STATE once the window is mapped. One
	// that knows no modal state drops it, as the xwm of Weston does; the
	// application keeps the input from the owner either way.
	drain(p, nil)
	modalState := slices.Contains(longs(p, m.xid, a.NET_WM_STATE), a.NET_WM_STATE_MODAL)
	t.Logf("the window manager keeps the modal state: %v", modalState)
	modal.Activate()
}

func TestCustomFrame_AsksForNoDecorations(t *testing.T) {
	p := newTestPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{CustomFrame: true})
	if err != nil {
		t.Fatal(err)
	}
	if !w.ClientDecorated() {
		t.Error("a custom frame is drawn by the application")
	}
	hints := longs(p, w.(*Window).xid, p.atoms.MOTIF_WM_HINTS)
	if len(hints) != 5 || hints[0] != motifDecorations || hints[2] != 0 {
		t.Errorf("_MOTIF_WM_HINTS %v", hints)
	}
	plain, err := p.CreateWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plain.ClientDecorated() {
		t.Error("a plain window keeps the frame of the window manager")
	}
	// The window manager answers these with its own work; they must not fail.
	w.ToggleMaximize()
	drain(p, nil)
	t.Logf("maximized after the toggle: %v", w.Maximized())
}

// A drag from one client to a window of another, through XDND. Nothing moves
// the pointer here, so the source is driven through its own methods, on its
// own goroutine, with the places a pointer would report.
func TestDrag_CrossesConnectionsThroughXDND(t *testing.T) {
	target := newTestPlatform(t)
	tw, err := target.CreateWindow(platform.WindowOptions{Title: "target", Width: 200, Height: 150})
	if err != nil {
		t.Fatal(err)
	}
	var events []platform.DropEvent
	tw.SetDropHandler(func(e *platform.DropEvent) input.DropEffect {
		events = append(events, *e)
		return input.DropCopy
	})
	drain(target, nil)
	// The middle of the target window, on the root.
	var rx, ry int32
	var child XID
	call(target.x.XTranslateCoordinates, uintptr(target.display), uintptr(tw.(*Window).xid), uintptr(target.root),
		100, 75, ptr(&rx), ptr(&ry), ptr(&child))

	commands := make(chan func(*Platform, *Window))
	ended := make(chan input.DropEffect, 1)
	go func() {
		source, err := New()
		if err != nil {
			close(ended)
			return
		}
		defer source.Destroy()
		sw, err := source.CreateWindow(platform.WindowOptions{Title: "source"})
		if err != nil {
			close(ended)
			return
		}
		w := sw.(*Window)
		w.buttonsDown = 1 << input.ButtonPrimary
		w.StartDrag(input.DragData{Text: "和文", Files: []string{"/tmp/メモ.txt"}}, input.DropCopy|input.DropMove,
			func(e input.DropEffect) { ended <- e })
		for {
			select {
			case cmd, ok := <-commands:
				if !ok {
					return
				}
				cmd(source, w)
			default:
				source.WaitEvent(5 * time.Millisecond)
			}
		}
	}()
	defer close(commands)
	commands <- func(p *Platform, w *Window) {
		p.dragMotion(&XKeyEvent{XRoot: rx, YRoot: ry, Time: 1})
	}
	drain(target, func(*platform.Event) bool { return len(events) > 0 })
	commands <- func(p *Platform, w *Window) {
		// The target answered the position; the source reads the status.
		for i := 0; i < 100 && p.dragging != nil && !p.dragging.accepted; i++ {
			p.WaitEvent(10 * time.Millisecond)
		}
		p.dragRelease(&XKeyEvent{XRoot: rx, YRoot: ry, Time: 2})
	}
	drain(target, func(*platform.Event) bool {
		return len(events) > 0 && events[len(events)-1].Phase == platform.DropDrop
	})
	select {
	case e := <-ended:
		if e != input.DropCopy {
			t.Errorf("the source ended with %v, want a copy", e)
		}
	case <-time.After(2 * time.Second):
		t.Error("the source got no XdndFinished")
	}
	if len(events) < 2 {
		t.Fatalf("events %+v", events)
	}
	over, drop := events[0], events[len(events)-1]
	if over.Phase != platform.DropOver || over.Kinds != input.DataText|input.DataFiles || over.X != 100 || over.Y != 75 {
		t.Errorf("over %+v", over)
	}
	if drop.Phase != platform.DropDrop || drop.Data.Text != "和文" || !slices.Equal(drop.Data.Files, []string{"/tmp/メモ.txt"}) {
		t.Errorf("drop %+v", drop)
	}
}

func TestPinch_XInputVersion(t *testing.T) {
	p := newTestPlatform(t)
	t.Logf("XInput 2.%d: touches %v, pinches %v", p.xiMinor, p.xiOpcode != 0, p.xiOpcode != 0 && p.xiMinor >= 4)
}
