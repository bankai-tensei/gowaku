package nav

import (
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// DockZone says where a panel dropped on a group goes: into it as a tab, or
// beside it on one side, which splits the room of the group.
//
// It is DockCenter (the zero value), DockLeft, DockRight, DockTop or
// DockBottom.
type DockZone uint8

// The zones. The zero value puts the panel in the group.
const (
	DockCenter DockZone = iota
	DockLeft
	DockRight
	DockTop
	DockBottom
)

// side returns z as the edge of a hidden panel: DockLeft for anything that is
// not an edge.
func (z DockZone) side() DockZone {
	if z < DockLeft || z > DockBottom {
		return DockLeft
	}
	return z
}

// String returns the name of the zone.
func (z DockZone) String() string {
	switch z {
	case DockLeft:
		return "left"
	case DockRight:
		return "right"
	case DockTop:
		return "top"
	case DockBottom:
		return "bottom"
	}
	return "center"
}

// axis returns the axis along which an edge zone splits, and whether the new
// node goes first.
func (z DockZone) axis() (layout.Axis, bool) {
	switch z {
	case DockTop:
		return layout.Vertical, true
	case DockBottom:
		return layout.Vertical, false
	case DockLeft:
		return layout.Horizontal, true
	}
	return layout.Horizontal, false
}

// DockNode is one node of a DockArrangement. With Children it is a split:
// the children side by side along Axis, Horizontal putting the first at the
// left, with Weights their shares of the room. Without Children it is a group
// of panels shown as tabs, in the order of Panels, with Active the one shown.
type DockNode struct {
	Axis     layout.Axis
	Children []DockNode
	// Weights are the shares of the children, in any unit. A list of another
	// length, or one with a share that is not positive, shares the room evenly.
	Weights []float32
	// Panels are the IDs of the panels of a group, and Active that of the
	// panel it shows: the first one if it is not one of Panels.
	Panels []string
	Active string
}

// DockArrangement is where the panels of a DockLayout are: a plain value to
// save with the window and give back to DockController.SetArrangement. It
// marshals to and from JSON with encoding/json, with a version that
// unmarshaling checks; see docs/ADR/0098-dock-layout.md for the format.
//
// An arrangement need not match the panels the layout has. Panels it names
// that the layout does not have are left out, and panels the layout has that
// it does not name go to the current group, unless Closed names them.
//
// The zero value has no panels, and lets the layout put them all in one group.
type DockArrangement struct {
	Root DockNode
	// Maximized is the ID of a panel whose group fills the layout, hiding the
	// others, or empty.
	Maximized string
	// Closed are the panels the user closed, which stay closed until
	// DockController.Activate shows them again.
	Closed []string
	// Hidden are the panels hidden at the edges of the layout, each shown as
	// a tab on the strip of its side, in order.
	Hidden []DockHidden
}

// DockHidden is a panel hidden at an edge of a DockLayout: a tab on the strip
// along that edge, whose panel slides out over the layout when pressed.
type DockHidden struct {
	ID string
	// Side is the edge, DockLeft, DockRight, DockTop or DockBottom. Any other
	// value reads as DockLeft.
	Side DockZone
	// Size is the width, or for the top and bottom the height, of the panel
	// when it slides out. Zero lets the layout choose.
	Size float32
	// Origin are the panels of the group the panel left, itself among them,
	// in order: the group it goes back to when pinned, if one of them is still
	// there. Panels hidden together, with the same Origin, go back together.
	Origin []string
	// Share is the part of the layout across Side that the group took, which
	// a panel whose group is gone takes back at that edge. Zero takes a
	// quarter.
	Share float32
}

// empty reports whether a holds no node: the zero value.
func (a *DockArrangement) empty() bool {
	return len(a.Root.Children) == 0 && len(a.Root.Panels) == 0
}

// dockNode is a node of the tree that a DockController keeps. Splits have
// children and groups have panels; a group with no panels only stands for an
// empty layout, at the root.
type dockNode struct {
	// id tells the node apart for as long as it lives, which keys its tab
	// strip and its dividers.
	id     int
	parent *dockNode
	axis   layout.Axis
	// children and weights are those of a split; the weights sum to one.
	children []*dockNode
	weights  []float32
	// panels and active are those of a group.
	panels []string
	active string
	// rect is where the node was at the last layout, in the layout.
	rect geom.Rect
}

func (n *dockNode) isGroup() bool { return len(n.children) == 0 }

// body returns the room of a group below its tab strip.
func (n *dockNode) body() geom.Rect {
	r := n.rect
	h := min(float32(tabBarHeight), r.H)
	return geom.Rect{X: r.X, Y: r.Y + h, W: r.W, H: r.H - h}
}

// indexOf returns the index of the panel id in the group, or -1.
func (n *dockNode) indexOf(id string) int { return slices.Index(n.panels, id) }

// fixActive keeps the active panel one of the panels: preferring the one at
// index i, then the one before it, when the active one went.
func (n *dockNode) fixActive(i int) {
	if n.active != "" && n.indexOf(n.active) >= 0 {
		return
	}
	switch {
	case len(n.panels) == 0:
		n.active = ""
	case i < len(n.panels):
		n.active = n.panels[max(i, 0)]
	default:
		n.active = n.panels[len(n.panels)-1]
	}
}

// eachGroup calls fn for every group below n, in order: the left or top first.
func (n *dockNode) eachGroup(fn func(g *dockNode)) {
	if n.isGroup() {
		fn(n)
		return
	}
	for _, c := range n.children {
		c.eachGroup(fn)
	}
}

// eachSplit calls fn for every split below n, n first.
func (n *dockNode) eachSplit(fn func(s *dockNode)) {
	if n.isGroup() {
		return
	}
	fn(n)
	for _, c := range n.children {
		c.eachSplit(fn)
	}
}

// contains reports whether m is n or below it.
func (n *dockNode) contains(m *dockNode) bool {
	for ; m != nil; m = m.parent {
		if m == n {
			return true
		}
	}
	return false
}

// minAlong returns the smallest length of n along axis.
func (n *dockNode) minAlong(axis layout.Axis) float32 {
	if n.isGroup() {
		if axis == layout.Vertical {
			return dockMinGroupHeight
		}
		return dockMinGroupWidth
	}
	var total float32
	for i, c := range n.children {
		m := c.minAlong(axis)
		if n.axis != axis {
			total = max(total, m)
			continue
		}
		if i > 0 {
			total += dividerThickness
		}
		total += m
	}
	return total
}

// place lays n out in r, and its children within it.
func (n *dockNode) place(r geom.Rect) {
	n.rect = r
	if n.isGroup() {
		return
	}
	length := r.W
	if n.axis == layout.Vertical {
		length = r.H
	}
	room := max(length-float32(len(n.children)-1)*dividerThickness, 0)
	at, used := float32(0), float32(0)
	for i, c := range n.children {
		size := room - used
		if i < len(n.children)-1 {
			size = min(geom.Floor32(n.weights[i]*room+0.5), room-used)
		}
		used += size
		if n.axis == layout.Vertical {
			c.place(geom.Rect{X: r.X, Y: r.Y + at, W: r.W, H: size})
		} else {
			c.place(geom.Rect{X: r.X + at, Y: r.Y, W: size, H: r.H})
		}
		at += size + dividerThickness
	}
}

// normalize scales w to sum to one, sharing evenly when any share is not a
// positive number.
func normalize(w []float32) {
	var sum float32
	for _, x := range w {
		if !(x > 0) || x > geom.Inf/2 {
			sum = 0
			break
		}
		sum += x
	}
	for i := range w {
		if sum > 0 {
			w[i] /= sum
		} else {
			w[i] = 1 / float32(len(w))
		}
	}
}
