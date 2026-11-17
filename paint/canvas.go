package paint

import "github.com/bankai-tensei/gowaku/geom"

// shadowReach is how far a shadow extends beyond its shape, as a multiple of
// the blur radius. It is three standard deviations.
const shadowReach = 1.5

// canvasState is what Save stores and Restore brings back.
type canvasState struct {
	// A point p of the drawing lands on p*scale + translation of the surface.
	translation geom.Offset
	scale       float32
	// transform carries the drawing from where translation and scale leave it
	// to the surface, once Transform got a rotation or a skew. The zero value
	// is the identity, which keeps the fast path of translation and scale.
	transform geom.Affine
	// flat means that the transform squashes everything onto a line or a
	// point, so that nothing is drawn.
	flat    bool
	clip    geom.Rect
	clipped bool
	opacity float32
	level   uint8
	// blend is what PushBlend set, for commands whose Paint leaves SrcOver.
	blend BlendMode
}

// clipEntry remembers a shaped clip for its OpClipPop.
type clipEntry struct {
	bounds    geom.Rect
	transform geom.Affine
	clip      geom.Rect
	clipped   bool
}

// maxClipLevels is what the depth buffer can tell apart.
const maxClipLevels = 255

// Canvas records drawing commands into a DisplayList. It keeps its storage
// across frames, so recording does not allocate once the buffers have grown.
//
// Call Reset before recording each frame; that includes the first one.
type Canvas struct {
	list  DisplayList
	state canvasState
	stack []canvasState
	// clips holds an entry per shaped clip in effect.
	clips []clipEntry
	// paths hold the copies of the paths that Ops point to: those given to
	// DrawPath and ClipPath, and those that ClipRRect and a turned ClipRect
	// build. They are reused between frames, and each keeps its address.
	paths     []*Path
	pathsUsed int
	// sources says, for each of paths, what it was last copied from, so that
	// a path drawn again unchanged is not copied again.
	sources []pathSource
	// layered means that the canvas belongs to a Layer.
	layered bool
	// stops and stopLists hold the copies of Gradient.MoreStops that Ops point
	// to, reused between frames.
	stops     []GradientStop
	stopLists [][]GradientStop
	// groups holds the groups that are open, innermost last.
	groups []groupEntry
}

// Reset discards everything recorded and returns to the initial state.
func (c *Canvas) Reset() {
	c.list.Ops = c.list.Ops[:0]
	clear(c.list.Shaders)
	c.list.Shaders = c.list.Shaders[:0]
	clear(c.list.Groups)
	c.list.Groups = c.list.Groups[:0]
	c.groups = c.groups[:0]
	c.stack = c.stack[:0]
	c.clips = c.clips[:0]
	c.pathsUsed = 0
	c.stops = c.stops[:0]
	clear(c.stopLists)
	c.stopLists = c.stopLists[:0]
	c.state = canvasState{scale: 1, opacity: 1}
}

// DisplayList returns what was recorded. It is valid until the next Reset.
func (c *Canvas) DisplayList() *DisplayList { return &c.list }

// Save pushes the transform, clip and opacity. Restore brings them back.
func (c *Canvas) Save() { c.stack = append(c.stack, c.state) }

// Restore undoes every state change since the matching Save. It panics if that
// Save came before the PushGroup of a group that is still open.
func (c *Canvas) Restore() {
	if len(c.stack) == 0 {
		panic("gowaku: Canvas.Restore called without a matching Save")
	}
	if len(c.groups) > 0 {
		c.checkGroupRestore()
	}
	// The body of restore, written out: Restore is on the path of every
	// widget, and a second call shows.
	saved := c.stack[len(c.stack)-1]
	for c.state.level > saved.level {
		c.popClip()
	}
	c.state = saved
	c.stack = c.stack[:len(c.stack)-1]
}

