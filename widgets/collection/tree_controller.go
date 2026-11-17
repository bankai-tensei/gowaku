package collection

// TreeController holds what a Tree shows open, which nodes are selected,
// and which one has the focus. The zero value is ready: nothing open, nothing
// selected. Its methods are for callbacks and InitState, not for Build, as
// they rebuild the tree.
type TreeController[K comparable] struct {
	expanded map[K]struct{}
	selected map[K]struct{}
	// order holds the selected keys in the order they were selected.
	order     []K
	focus     K
	hasFocus  bool
	anchor    K
	hasAnchor bool
	// generation counts the changes of expanded, which the tree reads again.
	generation uint64
	// reveal asks the tree to scroll the focused node into view.
	reveal bool
	// rename is the node to start editing, if renaming.
	rename   K
	renaming bool
	// listener is the tree that shows the controller.
	listener func()
}

func (c *TreeController[K]) notify() {
	if c.listener != nil {
		c.listener()
	}
}

// Expanded reports whether node is open.
func (c *TreeController[K]) Expanded(node K) bool {
	_, ok := c.expanded[node]
	return ok
}

// SetExpanded opens or closes node. Opening a node whose parent is closed
// shows nothing until the parent opens too.
func (c *TreeController[K]) SetExpanded(node K, open bool) {
	if c.Expanded(node) == open {
		return
	}
	if open {
		if c.expanded == nil {
			c.expanded = make(map[K]struct{})
		}
		c.expanded[node] = struct{}{}
	} else {
		delete(c.expanded, node)
	}
	c.generation++
	c.notify()
}

// Expand opens node.
func (c *TreeController[K]) Expand(node K) { c.SetExpanded(node, true) }

// Collapse closes node.
func (c *TreeController[K]) Collapse(node K) { c.SetExpanded(node, false) }

// Selected reports whether node is selected.
func (c *TreeController[K]) Selected(node K) bool {
	_, ok := c.selected[node]
	return ok
}

// Selection returns the selected nodes in the order they were selected. A
// selected node stays selected while its parent is closed.
func (c *TreeController[K]) Selection() []K { return append([]K(nil), c.order...) }

// SetSelected adds node to the selection or takes it out.
func (c *TreeController[K]) SetSelected(node K, on bool) {
	if c.setSelected(node, on) {
		c.notify()
	}
}

func (c *TreeController[K]) setSelected(node K, on bool) bool {
	if c.Selected(node) == on {
		return false
	}
	if on {
		if c.selected == nil {
			c.selected = make(map[K]struct{})
		}
		c.selected[node] = struct{}{}
		c.order = append(c.order, node)
		return true
	}
	delete(c.selected, node)
	for i, k := range c.order {
		if k == node {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	return true
}

// Select makes node the only selected node, gives it the focus and shows it.
func (c *TreeController[K]) Select(node K) {
	c.clear()
	c.setSelected(node, true)
	c.focus, c.hasFocus, c.reveal = node, true, true
	c.anchor, c.hasAnchor = node, true
	c.notify()
}

// ClearSelection selects nothing.
func (c *TreeController[K]) ClearSelection() {
	if len(c.order) > 0 {
		c.clear()
		c.notify()
	}
}

func (c *TreeController[K]) clear() {
	clear(c.selected)
	c.order = c.order[:0]
}

// Focused returns the node the arrows move from, if there is one.
func (c *TreeController[K]) Focused() (K, bool) { return c.focus, c.hasFocus }

// Focus moves the focus of the tree to node, without selecting it, and
// scrolls it into view once it shows. It does not take the keyboard from
// another widget.
func (c *TreeController[K]) Focus(node K) {
	c.focus, c.hasFocus, c.reveal = node, true, true
	c.notify()
}

// Rename starts editing the label of node, if the tree has OnRename and the
// node shows: after a Tree creates a node, say.
func (c *TreeController[K]) Rename(node K) {
	c.focus, c.hasFocus, c.reveal = node, true, true
	c.rename, c.renaming = node, true
	c.notify()
}
