//go:build gpu && darwin && arm64

package cocoa

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// fakeDrag is what the NSDraggingInfo of the test answers.
type fakeDrag struct {
	board    objc.ID
	location NSPoint // in the coordinates of the window
	mask     uint64
}

// t.Fatal and t.Skip would end the goroutine, which inside main is the loop
// of the main thread, so the tests report with t.Error and return there.

// fakeDraggingInfo returns an object that answers the messages of
// NSDraggingInfo that the view sends, from d. A real drag needs the mouse,
// which a test cannot move without the accessibility permission.
func fakeDraggingInfo(t *testing.T, d *fakeDrag) objc.ID {
	t.Helper()
	name := "GowakuTestDraggingInfo" + t.Name()
	cls := objc.GetClass(name)
	if cls == 0 {
		var err error
		cls, err = objc.RegisterClass(name, objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("draggingPasteboard"), Fn: func(objc.ID, objc.SEL) objc.ID { return d.board }},
			{Cmd: sel("draggingLocation"), Fn: func(objc.ID, objc.SEL) NSPoint { return d.location }},
			{Cmd: sel("draggingSourceOperationMask"), Fn: func(objc.ID, objc.SEL) uint64 { return d.mask }},
		})
		if err != nil {
			t.Errorf("register: %v", err)
			return 0
		}
	}
	return objc.ID(cls).Send(sel("new"))
}

func TestPlatform_DropFromPasteboard(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ドロップ.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, _ = filepath.EvalSymlinks(file)
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "drop", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()

		var events []platform.DropEvent
		answer := input.DropMove
		w.SetDropHandler(func(e *platform.DropEvent) input.DropEffect {
			events = append(events, *e)
			return answer
		})

		p.libs.pool(func() {
			registered := false
			types := w.view.Send(sel("registeredDraggedTypes"))
			for i := range count(types) {
				registered = registered || goString(at(types, i)) == typeFileURL
			}
			if !registered {
				t.Error("the view takes no file URLs")
			}

			board := class("NSPasteboard").Send(sel("pasteboardWithUniqueName"))
			defer board.Send(sel("releaseGlobally"))
			board.Send(sel("clearContents"))
			url := class("NSURL").Send(sel("fileURLWithPath:"), nsString(file))
			text := class("NSPasteboardItem").Send(sel("new")).Send(sel("autorelease"))
			text.Send(sel("setString:forType:"), nsString("テキスト"), nsString(typeString))
			if !objc.Send[bool](board, sel("writeObjects:"), nsArray(url, text)) {
				t.Error("writeObjects failed")
				return
			}

			// The point (50, 25) from the top left of the view, in the
			// coordinates of the window, whose origin is at the bottom.
			bounds := objc.Send[NSRect](w.view, sel("bounds"))
			d := &fakeDrag{board: board, location: NSPoint{50, bounds.Size.Height - 25},
				mask: NSDragOperationCopy | NSDragOperationGeneric}
			info := fakeDraggingInfo(t, d)
			if info == 0 {
				return
			}
			defer info.Send(sel("release"))

			if op := objc.Send[uint64](w.view, sel("draggingEntered:"), info); op != NSDragOperationGeneric {
				t.Errorf("entered: got operation %d, want Generic", op)
			}
			d.location.X = 60
			answer = input.DropLink // not allowed
			if op := objc.Send[uint64](w.view, sel("draggingUpdated:"), info); op != NSDragOperationNone {
				t.Errorf("updated with a link: got operation %d, want none", op)
			}
			if objc.Send[bool](w.view, sel("prepareForDragOperation:"), info) {
				t.Error("prepare accepted a refused drag")
			}
			answer = input.DropCopy
			objc.Send[uint64](w.view, sel("draggingUpdated:"), info)
			if !objc.Send[bool](w.view, sel("prepareForDragOperation:"), info) {
				t.Error("prepare refused")
			}
			if !objc.Send[bool](w.view, sel("performDragOperation:"), info) {
				t.Error("perform refused")
			}
			w.view.Send(sel("draggingEnded:"), info)

			// A second drag leaves without dropping.
			objc.Send[uint64](w.view, sel("draggingEntered:"), info)
			w.view.Send(sel("draggingExited:"), info)
			w.view.Send(sel("draggingEnded:"), info)
		})

		phases := []platform.DropPhase{platform.DropOver, platform.DropOver, platform.DropOver, platform.DropDrop,
			platform.DropOver, platform.DropLeave}
		if len(events) != len(phases) {
			t.Errorf("got %d events, want %d: %+v", len(events), len(phases), events)
			return
		}
		for i, e := range events {
			if e.Phase != phases[i] {
				t.Errorf("event %d: phase %v, want %v", i, e.Phase, phases[i])
			}
			if e.Kinds != input.DataFiles|input.DataText {
				t.Errorf("event %d: kinds %v", i, e.Kinds)
			}
		}
		first := events[0]
		if s := w.scale; first.X != 50*s || first.Y != 25*s {
			t.Errorf("position (%v, %v), want (%v, %v)", first.X, first.Y, 50*s, 25*s)
		}
		if first.Allowed != input.DropCopy|input.DropMove {
			t.Errorf("allowed %v", first.Allowed)
		}
		drop := events[3]
		// NSURL hands the path back decomposed (NFD), as the file system
		// names it, so compare files rather than strings.
		if len(drop.Data.Files) != 1 || !sameFile(drop.Data.Files[0], file) {
			t.Errorf("files %q, want %q", drop.Data.Files, file)
		}
		if drop.Data.Text != "テキスト" {
			t.Errorf("text %q", drop.Data.Text)
		}
		for i, e := range events {
			if e.Phase != platform.DropDrop && (e.Data.Text != "" || e.Data.Files != nil) {
				t.Errorf("event %d carries data before the drop", i)
			}
		}
		if _, ok := waitFor(p, platform.EventRedraw); !ok {
			t.Error("no redraw after the drag")
		}
	})
}

