package paint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// svgVisitBudget bounds how many elements a document may expand to through
// use. A chain of uses that each repeat the one before grows exponentially;
// past the budget the rest is left out and counted once in Skipped.
const svgVisitBudget = 1 << 17

type svgDocParser struct {
	svg  *SVG
	tree svgTree
	// tol is the tolerance for the outlines of strokes, in the units of the
	// view box.
	tol float32
	// active holds the elements being drawn, from the root down, through the
	// elements that use refers to, so that a reference loop is caught.
	active []*svgNode
	// budget counts the elements left to visit.
	budget    int
	exhausted bool
	// bounded means that the bounds of the drawing hold a shape.
	bounded bool
	// initial is the state at the root, before its own properties.
	initial svgState
	// ops is where shapes and groups go: the list of the drawing, or that of
	// a mask being read.
	ops *[]svgOp
	// frames are the groups open, innermost last.
	frames []svgFrame
	// geoms holds the geometry of every shape drawn so far, for the
	// bounding boxes that clip paths, masks and filters can refer to.
	geoms []svgGeom
	// outliner sets text, or is nil to leave text out.
	outliner TextOutliner
}

func (p *svgDocParser) run(data []byte) error {
	tree, skipped, err := readSVGTree(data)
	if err != nil {
		return err
	}
	p.tree = tree
	p.svg.Skipped += skipped
	p.budget = svgVisitBudget
	p.ops = &p.svg.ops
	cascade(tree.root, styleSheets(tree.root))
	if err := p.rootSVG(tree.root); err != nil {
		return err
	}
	p.svg.finish()
	return nil
}

// drawnLater lists the elements that draw only when referenced, or never.
func drawnLater(name string) bool {
	switch name {
	case "title", "desc", "metadata", "defs", "style", "symbol", "linearGradient", "radialGradient",
		"clipPath", "mask", "marker", "pattern", "filter":
		return true
	}
	return false
}

// visit takes one element from the budget, and reports false once it is spent.
func (p *svgDocParser) visit() bool {
	if p.budget == 0 {
		if !p.exhausted {
			p.svg.Skipped++
			p.exhausted = true
		}
		return false
	}
	p.budget--
	return true
}

// walk draws element n below the state parent, with its children.
func (p *svgDocParser) walk(n *svgNode, parent *svgState) error {
	if n.name == "" || n.foreign || drawnLater(n.name) {
		// Character data, elements of other vocabularies such as editor
		// metadata, and definitions draw nothing where they stand.
		return nil
	}
	if !p.visit() {
		return nil
	}
	switch n.name {
	case "g", "a", "switch", "svg", "use", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "text":
	default:
		p.svg.Skipped++
		return nil
	}
	st, err := p.style(parent, n)
	if err != nil {
		return fmt.Errorf("<%s>: %w", n.name, err)
	}
	if v, _ := n.prop("display"); v == "none" {
		return nil
	}
	if st.opacity <= 0 {
		return nil
	}
	g := p.beginGroup(n, &st)
	if err := p.content(n, &st); err != nil {
		return err
	}
	return p.endGroup(g, n, &st)
}

// content draws what element n holds, with state st.
func (p *svgDocParser) content(n *svgNode, st *svgState) error {
	switch n.name {
	case "use":
		return p.use(n, st)
	case "text":
		return p.textElement(n, st)
	case "svg":
		inner := *st
		if err := p.nestedSVG(&inner, n, 0, 0); err != nil {
			return fmt.Errorf("<svg>: %w", err)
		}
		return p.children(n, &inner)
	case "g", "a":
		return p.children(n, st)
	case "switch":
		// The first child that a browser would pick: conditions such as
		// systemLanguage are not evaluated, so that is the first element.
		for _, c := range n.children {
			if c.name != "" && !c.foreign {
				p.active = append(p.active, n)
				err := p.walk(c, st)
				p.active = p.active[:len(p.active)-1]
				return err
			}
		}
		return nil
	}
	path, open, err := p.shape(st, n)
	if err != nil {
		return fmt.Errorf("<%s>: %w", n.name, err)
	}
	if path == nil {
		return nil
	}
	var verts []svgVertex
	if st.markerStart != "" || st.markerMid != "" || st.markerEnd != "" {
		verts = shapeVertices(n, path)
	}
	if n.name == "path" || n.name == "polygon" || n.name == "polyline" || n.name == "line" {
		if v := n.attr("pathLength"); v != "" {
			if l, err := parseNumber(v); err == nil && l > 0 {
				st.pathLength = l
			}
		}
	}
	return p.emit(st, path, open, verts)
}

