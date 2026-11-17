package paint

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// A filter element resolves into the primitives below, in user space, which
// svgfilterpaint.go turns into a Filter of the canvas that the GPU runs. See
// docs/ADR/0113-svg-filters-text.md and
// docs/ADR/0114-gpu-layers-masks-filters.md.

type svgFeKind uint8

const (
	feBlur svgFeKind = iota
	feOffset
	feDropShadow
	feFlood
	feColorMatrix
	feComposite
	feMerge
	feBlend
	feComponentTransfer
	feMorphology
)

// The inputs of a primitive that are not results of another one.
const (
	feSourceGraphic = -1
	feSourceAlpha   = -2
	// feNothing is an input that the filter cannot draw, such as
	// BackgroundImage, which is transparent.
	feNothing = -3
)

// svgFilter is a resolved filter element.
type svgFilter struct {
	// region is the filter region in user space, and ctm takes user space to
	// the drawing.
	region geom.Rect
	ctm    geom.Affine
	prims  []svgPrimitive
	// out is the filter of the canvas, which build makes.
	out Filter
}

// svgPrimitive is one filter primitive. Lengths are in user space.
type svgPrimitive struct {
	kind svgFeKind
	// in and in2 are inputs: a result index, or one of the fe constants.
	in, in2 int
	// merge holds the inputs of feMerge.
	merge []int
	// linear means that it works in linearRGB.
	linear bool
	// sub is the primitive subregion in user space, and hasSub whether one
	// is set at all.
	sub    geom.Rect
	hasSub bool

	stdDev [2]float32
	dx, dy float32
	color  geom.Color
	matrix [20]float32
	// op is the operator of feComposite, the mode of feBlend, or 1 for the
	// dilate of feMorphology.
	op     string
	k      [4]float32
	radius [2]float32
	// funcs are the transfer functions of feComponentTransfer, for R, G, B
	// and A.
	funcs [4]svgTransfer
}

// svgTransfer is one transfer function of feComponentTransfer.
type svgTransfer struct {
	kind                            string // identity, table, discrete, linear or gamma
	table                           []float32
	slope, intercept, amp, exp, off float32
}

// filterOf resolves the filter that ref names for an element with state st
// and bounding box bbox. A reference to nothing filters nothing, and a filter
// that uses what cannot be drawn is left out and counted in Skipped.
func (p *svgDocParser) filterOf(ref string, st *svgState, bbox geom.Rect) (*svgFilter, error) {
	n := p.tree.lookup(ref)
	if n == nil || n.name != "filter" {
		if !strings.HasPrefix(strings.TrimSpace(ref), "url(") {
			// Filter functions of CSS, such as blur(), are not read.
			p.svg.Skipped++
		}
		return nil, nil
	}
	// A filter can take its attributes and primitives from another through
	// href, as gradients do.
	src := n
	for depth, cur := 0, n; cur != nil && depth < svgGradientDepth; depth++ {
		if hasPrimitives(cur) {
			src = cur
			break
		}
		next := p.tree.lookup(cur.href())
		if next == nil || next.name != "filter" || next == n {
			break
		}
		cur = next
	}
	boxUnits := strings.TrimSpace(n.attr("filterUnits")) != "userSpaceOnUse"
	primBox := strings.TrimSpace(n.attr("primitiveUnits")) == "objectBoundingBox"
	if (boxUnits || primBox) && (bbox.W <= 0 || bbox.H <= 0) {
		// No box to measure the region in: nothing is drawn.
		f := &svgFilter{ctm: st.ctm}
		f.build()
		return f, nil
	}
	f := &svgFilter{ctm: st.ctm}
	region, err := boxRect(n, [4]string{"-10%", "-10%", "120%", "120%"}, boxUnits, bbox, st.viewport)
	if err != nil {
		return nil, fmt.Errorf("<filter>: %w", err)
	}
	f.region = region
	inner, err := p.stateAt(src)
	if err != nil {
		return nil, err
	}
	results := map[string]int{}
	// The scale of lengths in primitive units.
	ux, uy := float32(1), float32(1)
	if primBox {
		ux, uy = bbox.W, bbox.H
	}
	for _, c := range src.children {
		if c.name == "" || c.foreign {
			continue
		}
		cst, err := p.style(&inner, c)
		if err != nil {
			return nil, fmt.Errorf("<%s>: %w", c.name, err)
		}
		prim, ok, err := p.primitive(c, &cst, results, len(f.prims), primBox, bbox, ux, uy)
		if err != nil {
			return nil, fmt.Errorf("<%s>: %w", c.name, err)
		}
		if !ok {
			// A primitive that cannot be drawn: the element draws
			// unfiltered.
			p.svg.Skipped++
			return nil, nil
		}
		if r := strings.TrimSpace(c.attr("result")); r != "" {
			results[r] = len(f.prims)
		}
		f.prims = append(f.prims, prim)
	}
	f.build()
	return f, nil
}

func hasPrimitives(n *svgNode) bool {
	return slices.ContainsFunc(n.children, func(c *svgNode) bool { return strings.HasPrefix(c.name, "fe") && !c.foreign })
}

