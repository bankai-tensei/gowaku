package paint

import (
	"math"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// svgVertex is a point of a path where a marker can go, with the directions
// in which the path arrives and leaves; a zero direction is none.
type svgVertex struct {
	pos     geom.Offset
	in, out geom.Offset
}

// svgVertices collects the vertices of a path in the order of its commands.
type svgVertices struct {
	list []svgVertex
	// start is the index of the first vertex of the current subpath, or -1
	// after a close.
	start int
}

func (v *svgVertices) move(p geom.Offset) {
	v.list = append(v.list, svgVertex{pos: p})
	v.start = len(v.list) - 1
}

// segment adds a segment from the last vertex to to, leaving it in direction
// out and arriving in direction in.
func (v *svgVertices) segment(from, to, out, in geom.Offset) {
	if v.start < 0 || len(v.list) == 0 {
		// A command after a close starts a new subpath where it closed.
		v.move(from)
	}
	v.list[len(v.list)-1].out = out
	v.list = append(v.list, svgVertex{pos: to, in: in})
}

// close draws the line back to the start of the subpath, and joins its two
// ends: the last vertex leaves along the first segment, and the first is
// entered along the closing one.
func (v *svgVertices) close() {
	if v.start < 0 || len(v.list) == 0 {
		return
	}
	first, last := v.list[v.start].pos, v.list[len(v.list)-1].pos
	if first != last {
		d := first.Sub(last)
		v.segment(last, first, d, d)
	}
	end := &v.list[len(v.list)-1]
	end.out = v.list[v.start].out
	v.list[v.start].in = end.in
	v.start = -1
}

// pointVertices returns the vertices of a polyline, closed for a polygon.
func pointVertices(pts []geom.Offset, closed bool) []svgVertex {
	v := svgVertices{start: -1}
	for i, p := range pts {
		if i == 0 {
			v.move(p)
			continue
		}
		d := p.Sub(pts[i-1])
		v.segment(pts[i-1], p, d, d)
	}
	if closed {
		v.close()
	}
	return v.list
}

// parseDashes reads stroke-dasharray. An odd list repeats; a list with a
// negative length or nothing but zeros is solid, as are none and values that
// do not parse.
func parseDashes(value string, ref float32) []float32 {
	value = strings.TrimSpace(value)
	if value == "" || value == "none" {
		return nil
	}
	var dashes []float32
	sum := float32(0)
	for f := range strings.FieldsFuncSeq(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
		d, err := parseLength(f, ref)
		if err != nil || d < 0 {
			return nil
		}
		dashes = append(dashes, d)
		sum += d
	}
	if sum <= 0 {
		return nil
	}
	if len(dashes)%2 == 1 {
		dashes = append(dashes, dashes...)
	}
	return dashes
}

// svgMaxDashes bounds the dashes of one path. Tiny dashes along a long path
// would otherwise take a vast amount of memory; past the bound the stroke is
// drawn solid.
const svgMaxDashes = 1 << 16

// dash returns the dashes of path p as open subpaths, flattened to within tol.
// dashes are the lengths of dashes and gaps, offset is where along them the
// path starts, and scale multiplies them, as pathLength asks. Dashes that run
// over the start of a closed contour are joined.
func dash(p *Path, dashes []float32, offset, scale, tol float32) *Path {
	// The path flattens with a fixed tolerance, so it is scaled to make that
	// tolerance tol.
	k := float32(flattenTolerance) / max(tol, 1e-6)
	flat := p.transformed(geom.Scaling(k, k))
	pts, starts := flat.Polygon()
	total := float32(0)
	for _, d := range dashes {
		total += d * scale
	}
	if !(total > 0) || math.IsInf(float64(total), 0) {
		return p
	}
	out := &Path{}
	count := 0
	for ci, start := range starts {
		end := len(pts)
		if ci+1 < len(starts) {
			end = starts[ci+1]
		}
		contour := pts[start:end]
		if flat.Closed(ci) && len(contour) > 0 {
			contour = append(contour[:len(contour):len(contour)], contour[0])
		}
		if len(contour) < 2 {
			continue
		}
		// Where in the pattern the contour starts.
		// The offset is reduced in float64: in float32 a large offset
		// over a short pattern leaves a phase that no dash can eat into.
		ph := math.Mod(float64(offset)*float64(scale), float64(total))
		if ph < 0 {
			ph += float64(total)
		}
		if math.IsNaN(ph) {
			ph = 0
		}
		phase := float32(ph)
		i := 0
		// The phase is under one pattern, so one pass over the dashes
		// is enough; rounding can leave a sliver, which is dropped.
		for n := 0; phase >= dashes[i]*scale && phase > 0; n++ {
			if n == len(dashes) {
				phase = 0
				break
			}
			phase -= dashes[i] * scale
			i = (i + 1) % len(dashes)
		}
		on := i%2 == 0
		left := dashes[i]*scale - phase
		firstMove, firstOn := len(out.verbs), on
		var lastMove int
		startedAtStart := on
		pen := contour[0].Scale(1 / k)
		if on {
			lastMove = len(out.verbs)
			out.MoveTo(pen.X, pen.Y)
		}
		for j := 1; j < len(contour); j++ {
			a, b := contour[j-1].Scale(1/k), contour[j].Scale(1/k)
			seg := b.Sub(a)
			length := geom.Sqrt32(seg.X*seg.X + seg.Y*seg.Y)
			at := float32(0)
			for length-at > left {
				at += left
				q := a.Add(seg.Scale(at / length))
				if on {
					out.LineTo(q.X, q.Y)
				} else {
					lastMove = len(out.verbs)
					out.MoveTo(q.X, q.Y)
				}
				on = !on
				i = (i + 1) % len(dashes)
				left = dashes[i] * scale
				count++
				if count > svgMaxDashes {
					return p
				}
			}
			// A dash that would start right at the end of an open
			// contour is not drawn, as in Chrome: "0 5" on a length of 10
			// has dots at 0 and 5 only.
			left -= length - at
			if on {
				out.LineTo(b.X, b.Y)
			}
		}

		if flat.Closed(ci) && on && startedAtStart && firstOn && lastMove > firstMove {
			// The last dash runs into the first: move it to the front so
			// that the two meet with a join instead of two caps.
			out = joinWrapped(out, firstMove, lastMove)
		}
	}
	return out
}

// joinWrapped moves the subpath of out that starts at verb last in front of
// the one that starts at verb first, joining the two.
func joinWrapped(out *Path, first, last int) *Path {
	// Every subpath of a dash is a move and lines, so verbs and points line
	// up one to one.
	tail := out.points[last:]
	head := out.points[first:last]
	res := &Path{}
	res.verbs = append(res.verbs, out.verbs[:first]...)
	res.points = append(res.points, out.points[:first]...)
	res.MoveTo(tail[0].X, tail[0].Y)
	for _, q := range tail[1:] {
		res.LineTo(q.X, q.Y)
	}
	// The head starts with its move, which lies where the tail ended.
	for i := 1; i < len(head); i++ {
		if out.verbs[first+i] == verbMove {
			res.MoveTo(head[i].X, head[i].Y)
			continue
		}
		res.LineTo(head[i].X, head[i].Y)
	}
	return res
}
