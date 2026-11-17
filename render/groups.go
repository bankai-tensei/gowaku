package render

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// A group of the canvas (paint.Canvas.PushGroup) draws into a picture of its
// own, a target of the GPU. Encode turns each group into jobs, each a batch
// drawn to one target with DrawToTarget, in an order where every job comes after
// those whose targets it reads: the content of the group, which may be
// multisampled, the passes of its filter, those of its mask, and last the
// picture drawn as an image where the group was, under its clip, faded and
// blended. Finish runs the jobs before the pages of the layers, which can hold
// pictures of groups. See docs/ADR/0114-gpu-layers-masks-filters.md.

const (
	// maxGroupSize bounds the side of the picture of a group, in pixels: the
	// targets of Vulkan share a depth buffer of 2048. A larger group draws at
	// a lower resolution and stretches.
	maxGroupSize = 2048
	// groupStep rounds the sizes of the targets of groups, so that pictures of
	// about the same size share one.
	groupStep = 64
	// groupIdle is how many frames a target of groups may go unused before it
	// is destroyed.
	groupIdle = 120
)

// groupTexture is a target that pictures of groups and the results of filters
// borrow.
type groupTexture struct {
	tex           gpu.Texture
	width, height int
	lastUsed      uint64
	inUse         bool
}

// groupJob is a batch to draw to a target before the frame.
type groupJob struct {
	target gpu.Texture
	batch  *gpu.Batch
	scale  float32
}

// beginGroups gives back the targets of the last frame and destroys those that
// have gone unused for long.
func (g *Resources) beginGroups() {
	for _, t := range g.groupTaken {
		t.inUse = false
	}
	g.groupTaken = g.groupTaken[:0]
	g.jobs = g.jobs[:0]
	g.beginGroupCache()
	kept := g.groupTextures[:0]
	for _, t := range g.groupTextures {
		if t.lastUsed+groupIdle < g.frame {
			t.tex.Destroy()
			continue
		}
		kept = append(kept, t)
	}
	clear(g.groupTextures[len(kept):])
	g.groupTextures = kept
}

// takeGroupTexture returns a target of at least width by height that nothing
// else uses until it is released or the frame ends. It returns nil if the
// backend cannot make one, which Finish reports.
func (g *Resources) takeGroupTexture(width, height int) *groupTexture {
	w := (width + groupStep - 1) / groupStep * groupStep
	h := (height + groupStep - 1) / groupStep * groupStep
	var found *groupTexture
	for _, t := range g.groupTextures {
		if !t.inUse && t.width == w && t.height == h {
			found = t
			break
		}
	}
	if found == nil {
		tex, err := g.store.CreateTarget(w, h)
		if err != nil {
			g.err = err
			return nil
		}
		found = &groupTexture{tex: tex, width: w, height: h}
		g.groupTextures = append(g.groupTextures, found)
	}
	found.inUse, found.lastUsed = true, g.frame
	g.groupTaken = append(g.groupTaken, found)
	return found
}

// releaseGroupTexture lets later jobs of the frame take t. The jobs run in the
// order they were made, so that those made before still find what they drew.
func releaseGroupTexture(t *groupTexture) {
	if t != nil {
		t.inUse = false
	}
}

// groupMark is where the jobs and targets of a frame stand, to go back to.
type groupMark struct{ jobs, taken int }

func (g *Resources) markGroups() groupMark { return groupMark{len(g.jobs), len(g.groupTaken)} }

// rollBack forgets the jobs and targets since m.
func (g *Resources) rollBack(m groupMark) {
	for _, t := range g.groupTaken[m.taken:] {
		t.inUse = false
	}
	g.groupTaken = g.groupTaken[:m.taken]
	g.jobs = g.jobs[:m.jobs]
}

// newJob returns an empty batch that Finish draws to t at scale.
func (g *Resources) newJob(t *groupTexture, scale float32) *gpu.Batch {
	if len(g.jobs) == len(g.jobBatches) {
		g.jobBatches = append(g.jobBatches, &gpu.Batch{})
	}
	b := g.jobBatches[len(g.jobs)]
	clearBatch(b)
	b.Ramps = nil
	g.jobs = append(g.jobs, groupJob{target: t.tex, batch: b, scale: scale})
	return b
}

