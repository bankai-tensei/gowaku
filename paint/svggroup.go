package paint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// An SVG draws a list of operations: shapes, and groups that composite what
// they hold as one picture, with an opacity, a clip path, a mask or a filter.
// Groups exist only where a browser would composite: an opacity on a single
// shape folds into its color, and a group without effects disappears. A
// drawing without groups draws its shapes straight onto the canvas, as icons
// always have; one with groups draws them inside groups of the canvas, which
// the GPU composites (Canvas.PushGroup). See
// docs/ADR/0112-svg-structure-paint-clip.md and
// docs/ADR/0114-gpu-layers-masks-filters.md.

type svgOpKind uint8

const (
	// svgOpShape draws SVG.shapes[index].
	svgOpShape svgOpKind = iota
	// svgOpPush opens SVG.groups[index], and svgOpPop closes it.
	svgOpPush
	svgOpPop
)

type svgOp struct {
	kind  svgOpKind
	index int
	// end is, for svgOpPush, the index of the matching svgOpPop in the same
	// list, which finish sets.
	end int
}

// svgGroup composites what it holds as one picture: filtered, then clipped,
// then masked, then faded, as SVG orders the effects.
type svgGroup struct {
	opacity float32
	clip    *svgClip
	mask    *svgMask
	filter  *svgFilter
	// hidden means that the clip shows nothing, so that the group draws
	// nothing.
	hidden bool
}

// svgClip is the union of paths, each filled with its own clip-rule, in the
// coordinates of the drawing, intersected with clip if set.
type svgClip struct {
	paths []*Path
	clip  *svgClip
}

// svgMask multiplies a group by the luminance, or the alpha, of what ops draw,
// inside region.
type svgMask struct {
	ops    []svgOp
	region *Path
	alpha  bool
}

// svgFrame is a group being read.
type svgFrame struct {
	group     int
	firstGeom int
	ctm       geom.Affine
}

// svgGeom is the geometry of a shape: its path in its own coordinates, and
// what places it in the drawing.
type svgGeom struct {
	path *Path
	ctm  geom.Affine
}

// op appends an operation to the list being read.
func (p *svgDocParser) op(kind svgOpKind, index int) {
	*p.ops = append(*p.ops, svgOp{kind: kind, index: index})
}

// hasEffect reports whether a property that makes a group is set to
// something other than none.
func hasEffect(n *svgNode, name string) bool {
	v, _ := n.prop(name)
	return v != "" && v != "none"
}

// beginGroup opens the group of element n if its properties ask for one, and
// returns its index, or -1. The opacity of st moves to the group.
func (p *svgDocParser) beginGroup(n *svgNode, st *svgState) int {
	if st.opacity >= 1 && !hasEffect(n, "clip-path") && !hasEffect(n, "mask") && !hasEffect(n, "filter") {
		return -1
	}
	g := len(p.svg.groups)
	p.svg.groups = append(p.svg.groups, svgGroup{opacity: st.opacity})
	st.opacity = 1
	p.op(svgOpPush, g)
	p.frames = append(p.frames, svgFrame{group: g, firstGeom: len(p.geoms), ctm: st.ctm})
	return g
}

// endGroup closes group g, which element n opened, and resolves its effects,
// which can refer to the bounding box of what it drew.
func (p *svgDocParser) endGroup(g int, n *svgNode, st *svgState) error {
	if g < 0 {
		return nil
	}
	p.op(svgOpPop, g)
	f := p.frames[len(p.frames)-1]
	p.frames = p.frames[:len(p.frames)-1]
	bbox := p.bboxOf(f)
	// Resolving an effect can add groups, which moves p.svg.groups, so each
	// result is stored by index.
	if v, _ := n.prop("filter"); v != "" && v != "none" {
		filter, err := p.filterOf(v, st, bbox)
		if err != nil {
			return fmt.Errorf("<%s>: %w", n.name, err)
		}
		p.svg.groups[g].filter = filter
	}
	if v, _ := n.prop("clip-path"); v != "" && v != "none" {
		clip, hidden, err := p.clipPathOf(v, st, bbox, 0)
		if err != nil {
			return fmt.Errorf("<%s>: %w", n.name, err)
		}
		p.svg.groups[g].clip = clip
		p.svg.groups[g].hidden = p.svg.groups[g].hidden || hidden
	}
	if v, _ := n.prop("mask"); v != "" && v != "none" {
		mask, err := p.maskOf(v, st, bbox)
		if err != nil {
			return fmt.Errorf("<%s>: %w", n.name, err)
		}
		p.svg.groups[g].mask = mask
	}
	return nil
}