// checkGroupRestore panics if the Save that Restore undoes came before the
// PushGroup of the innermost open group.
func (c *Canvas) checkGroupRestore() {
	if len(c.stack) <= c.groups[len(c.groups)-1].depth {
		panic("gowaku: Canvas.Restore called for a Save before PushGroup; call PopGroup first")
	}
}

func (c *Canvas) restore() {
	saved := c.stack[len(c.stack)-1]
	for c.state.level > saved.level {
		c.popClip()
	}
	c.state = saved
	c.stack = c.stack[:len(c.stack)-1]
}

func (c *Canvas) popClip() {
	e := c.clips[len(c.clips)-1]
	c.clips = c.clips[:len(c.clips)-1]
	c.state.level--
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpClipPop,
		RRect:     geom.RRect{Rect: e.bounds},
		Transform: e.transform,
		Clip:      e.clip,
		Clipped:   e.clipped,
		ClipLevel: c.state.level,
	})
}

// Translate moves the origin of everything drawn afterwards.
func (c *Canvas) Translate(dx, dy float32) {
	c.state.translation = c.place(geom.Offset{X: dx, Y: dy})
}

// Scale enlarges everything drawn afterwards by s, around the origin of the
// moment. Text keeps its pixel grid, so use it for transitions, not for zooming.
func (c *Canvas) Scale(s float32) { c.state.scale *= s }

// Transform applies m to everything drawn afterwards, in the coordinates of the
// moment: m acts first, then what was in effect. The zero value changes nothing.
//
// Translation and uniform scale take the same path as Translate and Scale.
// Anything else, such as a rotation or a skew, goes into Op.Transform: shapes
// keep their exact edges, glyphs are sampled from the atlas, and clips become
// stencil clips. See docs/ADR/0044-drawing-extensions.md.
func (c *Canvas) Transform(m geom.Affine) {
	if m.IsIdentity() {
		return
	}
	if m.IsScaleTranslate() {
		c.Translate(m.E, m.F)
		c.Scale(m.A)
		return
	}
	s, t := c.state.scale, c.state.translation
	if s == 0 || m.Det() == 0 {
		c.state.flat = true
		return
	}
	// The fast path keeps placing points with translation and scale, so m has
	// to act between that placement and its undoing: the surface sees
	// T(S(m(p))) = (T S m S⁻¹)(S(p)).
	place := geom.Affine{A: s, D: s, E: t.X, F: t.Y}
	unplace := geom.Affine{A: 1 / s, D: 1 / s, E: -t.X / s, F: -t.Y / s}
	outer := c.state.transform
	if outer == (geom.Affine{}) {
		outer = geom.Identity()
	}
	c.state.transform = outer.Mul(place).Mul(m).Mul(unplace)
}

// turned reports whether a rotation or a skew is in effect.
func (c *Canvas) turned() bool { return c.state.transform != geom.Affine{} }

// place returns where p lands on the surface.
func (c *Canvas) place(p geom.Offset) geom.Offset {
	return p.Scale(c.state.scale).Add(c.state.translation)
}

// placeRect returns where r lands on the surface.
func (c *Canvas) placeRect(r geom.Rect) geom.Rect {
	return r.Scale(c.state.scale).Translate(c.state.translation)
}

// placeRRect returns rr on the surface.
func (c *Canvas) placeRRect(rr geom.RRect) geom.RRect {
	s := c.state.scale
	return geom.RRect{
		Rect:        c.placeRect(rr.Rect),
		TopLeft:     rr.TopLeft * s,
		TopRight:    rr.TopRight * s,
		BottomRight: rr.BottomRight * s,
		BottomLeft:  rr.BottomLeft * s,
	}
}

