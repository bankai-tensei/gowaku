package nav

import "slices"

// The shares of a panel docked at an edge.
const (
	// dockRootShare is that of a panel docked at an edge of the whole layout.
	dockRootShare = 0.25
	// dockGroupShare is that of a panel docked at an edge of a group.
	dockGroupShare = 0.5
)

// DockController holds the arrangement of a DockLayout and changes it from
// code: to save and restore it, to show a panel, or to put everything back.
// The zero value is ready; keep it where it outlives a build, such as in a
// State, and put it in DockLayout.Controller. Without one the layout keeps its
// own.
//
// Changes made through the controller do not call DockLayout.OnChange, which
// reports those of the user. A controller belongs to one layout at a time. Its
// methods run on the UI goroutine.
type DockController struct {
	// root is the tree, nil until the first build or after Reset.
	root *dockNode
	// maximized is the group that fills the layout, or nil.
	maximized *dockNode
	// current is the group that has, or last had, the keyboard: new panels
	// and the keys of the layout go there. Nil selects the first group.
	current *dockNode
	// closed are the panels the user closed.
	closed []string
	// hidden are the panels hidden at the edges, and flyout the one of them
	// that slid out, or empty.
	hidden []dockHidden
	flyout string
	// focus is where the keyboard goes at the next layout.
	focus  dockFocus
	nextID int
	// view is the layout that shows the controller, or nil.
	view *dockState
}

// Arrangement returns where the panels are now, to save. It is a copy that
// later changes do not touch.
func (c *DockController) Arrangement() DockArrangement {
	var a DockArrangement
	if c.root == nil {
		return a
	}
	a.Root = c.export(c.root)
	if m := c.maximized; m != nil && c.root.contains(m) && m.active != "" {
		a.Maximized = m.active
	}
	a.Closed = slices.Clone(c.closed)
	for _, h := range c.hidden {
		a.Hidden = append(a.Hidden, DockHidden{ID: h.id, Side: h.side, Size: h.size, Origin: slices.Clone(h.origin), Share: h.share})
	}
	return a
}

// SetArrangement puts the panels where a says, as restored from a saved
// layout. Panels a names that the layout does not have are dropped at the next
// build, and panels the layout has that a does not name go to the current
// group, unless a.Closed names them. The zero value puts every panel in one
// group.
//
// What stays in place keeps its state: a hidden panel that slid out stays out
// if a hides it at the same edge, and a group that keeps its selected panel
// keeps its tab strip and stays the current group. A panel that moves follows
// a: one that slid out goes in if a docks it or hides it at another edge.
func (c *DockController) SetArrangement(a DockArrangement) {
	old, current, fly := c.root, c.current, c.flyout
	var side DockZone
	if i := c.hiddenIndex(fly); i >= 0 {
		side = c.hidden[i].side
	}
	c.load(a)
	if old != nil {
		c.keepGroups(old, current)
	}
	if i := c.hiddenIndex(fly); i >= 0 && c.hidden[i].side == side {
		c.flyout = fly
	}
	c.refresh()
}

// keepGroups gives each group of the tree the identity of the group of old
// that held its selected panel, or else the first of its panels that one
// held, so that its tab strip keeps its element; a group that takes that of
// current becomes the current group.
func (c *DockController) keepGroups(old, current *dockNode) {
	taken := make(map[int]bool)
	c.root.eachGroup(func(g *dockNode) {
		if o := groupIn(old, g.active); o != nil && !taken[o.id] {
			c.keepGroup(g, o, current, taken)
			return
		}
		for _, id := range g.panels {
			if o := groupIn(old, id); o != nil && !taken[o.id] {
				c.keepGroup(g, o, current, taken)
				return
			}
		}
	})
}

// keepGroup gives g the identity of o, the group it was.
func (c *DockController) keepGroup(g, o, current *dockNode, taken map[int]bool) {
	taken[o.id] = true
	g.id = o.id
	if o == current {
		c.current = g
	}
}