// bboxOf returns the bounding box of what frame f drew, in the coordinates
// in which the group was opened: the union of the geometry of its shapes,
// without strokes and markers.
func (p *svgDocParser) bboxOf(f svgFrame) geom.Rect {
	inv, ok := f.ctm.Invert()
	if !ok {
		return geom.Rect{}
	}
	first := true
	var lo, hi geom.Offset
	for _, gm := range p.geoms[f.firstGeom:] {
		m := inv.Mul(gm.ctm)
		pts, _ := gm.path.Polygon()
		for _, q := range pts {
			q = m.Apply(q)
			if first {
				lo, hi, first = q, q, false
				continue
			}
			lo.X, lo.Y = min(lo.X, q.X), min(lo.Y, q.Y)
			hi.X, hi.Y = max(hi.X, q.X), max(hi.Y, q.Y)
		}
	}
	return geom.Rect{X: lo.X, Y: lo.Y, W: hi.X - lo.X, H: hi.Y - lo.Y}
}

// unitsMatrix returns what objectBoundingBox units map through for bbox, and
// whether there is such a mapping: a box without area has none.
func unitsMatrix(boxUnits bool, bbox geom.Rect) (geom.Affine, bool) {
	if !boxUnits {
		return geom.Identity(), true
	}
	if bbox.W <= 0 || bbox.H <= 0 {
		return geom.Affine{}, false
	}
	return geom.Affine{A: bbox.W, D: bbox.H, E: bbox.X, F: bbox.Y}, true
}

// svgClipDepth bounds a chain of clip paths that clip each other.
const svgClipDepth = 16

// clipPathOf resolves the clip path that ref names for an element with state
// st and bounding box bbox. It returns nil for a reference to nothing, which
// clips nothing, and reports hidden for a clip that shows nothing.
func (p *svgDocParser) clipPathOf(ref string, st *svgState, bbox geom.Rect, depth int) (clip *svgClip, hidden bool, err error) {
	n := p.tree.lookup(ref)
	if n == nil || n.name != "clipPath" {
		if !strings.HasPrefix(strings.TrimSpace(ref), "url(") {
			// Basic shapes such as circle() of CSS are not read.
			p.svg.Skipped++
		}
		return nil, false, nil
	}
	if depth >= svgClipDepth || slices.Contains(p.active, n) {
		p.svg.Skipped++
		return nil, false, nil
	}
	inner, err := p.stateAt(n)
	if err != nil {
		return nil, false, err
	}
	units, ok := unitsMatrix(strings.TrimSpace(n.attr("clipPathUnits")) == "objectBoundingBox", bbox)
	if !ok {
		return nil, true, nil
	}
	m := st.ctm
	if v := n.attr("transform"); v != "" {
		t, err := parseTransform(v)
		if err != nil {
			return nil, false, fmt.Errorf("<clipPath>: %w", err)
		}
		m = m.Mul(t)
	}
	inner.ctm = m.Mul(units)
	inner.viewport = st.viewport
	clip = &svgClip{}
	p.active = append(p.active, n)
	defer func() { p.active = p.active[:len(p.active)-1] }()
	for _, c := range n.children {
		if err := p.clipChild(clip, c, &inner, 0); err != nil {
			return nil, false, err
		}
	}
	if len(clip.paths) == 0 {
		return nil, true, nil
	}
	if v, _ := n.prop("clip-path"); v != "" && v != "none" {
		outer, outerHidden, err := p.clipPathOf(v, st, bbox, depth+1)
		if err != nil {
			return nil, false, err
		}
		if outerHidden {
			return nil, true, nil
		}
		clip.clip = outer
	}
	return clip, false, nil
}

