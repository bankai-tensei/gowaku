package wayland

import (
	"image"
	"testing"
)

func TestPremultiplyBGRA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	copy(img.Pix, []byte{255, 128, 0, 128, 10, 20, 30, 255, 0, 0, 0, 0, 255, 255, 255, 255})
	got := make([]byte, 16)
	premultiplyBGRA(got, img)
	want := []byte{0, 64, 128, 128, 30, 20, 10, 255, 0, 0, 0, 0, 255, 255, 255, 255}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
