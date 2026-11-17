package gowakupreview

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"
)

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestCompare(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.png")
	white := solid(4, 3, color.NRGBA{255, 255, 255, 255})
	if changed, diff, err := compare(path, white); err != nil || changed || diff != nil {
		t.Fatalf("a first run must count as unchanged: %v %v %v", changed, diff, err)
	}
	if err := writePNG(path, white); err != nil {
		t.Fatal(err)
	}
	if changed, diff, err := compare(path, white); err != nil || changed || diff != nil {
		t.Errorf("the same picture changed: %v %v %v", changed, diff, err)
	}
	dot := solid(4, 3, color.NRGBA{255, 255, 255, 255})
	dot.SetNRGBA(2, 1, color.NRGBA{0, 0, 0, 255})
	changed, diff, err := compare(path, dot)
	if err != nil || !changed || diff == nil {
		t.Fatalf("a changed pixel was missed: %v %v %v", changed, diff, err)
	}
	if c := diff.NRGBAAt(2, 1); c.R < 200 || c.G > 80 {
		t.Errorf("the changed pixel is %v in the difference, want red", c)
	}
	if c := diff.NRGBAAt(0, 0); c.R != c.G || c.G != c.B {
		t.Errorf("an unchanged pixel is %v in the difference, want gray", c)
	}
	if changed, _, err := compare(path, solid(5, 3, color.NRGBA{255, 255, 255, 255})); err != nil || !changed {
		t.Error("a new size must count as a change")
	}
}

func TestParseTimes(t *testing.T) {
	got, err := parseTimes(" 0, 70ms,1.5s ,")
	if err != nil || len(got) != 3 || got[0] != 0 || got[1] != 70*time.Millisecond || got[2] != 1500*time.Millisecond {
		t.Errorf("got %v, %v", got, err)
	}
	if _, err := parseTimes("soon"); err == nil {
		t.Error("a bad duration must fail")
	}
}

func TestDirOf(t *testing.T) {
	if got := DirOf("github.com/a/b_test"); got != "github_com_a_b_test" {
		t.Errorf("got %q", got)
	}
}
