//go:build gowakudebug

package inspect

import (
	"fmt"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The functions here describe the three trees to a Tree: the children of a
// node, its label, and where the selection of the session sits in each.

// maxKeyLabel bounds the key a row of the element tree shows.
const maxKeyLabel = 24

// elementChildren returns the children of v.
func elementChildren(v core.ElementView) []core.ElementView {
	n := v.ChildCount()
	if n == 0 {
		return nil
	}
	out := make([]core.ElementView, n)
	for i := range out {
		out[i] = v.ChildAt(i)
	}
	return out
}

// elementLabel returns the type of the widget of v, with its key if it has one:
// `basic.Text #title`.
func elementLabel(v core.ElementView) string {
	name := TypeName(v.Widget())
	if k := v.Key(); k != nil {
		return name + "  #" + keyLabel(k)
	}
	return name
}

// keyLabel spells a key in a few characters.
func keyLabel(k core.Key) string {
	var s string
	if str, ok := k.(string); ok {
		s = str
	} else {
		s = fmt.Sprint(k)
	}
	return clip(s, maxKeyLabel)
}

// clip cuts s to n runes, with an ellipsis.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// kindLetter is the faint marker of the kind of an element.
func kindLetter(k core.ElementKind) string {
	switch k {
	case core.ElementStateful:
		return "F"
	case core.ElementInherited:
		return "I"
	case core.ElementRender:
		return "R"
	}
	return "S"
}

// objectChildren returns the children of ro.
func objectChildren(ro core.RenderObject) []core.RenderObject {
	n := ro.ChildCount()
	if n == 0 {
		return nil
	}
	out := make([]core.RenderObject, 0, n)
	for i := range n {
		if c := ro.ChildAt(i); c != nil {
			out = append(out, c)
		}
	}
	return out
}

// objectHasChildren reports whether ro has a child.
func objectHasChildren(ro core.RenderObject) bool {
	for i := range ro.ChildCount() {
		if ro.ChildAt(i) != nil {
			return true
		}
	}
	return false
}

// semKey names a node of the semantics tree across rebuilds of it: the ID of
// the object that opened it, and which of the nodes of that object it is, for
// an object that opens more than one.
type semKey struct {
	id  semantics.ID
	nth int32
}

// semIndex is a semantics tree with what a Tree needs to walk it by key.
type semIndex struct {
	root     *semantics.Node
	nodes    map[semKey]*semantics.Node
	parent   map[semKey]semKey
	children map[semKey][]semKey
	// byID is the first node of each ID.
	byID map[semantics.ID]semKey
}

// rootSem is the key of the root of the semantics tree, which no object opens.
var rootSem = semKey{}

// newSemIndex indexes the semantics tree of t.
func newSemIndex(t *core.Tree) *semIndex {
	x := &semIndex{
		root:     t.Semantics(),
		nodes:    map[semKey]*semantics.Node{},
		parent:   map[semKey]semKey{},
		children: map[semKey][]semKey{},
		byID:     map[semantics.ID]semKey{},
	}
	x.nodes[rootSem] = x.root
	x.add(rootSem, x.root)
	return x
}

func (x *semIndex) add(at semKey, n *semantics.Node) {
	if len(n.Children) == 0 {
		return
	}
	keys := make([]semKey, len(n.Children))
	for i, c := range n.Children {
		k := semKey{id: c.ID}
		for x.nodes[k] != nil {
			k.nth++
		}
		keys[i] = k
		x.nodes[k] = c
		x.parent[k] = at
		if _, ok := x.byID[c.ID]; !ok {
			x.byID[c.ID] = k
		}
		x.add(k, c)
	}
	x.children[at] = keys
}

// of returns the node that speaks for ro: the one ro opened, or else that of
// its nearest ancestor that opened one, which owner is.
func (x *semIndex) of(ro core.RenderObject) (k semKey, owner core.RenderObject, ok bool) {
	for ; ro != nil; ro = ro.Parent() {
		if id := core.SemanticsIDOf(ro); id != 0 {
			if k, ok := x.byID[id]; ok {
				return k, ro, true
			}
		}
	}
	return semKey{}, nil, false
}

// semLabel returns the role of n with its label and value:
// `button "Save"` or `slider "Volume" = 40`.
func semLabel(n *semantics.Node) string {
	var b strings.Builder
	b.WriteString(n.Role.String())
	if n.Label != "" {
		fmt.Fprintf(&b, " %q", clip(n.Label, 48))
	}
	if n.Value != "" {
		b.WriteString(" = " + clip(n.Value, 32))
	}
	return b.String()
}

// ownerOf returns the render object below root that opened the node of id, or
// nil. It walks the tree, which only a click asks for.
func ownerOf(root core.RenderObject, id semantics.ID) core.RenderObject {
	if root == nil || id == 0 {
		return nil
	}
	if core.SemanticsIDOf(root) == id {
		return root
	}
	for i := range root.ChildCount() {
		if found := ownerOf(root.ChildAt(i), id); found != nil {
			return found
		}
	}
	return nil
}

// expandDefault opens the top of a tree: every node along a chain of single
// children, and levels branching levels below the root, up to budget nodes.
func expandDefault[K comparable](expand func(K), children func(K) []K, node K, levels int, budget *int) {
	kids := children(node)
	if len(kids) == 0 || *budget <= 0 {
		return
	}
	expand(node)
	*budget--
	if len(kids) > 1 {
		levels--
	}
	if levels < 0 {
		return
	}
	for _, c := range kids {
		expandDefault(expand, children, c, levels, budget)
	}
}

// findNext returns the first node after from, in the order of the rows, whose
// label holds query, wrapping around to the root; from may be the zero K for a
// search from the top. ok is false when nothing matches.
func findNext[K comparable](roots []K, children func(K) []K, label func(K) string, from K, query string) (found K, ok bool) {
	var zero K
	query = strings.ToLower(query)
	passed := from == zero
	var first K
	var firstOK bool
	var walk func(n K) bool
	walk = func(n K) bool {
		if n != from && strings.Contains(strings.ToLower(label(n)), query) {
			if passed {
				found, ok = n, true
				return true
			}
			if !firstOK {
				first, firstOK = n, true
			}
		}
		if n == from {
			passed = true
		}
		for _, c := range children(n) {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for _, r := range roots {
		if walk(r) {
			return found, true
		}
	}
	if firstOK {
		return first, true
	}
	if from != zero && strings.Contains(strings.ToLower(label(from)), query) {
		return from, true
	}
	return zero, false
}
