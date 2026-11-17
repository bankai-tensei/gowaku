package soft

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
)

// multisampleCount is the MSAA level of the GPU backends.
const multisampleCount = 4

// canvas is what a frame draws to: the color, depth and stencil of each sample.
// The samples of a pixel are next to each other, so that a canvas of one sample
// has the memory layout of a Texture.
type canvas struct {
	width, height int
	scale         float32
	// positions holds where the samples of a pixel lie.
	positions []fixedPoint
	// color holds the R, G, B and A of each sample, premultiplied.
	color []uint8
	// depth holds the clip level of each sample. A real depth buffer stores
	// level/256 instead, which compares the same way.
	depth   []uint16
	stencil []uint8
	// backdrop is the copy of the color that blend modes which read the target
	// see, one premultiplied pixel per pixel. It is made when first needed.
	backdrop []uint8
	// blurA and blurB are the scratch targets of blurs, and filtered the
	// blurred picture of the draw call in progress. See blur.
	blurA, blurB, filtered *Texture
}

func newCanvas(width, height, samples int, scale float32) *canvas {
	n := width * height * samples
	return newCanvasOn(make([]uint8, n*4), width, height, samples, scale)
}

// newCanvasOn creates a canvas that draws to color, with depth and stencil at zero
// as a frame starts them.
func newCanvasOn(color []uint8, width, height, samples int, scale float32) *canvas {
	n := width * height * samples
	return &canvas{
		width:     width,
		height:    height,
		scale:     scale,
		positions: samplePositions(samples),
		color:     color,
		depth:     make([]uint16, n),
		stencil:   make([]uint8, n),
	}
}

// fill sets every sample to background as it is: a clear does not premultiply.
func (c *canvas) fill(background geom.Color) {
	r, g, b, a := unorm8(background.R), unorm8(background.G), unorm8(background.B), unorm8(background.A)
	for i := 0; i < len(c.color); i += 4 {
		c.color[i], c.color[i+1], c.color[i+2], c.color[i+3] = r, g, b, a
	}
}

// clear makes r transparent.
func (c *canvas) clear(r image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, c.width, c.height))
	samples := len(c.positions)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		first := c.sampleIndex(r.Min.X, y) * 4
		clear(c.color[first : first+r.Dx()*samples*4])
	}
}

// sampleIndex returns the index of the first sample of a pixel.
func (c *canvas) sampleIndex(x, y int) int { return (y*c.width + x) * len(c.positions) }

// blend composites src over sample i, by the blend state of the backends: ONE,
// INV_SRC_ALPHA for all channels. src is premultiplied and already 8 bits wide: the
// Iris Xe and the RTX 3050 Ti both round the shader output before they blend, which
// blending in float32 misses by one level on a good part of all translucent pixels.
func (c *canvas) blend(i int, src [4]uint8) {
	p := c.color[i*4 : i*4+4]
	inverse := 255 - int(src[3])
	for k := range p {
		p[k] = uint8(min(int(src[k])+(int(p[k])*inverse+127)/255, 255))
	}
}

// resolve averages the samples of each pixel, with halves rounding down as they do
// on the RTX 3050 Ti, and almost always on the Iris Xe. The result is opaque by
// definition, because a window ignores alpha.
func (c *canvas) resolve() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.width, c.height))
	samples := len(c.positions)
	for y := range c.height {
		for x := range c.width {
			first := c.sampleIndex(x, y) * 4
			out := img.Pix[y*img.Stride+x*4:]
			for k := range 3 {
				sum := (samples - 1) / 2
				for s := range samples {
					sum += int(c.color[first+s*4+k])
				}
				out[k] = uint8(sum / samples)
			}
			out[3] = 0xff
		}
	}
	return img
}

// unorm8 converts v the way a render target stores it: clamped to 0..1 and rounded
// to the nearest of 256 levels.
func unorm8(v float32) uint8 { return uint8(saturate(v)*255 + 0.5) }
