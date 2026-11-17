package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// chipRig is a chip whose state lives in the test, as an app keeps it.
type chipRig struct {
	tree               *core.Tree
	selected           bool
	changes, removed   int
	swap               func(core.Widget)
	chip               func() core.Widget
	removable, toggles bool
}

func newChipRig(t *testing.T, toggles, removable bool) *chipRig {
	t.Helper()
	r := &chipRig{toggles: toggles, removable: removable}
	r.chip = func() core.Widget {
		c := Chip{Label: "design", Selected: r.selected}
		if toggles {
			c.OnChange = func(v bool) {
				r.changes++
				r.selected = v
				r.swap(basic.Align{Child: r.chip()})
			}
		}
		if removable {
			c.OnRemove = func() { r.removed++ }
		}
		return c
	}
	r.tree, r.swap = wtest.SwapTree(t, basic.Align{Child: r.chip()})
	wtest.Ops(r.tree, wtest.Screen)
	return r
}

func TestChip_TogglesWithThePointerAndKeys(t *testing.T) {
	r := newChipRig(t, true, false)
	wtest.Tap(r.tree, geom.Offset{X: 10, Y: 12})
	wtest.Ops(r.tree, wtest.Screen)
	if r.changes != 1 || !r.selected {
		t.Fatalf("a click selects: %d %v", r.changes, r.selected)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(r.tree, wtest.Screen)
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	wtest.Ops(r.tree, wtest.Screen)
	if r.changes != 2 || r.selected {
		t.Fatalf("Space toggles back: %d %v", r.changes, r.selected)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter, Repeat: true})
	if r.changes != 2 {
		t.Error("a repeat must not toggle")
	}
}

func TestChip_SelectedUsesTheAccentTint(t *testing.T) {
	th := theme.Default().Light
	for _, tt := range []struct {
		name string
		w    Chip
		fill geom.Color
	}{
		{"選択中", Chip{Label: "x", Selected: true, OnChange: func(bool) {}}, th.Color.AccentTint},
		{"未選択", Chip{Label: "x", OnChange: func(bool) {}}, th.Color.Fill},
		{"タグ", Chip{Label: "x"}, th.Color.Fill},
		// Only a chip that toggles can be selected.
		{"押すだけ", Chip{Label: "x", Selected: true, OnPress: func() {}}, th.Color.Fill},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.w})
			got := wtest.Rrects(tree, wtest.Screen)
			if len(got) == 0 || got[0].Color != tt.fill {
				t.Fatalf("got %+v, want %+v", got, tt.fill)
			}
			if r := got[0].RRect; r.Rect.H != chipHeight || r.TopLeft != chipHeight/2 {
				t.Errorf("a chip is a pill 24 high: %+v", r)
			}
		})
	}
}

func TestChip_RemoveButtonAndKeys(t *testing.T) {
	r := newChipRig(t, true, true)
	remove := wtest.FindRole(r.tree.Semantics(), semantics.RoleButton)
	if remove == nil || remove.Label != "Remove design" {
		t.Fatalf("got %+v", remove)
	}
	wtest.Tap(r.tree, wtest.Center(remove.Rect))
	wtest.Ops(r.tree, wtest.Screen)
	if r.removed != 1 || r.changes != 0 {
		t.Fatalf("the × removes and does not toggle: removed %d, changes %d", r.removed, r.changes)
	}
	// Tab reaches the chip, not the ×, whose keys the chip has.
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(r.tree, wtest.Screen)
	if n := wtest.FindRole(r.tree.Semantics(), semantics.RoleCheckbox); n.State&semantics.Focused == 0 {
		t.Fatalf("Tab must focus the chip: %+v", n)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(r.tree, wtest.Screen)
	if n := wtest.FindRole(r.tree.Semantics(), semantics.RoleCheckbox); n.State&semantics.Focused == 0 {
		t.Fatal("the × must not be a stop of Tab of a chip that takes the focus")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyDelete})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyBackspace})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyBackspace, Repeat: true})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyDelete, Modifiers: input.ModControl})
	if r.removed != 3 {
		t.Errorf("Delete and Backspace remove once each: %d", r.removed)
	}
}

func TestChip_KeysWithoutRemovePassOn(t *testing.T) {
	var outer int
	r := &chipRig{}
	tree := wtest.TextTree(t, keyHandler{onKey: func(*core.KeyEvent) bool { outer++; return true }, child: basic.Align{
		Child: Chip{Label: "x", OnChange: func(bool) { r.changes++ }},
	}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyDelete})
	if outer != 1 {
		t.Errorf("Delete must pass on from a chip that cannot be removed: %d", outer)
	}
}