// children walks the children of n below st.
func (p *svgDocParser) children(n *svgNode, st *svgState) error {
	p.active = append(p.active, n)
	defer func() { p.active = p.active[:len(p.active)-1] }()
	for _, c := range n.children {
		if err := p.walk(c, st); err != nil {
			return err
		}
	}
	return nil
}

// use draws the element that n refers to, moved by its x and y. A symbol, or
// an svg, takes the width and height of n as its viewport.
func (p *svgDocParser) use(n *svgNode, st *svgState) error {
	ref := strings.TrimSpace(n.href())
	target := p.tree.lookup(ref)
	if target == nil {
		if ref != "" && !strings.HasPrefix(ref, "#") {
			// A reference to another document is not followed.
			p.svg.Skipped++
		}
		return nil
	}
	if slices.Contains(p.active, target) || target == n {
		// A use that ends up inside what it draws would never end.
		p.svg.Skipped++
		return nil
	}
	vp := st.viewport
	x, err := lengthAttr(n, "x", vp.W, 0)
	if err != nil {
		return fmt.Errorf("<use>: %w", err)
	}
	y, err := lengthAttr(n, "y", vp.H, 0)
	if err != nil {
		return fmt.Errorf("<use>: %w", err)
	}
	w, err := lengthAttr(n, "width", vp.W, 0)
	if err != nil {
		return fmt.Errorf("<use>: %w", err)
	}
	h, err := lengthAttr(n, "height", vp.H, 0)
	if err != nil {
		return fmt.Errorf("<use>: %w", err)
	}
	inner := *st
	inner.ctm = inner.ctm.Mul(geom.Translation(geom.Offset{X: x, Y: y}))
	p.active = append(p.active, n)
	defer func() { p.active = p.active[:len(p.active)-1] }()
	if target.name != "symbol" && target.name != "svg" {
		return p.walk(target, &inner)
	}
	if !p.visit() {
		return nil
	}
	sst, err := p.style(&inner, target)
	if err != nil {
		return fmt.Errorf("<%s>: %w", target.name, err)
	}
	if v, _ := target.prop("display"); v == "none" && target.name == "svg" {
		return nil
	}
	if target.name == "svg" {
		if err := p.nestedSVG(&sst, target, w, h); err != nil {
			return fmt.Errorf("<svg>: %w", err)
		}
		return p.children(target, &sst)
	}
	// Without a size on the use, a symbol takes its own, or else 100%.
	if w <= 0 {
		if w, err = lengthAttr(target, "width", vp.W, vp.W); err != nil {
			return fmt.Errorf("<symbol>: %w", err)
		}
	}
	if h <= 0 {
		if h, err = lengthAttr(target, "height", vp.H, vp.H); err != nil {
			return fmt.Errorf("<symbol>: %w", err)
		}
	}
	m, viewport, err := viewportTransform(target, 0, 0, w, h)
	if err != nil {
		return fmt.Errorf("<symbol>: %w", err)
	}
	sst.ctm = sst.ctm.Mul(m)
	sst.viewport = viewport
	return p.children(target, &sst)
}

