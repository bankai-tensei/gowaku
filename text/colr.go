package text

import (
	"image"
	"math"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"golang.org/x/image/vector"
)

// maxPaintDepth bounds the nesting of a COLR paint graph. The format allows
// cycles through PaintColrGlyph, which a broken font could contain.
const maxPaintDepth = 64

// affine maps (x, y) to (a·x + c·y + e, b·x + d·y + f).
type affine struct{ a, b, c, d, e, f float32 }

// then returns the transform that applies m first and t after it.
func (t affine) then(m affine) affine {
	return affine{
		a: t.a*m.a + t.c*m.b, b: t.b*m.a + t.d*m.b,
		c: t.a*m.c + t.c*m.d, d: t.b*m.c + t.d*m.d,
		e: t.a*m.e + t.c*m.f + t.e, f: t.b*m.e + t.d*m.f + t.f,
	}
}

func (t affine) apply(x, y float32) (float32, float32) {
	return t.a*x + t.c*y + t.e, t.b*x + t.d*y + t.f
}

func (t affine) invert() (affine, bool) {
	det := t.a*t.d - t.b*t.c
	if det == 0 {
		return affine{}, false
	}
	i := affine{a: t.d / det, b: -t.b / det, c: -t.c / det, d: t.a / det}
	i.e, i.f = -(i.a*t.e + i.c*t.f), -(i.b*t.e + i.d*t.f)
	return i, true
}

func translate(x, y float32) affine { return affine{a: 1, d: 1, e: x, f: y} }
func scaling(x, y float32) affine   { return affine{a: x, d: y} }

// around applies m about the point (x, y).
func around(m affine, x, y float32) affine {
	return translate(x, y).then(m).then(translate(-x, -y))
}

func rotation(turns float32) affine {
	s, c := math.Sincos(float64(turns) * math.Pi)
	return affine{a: float32(c), b: float32(s), c: float32(-s), d: float32(c)}
}

func skew(x, y float32) affine {
	return affine{a: 1, b: float32(math.Tan(float64(y) * math.Pi)), c: float32(-math.Tan(float64(x) * math.Pi)), d: 1}
}

func f214(v tables.Fixed214) float32 { return float32(v) / (1 << 14) }

// rgba is a premultiplied color.
type rgba [4]float32

// colorLayer is a premultiplied RGBA image of float channels, the size of the
// glyph being drawn.
type colorLayer struct {
	w, h int
	pix  []float32
}

func newColorLayer(w, h int) *colorLayer {
	return &colorLayer{w: w, h: h, pix: make([]float32, w*h*4)}
}

// colrPainter draws the paint graph of one color glyph.
type colrPainter struct {
	face  *font.Face
	w, h  int
	depth int
	// palette is the default palette, and foreground the color of entry
	// 0xFFFF, which stands for the text color. The atlas cannot know that
	// color, so it is opaque black.
	palette    []tables.ColorRecord
	foreground rgba
	rasterizer vector.Rasterizer
	mask       image.Alpha
}

// paint draws p under the transform m (font units to pixels) onto dst.
func (cp *colrPainter) paint(p tables.PaintTable, m affine, dst *colorLayer) {
	if cp.depth >= maxPaintDepth {
		return
	}
	cp.depth++
	defer func() { cp.depth-- }()
	colr := cp.face.COLR
	switch p := p.(type) {
	case tables.PaintColrLayersResolved:
		// COLRv0: each layer fills a glyph with one palette color.
		for _, l := range p {
			cp.fillGlyph(l.GlyphID, m, dst, func(*colorLayer) rgba { return cp.color(l.PaletteIndex, 1) }, nil)
		}
	case tables.PaintColrLayers:
		layers, err := colr.LayerList.Resolve(p)
		if err != nil {
			return
		}
		for _, l := range layers {
			cp.paint(l, m, dst)
		}
	case tables.PaintGlyph:
		cp.fillGlyph(p.GlyphID, m, dst, nil, p.Paint)
	case tables.PaintColrGlyph:
		if q, ok := colr.Search(p.GlyphID); ok {
			cp.paint(q, m, dst)
		}
	case tables.PaintSolid:
		cp.fillAll(dst, cp.color(p.PaletteIndex, f214(p.Alpha)))
	case tables.PaintVarSolid:
		cp.fillAll(dst, cp.color(p.PaletteIndex, f214(p.Alpha)))
	case tables.PaintLinearGradient, tables.PaintVarLinearGradient,
		tables.PaintRadialGradient, tables.PaintVarRadialGradient,
		tables.PaintSweepGradient, tables.PaintVarSweepGradient:
		cp.gradient(p, m, dst)
	case tables.PaintComposite:
		cp.composite(p, m, dst)
	default:
		if child, t, ok := transformOf(p); ok {
			cp.paint(child, m.then(t), dst)
		}
	}
}