// drawGroups runs the jobs of the frame.
func (g *Resources) drawGroups() error {
	for i := range g.jobs {
		j := &g.jobs[i]
		g.resolvePages(j.batch)
		if err := g.store.DrawToTarget(j.target, j.batch, j.scale); err != nil {
			return err
		}
	}
	return nil
}

// destroyGroups releases the targets of groups.
func (g *Resources) destroyGroups() {
	for _, t := range g.groupTextures {
		t.tex.Destroy()
	}
	g.groupTextures, g.groupTaken, g.jobs = nil, nil, nil
}

// groupRect is the picture of a group: its pixels, at the scale of the group,
// from the origin of the surface.
type groupRect struct {
	x, y, width, height int
	scale               float32
}

// logical returns where r lies on the surface, in logical pixels.
func (r groupRect) logical() geom.Rect {
	return geom.Rect{
		X: float32(r.x) / r.scale, Y: float32(r.y) / r.scale,
		W: float32(r.width) / r.scale, H: float32(r.height) / r.scale,
	}
}

// pixelsOf returns the pixels that region, in logical pixels, touches at scale.
func pixelsOf(region geom.Rect, scale float32) groupRect {
	x0, y0 := geom.Floor32(region.X*scale), geom.Floor32(region.Y*scale)
	x1, y1 := geom.Ceil32((region.X+region.W)*scale), geom.Ceil32((region.Y+region.H)*scale)
	if !(x1 > x0) || !(y1 > y0) {
		return groupRect{scale: scale}
	}
	return groupRect{x: int(x0), y: int(y0), width: min(int(x1-x0), maxGroupSize), height: min(int(y1-y0), maxGroupSize), scale: scale}
}

// fitScale returns what scale shrinks by for region, in logical pixels, to fit
// maxGroupSize.
func fitScale(region geom.Rect, scale float32) float32 {
	return min(1, maxGroupSize/max(region.W*scale, 1), maxGroupSize/max(region.H*scale, 1))
}

// placement returns what takes the coordinates of a command, before Origin and
// Scale place them, to the surface.
func placement(op *paint.Op) geom.Affine {
	z := op.Zoom()
	m := geom.Affine{A: z, D: z, E: op.Origin.X, F: op.Origin.Y}
	if !op.Transform.IsIdentity() {
		m = op.Transform.Mul(m)
	}
	return m
}

