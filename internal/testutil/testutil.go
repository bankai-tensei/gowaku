// Package testutil renders widgets to text, for tests that need neither a GPU
// nor a window.
package testutil

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// NewShaper returns a Shaper with the Go font alone, so that goldens do not
// depend on the fonts of the machine.
func NewShaper(t testing.TB) *text.Shaper {
	t.Helper()
	s, err := text.NewShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// WithShaper returns w below s, which is what Run does for its windows.
func WithShaper(s *text.Shaper, w core.Widget) core.Widget {
	return core.Inherited[*text.Shaper]{Value: s, Child: w}
}

// Render lays out w at size and returns its display list as text, one line per
// command.
func Render(w core.Widget, size geom.Size) string {
	t := core.NewTree(w)
	defer t.Dispose()
	return RenderTree(t, size)
}

// RenderTree is Render for a tree that the test keeps using.
func RenderTree(t *core.Tree, size geom.Size) string {
	t.Build()
	t.Layout(geom.Tight(size))
	for t.NeedsBuild() {
		t.Build()
		t.Layout(geom.Tight(size))
	}
	var cv paint.Canvas
	cv.Reset()
	t.Paint(&cv)
	return Format(cv.DisplayList())
}

// Format writes dl as text.
func Format(dl *paint.DisplayList) string {
	var b strings.Builder
	for i := range dl.Ops {
		op := &dl.Ops[i]
		r := op.RRect.Rect
		fmt.Fprintf(&b, "%s x=%g y=%g w=%g h=%g", op.Kind, r.X, r.Y, r.W, r.H)
		if rr := op.RRect; rr.TopLeft != 0 || rr.TopRight != 0 || rr.BottomRight != 0 || rr.BottomLeft != 0 {
			fmt.Fprintf(&b, " radii=%g,%g,%g,%g", rr.TopLeft, rr.TopRight, rr.BottomRight, rr.BottomLeft)
		}
		if op.HasGradient {
			g := &op.Gradient
			fmt.Fprintf(&b, " gradient=%d from=%g,%g %s to=%g,%g %s", g.Kind, g.Start.X, g.Start.Y, hex(g.StartColor), g.End.X, g.End.Y, hex(g.EndColor))
			for i := range g.StopCount {
				fmt.Fprintf(&b, " stop=%g %s", g.Stops[i].Offset, hex(g.Stops[i].Color))
			}
		} else {
			fmt.Fprintf(&b, " color=%s", hex(op.Color))
		}
		if op.BorderWidth > 0 {
			fmt.Fprintf(&b, " border=%g %s", op.BorderWidth, hex(op.BorderColor))
		}
		if op.Glyphs != nil {
			fmt.Fprintf(&b, " origin=%g,%g size=%g glyphs=%d", op.Origin.X, op.Origin.Y, op.Glyphs.Size, len(op.Glyphs.Glyphs))
		}
		if op.Blur > 0 {
			fmt.Fprintf(&b, " blur=%g", op.Blur)
		}
		if m := op.Transform; !m.IsIdentity() {
			fmt.Fprintf(&b, " transform=%.4g,%.4g,%.4g,%.4g,%.4g,%.4g", m.A, m.B, m.C, m.D, m.E, m.F)
		}
		if op.Clipped {
			fmt.Fprintf(&b, " clip=%g,%g,%g,%g", op.Clip.X, op.Clip.Y, op.Clip.W, op.Clip.H)
		}
		b.WriteByte('\n')
		if op.Kind == paint.OpLayer {
			for line := range strings.Lines(Format(op.Layer.DisplayList())) {
				b.WriteString("  " + line)
			}
		}
	}
	return b.String()
}

func hex(c geom.Color) string {
	s := c.Std()
	return fmt.Sprintf("#%02x%02x%02x%02x", s.R, s.G, s.B, s.A)
}

// Golden compares got with testdata/name.golden. Run the test with -update to
// rewrite the file after an intended change.
func Golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run with -update to create it", err)
	}
	if !bytes.Equal(want, []byte(got)) {
		t.Errorf("%s differs\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}
