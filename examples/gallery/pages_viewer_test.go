package main

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// TestPageViewer_FitsAndPressesThroughTheZoom renders the page, which shows
// the whole drawing at first, and presses a button on the drawing where the
// zoom shows it.
func TestPageViewer_FitsAndPressesThroughTheZoom(t *testing.T) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), pageViewer()))
	defer tree.Dispose()
	size := geom.Size{W: 860, H: 620}
	now := time.Second
	frame := func() {
		testutil.RenderTree(tree, size)
		for ; tree.Ticking(); now += 8 * time.Millisecond {
			tree.Tick(now)
			testutil.RenderTree(tree, size)
		}
	}
	frame()
	node := func(label string) *semantics.Node {
		root, _ := tree.SemanticsTree()
		var found *semantics.Node
		root.Walk(func(n *semantics.Node) bool {
			if n.Label == label {
				found = n
			}
			return found == nil
		})
		if found == nil {
			t.Fatalf("no node %q", label)
		}
		return found
	}
	viewer := node("図面")
	if viewer.Value == "100%" || node("倍率 "+viewer.Value) == nil {
		t.Errorf("the page opened at %s, want the whole drawing, with the label in step", viewer.Value)
	}
	// The semantics tree follows the zoom, so the rectangle of the button is
	// where it shows.
	button := node("開く").Rect
	at := geom.Offset{X: button.X + button.W/2, Y: button.Y + button.H/2}
	for _, kind := range []core.PointerKind{core.PointerMove, core.PointerDown, core.PointerUp} {
		tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at})
	}
	frame()
	node("「開く」を押しました。")
}