// placeGradient returns g on the surface, faded by the opacity, with stop
// offsets that never decrease.
func (c *Canvas) placeGradient(g *Gradient) Gradient {
	if g.StopCount < 0 || g.StopCount > MaxGradientStops {
		panic("gowaku: Gradient.StopCount must be from 0 to paint.MaxGradientStops")
	}
	out := *g
	out.Start, out.End, out.Focus = c.place(g.Start), c.place(g.End), c.place(g.Focus)
	if s, t := c.state.scale, c.state.translation; !g.Transform.IsIdentity() && s != 0 {
		// Placing the gradient moves its space with the shape:
		// P·T·P⁻¹ takes placed gradient points to placed shape points.
		place := geom.Affine{A: s, D: s, E: t.X, F: t.Y}
		unplace := geom.Affine{A: 1 / s, D: 1 / s, E: -t.X / s, F: -t.Y / s}
		out.Transform = place.Mul(g.Transform).Mul(unplace)
	}
	if more := g.more(); len(more) > 0 {
		out.MoreStops = c.copyStops(more)
		return out
	}
	out.StartColor, out.EndColor = c.fade(g.StartColor), c.fade(g.EndColor)
	for i := range out.StopCount {
		s := &out.Stops[i]
		s.Color = c.fade(s.Color)
		if i > 0 {
			s.Offset = max(s.Offset, out.Stops[i-1].Offset)
		}
	}
	return out
}

// copyStops returns stops faded by the opacity, with offsets that never
// decrease, in storage that lasts until Reset. Storage that grows leaves the
// copies made before where they are, so their pointers stay valid.
func (c *Canvas) copyStops(stops []GradientStop) *[]GradientStop {
	first := len(c.stops)
	for i, s := range stops {
		s.Color = c.fade(s.Color)
		if i > 0 {
			s.Offset = max(s.Offset, c.stops[len(c.stops)-1].Offset)
		}
		c.stops = append(c.stops, s)
	}
	c.stopLists = append(c.stopLists, c.stops[first:len(c.stops):len(c.stops)])
	return &c.stopLists[len(c.stopLists)-1]
}

// PushBlend draws everything afterwards with mode, unless a Paint names a mode of
// its own. Each command blends on its own, so where two of them overlap the upper
// one blends with the lower one; a group (PushGroup) blends the whole as one
// picture instead. Use Save and Restore to end its effect.
func (c *Canvas) PushBlend(mode BlendMode) { c.state.blend = mode }

// blendOf returns the mode a command with mode in its Paint draws with.
func (c *Canvas) blendOf(mode BlendMode) BlendMode {
	if mode != BlendSrcOver {
		return mode
	}
	return c.state.blend
}

// PushOpacity multiplies the alpha of everything drawn afterwards by a. Use
// Save and Restore to end its effect.
func (c *Canvas) PushOpacity(a float32) { c.state.opacity *= a }

// ClipRect limits drawing to r, on top of any clip already in effect.
func (c *Canvas) ClipRect(r geom.Rect) {
	if c.turned() {
		// A turned rectangle is no longer one for the scissor.
		c.clipShape(geom.RRect{Rect: r})
		return
	}
	r = c.placeRect(r)
	if c.state.clipped {
		r = r.Intersect(c.state.clip)
	}
	c.state.clip, c.state.clipped = r, true
}

// ClipRRect limits drawing to rr, on top of any clip already in effect. A
// rectangle without corners costs nothing, the others cost a stencil pass.
func (c *Canvas) ClipRRect(rr geom.RRect) {
	if rr.TopLeft == 0 && rr.TopRight == 0 && rr.BottomRight == 0 && rr.BottomLeft == 0 {
		c.ClipRect(rr.Rect)
		return
	}
	c.clipShape(rr)
}

// pathSource is the path a copy was made from, as it was then.
type pathSource struct {
	path *Path
	gen  uint64
	hash uint64
}

// keep returns a path of the canvas that lasts until the next Reset, for the
// caller to fill. Its contents are whatever it held in an earlier frame.
func (c *Canvas) keep() *Path {
	if c.pathsUsed == len(c.paths) {
		c.paths = append(c.paths, &Path{})
		c.sources = append(c.sources, pathSource{})
	}
	i := c.pathsUsed
	c.pathsUsed++
	c.sources[i] = pathSource{}
	return c.paths[i]
}