// clipChild adds the geometry of child c of a clip path to clip. Shapes and
// text count, and a use of one; anything else is ignored, as in SVG.
func (p *svgDocParser) clipChild(clip *svgClip, c *svgNode, parent *svgState, depth int) error {
	if c.name == "" || c.foreign || !p.visit() {
		return nil
	}
	st, err := p.style(parent, c)
	if err != nil {
		return fmt.Errorf("<%s>: %w", c.name, err)
	}
	if v, _ := c.prop("display"); v == "none" || !st.visible {
		return nil
	}
	if hasEffect(c, "clip-path") {
		// A clip path on a child of a clip path is not drawn.
		p.svg.Skipped++
	}
	switch c.name {
	case "use":
		target := p.tree.lookup(c.href())
		if target == nil || depth > 0 || slices.Contains(p.active, target) {
			return nil
		}
		x, err := lengthAttr(c, "x", st.viewport.W, 0)
		if err != nil {
			return fmt.Errorf("<use>: %w", err)
		}
		y, err := lengthAttr(c, "y", st.viewport.H, 0)
		if err != nil {
			return fmt.Errorf("<use>: %w", err)
		}
		st.ctm = st.ctm.Mul(geom.Translation(geom.Offset{X: x, Y: y}))
		return p.clipChild(clip, target, &st, depth+1)
	case "text":
		return p.clipText(clip, c, &st)
	}
	path, _, err := p.shape(&st, c)
	if err != nil {
		return fmt.Errorf("<%s>: %w", c.name, err)
	}
	if path == nil || st.ctm.Det() == 0 {
		return nil
	}
	q := path.transformed(st.ctm)
	q.Rule = st.clipRule
	clip.paths = append(clip.paths, q)
	return nil
}

// maskOf resolves the mask that ref names for an element with state st and
// bounding box bbox. A reference to nothing masks nothing.
func (p *svgDocParser) maskOf(ref string, st *svgState, bbox geom.Rect) (*svgMask, error) {
	n := p.tree.lookup(ref)
	if n == nil || n.name != "mask" {
		return nil, nil
	}
	if slices.Contains(p.active, n) {
		p.svg.Skipped++
		return nil, nil
	}
	boxUnits := strings.TrimSpace(n.attr("maskUnits")) != "userSpaceOnUse"
	m := &svgMask{}
	if v, _ := n.prop("mask-type"); v == "alpha" {
		m.alpha = true
	}
	units, ok := unitsMatrix(boxUnits, bbox)
	if !ok {
		// A box without area shows nothing through the mask.
		m.region = &Path{}
		return m, nil
	}
	vp := st.viewport
	defaults := [4]string{"-10%", "-10%", "120%", "120%"}
	refs := [4]float32{vp.W, vp.H, vp.W, vp.H}
	var r [4]float32
	for i, name := range [4]string{"x", "y", "width", "height"} {
		v := n.attr(name)
		if strings.TrimSpace(v) == "" {
			v = defaults[i]
		}
		ref := refs[i]
		if boxUnits {
			ref = 1
		}
		x, err := parseLength(v, ref)
		if err != nil {
			return nil, fmt.Errorf("<mask>: %w", err)
		}
		r[i] = x
	}
	if r[2] <= 0 || r[3] <= 0 {
		m.region = &Path{}
		return m, nil
	}
	region := &Path{}
	region.AddRRect(geom.RRect{Rect: geom.Rect{X: r[0], Y: r[1], W: r[2], H: r[3]}})
	m.region = region.transformed(st.ctm.Mul(units))
	inner, err := p.stateAt(n)
	if err != nil {
		return nil, err
	}
	contentUnits, ok := unitsMatrix(strings.TrimSpace(n.attr("maskContentUnits")) == "objectBoundingBox", bbox)
	if !ok {
		return m, nil
	}
	inner.ctm = st.ctm.Mul(contentUnits)
	inner.viewport = vp
	saved := p.ops
	p.ops = &m.ops
	err = p.children(n, &inner)
	p.ops = saved
	if err != nil {
		return nil, err
	}
	return m, nil
}

