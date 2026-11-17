//go:build gpu && darwin && arm64

package cocoa

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/platform"
)

func TestPlatform_CustomFrame(t *testing.T) {
	main(func() {
		p, err := New()
		if err != nil {
			t.Errorf("New: %v", err)
			return
		}
		defer p.Destroy()
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "frame", Width: 300, Height: 200, CustomFrame: true})
		if err != nil {
			t.Errorf("CreateWindow: %v", err)
			return
		}
		w := pw.(*Window)
		defer w.Close()
		if !w.ClientDecorated() {
			t.Error("CustomFrame window is not ClientDecorated")
		}
		p.libs.pool(func() {
			frame := objc.Send[NSRect](w.window, sel("frame"))
			bounds := objc.Send[NSRect](w.view, sel("bounds"))
			if bounds.Size.Height != frame.Size.Height {
				t.Errorf("content %v does not reach the top of the frame %v", bounds.Size, frame.Size)
			}
			closeButton := w.window.Send(sel("standardWindowButton:"), int64(0))
			if !objc.Send[bool](closeButton, sel("isHidden")) {
				t.Error("the close button of AppKit shows")
			}
		})
		w.ToggleMaximize()
		if !w.Maximized() {
			t.Error("ToggleMaximize did not zoom")
		}
	})
}
