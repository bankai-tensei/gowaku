package icon

import (
	"image"
	"image/color"
	"slices"
	"testing"
)

func square(n int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func TestFit_PicksTheSmallestThatCovers(t *testing.T) {
	red := square(16, color.NRGBA{R: 255, A: 255})
	green := square(32, color.NRGBA{G: 255, A: 255})
	blue := square(256, color.NRGBA{B: 255, A: 255})
	images := []*image.NRGBA{blue, red, green}
	if got := Fit(images, 32); got != green {
		t.Errorf("Fit(32) did not return the 32 pixel image as it is")
	}
	if got := Fit(images, 24); got.Rect.Dx() != 24 || got.Pix[1] != 255 {
		t.Errorf("Fit(24) = %v, first pixel %v; want 24 pixels scaled from the green one", got.Rect, got.Pix[:4])
	}
	if got := Fit(images, 512); got.Rect.Dx() != 512 || got.Pix[2] != 255 {
		t.Errorf("Fit(512) should scale the largest, the blue one, up")
	}
	if Fit(nil, 16) != nil || Fit(images, 0) != nil {
		t.Errorf("Fit of nothing should be nil")
	}
}

func TestFit_CentersAWideImage(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for i := 3; i < len(wide.Pix); i += 4 {
		wide.Pix[i] = 255
	}
	got := Fit([]*image.NRGBA{wide}, 16)
	if a := got.NRGBAAt(8, 0).A; a != 0 {
		t.Errorf("top row alpha = %d, want 0 (letterboxed)", a)
	}
	if a := got.NRGBAAt(8, 8).A; a != 255 {
		t.Errorf("middle alpha = %d, want 255", a)
	}
}

func TestLadder(t *testing.T) {
	if got, want := Ladder([]*image.NRGBA{square(100, color.NRGBA{})}), []int{16, 24, 32, 48, 64}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := Ladder([]*image.NRGBA{square(8, color.NRGBA{})}), []int{16}; !slices.Equal(got, want) {
		t.Errorf("small image: got %v, want %v", got, want)
	}
	if Ladder(nil) != nil {
		t.Errorf("no images should give no sizes")
	}
}
