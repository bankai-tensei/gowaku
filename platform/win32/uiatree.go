package win32

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// snapshot is the tree of the last Update, indexed for the questions of UI
// Automation. It does not call the OS, so that it is tested on any system.
// The root has the ID zero and stands for the window.
type snapshot struct {
	root    *semantics.Node
	nodes   map[semantics.ID]*semantics.Node
	parents map[semantics.ID]semantics.ID
}

func newSnapshot(root *semantics.Node) snapshot {
	s := snapshot{
		root:    root,
		nodes:   make(map[semantics.ID]*semantics.Node),
		parents: make(map[semantics.ID]semantics.ID),
	}
	if root != nil {
		var walk func(n *semantics.Node)
		walk = func(n *semantics.Node) {
			for _, c := range n.Children {
				s.nodes[c.ID] = c
				s.parents[c.ID] = n.ID
				walk(c)
			}
		}
		walk(root)
	}
	return s
}

// node returns the node of id, the root for zero, or nil if the tree lacks
// it. The root itself is nil before the first Update.
func (s *snapshot) node(id semantics.ID) *semantics.Node {
	if id == 0 {
		return s.root
	}
	return s.nodes[id]
}

// has reports whether the tree holds id. The root is always there.
func (s *snapshot) has(id semantics.ID) bool {
	return id == 0 || s.nodes[id] != nil
}

// navigate answers IRawElementProviderFragment::Navigate from node id. The
// root has neither parent nor siblings: the window around it does.
func (s *snapshot) navigate(id semantics.ID, dir int32) (semantics.ID, bool) {
	n := s.node(id)
	if n == nil && id != 0 {
		return 0, false
	}
	switch dir {
	case NavigateDirection_Parent:
		if id == 0 {
			return 0, false
		}
		return s.parents[id], true
	case NavigateDirection_FirstChild, NavigateDirection_LastChild:
		if n == nil || len(n.Children) == 0 {
			return 0, false
		}
		if dir == NavigateDirection_FirstChild {
			return n.Children[0].ID, true
		}
		return n.Children[len(n.Children)-1].ID, true
	case NavigateDirection_NextSibling, NavigateDirection_PreviousSibling:
		if id == 0 {
			return 0, false
		}
		parent := s.node(s.parents[id])
		if parent == nil {
			return 0, false
		}
		for i, c := range parent.Children {
			if c.ID != id {
				continue
			}
			j := i + 1
			if dir == NavigateDirection_PreviousSibling {
				j = i - 1
			}
			if j < 0 || j >= len(parent.Children) {
				return 0, false
			}
			return parent.Children[j].ID, true
		}
	}
	return 0, false
}

// hitTest returns the deepest node whose box holds p, in logical pixels of
// the window, or zero for none. Of overlapping siblings the last, drawn on
// top, wins.
func (s *snapshot) hitTest(p geom.Offset) semantics.ID {
	var found semantics.ID
	n := s.root
	for n != nil {
		var next *semantics.Node
		for i := len(n.Children) - 1; i >= 0; i-- {
			if c := n.Children[i]; c.Rect.Contains(p) {
				next = c
				break
			}
		}
		if next != nil {
			found = next.ID
		}
		n = next
	}
	return found
}

// selectionContainer returns the nearest node above id that holds choices:
// the group of a radio button, the list of an item, the grid of a row or cell,
// the tree of a tree item, however deep the item is.
func (s *snapshot) selectionContainer(id semantics.ID) (semantics.ID, bool) {
	for {
		pid, ok := s.parents[id]
		if !ok || pid == 0 {
			return 0, false
		}
		if parent := s.node(pid); parent != nil && isSelectionContainer(parent) {
			return pid, true
		}
		id = pid
	}
}

