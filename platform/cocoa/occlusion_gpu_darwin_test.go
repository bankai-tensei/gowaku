//go:build gpu && darwin && arm64

package cocoa

import (
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

// waitForSize pumps events until a resize of w arrives whose size is zero or
// not, as asked, or two seconds pass.
func waitForSize(p *Platform, w *Window, zero bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ev := p.WaitEvent(50 * time.Millisecond)
		if ev.Kind == platform.EventResize && ev.Window == w.id && (ev.PhysicalWidth == 0) == zero {
			return true
		}
	}
	return false
}

// TestPlatform_HiddenWindowStopsDrawing checks that a window nobody can see
// reports a size of zero, as a minimized one does, so that the application
// does not draw it: Metal does not wait for vertical blank then.
func TestPlatform_HiddenWindowStopsDrawing(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "occlusion", Width: 200, Height: 100})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()
		// A new window becomes visible a moment after it opens; only a change
		// is reported.
		for deadline := time.Now().Add(2 * time.Second); w.hidden() && time.Now().Before(deadline); {
			p.WaitEvent(50 * time.Millisecond)
		}
		p.libs.pool(func() { w.window.Send(sel("orderOut:"), objc.ID(0)) })
		if !waitForSize(p, w, true) {
			t.Error("a window taken off the screen did not report a size of zero")
		}
		p.libs.pool(func() { w.window.Send(sel("orderFront:"), objc.ID(0)) })
		if !waitForSize(p, w, false) {
			t.Error("a window shown again did not report its size")
		}
	})
}
