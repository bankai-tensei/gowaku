package render

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// The modes of paint and gpu must match one to one; this fails to compile if
// their counts ever differ.
const _ = uint(gpu.BlendModeCount-paint.BlendModeCount) + uint(paint.BlendModeCount-gpu.BlendModeCount)

const (
	// reachMargin is how far, in logical pixels, the quads of a command can
	// reach past its bounds: the antialiased edge, rounded up.
	reachMargin = 2
	// glyphMargin is that of glyphs, which can overhang the box of their run.
	glyphMargin = 4
)

// opReach returns an upright rectangle that holds every quad op makes.
func opReach(op *paint.Op) geom.Rect {
	r := op.RRect.Rect
	m := float32(reachMargin)
	if op.Kind == paint.OpGlyphs {
		m = glyphMargin
	}
	m += op.Blur * 1.5
	r = geom.Rect{X: r.X - m, Y: r.Y - m, W: r.W + 2*m, H: r.H + 2*m}
	if !op.Transform.IsIdentity() {
		r = op.Transform.ApplyRect(r)
	}
	return r
}

// beginBackdrop prepares for op, whose blend reads the backdrop. All instances
// of one draw call read the same copy of the target, taken before any of them is
// drawn, so op joins the last call only if it does not overlap what that call
// covers. Otherwise it gets a call of its own, which addDraw fills.
func beginBackdrop(b *gpu.Batch, op *paint.Op) {
	r := opReach(op)
	mode := gpu.BlendMode(op.Blend)
	if n := len(b.Draws); n > 0 {
		d := &b.Draws[n-1]
		// A shader starts a call of its own, which addShaderDraw fills.
		if d.Kind == gpu.DrawQuads && d.Blur == 0 && d.Shader == nil && op.Kind != paint.OpShader && d.Blend == mode &&
			sameClip(d, op) && d.Bounds.Intersect(r).Size().Empty() {
			d.Bounds = unionRect(d.Bounds, r)
			return
		}
	}
	// An empty call: addDraw adds to it. If op ends up drawing nothing, the
	// backends skip it.
	b.Draws = append(b.Draws, gpu.DrawCall{
		First:     len(b.Instances),
		Clip:      op.Clip,
		Clipped:   op.Clipped,
		ClipLevel: op.ClipLevel,
		Blend:     mode,
		Bounds:    r,
	})
}

// backdropFits reports whether op can join d as far as its blend goes: a mode
// that reads the backdrop needs what op covers to be part of what beginBackdrop
// cleared for d.
func backdropFits(d *gpu.DrawCall, op *paint.Op) bool {
	if !d.Blend.ReadsBackdrop() {
		return true
	}
	return unionRect(d.Bounds, opReach(op)) == d.Bounds
}

func unionRect(a, b geom.Rect) geom.Rect {
	if a.Size().Empty() {
		return b
	}
	if b.Size().Empty() {
		return a
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return geom.Rect{X: x, Y: y, W: max(a.X+a.W, b.X+b.W) - x, H: max(a.Y+a.H, b.Y+b.H) - y}
}

// finishBlends completes the blended draw calls from first on: their instances
// carry the mode for the pixel shader, and Bounds becomes the exact extent of
// their quads for those that read the backdrop.
func finishBlends(b *gpu.Batch, first int, scale float32) {
	for i := first; i < len(b.Draws); i++ {
		d := &b.Draws[i]
		if d.Blend == gpu.BlendSrcOver {
			continue
		}
		reads := d.Blend.ReadsBackdrop()
		var bounds geom.Rect
		for k := d.First; k < d.First+d.Count; k++ {
			in := &b.Instances[k]
			in.Kind = in.Kind.WithBlend(d.Blend)
			if reads {
				bounds = unionRect(bounds, reachOf(in, scale))
			}
		}
		if reads {
			d.Bounds = bounds
		}
	}
}

// encodeBackdrop adds the backdrop blur of op: a call of its own, whose one
// instance draws the blurred target into the rounded box.
func encodeBackdrop(op *paint.Op, scale float32, b *gpu.Batch) {
	in := gpu.Instance{
		Rect:  op.RRect.Rect,
		Radii: [4]float32{op.RRect.TopLeft, op.RRect.TopRight, op.RRect.BottomRight, op.RRect.BottomLeft},
		Color: op.Color,
		Blur:  op.Blur,
		Kind:  gpu.InstanceBackdrop,
	}
	if op.Transform != (geom.Affine{}) {
		in.SetTransform(op.Transform)
	}
	b.Draws = append(b.Draws, gpu.DrawCall{
		First:     len(b.Instances),
		Count:     1,
		Clip:      op.Clip,
		Clipped:   op.Clipped,
		ClipLevel: op.ClipLevel,
		Blend:     gpu.BlendMode(op.Blend),
		Bounds:    reachOf(&in, scale),
		Blur:      op.Blur,
	})
	b.Instances = append(b.Instances, in)
}
