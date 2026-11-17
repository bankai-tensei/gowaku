package collection

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// colorGrid is a PropertyGrid with one color property in a window with an
// overlay, and the colors it reported.
type colorGrid struct {
	tree  *core.Tree
	value geom.Color
	log   []geom.Color
	press int
}

var colorScreen = geom.Size{W: 500, H: 700}

func newColorGrid(t *testing.T, custom bool) *colorGrid {
	t.Helper()
	g := &colorGrid{value: geom.Hex(0x2f5fd0)}
	var state *wtest.SwapState
	g.tree = wtest.TextTree(t, overlay.Overlay{Child: wtest.Swap{State: &state, Build: func() core.Widget {
		p := PropertyColor{Value: g.value, OnChange: func(c geom.Color) {
			g.log = append(g.log, c)
			state.SetState(func() { g.value = c })
		}}
		if custom {
			p.OnPress = func() { g.press++ }
		}
		return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 360, Child: PropertyGrid{Categories: []PropertyCategory{
			{Name: "Look", Properties: []Property{{Name: "Tint", Editor: p}}},
		}}}}
	}}})
	g.render()
	return g
}

func (g *colorGrid) render() { wtest.Ops(g.tree, colorScreen) }

func (g *colorGrid) root() *semantics.Node {
	g.render()
	root, _ := g.tree.SemanticsTree()
	return root
}

func (g *colorGrid) find(role semantics.Role, label string) *semantics.Node {
	var out *semantics.Node
	g.root().Walk(func(n *semantics.Node) bool {
		if n.Role == role && (label == "" || n.Label == label) {
			out = n
		}
		return out == nil
	})
	return out
}

func (g *colorGrid) swatch(t *testing.T) *semantics.Node {
	t.Helper()
	n := g.find(semantics.RoleButton, "Tint")
	if n == nil {
		t.Fatal("no swatch button")
	}
	return n
}

func (g *colorGrid) tap(at geom.Offset) {
	wtest.Tap(g.tree, at)
	g.render()
}

func (g *colorGrid) key(k input.KeyCode) {
	g.tree.DispatchKey(core.KeyEvent{Key: k})
	g.render()
}

func TestPropertyColor_OpensThePickerAndCommits(t *testing.T) {
	g := newColorGrid(t, false)
	sw := g.swatch(t)
	if sw.Value != "#2F5FD0" {
		t.Errorf("the swatch says %q", sw.Value)
	}
	if g.find(semantics.RoleDialog, "") != nil {
		t.Fatal("closed at first")
	}
	g.tap(wtest.Center(sw.Rect))
	dialog := g.find(semantics.RoleDialog, "Choose color")
	if dialog == nil {
		t.Fatal("a press opens the picker")
	}
	if dialog.Rect.Y < sw.Rect.Y+sw.Rect.H {
		t.Errorf("the picker opens below the swatch: %+v under %+v", dialog.Rect, sw.Rect)
	}
	root, focus := g.tree.SemanticsTree()
	if n := root.Find(focus); n == nil || n.Label != "Saturation and brightness" {
		t.Errorf("the square takes the keyboard, focus on %+v", n)
	}

	// Keys reach the picker, and each change is reported at once.
	g.key(input.KeyUp)
	if len(g.log) != 1 {
		t.Fatalf("got %v", g.log)
	}
	want := theme.HSVFromColor(geom.Hex(0x2f5fd0))
	want.V += 0.01
	if got := theme.HSVFromColor(g.log[0]); geom.Abs32(got.V-want.V) > 1e-3 || geom.Abs32(got.H-want.H) > 1e-3 {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := g.swatch(t).Value; got != theme.FormatHex(g.log[0]) {
		t.Errorf("the swatch follows, got %q", got)
	}

	// A drag on the square.
	plane := g.find(semantics.RoleSlider, "Saturation and brightness")
	g.tap(geom.Offset{X: plane.Rect.X + 1, Y: plane.Rect.Y + 1})
	if h := theme.HSVFromColor(g.value); h.S > 0.02 || h.V < 0.98 {
		t.Errorf("the top left corner is white, got %+v", g.value)
	}

	// A click beside the card keeps the color and closes.
	g.tap(geom.Offset{X: 490, Y: 690})
	if g.find(semantics.RoleDialog, "") != nil {
		t.Fatal("a click beside the picker closes it")
	}
	if g.value != g.log[len(g.log)-1] {
		t.Error("closing keeps the color")
	}
	root, focus = g.tree.SemanticsTree()
	if n := root.Find(focus); n == nil || n.Label != "Tint" {
		t.Errorf("the keyboard goes back to the swatch, focus on %+v", n)
	}
}

func TestPropertyColor_EscapeGoesBack(t *testing.T) {
	g := newColorGrid(t, false)
	g.tap(wtest.Center(g.swatch(t).Rect))
	g.key(input.KeyRight)
	g.key(input.KeyRight)
	if g.value == geom.Hex(0x2f5fd0) {
		t.Fatal("the keys change the color")
	}
	g.key(input.KeyEscape)
	if g.find(semantics.RoleDialog, "") != nil {
		t.Fatal("Escape closes the picker")
	}
	if g.value != geom.Hex(0x2f5fd0) {
		t.Errorf("Escape goes back to the old color, got %+v", g.value)
	}
	n := len(g.log)
	g.tap(wtest.Center(g.swatch(t).Rect))
	g.key(input.KeyEscape)
	if len(g.log) != n {
		t.Error("Escape without a change reports nothing")
	}
	g.tap(wtest.Center(g.swatch(t).Rect))
	g.key(input.KeyEnter)
	if g.find(semantics.RoleDialog, "") != nil {
		t.Error("Enter closes the picker")
	}
}

func TestPropertyColor_CustomOnPressWins(t *testing.T) {
	g := newColorGrid(t, true)
	g.tap(wtest.Center(g.swatch(t).Rect))
	if g.press != 1 {
		t.Errorf("OnPress runs, got %d", g.press)
	}
	if g.find(semantics.RoleDialog, "") != nil {
		t.Error("a custom OnPress opens no picker of its own")
	}
}

func TestPropertyColor_WithoutCallbacksIsReadOnly(t *testing.T) {
	tree := wtest.TextTree(t, overlay.Overlay{Child: PropertyGrid{Categories: []PropertyCategory{
		{Name: "Look", Properties: []Property{{Name: "Tint", Editor: PropertyColor{Value: geom.Hex(0x2f5fd0)}}}},
	}}})
	wtest.Ops(tree, colorScreen)
	root, _ := tree.SemanticsTree()
	var button bool
	root.Walk(func(n *semantics.Node) bool {
		button = button || n.Role == semantics.RoleButton && n.Label == "Tint"
		return true
	})
	if button {
		t.Error("a color without callbacks is no button")
	}
}
