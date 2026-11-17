//go:build gowakudebug

package inspect

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/internal/golden"
	_ "github.com/bankai-tensei/gowaku/internal/testutil" // defines -update
	"github.com/bankai-tensei/gowaku/paint"
)

// updating reports whether the test runs with -update, the flag that
// internal/testutil defines for its golden files.
func updating() bool {
	f := flag.Lookup("update")
	return f != nil && f.Value.String() == "true"
}

// goldenSession returns a session over a small tree, with a selection and a
// few frames whose every number is fixed, so that the window draws the same on
// every run.
func goldenSession(t *testing.T) *Session {
	t.Helper()
	target := newFakeTarget("Sample", sampleTree(), geom.Size{W: 200, H: 150})
	t.Cleanup(target.tree.Dispose)
	s := NewSession(&fakeHost{targets: []Target{target}}, nil)
	var cv paint.Canvas
	for i := range 24 {
		cv.Reset()
		s.BeginFrame(target, uint64(i+1))
		if i%6 == 0 {
			target.tree.MarkNeedsPaint()
			target.tree.Repaint(&cv)
		}
		ms := time.Duration(1+i%5) * time.Millisecond
		s.EndFrame(target, [StageCount]time.Duration{ms / 4, ms / 4, ms / 4, ms / 8, ms / 8}, uint64(i%3), i%6 == 0)
	}
	for i := range s.Frames() {
		f := s.Frame(i)
		f.At = time.Duration(i) * 16 * time.Millisecond
		f.GPU = time.Duration(500+100*(i%4)) * time.Microsecond
	}
	s.SelectRenderObject(target, findRO[*core.RenderRepaintBoundary](target))
	s.SelectFrame(s.Frames() - 1)
	return s
}

// TestInspector_Golden draws the Inspector window on the CPU and compares it
// with testdata/<name>.png, with the tolerance of internal/golden. Run with
// -update to write the pictures, and look at them before committing them.
func TestInspector_Golden(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "inspector-light"
		if dark {
			name = "inspector-dark"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name+".png")
			if !updating() {
				if _, err := os.Stat(path); err != nil {
					t.Skipf("no golden at %s; run go test -tags gowakudebug ./internal/inspect -run TestInspector_Golden -update to create it", path)
				}
			}
			tt := gowakutest.New(t, Inspector{Session: goldenSession(t)}, gowakutest.Options{
				Width: 1180, Height: 780, Dark: dark, Fonts: [][]byte{goregular.TTF},
			})
			got := tt.Screenshot()
			if updating() {
				if err := golden.Save(path, got); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := golden.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := golden.Compare(got, want)
			if err != nil {
				t.Fatalf("%v; run with -update if the size changed on purpose", err)
			}
			t.Logf("%s: %v", name, d)
			if d.Failed() {
				out := filepath.Join(t.TempDir(), name+".png")
				if err := golden.Save(out, got); err != nil {
					t.Error(err)
				}
				t.Errorf("%v; got %s", d, out)
			}
		})
	}
}
