package render

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

const (
	// layerPageSize is the width and height of a page of layers.
	layerPageSize = 2048
	// layerPageLimit bounds the layers on the GPU to 256 MB.
	layerPageLimit = 16
	// layerPadding keeps the sampler of one layer away from the next.
	layerPadding = 1
	// shelfStep rounds the height of a shelf, so that rows of about the same
	// height share one.
	shelfStep = 8
	// layerPageIdle is how many frames a page must go unused before a new layer
	// takes it over instead of a new page. A list that scrolls would otherwise
	// fill the whole budget with rows that left the screen.
	layerPageIdle = 2
	// layerCalmFrames is how many frames a layer must stay the same before it
	// goes on a page after a change that followed another.
	layerCalmFrames = 2
)

// layerEntry is where a layer lives on the GPU.
type layerEntry struct {
	page *layerPage
	// slot is the room reserved on the page. It can outgrow the pixels in use.
	slot image.Rectangle
	// left and top are the position of the pixels relative to the origin of the
	// layer, and width and height their size, all in physical pixels.
	left, top, width, height int
	version                  uint64
	// changed is the frame of the last new version, and hot means that the
	// versions came too fast to keep up with.
	changed uint64
	hot     bool
	scale   float32
	// inline means that the layer cannot go on a page, so that its commands
	// join the frame each time. See docs/ADR/0019-layers.md.
	inline bool
}

// shelf is a row of a page. Slots fill it from the left.
type shelf struct {
	y, height, used int
}

// layerPool keeps layers on pages of their own. Resources has two: one for the
// layers that the frame draws, and one for those that custom shaders read,
// whose pages are drawn first, so that a layer drawn on a page of the first
// pool can read one of the second.
type layerPool struct {
	entries map[*paint.Layer]*layerEntry
	pages   []*layerPage
	// scratch is where a layer is encoded before it finds its place.
	scratch gpu.Batch
	// limit is the most pages the pool holds.
	limit int
}

func newLayerPool(limit int) layerPool {
	return layerPool{entries: make(map[*paint.Layer]*layerEntry), limit: limit}
}

// destroy releases the pages.
func (pool *layerPool) destroy() {
	for _, p := range pool.pages {
		p.target.Destroy()
	}
	pool.pages = nil
	clear(pool.entries)
}

// layerPage is a texture that holds layers.
type layerPage struct {
	target   gpu.Texture
	shelves  []shelf
	used     int
	lastUsed uint64
	// batch is what Finish draws to the page.
	batch gpu.Batch
}

func (p *layerPage) reset() {
	p.shelves = p.shelves[:0]
	p.used = 0
}

// allocate reserves a slot, or reports that none fits.
func (p *layerPage) allocate(width, height int) (image.Rectangle, bool) {
	for i := range p.shelves {
		s := &p.shelves[i]
		if height <= s.height && height*2 > s.height && s.used+width <= layerPageSize {
			r := image.Rect(s.used, s.y, s.used+width, s.y+s.height)
			s.used += width
			return r, true
		}
	}
	h := (height + shelfStep - 1) / shelfStep * shelfStep
	if p.used+h > layerPageSize || width > layerPageSize {
		return image.Rectangle{}, false
	}
	p.shelves = append(p.shelves, shelf{y: p.used, height: h, used: width})
	r := image.Rect(0, p.used, width, p.used+h)
	p.used += h
	return r, true
}