func TestChip_PlainTagRemoveIsAStopOfTab(t *testing.T) {
	r := newChipRig(t, false, true)
	root := r.tree.Semantics()
	group := wtest.FindRole(root, semantics.RoleGroup)
	if group == nil || group.Label != "design" || group.Actions&semantics.ActionTap != 0 {
		t.Fatalf("a tag is a group that does nothing itself: %+v", group)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(r.tree, wtest.Screen)
	remove := wtest.FindRole(r.tree.Semantics(), semantics.RoleButton)
	if remove == nil || remove.State&semantics.Focused == 0 {
		t.Fatalf("Tab must reach the × of a tag: %+v", remove)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyDelete})
	if r.removed != 2 {
		t.Errorf("Enter and Delete remove: %d", r.removed)
	}
}

func TestChip_PlainTagIsNoControl(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Chip{Label: "draft"}})
	rest := testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 8, Y: 8}})
	if tree.Ticking() || testutil.RenderTree(tree, wtest.Screen) != rest {
		t.Error("a tag does not react to the pointer")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() != nil {
		t.Error("a tag takes no focus")
	}
	wtest.Ops(tree, wtest.Screen)
	n := wtest.FindRole(tree.Semantics(), semantics.RoleText)
	if n == nil || n.Label != "draft" || wtest.FindRole(tree.Semantics(), semantics.RoleButton) != nil {
		t.Errorf("a tag reads as its text: %+v", n)
	}
}

func TestChip_Semantics(t *testing.T) {
	tests := []struct {
		name  string
		w     Chip
		role  semantics.Role
		value string
		state semantics.State
	}{
		{"選択中", Chip{Label: "All", Selected: true, OnChange: func(bool) {}}, semantics.RoleCheckbox, "checked", semantics.Checked},
		{"未選択", Chip{Label: "All", OnChange: func(bool) {}}, semantics.RoleCheckbox, "unchecked", 0},
		{"押す", Chip{Label: "All", OnPress: func() {}}, semantics.RoleButton, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.w})
			wtest.Ops(tree, wtest.Screen)
			n := wtest.FindRole(tree.Semantics(), tt.role)
			if n == nil {
				t.Fatalf("no %v", tt.role)
			}
			want := semantics.Focusable | tt.state
			if n.Label != "All" || n.Value != tt.value || n.State != want || n.Actions&semantics.ActionTap == 0 {
				t.Errorf("got %+v", n)
			}
			if wtest.FindRole(n, semantics.RoleText) != nil {
				t.Error("the label must be read once")
			}
		})
	}
}

func TestChip_RemoveLabelFollowsTheLocale(t *testing.T) {
	tree := wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.Japanese}, Child: basic.Align{
		Child: Chip{Label: "デザイン", OnRemove: func() {}},
	}})
	wtest.Ops(tree, wtest.Screen)
	if n := wtest.FindRole(tree.Semantics(), semantics.RoleButton); n == nil || n.Label != "デザインを削除" {
		t.Errorf("got %+v", n)
	}
}

func TestChip_IconAndRemoveKeepTheHeight(t *testing.T) {
	path := kit.CrossPath(24, 12, 2)
	tree := wtest.TextTree(t, basic.Align{Child: Chip{Label: "x", Icon: path, OnChange: func(bool) {}, OnRemove: func() {}}})
	got := wtest.Rrects(tree, wtest.Screen)
	if len(got) == 0 || got[0].RRect.Rect.H != chipHeight {
		t.Fatalf("got %+v", got)
	}
	var w float32
	tree2 := wtest.TextTree(t, basic.Align{Child: Chip{Label: "x", OnChange: func(bool) {}}})
	if r := wtest.Rrects(tree2, wtest.Screen); len(r) > 0 {
		w = r[0].RRect.Rect.W
	}
	if got[0].RRect.Rect.W <= w {
		t.Errorf("the icon and the × take room: %v, plain %v", got[0].RRect.Rect.W, w)
	}
}

func TestChip_HoverAndPressMove(t *testing.T) {
	for name, w := range map[string]core.Widget{
		"切り替え": Chip{Label: "x", OnChange: func(bool) {}},
		"押す":   Chip{Label: "x", OnPress: func() {}},
	} {
		t.Run(name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: w})
			rest := testutil.RenderTree(tree, wtest.Screen)
			pos := geom.Offset{X: 6, Y: 12}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			if !tree.Ticking() {
				t.Fatal("hover must animate")
			}
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			hovered := testutil.RenderTree(tree, wtest.Screen)
			if hovered == rest {
				t.Fatal("hover must show")
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) == hovered {
				t.Error("a press must show")
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) != hovered {
				t.Error("the press must end")
			}
		})
	}
}

func TestChip_GroupInAWrap(t *testing.T) {
	chips := make([]core.Widget, 6)
	for i := range chips {
		chips[i] = Chip{Label: "filter", OnChange: func(bool) {}}
	}
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Child: basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}}})
	var rows []float32
	for _, op := range wtest.Rrects(tree, wtest.Screen) {
		if len(rows) == 0 || rows[len(rows)-1] != op.RRect.Rect.Y {
			rows = append(rows, op.RRect.Rect.Y)
		}
	}
	if len(rows) < 2 || rows[1]-rows[0] != chipHeight+theme.Space8 {
		t.Errorf("the chips must wrap on rows 8 apart: %v", rows)
	}
}