// keepCopy returns a copy of p that lasts until the next Reset. Drawing the
// same unchanged paths in the same order frame after frame, as widgets do,
// finds each copy already made and only checks it. p must be flattened.
func (c *Canvas) keepCopy(p *Path) *Path {
	if c.pathsUsed == len(c.paths) {
		c.paths = append(c.paths, &Path{})
		c.sources = append(c.sources, pathSource{})
	}
	i := c.pathsUsed
	c.pathsUsed++
	q, src := c.paths[i], &c.sources[i]
	// The hash guards against a path assigned over with another one, which
	// brings along the count of changes of the other.
	if src.path != p || src.gen != p.gen || src.hash != p.hash {
		q.Set(p)
		*src = pathSource{path: p, gen: p.gen, hash: p.hash}
	}
	q.Rule = p.Rule
	return q
}

// clipShape clips to rr with a path of its own.
func (c *Canvas) clipShape(rr geom.RRect) {
	if !c.clipping() {
		return
	}
	p := c.keep()
	p.Reset()
	p.AddRRect(rr)
	c.clipPath(p)
}

// ClipPath limits drawing to p, on top of any clip already in effect. Use Save
// and Restore to end its effect. The canvas records a copy of p, so p can be
// changed or reused right after the call.
func (c *Canvas) ClipPath(p *Path) {
	if c.clipping() {
		p.flatten()
		c.clipPath(c.keepCopy(p))
	}
}

// clipping reports whether a clip pushed now takes effect. It panics when the
// depth buffer has no level left for it.
func (c *Canvas) clipping() bool {
	if c.state.level == maxClipLevels {
		panic("gowaku: too many nested clips")
	}
	// Under a flat transform nothing is drawn until the Restore that ends it.
	return !c.state.flat
}

// clipPath pushes the clip of p, which belongs to the canvas.
func (c *Canvas) clipPath(p *Path) {
	bounds := c.placeRect(p.Bounds())
	e := clipEntry{bounds: bounds, transform: c.state.transform, clip: c.state.clip, clipped: c.state.clipped}
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpClipPush,
		RRect:     geom.RRect{Rect: bounds},
		Path:      p,
		Origin:    c.state.translation,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	})
	c.clips = append(c.clips, e)
	c.state.level++
	// The rectangle spares the GPU everything outside it.
	bounds = c.surfaceRect(bounds)
	if c.state.clipped {
		bounds = bounds.Intersect(c.state.clip)
	}
	c.state.clip, c.state.clipped = bounds, true
}

// DrawPath fills p and strokes it, centered on the path. The canvas records a
// copy of p, so p can be changed, or reset and refilled for the next shape,
// right after the call. The copy reuses storage of the canvas, so drawing does
// not allocate once that has grown.
func (c *Canvas) DrawPath(p *Path, pt Paint) {
	bounds := p.Bounds()
	var stroke Stroke
	if pt.Stroke != nil && pt.Stroke.Width > 0 {
		stroke = *pt.Stroke
		stroke.Color = c.fade(stroke.Color)
		half := stroke.Width / 2
		bounds = geom.Rect{X: bounds.X - half, Y: bounds.Y - half, W: bounds.W + stroke.Width, H: bounds.H + stroke.Width}
	}
	bounds = c.placeRect(bounds)
	stroke.Width *= c.state.scale
	if p.Empty() || !c.visible(bounds) {
		return
	}
	op := Op{
		Kind:      OpPath,
		Blend:     c.blendOf(pt.Blend),
		RRect:     geom.RRect{Rect: bounds},
		Color:     c.fade(pt.Color),
		Origin:    c.state.translation,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Stroke:    stroke,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	}
	drawn := op.Color.A > 0 || stroke.Color.A > 0
	if pt.Gradient != nil {
		op.Gradient, op.HasGradient = c.placeGradient(pt.Gradient), true
		drawn = true
	}
	if drawn {
		op.Path = c.keepCopy(p)
		c.list.Ops = append(c.list.Ops, op)
	}
}

