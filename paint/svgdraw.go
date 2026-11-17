package paint

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/geom"
)

const (
	svgNamespace = "http://www.w3.org/2000/svg"

	// svgReferenceSize is the size on screen, in logical pixels, up to which
	// the outlines of strokes keep within the tolerance of a fill. They are
	// built once, in the units of the drawing; beyond twice this size the
	// flattened curves of a stroke can stray more than a quarter pixel.
	svgReferenceSize = 256

	// svgSizes is how many sizes an SVG keeps scaled copies for.
	svgSizes = 4

	// svgMaxGroups bounds the nesting of groups that composite on their own;
	// deeper groups draw straight into the one around them.
	svgMaxGroups = 24
)

// SVG is a drawing read by ParseSVG. Besides the shapes and styles of icon
// sets (docs/ADR/0045-shaders-svg-animated-images.md), it knows style
// elements with a subset of CSS, use, symbol and defs, linear and radial
// gradients, dashes and markers, clip paths, masks and group opacity
// (docs/ADR/0112-svg-structure-paint-clip.md), filters, and text set by a
// TextOutliner (docs/ADR/0113-svg-filters-text.md).
//
// An SVG is used on the UI goroutine only. Draw keeps scaled copies of the
// shapes, so it is not safe to draw one SVG from several goroutines.
type SVG struct {
	// ViewBox is the area of the drawing that Draw fits into its rectangle.
	ViewBox geom.Rect
	// Width and Height are the size the drawing asks for: its width and
	// height attributes, or else the size of the view box.
	Width, Height float32
	// Skipped counts what ParseSVG left out because it cannot draw it:
	// elements such as image and foreignObject, patterns, filters with a
	// primitive it does not know, textPath, references to other documents
	// and loops of references; and text, until SetText.
	Skipped int

	align svgAlign
	// bounds holds every shape, in the coordinates of the drawing.
	bounds geom.Rect
	shapes []svgShape
	// grads holds the gradients of shapes, in the coordinates of the drawing.
	grads []svgGradient
	// ops is what the drawing draws, in order, and groups the groups that
	// ops open. See svggroup.go.
	ops    []svgOp
	groups []svgGroup
	// composite means that ops open groups, which draw as groups of the
	// canvas.
	composite bool
	// hasText means that the drawing has text; data is then the document,
	// to read again with outliner.
	hasText  bool
	data     []byte
	outliner TextOutliner
	// sizes holds the shapes scaled for the sizes drawn most recently.
	sizes [svgSizes]svgSized
	clock uint64
	// gen counts the times SetText read the document again, which SVGCache
	// sees.
	gen uint64
}

// svgShape is a filled path in the coordinates of the drawing. A stroke is
// the fill of its outline.
type svgShape struct {
	path  *Path
	color geom.Color
	// current means that the color is the one Draw is given, with the alpha
	// of color.
	current bool
	// grad, if positive, fills the shape with SVG.grads[grad-1] instead.
	grad int
}

// svgSized is the shapes of an SVG scaled for one size.
type svgSized struct {
	sx, sy float32
	paths  []*Path
	// grads holds the gradients scaled alike.
	grads []Gradient
	// filters holds the filters of the groups, by group, and clips the paths
	// of clip paths and the regions of masks, each scaled alike.
	filters []Filter
	clips   map[*Path]*Path
	used    uint64
}

// svgAlign is preserveAspectRatio.
type svgAlign struct {
	x, y  float32
	none  bool
	slice bool
}

// ParseSVG reads an SVG document. Elements that it cannot draw are left out and
// counted in Skipped, so that one unusual icon still shows what it can. Broken
// XML, and numbers, lengths or transforms that do not parse, are errors.
//
// Text needs fonts, which paint does not have: it is left out and counted in
// Skipped until SetText supplies a TextOutliner. The SVG widget does that.
func ParseSVG(data []byte) (*SVG, error) {
	return parseSVG(data, nil)
}

func parseSVG(data []byte, o TextOutliner) (*SVG, error) {
	p := svgDocParser{svg: &SVG{align: svgAlign{x: 0.5, y: 0.5}}, outliner: o}
	if err := p.run(data); err != nil {
		return nil, fmt.Errorf("paint: parse svg: %w", err)
	}
	if p.svg.hasText {
		// Kept to set the text later.
		p.svg.data = append([]byte(nil), data...)
	}
	p.svg.outliner = o
	return p.svg, nil
}