// encodeGroup draws the group that the OpGroupPush at dl.Ops[at] opens, and
// returns the index of its OpGroupPop and whether it added a call that blends.
func (g *Resources) encodeGroup(dl *paint.DisplayList, at int, scale float32, b *gpu.Batch) (end int, blended bool) {
	op := &dl.Ops[at]
	call := &dl.Groups[op.ShaderCall]
	end = call.End
	if end < 0 {
		end = len(dl.Ops)
	}
	contentEnd := end
	if call.Mask >= 0 {
		contentEnd = call.Mask
	}
	if g == nil {
		// Without Resources there are no targets: the content draws as it is.
		if call.Style.Cache != nil {
			// It was recorded without the clip.
			var content gpu.Batch
			g.encodeRange(dl, at+1, contentEnd, scale, &content)
			appendMoved(b, &content, geom.Offset{}, geom.Affine{}, op.Clip, op.Clipped, 0)
			return end, false
		}
		g.encodeRange(dl, at+1, contentEnd, scale, b)
		return end, false
	}
	style := &call.Style
	filter := style.Filter
	if style.Mask != paint.MaskNone && call.Mask < 0 || op.Color.A <= 0 || op.Blend == paint.BlendDst ||
		filter != nil && (len(filter.Primitives) == 0 || filter.Region.Size().Empty()) {
		// An empty mask, no opacity or an empty filter shows nothing.
		return end, false
	}
	// A cached group keeps its picture where pages would not hold it twice,
	// and where it can move by whole pixels.
	cached := style.Cache != nil && g.rasterizing == 0 && op.Transform.IsIdentity()
	var entry *cachedGroup
	if cached {
		entry = g.cachedGroups[style.Cache]
		version := style.Cache.Version()
		if entry != nil && entry.fits(version, op, scale) {
			if entry.tex != nil {
				entry.lastUsed = g.frame
				return end, g.compositeGroup(op, entry.tex, entry.moved(op, scale), b)
			}
		} else if style.Filter == nil && style.Mask == paint.MaskNone && op.Color.A == 1 && op.Blend == paint.BlendSrcOver {
			// The first draw of what a plain group draws joins the frame, as
			// it would without the group: content that changes every frame
			// costs nothing more. The next draw keeps the picture.
			g.rememberGroup(style.Cache, entry, version, op, scale)
			content := g.takeScratch()
			clearBatch(content)
			g.encodeRange(dl, at+1, contentEnd, scale, content)
			appendMoved(b, content, geom.Offset{}, geom.Affine{}, op.Clip, op.Clipped, 0)
			g.scratchUsed--
			return end, false
		}
	}
	g.groupDepth++
	content := g.takeScratch()
	mark := g.markGroups()
	var region geom.Rect
	var fm geom.Affine
	var k float32
	if filter != nil {
		fm = placement(op)
		if !filter.Transform.IsIdentity() {
			fm = fm.Mul(filter.Transform)
		}
		region = fm.ApplyRect(filter.Region)
		if cached && fitScale(region, scale) < 1 {
			cached = false
		}
		if op.Clipped && !cached {
			reach := filterReach(filter, fm)
			c := op.Clip
			region = region.Intersect(geom.Rect{X: c.X - reach, Y: c.Y - reach, W: c.W + 2*reach, H: c.H + 2*reach})
		}
		k = fitScale(region, scale)
		clearBatch(content)
		if region.W > 0 && region.H > 0 && k > 0 {
			// A region that is empty, or not a number under a flat
			// transform, draws nothing, and its content is not encoded.
			g.encodeRange(dl, at+1, contentEnd, scale*k, content)
		}
	} else {
		clearBatch(content)
		g.encodeRange(dl, at+1, contentEnd, scale, content)
		if style.Mask == paint.MaskNone && foldable(op, content) {
			// One shape faded alone looks as the group would: it needs no
			// picture.
			appendMoved(b, content, geom.Offset{}, geom.Affine{}, op.Clip, op.Clipped, 0)
			fade(&b.Instances[len(b.Instances)-1], op.Color.A)
			g.scratchUsed--
			g.groupDepth--
			return end, false
		}
		left, top, right, bottom := extent(content, scale)
		region = geom.Rect{X: float32(left) / scale, Y: float32(top) / scale, W: float32(right-left) / scale, H: float32(bottom-top) / scale}
		if cached && fitScale(region, scale) < 1 {
			// Too large to keep whole: it draws as an uncached group, cut to
			// the clip.
			cached = false
		}
		if op.Clipped && !cached {
			region = region.Intersect(op.Clip)
		}
		if k = fitScale(region, scale); k < 1 {
			// The content has to be encoded again at the scale it is drawn at,
			// which glyphs and the paths of the atlas depend on.
			g.rollBack(mark)
			clearBatch(content)
			g.encodeRange(dl, at+1, contentEnd, scale*k, content)
		}
	}
	r := pixelsOf(region, scale*k)
	var picture *groupTexture
	if r.width > 0 && r.height > 0 {
		picture = g.drawContent(content, op, r, filter != nil)
		if picture != nil && filter != nil {
			// Filter coordinates to the pixels of the picture.
			px := geom.Affine{A: r.scale, D: r.scale, E: -float32(r.x), F: -float32(r.y)}.Mul(fm)
			picture = g.runFilter(filter, px, r, picture)
		}
		if picture != nil && style.Mask != paint.MaskNone {
			clearBatch(content)
			g.encodeRange(dl, call.Mask+1, end, r.scale, content)
			mask := g.drawContent(content, op, r, false)
			picture = g.applyMask(picture, mask, style.Mask == paint.MaskLuminance, r)
		}
	}
	g.scratchUsed--
	g.groupDepth--
	if picture == nil {
		return end, false
	}
	if cached {
		g.keepGroup(style.Cache, entry, picture, r, style.Cache.Version(), op, scale)
	}
	return end, g.compositeGroup(op, picture, r, b)
}