// Reset puts the panels back where DockLayout.Default says, or all in one
// group without one, and opens the closed ones.
func (c *DockController) Reset() {
	c.root, c.maximized, c.current, c.closed = nil, nil, nil, nil
	c.hidden, c.flyout = nil, ""
	c.refresh()
}

// Activate shows the panel with id: it selects its tab, a closed panel opens
// again in the current group, and a hidden one slides out as Show does.
//
// Like the other methods that take the ID of a panel, it works on a panel
// added to DockLayout.Panels in the same frame, before the layout builds with
// it: the panel goes to the current group, as the build would put it, and
// then the method acts on it. A panel the layout does not have at its next
// build leaves the arrangement then. Before the layout first shows, the
// methods do nothing.
func (c *DockController) Activate(id string) {
	if !c.adopt(id) {
		return
	}
	c.activate(id)
	c.refresh()
}

// Close closes the panel with id, as its close button does, whether or not it
// is closable. It stays closed until Activate. It does not ask
// DockLayout.OnCloseRequest, which is how a close that the request held back
// goes ahead once the user agrees, and reports neither OnClose nor OnChange.
func (c *DockController) Close(id string) {
	if !c.adopt(id) {
		return
	}
	c.close(id)
	c.refresh()
}

// Move moves the panel with id to the group that holds the panel target: into
// it as the last tab with DockCenter, or beside it on the side zone names. An
// empty target is the whole layout, at whose edge the panel docks; with
// DockCenter it goes to the current group. A hidden panel docks there too. A
// panel added to the layout in the same frame goes to the current group
// first, as Activate describes, but a target must be in the arrangement
// already: a panel of the last build, or one that an earlier call took in. A
// target that is not leaves things as they are.
func (c *DockController) Move(id, target string, zone DockZone) {
	if !c.adopt(id) {
		return
	}
	var to *dockNode
	switch {
	case target != "":
		if to = c.groupOf(target); to == nil {
			return
		}
	case zone == DockCenter:
		to = c.currentGroup()
	}
	c.move(id, to, zone, -1)
	c.refresh()
}

// Maximize makes the group that holds the panel with id fill the layout,
// hiding the others, or, with an empty id, shows them all again.
func (c *DockController) Maximize(id string) {
	if id != "" && !c.adopt(id) {
		return
	}
	c.maximized = nil
	if id != "" {
		c.maximized = c.groupOf(id)
	}
	c.refresh()
}

// adopt readies the controller for a change to the panel id, and reports
// whether it can make one: the tree is there, loaded from the layout that
// shows the controller if Reset dropped it, and holds id. A panel the tree
// does not know, which the layout has been given since its last build, goes
// to the current group, where that build would put it; sync drops it again if
// the layout does not have it.
func (c *DockController) adopt(id string) bool {
	if c.root == nil && c.view != nil {
		w := c.view.Widget()
		ids := make([]string, len(w.Panels))
		for i := range w.Panels {
			ids[i] = w.Panels[i].ID
		}
		c.sync(ids, &w.Default)
	}
	if c.root == nil || id == "" {
		return false
	}
	if c.groupOf(id) == nil && c.hiddenIndex(id) < 0 && !slices.Contains(c.closed, id) {
		g := c.currentGroup()
		g.panels = append(g.panels, id)
		g.fixActive(0)
	}
	return true
}

// refresh shows a change.
func (c *DockController) refresh() {
	if c.view != nil {
		c.view.rebuild()
	}
}

// newNode returns a node with a new id.
func (c *DockController) newNode() *dockNode {
	c.nextID++
	return &dockNode{id: c.nextID}
}

// load replaces the tree with that of a.
func (c *DockController) load(a DockArrangement) {
	c.maximized, c.current = nil, nil
	c.closed = slices.Clone(a.Closed)
	c.hidden, c.flyout = nil, ""
	for _, h := range a.Hidden {
		c.hidden = append(c.hidden, dockHidden{
			id: h.ID, side: h.Side.side(), size: max(h.Size, 0), origin: slices.Clone(h.Origin), share: h.Share,
		})
	}
	if a.empty() {
		c.root = c.newNode()
	} else {
		c.root = c.tidy(c.build(&a.Root))
		c.root.parent = nil
	}
	if a.Maximized != "" {
		c.maximized = c.groupOf(a.Maximized)
	}
}