// encodeLayer draws l, keeping its pixels on a page if it can.
func (g *Resources) encodeLayer(op *paint.Op, scale float32, b *gpu.Batch) {
	if g == nil {
		encodeInline(op, nil, scale, b, &gpu.Batch{})
		return
	}
	if g.groupDepth > 0 {
		// The pages are drawn after the pictures of groups, so a layer inside
		// a group joins it.
		scratch := g.takeScratch()
		encodeInline(op, g, scale, b, scratch)
		g.scratchUsed--
		return
	}
	e := g.layers.entry(op.Layer)
	left, top := snapOrigin(op.Origin, scale)
	g.refresh(&g.layers, op.Layer, e, scale, false)
	if e.inline || e.hot {
		encodeInline(op, g, scale, b, &g.layers.scratch)
		return
	}
	if e.width == 0 {
		return
	}
	e.page.lastUsed = g.frame
	addDraw(b, op, nil, e.page.target)
	size := float32(layerPageSize)
	x, y := float32(e.slot.Min.X+layerPadding), float32(e.slot.Min.Y+layerPadding)
	w, h := float32(e.width), float32(e.height)
	b.Instances = append(b.Instances, gpu.Instance{
		Rect:          geom.Rect{X: (left + float32(e.left)) / scale, Y: (top + float32(e.top)) / scale, W: w / scale, H: h / scale},
		Color:         geom.Color{A: 1},
		GradientStart: geom.Offset{X: x / size, Y: y / size},
		GradientEnd:   geom.Offset{X: (x + w) / size, Y: (y + h) / size},
		Kind:          gpu.InstanceImage,
	})
	// A turned layer draws its upright pixels turned, through the sampler.
	if op.Transform != (geom.Affine{}) {
		b.Instances[len(b.Instances)-1].SetTransform(op.Transform)
	}
}

// entry returns the entry of l, made if l is new.
func (pool *layerPool) entry(l *paint.Layer) *layerEntry {
	e := pool.entries[l]
	if e == nil {
		e = &layerEntry{}
		pool.entries[l] = e
	}
	return e
}

// refresh brings e up to date. A layer that keeps changing gains nothing from its
// pixels, which only add a pass, so that it waits for calm as a part of the frame.
// An eager layer, which a shader reads, has no other way to look right, and never
// waits.
func (g *Resources) refresh(pool *layerPool, l *paint.Layer, e *layerEntry, scale float32, eager bool) {
	if e.version != l.Version() || e.scale != scale {
		e.hot = !eager && e.changed != 0 && g.frame-e.changed <= layerCalmFrames
		e.changed = g.frame
		if e.hot {
			e.version, e.scale = l.Version(), scale
			return
		}
		g.rasterize(pool, l, e, scale)
		return
	}
	if e.hot && (eager || g.frame-e.changed > layerCalmFrames) {
		e.hot = false
		g.rasterize(pool, l, e, scale)
	}
}

// snapOrigin returns where a layer lands, in whole physical pixels. Its pixels
// map one to one to those of the surface.
func snapOrigin(origin geom.Offset, scale float32) (x, y float32) {
	return geom.Floor32(origin.X*scale + 0.5), geom.Floor32(origin.Y*scale + 0.5)
}

// rasterize brings the pixels of l up to date, or marks it as inline.
func (g *Resources) rasterize(pool *layerPool, l *paint.Layer, e *layerEntry, scale float32) {
	e.version, e.scale, e.inline = l.Version(), scale, false
	s := &pool.scratch
	clearBatch(s)
	// The page holds the pictures of the groups in the layer.
	g.rasterizing++
	g.encodeOps(l.DisplayList(), scale, s)
	g.rasterizing--
	if s.Multisampled() || s.CountBlurs() > 0 {
		// The pages have one sample per pixel, and a backdrop blur reads the
		// surface under the layer.
		e.inline = true
		return
	}
	left, top, right, bottom := extent(s, scale)
	e.left, e.top, e.width, e.height = left, top, right-left, bottom-top
	if e.width <= 0 || e.height <= 0 {
		e.width, e.height = 0, 0
		return
	}
	width, height := e.width+2*layerPadding, e.height+2*layerPadding
	if e.page == nil || e.slot.Dx() < width || e.slot.Dy() < height {
		if !g.place(pool, e, width, height) {
			e.inline = true
			return
		}
	}
	p := e.page
	p.lastUsed = g.frame
	p.batch.Clears = append(p.batch.Clears, e.slot)
	// Everything moves by whole physical pixels, which keeps the rounding of the
	// encoder as it was.
	dx := float32(e.slot.Min.X+layerPadding-e.left) / scale
	dy := float32(e.slot.Min.Y+layerPadding-e.top) / scale
	limit := geom.Rect{
		X: float32(e.slot.Min.X+layerPadding) / scale,
		Y: float32(e.slot.Min.Y+layerPadding) / scale,
		W: float32(e.width) / scale,
		H: float32(e.height) / scale,
	}
	appendMoved(&p.batch, s, geom.Offset{X: dx, Y: dy}, geom.Affine{}, limit, true, 0)
}

