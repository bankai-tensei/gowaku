package golden

import (
	"flag"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/bankai-tensei/gowaku/gpu/soft"
)

var update = flag.Bool("update", false, "rewrite the pictures from the CPU backend")

// check compares what b draws for s with the stored picture.
func check(t *testing.T, s *Scene, img *image.NRGBA, backend string) {
	t.Helper()
	want, err := Load(Path(s.Name))
	if err != nil {
		t.Fatalf("%v; run with -update to create it", err)
	}
	d, err := Compare(img, want)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s %s: %v", s.Name, backend, d)
	if d.Failed() {
		// GOWAKU_GOLDEN_OUT keeps the pictures that fail, which the temporary
		// directory of the test does not.
		dir := os.Getenv("GOWAKU_GOLDEN_OUT")
		if dir == "" {
			dir = t.TempDir()
		}
		out := filepath.Join(dir, s.Name+"-"+backend+".png")
		if err := Save(out, img); err != nil {
			t.Error(err)
		}
		t.Errorf("%s: %v; got %s", backend, d, out)
	}
}

func TestGolden_CPU(t *testing.T) {
	device := soft.New()
	t.Cleanup(device.Destroy)
	r, err := NewRenderer(device)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Destroy)
	scenes := Scenes()
	for i := range scenes {
		s := &scenes[i]
		t.Run(s.Name, func(t *testing.T) {
			surface, err := device.CreateSurface(nil, s.Width, s.Height, s.Scale)
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Destroy()
			img, err := r.Render(s, surface)
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := Save(Path(s.Name), img); err != nil {
					t.Fatal(err)
				}
				return
			}
			check(t, s, img, "soft")
		})
	}
}

func TestCompare(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	b := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	if d, err := Compare(a, b); err != nil || d.Failed() || d.Worst != 0 {
		t.Errorf("same pictures: %v, %v", d, err)
	}
	b.Pix[0] = 2
	if d, err := Compare(a, b); err != nil || d.Failed() || d.Worst != 2 {
		t.Errorf("rounding must pass: %v, %v", d, err)
	}
	for i := range 30 {
		b.Pix[i*4] = 100
	}
	if d, err := Compare(a, b); err != nil || !d.Failed() {
		t.Errorf("30 wrong pixels must fail: %v, %v", d, err)
	}
	if _, err := Compare(a, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err == nil {
		t.Error("sizes that differ must fail")
	}
}
