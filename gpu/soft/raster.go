package soft

import (
	"image"
	"math"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

const (
	// subpixelBits is the precision of vertices in the rasterizer of Direct3D.
	subpixelBits = 8
	subpixel     = 1 << subpixelBits

	// fixedLimit bounds vertices, so that the products of triangle edges fit
	// int64. It is two million pixels away, far outside any surface.
	fixedLimit = 1 << 29
)

// depthFunc is the comparison of a depth test.
type depthFunc uint8

const (
	depthEqual depthFunc = iota
	depthGreater
	depthLess
)

// quadState is how quads pass the output merger. It stands for the depth-stencil
// state and blend state that the GPU backends bind.
type quadState struct {
	depth depthFunc
	// level is the depth that the quad has.
	level uint16
	// writeDepth stores level in the samples that pass.
	writeDepth bool
	// takeStencil passes only samples with a stencil other than zero, and resets
	// them to zero even if they fail the depth test.
	takeStencil bool
	// writeColor runs the pixel shader and blends.
	writeColor bool
	// blend is the mode of the draw call.
	blend gpu.BlendMode
}

// pass runs the stencil and depth tests of sample i and applies their writes.
func (c *canvas) pass(i int, st *quadState) bool {
	if st.takeStencil {
		if c.stencil[i] == 0 {
			return false
		}
		c.stencil[i] = 0
	}
	var passed bool
	switch st.depth {
	case depthEqual:
		passed = st.level == c.depth[i]
	case depthGreater:
		passed = st.level > c.depth[i]
	case depthLess:
		passed = st.level < c.depth[i]
	}
	if passed && st.writeDepth {
		c.depth[i] = st.level
	}
	return passed
}

// draw runs the draw calls of batch. See docs/ADR/0009-paths-and-clips.md for the
// steps. Like the GPU backends it does nothing for a batch without instances.
func (c *canvas) draw(batch *gpu.Batch) {
	if len(batch.Instances) == 0 {
		return
	}
	for i := range batch.Draws {
		call := &batch.Draws[i]
		if call.Count == 0 {
			continue
		}
		if call.Shader != nil && call.Shader.Fallback == nil && batch.Instances[call.First].Kind.Base() != gpu.InstanceEffect {
			// The CPU cannot run the HLSL, and there is nothing in its place.
			// The effects of render it computes itself.
			continue
		}
		scissor := c.scissor(call)
		level := uint16(call.ClipLevel)
		if call.Blur > 0 && !c.blur(batch, call) {
			continue
		}
		if call.Blend.ReadsBackdrop() {
			c.copyBackdrop(gpu.BackdropRect(call, c.scale, c.width, c.height))
		}
		switch call.Kind {
		case gpu.DrawQuads:
			c.quads(batch, call, scissor, &quadState{level: level, writeColor: true, blend: call.Blend})
		case gpu.DrawFill:
			c.fan(batch, call, scissor, level)
			c.quads(batch, call, scissor, &quadState{level: level, takeStencil: true, writeColor: true, blend: call.Blend})
		case gpu.DrawClipPush:
			c.fan(batch, call, scissor, level)
			c.quads(batch, call, scissor, &quadState{depth: depthGreater, level: level + 1, writeDepth: true, takeStencil: true})
		case gpu.DrawClipPop:
			c.quads(batch, call, scissor, &quadState{depth: depthLess, level: level, writeDepth: true})
		}
	}
}

// scissor returns the pixels that call can touch.
func (c *canvas) scissor(call *gpu.DrawCall) image.Rectangle {
	if !call.Clipped {
		return image.Rect(0, 0, c.width, c.height)
	}
	left, top, right, bottom := gpu.PhysicalBounds(call.Clip, c.scale)
	width, height := int32(c.width), int32(c.height)
	return image.Rectangle{
		Min: image.Pt(int(min(max(left, 0), width)), int(min(max(top, 0), height))),
		Max: image.Pt(int(min(max(right, 0), width)), int(min(max(bottom, 0), height))),
	}
}

// quads draws the instances of call.
func (c *canvas) quads(batch *gpu.Batch, call *gpu.DrawCall, scissor image.Rectangle, st *quadState) {
	atlas, _ := call.Texture.(*Texture)
	picture, _ := call.Image.(*Texture)
	ramps, _ := batch.Ramps.(*Texture)
	for i := call.First; i < call.First+call.Count; i++ {
		f := fragment{in: &batch.Instances[i], scale: c.scale, atlas: atlas, picture: picture, ramps: ramps, filtered: c.filtered, shader: call.Shader}
		c.quad(&f, scissor, st)
	}
}

// quad draws one instance. A quad is two triangles to the GPU, but they share the
// color of every pixel and, by the fill rule, own each sample once, so that a
// rectangle test gives the same result.
func (c *canvas) quad(f *fragment, scissor image.Rectangle, st *quadState) {
	if m, ok := f.in.Affine(); ok {
		c.turnedQuad(f, m, scissor, st)
		return
	}
	margin := quadMargin(f.in, c.scale)
	x := newQuadAxis(f.in.Rect.X, f.in.Rect.W, margin, c.scale, c.width)
	y := newQuadAxis(f.in.Rect.Y, f.in.Rect.H, margin, c.scale, c.height)
	if x.low == x.high || y.low == y.high {
		return
	}
	area := image.Rectangle{
		Min: image.Pt(int(x.low>>subpixelBits), int(y.low>>subpixelBits)),
		Max: image.Pt(int((x.high+subpixel-1)>>subpixelBits), int((y.high+subpixel-1)>>subpixelBits)),
	}.Intersect(scissor)

	var passed [multisampleCount]int
	for py := area.Min.Y; py < area.Max.Y; py++ {
		for px := area.Min.X; px < area.Max.X; px++ {
			first := c.sampleIndex(px, py)
			count := 0
			for s, position := range c.positions {
				if x.covers(int64(px)<<subpixelBits+position.x) && y.covers(int64(py)<<subpixelBits+position.y) &&
					c.pass(first+s, st) {
					passed[count] = first + s
					count++
				}
			}
			if count == 0 || !st.writeColor {
				continue
			}
			f.pixelX, f.pixelY = px, py
			f.logical = geom.Offset{X: x.logicalAt(px), Y: y.logicalAt(py)}
			src := c.output(f, st)
			for _, i := range passed[:count] {
				c.merge(i, src, st.blend)
			}
		}
	}
}

// quadMargin returns how far the vertex shader moves the corners out of the
// rectangle of in.
func quadMargin(in *gpu.Instance, scale float32) float32 {
	m, turned := in.Affine()
	margin := 1 / scale
	if turned {
		margin /= max(m.MinScale(), 1e-6)
	}
	kind := in.Kind.Base()
	if kind == gpu.InstanceShadow {
		margin += 1.5 * in.Blur
	}
	if kind >= gpu.InstanceGlyph && kind != gpu.InstanceBackdrop {
		switch {
		case !turned || kind == gpu.InstanceCover:
			margin = 0
		case kind == gpu.InstanceGlyph:
			margin = 1 / scale
		}
	}
	return margin
}

// quadAxis is the extent of a quad along one axis.
type quadAxis struct {
	// start and end are the vertices in rasterizer coordinates, and logicalStart
	// and logicalEnd the varying that they carry.
	start, end               int64
	logicalStart, logicalEnd float32
	// low and high order start and end.
	low, high int64
}

func newQuadAxis(origin, size, margin, scale float32, viewport int) quadAxis {
	a := quadAxis{logicalStart: origin - margin}
	a.logicalEnd = a.logicalStart + (size + 2*margin)
	a.start = snap(a.logicalStart, scale, viewport)
	a.end = snap(a.logicalEnd, scale, viewport)
	a.low, a.high = min(a.start, a.end), max(a.start, a.end)
	return a
}

// covers applies the fill rule to a sample: top and left edges are inside, bottom
// and right edges are not.
func (a *quadAxis) covers(v int64) bool { return a.low <= v && v < a.high }

// logicalAt interpolates the varying at the center of pixel p.
func (a *quadAxis) logicalAt(p int) float32 {
	center := int64(p)<<subpixelBits + subpixel/2
	t := float32(center-a.start) / float32(a.end-a.start)
	return a.logicalStart + t*(a.logicalEnd-a.logicalStart)
}

// snap takes a logical coordinate through the vertex shader and the viewport
// transform, in the same order of float32 operations, to the fixed point of the
// rasterizer. Which way the y axis of NDC points cancels out.
func snap(logical, scale float32, viewport int) int64 {
	size := float32(viewport)
	ndc := logical*scale/size*2 - 1
	physical := (ndc + 1) * 0.5 * size
	fixed := math.RoundToEven(float64(physical) * subpixel)
	return int64(min(max(fixed, -fixedLimit), fixedLimit))
}

// fan marks the triangles of call in the stencil buffer.
func (c *canvas) fan(batch *gpu.Batch, call *gpu.DrawCall, scissor image.Rectangle, level uint16) {
	for i := 0; i+3 <= call.VertexCount; i += 3 {
		v := batch.Vertices[call.VertexFirst+i : call.VertexFirst+i+3]
		c.markTriangle(c.snapVertex(v[0]), c.snapVertex(v[1]), c.snapVertex(v[2]), scissor, level, call.EvenOdd)
	}
}

func (c *canvas) snapVertex(v geom.Offset) fixedPoint {
	return fixedPoint{snap(v.X, c.scale, c.width), snap(v.Y, c.scale, c.height)}
}

// markTriangle applies the stencil operation of the fill rule to the samples that
// the triangle covers and that lie at level.
func (c *canvas) markTriangle(a, b, d fixedPoint, scissor image.Rectangle, level uint16, evenOdd bool) {
	twiceArea := edgeValue(a, b, d)
	if twiceArea == 0 {
		return
	}
	// Direct3D calls a triangle front facing if it runs clockwise on screen. The
	// tests below assume that order.
	front := twiceArea > 0
	if !front {
		b, d = d, b
	}
	area := image.Rectangle{
		Min: image.Pt(int(min(a.x, b.x, d.x)>>subpixelBits), int(min(a.y, b.y, d.y)>>subpixelBits)),
		Max: image.Pt(int((max(a.x, b.x, d.x)+subpixel-1)>>subpixelBits), int((max(a.y, b.y, d.y)+subpixel-1)>>subpixelBits)),
	}.Intersect(scissor)
	for py := area.Min.Y; py < area.Max.Y; py++ {
		for px := area.Min.X; px < area.Max.X; px++ {
			first := c.sampleIndex(px, py)
			for s, position := range c.positions {
				p := fixedPoint{int64(px)<<subpixelBits + position.x, int64(py)<<subpixelBits + position.y}
				if !insideEdge(a, b, p) || !insideEdge(b, d, p) || !insideEdge(d, a, p) {
					continue
				}
				if i := first + s; c.depth[i] == level {
					c.stencil[i] = nextStencil(c.stencil[i], front, evenOdd)
				}
			}
		}
	}
}

// nextStencil is the stencil operation of pipeline P2. INCR and DECR wrap around.
func nextStencil(v uint8, front, evenOdd bool) uint8 {
	switch {
	case evenOdd:
		return ^v
	case front:
		return v + 1
	}
	return v - 1
}

// fixedPoint is a position in rasterizer coordinates: 1/256 of a physical pixel.
type fixedPoint struct{ x, y int64 }

// samplePositions returns the sample pattern of a pixel. Four samples follow the
// standard pattern of Direct3D, which Vulkan shares, given in 1/16 of a pixel from
// the center as (-2, -6), (6, -2), (-6, 2) and (2, 6).
func samplePositions(samples int) []fixedPoint {
	const center = subpixel / 2
	if samples == 1 {
		return []fixedPoint{{center, center}}
	}
	const unit = subpixel / 16
	return []fixedPoint{
		{center - 2*unit, center - 6*unit},
		{center + 6*unit, center - 2*unit},
		{center - 6*unit, center + 2*unit},
		{center + 2*unit, center + 6*unit},
	}
}

// edgeValue is positive for points to the right of the edge from a to b, as seen on
// screen where y grows downward. For a clockwise triangle that is the inside.
func edgeValue(a, b, p fixedPoint) int64 {
	return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
}

// insideEdge applies the top-left rule to an edge of a clockwise triangle: a sample
// on the edge is inside if the edge is a top edge, which runs to the right, or a
// left edge, which runs up.
func insideEdge(a, b, p fixedPoint) bool {
	v := edgeValue(a, b, p)
	if v != 0 {
		return v > 0
	}
	return b.y < a.y || (b.y == a.y && b.x > a.x)
}