func (p *svgDocParser) rootSVG(n *svgNode) error {
	s := p.svg
	initial := svgState{
		fill:          svgPaint{kind: svgPaintColor, color: geom.Color{A: 1}},
		fillOpacity:   1,
		strokeOpacity: 1,
		strokeWidth:   1,
		miterLimit:    defaultMiterLimit,
		visible:       true,
		opacity:       1,
		order:         [3]uint8{svgPaintFill, svgPaintStroke, svgPaintMarkers},
		text:          svgTextState{size: 16, weight: 400},
		ctm:           geom.Identity(),
	}
	var w, h float32
	var hasW, hasH bool
	var err error
	if v := n.attr("width"); v != "" && v != "auto" && !strings.HasSuffix(v, "%") {
		if w, err = parseLength(v, 0); err != nil {
			return err
		}
		hasW = true
	}
	if v := n.attr("height"); v != "" && v != "auto" && !strings.HasSuffix(v, "%") {
		if h, err = parseLength(v, 0); err != nil {
			return err
		}
		hasH = true
	}
	vb, hasVB, err := parseViewBox(n.attr("viewBox"))
	if err != nil {
		return err
	}
	switch {
	case hasVB:
		s.ViewBox = vb
		switch {
		case hasW && hasH:
		case hasW:
			h = w * vb.H / vb.W
		case hasH:
			w = h * vb.W / vb.H
		default:
			w, h = vb.W, vb.H
		}
	default:
		// Browsers size an svg without a size as 300 by 150.
		if !hasW {
			w = 300
		}
		if !hasH {
			h = 150
		}
		s.ViewBox = geom.Rect{W: w, H: h}
	}
	s.Width, s.Height = w, h
	if s.align, err = parseAlign(n.attr("preserveAspectRatio")); err != nil {
		return err
	}
	p.tol = flattenTolerance * max(s.ViewBox.W, s.ViewBox.H) / svgReferenceSize
	initial.viewport = s.ViewBox.Size()
	p.initial = initial
	st, err := p.style(&initial, n)
	if err != nil {
		return err
	}
	if v, _ := n.prop("display"); v == "none" || st.opacity <= 0 {
		return nil
	}
	g := p.beginGroup(n, &st)
	if err := p.children(n, &st); err != nil {
		return err
	}
	return p.endGroup(g, n, &st)
}

// nestedSVG sets up the viewport of an svg inside the drawing. Its content is
// not clipped to the viewport. A positive w or h, from a use, replaces the
// width or height of the element.
func (p *svgDocParser) nestedSVG(st *svgState, n *svgNode, w, h float32) error {
	vp := st.viewport
	x, err := lengthAttr(n, "x", vp.W, 0)
	if err != nil {
		return err
	}
	y, err := lengthAttr(n, "y", vp.H, 0)
	if err != nil {
		return err
	}
	if w <= 0 {
		if w, err = lengthAttr(n, "width", vp.W, vp.W); err != nil {
			return err
		}
	}
	if h <= 0 {
		if h, err = lengthAttr(n, "height", vp.H, vp.H); err != nil {
			return err
		}
	}
	m, viewport, err := viewportTransform(n, x, y, w, h)
	if err != nil {
		return err
	}
	st.ctm = st.ctm.Mul(m)
	st.viewport = viewport
	return nil
}

// viewportTransform returns the transform that the viewBox and
// preserveAspectRatio of n set up for a viewport at x, y of w by h, and the
// size that percentages inside then refer to.
func viewportTransform(n *svgNode, x, y, w, h float32) (geom.Affine, geom.Size, error) {
	vb, hasVB, err := parseViewBox(n.attr("viewBox"))
	if err != nil {
		return geom.Affine{}, geom.Size{}, err
	}
	if !hasVB || w <= 0 || h <= 0 {
		return geom.Translation(geom.Offset{X: x, Y: y}), geom.Size{W: w, H: h}, nil
	}
	al, err := parseAlign(n.attr("preserveAspectRatio"))
	if err != nil {
		return geom.Affine{}, geom.Size{}, err
	}
	sx, sy := w/vb.W, h/vb.H
	if !al.none {
		k := min(sx, sy)
		if al.slice {
			k = max(sx, sy)
		}
		sx, sy = k, k
	}
	tx := x + (w-vb.W*sx)*al.x - vb.X*sx
	ty := y + (h-vb.H*sy)*al.y - vb.Y*sy
	return geom.Affine{A: sx, D: sy, E: tx, F: ty}, vb.Size(), nil
}

// style returns the state of element n below parent: the properties that the
// cascade gives it, and its transform.
func (p *svgDocParser) style(parent *svgState, n *svgNode) (svgState, error) {
	st := *parent
	st.pathLength = 0
	st.opacity = 1
	for _, d := range n.decls {
		if err := p.property(&st, parent, d.name, d.value); err != nil {
			return st, err
		}
	}
	if v := n.attr("transform"); v != "" {
		m, err := parseTransform(v)
		if err != nil {
			return st, err
		}
		st.ctm = st.ctm.Mul(m)
	}
	return st, nil
}

