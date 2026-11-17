package soft

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
)

// turnedQuad draws an instance with a transform. The vertex shader moves the
// four corners of the quad to the surface, and the GPU rasterizes the two
// triangles of the strip, which share an edge that the top-left rule gives to one
// of them. The varying is affine in the surface position, so its value at the
// center of a pixel is the inverse transform of that center.
func (c *canvas) turnedQuad(f *fragment, m geom.Affine, scissor image.Rectangle, st *quadState) {
	inverse, ok := m.Invert()
	if !ok {
		return
	}
	margin := quadMargin(f.in, c.scale)
	r := f.in.Rect
	x0, y0 := r.X-margin, r.Y-margin
	x1, y1 := x0+(r.W+2*margin), y0+(r.H+2*margin)
	corners := [4]geom.Offset{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x0, Y: y1}, {X: x1, Y: y1}}
	var v [4]fixedPoint
	for i, p := range corners {
		s := m.Apply(p)
		v[i] = fixedPoint{snap(s.X, c.scale, c.width), snap(s.Y, c.scale, c.height)}
	}
	triangles := [2][3]fixedPoint{clockwise(v[0], v[1], v[2]), clockwise(v[1], v[3], v[2])}
	area := image.Rectangle{
		Min: image.Pt(int(min(v[0].x, v[1].x, v[2].x, v[3].x)>>subpixelBits), int(min(v[0].y, v[1].y, v[2].y, v[3].y)>>subpixelBits)),
		Max: image.Pt(int((max(v[0].x, v[1].x, v[2].x, v[3].x)+subpixel-1)>>subpixelBits),
			int((max(v[0].y, v[1].y, v[2].y, v[3].y)+subpixel-1)>>subpixelBits)),
	}.Intersect(scissor)

	var passed [multisampleCount]int
	for py := area.Min.Y; py < area.Max.Y; py++ {
		for px := area.Min.X; px < area.Max.X; px++ {
			first := c.sampleIndex(px, py)
			count := 0
			for s, position := range c.positions {
				p := fixedPoint{int64(px)<<subpixelBits + position.x, int64(py)<<subpixelBits + position.y}
				if !insideTriangle(&triangles[0], p) && !insideTriangle(&triangles[1], p) {
					continue
				}
				if c.pass(first+s, st) {
					passed[count] = first + s
					count++
				}
			}
			if count == 0 || !st.writeColor {
				continue
			}
			f.pixelX, f.pixelY = px, py
			center := geom.Offset{X: (float32(px) + 0.5) / c.scale, Y: (float32(py) + 0.5) / c.scale}
			f.logical = inverse.Apply(center)
			src := c.output(f, st)
			for _, i := range passed[:count] {
				c.merge(i, src, st.blend)
			}
		}
	}
}

// clockwise orders a triangle the way insideEdge expects. A flat triangle
// comes back as it is, and covers nothing.
func clockwise(a, b, d fixedPoint) [3]fixedPoint {
	if edgeValue(a, b, d) < 0 {
		b, d = d, b
	}
	return [3]fixedPoint{a, b, d}
}

func insideTriangle(t *[3]fixedPoint, p fixedPoint) bool {
	if edgeValue(t[0], t[1], t[2]) == 0 {
		return false
	}
	return insideEdge(t[0], t[1], p) && insideEdge(t[1], t[2], p) && insideEdge(t[2], t[0], p)
}
