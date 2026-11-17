package collection

import (
	"fmt"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func TestTouch_TapPanAndPinch(t *testing.T) {
	var taps int
	var pinch float32 = 1
	rows := List{ItemCount: 100, ItemExtent: 40, ItemBuilder: func(i int) core.Widget {
		return control.Button{Label: fmt.Sprintf("row %d", i), OnPress: func() { taps++ }}
	}}
	tree := wtest.TextTree(t, interact.GestureDetector{OnPinch: func(f float32, _ geom.Offset) { pinch *= f }, Child: rows})
	wtest.Ops(tree, wtest.Screen)

	tree.DispatchPointer(finger(core.PointerDown, 1, 40, 20))
	tree.DispatchPointer(finger(core.PointerMove, 1, 42, 22))
	tree.DispatchPointer(finger(core.PointerUp, 1, 42, 22))
	if taps != 1 {
		t.Fatalf("a finger that barely moves taps: %d", taps)
	}

	before := firstRow(t, tree)
	tree.DispatchPointer(finger(core.PointerDown, 1, 40, 200))
	for y := float32(190); y >= 40; y -= 10 {
		tree.DispatchPointer(finger(core.PointerMove, 1, 40, y))
	}
	tree.DispatchPointer(finger(core.PointerUp, 1, 40, 40))
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if taps != 1 {
		t.Error("a pan must not tap the row it started on")
	}
	if after := firstRow(t, tree); after <= before {
		t.Errorf("a finger moving up must scroll toward the end: first row %d, then %d", before, after)
	}

	tree.DispatchPointer(finger(core.PointerDown, 1, 100, 100))
	tree.DispatchPointer(finger(core.PointerDown, 2, 200, 100))
	tree.DispatchPointer(finger(core.PointerMove, 2, 300, 100))
	tree.DispatchPointer(finger(core.PointerUp, 1, 100, 100))
	tree.DispatchPointer(finger(core.PointerUp, 2, 300, 100))
	if pinch < 1.99 || pinch > 2.01 {
		t.Errorf("two fingers moving to twice the distance pinch by 2, got %v", pinch)
	}
	if taps != 1 {
		t.Error("a pinch must not tap")
	}
}

// firstRow returns the number of the first row a reader sees.
func firstRow(t *testing.T, tree *core.Tree) int {
	t.Helper()
	root, _ := tree.SemanticsTree()
	first := -1
	root.Walk(func(n *semantics.Node) bool {
		var i int
		if _, err := fmt.Sscanf(n.Label, "row %d", &i); err == nil && first < 0 {
			first = i
			return false
		}
		return true
	})
	return first
}
func finger(kind core.PointerKind, id uint32, x, y float32) core.PointerEvent {
	return core.PointerEvent{Kind: kind, ID: id, Pos: geom.Offset{X: x, Y: y}, Device: input.DeviceTouch}
}
