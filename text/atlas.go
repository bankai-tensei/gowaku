package text

import (
	"image"
	"math"

	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/vector"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

const (
	// AtlasPageSize is the width and height of an atlas page, in texels.
	AtlasPageSize = 2048

	// atlasMaxPages bounds the memory of the atlas at 32 MB. Past that the
	// least recently used glyphs make room for new ones.
	atlasMaxPages = 8
	// atlasMaxColorPages bounds the color glyphs at 32 MB, four bytes a texel.
	atlasMaxColorPages = 2

	// SubpixelSteps is how many horizontal glyph positions the atlas tells
	// apart within a pixel.
	SubpixelSteps = 4

	// obliqueShear is how far a synthetic oblique leans: the horizontal shift
	// per unit of height above the baseline, tan 12°, as FreeType slants
	// (FT_GlyphSlot_Oblique). Italic text in a face that has no italic is
	// drawn this way. See docs/ADR/0101-synthetic-italic.md.
	obliqueShear = 0.2126
)

// GlyphKey identifies a rasterized glyph.
type GlyphKey struct {
	Face paint.FaceID
	ID   uint32
	// Size is the font size in physical pixels.
	Size float32
	// Subpixel is the horizontal position within a pixel, in 1/SubpixelSteps.
	Subpixel uint8
	// Sideways turns the glyph 90° clockwise about its origin.
	Sideways bool
}

// ShapeKey identifies a rasterized small path.
type ShapeKey struct {
	// Path is the shape. The atlas goes by what it holds, not by its address,
	// so a path that changes gets a new image and equal paths share one.
	Path *paint.Path
	// Scale is physical pixels per unit of the path.
	Scale float32
	// Subpixel is the horizontal position within a pixel, in 1/SubpixelSteps.
	Subpixel uint8
}

// shapeKey is a ShapeKey by the contents of its path.
type shapeKey struct {
	hash     uint64
	scale    float32
	subpixel uint8
}

// shapeEntry is a rasterized path, with a copy of the path that tells it apart
// from another one of the same hash.
type shapeEntry struct {
	path *paint.Path
	// item is the image, or noItem for a path with nothing to draw.
	item int32
}

// GlyphImage says where a glyph is in the atlas. Width 0 means nothing to draw.
type GlyphImage struct {
	// Page is a coverage page, or a color page if Color is set.
	Page int
	// X and Y are the top left texel on the page.
	X, Y int
	// Width and Height are in texels, which map one to one to physical pixels.
	Width, Height int
	// Left and Top go from the glyph origin to the top left of the image.
	Left, Top int
	// Color marks a color glyph, such as an emoji, on a page of premultiplied
	// RGBA. It is drawn as it is, not in the color of the text.
	Color bool
}

// Atlas keeps rasterized glyphs on pages of 8-bit coverage, and color glyphs on
// pages of RGBA. It lives on the CPU; render uploads what changed.
//
// Once the pages reach their limit, the glyphs used least recently make room
// for new ones, one at a time, in shelves of about the right height. A glyph
// used since the last BeginFrame is never evicted. See
// docs/ADR/0079-atlas-lru.md.
type Atlas struct {
	shaper *Shaper
	size   int
	gray   pageSet
	color  pageSet
	// entries and shapes map keys to items, or to noItem for an image with
	// nothing to draw.
	entries map[GlyphKey]int32
	shapes  map[shapeKey]shapeEntry
	// items are the images on the pages, and freeItems the indices of items
	// that hold none.
	items     []atlasItem
	freeItems []int32
	frame     uint64
	// order is scratch for the shelves of a page from the top.
	order []int32

	rasterizer vector.Rasterizer
	// nonZero rasterizes paths whose contours overlap, which rasterizer
	// cannot do exactly.
	nonZero nonZero
	scratch image.Alpha
	// colorScratch holds a color glyph, premultiplied RGBA with a stride of
	// four times its width.
	colorScratch []byte
	painter      colrPainter

	// rasters counts the images drawn, for the tests and benchmarks.
	rasters int
}

// NewAtlas returns an empty atlas for the glyphs of s.
func NewAtlas(s *Shaper) *Atlas { return newAtlas(s, AtlasPageSize, atlasMaxPages) }

func newAtlas(s *Shaper, size, maxPages int) *Atlas {
	return &Atlas{
		shaper:  s,
		size:    size,
		gray:    pageSet{bytes: 1, max: maxPages, head: noItem, tail: noItem},
		color:   pageSet{bytes: 4, max: atlasMaxColorPages, color: true, head: noItem, tail: noItem},
		entries: make(map[GlyphKey]int32),
		shapes:  make(map[shapeKey]shapeEntry),
	}
}

// BeginFrame starts a frame. Glyphs used since the last BeginFrame are safe
// from eviction until the next one.
func (a *Atlas) BeginFrame() { a.frame++ }

// Glyph returns the image of k, rasterizing it on first use. The hit path does
// not allocate.
func (a *Atlas) Glyph(k GlyphKey) GlyphImage {
	i, ok := a.entries[k]
	if !ok {
		i = a.add(k)
		a.entries[k] = i
	}
	if i == noItem {
		return GlyphImage{}
	}
	a.touch(i)
	return a.items[i].img
}

// Shape returns the image of a small path, rasterizing it the first time the
// atlas sees what the path holds. The path may change afterwards. Only the
// non-zero fill rule is supported. The hit path does not allocate.
func (a *Atlas) Shape(k ShapeKey) GlyphImage {
	key := shapeKey{hash: k.Path.Hash(), scale: k.Scale, subpixel: k.Subpixel}
	e, ok := a.shapes[key]
	if !ok || !e.path.Equal(k.Path) {
		// A different path of the same hash takes the place over.
		if ok && e.item != noItem {
			a.orphan(e.item)
		}
		if e.path == nil {
			e.path = &paint.Path{}
		}
		e.path.Set(k.Path)
		e.item = noItem
		a.rasters++
		if raster, drawn := a.rasterizeShape(k); drawn {
			if img, shelf := a.store(raster); img.Width > 0 {
				e.item = a.newItem(img, shelf)
				a.items[e.item].kind = itemShape
				a.items[e.item].shape = key
			}
		}
		a.shapes[key] = e
	}
	if e.item == noItem {
		return GlyphImage{}
	}
	a.touch(e.item)
	return a.items[e.item].img
}

// PageCount returns the number of coverage pages so far.
func (a *Atlas) PageCount() int { return len(a.gray.pages) }

// ColorPageCount returns the number of color pages so far.
func (a *Atlas) ColorPageCount() int { return len(a.color.pages) }

// PageSize returns the width and height of the pages.
func (a *Atlas) PageSize() int { return a.size }

// TakeDirty returns the part of coverage page i that changed since the last
// call, and the pixels of the whole page with a stride of PageSize. The
// rectangle is empty if nothing changed.
func (a *Atlas) TakeDirty(i int) (image.Rectangle, []byte) { return takeDirty(a.gray.pages[i]) }

// TakeColorDirty is TakeDirty for color page i, whose stride is four times
// PageSize.
func (a *Atlas) TakeColorDirty(i int) (image.Rectangle, []byte) { return takeDirty(a.color.pages[i]) }

func takeDirty(p *atlasPage) (image.Rectangle, []byte) {
	r := p.dirty
	p.dirty = image.Rectangle{}
	return r, p.pix
}

// add rasterizes k and stores it as a new item, or returns noItem if it has
// nothing to draw.
func (a *Atlas) add(k GlyphKey) int32 {
	a.rasters++
	var img GlyphImage
	var shelf int32
	if raster, ok := a.rasterizeColor(k); ok {
		img, shelf = a.storeColor(raster)
	} else if raster, ok := a.rasterize(k); ok {
		img, shelf = a.store(raster)
	}
	if img.Width == 0 {
		return noItem
	}
	i := a.newItem(img, shelf)
	a.items[i].kind = itemGlyph
	a.items[i].glyph = k
	return i
}

// store copies a.scratch onto a coverage page.
func (a *Atlas) store(img GlyphImage) (GlyphImage, int32) {
	return a.copyIn(&a.gray, img, a.scratch.Pix, a.scratch.Stride)
}

// storeColor copies a.colorScratch onto a color page.
func (a *Atlas) storeColor(img GlyphImage) (GlyphImage, int32) {
	return a.copyIn(&a.color, img, a.colorScratch, img.Width*4)
}

// copyIn finds room for img in set and copies its pixels there. It returns
// img placed, and its shelf, or a blank image if it is larger than a page.
func (a *Atlas) copyIn(set *pageSet, img GlyphImage, src []byte, stride int) (GlyphImage, int32) {
	if img.Width > a.size || img.Height > a.size {
		// A glyph taller than a page is larger than any screen.
		return GlyphImage{}, 0
	}
	page, shelf, x, y := a.place(set, img.Width, img.Height)
	img.Page, img.X, img.Y, img.Color = page, x, y, set.color
	p := set.pages[page]
	row := img.Width * set.bytes
	for r := range img.Height {
		copy(p.pix[((y+r)*a.size+x)*set.bytes:], src[r*stride:r*stride+row])
	}
	p.dirty = p.dirty.Union(image.Rect(x, y, x+img.Width, y+img.Height))
	return img, shelf
}

// rasterize draws k into a.scratch. It reports false for a glyph with no ink,
// such as a space, or without an outline, such as a bitmap-only glyph.
func (a *Atlas) rasterize(k GlyphKey) (GlyphImage, bool) {
	face := a.shaper.faces[k.Face]
	outline, ok := face.GlyphDataOutline(ot.GID(k.ID))
	if !ok || len(outline.Segments) == 0 {
		return GlyphImage{}, false
	}
	unit := k.Size / float32(face.Upem())
	shift := float32(k.Subpixel) / SubpixelSteps
	slanted := a.shaper.slanted[k.Face]
	// screen maps a point of the outline, y up, to pixels from the origin, y
	// down. A synthetic oblique leans the glyph about its baseline first.
	// Turning clockwise takes the up of the glyph to the right.
	screen := func(p ot.SegmentPoint) (x, y float32) {
		gx := p.X
		if slanted {
			// The conversion keeps arm64 from fusing this into FMA, so
			// every architecture rounds alike; see ADR 0086.
			gx = p.X + float32(obliqueShear*p.Y)
		}
		if k.Sideways {
			return p.Y * unit, gx * unit
		}
		return gx * unit, -p.Y * unit
	}

	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -minX, -minY
	for i := range outline.Segments {
		seg := &outline.Segments[i]
		for _, p := range seg.ArgsSlice() {
			x, y := screen(p)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	left := int(math.Floor(float64(minX + shift)))
	right := int(math.Ceil(float64(maxX + shift)))
	top := int(math.Floor(float64(minY)))
	bottom := int(math.Ceil(float64(maxY)))
	width, height := right-left, bottom-top
	if width <= 0 || height <= 0 {
		return GlyphImage{}, false
	}

	ox, oy := shift-float32(left), -float32(top)
	r := &a.rasterizer
	r.Reset(width, height)
	open := false
	for i := range outline.Segments {
		seg := &outline.Segments[i]
		x := func(n int) float32 { x, _ := screen(seg.Args[n]); return x + ox }
		y := func(n int) float32 { _, y := screen(seg.Args[n]); return y + oy }
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if open {
				r.ClosePath()
			}
			r.MoveTo(x(0), y(0))
			open = true
		case ot.SegmentOpLineTo:
			r.LineTo(x(0), y(0))
		case ot.SegmentOpQuadTo:
			r.QuadTo(x(0), y(0), x(1), y(1))
		case ot.SegmentOpCubeTo:
			r.CubeTo(x(0), y(0), x(1), y(1), x(2), y(2))
		}
	}
	if open {
		r.ClosePath()
	}
	a.finishRaster(width, height)
	return GlyphImage{Width: width, Height: height, Left: left, Top: top}, true
}

// clearScratch sizes a.scratch to width by height and clears it.
func (a *Atlas) clearScratch(width, height int) {
	a.scratch.Rect = image.Rect(0, 0, width, height)
	a.scratch.Stride = width
	if cap(a.scratch.Pix) < width*height {
		a.scratch.Pix = make([]byte, width*height)
	}
	a.scratch.Pix = a.scratch.Pix[:width*height]
	clear(a.scratch.Pix)
}

// finishRaster draws what a.rasterizer holds into a.scratch.
func (a *Atlas) finishRaster(width, height int) {
	a.clearScratch(width, height)
	a.rasterizer.Draw(&a.scratch, a.scratch.Rect, image.Opaque, image.Point{})
}

// rasterizeShape draws a path into a.scratch.
func (a *Atlas) rasterizeShape(k ShapeKey) (GlyphImage, bool) {
	b := k.Path.Bounds()
	shift := float32(k.Subpixel) / SubpixelSteps
	left := int(math.Floor(float64(b.X*k.Scale + shift)))
	right := int(math.Ceil(float64((b.X+b.W)*k.Scale+shift))) + 1
	top := int(math.Floor(float64(b.Y * k.Scale)))
	bottom := int(math.Ceil(float64((b.Y+b.H)*k.Scale))) + 1
	width, height := right-left, bottom-top
	if width <= 0 || height <= 0 {
		return GlyphImage{}, false
	}
	ox, oy := shift-float32(left), -float32(top)
	at := func(p geom.Offset) (x, y float32) { return p.X*k.Scale + ox, p.Y*k.Scale + oy }
	if points, starts := k.Path.Polygon(); len(points) > 0 {
		a.nonZero.reset(points, starts, at)
		if a.nonZero.overlapping(height) {
			a.clearScratch(width, height)
			a.nonZero.draw(a.scratch.Pix, width, height)
			return GlyphImage{Width: width, Height: height, Left: left, Top: top}, true
		}
	}
	r := &a.rasterizer
	r.Reset(width, height)
	open := false
	k.Path.Walk(
		func(p geom.Offset) {
			if open {
				r.ClosePath()
			}
			x, y := at(p)
			r.MoveTo(x, y)
			open = true
		},
		func(p geom.Offset) {
			x, y := at(p)
			r.LineTo(x, y)
		},
		func(c, to geom.Offset) {
			cx, cy := at(c)
			x, y := at(to)
			r.QuadTo(cx, cy, x, y)
		},
		func(c1, c2, to geom.Offset) {
			ax, ay := at(c1)
			bx, by := at(c2)
			x, y := at(to)
			r.CubeTo(ax, ay, bx, by, x, y)
		},
		func() {
			r.ClosePath()
			open = false
		},
	)
	if open {
		r.ClosePath()
	}
	a.finishRaster(width, height)
	return GlyphImage{Width: width, Height: height, Left: left, Top: top}, true
}