// foldable reports whether the group of op, without a filter or a mask, can draw
// its content, encoded in content, straight onto the surface with its opacity:
// a single shape that blends source-over, inside and out, and whose colors the
// instance holds (not a custom shader, nor gradient stops from the ramps).
func foldable(op *paint.Op, content *gpu.Batch) bool {
	if op.Blend != paint.BlendSrcOver || len(content.Draws) != 1 || len(content.Instances) != 1 ||
		len(content.Vertices) != 0 || len(content.Clears) != 0 {
		return false
	}
	d := &content.Draws[0]
	if d.Kind != gpu.DrawQuads || d.Count != 1 || d.Blend != gpu.BlendSrcOver || d.Blur != 0 || d.Shader != nil {
		return false
	}
	in := &content.Instances[0]
	switch in.Kind.Base() {
	case gpu.InstanceShader, gpu.InstanceEffect, gpu.InstanceBackdrop:
		return false
	}
	return in.Gradient&gpu.GradientRamp == 0
}

// fade multiplies the colors of in by the opacity a.
func fade(in *gpu.Instance, a float32) {
	in.Color.A *= a
	in.EndColor.A *= a
	in.BorderColor.A *= a
	in.MidColors[0].A *= a
	in.MidColors[1].A *= a
}

// filterReach returns how far, in logical pixels, a filter can carry what lies
// outside the clip into it: by blurs, offsets and dilation.
func filterReach(f *paint.Filter, fm geom.Affine) float32 {
	sx, sy := axisScales(fm)
	s := max(sx, sy)
	var reach float32
	for i := range f.Primitives {
		p := &f.Primitives[i]
		switch p.Kind {
		case paint.FilterBlur, paint.FilterDropShadow:
			reach += 3*max(geom.Abs32(p.StdDev.X), geom.Abs32(p.StdDev.Y))*s + (geom.Abs32(p.Offset.X)+geom.Abs32(p.Offset.Y))*s
		case paint.FilterOffset:
			reach += (geom.Abs32(p.Offset.X) + geom.Abs32(p.Offset.Y)) * s
		case paint.FilterMorphology:
			reach += max(geom.Abs32(p.Radius.X), geom.Abs32(p.Radius.Y)) * s
		}
	}
	return reach + 1
}

// axisScales returns how much m stretches the x and the y axis.
func axisScales(m geom.Affine) (float32, float32) {
	return geom.Sqrt32(m.A*m.A + m.B*m.B), geom.Sqrt32(m.C*m.C + m.D*m.D)
}

// drawContent makes the job that draws content, encoded on the surface, into a
// new target that holds the pixels of r, and returns the target. Clip levels
// count from the group; the scissor of the group itself goes for a filtered
// group, whose filter can carry what lies outside it inside.
func (g *Resources) drawContent(content *gpu.Batch, op *paint.Op, r groupRect, filtered bool) *groupTexture {
	t := g.takeGroupTexture(r.width, r.height)
	if t == nil {
		return nil
	}
	jb := g.newJob(t, r.scale)
	jb.Clears = append(jb.Clears, image.Rect(0, 0, r.width, r.height))
	d := geom.Offset{X: -float32(r.x) / r.scale, Y: -float32(r.y) / r.scale}
	appendMoved(jb, content, d, geom.Affine{}, geom.Rect{}, false, 0)
	outer := op.Clip.Translate(d)
	for i := range jb.Draws {
		c := &jb.Draws[i]
		c.ClipLevel -= min(c.ClipLevel, op.ClipLevel)
		if filtered && op.Clipped && c.Clipped && c.Clip == outer {
			c.Clipped = false
		}
	}
	if jb.Multisampled() {
		keepSingleCopy(jb)
	}
	return t
}