// place finds a slot for e, making room if it has to. It reports whether it
// could.
func (g *Resources) place(pool *layerPool, e *layerEntry, width, height int) bool {
	if width > layerPageSize || height > layerPageSize {
		return false
	}
	e.page = nil
	for _, p := range pool.pages {
		if r, ok := p.allocate(width, height); ok {
			e.page, e.slot = p, r
			return true
		}
	}
	// The page unused for the longest goes as a whole. Its layers come back when
	// drawn again, which spares the pages from fragmenting.
	var oldest *layerPage
	for _, p := range pool.pages {
		if p.lastUsed != g.frame && (oldest == nil || p.lastUsed < oldest.lastUsed) {
			oldest = p
		}
	}
	idle := oldest != nil && oldest.lastUsed+layerPageIdle <= g.frame
	if !idle && len(pool.pages) < pool.limit {
		t, err := g.store.CreateTarget(layerPageSize, layerPageSize)
		if err != nil {
			g.err = err
			return false
		}
		p := &layerPage{target: t}
		pool.pages = append(pool.pages, p)
		e.slot, _ = p.allocate(width, height)
		e.page = p
		return true
	}
	if oldest == nil {
		return false
	}
	for l, other := range pool.entries {
		if other.page == oldest && other != e {
			delete(pool.entries, l)
		}
	}
	oldest.reset()
	e.slot, _ = oldest.allocate(width, height)
	e.page = oldest
	return true
}

// extent returns what b touches, in physical pixels.
func extent(b *gpu.Batch, scale float32) (left, top, right, bottom int) {
	first := true
	for i := range b.Draws {
		d := &b.Draws[i]
		for k := d.First; k < d.First+d.Count; k++ {
			r := reachOf(&b.Instances[k], scale)
			if d.Clipped {
				r = r.Intersect(d.Clip)
			}
			if r.Size().Empty() {
				continue
			}
			l, t := int(geom.Floor32(r.X*scale)), int(geom.Floor32(r.Y*scale))
			rt, bt := int(geom.Floor32((r.X+r.W)*scale))+1, int(geom.Floor32((r.Y+r.H)*scale))+1
			if first {
				left, top, right, bottom, first = l, t, rt, bt, false
				continue
			}
			left, top, right, bottom = min(left, l), min(top, t), max(right, rt), max(bottom, bt)
		}
	}
	return left, top, right, bottom
}

// reachOf returns the upright bounds of the quad that the vertex shader makes of
// in.
func reachOf(in *gpu.Instance, scale float32) geom.Rect {
	t, turned := in.Affine()
	var m float32
	switch in.Kind.Base() {
	case gpu.InstanceRRect, gpu.InstanceBackdrop:
		m = 1 / scale
	case gpu.InstanceShadow:
		m = 1/scale + 1.5*in.Blur
	}
	if turned {
		// The margins of sdf.vs.hlsl, rounded up.
		m += 1 / (scale * max(t.MinScale(), 1e-6))
	}
	r := in.Rect
	r = geom.Rect{X: r.X - m, Y: r.Y - m, W: r.W + 2*m, H: r.H + 2*m}
	if turned {
		r = t.ApplyRect(r)
	}
	return r
}