// boxRect reads x, y, width and height of n, with defaults def, as fractions of
// bbox or as lengths in user space.
func boxRect(n *svgNode, def [4]string, boxUnits bool, bbox geom.Rect, vp geom.Size) (geom.Rect, error) {
	refs := [4]float32{vp.W, vp.H, vp.W, vp.H}
	var r [4]float32
	for i, name := range [4]string{"x", "y", "width", "height"} {
		v := n.attr(name)
		if strings.TrimSpace(v) == "" {
			v = def[i]
		}
		ref := refs[i]
		if boxUnits {
			ref = 1
		}
		x, err := parseLength(v, ref)
		if err != nil {
			return geom.Rect{}, err
		}
		r[i] = x
	}
	if boxUnits {
		return geom.Rect{X: bbox.X + r[0]*bbox.W, Y: bbox.Y + r[1]*bbox.H, W: r[2] * bbox.W, H: r[3] * bbox.H}, nil
	}
	return geom.Rect{X: r[0], Y: r[1], W: r[2], H: r[3]}, nil
}

// input reads an in or in2 attribute.
func input(v string, results map[string]int, index int) int {
	switch v = strings.TrimSpace(v); v {
	case "":
		if index == 0 {
			return feSourceGraphic
		}
		return index - 1
	case "SourceGraphic":
		return feSourceGraphic
	case "SourceAlpha":
		return feSourceAlpha
	case "BackgroundImage", "BackgroundAlpha", "FillPaint", "StrokePaint":
		return feNothing
	}
	if r, ok := results[v]; ok {
		return r
	}
	// A name that no result has refers to the previous result.
	if index == 0 {
		return feSourceGraphic
	}
	return index - 1
}

// primitive reads filter primitive c. It reports false for a primitive that
// cannot be drawn.
func (p *svgDocParser) primitive(c *svgNode, st *svgState, results map[string]int, index int, primBox bool,
	bbox geom.Rect, ux, uy float32) (svgPrimitive, bool, error) {
	pr := svgPrimitive{in: input(c.attr("in"), results, index), in2: input(c.attr("in2"), results, index), linear: true}
	if v, ok := c.prop("color-interpolation-filters"); ok && v == "sRGB" {
		pr.linear = false
	}
	if c.has("x") || c.has("y") || c.has("width") || c.has("height") {
		sub, err := boxRect(c, [4]string{"0%", "0%", "100%", "100%"}, primBox, bbox, st.viewport)
		if err != nil {
			return pr, false, err
		}
		pr.sub, pr.hasSub = sub, true
		// Attributes left out cover everything along their axis.
		const huge = 1 << 24
		if !c.has("x") {
			pr.sub.X, pr.sub.W = -huge, 2*huge
		}
		if !c.has("y") {
			pr.sub.Y, pr.sub.H = -huge, 2*huge
		}
	}
	nums := func(name string) ([]float32, error) { return parseNumbers(c.attr(name)) }
	switch c.name {
	case "feGaussianBlur", "feDropShadow":
		pr.kind = feBlur
		v, err := nums("stdDeviation")
		if err != nil {
			return pr, false, err
		}
		switch len(v) {
		case 0:
			if c.name == "feDropShadow" {
				v = []float32{2}
			} else {
				v = []float32{0}
			}
			fallthrough
		case 1:
			v = append(v, v[0])
		}
		if v[0] < 0 || v[1] < 0 {
			v[0], v[1] = 0, 0
		}
		pr.stdDev = [2]float32{v[0] * ux, v[1] * uy}
		if c.name == "feDropShadow" {
			pr.kind = feDropShadow
			pr.dx, pr.dy = 2*ux, 2*uy
			if err := offsetAttrs(c, &pr, ux, uy); err != nil {
				return pr, false, err
			}
			pr.color = floodColor(c, st)
		}
	case "feOffset":
		pr.kind = feOffset
		if err := offsetAttrs(c, &pr, ux, uy); err != nil {
			return pr, false, err
		}
	case "feFlood":
		pr.kind = feFlood
		pr.color = floodColor(c, st)
	case "feColorMatrix":
		pr.kind = feColorMatrix
		m, err := colorMatrix(c)
		if err != nil {
			return pr, false, err
		}
		pr.matrix = m
	case "feComposite":
		pr.kind = feComposite
		pr.op = strings.TrimSpace(c.attr("operator"))
		if pr.op == "" {
			pr.op = "over"
		}
		for i, name := range [4]string{"k1", "k2", "k3", "k4"} {
			if v := c.attr(name); v != "" {
				k, err := parseNumber(v)
				if err != nil {
					return pr, false, err
				}
				pr.k[i] = k
			}
		}
		switch pr.op {
		case "over", "in", "out", "atop", "xor", "arithmetic", "lighter":
		default:
			pr.op = "over"
		}
	case "feMerge":
		pr.kind = feMerge
		for _, m := range c.children {
			if m.name == "feMergeNode" {
				pr.merge = append(pr.merge, input(m.attr("in"), results, index))
			}
		}
	case "feBlend":
		pr.kind = feBlend
		pr.op = strings.TrimSpace(c.attr("mode"))
		if _, ok := svgBlendModes[pr.op]; !ok {
			pr.op = "normal"
		}
	case "feComponentTransfer":
		pr.kind = feComponentTransfer
		for i := range pr.funcs {
			pr.funcs[i].kind = "identity"
		}
		for _, f := range c.children {
			ch := strings.Index("RGBA", strings.TrimPrefix(f.name, "feFunc"))
			if !strings.HasPrefix(f.name, "feFunc") || len(f.name) != 7 || ch < 0 {
				continue
			}
			tf, err := transferFunc(f)
			if err != nil {
				return pr, false, err
			}
			pr.funcs[ch] = tf
		}
	case "feMorphology":
		pr.kind = feMorphology
		v, err := nums("radius")
		if err != nil {
			return pr, false, err
		}
		if len(v) == 0 {
			v = []float32{0}
		}
		if len(v) == 1 {
			v = append(v, v[0])
		}
		pr.radius = [2]float32{max(v[0], 0) * ux, max(v[1], 0) * uy}
		if strings.TrimSpace(c.attr("operator")) == "dilate" {
			pr.op = "dilate"
		}
	default:
		return pr, false, nil
	}
	return pr, true, nil
}