func sameFile(a, b string) bool {
	sa, errA := os.Stat(a)
	sb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(sa, sb)
}

// postMouse posts an event of the primary button, a press (1) or a release
// (2), at (x, y) points from the top left of the view of w.
func postMouse(p *Platform, w *Window, kind uint64, x, y float64) {
	p.libs.pool(func() {
		bounds := objc.Send[NSRect](w.view, sel("bounds"))
		ev := class("NSEvent").Send(sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
			kind, NSPoint{x, bounds.Size.Height - y}, uint64(0), float64(0),
			objc.Send[int64](w.window, sel("windowNumber")), objc.ID(0), int64(0), int64(1), float32(1))
		p.app.Send(sel("postEvent:atStart:"), ev, false)
	})
}

func TestPlatform_StartDragEnds(t *testing.T) {
	// The session runs as AppKit's own loop inside nextEventMatchingMask
	// until the button goes up, which it reads from the window server, not
	// from posted events. Without a real mouse it can wait for a long time,
	// so that half runs only on request.
	session := os.Getenv("GOWAKU_DRAG_SESSION_TEST") != ""
	if !session {
		t.Log("set GOWAKU_DRAG_SESSION_TEST=1 to start a real NSDraggingSession")
	}
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "drag", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()

		// Without a press there is nothing to drag from.
		var refused []input.DropEffect
		w.StartDrag(input.DragData{Text: "x"}, input.DropCopy, func(e input.DropEffect) { refused = append(refused, e) })
		if len(refused) != 1 || refused[0] != input.DropNone {
			t.Errorf("without a press: got %v", refused)
		}

		if !session {
			return
		}
		postMouse(p, w, 1, 20, 20)
		if _, ok := waitFor(p, platform.EventPointerDown); !ok {
			t.Error("no press")
			return
		}
		// A release where the press was ends the session with no
		// operation; what is tested is that it starts and that done comes
		// back, once, from draggingSession:endedAtPoint:operation:.
		var ended []input.DropEffect
		start := time.Now()
		w.StartDrag(input.DragData{Text: "ドラッグ", Files: []string{"/tmp"}}, input.DropCopy|input.DropMove,
			func(e input.DropEffect) { ended = append(ended, e) })
		postMouse(p, w, 2, 20, 20)
		deadline := time.Now().Add(3 * time.Second)
		for len(ended) == 0 && time.Now().Before(deadline) {
			p.WaitEvent(50 * time.Millisecond)
		}
		if len(ended) != 1 {
			t.Errorf("done called %d times", len(ended))
			return
		}
		t.Logf("the drag ended with %v after %v", ended[0], time.Since(start))
	})
}
