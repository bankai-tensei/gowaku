package paint

import (
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// svgGradient is a gradient that fills a shape, in the coordinates of the
// drawing. Its MoreStops holds every stop, with the opacity of the shape
// applied.
type svgGradient struct {
	g Gradient
}

// svgGradientAttrs are the attributes of a gradient, after following href.
type svgGradientAttrs struct {
	radial bool
	// set holds the geometry attributes found, by name.
	set       map[string]string
	userSpace bool
	transform geom.Affine
	spread    SpreadMode
	stops     []*svgNode
}

// svgGradientDepth bounds a chain of gradients that refer to each other.
const svgGradientDepth = 16

// server resolves paint pt, which refers to a paint server, for a shape whose
// geometry has bounds bbox in its own coordinates. It returns the gradient and
// svgPaintServer, a single color and svgPaintColor or svgPaintCurrent, or
// svgPaintNone for nothing to draw.
func (p *svgDocParser) server(st *svgState, pt svgPaint, bbox geom.Rect, opacity float32) (svgGradient, geom.Color, svgPaintKind) {
	n := p.tree.lookup(pt.ref)
	if n == nil || (n.name != "linearGradient" && n.name != "radialGradient") {
		if n != nil && n.name == "pattern" {
			// Patterns are not drawn. See docs/ADR/0112-svg-structure-paint-clip.md.
			p.svg.Skipped++
		}
		return svgGradient{}, pt.fallbackColor, pt.fallback
	}
	a := p.gradientAttrs(n)
	stops := p.gradientStops(st, a.stops)
	switch len(stops) {
	case 0:
		return svgGradient{}, geom.Color{}, svgPaintNone
	case 1:
		return svgGradient{}, stops[0].Color, svgPaintColor
	}
	vp := st.viewport
	unit := func(name, def string, ref float32) float32 {
		v, ok := a.set[name]
		if !ok {
			v = def
		}
		if !a.userSpace {
			ref = 1
		}
		x, err := parseLength(v, ref)
		if err != nil {
			// A value that does not parse is ignored, as in browsers. The
			// defaults are percentages, which always parse.
			x, err = parseLength(def, ref)
			_ = err
		}
		return x
	}
	space := geom.Identity()
	if !a.userSpace {
		if bbox.W <= 0 || bbox.H <= 0 {
			// A box without area has no units to measure in, and the
			// element is not painted.
			return svgGradient{}, geom.Color{}, svgPaintNone
		}
		space = geom.Affine{A: bbox.W, D: bbox.H, E: bbox.X, F: bbox.Y}
	}
	g := Gradient{Spread: a.spread, Transform: st.ctm.Mul(space).Mul(a.transform)}
	if a.radial {
		cx, cy := unit("cx", "50%", vp.W), unit("cy", "50%", vp.H)
		r := unit("r", "50%", diagonal(vp))
		if r <= 0 {
			// A gradient of no size paints its last color.
			return svgGradient{}, stops[len(stops)-1].Color, svgPaintColor
		}
		g.Kind = GradientRadial
		g.Start, g.End = geom.Offset{X: cx, Y: cy}, geom.Offset{X: cx + r, Y: cy}
		fx, fy := cx, cy
		if _, ok := a.set["fx"]; ok {
			fx = unit("fx", "50%", vp.W)
		}
		if _, ok := a.set["fy"]; ok {
			fy = unit("fy", "50%", vp.H)
		}
		if fx != cx || fy != cy {
			g.Kind, g.Focus = GradientFocal, geom.Offset{X: fx, Y: fy}
		}
	} else {
		g.Kind = GradientLinear
		g.Start = geom.Offset{X: unit("x1", "0%", vp.W), Y: unit("y1", "0%", vp.H)}
		g.End = geom.Offset{X: unit("x2", "100%", vp.W), Y: unit("y2", "0%", vp.H)}
		if g.Start == g.End {
			return svgGradient{}, stops[len(stops)-1].Color, svgPaintColor
		}
	}
	if g.Transform.Det() == 0 {
		return svgGradient{}, geom.Color{}, svgPaintNone
	}
	for i := range stops {
		stops[i].Color.A *= opacity
	}
	stops = straightStops(stops)
	g.MoreStops = &stops
	return svgGradient{g: g}, geom.Color{}, svgPaintServer
}

// gradientAttrs follows the href chain of gradient n. An attribute left out
// takes the value of the first gradient down the chain that has it, and the
// stops come from the first one that has any.
func (p *svgDocParser) gradientAttrs(n *svgNode) svgGradientAttrs {
	a := svgGradientAttrs{radial: n.name == "radialGradient", set: make(map[string]string), transform: geom.Identity()}
	geometry := []string{"x1", "y1", "x2", "y2"}
	if a.radial {
		geometry = []string{"cx", "cy", "r", "fx", "fy"}
	}
	var units, transform, spread string
	var hasUnits, hasTransform, hasSpread, hasStops bool
	seen := make(map[*svgNode]bool)
	for cur, depth := n, 0; cur != nil && depth < svgGradientDepth && !seen[cur]; depth++ {
		seen[cur] = true
		for _, name := range geometry {
			if _, ok := a.set[name]; !ok && cur.name == n.name && cur.has(name) {
				a.set[name] = cur.attr(name)
			}
		}
		if !hasUnits && cur.has("gradientUnits") {
			units, hasUnits = cur.attr("gradientUnits"), true
		}
		if !hasTransform && cur.has("gradientTransform") {
			transform, hasTransform = cur.attr("gradientTransform"), true
		}
		if !hasSpread && cur.has("spreadMethod") {
			spread, hasSpread = cur.attr("spreadMethod"), true
		}
		if !hasStops {
			for _, c := range cur.children {
				if c.name == "stop" && !c.foreign {
					a.stops = append(a.stops, c)
				}
			}
			hasStops = len(a.stops) > 0
		}
		next := p.tree.lookup(cur.href())
		if next == nil || (next.name != "linearGradient" && next.name != "radialGradient") {
			break
		}
		cur = next
	}
	a.userSpace = strings.TrimSpace(units) == "userSpaceOnUse"
	if m, err := parseTransform(transform); err == nil {
		a.transform = m
	}
	switch strings.TrimSpace(spread) {
	case "reflect":
		a.spread = SpreadReflect
	case "repeat":
		a.spread = SpreadRepeat
	}
	return a
}

// gradientStops reads the stops, with offsets clamped to 0 to 1 that never
// decrease. currentColor in a stop takes the
// color property of the stop or its ancestors, or else black.
func (p *svgDocParser) gradientStops(st *svgState, nodes []*svgNode) []GradientStop {
	stops := make([]GradientStop, 0, len(nodes))
	last := float32(0)
	for _, n := range nodes {
		off := float32(0)
		if v := strings.TrimSpace(n.attr("offset")); v != "" {
			if strings.HasSuffix(v, "%") {
				if x, err := parseNumber(v[:len(v)-1]); err == nil {
					off = x / 100
				}
			} else if x, err := parseNumber(v); err == nil {
				off = x
			}
		}
		off = max(geom.Clamp32(off, 0, 1), last)
		last = off
		c := geom.Color{A: 1}
		if v, ok := n.prop("stop-color"); ok {
			if strings.EqualFold(v, "currentColor") {
				c = stopCurrentColor(n, st)
			} else if parsed, ok := parseColor(v); ok {
				c = parsed
			}
		}
		if v, ok := n.prop("stop-opacity"); ok {
			if a, err := parseOpacity(v); err == nil {
				c.A *= a
			}
		}
		stops = append(stops, GradientStop{Offset: off, Color: c})
	}
	return stops
}

// stopCurrentColor is the color property in effect at stop n: set on it or an
// ancestor, or else on the element that the gradient paints.
func stopCurrentColor(n *svgNode, st *svgState) geom.Color {
	for m := n; m != nil; m = m.parent {
		if v, ok := m.prop("color"); ok {
			if c, ok := parseColor(v); ok {
				return c
			}
		}
	}
	if st.hasColor {
		return st.color
	}
	return geom.Color{A: 1}
}

// svgAlphaSteps is how many stops straightStops puts between two stops whose
// alpha differs.
const svgAlphaSteps = 8

// straightStops returns stops with more stops between those whose alpha and
// color both differ. Browsers blend the colors of SVG gradients before
// applying alpha, where paint blends premultiplied colors; the extra stops,
// blended straight, bring the two close.
func straightStops(stops []GradientStop) []GradientStop {
	need := false
	for i := 1; i < len(stops); i++ {
		a, b := stops[i-1].Color, stops[i].Color
		if a.A != b.A && (a.R != b.R || a.G != b.G || a.B != b.B) && stops[i].Offset > stops[i-1].Offset {
			need = true
		}
	}
	if !need {
		return stops
	}
	out := make([]GradientStop, 0, len(stops)*svgAlphaSteps)
	out = append(out, stops[0])
	for i := 1; i < len(stops); i++ {
		a, b := stops[i-1], stops[i]
		if a.Color.A != b.Color.A && b.Offset > a.Offset &&
			(a.Color.R != b.Color.R || a.Color.G != b.Color.G || a.Color.B != b.Color.B) {
			for k := 1; k < svgAlphaSteps; k++ {
				t := float32(k) / svgAlphaSteps
				out = append(out, GradientStop{
					Offset: a.Offset + (b.Offset-a.Offset)*t,
					Color: geom.Color{
						R: a.Color.R + (b.Color.R-a.Color.R)*t,
						G: a.Color.G + (b.Color.G-a.Color.G)*t,
						B: a.Color.B + (b.Color.B-a.Color.B)*t,
						A: a.Color.A + (b.Color.A-a.Color.A)*t,
					},
				})
			}
		}
		out = append(out, b)
	}
	return out
}
