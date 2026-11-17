// Package icon picks and scales the pictures of a window icon to the sizes a
// window system wants. Every platform package uses it.
// See docs/ADR/0111-window-decorations.md.
package icon

import (
	"image"

	"golang.org/x/image/draw"
)

// sizes are the square sizes, in pixels, that platforms which take a list
// (X11, Wayland without a preference) are given, up to the largest image.
var sizes = [...]int{16, 24, 32, 48, 64, 128, 256}

// Fit returns images as a picture of size by size pixels: the smallest image
// that covers size, or the largest if none does, scaled with Catmull-Rom and
// centered on transparency if it is not square. It returns nil for no images
// or a size below one. An image that already fits is returned as it is.
func Fit(images []*image.NRGBA, size int) *image.NRGBA {
	src := pick(images, size)
	if src == nil || size < 1 {
		return nil
	}
	b := src.Rect
	if b.Dx() == size && b.Dy() == size && b.Min == (image.Point{}) {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	// The longer side fills the square, and the shorter one is centered.
	w, h := size, size
	if b.Dx() > b.Dy() {
		h = max(1, size*b.Dy()/b.Dx())
	} else if b.Dy() > b.Dx() {
		w = max(1, size*b.Dx()/b.Dy())
	}
	x, y := (size-w)/2, (size-h)/2
	draw.CatmullRom.Scale(dst, image.Rect(x, y, x+w, y+h), src, b, draw.Src, nil)
	return dst
}

// Largest returns the longer side of the largest image, or zero.
func Largest(images []*image.NRGBA) int {
	n := 0
	for _, img := range images {
		if img != nil {
			n = max(n, img.Rect.Dx(), img.Rect.Dy())
		}
	}
	return n
}

// Ladder returns the standard sizes up to the largest image, and at least the
// smallest of them when there is any image.
func Ladder(images []*image.NRGBA) []int {
	largest := Largest(images)
	if largest == 0 {
		return nil
	}
	out := []int{sizes[0]}
	for _, s := range sizes[1:] {
		if s <= largest {
			out = append(out, s)
		}
	}
	return out
}

// pick returns the smallest image whose shorter side covers size, or the
// largest image.
func pick(images []*image.NRGBA, size int) *image.NRGBA {
	var best, largest *image.NRGBA
	for _, img := range images {
		if img == nil || img.Rect.Empty() {
			continue
		}
		side := min(img.Rect.Dx(), img.Rect.Dy())
		if largest == nil || side > min(largest.Rect.Dx(), largest.Rect.Dy()) {
			largest = img
		}
		if side >= size && (best == nil || side < min(best.Rect.Dx(), best.Rect.Dy())) {
			best = img
		}
	}
	if best != nil {
		return best
	}
	return largest
}
