//go:build gpu

package win32

import (
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
)

// A pinch needs fingers on the touchpad, so the content of the viewport is set
// to a scale directly, as DirectManipulation would, and the handler called.
func TestPinch_ViewportScaleBecomesEvents(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "pinch"})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	if w.pinch == nil {
		t.Skip("no DirectManipulation")
	}
	for p.WaitEvent(0).Kind != platform.EventNone {
	}
	pc := w.pinch
	for _, scale := range []float32{1.5, 3} {
		m := [6]float32{scale, 0, 0, scale, 0, 0}
		pc.content.Call(iDMContent_SyncTransform, uintptr(unsafe.Pointer(&m[0])), 6)
		pinchContentUpdated(pc.handler, nil, pc.content)
	}
	var factors []float32
	for ev := p.WaitEvent(0); ev.Kind != platform.EventNone; ev = p.WaitEvent(0) {
		if ev.Kind == platform.EventPinch {
			factors = append(factors, ev.Pinch)
		}
	}
	if len(factors) != 2 || factors[0] != 1.5 || factors[1] != 2 {
		t.Errorf("factors %v, want [1.5 2]", factors)
	}
	pinchStatusChanged(pc.handler, nil, DIRECTMANIPULATION_READY, DIRECTMANIPULATION_RUNNING)
	if pc.scale != 1 {
		t.Errorf("a new pinch must start from 1, got %v", pc.scale)
	}
}
