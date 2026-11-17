package paint

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// shapeVertices returns the vertices of a path, line, polyline or polygon,
// where markers go. Other shapes take no markers, as in browsers.
func shapeVertices(n *svgNode, path *Path) []svgVertex {
	switch n.name {
	case "path":
		v := svgVertices{start: -1}
		sp := svgParser{in: n.attr("d"), path: &Path{}, verts: &v}
		// The path parsed once already, so it parses again.
		if err := sp.run(); err != nil {
			return nil
		}
		return v.list
	case "line", "polyline", "polygon":
		var pts []geom.Offset
		for i, verb := range path.verbs {
			if verb == verbMove || verb == verbLine {
				pts = append(pts, path.points[i])
			}
		}
		return pointVertices(pts, n.name == "polygon")
	}
	return nil
}

// pathLength returns the length of p, flattened to within tol.
func pathLength(p *Path, tol float32) float32 {
	k := float32(flattenTolerance) / max(tol, 1e-6)
	flat := p.transformed(geom.Scaling(k, k))
	pts, starts := flat.Polygon()
	total := float32(0)
	for ci, start := range starts {
		end := len(pts)
		if ci+1 < len(starts) {
			end = starts[ci+1]
		}
		for i := start + 1; i < end; i++ {
			d := pts[i].Sub(pts[i-1])
			total += geom.Sqrt32(d.X*d.X + d.Y*d.Y)
		}
		if flat.Closed(ci) && end > start {
			d := pts[start].Sub(pts[end-1])
			total += geom.Sqrt32(d.X*d.X + d.Y*d.Y)
		}
	}
	return total / k
}

// markers draws the markers of a shape at verts.
func (p *svgDocParser) markers(st *svgState, verts []svgVertex) error {
	last := len(verts) - 1
	for i, v := range verts {
		var refs [2]string
		switch {
		case i == 0 && last == 0:
			refs = [2]string{st.markerStart, st.markerEnd}
		case i == 0:
			refs[0] = st.markerStart
		case i == last:
			refs[0] = st.markerEnd
		default:
			refs[0] = st.markerMid
		}
		for k, ref := range refs {
			if ref == "" {
				continue
			}
			kind := svgMarkerMid
			switch {
			case i == 0 && k == 0:
				kind = svgMarkerStart
			case i == last:
				kind = svgMarkerEnd
			}
			if err := p.marker(st, ref, v, kind); err != nil {
				return err
			}
		}
	}
	return nil
}

type svgMarkerKind uint8

const (
	svgMarkerStart svgMarkerKind = iota
	svgMarkerMid
	svgMarkerEnd
)

// marker draws the marker that ref names at vertex v of a shape with state st.
func (p *svgDocParser) marker(st *svgState, ref string, v svgVertex, kind svgMarkerKind) error {
	n := p.tree.lookup(ref)
	if n == nil || n.name != "marker" {
		return nil
	}
	if slices.Contains(p.active, n) {
		// A marker inside its own content would never end.
		p.svg.Skipped++
		return nil
	}
	if !p.visit() {
		return nil
	}
	vp := st.viewport
	w, err := lengthAttr(n, "markerWidth", vp.W, 3)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	h, err := lengthAttr(n, "markerHeight", vp.H, 3)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	if w <= 0 || h <= 0 {
		return nil
	}
	view, viewport, err := viewportTransform(n, 0, 0, w, h)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	refX, err := markerRefLength(n.attr("refX"), viewport.W)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	refY, err := markerRefLength(n.attr("refY"), viewport.H)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	angle, err := markerAngle(n.attr("orient"), v, kind)
	if err != nil {
		return fmt.Errorf("<marker>: %w", err)
	}
	scale := float32(1)
	if strings.TrimSpace(n.attr("markerUnits")) != "userSpaceOnUse" {
		scale = st.strokeWidth
	}
	anchor := view.Apply(geom.Offset{X: refX, Y: refY})
	m := st.ctm.Mul(geom.Translation(v.pos)).Mul(geom.Rotation(angle)).Mul(geom.Scaling(scale, scale)).
		Mul(geom.Translation(anchor.Scale(-1)))
	inner, err := p.stateAt(n)
	if err != nil {
		return err
	}
	inner.ctm = m.Mul(view)
	inner.viewport = viewport
	if v, _ := n.prop("display"); v == "none" {
		return nil
	}
	// The content is clipped to the viewport of the marker, unless overflow
	// says otherwise; see clipMarker.
	return p.clipMarker(n, m, geom.Rect{W: w, H: h}, &inner)
}

// markerRefLength reads refX or refY, which may also be left, center, right,
// top or bottom.
func markerRefLength(v string, ref float32) (float32, error) {
	switch strings.TrimSpace(v) {
	case "":
		return 0, nil
	case "left", "top":
		return 0, nil
	case "center":
		return ref / 2, nil
	case "right", "bottom":
		return ref, nil
	}
	return parseLength(v, ref)
}

// markerAngle returns the rotation of a marker at v, in radians, as orient
// says: auto follows the path, bisecting the turn at a vertex between two
// segments; auto-start-reverse also turns a start marker around.
func markerAngle(orient string, v svgVertex, kind svgMarkerKind) (float32, error) {
	orient = strings.TrimSpace(orient)
	switch orient {
	case "auto", "auto-start-reverse":
		a := vertexAngle(v)
		if orient == "auto-start-reverse" && kind == svgMarkerStart {
			a += math.Pi
		}
		return a, nil
	case "":
		return 0, nil
	}
	unit := float32(math.Pi / 180)
	for _, u := range [...]struct {
		suffix string
		scale  float32
	}{{"deg", math.Pi / 180}, {"grad", math.Pi / 200}, {"rad", 1}, {"turn", 2 * math.Pi}} {
		if s, ok := strings.CutSuffix(orient, u.suffix); ok {
			orient, unit = s, u.scale
			break
		}
	}
	a, err := parseNumber(orient)
	return a * unit, err
}

// vertexAngle is the direction of the path at v, in radians. A start takes
// the direction out of it and an end the one into it, unless the subpath is
// closed there; anywhere else it is the bisector of the two.
func vertexAngle(v svgVertex) float32 {
	angle := func(d geom.Offset) float64 { return math.Atan2(float64(d.Y), float64(d.X)) }
	hasIn, hasOut := v.in != (geom.Offset{}), v.out != (geom.Offset{})
	switch {
	case !hasIn && !hasOut:
		return 0
	case !hasIn:
		return float32(angle(v.out))
	case !hasOut:
		return float32(angle(v.in))
	}
	in, out := angle(v.in), angle(v.out)
	// Bisect the smaller turn, as browsers do.
	if in-out > math.Pi {
		in -= 2 * math.Pi
	} else if out-in > math.Pi {
		out -= 2 * math.Pi
	}
	return float32((in + out) / 2)
}

// stateAt returns the state that the content of n, which is drawn by
// reference, inherits: the properties of its ancestors and its own, in the
// coordinates of the drawing.
func (p *svgDocParser) stateAt(n *svgNode) (svgState, error) {
	var chain []*svgNode
	for a := n; a != nil; a = a.parent {
		chain = append(chain, a)
	}
	st := p.initial
	for i := len(chain) - 1; i >= 0; i-- {
		next, err := p.style(&st, chain[i])
		if err != nil {
			return st, fmt.Errorf("<%s>: %w", chain[i].name, err)
		}
		st = next
	}
	// Transforms and opacity do not pass down to content drawn elsewhere.
	st.ctm = geom.Identity()
	st.opacity = 1
	return st, nil
}
