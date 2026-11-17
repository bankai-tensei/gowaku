package wtest

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/semantics"
)

// NodesOf appends to out every node under n with role.
func NodesOf(n *semantics.Node, role semantics.Role, out []*semantics.Node) []*semantics.Node {
	if n.Role == role {
		out = append(out, n)
	}
	for _, c := range n.Children {
		out = NodesOf(c, role, out)
	}
	return out
}

// Buttons appends to out every button under n.
func Buttons(n *semantics.Node, out []*semantics.Node) []*semantics.Node {
	return NodesOf(n, semantics.RoleButton, out)
}

// FindRole returns the first node under n with role, or nil.
func FindRole(n *semantics.Node, role semantics.Role) *semantics.Node {
	if n.Role == role {
		return n
	}
	for _, c := range n.Children {
		if got := FindRole(c, role); got != nil {
			return got
		}
	}
	return nil
}

// ByLabel returns the first node under root with label, failing the test
// when there is none.
func ByLabel(t *testing.T, root *semantics.Node, label string) *semantics.Node {
	t.Helper()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == label {
			found = n
			return false
		}
		return true
	})
	if found == nil {
		t.Fatalf("no node %q", label)
	}
	return found
}

// FocusedLabel returns the label of the node with the focus.
func FocusedLabel(tree *core.Tree) string {
	root, focus := tree.SemanticsTree()
	if n := root.Find(focus); n != nil {
		return n.Label
	}
	return ""
}

// RolesOf returns every node under n with role r.
func RolesOf(n *semantics.Node, r semantics.Role) []*semantics.Node {
	var out []*semantics.Node
	n.Walk(func(m *semantics.Node) bool {
		if m.Role == r {
			out = append(out, m)
		}
		return true
	})
	return out
}