// build returns the node of n and those below it. Duplicates are left to sync.
func (c *DockController) build(n *DockNode) *dockNode {
	out := c.newNode()
	out.axis = n.Axis
	if len(n.Children) == 0 {
		out.panels = slices.Clone(n.Panels)
		out.active = n.Active
		return out
	}
	out.children = make([]*dockNode, len(n.Children))
	out.weights = make([]float32, len(n.Children))
	even := len(n.Weights) != len(n.Children)
	for i := range n.Children {
		out.children[i] = c.build(&n.Children[i])
		out.children[i].parent = out
		if !even {
			out.weights[i] = n.Weights[i]
		}
	}
	normalize(out.weights)
	return out
}

// export returns the plain value of n.
func (c *DockController) export(n *dockNode) DockNode {
	if n.isGroup() {
		return DockNode{Panels: slices.Clone(n.panels), Active: n.active}
	}
	out := DockNode{Axis: n.axis, Children: make([]DockNode, len(n.children)), Weights: slices.Clone(n.weights)}
	for i, ch := range n.children {
		out.Children[i] = c.export(ch)
	}
	return out
}

// sync makes the tree hold exactly the panels with ids, without duplicates:
// it loads def, or one group, the first time; drops the panels that went;
// and puts the new ones in the current group.
func (c *DockController) sync(ids []string, def *DockArrangement) {
	if c.root == nil {
		c.load(*def)
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	seen := make(map[string]bool, len(ids))
	c.root.eachGroup(func(g *dockNode) {
		kept := g.panels[:0]
		for _, id := range g.panels {
			if known[id] && !seen[id] {
				seen[id] = true
				kept = append(kept, id)
			}
		}
		clear(g.panels[len(kept):])
		g.panels = kept
		g.fixActive(0)
	})
	closed := c.closed[:0]
	for _, id := range c.closed {
		if known[id] && !seen[id] {
			seen[id] = true
			closed = append(closed, id)
		}
	}
	clear(c.closed[len(closed):])
	c.closed = closed
	hidden := c.hidden[:0]
	for _, h := range c.hidden {
		if known[h.id] && !seen[h.id] {
			seen[h.id] = true
			hidden = append(hidden, h)
		}
	}
	clear(c.hidden[len(hidden):])
	c.hidden = hidden
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			g := c.currentGroup()
			g.panels = append(g.panels, id)
			g.fixActive(0)
		}
	}
	c.retidy()
}

// retidy tidies the tree and forgets the groups that left it.
func (c *DockController) retidy() {
	c.root = c.tidy(c.root)
	c.root.parent = nil
	if c.maximized != nil && (!c.root.contains(c.maximized) || len(c.maximized.panels) == 0 || c.root == c.maximized) {
		c.maximized = nil
	}
	if c.current != nil && !c.root.contains(c.current) {
		c.current = nil
	}
	if c.flyout != "" && c.hiddenIndex(c.flyout) < 0 {
		c.flyout = ""
	}
}

// tidy returns n with empty groups taken out, splits of one child replaced
// by the child, and splits within a split along the same axis spliced into
// it. The node returned has no parent set. An empty tree is an empty group.
func (c *DockController) tidy(n *dockNode) *dockNode {
	if n.isGroup() {
		return n
	}
	kids := make([]*dockNode, 0, len(n.children))
	weights := make([]float32, 0, len(n.children))
	for i, ch := range n.children {
		ch = c.tidy(ch)
		w := n.weights[i]
		switch {
		case ch.isGroup() && len(ch.panels) == 0:
			continue
		case !ch.isGroup() && ch.axis == n.axis:
			for j, g := range ch.children {
				kids = append(kids, g)
				weights = append(weights, w*ch.weights[j])
			}
			continue
		}
		kids = append(kids, ch)
		weights = append(weights, w)
	}
	switch len(kids) {
	case 0:
		return c.newNode()
	case 1:
		kids[0].parent = nil
		return kids[0]
	}
	normalize(weights)
	n.children, n.weights = kids, weights
	for _, ch := range kids {
		ch.parent = n
	}
	return n
}

