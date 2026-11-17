package semantics

import "slices"

// Change is a set of the parts of a node that changed between two trees.
type Change uint16

// The parts.
const (
	ChangedRole Change = 1 << iota
	ChangedLabel
	ChangedValue
	ChangedHint
	// ChangedState means the State or the Sort of the node changed.
	ChangedState
	ChangedActions
	ChangedRange
	ChangedRect
	// ChangedText means the text of a field changed.
	ChangedText
	// ChangedSelection means the selection or caret of a field moved.
	ChangedSelection
	// ChangedChildren means the IDs of the children, or their order, changed.
	ChangedChildren
	ChangedLive
	// ChangedPosition means the size of a grid, or the place of a row or cell
	// in it, changed.
	ChangedPosition
)

// Changed is a node that is in both trees, with what differs.
type Changed struct {
	Node *Node
	What Change
}

// Diff is the difference between two trees, from Compare.
type Diff struct {
	// Added holds the nodes of the new tree that the old one lacked, parents
	// first.
	Added []*Node
	// Removed holds the IDs of the nodes of the old tree that the new one lacks.
	Removed []ID
	// Changed holds the nodes of both trees whose description changed.
	Changed []Changed
	// FocusMoved means the focus is on another node, or on none.
	FocusMoved bool
}

// Reset empties d, keeping its storage.
func (d *Diff) Reset() {
	clear(d.Added)
	clear(d.Changed)
	d.Added, d.Removed, d.Changed = d.Added[:0], d.Removed[:0], d.Changed[:0]
	d.FocusMoved = false
}

// Empty reports whether nothing changed.
func (d *Diff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 && !d.FocusMoved
}

// Compare writes into d what turns the tree before, with the focus on oldFocus,
// into after with the focus on newFocus. Either tree may be nil. Nodes are
// matched by ID.
func Compare(before, after *Node, oldFocus, newFocus ID, d *Diff) {
	d.Reset()
	d.FocusMoved = oldFocus != newFocus
	was := index(before)
	is := index(after)
	if after != nil {
		after.Walk(func(n *Node) bool {
			o, ok := was[n.ID]
			if !ok {
				d.Added = append(d.Added, n)
				return true
			}
			if what := compareNode(o, n); what != 0 {
				d.Changed = append(d.Changed, Changed{Node: n, What: what})
			}
			return true
		})
	}
	if before != nil {
		before.Walk(func(n *Node) bool {
			if _, ok := is[n.ID]; !ok {
				d.Removed = append(d.Removed, n.ID)
			}
			return true
		})
	}
}

func index(root *Node) map[ID]*Node {
	m := make(map[ID]*Node)
	if root != nil {
		root.Walk(func(n *Node) bool {
			m[n.ID] = n
			return true
		})
	}
	return m
}

func compareNode(a, b *Node) Change {
	var c Change
	if a.Role != b.Role {
		c |= ChangedRole
	}
	if a.Label != b.Label {
		c |= ChangedLabel
	}
	if a.Value != b.Value {
		c |= ChangedValue
	}
	if a.Hint != b.Hint || a.Level != b.Level || a.PosInSet != b.PosInSet || a.SetSize != b.SetSize {
		c |= ChangedHint
	}
	if a.State != b.State || a.Sort != b.Sort {
		c |= ChangedState
	}
	if a.Actions != b.Actions {
		c |= ChangedActions
	}
	if a.Range != b.Range {
		c |= ChangedRange
	}
	if a.Rect != b.Rect {
		c |= ChangedRect
	}
	if a.Live != b.Live {
		c |= ChangedLive
	}
	if a.RowCount != b.RowCount || a.ColumnCount != b.ColumnCount || a.RowIndex != b.RowIndex || a.ColumnIndex != b.ColumnIndex {
		c |= ChangedPosition
	}
	c |= compareText(a.Text, b.Text)
	if !slices.EqualFunc(a.Children, b.Children, func(x, y *Node) bool { return x.ID == y.ID }) {
		c |= ChangedChildren
	}
	return c
}

func compareText(a, b *TextInfo) Change {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil || b == nil:
		return ChangedText | ChangedSelection
	}
	var c Change
	if a.Text != b.Text || !slices.Equal(a.LineStarts, b.LineStarts) {
		c |= ChangedText
	}
	if a.SelectionStart != b.SelectionStart || a.SelectionEnd != b.SelectionEnd {
		c |= ChangedSelection
	}
	if !slices.Equal(a.LineRects, b.LineRects) {
		c |= ChangedRect
	}
	return c
}