// DrawImage draws img stretched into dst, with the given opacity and the default
// FilterQuality. It keeps a pointer to img until the next Reset.
func (c *Canvas) DrawImage(img *Image, dst geom.Rect, opacity float32) {
	c.DrawImageQuality(img, dst, opacity, FilterDefault)
}

// DrawImageQuality is DrawImage with the given filter quality.
func (c *Canvas) DrawImageQuality(img *Image, dst geom.Rect, opacity float32, quality FilterQuality) {
	bounds := c.placeRect(dst)
	faded := c.fade(geom.Color{R: 1, G: 1, B: 1, A: opacity})
	if bounds.Size().Empty() || faded.A <= 0 || !c.visible(bounds) {
		return
	}
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpImage,
		Blend:     c.state.blend,
		Filter:    quality,
		RRect:     geom.RRect{Rect: bounds},
		Color:     faded,
		Image:     img,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	})
}

// DrawRect fills and outlines r: DrawRRect with square corners.
func (c *Canvas) DrawRect(r geom.Rect, p Paint) { c.DrawRRect(geom.RRectAll(r, 0), p) }

// DrawRRect fills and outlines rr.
func (c *Canvas) DrawRRect(rr geom.RRect, p Paint) {
	placed := c.placeRRect(rr)
	if placed.Rect.Size().Empty() || !c.visible(placed.Rect) {
		return
	}
	op := Op{
		Kind:      OpRRect,
		Blend:     c.blendOf(p.Blend),
		RRect:     placed,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Color:     c.fade(p.Color),
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	}
	drawn := op.Color.A > 0
	if p.Gradient != nil {
		g := c.placeGradient(p.Gradient)
		op.Gradient, op.HasGradient = g, true
		drawn = g.visible()
	}
	if p.Stroke != nil && p.Stroke.Width > 0 {
		op.BorderWidth, op.BorderColor = p.Stroke.Width*c.state.scale, c.fade(p.Stroke.Color)
		drawn = drawn || op.BorderColor.A > 0
	}
	if drawn {
		c.list.Ops = append(c.list.Ops, op)
	}
}

// DrawShadow draws the shadow that rr casts, moved by offset. Blur is the blur
// radius as in CSS. A shadow also darkens the area under rr, so draw the shape
// on top of it. Differing corner radii are approximated by the largest one.
func (c *Canvas) DrawShadow(rr geom.RRect, blur float32, color geom.Color, offset geom.Offset) {
	if blur <= 0 {
		// The soft edge formula divides by the blur, and a hard shadow is
		// exactly the shape.
		c.DrawRRect(geom.RRect{
			Rect:        rr.Rect.Translate(offset),
			TopLeft:     rr.TopLeft,
			TopRight:    rr.TopRight,
			BottomRight: rr.BottomRight,
			BottomLeft:  rr.BottomLeft,
		}, Paint{Color: color})
		return
	}
	placed := c.placeRRect(rr)
	placed.Rect = placed.Rect.Translate(offset.Scale(c.state.scale))
	shape := placed.Rect
	blur *= c.state.scale
	reach := blur * shadowReach
	bounds := geom.Rect{X: shape.X - reach, Y: shape.Y - reach, W: shape.W + 2*reach, H: shape.H + 2*reach}
	faded := c.fade(color)
	if shape.Size().Empty() || faded.A <= 0 || !c.visible(bounds) {
		return
	}
	op := Op{
		Kind:      OpShadow,
		Blend:     c.state.blend,
		RRect:     placed,
		Color:     faded,
		Blur:      blur,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	}
	c.list.Ops = append(c.list.Ops, op)
}

