package main

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The page of the number, range and token inputs builds and shows each of
// them, without a GPU.
func TestPageInputs_Builds(t *testing.T) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), pageInputs()))
	defer tree.Dispose()
	if out := testutil.RenderTree(tree, geom.Size{W: 1000, H: 900}); out == "" {
		t.Fatal("the page paints nothing")
	}
	count := map[semantics.Role]int{}
	root, _ := tree.SemanticsTree()
	root.Walk(func(n *semantics.Node) bool {
		count[n.Role]++
		return true
	})
	if count[semantics.RoleStepper] != 4 || count[semantics.RoleSlider] != 6 || count[semantics.RoleList] != 3 {
		t.Errorf("got %v", count)
	}
}