// svgPaintKind is what fill or stroke is set to.
type svgPaintKind uint8

const (
	svgPaintNone svgPaintKind = iota
	svgPaintColor
	svgPaintCurrent
	// svgPaintServer refers to a gradient or a pattern by ref, with a
	// fallback for when the reference cannot be drawn.
	svgPaintServer
)

type svgPaint struct {
	kind  svgPaintKind
	color geom.Color
	ref   string
	// fallback and fallbackColor are what a paint server falls back to.
	fallback      svgPaintKind
	fallbackColor geom.Color
}

// svgState is the style in effect at an element.
type svgState struct {
	fill, stroke  svgPaint
	fillOpacity   float32
	strokeOpacity float32
	strokeWidth   float32
	cap           LineCap
	join          LineJoin
	miterLimit    float32
	rule          FillRule
	clipRule      FillRule
	// color is the value of the color property, if set; currentColor uses
	// it, or else what Draw is given.
	color    geom.Color
	hasColor bool
	visible  bool
	// opacity is the opacity of the element itself, which is not inherited:
	// an element that has one draws as a group. See svggroup.go.
	opacity float32
	// pathLength is the length that the author gave the path, which scales
	// its dashes, or zero.
	pathLength float32
	// dashes are the lengths of stroke-dasharray, always an even number of
	// them, or nil for a solid stroke.
	dashes     []float32
	dashOffset float32
	// markerStart, markerMid and markerEnd are references to markers.
	markerStart, markerMid, markerEnd string
	// text is the style of text.
	text svgTextState
	// order is paint-order: the steps of svgPaintSteps in the order drawn.
	order [3]uint8
	ctm   geom.Affine
	// viewport is the size that percentages refer to.
	viewport geom.Size
}

// property applies one presentation attribute or style declaration.
func (p *svgDocParser) property(st, parent *svgState, name, value string) error {
	value = strings.TrimSpace(value)
	if value == "inherit" {
		p.inherit(st, parent, name)
		return nil
	}
	switch name {
	case "fill":
		p.paint(&st.fill, value)
	case "stroke":
		p.paint(&st.stroke, value)
	case "color":
		if c, ok := parseColor(value); ok {
			st.color, st.hasColor = c, true
		}
	case "fill-opacity", "stroke-opacity", "opacity":
		v, err := parseOpacity(value)
		if err != nil {
			return err
		}
		switch name {
		case "fill-opacity":
			st.fillOpacity = v
		case "stroke-opacity":
			st.strokeOpacity = v
		default:
			st.opacity = v
		}
	case "stroke-width":
		v, err := parseLength(value, diagonal(st.viewport))
		if err != nil {
			return err
		}
		if v >= 0 {
			st.strokeWidth = v
		}
	case "stroke-miterlimit":
		v, err := parseNumber(value)
		if err != nil {
			return err
		}
		if v >= 1 {
			st.miterLimit = v
		}
	case "stroke-linecap":
		switch value {
		case "butt":
			st.cap = CapButt
		case "round":
			st.cap = CapRound
		case "square":
			st.cap = CapSquare
		}
	case "stroke-linejoin":
		switch value {
		case "miter", "miter-clip", "arcs":
			st.join = JoinMiter
		case "round":
			st.join = JoinRound
		case "bevel":
			st.join = JoinBevel
		}
	case "fill-rule":
		switch value {
		case "nonzero":
			st.rule = FillNonZero
		case "evenodd":
			st.rule = FillEvenOdd
		}
	case "visibility":
		switch value {
		case "visible":
			st.visible = true
		case "hidden", "collapse":
			st.visible = false
		}
	case "stroke-dasharray":
		st.dashes = parseDashes(value, diagonal(st.viewport))
	case "stroke-dashoffset":
		if v, err := parseLength(value, diagonal(st.viewport)); err == nil {
			st.dashOffset = v
		}
	case "marker-start":
		st.markerStart = markerRef(value)
	case "marker-mid":
		st.markerMid = markerRef(value)
	case "marker-end":
		st.markerEnd = markerRef(value)
	case "marker":
		ref := markerRef(value)
		st.markerStart, st.markerMid, st.markerEnd = ref, ref, ref
	case "paint-order":
		st.order = parsePaintOrder(value)
	case "font-family", "font-size", "font-weight", "font-style", "letter-spacing", "text-anchor",
		"dominant-baseline", "alignment-baseline":
		textProperty(st, parent, name, value)
	case "clip-rule":
		switch value {
		case "nonzero":
			st.clipRule = FillNonZero
		case "evenodd":
			st.clipRule = FillEvenOdd
		}
	}
	return nil
}

