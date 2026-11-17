package render

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

const (
	// smallPath is the size in physical pixels up to which a path goes through
	// the atlas: an icon gets exact coverage, and costs one quad.
	smallPath = 64

	// joinSteps is the number of triangles around a round stroke join.
	joinSteps = 10

	// straightJoin is the cosine of a turn too small to need a round join.
	straightJoin = 0.98
)

// encodePath draws a path through the atlas if it can, or else with stencil and
// cover.
func encodePath(op *paint.Op, res *Resources, scale float32, b *gpu.Batch) {
	if res != nil && atlasCanDraw(op, scale) {
		res.encodeSmallPath(op, scale, b)
		return
	}
	if op.Color.A > 0 || op.HasGradient {
		first := len(b.Vertices)
		appendFan(op, b)
		bounds := op.Path.Bounds().Scale(op.Zoom()).Translate(op.Origin)
		addShape(b, op, gpu.DrawFill, first, res.coverOf(op, bounds))
	}
	if op.Stroke.Width > 0 && op.Stroke.Color.A > 0 {
		first := len(b.Vertices)
		appendStroke(op, b)
		cover := gpu.Instance{Rect: outset(op.RRect.Rect), Color: op.Stroke.Color, Kind: gpu.InstanceCover}
		if op.Transform != (geom.Affine{}) {
			cover.SetTransform(op.Transform)
		}
		addShape(b, op, gpu.DrawFill, first, cover)
	}
}

func atlasCanDraw(op *paint.Op, scale float32) bool {
	bounds := op.Path.Bounds()
	// A scale transition would fill the atlas with a new size per frame, and a
	// turned path has no pixel grid to match.
	return op.Zoom() == 1 && op.Transform.IsIdentity() && !op.HasGradient && op.Stroke.Width == 0 && op.Path.Rule == paint.FillNonZero &&
		bounds.W*scale <= smallPath && bounds.H*scale <= smallPath
}

func (g *Resources) encodeSmallPath(op *paint.Op, scale float32, b *gpu.Batch) {
	x := op.Origin.X * scale
	pixel := geom.Floor32(x)
	step := uint8((x - pixel) * text.SubpixelSteps)
	y := geom.Floor32(op.Origin.Y*scale + 0.5)
	img := g.atlas.Shape(text.ShapeKey{Path: op.Path, Scale: scale, Subpixel: step})
	if img.Width == 0 {
		return
	}
	addDraw(b, op, pageTexture(img.Page), nil)
	b.Instances = append(b.Instances, gpu.Instance{
		Rect: geom.Rect{
			X: (pixel + float32(img.Left)) / scale,
			Y: (y + float32(img.Top)) / scale,
			W: float32(img.Width) / scale,
			H: float32(img.Height) / scale,
		},
		Radii: [4]float32{float32(img.X), float32(img.Y)},
		Color: op.Color,
		Kind:  gpu.InstanceGlyph,
	})
}

// coverOf returns the instance that paints a shape within bounds. A turned
// cover is the turned quad of bounds, which holds the triangles.
func (g *Resources) coverOf(op *paint.Op, bounds geom.Rect) gpu.Instance {
	in := g.instanceOf(op)
	in.Rect = outset(bounds)
	in.Radii = [4]float32{}
	in.Kind = gpu.InstanceCover
	return in
}

// outset grows r by a pixel, so that rounding cannot cut off an edge.
func outset(r geom.Rect) geom.Rect {
	return geom.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}
}

func addShape(b *gpu.Batch, op *paint.Op, kind gpu.DrawKind, firstVertex int, cover gpu.Instance) {
	b.Draws = append(b.Draws, gpu.DrawCall{
		Kind:        kind,
		First:       len(b.Instances),
		Count:       1,
		VertexFirst: firstVertex,
		VertexCount: len(b.Vertices) - firstVertex,
		EvenOdd:     op.Path != nil && op.Path.Rule == paint.FillEvenOdd,
		Clip:        op.Clip,
		Clipped:     op.Clipped,
		ClipLevel:   op.ClipLevel,
		Blend:       gpu.BlendMode(op.Blend),
	})
	b.Instances = append(b.Instances, cover)
}

func encodeClipPush(op *paint.Op, b *gpu.Batch) {
	first := len(b.Vertices)
	appendFan(op, b)
	addShape(b, op, gpu.DrawClipPush, first, clipCover(op))
}

// encodeClipPop undoes the push of the same bounds and transform, so that its
// quad covers every pixel the push could reach.
func encodeClipPop(op *paint.Op, b *gpu.Batch) {
	addShape(b, op, gpu.DrawClipPop, len(b.Vertices), clipCover(op))
}