// groupOf returns the group that holds the panel id, or nil.
func (c *DockController) groupOf(id string) *dockNode { return groupIn(c.root, id) }

// groupIn returns the group below root that holds the panel id, or nil.
func groupIn(root *dockNode, id string) *dockNode {
	var out *dockNode
	if root != nil && id != "" {
		root.eachGroup(func(g *dockNode) {
			if out == nil && g.indexOf(id) >= 0 {
				out = g
			}
		})
	}
	return out
}

// groupByID returns the group whose node has id, or nil.
func (c *DockController) groupByID(id int) *dockNode {
	var out *dockNode
	if c.root != nil {
		c.root.eachGroup(func(g *dockNode) {
			if g.id == id {
				out = g
			}
		})
	}
	return out
}

// firstGroup returns the first group of the tree.
func (c *DockController) firstGroup() *dockNode {
	n := c.root
	for !n.isGroup() {
		n = n.children[0]
	}
	return n
}

// currentGroup returns the group that new panels go to.
func (c *DockController) currentGroup() *dockNode {
	if c.current != nil && c.root.contains(c.current) && c.current.isGroup() {
		return c.current
	}
	return c.firstGroup()
}

// activate selects the panel id, opening it in the current group if it was
// closed.
func (c *DockController) activate(id string) {
	if c.hiddenIndex(id) >= 0 {
		c.show(id)
		return
	}
	if i := slices.Index(c.closed, id); i >= 0 {
		c.closed = slices.Delete(c.closed, i, i+1)
		if c.root != nil {
			g := c.currentGroup()
			g.panels = append(g.panels, id)
		}
	}
	if g := c.groupOf(id); g != nil {
		g.active = id
		c.current = g
		if c.maximized != nil && c.maximized != g {
			c.maximized = nil
		}
	}
}

// close takes the panel id out of the tree, and keeps it among the closed
// ones. It reports whether the panel was there.
func (c *DockController) close(id string) bool {
	if !c.unhide(id) && c.take(id) == nil {
		return false
	}
	c.closed = append(c.closed, id)
	c.retidy()
	return true
}

// take takes the panel id out of its group, which keeps its place in the tree
// even if it is empty, and returns the group, or nil.
func (c *DockController) take(id string) *dockNode {
	g := c.groupOf(id)
	if g == nil {
		return nil
	}
	i := g.indexOf(id)
	g.panels = slices.Delete(g.panels, i, i+1)
	if g.active == id {
		g.active = ""
	}
	g.fixActive(i)
	return g
}

// reorder moves the panel at from to the index to, within g.
func (c *DockController) reorder(g *dockNode, from, to int) {
	if from < 0 || from >= len(g.panels) || to < 0 || to >= len(g.panels) || from == to {
		return
	}
	id := g.panels[from]
	g.panels = slices.Insert(slices.Delete(g.panels, from, from+1), to, id)
}

// canMove reports whether moving the panel id to the zone of the group to,
// or of the whole layout for nil, changes anything.
func (c *DockController) canMove(id string, to *dockNode, zone DockZone) bool {
	from := c.groupOf(id)
	if from == nil {
		// A hidden panel goes anywhere but the middle of nothing.
		return c.hiddenIndex(id) >= 0 && (to != nil || zone != DockCenter)
	}
	alone := len(from.panels) == 1
	switch to {
	case nil:
		// The edge of the whole layout, unless the layout is this panel.
		return zone != DockCenter && (!alone || c.root != from)
	case from:
		return zone != DockCenter && !alone
	}
	return true
}