// markerRef returns the reference of a marker property, or "" for none.
func markerRef(value string) string {
	if strings.HasPrefix(value, "url(") {
		return value
	}
	return ""
}

// The steps of paint-order.
const (
	svgPaintFill uint8 = iota
	svgPaintStroke
	svgPaintMarkers
)

// parsePaintOrder reads paint-order: the steps named first, then the others
// in their usual order.
func parsePaintOrder(value string) [3]uint8 {
	order := [3]uint8{svgPaintFill, svgPaintStroke, svgPaintMarkers}
	n := 0
	var seen [3]bool
	for _, f := range strings.Fields(value) {
		var step uint8
		switch f {
		case "fill":
			step = svgPaintFill
		case "stroke":
			step = svgPaintStroke
		case "markers":
			step = svgPaintMarkers
		default:
			return [3]uint8{svgPaintFill, svgPaintStroke, svgPaintMarkers}
		}
		if !seen[step] && n < 3 {
			seen[step] = true
			order[n] = step
			n++
		}
	}
	for step := range uint8(3) {
		if !seen[step] && n < 3 {
			order[n] = step
			n++
		}
	}
	return order
}

func (p *svgDocParser) inherit(st, parent *svgState, name string) {
	switch name {
	case "fill":
		st.fill = parent.fill
	case "stroke":
		st.stroke = parent.stroke
	case "color":
		st.color, st.hasColor = parent.color, parent.hasColor
	case "fill-opacity":
		st.fillOpacity = parent.fillOpacity
	case "stroke-opacity":
		st.strokeOpacity = parent.strokeOpacity
	case "stroke-width":
		st.strokeWidth = parent.strokeWidth
	case "stroke-miterlimit":
		st.miterLimit = parent.miterLimit
	case "stroke-linecap":
		st.cap = parent.cap
	case "stroke-linejoin":
		st.join = parent.join
	case "fill-rule":
		st.rule = parent.rule
	case "clip-rule":
		st.clipRule = parent.clipRule
	case "visibility":
		st.visible = parent.visible
	case "stroke-dasharray":
		st.dashes = parent.dashes
	case "stroke-dashoffset":
		st.dashOffset = parent.dashOffset
	case "marker-start":
		st.markerStart = parent.markerStart
	case "marker-mid":
		st.markerMid = parent.markerMid
	case "marker-end":
		st.markerEnd = parent.markerEnd
	case "paint-order":
		st.order = parent.order
	}
}

// paint reads a value of fill or stroke. A value it does not understand leaves
// the inherited one, as in CSS.
func (p *svgDocParser) paint(dst *svgPaint, value string) {
	switch {
	case value == "none":
		*dst = svgPaint{}
	case strings.EqualFold(value, "currentColor"):
		*dst = svgPaint{kind: svgPaintCurrent}
	case strings.HasPrefix(value, "url("):
		// A paint server. The fallback after the reference applies when it
		// cannot be drawn, or else nothing.
		pt := svgPaint{kind: svgPaintServer, ref: value}
		if end := strings.IndexByte(value, ')'); end >= 0 {
			pt.ref = value[:end+1]
			if rest := strings.TrimSpace(value[end+1:]); rest != "" {
				var fb svgPaint
				p.paint(&fb, rest)
				if fb.kind != svgPaintServer {
					pt.fallback, pt.fallbackColor = fb.kind, fb.color
				}
			}
		}
		*dst = pt
	default:
		if c, ok := parseColor(value); ok {
			*dst = svgPaint{kind: svgPaintColor, color: c}
		}
	}
}

