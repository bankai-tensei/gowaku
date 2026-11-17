package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// The sparklines of the page sit in the cells of a real table, and move with
// their rows when the table sorts.
func TestPageVisualSparklineTable(t *testing.T) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), overlay.Overlay{Child: pageVisual()}))
	t.Cleanup(tree.Dispose)
	size := geom.Size{W: 1000, H: 2400}
	testutil.RenderTree(tree, size)
	grid := func() *semantics.Node {
		root, _ := tree.SemanticsTree()
		var found *semantics.Node
		root.Walk(func(n *semantics.Node) bool {
			if n.Role == semantics.RoleGrid && n.Label == "気温" {
				found = n
				return false
			}
			return true
		})
		if found == nil {
			t.Fatal("no table of sparklines")
		}
		return found
	}
	// Each row holds the line of its own place: the first image of the row
	// is named after the place in its first cell.
	check := func() []string {
		var places []string
		for _, row := range grid().Children[1:] {
			place := row.Children[0].Label
			places = append(places, place)
			var img string
			row.Walk(func(n *semantics.Node) bool {
				if n.Role == semantics.RoleImage {
					img = n.Label
					return false
				}
				return true
			})
			if !strings.HasPrefix(img, place+"の気温") {
				t.Errorf("the row of %s shows %q", place, img)
			}
		}
		return places
	}
	before := check()
	if len(before) != 5 || before[0] != "東京" {
		t.Fatalf("places %q", before)
	}
	// A click on the header of 現在 sorts by the temperature now.
	head := grid().Children[0].Children[2]
	if head.Label != "現在" {
		t.Fatalf("header %q", head.Label)
	}
	at := geom.Offset{X: head.Rect.X + head.Rect.W/2, Y: head.Rect.Y + head.Rect.H/2}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonPrimary, Time: 1000})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonPrimary, Time: 1000})
	testutil.RenderTree(tree, size)
	if after := check(); slices.Equal(after, before) {
		t.Errorf("the rows did not sort: %q", after)
	}
}
