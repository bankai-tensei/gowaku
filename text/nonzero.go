package text

import (
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
)

// nonZeroSamples is the number of scanlines per pixel row of the exact
// non-zero rasterizer. Across a scanline the coverage is exact.
const nonZeroSamples = 16

// nonZeroEdge is an edge of a polygon in pixels, from top to bottom, with dir
// +1 if it went down and -1 if it went up.
type nonZeroEdge struct {
	x0, y0, x1, y1 float32
	dir            int8
}

type nonZeroCrossing struct {
	x   float32
	dir int8
}

// nonZero rasterizes polygons whose contours overlap with the non-zero rule.
//
// The accumulating rasterizer of x/image/vector sums signed areas and clamps
// them, which is exact while no point is inside two contours, or inside
// contours that wind both ways. Where contours of a path overlap at an edge it
// covers the edge twice, so the edge comes out bolder; icon sets do that, for
// example Material Symbols draw some letters twice, and the outline of a stroke
// is made of overlapping pieces. See docs/ADR/0045-shaders-svg-animated-images.md.
type nonZero struct {
	edges     []nonZeroEdge
	crossings []nonZeroCrossing
	row       []float32
}

// reset takes the polygon of a path, already placed on the pixel grid.
func (n *nonZero) reset(points []geom.Offset, starts []int, at func(geom.Offset) (float32, float32)) {
	n.edges = n.edges[:0]
	for i, start := range starts {
		end := len(points)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		count := end - start
		for k := range count {
			ax, ay := at(points[start+k])
			bx, by := at(points[start+(k+1)%count])
			switch {
			case ay < by:
				n.edges = append(n.edges, nonZeroEdge{ax, ay, bx, by, 1})
			case ay > by:
				n.edges = append(n.edges, nonZeroEdge{bx, by, ax, ay, -1})
			}
		}
	}
}

// scan returns the crossings of the edges with the scanline at y, by x.
func (n *nonZero) scan(y float32) []nonZeroCrossing {
	n.crossings = n.crossings[:0]
	for i := range n.edges {
		e := &n.edges[i]
		if y < e.y0 || y >= e.y1 {
			continue
		}
		x := e.x0 + (e.x1-e.x0)*(y-e.y0)/(e.y1-e.y0)
		n.crossings = append(n.crossings, nonZeroCrossing{x, e.dir})
	}
	slices.SortFunc(n.crossings, func(a, b nonZeroCrossing) int {
		switch {
		case a.x < b.x:
			return -1
		case a.x > b.x:
			return 1
		}
		return 0
	})
	return n.crossings
}

// overlapping reports whether some point of the polygon lies inside two
// contours, or the contours wind both ways, which is when the accumulating
// rasterizer can be wrong.
func (n *nonZero) overlapping(height int) bool {
	positive, negative := false, false
	for py := range height {
		for s := range nonZeroSamples {
			w := 0
			cs := n.scan(float32(py) + (float32(s)+0.5)/nonZeroSamples)
			for i, c := range cs {
				w += int(c.dir)
				if i+1 < len(cs) && cs[i+1].x > c.x {
					switch {
					case w > 1 || w < -1:
						return true
					case w > 0:
						positive = true
					case w < 0:
						negative = true
					}
					if positive && negative {
						return true
					}
				}
			}
		}
	}
	return false
}

// draw writes the coverage of the polygon into pix, which is width by height
// and cleared.
func (n *nonZero) draw(pix []byte, width, height int) {
	if cap(n.row) < width+1 {
		n.row = make([]float32, width+1)
	}
	row := n.row[:width+1]
	for py := range height {
		clear(row)
		for s := range nonZeroSamples {
			cs := n.scan(float32(py) + (float32(s)+0.5)/nonZeroSamples)
			w := 0
			var from float32
			for _, c := range cs {
				was := w
				w += int(c.dir)
				switch {
				case was == 0 && w != 0:
					from = c.x
				case was != 0 && w == 0:
					coverSpan(row, from, c.x, width)
				}
			}
		}
		out := pix[py*width : (py+1)*width]
		for x := range width {
			v := row[x] / nonZeroSamples
			out[x] = uint8(min(v, 1)*255 + 0.5)
		}
	}
}

// coverSpan adds the coverage of [from, to) on one scanline to row.
func coverSpan(row []float32, from, to float32, width int) {
	from, to = max(from, 0), min(to, float32(width))
	if to <= from {
		return
	}
	a, b := int(from), int(to)
	if a == b {
		row[a] += to - from
		return
	}
	row[a] += float32(a+1) - from
	for x := a + 1; x < b; x++ {
		row[x]++
	}
	if b < width {
		row[b] += to - float32(b)
	}
}
