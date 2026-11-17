package x11

import (
	"image"
	"testing"
)

func TestIconData(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0x11, 0x22, 0x33, 0x80
	}
	data := iconData([]*image.NRGBA{img})
	// 16, 24 and 32 pixels, each with its width and height.
	if want := 2*3 + 16*16 + 24*24 + 32*32; len(data) != want {
		t.Fatalf("got %d longs, want %d", len(data), want)
	}
	if data[0] != 16 || data[1] != 16 {
		t.Errorf("first size %dx%d, want 16x16", data[0], data[1])
	}
	last := 2 + 16*16 + 2 + 24*24
	if data[last] != 32 || data[last+2] != 0x80112233 {
		t.Errorf("the 32 pixel icon starts %d, first pixel %#x; want 32 and 0x80112233", data[last], data[last+2])
	}
	if iconData(nil) != nil {
		t.Error("no images should give no property")
	}
}

func TestWMClass(t *testing.T) {
	if got := wmClass(""); got != "gowaku\x00gowaku\x00" {
		t.Errorf("empty: got %q", got)
	}
	if got := wmClass("com.example.App"); got != "com.example.App\x00com.example.App\x00" {
		t.Errorf("got %q", got)
	}
}
