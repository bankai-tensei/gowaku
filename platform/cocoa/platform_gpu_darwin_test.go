//go:build gpu && darwin && arm64

package cocoa

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// onMain runs the functions of the tests on the main thread, where AppKit
// works. Tests run on other goroutines.
var onMain = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case fn := <-onMain:
			fn()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// main runs fn on the main thread and waits for it. fn must not call t.Fatal
// or t.Skip: they end the goroutine of the main thread, and the test hangs.
func main(fn func()) {
	finished := make(chan struct{})
	onMain <- func() { fn(); close(finished) }
	<-finished
}

// waitFor pumps events until one of kind arrives, or a second passes.
func waitFor(p *Platform, kind platform.EventKind) (platform.Event, bool) {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ev := p.WaitEvent(50 * time.Millisecond)
		if ev.Kind == kind {
			return ev, true
		}
	}
	return platform.Event{}, false
}

// postKey posts a key down of AppKit to w, as the keyboard would. The last
// two arguments, isARepeat and keyCode, go on the stack, where the Apple ABI
// packs them by size into one slot: a BOOL at byte 0 and the uint16 at byte 2.
// purego gives each argument a slot of its own, so they go in as one.
func postKey(p *Platform, w *Window, chars string, code uint16, flags uint64) {
	p.libs.pool(func() {
		ev := class("NSEvent").Send(sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
			uint64(10), NSPoint{}, flags, float64(0), objc.Send[int64](w.window, sel("windowNumber")), objc.ID(0),
			nsString(chars), nsString(chars), uint64(code)<<16)
		p.app.Send(sel("postEvent:atStart:"), ev, false)
	})
}

func TestPlatform_WindowKeysAndQuit(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "test", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		if width, height := w.PhysicalSize(); width != int(200*w.Scale()) || height != int(100*w.Scale()) {
			t.Errorf("size %dx%d at scale %v", width, height, w.Scale())
		}
		if h, ok := w.NativeHandle().(platform.CocoaHandle); !ok || h.Layer == 0 {
			t.Error("no CAMetalLayer")
		}

		// A wake from another goroutine ends a wait that would last forever.
		go func() {
			time.Sleep(20 * time.Millisecond)
			p.Wake()
		}()
		if _, ok := waitFor(p, platform.EventWake); !ok {
			t.Error("Wake did not end the wait")
		}

		w.SetIMEEnabled(true)
		postKey(p, w, "a", kVK_ANSI_A, 0)
		if ev, ok := waitFor(p, platform.EventKeyDown); !ok || ev.Key != input.KeyA {
			t.Errorf("key down: got %+v", ev)
		}
		if ev, ok := waitFor(p, platform.EventText); !ok || ev.Text != "a" {
			t.Errorf("text: got %+v", ev)
		}
		postKey(p, w, "c", kVK_ANSI_C, NSEventModifierFlagCommand)
		if ev, ok := waitFor(p, platform.EventKeyDown); !ok || ev.Key != input.KeyC || ev.Modifiers != input.ModControl {
			t.Errorf("Command+C: got %+v", ev)
		}

		// Quit, from the menu or the Dock, asks the application, which quits
		// by closing its windows.
		p.app.Send(sel("terminate:"), objc.ID(0))
		if ev, ok := waitFor(p, platform.EventQuit); !ok || ev.Window != 0 {
			t.Errorf("terminate: got %+v", ev)
		}

		// A hidden window leaves the screen, and a click on the Dock icon
		// while none shows asks the application to show it again.
		w.SetVisible(false)
		if objc.Send[bool](w.window, sel("isVisible")) {
			t.Error("SetVisible(false) left the window on the screen")
		}
		p.libs.pool(func() {
			p.appDelegate.Send(sel("applicationShouldHandleReopen:hasVisibleWindows:"), p.app, false)
		})
		if _, ok := waitFor(p, platform.EventReopen); !ok {
			t.Error("a reopen without visible windows sent no EventReopen")
		}
		w.SetVisible(true)
		if !objc.Send[bool](w.window, sel("isVisible")) {
			t.Error("SetVisible(true) did not show the window")
		}

		// The clipboard is the user's: put back what was on it.
		saved := w.ClipboardText()
		defer w.SetClipboardText(saved)
		w.SetClipboardText("gowaku クリップボード")
		if got := w.ClipboardText(); got != "gowaku クリップボード" {
			t.Errorf("clipboard: got %q", got)
		}
		// Files go as file URLs, one item each, next to the text.
		files := []string{"/gowaku-test/a.txt", "/gowaku-test/b c.png"}
		w.WriteClipboard(platform.ClipboardContent{Text: "with files", Files: files})
		both := platform.ClipboardText | platform.ClipboardFiles
		if kinds := w.ClipboardKinds(); kinds&both != both {
			t.Errorf("clipboard kinds with files: got %v", kinds)
		}
		if got := w.ReadClipboard(both); got.Text != "with files" || !slices.Equal(got.Files, files) {
			t.Errorf("clipboard with files: got %q and %q", got.Text, got.Files)
		}

		// The cursors that resize and the types of a file dialog go by names
		// that depend on the version of macOS: a wrong one raises.
		for _, c := range []input.Cursor{input.CursorColResize, input.CursorRowResize, input.CursorArrow} {
			w.SetCursor(c)
		}
		for _, ext := range []string{"png", "gowaku-unknown"} {
			if class("UTType").Send(sel("typeWithFilenameExtension:"), nsString(ext)) == 0 {
				t.Errorf("no UTType for the extension %q", ext)
			}
		}
		w.Activate()
		w.Close()
	})
}
