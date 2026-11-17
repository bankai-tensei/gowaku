package field

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// verticalHost records what the field says about the writing direction.
type verticalHost struct {
	wtest.FakeHost
	vertical []bool
}

func (h *verticalHost) SetIMEVertical(v bool) { h.vertical = append(h.vertical, v) }

func verticalField(t *testing.T, value string) (*fieldHarness, *verticalHost) {
	t.Helper()
	h := newField(t, Input{Value: value, Multiline: true, Style: text.Style{Size: 10, Orientation: text.Vertical}})
	host := &verticalHost{}
	h.tree.SetHost(host)
	h.host = &host.FakeHost
	h.frame()
	return h, host
}

func TestTextField_VerticalTakesTheHeight(t *testing.T) {
	h, _ := verticalField(t, "abc")
	sz := h.field().Size()
	if sz.H != h.size.H {
		t.Errorf("height %v, want the full %v", sz.H, h.size.H)
	}
	if sz.W >= sz.H || sz.W < fieldHeight {
		t.Errorf("width %v, want one column wide", sz.W)
	}
}

func TestTextField_VerticalArrows(t *testing.T) {
	// Two columns: "ab" on the right, "cd" left of it.
	h := newField(t, Input{Value: "ab\ncd", Multiline: true, Style: text.Style{Size: 10, Orientation: text.Vertical}})
	h.Key(input.KeyDown, 0)
	h.expectSelection(1, 1)
	h.Key(input.KeyUp, 0)
	h.expectSelection(0, 0)
	// Left goes on to the next column, at the same height.
	h.Key(input.KeyDown, 0)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(4, 4)
	h.Key(input.KeyRight, 0)
	h.expectSelection(1, 1)
}

func TestTextField_VerticalTellsTheIME(t *testing.T) {
	h, host := verticalField(t, "abc")
	h.Key(input.KeyDown, 0)
	if len(host.vertical) == 0 || !host.vertical[len(host.vertical)-1] {
		t.Fatalf("the IME must hear of vertical text: %v", host.vertical)
	}
	r := host.Rect
	if r.H != 0 || r.W <= 0 {
		t.Errorf("caret rect %v, want a horizontal bar across the column", r)
	}
	// The first column hugs the right edge of the field.
	f := h.field()
	right := f.textOrigin().X + f.laidOut.Size().W
	if want := f.Size().W - f.insets.Right; right != want {
		t.Errorf("text ends at %v, want %v", right, want)
	}
}

// BenchmarkTextField_Keystroke100k types into a field of 100,000 characters
// and runs the frame that follows: build, layout and paint.
func BenchmarkTextField_Keystroke100k(b *testing.B) {
	var sb strings.Builder
	for sb.Len() < 100000 {
		sb.WriteString("Typesetting with a narrow measure leaves wide gaps between words.\n")
	}
	h := newField(b, Input{Value: sb.String(), Multiline: true})
	h.Key(input.KeyHome, input.ModControl)
	for range 20 {
		h.Key(input.KeyDown, 0)
	}
	var cv paint.Canvas
	frame := func() {
		h.tree.Build()
		h.tree.Layout(geom.Tight(h.size))
		cv.Reset()
		h.tree.Paint(&cv)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		h.tree.DispatchText(string(rune('a' + i%26)))
		frame()
	}
}
