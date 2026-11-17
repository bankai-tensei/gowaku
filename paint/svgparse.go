package paint

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// forEachDeclaration calls fn for each declaration of a style attribute.
func forEachDeclaration(style string, fn func(name, value string)) {
	for decl := range strings.SplitSeq(style, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
		fn(strings.ToLower(strings.TrimSpace(name)), value)
	}
}

// maxScale returns how much m stretches a length at most: its largest singular
// value.
func maxScale(m geom.Affine) float32 {
	a, b, c, d := float64(m.A), float64(m.B), float64(m.C), float64(m.D)
	s := (a*a + b*b + c*c + d*d) / 2
	det := a*d - b*c
	v := math.Sqrt(s + math.Sqrt(math.Max(s*s-det*det, 0)))
	if v == 0 {
		return 1
	}
	return float32(v)
}

// diagonal is what a percentage of a length that is neither horizontal nor
// vertical refers to.
func diagonal(vp geom.Size) float32 {
	return geom.Sqrt32((vp.W*vp.W + vp.H*vp.H) / 2)
}

// lengthAttr reads a length attribute, or returns def if it is absent. A
// property in the style attribute does not set geometry, as in SVG 1.1.
func lengthAttr(n *svgNode, name string, ref, def float32) (float32, error) {
	v := n.attr(name)
	if strings.TrimSpace(v) == "" {
		return def, nil
	}
	return parseLength(v, ref)
}

// parseLength reads a length with an optional unit. Percentages are of ref.
func parseLength(s string, ref float32) (float32, error) {
	s = strings.TrimSpace(s)
	unit := float32(1)
	for _, u := range [...]struct {
		suffix string
		scale  float32
	}{{"px", 1}, {"pt", 4.0 / 3}, {"pc", 16}, {"mm", 96 / 25.4}, {"cm", 96 / 2.54}, {"in", 96}, {"em", 16}, {"ex", 8}, {"%", -1}} {
		if strings.HasSuffix(s, u.suffix) {
			s = strings.TrimSpace(s[:len(s)-len(u.suffix)])
			unit = u.scale
			if u.scale < 0 {
				unit = ref / 100
			}
			break
		}
	}
	v, err := parseNumber(s)
	return v * unit, err
}

func parseNumber(s string) (float32, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		// NaN and infinities would poison every computation after them.
		return 0, fmt.Errorf("bad number %q", s)
	}
	return float32(v), nil
}

func parseOpacity(s string) (float32, error) {
	scale := float32(1)
	if strings.HasSuffix(s, "%") {
		s, scale = s[:len(s)-1], 100
	}
	v, err := parseNumber(s)
	return geom.Clamp32(v/scale, 0, 1), err
}

// parseNumbers reads a list of numbers apart by spaces or commas, where a sign
// or a second dot may also start the next number, as in path data.
func parseNumbers(s string) ([]float32, error) {
	sp := svgParser{in: s}
	var out []float32
	for {
		sp.skipSpace()
		if sp.at == len(sp.in) {
			return out, nil
		}
		v, err := sp.number()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func parseViewBox(s string) (geom.Rect, bool, error) {
	if strings.TrimSpace(s) == "" {
		return geom.Rect{}, false, nil
	}
	v, err := parseNumbers(s)
	if err != nil {
		return geom.Rect{}, false, err
	}
	if len(v) != 4 || v[2] <= 0 || v[3] <= 0 {
		return geom.Rect{}, false, fmt.Errorf("bad viewBox %q", s)
	}
	return geom.Rect{X: v[0], Y: v[1], W: v[2], H: v[3]}, true, nil
}

// parseAlign reads preserveAspectRatio. The default is xMidYMid meet.
func parseAlign(s string) (svgAlign, error) {
	al := svgAlign{x: 0.5, y: 0.5}
	f := strings.Fields(s)
	if len(f) > 0 && f[0] == "defer" {
		f = f[1:]
	}
	if len(f) == 0 {
		return al, nil
	}
	if f[0] == "none" {
		al.none = true
	} else {
		if len(f[0]) != 8 || f[0][0] != 'x' || f[0][4] != 'Y' {
			return al, fmt.Errorf("bad preserveAspectRatio %q", s)
		}
		pos := func(v string) (float32, bool) {
			switch v {
			case "Min":
				return 0, true
			case "Mid":
				return 0.5, true
			case "Max":
				return 1, true
			}
			return 0, false
		}
		var okX, okY bool
		al.x, okX = pos(f[0][1:4])
		al.y, okY = pos(f[0][5:8])
		if !okX || !okY {
			return al, fmt.Errorf("bad preserveAspectRatio %q", s)
		}
	}
	if len(f) > 1 {
		switch f[1] {
		case "meet":
		case "slice":
			al.slice = true
		default:
			return al, fmt.Errorf("bad preserveAspectRatio %q", s)
		}
	}
	return al, nil
}

// parseTransform reads a transform list. The functions apply from right to
// left, so the result is their product from left to right.
func parseTransform(s string) (geom.Affine, error) {
	m := geom.Identity()
	rest := s
	for {
		rest = strings.TrimLeft(rest, " \t\r\n,")
		if rest == "" {
			return m, nil
		}
		open := strings.IndexByte(rest, '(')
		end := strings.IndexByte(rest, ')')
		if open < 0 || end < open {
			return m, fmt.Errorf("bad transform %q", s)
		}
		name := strings.TrimSpace(rest[:open])
		args, err := parseNumbers(rest[open+1 : end])
		if err != nil {
			return m, fmt.Errorf("bad transform %q: %w", s, err)
		}
		f, ok := transformFunction(name, args)
		if !ok {
			return m, fmt.Errorf("bad transform %q", s)
		}
		m = m.Mul(f)
		rest = rest[end+1:]
	}
}

func transformFunction(name string, a []float32) (geom.Affine, bool) {
	deg := func(v float32) float32 { return v * math.Pi / 180 }
	switch {
	case name == "matrix" && len(a) == 6:
		return geom.Affine{A: a[0], B: a[1], C: a[2], D: a[3], E: a[4], F: a[5]}, true
	case name == "translate" && len(a) == 1:
		return geom.Translation(geom.Offset{X: a[0]}), true
	case name == "translate" && len(a) == 2:
		return geom.Translation(geom.Offset{X: a[0], Y: a[1]}), true
	case name == "scale" && len(a) == 1:
		return geom.Scaling(a[0], a[0]), true
	case name == "scale" && len(a) == 2:
		return geom.Scaling(a[0], a[1]), true
	case name == "rotate" && len(a) == 1:
		return geom.Rotation(deg(a[0])), true
	case name == "rotate" && len(a) == 3:
		c := geom.Offset{X: a[1], Y: a[2]}
		return geom.Translation(c).Mul(geom.Rotation(deg(a[0]))).Mul(geom.Translation(c.Scale(-1))), true
	case name == "skewX" && len(a) == 1:
		return geom.Affine{A: 1, C: geom.Tan32(deg(a[0])), D: 1}, true
	case name == "skewY" && len(a) == 1:
		return geom.Affine{A: 1, B: geom.Tan32(deg(a[0])), D: 1}, true
	}
	return geom.Affine{}, false
}