// keepSingleCopy takes out of a batch that needs multisampling what a
// multisampled target cannot do: a blend that reads the backdrop draws
// source-over, and a backdrop blur draws nothing.
func keepSingleCopy(b *gpu.Batch) {
	for i := range b.Draws {
		c := &b.Draws[i]
		if c.Blend.ReadsBackdrop() {
			c.Blend = gpu.BlendSrcOver
			for k := c.First; k < c.First+c.Count; k++ {
				b.Instances[k].Kind = b.Instances[k].Kind.Base()
			}
		}
		if c.Blur > 0 {
			// Only a backdrop blur, a call of its own, blurs.
			c.Blur, c.Count = 0, 0
		}
	}
}

// compositeGroup draws the picture of a group at r into b, under the clip of op,
// faded and blended. It reports whether it added a call that blends.
func (g *Resources) compositeGroup(op *paint.Op, t *groupTexture, r groupRect, b *gpu.Batch) bool {
	rect := r.logical()
	mode := gpu.BlendMode(op.Blend)
	if mode == gpu.BlendSrcOver {
		addDraw(b, op, nil, t.tex)
	} else {
		b.Draws = append(b.Draws, gpu.DrawCall{
			First:     len(b.Instances),
			Count:     1,
			Image:     t.tex,
			Clip:      op.Clip,
			Clipped:   op.Clipped,
			ClipLevel: op.ClipLevel,
			Blend:     mode,
			Bounds:    rect,
		})
	}
	b.Instances = append(b.Instances, gpu.Instance{
		Rect:        rect,
		Color:       geom.Color{A: op.Color.A},
		GradientEnd: geom.Offset{X: float32(r.width) / float32(t.width), Y: float32(r.height) / float32(t.height)},
		Kind:        gpu.InstanceImage,
	})
	return mode != gpu.BlendSrcOver
}

// effect returns the shader of the passes of filters and masks, or nil if the
// backend refused it, in which case groups draw without them.
func (g *Resources) effect() *gpu.Shader {
	e := g.shaderOf(effectShader)
	if e.failed {
		return nil
	}
	return &e.code
}

// effectPass makes the job of one pass that writes out, all r of it, or sub of
// it if sub is not empty. in0 and in1 are the textures it reads, which can be
// nil when its flags say so.
func (g *Resources) effectPass(out *groupTexture, r groupRect, op gpu.EffectOp, flags uint32, p *gpu.EffectParams,
	in0, in1 gpu.Texture, sub image.Rectangle) {
	shader := g.effect()
	b := g.newJob(out, 1)
	b.Clears = append(b.Clears, image.Rect(0, 0, r.width, r.height))
	if shader == nil {
		return
	}
	full := image.Rect(0, 0, r.width, r.height)
	if sub.Empty() {
		sub = full
	}
	p[3] = [4]float32{float32(r.width), float32(r.height)}
	b.Draws = append(b.Draws, gpu.DrawCall{
		Count:   1,
		Image:   in0,
		Texture: in1,
		Shader:  shader,
		Clip:    geom.Rect{X: float32(sub.Min.X), Y: float32(sub.Min.Y), W: float32(sub.Dx()), H: float32(sub.Dy())},
		Clipped: true,
	})
	b.Instances = append(b.Instances, gpu.Instance{})
	b.Instances[0].SetEffect(geom.Rect{W: float32(r.width), H: float32(r.height)}, op, flags, p)
}

// applyMask returns picture multiplied by mask, a new target, and releases both.
// A mask that could not be drawn hides everything.
func (g *Resources) applyMask(picture, mask *groupTexture, luminance bool, r groupRect) *groupTexture {
	if mask == nil {
		releaseGroupTexture(picture)
		return nil
	}
	out := g.takeGroupTexture(r.width, r.height)
	if out != nil {
		var flags uint32
		if luminance {
			flags = gpu.EffectLuminance
		}
		var p gpu.EffectParams
		g.effectPass(out, r, gpu.EffectMask, flags, &p, picture.tex, mask.tex, image.Rectangle{})
	}
	releaseGroupTexture(picture)
	releaseGroupTexture(mask)
	return out
}