// HasText reports whether the drawing has text, which SetText sets.
func (s *SVG) HasText() bool { return s.hasText }

// SetText sets the text of the drawing with o, by reading the document again:
// what was left out for want of fonts is drawn from then on, and Skipped no
// longer counts it. Setting the same o again does nothing, so o must be a
// comparable value, such as a *text.Shaper; a nil o leaves the text out. It
// is meant for the UI goroutine, like Draw. See
// docs/ADR/0113-svg-filters-text.md.
func (s *SVG) SetText(o TextOutliner) error {
	if !s.hasText || s.outliner == o {
		return nil
	}
	n, err := parseSVG(s.data, o)
	if err != nil {
		return err
	}
	n.gen = s.gen + 1
	*s = *n
	return nil
}

// Draw draws the drawing fitted into dst, as preserveAspectRatio says, with
// current as the value of currentColor.
//
// It scales copies of the shapes for the size of dst, and keeps them for the
// four sizes drawn most recently; drawing at one of those sizes allocates
// nothing. The copies are what the path atlas keys on, so an icon drawn at the
// same size again reuses its pixels.
//
// A drawing with groups that composite (an opacity over several shapes, a clip
// path, a mask or a filter) draws them as groups of the canvas, which the GPU
// composites at the scale of the surface (Canvas.PushGroup). See
// docs/ADR/0114-gpu-layers-masks-filters.md.
func (s *SVG) Draw(cv *Canvas, dst geom.Rect, current geom.Color) {
	vb := s.ViewBox
	if vb.W <= 0 || vb.H <= 0 || dst.W <= 0 || dst.H <= 0 || len(s.ops) == 0 {
		return
	}
	sx, sy := dst.W/vb.W, dst.H/vb.H
	if !s.align.none {
		k := min(sx, sy)
		if s.align.slice {
			k = max(sx, sy)
		}
		sx, sy = k, k
	}
	// The float32 conversions keep arm64 from fusing these into FMA, so every
	// architecture rounds alike; see ADR 0086.
	off := geom.Offset{
		X: dst.X + float32((dst.W-float32(vb.W*sx))*s.align.x),
		Y: dst.Y + float32((dst.H-float32(vb.H*sy))*s.align.y),
	}
	z := s.sized(sx, sy)
	if s.composite {
		s.drawComposite(cv, z, dst, off, current, nil)
		return
	}
	paths := z.paths
	cv.Save()
	// Like a browser, clip to the viewport, but only when something reaches
	// out of it: a clip costs a scissor.
	b := s.bounds
	drawn := geom.Rect{X: off.X + float32((b.X-vb.X)*sx), Y: off.Y + float32((b.Y-vb.Y)*sy), W: float32(b.W * sx), H: float32(b.H * sy)}
	if drawn.X < dst.X || drawn.Y < dst.Y || drawn.X+drawn.W > dst.X+dst.W || drawn.Y+drawn.H > dst.Y+dst.H {
		cv.ClipRect(dst)
	}
	cv.Translate(off.X, off.Y)
	for _, o := range s.ops {
		i := o.index
		sh := &s.shapes[i]
		c := sh.color
		if sh.current {
			c = geom.Color{R: current.R, G: current.G, B: current.B, A: current.A * sh.color.A}
		}
		if sh.grad > 0 {
			cv.DrawPath(paths[i], Paint{Gradient: &z.grads[sh.grad-1]})
			continue
		}
		cv.DrawPath(paths[i], Paint{Color: c})
	}
	cv.Restore()
}

// drawComposite draws a drawing with groups, with the view box at off, inside a
// cached group of its own if cache is set. It is apart from Draw, which keeps
// the path of icons as short as it was.
func (s *SVG) drawComposite(cv *Canvas, z *svgSized, dst geom.Rect, off geom.Offset, current geom.Color, cache *SVGCache) {
	if cache != nil {
		cache.check(s, dst, current)
		// The group moves with the translation at PushGroup, so the drawing
		// goes there and draws from zero.
		cv.Save()
		cv.Translate(dst.X, dst.Y)
		off = off.Sub(geom.Offset{X: dst.X, Y: dst.Y})
		dst.X, dst.Y = 0, 0
		cv.PushGroup(GroupStyle{Cache: &cache.group})
	}
	cv.Save()
	// The viewport clips, as overflow does in a browser; groups also keep
	// their pictures inside it.
	cv.ClipRect(dst)
	cv.Translate(off.X, off.Y)
	s.drawOps(cv, z, s.ops, current, 0)
	cv.Restore()
	if cache != nil {
		cv.PopGroup()
		cv.Restore()
	}
}

