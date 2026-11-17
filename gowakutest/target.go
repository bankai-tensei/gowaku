package gowakutest

import (
	"fmt"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// targetGrid is how many points across and down target tries, after the
// middle, to find one of the visible part of a node that nothing covers.
const targetGrid = 5

// target returns the point at which the action what, such as "Tap", reaches
// the node labeled label, the one Find returns: the middle of the part of the
// node that shows, or, if something drawn above covers that, another point of
// that part that nothing covers. It fails the test, saying where the node is,
// if there is no such node, if it lies outside the window, if the clips of
// its ancestors (core.Clipper) hide it whole, or if something covers all of
// what shows of it, since a click there would reach something else.
func (tt *Tester) target(what, label string) geom.Offset {
	tt.t.Helper()
	if tt.tree.NeedsFrame() {
		tt.frame()
	}
	// SemanticsTree, unlike Semantics, keeps the objects of the nodes.
	root, _ := tt.tree.SemanticsTree()
	n, ok := best(findAll(root, label, -1, nil))
	if !ok {
		tt.t.Fatalf("gowakutest: %s: nothing labeled %q in\n%s", what, label, tt.Dump())
	}
	owner := tt.tree.SemanticsOwner(n.ID)
	window := geom.Rect{W: tt.size.W, H: tt.size.H}
	r := n.Rect
	if r.W <= 0 || r.H <= 0 {
		// A node of no size is reached at its place, if that is in view.
		p := r.Center()
		if !window.Contains(p) {
			tt.t.Fatalf("gowakutest: %s: the node %q at %s %s; scroll it into view", what, label, place(r), whereOutside(r, tt.size))
		}
		tt.uncovered(what, label, root, n, owner, []geom.Offset{p})
		return p
	}
	if r.Intersect(window).Size().Empty() {
		tt.t.Fatalf("gowakutest: %s: the node %q at %s %s; scroll it into view", what, label, place(r), whereOutside(r, tt.size))
	}
	vis := r.Intersect(window)
	if owner != nil {
		var by core.RenderObject
		var shown geom.Rect
		vis, by, shown = clipOf(owner, vis)
		if by != nil {
			tt.t.Fatalf("gowakutest: %s: the node %q at %s is cut away whole by %s, which shows only %s of the %gx%g window; scroll it into view",
				what, label, place(r), typeName(by), place(shown), tt.size.W, tt.size.H)
		}
	}
	points := make([]geom.Offset, 0, 1+targetGrid*targetGrid)
	points = append(points, vis.Center())
	for j := range targetGrid {
		for i := range targetGrid {
			points = append(points, geom.Offset{
				X: vis.X + vis.W*(float32(i)+0.5)/targetGrid,
				Y: vis.Y + vis.H*(float32(j)+0.5)/targetGrid,
			})
		}
	}
	return tt.uncovered(what, label, root, n, owner, points)
}

// uncovered returns the first of points at which a hit test reaches owner, the
// object of the node n of the tree root, or what is inside it, and fails the test, naming what
// covers the first point, if there is none. A point where nothing at all is
// hit, or where only the ancestors of owner are, is not covered.
func (tt *Tester) uncovered(what, label string, root, n *Node, owner core.RenderObject, points []geom.Offset) geom.Offset {
	tt.t.Helper()
	if owner == nil {
		return points[0]
	}
	var cover core.RenderObject
	for k, p := range points {
		path := tt.tree.HitTest(p).Path
		if len(path) == 0 || within(path[0], owner) || within(owner, path[0]) {
			return p
		}
		if k == 0 {
			cover = path[0]
		}
	}
	tt.t.Fatalf("gowakutest: %s: the node %q at %s is covered at (%g, %g) by %s%s, which is drawn above it and would take the pointer",
		what, label, place(n.Rect), points[0].X, points[0].Y, typeName(cover), tt.nameOf(root, cover))
	return geom.Offset{}
}

// atWindow fails the action what, as "TapAt", if pos lies outside the window,
// where no pointer reaches.
func (tt *Tester) atWindow(what string, pos geom.Offset) {
	tt.t.Helper()
	if pos.X < 0 || pos.Y < 0 || pos.X >= tt.size.W || pos.Y >= tt.size.H {
		tt.t.Fatalf("gowakutest: %s: (%g, %g) %s", what, pos.X, pos.Y, whereOutside(geom.Rect{X: pos.X, Y: pos.Y}, tt.size))
	}
}

// within reports whether ro is a or lies inside it.
func within(ro, a core.RenderObject) bool {
	for ; ro != nil; ro = ro.Parent() {
		if ro == a {
			return true
		}
	}
	return false
}

// clipOf returns what of vis, a part of the window, the ancestors of ro let
// show, through their clips and the transforms that move the pointer with the
// picture. If they hide it whole, it returns too the ancestor whose clip took
// the last of it, and the clip of that one, in the window.
func clipOf(ro core.RenderObject, vis geom.Rect) (geom.Rect, core.RenderObject, geom.Rect) {
	var up []core.RenderObject
	for a := ro.Parent(); a != nil; a = a.Parent() {
		up = append(up, a)
	}
	m := geom.Identity()
	for i := len(up) - 1; i >= 0; i-- {
		a := up[i]
		m = m.Mul(geom.Translation(a.Offset()))
		if c, ok := a.(core.Clipper); ok {
			if cc, cuts := c.ChildClip(); cuts {
				cut := m.ApplyRect(cc)
				if vis = vis.Intersect(cut); vis.Size().Empty() {
					return vis, a, cut
				}
			}
		}
		if t, ok := a.(core.Transformer); ok {
			if c := t.ChildTransform(); !c.IsIdentity() {
				m = m.Mul(c)
			}
		}
	}
	return vis, nil, geom.Rect{}
}

// nameOf says, for a message, which node of the accessibility tree ro draws:
// that of ro or of its nearest ancestor that has a label, in the tree root
// of the last SemanticsTree.
func (tt *Tester) nameOf(root *Node, ro core.RenderObject) string {
	nodes := map[core.RenderObject]*Node{}
	var visit func(n *Node)
	visit = func(n *Node) {
		if n.Label != "" {
			if o := tt.tree.SemanticsOwner(n.ID); o != nil {
				nodes[o] = n
			}
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(root)
	for a := ro; a != nil; a = a.Parent() {
		if n := nodes[a]; n != nil {
			return fmt.Sprintf(" (the %s %q)", strings.ToLower(n.Role.String()), n.Label)
		}
	}
	return ""
}

// place writes where r is, for a message: its top left and size.
func place(r geom.Rect) string {
	return fmt.Sprintf("x=%g y=%g (%gx%g)", r.X, r.Y, r.W, r.H)
}

// whereOutside says on which side of a window of size sz the rectangle r
// lies, which must be outside it.
func whereOutside(r geom.Rect, sz geom.Size) string {
	switch {
	case r.Y >= sz.H:
		return fmt.Sprintf("is below the bottom of the %gx%g window", sz.W, sz.H)
	case r.Y+r.H <= 0:
		return fmt.Sprintf("is above the top of the %gx%g window", sz.W, sz.H)
	case r.X >= sz.W:
		return fmt.Sprintf("is right of the %gx%g window", sz.W, sz.H)
	default:
		return fmt.Sprintf("is left of the %gx%g window", sz.W, sz.H)
	}
}
