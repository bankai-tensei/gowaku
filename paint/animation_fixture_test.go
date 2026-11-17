package paint

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The WebP fixtures in testdata/anim are made from pictures that these
// functions draw. TestAnimationFixtureSources writes them as PNG files when
// GOWAKU_ANIM_SOURCES names a directory; testdata/anim/README.md has the
// ffmpeg commands that encode them.

// fixtureStill returns the still pictures that become small WebP bitstreams.
func fixtureStill() map[string]image.Image {
	return map[string]image.Image{
		// Opaque red, 4x4.
		"red4": fill(4, 4, color.NRGBA{R: 255, A: 255}),
		// Blue at half alpha, 4x4.
		"blue4half": fill(4, 4, color.NRGBA{B: 255, A: 128}),
		// Opaque green, 2x2.
		"green2": fill(2, 2, color.NRGBA{G: 255, A: 255}),
		// Opaque white with a transparent left half, 4x4, for ALPH with VP8.
		"whitehalf4": func() image.Image {
			m := fill(4, 4, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			for y := range 4 {
				for x := range 2 {
					m.SetNRGBA(x, y, color.NRGBA{})
				}
			}
			return m
		}(),
	}
}

// fixtureMovie returns the frames of testdata/anim/movie.webp and movie.png:
// a red square that moves over a transparent canvas, with a blue bar at half
// alpha that comes and goes, so that the encoders use partial frames, both
// blends and disposal.
func fixtureMovie() []*image.NRGBA {
	const w, h, n = 32, 24, 6
	frames := make([]*image.NRGBA, n)
	for i := range frames {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 4; y < 12; y++ {
			for x := 2 + 4*i; x < 10+4*i; x++ {
				m.SetNRGBA(x, y, color.NRGBA{R: 230, G: 40, B: 30, A: 255})
			}
		}
		if i%2 == 1 {
			for y := 16; y < 22; y++ {
				for x := range w {
					m.SetNRGBA(x, y, color.NRGBA{R: 20, G: 60, B: 240, A: 128})
				}
			}
		}
		frames[i] = m
	}
	return frames
}

func fill(w, h int, c color.NRGBA) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(m.Pix); i += 4 {
		m.Pix[i], m.Pix[i+1], m.Pix[i+2], m.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return m
}

func TestAnimationFixtureSources(t *testing.T) {
	dir := os.Getenv("GOWAKU_ANIM_SOURCES")
	if dir == "" {
		t.Skip("set GOWAKU_ANIM_SOURCES to a directory to write the sources of the WebP fixtures")
	}
	write := func(name string, m image.Image) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, m); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for name, m := range fixtureStill() {
		write(name+".png", m)
	}
	for i, m := range fixtureMovie() {
		write("movie"+string(rune('0'+i))+".png", m)
	}
}