// SVGCache keeps the composited picture of an SVG drawn at one place on the
// GPU, as a cached group (GroupCache): the next frames that draw the drawing
// at the same size, in the same color, draw that picture instead of running
// the passes of its groups again. Moving the drawing keeps the picture. Give
// each place that draws an SVG its own; the zero value is ready to use.
type SVGCache struct {
	group GroupCache
	// What the picture was drawn for.
	svg     *SVG
	gen     uint64
	w, h    float32
	current geom.Color
}

// check invalidates the group of c unless it was drawn for s at the size of
// dst in current.
func (c *SVGCache) check(s *SVG, dst geom.Rect, current geom.Color) {
	if c.svg == s && c.gen == s.gen && c.w == dst.W && c.h == dst.H && c.current == current {
		return
	}
	c.svg, c.gen, c.w, c.h, c.current = s, s.gen, dst.W, dst.H, current
	c.group.Invalidate()
}

// DrawCached is Draw, keeping the composited picture of a drawing with groups
// in cache from frame to frame. A drawing without groups, such as an icon,
// draws as Draw draws it, and cache stays unused. See
// docs/ADR/0115-cached-groups.md.
func (s *SVG) DrawCached(cv *Canvas, dst geom.Rect, current geom.Color, cache *SVGCache) {
	if !s.composite {
		s.Draw(cv, dst, current)
		return
	}
	vb := s.ViewBox
	if vb.W <= 0 || vb.H <= 0 || dst.W <= 0 || dst.H <= 0 || len(s.ops) == 0 {
		return
	}
	sx, sy, off := s.fit(dst)
	s.drawComposite(cv, s.sized(sx, sy), dst, off, current, cache)
}

// fit returns the scales and the place of the view box that fit it into dst.
func (s *SVG) fit(dst geom.Rect) (sx, sy float32, off geom.Offset) {
	vb := s.ViewBox
	sx, sy = dst.W/vb.W, dst.H/vb.H
	if !s.align.none {
		k := min(sx, sy)
		if s.align.slice {
			k = max(sx, sy)
		}
		sx, sy = k, k
	}
	// The float32 conversions keep arm64 from fusing these into FMA, so every
	// architecture rounds alike; see ADR 0086.
	off = geom.Offset{
		X: dst.X + float32((dst.W-float32(vb.W*sx))*s.align.x),
		Y: dst.Y + float32((dst.H-float32(vb.H*sy))*s.align.y),
	}
	return sx, sy, off
}

// drawShape draws shape i of z.
func (s *SVG) drawShape(cv *Canvas, z *svgSized, paths []*Path, i int, current geom.Color) {
	sh := &s.shapes[i]
	if sh.grad > 0 {
		cv.DrawPath(paths[i], Paint{Gradient: &z.grads[sh.grad-1]})
		return
	}
	c := sh.color
	if sh.current {
		c = geom.Color{R: current.R, G: current.G, B: current.B, A: current.A * sh.color.A}
	}
	cv.DrawPath(paths[i], Paint{Color: c})
}

// drawOps draws ops, which open groups at depth, with the shapes of z.
func (s *SVG) drawOps(cv *Canvas, z *svgSized, ops []svgOp, current geom.Color, depth int) {
	for i := 0; i < len(ops); i++ {
		o := ops[i]
		switch o.kind {
		case svgOpShape:
			s.drawShape(cv, z, z.paths, o.index, current)
		case svgOpPush:
			end := min(max(o.end, i+1), len(ops))
			inner := ops[i+1 : end]
			i = end
			if depth >= svgMaxGroups {
				s.drawOps(cv, z, inner, current, depth)
				continue
			}
			s.drawGroup(cv, z, o.index, inner, current, depth+1)
		}
	}
}