// shape returns the path of a basic shape or a path element in its own
// coordinates, and whether it is open, so that it has no fill. A shape that
// draws nothing returns nil.
func (p *svgDocParser) shape(st *svgState, t *svgNode) (*Path, bool, error) {
	vp := st.viewport
	num := func(name string, ref float32) (float32, error) { return lengthAttr(t, name, ref, 0) }
	switch t.name {
	case "path":
		d := t.attr("d")
		if strings.TrimSpace(d) == "" {
			return nil, false, nil
		}
		path, err := ParseSVGPath(d)
		return path, false, err
	case "rect":
		return rectShape(t, vp)
	case "circle", "ellipse":
		cx, err := num("cx", vp.W)
		if err != nil {
			return nil, false, err
		}
		cy, err := num("cy", vp.H)
		if err != nil {
			return nil, false, err
		}
		var rx, ry float32
		if t.name == "circle" {
			if rx, err = num("r", diagonal(vp)); err != nil {
				return nil, false, err
			}
			ry = rx
		} else {
			if rx, ry, err = radii(t, vp); err != nil {
				return nil, false, err
			}
		}
		if rx <= 0 || ry <= 0 {
			return nil, false, nil
		}
		return ellipsePath(cx, cy, rx, ry), false, nil
	case "line":
		var v [4]float32
		for i, name := range [4]string{"x1", "y1", "x2", "y2"} {
			ref := vp.W
			if i%2 == 1 {
				ref = vp.H
			}
			x, err := num(name, ref)
			if err != nil {
				return nil, false, err
			}
			v[i] = x
		}
		path := &Path{}
		path.MoveTo(v[0], v[1])
		path.LineTo(v[2], v[3])
		return path, true, nil
	case "polyline", "polygon":
		pts, err := parseNumbers(t.attr("points"))
		if err != nil {
			return nil, false, err
		}
		// An odd number leaves the last one out, as browsers do.
		if len(pts) < 4 {
			return nil, false, nil
		}
		path := &Path{}
		path.MoveTo(pts[0], pts[1])
		for i := 2; i+1 < len(pts); i += 2 {
			path.LineTo(pts[i], pts[i+1])
		}
		if t.name == "polygon" {
			path.Close()
		}
		return path, false, nil
	}
	return nil, false, nil
}

// radii reads rx and ry of an ellipse, where auto, or one left out, takes the
// other.
func radii(t *svgNode, vp geom.Size) (rx, ry float32, err error) {
	rxs, rys := t.attr("rx"), t.attr("ry")
	if rxs != "" && rxs != "auto" {
		if rx, err = parseLength(rxs, vp.W); err != nil {
			return 0, 0, err
		}
	}
	if rys != "" && rys != "auto" {
		if ry, err = parseLength(rys, vp.H); err != nil {
			return 0, 0, err
		}
	}
	hasX, hasY := rxs != "" && rxs != "auto", rys != "" && rys != "auto"
	switch {
	case hasX && !hasY:
		ry = rx
	case hasY && !hasX:
		rx = ry
	}
	return rx, ry, nil
}

func rectShape(t *svgNode, vp geom.Size) (*Path, bool, error) {
	var v [4]float32
	for i, name := range [4]string{"x", "y", "width", "height"} {
		ref := vp.W
		if i%2 == 1 {
			ref = vp.H
		}
		x, err := lengthAttr(t, name, ref, 0)
		if err != nil {
			return nil, false, err
		}
		v[i] = x
	}
	x, y, w, h := v[0], v[1], v[2], v[3]
	if w <= 0 || h <= 0 {
		return nil, false, nil
	}
	rx, ry, err := radii(t, vp)
	if err != nil {
		return nil, false, err
	}
	rx, ry = min(max(rx, 0), w/2), min(max(ry, 0), h/2)
	path := &Path{}
	if rx == 0 || ry == 0 {
		path.MoveTo(x, y)
		path.LineTo(x+w, y)
		path.LineTo(x+w, y+h)
		path.LineTo(x, y+h)
		path.Close()
		return path, false, nil
	}
	const k = circleControl
	r, b := x+w, y+h
	path.MoveTo(x+rx, y)
	path.LineTo(r-rx, y)
	path.CubeTo(r-rx+rx*k, y, r, y+ry-ry*k, r, y+ry)
	path.LineTo(r, b-ry)
	path.CubeTo(r, b-ry+ry*k, r-rx+rx*k, b, r-rx, b)
	path.LineTo(x+rx, b)
	path.CubeTo(x+rx-rx*k, b, x, b-ry+ry*k, x, b-ry)
	path.LineTo(x, y+ry)
	path.CubeTo(x, y+ry-ry*k, x+rx-rx*k, y, x+rx, y)
	path.Close()
	return path, false, nil
}

