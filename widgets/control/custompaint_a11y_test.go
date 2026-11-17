package control

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// TestCustomPaint_ItemsOverTheCanvas follows the godoc of CustomPaint: one
// Semantics with a Focus per drawn item, positioned over the canvas.
func TestCustomPaint_ItemsOverTheCanvas(t *testing.T) {
	taps, spoken := 0, 0
	item := func(x float32, label string) core.Widget {
		return basic.Positioned{Left: x, Top: 20, Width: 40, Height: 40, Child: interact.Semantics{
			Role: semantics.RoleButton, Label: label, OnTap: func() { spoken++ },
			Child: interact.Focus{Enabled: true, Child: basic.SizedBox{}},
		}}
	}
	tree := wtest.A11yTree(t, basic.Stack{Children: []core.Widget{
		interact.GestureDetector{
			OnTapAt: func(layout.Tap) { taps++ },
			Child:   CustomPaint{Width: 200, Height: 80, Painter: layout.PainterFunc(func(*paint.Canvas, geom.Size) {})},
		},
		item(20, "Mon"),
		item(80, "Tue"),
	}})
	root, _ := tree.SemanticsTree()
	for _, label := range []string{"Mon", "Tue"} {
		n := wtest.ByLabel(t, root, label)
		if n.Role != semantics.RoleButton || n.Actions&semantics.ActionTap == 0 {
			t.Errorf("%s: role %v, actions %v", label, n.Role, n.Actions)
		}
	}
	wtest.Tap(tree, geom.Offset{X: 40, Y: 40})
	if taps != 1 {
		t.Errorf("a tap on an item reached the canvas %d times, want 1", taps)
	}
	var focused []string
	for range 2 {
		tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		focused = append(focused, wtest.FocusedLabel(tree))
	}
	if focused[0] != "Mon" || focused[1] != "Tue" {
		t.Errorf("Tab focused %v, want Mon then Tue", focused)
	}
}
