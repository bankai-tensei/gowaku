package paint

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // registers the format with DecodeImage
	_ "image/png"  // registers the format with DecodeImage
	"io"

	_ "golang.org/x/image/webp" // registers the format with DecodeImage

	"github.com/bankai-tensei/gowaku/geom"
)

// FilterQuality says how a picture is filtered when it is drawn at a size other
// than its own.
type FilterQuality uint8

// The filter qualities. The zero value is FilterMipmap.
const (
	// FilterDefault is FilterMipmap.
	FilterDefault FilterQuality = iota
	// FilterNearest takes the texel under each pixel, which keeps hard edges
	// for pixel art.
	FilterNearest
	// FilterLinear blends the four texels around each pixel. Shrunk to less
	// than half, a picture with fine detail flickers and shows moiré.
	FilterLinear
	// FilterMipmap blends between halved copies of the picture, chosen by
	// how much it shrinks on the surface, so that shrunk pictures stay smooth.
	// Enlarged or at its own size, a picture looks as with FilterLinear.
	FilterMipmap
)

// String returns the name of the quality.
func (q FilterQuality) String() string {
	switch q {
	case FilterDefault:
		return "default"
	case FilterNearest:
		return "nearest"
	case FilterLinear:
		return "linear"
	case FilterMipmap:
		return "mipmap"
	}
	return "unknown"
}

// Image is a picture to draw. Its pixels are premultiplied RGBA and do not
// change. Create it once and keep it: the GPU copy belongs to the *Image.
type Image struct {
	pix           []byte
	width, height int
}

// NewImage copies m into an Image.
func NewImage(m image.Image) *Image {
	b := m.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// image.RGBA is premultiplied, so that drawing into it converts.
	draw.Draw(rgba, rgba.Bounds(), m, b.Min, draw.Src)
	return &Image{pix: rgba.Pix, width: b.Dx(), height: b.Dy()}
}

// DecodeImage reads a PNG, JPEG or WebP file.
func DecodeImage(r io.Reader) (*Image, error) {
	m, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("paint: decode image: %w", err)
	}
	return NewImage(m), nil
}

// Size returns the size of the picture in pixels.
func (m *Image) Size() geom.Size { return geom.Size{W: float32(m.width), H: float32(m.height)} }

// Pixels returns the premultiplied RGBA pixels, four bytes each, without gaps.
func (m *Image) Pixels() (pix []byte, width, height int) { return m.pix, m.width, m.height }

// At returns the pixel at (x, y), for tests.
func (m *Image) At(x, y int) color.RGBA {
	i := (y*m.width + x) * 4
	return color.RGBA{R: m.pix[i], G: m.pix[i+1], B: m.pix[i+2], A: m.pix[i+3]}
}