// clipMarker draws the content of marker n with state st, clipped to the
// viewport clip of the marker that m places, as overflow hidden asks. The
// clip is dropped when the content stays inside it, which it usually does.
func (p *svgDocParser) clipMarker(n *svgNode, m geom.Affine, clip geom.Rect, st *svgState) error {
	if v, _ := n.prop("overflow"); v == "visible" || v == "auto" {
		return p.children(n, st)
	}
	g := len(p.svg.groups)
	p.svg.groups = append(p.svg.groups, svgGroup{opacity: 1})
	p.op(svgOpPush, g)
	firstShape := len(p.svg.shapes)
	if err := p.children(n, st); err != nil {
		return err
	}
	p.op(svgOpPop, g)
	inv, ok := m.Invert()
	if !ok {
		return nil
	}
	const eps = 1e-3
	inside := true
	for _, sh := range p.svg.shapes[firstShape:] {
		pts, _ := sh.path.Polygon()
		for _, q := range pts {
			q = inv.Apply(q)
			if q.X < clip.X-eps || q.Y < clip.Y-eps || q.X > clip.X+clip.W+eps || q.Y > clip.Y+clip.H+eps {
				inside = false
				break
			}
		}
		if !inside {
			break
		}
	}
	if !inside {
		r := &Path{}
		r.AddRRect(geom.RRect{Rect: clip})
		p.svg.groups[g].clip = &svgClip{paths: []*Path{r.transformed(m)}}
	}
	return nil
}

// finish folds the groups that need no compositing into their content.
func (s *SVG) finish() {
	s.ops = s.fold(s.ops)
	linkGroups(s.ops)
	for i := range s.groups {
		if mk := s.groups[i].mask; mk != nil {
			mk.ops = s.fold(mk.ops)
			linkGroups(mk.ops)
		}
	}
	s.composite = slices.ContainsFunc(s.ops, func(o svgOp) bool { return o.kind == svgOpPush })
}

// linkGroups sets the end of every push of ops. fold leaves them balanced.
func linkGroups(ops []svgOp) {
	var open []int
	for i := range ops {
		switch ops[i].kind {
		case svgOpPush:
			open = append(open, i)
		case svgOpPop:
			if n := len(open); n > 0 {
				ops[open[n-1]].end = i
				open = open[:n-1]
			}
		}
	}
	for _, i := range open {
		ops[i].end = len(ops)
	}
}

// fold returns ops with the groups that need no compositing replaced by what
// they hold: groups without effects, and an opacity over a single shape,
// which multiplies into its color. A group that shows nothing goes with its
// content.
func (s *SVG) fold(ops []svgOp) []svgOp {
	out := make([]svgOp, 0, len(ops))
	for i := 0; i < len(ops); i++ {
		o := ops[i]
		if o.kind != svgOpPush {
			out = append(out, o)
			continue
		}
		// Find the matching pop.
		depth, j := 1, i+1
		for ; j < len(ops) && depth > 0; j++ {
			switch ops[j].kind {
			case svgOpPush:
				depth++
			case svgOpPop:
				depth--
			}
		}
		inner := s.fold(ops[i+1 : j-1])
		i = j - 1
		g := &s.groups[o.index]
		if g.hidden || g.opacity <= 0 || len(inner) == 0 && g.filter == nil {
			continue
		}
		effects := g.clip != nil || g.mask != nil || g.filter != nil
		switch {
		case !effects && g.opacity >= 1:
			out = append(out, inner...)
		case !effects && len(inner) == 1 && inner[0].kind == svgOpShape:
			s.fade(inner[0].index, g.opacity)
			out = append(out, inner...)
		default:
			out = append(out, o)
			out = append(out, inner...)
			out = append(out, svgOp{kind: svgOpPop, index: o.index})
		}
	}
	return out
}

// fade multiplies the alpha of shape i by a.
func (s *SVG) fade(i int, a float32) {
	sh := &s.shapes[i]
	sh.color.A *= a
	if sh.grad > 0 {
		stops := *s.grads[sh.grad-1].g.MoreStops
		for k := range stops {
			stops[k].Color.A *= a
		}
	}
}