// encodeInline adds the commands of a layer to b, at the place and under the
// clips of op.
func encodeInline(op *paint.Op, g *Resources, scale float32, b, scratch *gpu.Batch) {
	clearBatch(scratch)
	g.encodeOps(op.Layer.DisplayList(), scale, scratch)
	left, top := snapOrigin(op.Origin, scale)
	appendMoved(b, scratch, geom.Offset{X: left / scale, Y: top / scale}, op.Transform, op.Clip, op.Clipped, op.ClipLevel)
}

// appendMoved adds src to dst, moved by d and then transformed by m, under the
// clip and level given. The zero m only moves.
//
// The scissor rectangles of src cannot turn with m, so that under a rotation each
// becomes its upright bounds: a clip inside a turned layer that has to be drawn
// inline lets through the corners of those bounds.
func appendMoved(dst, src *gpu.Batch, d geom.Offset, m geom.Affine, clip geom.Rect, clipped bool, level uint8) {
	firstInstance, firstVertex := len(dst.Instances), len(dst.Vertices)
	turned := !m.IsIdentity()
	outer := geom.Identity()
	if turned {
		outer = m
	}
	move := geom.Translation(d)
	for i := range src.Instances {
		in := src.Instances[i]
		if inner, ok := in.Affine(); ok || turned {
			// The coordinates stay where they are, and the transform takes
			// them to the surface.
			in.SetTransform(outer.Mul(move).Mul(inner))
		} else {
			in.Rect = in.Rect.Translate(d)
			if k := in.Kind.Base(); k != gpu.InstanceImage && k != gpu.InstanceShader && k != gpu.InstanceEffect {
				in.GradientStart, in.GradientEnd = in.GradientStart.Add(d), in.GradientEnd.Add(d)
			}
		}
		dst.Instances = append(dst.Instances, in)
	}
	for _, v := range src.Vertices {
		v = v.Add(d)
		if turned {
			v = m.Apply(v)
		}
		dst.Vertices = append(dst.Vertices, v)
	}
	for i := range src.Draws {
		call := src.Draws[i]
		call.First += firstInstance
		call.VertexFirst += firstVertex
		call.ClipLevel += level
		if call.Clipped {
			call.Clip = call.Clip.Translate(d)
			if turned {
				call.Clip = m.ApplyRect(call.Clip)
			}
			if clipped {
				call.Clip = call.Clip.Intersect(clip)
			}
		} else {
			call.Clip, call.Clipped = clip, clipped
		}
		if call.Blend != gpu.BlendSrcOver || call.Blur > 0 {
			call.Bounds = call.Bounds.Translate(d)
			if turned {
				call.Bounds = m.ApplyRect(call.Bounds)
			}
		}
		dst.Draws = append(dst.Draws, call)
	}
}

func clearBatch(b *gpu.Batch) {
	b.Instances = b.Instances[:0]
	b.Vertices = b.Vertices[:0]
	b.Draws = b.Draws[:0]
	b.Clears = b.Clears[:0]
}

// drawLayers draws what rasterize collected. The pages of the layers that
// shaders read go first, since layers on the other pages can read them.
func (g *Resources) drawLayers(scale float32) error {
	if err := g.drawPages(&g.shaderLayers, scale); err != nil {
		return err
	}
	return g.drawPages(&g.layers, scale)
}

func (g *Resources) drawPages(pool *layerPool, scale float32) error {
	for _, p := range pool.pages {
		if len(p.batch.Clears) == 0 {
			continue
		}
		g.resolvePages(&p.batch)
		err := g.store.DrawToTarget(p.target, &p.batch, scale)
		clearBatch(&p.batch)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Resources) resolvePages(b *gpu.Batch) {
	if g.ramps != nil {
		b.Ramps = g.ramps.texture
	}
	for i := range b.Draws {
		if p, ok := b.Draws[i].Texture.(pageTexture); ok {
			b.Draws[i].Texture = g.textures[p]
		}
		if p, ok := b.Draws[i].Image.(colorPageTexture); ok {
			b.Draws[i].Image = g.colorTextures[p]
		}
		if _, ok := b.Draws[i].Texture.(rampsTexture); ok {
			b.Draws[i].Texture = g.ramps.texture
		}
	}
}
