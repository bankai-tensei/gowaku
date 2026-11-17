//go:build gowakudebug

package inspect

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// fakeTarget is a window under inspection without a window: a tree laid out
// and painted by hand.
type fakeTarget struct {
	title   string
	tree    *core.Tree
	size    geom.Size
	redraws int
}

func newFakeTarget(title string, root core.Widget, size geom.Size) *fakeTarget {
	t := &fakeTarget{title: title, tree: core.NewTree(root), size: size}
	t.tree.Build()
	t.tree.Layout(geom.Tight(size))
	return t
}

func (t *fakeTarget) Title() string    { return t.title }
func (t *fakeTarget) Tree() *core.Tree { return t.tree }
func (t *fakeTarget) Size() geom.Size  { return t.size }
func (t *fakeTarget) Redraw()          { t.redraws++ }

// fakeHost serves targets.
type fakeHost struct {
	targets   []Target
	activated []Target
}

func (h *fakeHost) Targets() []Target { return h.targets }
func (h *fakeHost) Activate(t Target) { h.activated = append(h.activated, t) }
