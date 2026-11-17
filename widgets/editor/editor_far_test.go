package editor

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// TestEditor_ScrollsByAPixelFarDown scrolls a document of two million lines to
// near its end, 40 million pixels down, where a float32 offset would move in
// steps of 4 (ADR 0150), and moves it by one pixel.
func TestEditor_ScrollsByAPixelFarDown(t *testing.T) {
	const n = 2_000_000
	var ctl Controller
	ctl.SetText(strings.Repeat("x\n", n))
	h := newEditor(t, &ctl, Editor{LineNumbers: true, ReadOnly: true})
	r := h.view()
	r.state.Jump(float64(n-100) * float64(r.lineHeight))
	h.frame()
	top := r.top()
	if top < 30_000_000 {
		t.Fatalf("the jump lands at %v", top)
	}
	first := r.first
	y := r.viewTop(first)
	for _, d := range []float64{1, 0.5, 3} {
		r.state.Jump(top + d)
		h.frame()
		if got := r.top() - top; got != d {
			t.Fatalf("a jump of %v moves the text by %v", d, got)
		}
		if got := y - r.viewTop(first); got != float32(d) {
			t.Fatalf("a jump of %v moves line %d by %v", d, first, got)
		}
	}
	// A click finds the line under the pointer, far down too.
	r.state.Jump(top)
	h.frame()
	at := r.positionAt(geom.Offset{X: r.textLeft + 1, Y: r.viewTop(first) + 1})
	if at.Line != first {
		t.Fatalf("a click on line %d lands on line %d", first, at.Line)
	}
}