// drawGroup draws inner as group g: filtered, clipped, masked and faded, as one
// picture. A clip is a mask of the alpha of its paths. With both a clip and a
// mask, the mask goes on a group around that of the clip.
func (s *SVG) drawGroup(cv *Canvas, z *svgSized, g int, inner []svgOp, current geom.Color, depth int) {
	gr := &s.groups[g]
	cv.Save()
	cv.PushOpacity(gr.opacity)
	maskMode := MaskLuminance
	if gr.mask != nil && gr.mask.alpha {
		maskMode = MaskAlpha
	}
	outer := gr.mask != nil && gr.clip != nil
	if outer {
		cv.PushGroup(GroupStyle{Mask: maskMode})
	}
	var style GroupStyle
	if gr.filter != nil {
		style.Filter = &z.filters[g]
	}
	switch {
	case gr.clip != nil:
		style.Mask = MaskAlpha
	case gr.mask != nil:
		style.Mask = maskMode
	}
	cv.PushGroup(style)
	s.drawOps(cv, z, inner, current, depth)
	switch {
	case gr.clip != nil:
		cv.BeginMask()
		s.drawClip(cv, z, gr.clip)
	case gr.mask != nil:
		cv.BeginMask()
		s.drawMask(cv, z, gr.mask, current, depth)
	}
	cv.PopGroup()
	if outer {
		cv.BeginMask()
		s.drawMask(cv, z, gr.mask, current, depth)
		cv.PopGroup()
	}
	cv.Restore()
}

// drawClip draws the paths of clip opaque, as the mask of a group. A clip path
// with a clip path of its own draws as a group masked by that one.
func (s *SVG) drawClip(cv *Canvas, z *svgSized, clip *svgClip) {
	if clip.clip != nil {
		cv.PushGroup(GroupStyle{Mask: MaskAlpha})
	}
	for _, p := range clip.paths {
		cv.DrawPath(z.clips[p], Paint{Color: geom.Color{A: 1}})
	}
	if clip.clip != nil {
		cv.BeginMask()
		s.drawClip(cv, z, clip.clip)
		cv.PopGroup()
	}
}

// drawMask draws the content of mask inside its region.
func (s *SVG) drawMask(cv *Canvas, z *svgSized, mask *svgMask, current geom.Color, depth int) {
	region := z.clips[mask.region]
	if region == nil || region.Empty() {
		// A mask without a region hides everything.
		return
	}
	cv.Save()
	cv.ClipPath(region)
	s.drawOps(cv, z, mask.ops, current, depth)
	cv.Restore()
}

// sized returns the shapes scaled by sx and sy, with the origin of the view box
// at zero.
func (s *SVG) sized(sx, sy float32) *svgSized {
	s.clock++
	oldest := 0
	for i := range s.sizes {
		z := &s.sizes[i]
		if z.paths != nil && z.sx == sx && z.sy == sy {
			z.used = s.clock
			return z
		}
		if z.used < s.sizes[oldest].used {
			oldest = i
		}
	}
	// New paths rather than reused ones: the atlas keys on the pointer.
	z := &s.sizes[oldest]
	z.sx, z.sy, z.used = sx, sy, s.clock
	z.paths = make([]*Path, len(s.shapes))
	m := geom.Affine{A: sx, D: sy, E: -s.ViewBox.X * sx, F: -s.ViewBox.Y * sy}
	for i := range s.shapes {
		z.paths[i] = s.shapes[i].path.transformed(m)
	}
	z.grads = make([]Gradient, len(s.grads))
	for i := range s.grads {
		g := s.grads[i].g
		g.Transform = m.Mul(g.Transform)
		z.grads[i] = g
	}
	z.filters, z.clips = nil, nil
	if s.composite {
		z.filters = make([]Filter, len(s.groups))
		z.clips = make(map[*Path]*Path)
		for i := range s.groups {
			g := &s.groups[i]
			if f := g.filter; f != nil {
				z.filters[i] = f.out
				z.filters[i].Transform = m.Mul(f.ctm)
			}
			for c := g.clip; c != nil; c = c.clip {
				for _, p := range c.paths {
					z.clips[p] = p.transformed(m)
				}
			}
			if g.mask != nil && g.mask.region != nil {
				z.clips[g.mask.region] = g.mask.region.transformed(m)
			}
		}
	}
	return z
}
