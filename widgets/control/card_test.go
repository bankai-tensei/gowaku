package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestCard_LooksAsDesignSays(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Align{Child: Card{Child: box(100, 40, wtest.Red)}})
	var shadows int
	var surface, content *paint.Op
	all := wtest.Ops(tree, wtest.Screen)
	for i := range all {
		op := &all[i]
		switch {
		case op.Kind == paint.OpShadow:
			shadows++
		case op.Kind == paint.OpRRect && surface == nil:
			surface = op
		case op.Kind == paint.OpRRect:
			content = op
		}
	}
	if surface == nil || content == nil {
		t.Fatalf("got %+v", all)
	}
	if surface.Color != th.Color.Card || surface.RRect.TopLeft != 12 {
		t.Errorf("surface %+v", surface)
	}
	if surface.BorderColor != th.Color.Hair || surface.BorderWidth != 1 {
		t.Errorf("a card has a hairline: %v %+v", surface.BorderWidth, surface.BorderColor)
	}
	if shadows != len(th.Color.Shadows(theme.LiftControl)) {
		t.Errorf("got %d shadows", shadows)
	}
	if want := (geom.Rect{X: 16, Y: 16, W: 100, H: 40}); content.RRect.Rect != want {
		t.Errorf("padding: got %+v, want %+v", content.RRect.Rect, want)
	}
	if want := (geom.Rect{W: 132, H: 72}); surface.RRect.Rect != want {
		t.Errorf("got %+v, want %+v", surface.RRect.Rect, want)
	}
}

func TestCard_PaddingReplacesTheDefault(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Card{Padding: geom.All(4), Child: box(10, 10, wtest.Red)}})
	got := wtest.Rrects(tree, wtest.Screen)
	if len(got) != 2 || got[1].RRect.Rect.X != 4 {
		t.Errorf("got %+v", got)
	}
}

func TestCard_PressAndKeys(t *testing.T) {
	var pressed int
	tree := wtest.TextTree(t, basic.Column{Size: layout.MainMin, Children: []core.Widget{
		Card{Label: "Note", OnPress: func() { pressed++ }, Child: box(100, 40, wtest.Red)},
	}})
	wtest.Ops(tree, wtest.Screen)
	wtest.Tap(tree, geom.Offset{X: 50, Y: 30})
	if pressed != 1 {
		t.Fatalf("a click runs OnPress: %d", pressed)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace, Repeat: true})
	if pressed != 3 {
		t.Errorf("Enter and Space run OnPress, a repeat does not: %d", pressed)
	}
}

func TestCard_Semantics(t *testing.T) {
	tests := []struct {
		name    string
		w       Card
		role    semantics.Role
		actions semantics.Actions
	}{
		{"ただの面", Card{Label: "Summary", Child: basic.Text{Content: "x"}}, semantics.RoleGroup, 0},
		{"押せる", Card{Label: "Note", OnPress: func() {}, Child: basic.Text{Content: "x"}}, semantics.RoleButton,
			semantics.ActionTap | semantics.ActionFocus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.w})
			wtest.Ops(tree, wtest.Screen)
			n := wtest.FindRole(tree.Semantics(), tt.role)
			if n == nil {
				t.Fatalf("no %v node", tt.role)
			}
			if n.Label != tt.w.Label || n.Actions != tt.actions {
				t.Errorf("got %q actions %v, want %q %v", n.Label, n.Actions, tt.w.Label, tt.actions)
			}
			if wtest.FindRole(n, semantics.RoleText) == nil {
				t.Error("the content must stay readable")
			}
		})
	}
	tree := wtest.TextTree(t, basic.Align{Child: Card{Child: basic.Text{Content: "x"}}})
	wtest.Ops(tree, wtest.Screen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleGroup) != nil {
		t.Error("a card without a label is no group")
	}
}

func TestCard_PressableWithoutLabelIsNamedByItsText(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Card{OnPress: func() {}, Child: basic.Column{Size: layout.MainMin, Children: []core.Widget{
		basic.Text{Content: "Title"}, basic.Text{Content: "Detail"},
	}}}})
	wtest.Ops(tree, wtest.Screen)
	n := wtest.FindRole(tree.Semantics(), semantics.RoleButton)
	if n == nil {
		t.Fatal("no button")
	}
	if n.Label != "Title Detail" {
		t.Errorf("got %q", n.Label)
	}
	if wtest.FindRole(n, semantics.RoleText) != nil {
		t.Error("the texts that name the button must not be read again")
	}
}

func TestCard_HoverAndPressMoveOnlyAPressableOne(t *testing.T) {
	for _, tt := range []struct {
		name  string
		w     Card
		moves bool
	}{
		{"押せる", Card{OnPress: func() {}, Child: box(100, 40, wtest.Red)}, true},
		{"ただの面", Card{Child: box(100, 40, wtest.Red)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.w})
			rest := testutil.RenderTree(tree, wtest.Screen)
			pos := geom.Offset{X: 8, Y: 8}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			hovered := testutil.RenderTree(tree, wtest.Screen)
			if (hovered != rest) != tt.moves {
				t.Fatalf("hover shows: %v, want %v", hovered != rest, tt.moves)
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
			wtest.Settled(t, tree, wtest.Screen, &now)
			pressedAt := wtest.Rrects(tree, wtest.Screen)[0].RRect.Rect
			if tt.moves {
				if want := float32(132 * (1 - cardSink)); pressedAt.W > want+0.1 || pressedAt.W < want-0.1 {
					t.Errorf("a press sinks the card to %v, got %v", want, pressedAt.W)
				}
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
			wtest.Settled(t, tree, wtest.Screen, &now)
			if got := testutil.RenderTree(tree, wtest.Screen); got != hovered {
				t.Errorf("the press must end:\n%s\n%s", got, hovered)
			}
		})
	}
}

func TestCard_FocusRingFromTheKeyboard(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Card{OnPress: func() {}, Child: box(100, 40, wtest.Red)}})
	if hasRing(tree) {
		t.Fatal("no ring before the focus")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if !hasRing(tree) {
		t.Error("the keyboard must show the focus ring")
	}
}

// hasRing reports whether tree paints a focus ring that shows.
func hasRing(tree *core.Tree) bool {
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Kind == paint.OpRRect && op.BorderWidth == 3 && op.BorderColor.A > 0 {
			return true
		}
	}
	return false
}

// box is a w×h Container of color c.
func box(w, h float32, c geom.Color) core.Widget {
	return basic.Container{Width: w, Height: h, Color: c}
}
