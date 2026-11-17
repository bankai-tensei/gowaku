//go:build gpu

package golden

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

// checkOnWindows holds b to the pictures of the CPU backend, on real windows.
// The tests are not subtests: a window belongs to the thread that made it.
func checkOnWindows(t *testing.T, p platform.Platform, name string, b Backend) {
	t.Helper()
	r, err := NewRenderer(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Destroy)
	// The size of a window is logical, so the scale of the OS comes first.
	probe, err := p.CreateWindow(platform.WindowOptions{Title: "golden", Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	system := probe.Scale()
	probe.Close()

	scenes := Scenes()
	for i := range scenes {
		s := &scenes[i]
		func() {
			w, err := p.CreateWindow(platform.WindowOptions{
				Title: "golden", Width: float32(s.Width) / system, Height: float32(s.Height) / system,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			width, height := w.PhysicalSize()
			// A window of points at a scale of 2, as on a Retina Mac, cannot
			// have an odd physical size, and comes out a pixel larger. The
			// surface still takes the size of the scene, which a CAMetalLayer
			// allows.
			larger := width-s.Width <= 1 && height-s.Height <= 1 && width >= s.Width && height >= s.Height
			if (width != s.Width || height != s.Height) && !(larger && system == 2) {
				t.Errorf("%s: window of %dx%d, want %dx%d", s.Name, width, height, s.Width, s.Height)
				return
			}
			surface, err := b.CreateSurface(w.NativeHandle(), s.Width, s.Height, s.Scale)
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Destroy()
			img, err := r.Render(s, surface)
			if err != nil {
				t.Fatal(err)
			}
			check(t, s, img, name)
		}()
	}
}
