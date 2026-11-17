package text

import (
	"bytes"
	"image"
	"image/png"
	"math"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// rasterizeColor draws a color glyph into a.colorScratch: a COLR paint graph
// (v0 layers or v1 paints) or an sbix or CBDT bitmap. It reports false for a
// glyph that has no color data, which then takes the outline path.
func (a *Atlas) rasterizeColor(k GlyphKey) (GlyphImage, bool) {
	face := a.shaper.faces[k.Face]
	gid := font.GID(k.ID)
	if face.COLR != nil {
		if p, ok := face.COLR.Search(tables.GlyphID(gid)); ok {
			return a.rasterizeCOLR(face, k, p)
		}
	}
	return a.rasterizeBitmap(face, k)
}

// glyphTransform returns the map from font units to the pixels of an image
// whose top left is at (left, top) from the origin, with the subpixel shift and
// the sideways turn of k.
func glyphTransform(k GlyphKey, unit float32, left, top int) affine {
	shift := float32(k.Subpixel) / SubpixelSteps
	m := affine{a: unit, d: -unit}
	if k.Sideways {
		m = affine{b: unit, c: unit}
	}
	m.e, m.f = shift-float32(left), -float32(top)
	return m
}

// pixelBounds returns the pixels that the font-unit box covers under k.
func pixelBounds(k GlyphKey, unit, xMin, yMin, xMax, yMax float32) (left, top, width, height int) {
	minX, minY, maxX, maxY := pixelBox(k, unit, xMin, yMin, xMax, yMax)
	left, top = int(math.Floor(float64(minX))), int(math.Floor(float64(minY)))
	return left, top, int(math.Ceil(float64(maxX))) - left, int(math.Ceil(float64(maxY))) - top
}

// pixelBox returns the font-unit box under k in fractional pixels from the
// origin.
func pixelBox(k GlyphKey, unit, xMin, yMin, xMax, yMax float32) (minX, minY, maxX, maxY float32) {
	m := glyphTransform(k, unit, 0, 0)
	minX, minY = float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY = -minX, -minY
	for _, p := range [4][2]float32{{xMin, yMin}, {xMax, yMin}, {xMin, yMax}, {xMax, yMax}} {
		x, y := m.apply(p[0], p[1])
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func (a *Atlas) rasterizeCOLR(face *font.Face, k GlyphKey, p tables.PaintTable) (GlyphImage, bool) {
	unit := k.Size / float32(face.Upem())
	xMin, yMin, xMax, yMax, ok := colrBox(face, font.GID(k.ID))
	if !ok {
		return GlyphImage{}, false
	}
	left, top, w, h := pixelBounds(k, unit, xMin, yMin, xMax, yMax)
	if w <= 0 || h <= 0 || w > a.size || h > a.size {
		return GlyphImage{}, false
	}
	layer := newColorLayer(w, h)
	cp := &a.painter
	cp.face, cp.w, cp.h, cp.depth = face, w, h, 0
	cp.palette = nil
	if len(face.CPAL) > 0 {
		cp.palette = face.CPAL[0]
	}
	cp.foreground = rgba{0, 0, 0, 1}
	cp.paint(p, glyphTransform(k, unit, left, top), layer)
	a.colorScratch = resizeBytes(a.colorScratch, w*h*4)
	for i, v := range layer.pix {
		a.colorScratch[i] = uint8(min(max(v, 0), 1)*255 + 0.5)
	}
	return GlyphImage{Width: w, Height: h, Left: left, Top: top}, true
}

// colrBox returns the box a color glyph may paint, in font units: its clip box,
// or else the box of its advance from the descender to the ascender.
func colrBox(face *font.Face, gid font.GID) (xMin, yMin, xMax, yMax float32, ok bool) {
	if box, found := face.COLR.ClipList.Search(tables.GlyphID(gid)); found {
		switch b := box.(type) {
		case tables.ClipBoxFormat1:
			return float32(b.XMin), float32(b.YMin), float32(b.XMax), float32(b.YMax), true
		case tables.ClipBoxFormat2:
			return float32(b.XMin), float32(b.YMin), float32(b.XMax), float32(b.YMax), true
		}
	}
	ext, found := face.FontHExtents()
	if !found {
		return 0, 0, 0, 0, false
	}
	return 0, ext.Descender, face.HorizontalAdvance(gid), ext.Ascender, true
}

// rasterizeBitmap scales the PNG bitmap of a glyph to the size of k: an sbix
// glyph, as in Apple Color Emoji with strikes from 20 to 160 pixels per em, or
// a CBDT glyph, as in Noto Color Emoji at 109. go-text picks the strike for
// the ppem and gives the box of the bitmap, origin offsets included, in font
// units. The bitmap keeps its scale at every subpixel position: it is placed at
// its fractional box rather than stretched to whole pixels.
func (a *Atlas) rasterizeBitmap(face *font.Face, k GlyphKey) (GlyphImage, bool) {
	gid := font.GID(k.ID)
	ppem := uint16(min(max(k.Size, 1), math.MaxUint16))
	face.SetPpem(ppem, ppem)
	bm, ok := face.GlyphDataBitmap(gid)
	if !ok || bm.Format != font.PNG {
		return GlyphImage{}, false
	}
	ext, ok := face.GlyphExtents(gid)
	if !ok {
		return GlyphImage{}, false
	}
	src, err := png.Decode(bytes.NewReader(bm.Data))
	if err != nil {
		return GlyphImage{}, false
	}
	if k.Sideways {
		src = turnClockwise(src)
	}
	unit := k.Size / float32(face.Upem())
	x0, y0, x1, y1 := pixelBox(k, unit, ext.XBearing, ext.YBearing+ext.Height, ext.XBearing+ext.Width, ext.YBearing)
	left, top := int(math.Floor(float64(x0))), int(math.Floor(float64(y0)))
	w, h := int(math.Ceil(float64(x1)))-left, int(math.Ceil(float64(y1)))-top
	sb := src.Bounds()
	if w <= 0 || h <= 0 || w > a.size || h > a.size || sb.Empty() {
		return GlyphImage{}, false
	}
	a.colorScratch = resizeBytes(a.colorScratch, w*h*4)
	clear(a.colorScratch)
	dst := &image.RGBA{Pix: a.colorScratch, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	sx := float64(x1-x0) / float64(sb.Dx())
	sy := float64(y1-y0) / float64(sb.Dy())
	s2d := f64.Aff3{
		sx, 0, float64(x0) - float64(left) - float64(sb.Min.X)*sx,
		0, sy, float64(y0) - float64(top) - float64(sb.Min.Y)*sy,
	}
	draw.CatmullRom.Transform(dst, s2d, src, sb, draw.Src, nil)
	return GlyphImage{Width: w, Height: h, Left: left, Top: top}, true
}

// turnClockwise returns img turned 90° clockwise.
func turnClockwise(img image.Image) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(b.Max.Y-1-y, x-b.Min.X, img.At(x, y))
		}
	}
	return out
}

func resizeBytes(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, n)
	}
	return b[:n]
}
