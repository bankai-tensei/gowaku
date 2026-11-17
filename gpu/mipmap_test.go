package gpu

import (
	"bytes"
	"testing"
)

func TestMipLevels_HalveToOneTexel(t *testing.T) {
	for _, c := range []struct{ w, h, want int }{
		{1, 1, 1}, {2, 1, 2}, {2, 2, 2}, {3, 3, 2}, {4, 4, 3}, {256, 256, 9}, {300, 7, 9}, {5, 1024, 11},
	} {
		if got := MipLevels(c.w, c.h); got != c.want {
			t.Errorf("%dx%d: got %d levels, want %d", c.w, c.h, got, c.want)
		}
	}
	if w, h := MipSize(300, 7, 3); w != 37 || h != 1 {
		t.Errorf("level 3 of 300x7: got %dx%d", w, h)
	}
}

func TestBuildMips_EvenSizesAverageBoxes(t *testing.T) {
	// 2×2 texels of one channel each, in every channel the same.
	pix := []byte{
		0, 0, 0, 0, 255, 255, 255, 255,
		10, 10, 10, 10, 31, 31, 31, 31,
	}
	got := BuildMips(pix, 8, 2, 2)
	// (0 + 255 + 10 + 31) / 4 = 74, rounded to the nearest.
	if want := []byte{74, 74, 74, 74}; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildMips_OddSizesKeepEveryTexel(t *testing.T) {
	// 3×1: the one texel below covers all three, a third each.
	pix := []byte{30, 0, 0, 255, 60, 0, 0, 255, 255, 0, 0, 255}
	got := BuildMips(pix, 12, 3, 1)
	if want := []byte{115, 0, 0, 255}; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// A uniform picture of any size stays uniform at every level.
	const w, h = 37, 13
	flat := bytes.Repeat([]byte{200, 100, 50, 255}, w*h)
	levels := BuildMips(flat, w*4, w, h)
	size := 0
	for k := 1; k < MipLevels(w, h); k++ {
		lw, lh := MipSize(w, h, k)
		size += lw * lh * 4
	}
	if len(levels) != size {
		t.Fatalf("got %d bytes, want %d", len(levels), size)
	}
	for i := 0; i < len(levels); i += 4 {
		if !bytes.Equal(levels[i:i+4], flat[:4]) {
			t.Fatalf("texel %d: got %v", i/4, levels[i:i+4])
		}
	}
}

func TestImageFilter_RidesInGradient(t *testing.T) {
	var in Instance
	if in.ImageFilter() != FilterLinear {
		t.Error("the zero value must be FilterLinear, what layers use")
	}
	in.SetImageFilter(FilterMipmap)
	if in.ImageFilter() != FilterMipmap || in.Gradient&GradientRamp != 0 {
		t.Errorf("got %v", in.ImageFilter())
	}
}

func TestBuildMips_EvenPathMatchesTheWeights(t *testing.T) {
	const w, h = 8, 4
	pix := make([]byte, w*h*4)
	for i := range pix {
		pix[i] = byte(i*37 + i/5)
	}
	fast := BuildMips(pix, w*4, w, h)
	slow := make([]byte, 4*2*4)
	halve(slow, 4, pix, w*4, mipTaps(nil, w, 4), mipTaps(nil, h, 2), nil)
	if !bytes.Equal(fast[:len(slow)], slow) {
		t.Errorf("got %v, want %v", fast[:len(slow)], slow)
	}
}