// DrawGlyphs draws run with its origin at off, in color. It keeps a pointer to
// run, which must stay unchanged until the next Reset.
func (c *Canvas) DrawGlyphs(run *GlyphRun, off geom.Offset, color geom.Color) {
	origin := c.place(off.Add(run.Origin))
	bounds := run.Bounds.Scale(c.state.scale).Translate(origin)
	if run.Color != (geom.Color{}) {
		color = run.Color
	}
	faded := c.fade(color)
	if len(run.Glyphs) == 0 || faded.A <= 0 || !c.visible(bounds) {
		return
	}
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpGlyphs,
		Blend:     c.state.blend,
		RRect:     geom.RRect{Rect: bounds},
		Color:     faded,
		Glyphs:    run,
		Origin:    origin,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	})
}

// DrawBackdrop blurs what has been drawn so far inside rr, as backdrop-filter:
// blur() does in CSS, with sigma the standard deviation of the Gaussian in
// logical pixels. Draw what goes over it afterwards, such as the translucent fill
// of a glass card. The current clip limits it, and the opacity in effect fades
// the blurred picture over the sharp one. A layer that holds a backdrop reads
// the surface, so it is drawn as part of the frame each time (ADR 0019). See
// docs/ADR/0044-drawing-extensions.md.
func (c *Canvas) DrawBackdrop(rr geom.RRect, sigma float32) {
	placed := c.placeRRect(rr)
	sigma *= c.state.scale
	if !(sigma > 0) || c.state.opacity <= 0 || placed.Rect.Size().Empty() || !c.visible(placed.Rect) {
		return
	}
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpBackdrop,
		RRect:     placed,
		Color:     geom.Color{R: 1, G: 1, B: 1, A: c.state.opacity},
		Blur:      sigma,
		Scale:     c.state.scale,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	})
}

// CanDrawLayer reports whether DrawLayer can stand in for drawing directly. A
// layer holds finished pixels, so it cannot follow a scale, and fading or
// blending it as one picture would differ from fading or blending each of its
// commands: a group (PushGroup) around DrawLayer does that. Layers do not nest
// either: one recorded inside another melts into it. A rotation or a skew from
// Transform is fine: the pixels are drawn turned.
func (c *Canvas) CanDrawLayer() bool {
	return !c.layered && c.state.scale == 1 && c.state.opacity == 1 && c.state.blend == BlendSrcOver
}

// DrawLayer draws l with its origin at off. It keeps a pointer to l until the next
// Reset. It panics unless CanDrawLayer reports true.
func (c *Canvas) DrawLayer(l *Layer, off geom.Offset) {
	if !c.CanDrawLayer() {
		panic("gowaku: Canvas.DrawLayer called while CanDrawLayer reports false")
	}
	origin := c.place(off)
	bounds := l.bounds.Translate(origin)
	if l.Empty() || !c.visible(bounds) {
		return
	}
	c.list.Ops = append(c.list.Ops, Op{
		Kind:      OpLayer,
		RRect:     geom.RRect{Rect: bounds},
		Layer:     l,
		Origin:    origin,
		Scale:     1,
		Transform: c.state.transform,
		Clip:      c.state.clip,
		Clipped:   c.state.clipped,
		ClipLevel: c.state.level,
	})
}

// visible reports whether r, placed by translation and scale, overlaps the clip.
func (c *Canvas) visible(r geom.Rect) bool {
	if c.state.flat {
		return false
	}
	return !c.state.clipped || !c.surfaceRect(r).Intersect(c.state.clip).Size().Empty()
}

// surfaceRect returns the upright bounds on the surface of r, placed by
// translation and scale.
func (c *Canvas) surfaceRect(r geom.Rect) geom.Rect {
	if !c.turned() {
		return r
	}
	return c.state.transform.ApplyRect(r)
}

func (c *Canvas) fade(color geom.Color) geom.Color {
	color.A *= c.state.opacity
	return color
}