// move moves the panel id to the group to at index, -1 for the end, with
// DockCenter, or beside it on the side zone names, or, for nil, at that edge
// of the whole layout. The panel becomes the active one of its group.
func (c *DockController) move(id string, to *dockNode, zone DockZone, index int) {
	if !c.canMove(id, to, zone) {
		return
	}
	if !c.unhide(id) {
		c.take(id)
	}
	c.maximized = nil
	// The group the panel left goes first, if it is empty, so that its room
	// does not count in the share of the new group.
	c.retidy()
	if zone == DockCenter {
		if index < 0 || index > len(to.panels) {
			index = len(to.panels)
		}
		to.panels = slices.Insert(to.panels, index, id)
		to.active = id
		c.current = to
		c.retidy()
		return
	}
	g := c.newNode()
	g.panels, g.active = []string{id}, id
	if to == nil {
		c.dockAt(c.root, g, zone, dockRootShare)
	} else {
		c.dockAt(to, g, zone, dockGroupShare)
	}
	c.current = g
	c.retidy()
}

// dockAt puts the group g beside the node at, on the side zone names, with
// share of the room at has. Beside a node whose parent splits along the same
// axis, g joins the parent; otherwise a new split takes the place of at.
func (c *DockController) dockAt(at, g *dockNode, zone DockZone, share float32) {
	axis, first := zone.axis()
	if p := at.parent; p != nil && p.axis == axis {
		i := slices.Index(p.children, at)
		w := p.weights[i]
		p.weights[i] = w * (1 - share)
		if !first {
			i++
		}
		p.children = slices.Insert(p.children, i, g)
		p.weights = slices.Insert(p.weights, i, w*share)
		g.parent = p
		return
	}
	if !at.isGroup() && at.axis == axis && at.parent == nil {
		// The edge of a root that splits along the same axis.
		for i := range at.weights {
			at.weights[i] *= 1 - share
		}
		i := 0
		if !first {
			i = len(at.children)
		}
		at.children = slices.Insert(at.children, i, g)
		at.weights = slices.Insert(at.weights, i, share)
		g.parent = at
		return
	}
	split := c.newNode()
	split.axis = axis
	parent := at.parent
	if first {
		split.children, split.weights = []*dockNode{g, at}, []float32{share, 1 - share}
	} else {
		split.children, split.weights = []*dockNode{at, g}, []float32{1 - share, share}
	}
	c.replace(parent, at, split)
	at.parent, g.parent = split, split
}

// replace puts n where old is below parent, or at the root for nil.
func (c *DockController) replace(parent, old, n *dockNode) {
	n.parent = parent
	if parent == nil {
		c.root = n
		return
	}
	parent.children[slices.Index(parent.children, old)] = n
}

// neighbor returns the group next to g on the side zone names, by where they
// were at the last layout: the nearest one that overlaps it across, or nil.
func (c *DockController) neighbor(g *dockNode, zone DockZone) *dockNode {
	const slack = 2 * dividerThickness
	var best *dockNode
	var bestGap, bestOverlap float32
	r := g.rect
	c.root.eachGroup(func(o *dockNode) {
		if o == g {
			return
		}
		q := o.rect
		var gap, overlap float32
		switch zone {
		case DockLeft:
			gap, overlap = r.X-(q.X+q.W), min(r.Y+r.H, q.Y+q.H)-max(r.Y, q.Y)
		case DockRight:
			gap, overlap = q.X-(r.X+r.W), min(r.Y+r.H, q.Y+q.H)-max(r.Y, q.Y)
		case DockTop:
			gap, overlap = r.Y-(q.Y+q.H), min(r.X+r.W, q.X+q.W)-max(r.X, q.X)
		case DockBottom:
			gap, overlap = q.Y-(r.Y+r.H), min(r.X+r.W, q.X+q.W)-max(r.X, q.X)
		default:
			return
		}
		if gap < -slack || overlap <= 0 {
			return
		}
		if best == nil || gap < bestGap || (gap == bestGap && overlap > bestOverlap) {
			best, bestGap, bestOverlap = o, gap, overlap
		}
	})
	return best
}
