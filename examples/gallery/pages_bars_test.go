package main

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// The page of the bars builds, closes a bar, and opens its palette.
func TestPageBars(t *testing.T) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), overlay.Overlay{Child: pageBars()}))
	t.Cleanup(tree.Dispose)
	size := geom.Size{W: 860, H: 620}
	now := time.Second
	frames := func(n int) {
		for range n {
			now += 16 * time.Millisecond
			tree.Tick(now)
			testutil.RenderTree(tree, size)
		}
	}
	frames(1)
	count := func(role semantics.Role, label string) int {
		root, _ := tree.SemanticsTree()
		n := 0
		root.Walk(func(node *semantics.Node) bool {
			if node.Role == role && (label == "" || node.Label == label) {
				n++
			}
			return true
		})
		return n
	}
	if got := count(semantics.RoleButton, "Close"); got != 4 {
		t.Errorf("%d close buttons, want one per bar", got)
	}
	if count(semantics.RoleGroup, "Status bar") != 1 {
		t.Error("no status bar")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyP, Modifiers: input.ModControl | input.ModShift})
	frames(40)
	if got := count(semantics.RoleListItem, ""); got != 7 {
		t.Errorf("the palette lists %d commands, want 7", got)
	}
	tree.DispatchText("閉じる")
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	frames(60)
	if got := count(semantics.RoleButton, "Close"); got != 0 {
		t.Errorf("すべての通知を閉じる left %d bars", got)
	}
}