// ellipsePath returns an ellipse that starts at its right and runs clockwise on
// the screen, as SVG says.
func ellipsePath(cx, cy, rx, ry float32) *Path {
	const k = circleControl
	p := &Path{}
	p.MoveTo(cx+rx, cy)
	p.CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry)
	p.CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy)
	p.CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry)
	p.CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy)
	p.Close()
	return p
}

// emit adds the fill, the stroke and the markers of a shape, in the order of
// paint-order. verts are the vertices where markers go.
func (p *svgDocParser) emit(st *svgState, path *Path, open bool, verts []svgVertex) error {
	if !st.visible || st.opacity <= 0 || st.ctm.Det() == 0 {
		return nil
	}
	bbox := path.Bounds()
	p.geoms = append(p.geoms, svgGeom{path: path, ctm: st.ctm})
	for _, step := range st.order {
		switch step {
		case svgPaintFill:
			if !open && st.fill.kind != svgPaintNone {
				path.Rule = st.rule
				p.add(st, st.fill, st.fillOpacity, path.transformed(st.ctm), bbox)
			}
		case svgPaintStroke:
			if st.stroke.kind == svgPaintNone || st.strokeWidth <= 0 {
				continue
			}
			tol := p.tol / maxScale(st.ctm)
			stroked := path
			if st.dashes != nil {
				scale := float32(1)
				if st.pathLength > 0 {
					scale = pathLength(path, tol) / st.pathLength
				}
				stroked = dash(path, st.dashes, st.dashOffset, scale, tol)
			}
			outline := stroked.Stroke(StrokeStyle{
				Width:      st.strokeWidth,
				Cap:        st.cap,
				Join:       st.join,
				MiterLimit: st.miterLimit,
				Tolerance:  tol,
			})
			if !outline.Empty() {
				p.add(st, st.stroke, st.strokeOpacity, outline.transformed(st.ctm), bbox)
			}
		case svgPaintMarkers:
			// Markers are not part of the bounding box of the shape.
			k := len(p.geoms)
			if err := p.markers(st, verts); err != nil {
				return err
			}
			p.geoms = p.geoms[:k]
		}
	}
	return nil
}

// add adds path, in the coordinates of the drawing, painted with pt. bbox is
// the box of the geometry of the element in its own coordinates, which
// gradients in objectBoundingBox units refer to.
func (p *svgDocParser) add(st *svgState, pt svgPaint, opacity float32, path *Path, bbox geom.Rect) {
	sh := svgShape{path: path, color: pt.color}
	if pt.kind == svgPaintServer {
		g, c, kind := p.server(st, pt, bbox, opacity)
		switch kind {
		case svgPaintNone:
			return
		case svgPaintServer:
			p.svg.grads = append(p.svg.grads, g)
			sh.grad = len(p.svg.grads)
			p.bound(path.Bounds())
			p.svg.shapes = append(p.svg.shapes, sh)
			p.op(svgOpShape, len(p.svg.shapes)-1)
			return
		}
		pt.kind, sh.color = kind, c
	}
	if pt.kind == svgPaintCurrent {
		if st.hasColor {
			sh.color = st.color
		} else {
			sh.color, sh.current = geom.Color{A: 1}, true
		}
	}
	sh.color.A *= opacity
	if sh.color.A <= 0 {
		return
	}
	p.bound(path.Bounds())
	p.svg.shapes = append(p.svg.shapes, sh)
	p.op(svgOpShape, len(p.svg.shapes)-1)
}

// bound grows the bounds of the drawing by b.
func (p *svgDocParser) bound(b geom.Rect) {
	if !p.bounded {
		p.svg.bounds, p.bounded = b, true
		return
	}
	u := p.svg.bounds
	x0, y0 := min(u.X, b.X), min(u.Y, b.Y)
	x1, y1 := max(u.X+u.W, b.X+b.W), max(u.Y+u.H, b.Y+b.H)
	p.svg.bounds = geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