func clipCover(op *paint.Op) gpu.Instance {
	in := gpu.Instance{Rect: outset(op.RRect.Rect), Kind: gpu.InstanceCover}
	if op.Transform != (geom.Affine{}) {
		in.SetTransform(op.Transform)
	}
	return in
}

// placer puts the points of a path on the surface: Scale and Origin first, then
// the transform if there is one.
type placer struct {
	zoom   float32
	origin geom.Offset
	m      geom.Affine
	turned bool
}

func placerOf(op *paint.Op) placer {
	return placer{zoom: op.Zoom(), origin: op.Origin, m: op.Transform, turned: !op.Transform.IsIdentity()}
}

// at places p by Scale and Origin.
func (pl *placer) at(p geom.Offset) geom.Offset { return p.Scale(pl.zoom).Add(pl.origin) }

// out applies the transform to a point that at placed.
func (pl *placer) out(p geom.Offset) geom.Offset {
	if !pl.turned {
		return p
	}
	return pl.m.Apply(p)
}

// appendFan adds a triangle fan per contour. Fans of a concave shape overlap and
// stick out, which is what the stencil counting sorts out.
func appendFan(op *paint.Op, b *gpu.Batch) {
	points, starts := op.Path.Polygon()
	pl := placerOf(op)
	for i, start := range starts {
		end := len(points)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		hub := pl.out(pl.at(points[start]))
		for k := start + 1; k+1 < end; k++ {
			b.Vertices = append(b.Vertices, hub, pl.out(pl.at(points[k])), pl.out(pl.at(points[k+1])))
		}
	}
}

// appendStroke covers the outline of p with triangles of one orientation, so
// that where they overlap the non-zero rule keeps them joined. The outline is
// built before the transform, which turns and slants it with the path.
func appendStroke(op *paint.Op, b *gpu.Batch) {
	p := op.Path
	points, starts := p.Polygon()
	pl := placerOf(op)
	half := op.Stroke.Width / 2
	for i, start := range starts {
		end := len(points)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		line := points[start:end]
		n := len(line)
		last := n - 1
		if p.Closed(i) {
			last = n
		}
		for k := range last {
			a, c := pl.at(line[k]), pl.at(line[(k+1)%n])
			appendSegment(&pl, a, c, half, b)
			next := pl.at(line[(k+2)%n])
			if (k+2 < n || p.Closed(i)) && needsJoin(a, c, next) {
				appendDisc(&pl, c, half, b)
			}
		}
		if !p.Closed(i) && n > 0 {
			appendDisc(&pl, pl.at(line[0]), half, b)
			appendDisc(&pl, pl.at(line[n-1]), half, b)
		}
	}
}

func appendSegment(pl *placer, a, c geom.Offset, half float32, b *gpu.Batch) {
	d := c.Sub(a)
	length := geom.Sqrt32(d.X*d.X + d.Y*d.Y)
	if length == 0 {
		return
	}
	n := geom.Offset{X: -d.Y / length * half, Y: d.X / length * half}
	appendTriangle(pl, a.Add(n), c.Add(n), c.Sub(n), b)
	appendTriangle(pl, a.Add(n), c.Sub(n), a.Sub(n), b)
}

func needsJoin(a, c, next geom.Offset) bool {
	u, v := c.Sub(a), next.Sub(c)
	lengths := geom.Sqrt32((u.X*u.X + u.Y*u.Y) * (v.X*v.X + v.Y*v.Y))
	return lengths > 0 && (u.X*v.X+u.Y*v.Y)/lengths < straightJoin
}

// unitCircle holds the corners of a join, which never change.
var unitCircle = [joinSteps + 1]geom.Offset{
	{X: 1, Y: 0}, {X: 0.809017, Y: 0.587785}, {X: 0.309017, Y: 0.951057}, {X: -0.309017, Y: 0.951057},
	{X: -0.809017, Y: 0.587785}, {X: -1, Y: 0}, {X: -0.809017, Y: -0.587785}, {X: -0.309017, Y: -0.951057},
	{X: 0.309017, Y: -0.951057}, {X: 0.809017, Y: -0.587785}, {X: 1, Y: 0},
}

func appendDisc(pl *placer, c geom.Offset, r float32, b *gpu.Batch) {
	for i := range joinSteps {
		appendTriangle(pl, c, c.Add(unitCircle[i].Scale(r)), c.Add(unitCircle[i+1].Scale(r)), b)
	}
}

// appendTriangle adds a triangle, transformed and turned to face the same way
// as all others.
func appendTriangle(pl *placer, a, c, d geom.Offset, b *gpu.Batch) {
	a, c, d = pl.out(a), pl.out(c), pl.out(d)
	u, v := c.Sub(a), d.Sub(a)
	if u.X*v.Y-u.Y*v.X < 0 {
		c, d = d, c
	}
	b.Vertices = append(b.Vertices, a, c, d)
}