// selection returns the selected choices of the container id, in the order of
// the tree. It does not look inside a choice, or inside another container.
func (s *snapshot) selection(id semantics.ID) []semantics.ID {
	n := s.node(id)
	if n == nil {
		return nil
	}
	var out []semantics.ID
	var walk func(n *semantics.Node)
	walk = func(n *semantics.Node) {
		for _, c := range n.Children {
			switch {
			case isSelectionItem(c):
				if c.State&semantics.Selected != 0 {
					out = append(out, c.ID)
				}
			case !isSelectionContainer(c):
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

// gridOf returns the grid above the node id.
func (s *snapshot) gridOf(id semantics.ID) (semantics.ID, bool) {
	for {
		pid, ok := s.parents[id]
		if !ok || pid == 0 {
			return 0, false
		}
		if parent := s.node(pid); parent != nil && parent.Role == semantics.RoleGrid {
			return pid, true
		}
		id = pid
	}
}

// gridSize returns the number of rows and columns of the grid id: those it
// states, as a grid that has nodes only for the rows in view does, else those
// it holds.
func (s *snapshot) gridSize(id semantics.ID) (rows, cols int) {
	g := s.node(id)
	if g == nil {
		return 0, 0
	}
	rows, cols = g.RowCount, g.ColumnCount
	if rows > 0 && cols > 0 {
		return rows, cols
	}
	var r, c int
	for _, row := range gridRows(g) {
		r = max(r, rowIndex(g, row)+1)
		for _, cell := range row.Children {
			if isGridItem(cell) {
				c = max(c, cellIndex(row, cell)+1)
			}
		}
	}
	if rows <= 0 {
		rows = r
	}
	if cols <= 0 {
		cols = c
	}
	return rows, cols
}

// gridPosition returns the row and column of the cell id in its grid, from 0.
func (s *snapshot) gridPosition(id semantics.ID) (row, col int, ok bool) {
	n := s.node(id)
	gid, found := s.gridOf(id)
	if n == nil || !found {
		return 0, 0, false
	}
	parent := s.node(s.parents[id])
	if parent == nil {
		return 0, 0, false
	}
	row = n.RowIndex - 1
	if row < 0 && parent.Role == semantics.RoleRow {
		row = rowIndex(s.node(gid), parent)
	}
	col = cellIndex(parent, n)
	return max(row, 0), col, true
}

// gridItem returns the cell of the grid id at row and col, from 0, if it has
// a node.
func (s *snapshot) gridItem(id semantics.ID, row, col int) (semantics.ID, bool) {
	g := s.node(id)
	if g == nil {
		return 0, false
	}
	for _, r := range gridRows(g) {
		if rowIndex(g, r) != row {
			continue
		}
		for _, c := range r.Children {
			if isGridItem(c) && cellIndex(r, c) == col {
				return c.ID, true
			}
		}
	}
	return 0, false
}

// columnHeaders returns the titles of the columns of the grid id, or only that
// of column col when col is not negative.
func (s *snapshot) columnHeaders(id semantics.ID, col int) []semantics.ID {
	g := s.node(id)
	if g == nil {
		return nil
	}
	var out []semantics.ID
	for _, r := range gridRows(g) {
		for _, c := range r.Children {
			if c.Role == semantics.RoleColumnHeader && (col < 0 || cellIndex(r, c) == col) {
				out = append(out, c.ID)
			}
		}
	}
	return out
}

// gridRows returns the rows of g: its children, or theirs where a group comes
// between.
func gridRows(g *semantics.Node) []*semantics.Node {
	var rows []*semantics.Node
	for _, c := range g.Children {
		switch {
		case c.Role == semantics.RoleRow:
			rows = append(rows, c)
		case len(c.Children) > 0 && c.Role == semantics.RoleNone:
			rows = append(rows, gridRows(c)...)
		}
	}
	return rows
}

// rowIndex returns the place of row in the grid g, from 0: the one it states,
// else its place among the rows.
func rowIndex(g, row *semantics.Node) int {
	if row.RowIndex > 0 {
		return row.RowIndex - 1
	}
	for i, r := range gridRows(g) {
		if r == row {
			return i
		}
	}
	return 0
}

// cellIndex returns the column of cell in row, from 0: the one it states, else
// its place among the cells.
func cellIndex(row, cell *semantics.Node) int {
	if cell.ColumnIndex > 0 {
		return cell.ColumnIndex - 1
	}
	i := 0
	for _, c := range row.Children {
		if c == cell {
			return i
		}
		if isGridItem(c) {
			i++
		}
	}
	return 0
}