// transformOf returns the child and transform of a paint that only transforms.
// Variations are ignored: the default instance is drawn.
func transformOf(p tables.PaintTable) (tables.PaintTable, affine, bool) {
	switch p := p.(type) {
	case tables.PaintTransform:
		t := p.Transform
		return p.Paint, affine{t.Xx, t.Yx, t.Xy, t.Yy, t.Dx, t.Dy}, true
	case tables.PaintVarTransform:
		t := p.Transform
		return p.Paint, affine{t.Xx, t.Yx, t.Xy, t.Yy, t.Dx, t.Dy}, true
	case tables.PaintTranslate:
		return p.Paint, translate(float32(p.Dx), float32(p.Dy)), true
	case tables.PaintVarTranslate:
		return p.Paint, translate(float32(p.Dx), float32(p.Dy)), true
	case tables.PaintScale:
		return p.Paint, scaling(f214(p.ScaleX), f214(p.ScaleY)), true
	case tables.PaintVarScale:
		return p.Paint, scaling(f214(p.ScaleX), f214(p.ScaleY)), true
	case tables.PaintScaleAroundCenter:
		return p.Paint, around(scaling(f214(p.ScaleX), f214(p.ScaleY)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintVarScaleAroundCenter:
		return p.Paint, around(scaling(f214(p.ScaleX), f214(p.ScaleY)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintScaleUniform:
		return p.Paint, scaling(f214(p.Scale), f214(p.Scale)), true
	case tables.PaintVarScaleUniform:
		return p.Paint, scaling(f214(p.Scale), f214(p.Scale)), true
	case tables.PaintScaleUniformAroundCenter:
		return p.Paint, around(scaling(f214(p.Scale), f214(p.Scale)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintVarScaleUniformAroundCenter:
		return p.Paint, around(scaling(f214(p.Scale), f214(p.Scale)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintRotate:
		return p.Paint, rotation(f214(p.Angle)), true
	case tables.PaintVarRotate:
		return p.Paint, rotation(f214(p.Angle)), true
	case tables.PaintRotateAroundCenter:
		return p.Paint, around(rotation(f214(p.Angle)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintVarRotateAroundCenter:
		return p.Paint, around(rotation(f214(p.Angle)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintSkew:
		return p.Paint, skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), true
	case tables.PaintVarSkew:
		return p.Paint, skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), true
	case tables.PaintSkewAroundCenter:
		return p.Paint, around(skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), float32(p.CenterX), float32(p.CenterY)), true
	case tables.PaintVarSkewAroundCenter:
		return p.Paint, around(skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), float32(p.CenterX), float32(p.CenterY)), true
	}
	return nil, affine{}, false
}

// color returns palette entry i at alpha, premultiplied.
func (cp *colrPainter) color(i uint16, alpha float32) rgba {
	c := cp.foreground
	if int(i) < len(cp.palette) {
		r := cp.palette[i]
		a := float32(r.Alpha) / 255
		c = rgba{float32(r.Red) / 255 * a, float32(r.Green) / 255 * a, float32(r.Blue) / 255 * a, a}
	}
	for k := range c {
		c[k] *= alpha
	}
	return c
}

// fillGlyph draws the outline of glyph gid onto dst, filled with a solid color
// from solid or else with the paint fill.
func (cp *colrPainter) fillGlyph(gid uint16, m affine, dst *colorLayer, solid func(*colorLayer) rgba, fill tables.PaintTable) {
	if !cp.rasterizeMask(gid, m) {
		return
	}
	src := newColorLayer(cp.w, cp.h)
	if solid != nil {
		cp.fillAll(src, solid(src))
	} else {
		cp.paint(fill, m, src)
	}
	for i := range cp.w * cp.h {
		cov := float32(cp.mask.Pix[i]) / 255
		if cov == 0 {
			continue
		}
		s := src.pix[i*4 : i*4+4]
		d := dst.pix[i*4 : i*4+4]
		inv := 1 - s[3]*cov
		for k := range 4 {
			d[k] = s[k]*cov + d[k]*inv
		}
	}
}

// rasterizeMask draws the coverage of the outline of gid into cp.mask.
func (cp *colrPainter) rasterizeMask(gid uint16, m affine) bool {
	outline, ok := cp.face.GlyphDataOutline(font.GID(gid))
	if !ok || len(outline.Segments) == 0 {
		return false
	}
	r := &cp.rasterizer
	r.Reset(cp.w, cp.h)
	open := false
	for i := range outline.Segments {
		seg := &outline.Segments[i]
		pt := func(n int) (float32, float32) { return m.apply(seg.Args[n].X, seg.Args[n].Y) }
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if open {
				r.ClosePath()
			}
			r.MoveTo(pt(0))
			open = true
		case ot.SegmentOpLineTo:
			r.LineTo(pt(0))
		case ot.SegmentOpQuadTo:
			x1, y1 := pt(0)
			x2, y2 := pt(1)
			r.QuadTo(x1, y1, x2, y2)
		case ot.SegmentOpCubeTo:
			x1, y1 := pt(0)
			x2, y2 := pt(1)
			x3, y3 := pt(2)
			r.CubeTo(x1, y1, x2, y2, x3, y3)
		}
	}
	if open {
		r.ClosePath()
	}
	cp.mask.Rect = image.Rect(0, 0, cp.w, cp.h)
	cp.mask.Stride = cp.w
	if cap(cp.mask.Pix) < cp.w*cp.h {
		cp.mask.Pix = make([]byte, cp.w*cp.h)
	}
	cp.mask.Pix = cp.mask.Pix[:cp.w*cp.h]
	clear(cp.mask.Pix)
	r.Draw(&cp.mask, cp.mask.Rect, image.Opaque, image.Point{})
	return true
}

func (cp *colrPainter) fillAll(dst *colorLayer, c rgba) {
	for i := 0; i < len(dst.pix); i += 4 {
		inv := 1 - c[3]
		for k := range 4 {
			dst.pix[i+k] = c[k] + dst.pix[i+k]*inv
		}
	}
}

// composite draws the backdrop and the source apart and combines them.
func (cp *colrPainter) composite(p tables.PaintComposite, m affine, dst *colorLayer) {
	back := newColorLayer(cp.w, cp.h)
	cp.paint(p.BackdropPaint, m, back)
	src := newColorLayer(cp.w, cp.h)
	cp.paint(p.SourcePaint, m, src)
	for i := 0; i < len(back.pix); i += 4 {
		out := compose(p.CompositeMode, rgba(src.pix[i:i+4]), rgba(back.pix[i:i+4]))
		d := dst.pix[i : i+4]
		inv := 1 - out[3]
		for k := range 4 {
			d[k] = out[k] + d[k]*inv
		}
	}
}

// compose combines a source and a backdrop pixel, both premultiplied, by the
// Porter-Duff operators and the separable blend modes. The non-separable modes
// (hue, saturation, color, luminosity) fall back to source over.
func compose(mode tables.CompositeMode, s, b rgba) rgba {
	sa, ba := s[3], b[3]
	var fs, fb float32 // the Porter-Duff factors
	switch mode {
	case tables.CompositeClear:
		return rgba{}
	case tables.CompositeSrc:
		return s
	case tables.CompositeDest:
		return b
	case tables.CompositeDestOver:
		fs, fb = 1-ba, 1
	case tables.CompositeSrcIn:
		fs, fb = ba, 0
	case tables.CompositeDestIn:
		fs, fb = 0, sa
	case tables.CompositeSrcOut:
		fs, fb = 1-ba, 0
	case tables.CompositeDestOut:
		fs, fb = 0, 1-sa
	case tables.CompositeSrcAtop:
		fs, fb = ba, 1-sa
	case tables.CompositeDestAtop:
		fs, fb = 1-ba, sa
	case tables.CompositeXor:
		fs, fb = 1-ba, 1-sa
	case tables.CompositePlus:
		fs, fb = 1, 1
	case tables.CompositeSrcOver:
		fs, fb = 1, 1-sa
	default:
		blend, ok := blendFunc(mode)
		if !ok {
			fs, fb = 1, 1-sa
			break
		}
		// W3C Compositing: co = cs·(1−αb) + cb·(1−αs) + αs·αb·B(Cs, Cb).
		var out rgba
		for k := range 3 {
			cs, cb := unpremul(s[k], sa), unpremul(b[k], ba)
			out[k] = s[k]*(1-ba) + b[k]*(1-sa) + sa*ba*blend(cs, cb)
		}
		out[3] = sa + ba - sa*ba
		return out
	}
	var out rgba
	for k := range 4 {
		out[k] = min(s[k]*fs+b[k]*fb, 1)
	}
	return out
}

func unpremul(c, a float32) float32 {
	if a == 0 {
		return 0
	}
	return c / a
}

func blendFunc(mode tables.CompositeMode) (func(s, b float32) float32, bool) {
	switch mode {
	case tables.CompositeMultiply:
		return func(s, b float32) float32 { return s * b }, true
	case tables.CompositeScreen:
		return func(s, b float32) float32 { return s + b - s*b }, true
	case tables.CompositeDarken:
		return func(s, b float32) float32 { return min(s, b) }, true
	case tables.CompositeLighten:
		return func(s, b float32) float32 { return max(s, b) }, true
	case tables.CompositeDifference:
		return func(s, b float32) float32 { return abs32(s - b) }, true
	case tables.CompositeExclusion:
		return func(s, b float32) float32 { return s + b - 2*s*b }, true
	case tables.CompositeOverlay:
		return func(s, b float32) float32 { return hardLight(b, s) }, true
	case tables.CompositeHardLight:
		return hardLight, true
	}
	return nil, false
}

func hardLight(s, b float32) float32 {
	if s <= 0.5 {
		return b * 2 * s
	}
	return b + (2*s - 1) - b*(2*s-1)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