func offsetAttrs(c *svgNode, pr *svgPrimitive, ux, uy float32) error {
	if v := c.attr("dx"); v != "" {
		d, err := parseNumber(v)
		if err != nil {
			return err
		}
		pr.dx = d * ux
	}
	if v := c.attr("dy"); v != "" {
		d, err := parseNumber(v)
		if err != nil {
			return err
		}
		pr.dy = d * uy
	}
	return nil
}

// floodColor reads flood-color and flood-opacity, black by default.
func floodColor(c *svgNode, st *svgState) geom.Color {
	col := geom.Color{A: 1}
	if v, ok := c.prop("flood-color"); ok {
		if strings.EqualFold(v, "currentColor") {
			if st.hasColor {
				col = st.color
			}
		} else if parsed, ok := parseColor(v); ok {
			col = parsed
		}
	}
	if v, ok := c.prop("flood-opacity"); ok {
		if a, err := parseOpacity(v); err == nil {
			col.A *= a
		}
	}
	return col
}

// colorMatrix reads the type and values of feColorMatrix as a 4 by 5 matrix.
func colorMatrix(c *svgNode) ([20]float32, error) {
	identity := [20]float32{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0}
	v, err := parseNumbers(c.attr("values"))
	if err != nil {
		return identity, err
	}
	switch strings.TrimSpace(c.attr("type")) {
	case "saturate":
		s := float32(1)
		if len(v) > 0 {
			s = v[0]
		}
		return [20]float32{
			0.213 + 0.787*s, 0.715 - 0.715*s, 0.072 - 0.072*s, 0, 0,
			0.213 - 0.213*s, 0.715 + 0.285*s, 0.072 - 0.072*s, 0, 0,
			0.213 - 0.213*s, 0.715 - 0.715*s, 0.072 + 0.928*s, 0, 0,
			0, 0, 0, 1, 0,
		}, nil
	case "hueRotate":
		a := 0.0
		if len(v) > 0 {
			a = float64(v[0]) * math.Pi / 180
		}
		cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
		return [20]float32{
			0.213 + cs*0.787 - sn*0.213, 0.715 - cs*0.715 - sn*0.715, 0.072 - cs*0.072 + sn*0.928, 0, 0,
			0.213 - cs*0.213 + sn*0.143, 0.715 + cs*0.285 + sn*0.140, 0.072 - cs*0.072 - sn*0.283, 0, 0,
			0.213 - cs*0.213 - sn*0.787, 0.715 - cs*0.715 + sn*0.715, 0.072 + cs*0.928 + sn*0.072, 0, 0,
			0, 0, 0, 1, 0,
		}, nil
	case "luminanceToAlpha":
		return [20]float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.2125, 0.7154, 0.0721, 0, 0}, nil
	}
	if len(v) != 20 {
		return identity, nil
	}
	var m [20]float32
	copy(m[:], v)
	return m, nil
}

func transferFunc(f *svgNode) (svgTransfer, error) {
	tf := svgTransfer{kind: strings.TrimSpace(f.attr("type")), slope: 1, amp: 1, exp: 1}
	num := func(name string, dst *float32) error {
		if v := f.attr(name); v != "" {
			x, err := parseNumber(v)
			if err != nil {
				return err
			}
			*dst = x
		}
		return nil
	}
	for name, dst := range map[string]*float32{"slope": &tf.slope, "intercept": &tf.intercept, "amplitude": &tf.amp, "exponent": &tf.exp, "offset": &tf.off} {
		if err := num(name, dst); err != nil {
			return tf, err
		}
	}
	t, err := parseNumbers(f.attr("tableValues"))
	if err != nil {
		return tf, err
	}
	tf.table = t
	switch tf.kind {
	case "table", "discrete":
		if len(tf.table) == 0 {
			tf.kind = "identity"
		}
	case "linear", "gamma":
	default:
		tf.kind = "identity"
	}
	return tf, nil
}
