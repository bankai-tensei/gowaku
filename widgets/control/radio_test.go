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
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// radioHarness keeps the value of a RadioGroup of strings and rebuilds it.
type radioHarness struct {
	tree    *core.Tree
	value   string
	changes int
}

func newRadioHarness(t *testing.T, value string, options []RadioOption[string], after core.Widget) *radioHarness {
	t.Helper()
	h := &radioHarness{value: value}
	var swap func(core.Widget)
	var build func() core.Widget
	build = func() core.Widget {
		group := RadioGroup[string]{Value: h.value, Options: options, Label: "サイズ", OnChange: func(v string) {
			h.value = v
			h.changes++
			swap(build())
		}}
		if after == nil {
			return basic.Align{Child: group}
		}
		return basic.Align{Child: basic.Column{Size: layout.MainMin, Children: []core.Widget{group, after}}}
	}
	h.tree, swap = wtest.SwapTree(t, build())
	testutil.RenderTree(h.tree, wtest.Screen)
	return h
}

func (h *radioHarness) Key(k input.KeyCode) {
	h.tree.DispatchKey(core.KeyEvent{Key: k})
	testutil.RenderTree(h.tree, wtest.Screen)
}

func sizes() []RadioOption[string] {
	return []RadioOption[string]{
		{Value: "s", Label: "Small"},
		{Value: "m", Label: "Medium", Disabled: true},
		{Value: "l", Label: "Large"},
	}
}

func TestRadioGroup_ClickAnywhereOnTheRow(t *testing.T) {
	h := newRadioHarness(t, "s", sizes(), nil)
	root, _ := h.tree.SemanticsTree()
	large := wtest.ByLabel(t, root, "Large")
	// The right end of the row is the label, away from the circle.
	at := geom.Offset{X: large.Rect.X + large.Rect.W - 4, Y: large.Rect.Y + large.Rect.H/2}
	wtest.Tap(h.tree, at)
	if h.value != "l" {
		t.Fatalf("got %q", h.value)
	}
	testutil.RenderTree(h.tree, wtest.Screen)
	medium := wtest.ByLabel(t, root, "Medium")
	wtest.Tap(h.tree, wtest.Center(medium.Rect))
	if h.value != "l" {
		t.Errorf("a disabled choice must not be taken, got %q", h.value)
	}
	wtest.Tap(h.tree, at)
	if h.changes != 1 {
		t.Errorf("the selected choice must not ask again, got %d changes", h.changes)
	}
}

func TestRadioGroup_KeysMoveAndSelect(t *testing.T) {
	h := newRadioHarness(t, "l", sizes(), nil)
	h.Key(input.KeyTab)
	root, focus := h.tree.SemanticsTree()
	if n := root.Find(focus); n == nil || n.Label != "Large" {
		t.Fatalf("Tab must enter at the selected choice, got %+v", n)
	}
	cases := []struct {
		name string
		key  input.KeyCode
		want string
	}{
		{"下は端で先頭へ回る", input.KeyDown, "s"},
		{"下は無効を飛ばす", input.KeyDown, "l"},
		{"上も無効を飛ばす", input.KeyUp, "s"},
		{"左は上と同じ", input.KeyLeft, "l"},
		{"右は下と同じ", input.KeyRight, "s"},
	}
	for _, tc := range cases {
		h.Key(tc.key)
		if h.value != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, h.value, tc.want)
		}
	}
	root, focus = h.tree.SemanticsTree()
	if n := root.Find(focus); n == nil || n.Label != "Small" {
		t.Errorf("the reader must follow the choice, got %+v", n)
	}
}

func TestRadioGroup_SpaceSelectsWhenNoneIs(t *testing.T) {
	h := newRadioHarness(t, "", sizes(), nil)
	h.Key(input.KeyTab)
	if h.value != "" {
		t.Fatalf("Tab alone must not choose, got %q", h.value)
	}
	h.Key(input.KeySpace)
	if h.value != "s" {
		t.Errorf("Space must choose the first choice, got %q", h.value)
	}
}

func TestRadioGroup_IsOneTabStop(t *testing.T) {
	h := newRadioHarness(t, "s", sizes(), Button{Label: "OK", OnPress: func() {}})
	seen := map[core.FocusTarget]bool{}
	for range 4 {
		h.Key(input.KeyTab)
		seen[h.tree.Focused()] = true
	}
	if len(seen) != 2 {
		t.Errorf("the group and the button must take turns, got %d stops", len(seen))
	}
}

func TestRadioGroup_Semantics(t *testing.T) {
	h := newRadioHarness(t, "s", sizes(), nil)
	root, _ := h.tree.SemanticsTree()
	group := wtest.FindRole(root, semantics.RoleRadioGroup)
	if group == nil || group.Label != "サイズ" || group.Value != "Small" || group.State&semantics.Focusable == 0 {
		t.Fatalf("got %+v", group)
	}
	if len(group.Children) != 3 {
		t.Fatalf("the radios must be the children of the group, got %d", len(group.Children))
	}
	for i, want := range []struct {
		label string
		state semantics.State
		tap   bool
	}{
		{"Small", semantics.Selected, true},
		{"Medium", semantics.Disabled, false},
		{"Large", 0, true},
	} {
		n := group.Children[i]
		if n.Role != semantics.RoleRadio || n.Label != want.label || n.State&(semantics.Selected|semantics.Disabled) != want.state {
			t.Errorf("radio %d: got %+v", i, n)
		}
		if got := n.Actions&semantics.ActionTap != 0; got != want.tap {
			t.Errorf("radio %d: tap %v, want %v", i, got, want.tap)
		}
		if len(n.Children) != 0 {
			t.Errorf("radio %d: the label must not be read twice", i)
		}
	}
	if !h.tree.PerformSemanticsAction(group.Children[2].ID, semantics.ActionTap, core.SemanticsArg{}) || h.value != "l" {
		t.Errorf("a reader must choose, got %q", h.value)
	}
}

func TestRadioGroup_DisabledWithoutOnChange(t *testing.T) {
	tree := wtest.A11yTree(t, RadioGroup[int]{Value: 1, Label: "x", Options: []RadioOption[int]{{Value: 1, Label: "a"}, {Value: 2, Label: "b"}}})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() != nil {
		t.Error("a disabled group must not take the focus")
	}
	root, _ := tree.SemanticsTree()
	if g := wtest.FindRole(root, semantics.RoleRadioGroup); g.State&semantics.Disabled == 0 {
		t.Errorf("got %+v", g)
	}
}

func TestRadioGroup_Horizontal(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Align{Child: RadioGroup[int]{Horizontal: true, Options: []RadioOption[int]{{Value: 1, Label: "a"}, {Value: 2, Label: "b"}}}})
	root, _ := tree.SemanticsTree()
	a, b := wtest.ByLabel(t, root, "a"), wtest.ByLabel(t, root, "b")
	if a.Rect.Y != b.Rect.Y || b.Rect.X <= a.Rect.X+a.Rect.W {
		t.Errorf("got %+v and %+v", a.Rect, b.Rect)
	}
}

func TestRadioGroup_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, RadioGroup[string]{})
	testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
}

func TestRadio_StandaloneTakesItsOwnFocus(t *testing.T) {
	selected := 0
	tree := wtest.A11yTree(t, basic.Align{Child: Radio{Label: "One", OnSelect: func() { selected++ }}})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	if selected != 1 {
		t.Fatalf("Space must select, got %d", selected)
	}
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "One")
	if n.Role != semantics.RoleRadio || n.State&semantics.Focused == 0 {
		t.Errorf("got %+v", n)
	}
	wtest.Tap(tree, wtest.Center(n.Rect))
	if selected != 2 {
		t.Errorf("a click must select, got %d", selected)
	}
}

func TestRadio_SelectedDoesNotAskAgain(t *testing.T) {
	selected := 0
	tree := wtest.A11yTree(t, basic.Align{Child: Radio{Selected: true, Label: "One", OnSelect: func() { selected++ }}})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "One")
	wtest.Tap(tree, wtest.Center(n.Rect))
	if selected != 0 || n.State&semantics.Selected == 0 {
		t.Errorf("got %d calls, state %b", selected, n.State)
	}
}

func TestRadio_Disabled(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Align{Child: Radio{Label: "One"}})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "One")
	if n.State&semantics.Disabled == 0 || n.Actions&semantics.ActionTap != 0 || n.State&semantics.Focusable != 0 {
		t.Errorf("state %b actions %b", n.State, n.Actions)
	}
}

func TestRadio_SelectionFillsTheCircle(t *testing.T) {
	th := theme.Default().Light
	tree, swap := wtest.SwapTree(t, basic.Align{Child: Radio{Label: "x", OnSelect: func() {}}})
	accent := func() (fill, dot bool) {
		for _, op := range wtest.Ops(tree, wtest.Screen) {
			if op.RRect.Rect.W == radioCircle && op.Color == th.Color.Accent {
				fill = true
			}
			if op.RRect.Rect.W == radioDot && op.Color == th.Color.OnAccent {
				dot = true
			}
		}
		return fill, dot
	}
	if fill, dot := accent(); fill || dot {
		t.Fatal("an empty circle must not fill")
	}
	swap(basic.Align{Child: Radio{Selected: true, Label: "x", OnSelect: func() {}}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if fill, dot := accent(); !fill || !dot {
		t.Errorf("got fill %v, dot %v", fill, dot)
	}
}

// topLeft puts a control where the pointer of the design checks is.
var topLeft = layout.AlignTopLeft

// choiceControls are the controls of this file for the shared design checks.
func choiceControls() []struct {
	name string
	w    core.Widget
} {
	return []struct {
		name string
		w    core.Widget
	}{
		{"Radio", Radio{Label: "x", OnSelect: func() {}}},
		{"選択済みの Radio", Radio{Selected: true, Label: "x", OnSelect: func() {}}},
		{"RadioGroup", RadioGroup[int]{Value: 1, Options: []RadioOption[int]{{Value: 1, Label: "a"}, {Value: 2, Label: "b"}}, OnChange: func(int) {}}},
		{"Tabs", basic.SizedBox{Width: 200, Height: 100, Child: Tabs{Pages: []TabPage{{Label: "a"}, {Label: "b"}}, OnChange: func(int) {}}}},
	}
}

func TestChoice_HoverMoves(t *testing.T) {
	for _, tt := range choiceControls() {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Alignment: topLeft, Child: tt.w})
			rest := testutil.RenderTree(tree, wtest.Screen)
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 8, Y: 8}})
			testutil.RenderTree(tree, wtest.Screen)
			if !tree.Ticking() {
				t.Fatal("hover must animate")
			}
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) == rest {
				t.Error("hover must show")
			}
		})
	}
}

func TestChoice_PressMoves(t *testing.T) {
	for _, tt := range choiceControls() {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Alignment: topLeft, Child: tt.w})
			testutil.RenderTree(tree, wtest.Screen)
			pos := geom.Offset{X: 8, Y: 8}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			rest := testutil.RenderTree(tree, wtest.Screen)
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) == rest {
				t.Error("a press must show")
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			wtest.Settled(t, tree, wtest.Screen, &now)
			if got := testutil.RenderTree(tree, wtest.Screen); got != rest {
				t.Errorf("the press must end:\n%s\n%s", got, rest)
			}
		})
	}
}

func TestChoice_FocusShowsTheRing(t *testing.T) {
	for _, tt := range choiceControls() {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Alignment: topLeft, Child: tt.w})
			ring := func() bool {
				for _, op := range wtest.Ops(tree, wtest.Screen) {
					if op.BorderWidth == 3 && op.BorderColor.A > 0 {
						return true
					}
				}
				return false
			}
			if ring() {
				t.Fatal("no ring without the focus")
			}
			tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
			wtest.Ops(tree, wtest.Screen)
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			if !ring() {
				t.Error("the focus must bring the ring")
			}
		})
	}
}

func TestChoice_DisabledUsesTokensNotOpacity(t *testing.T) {
	th := theme.Default().Light
	for name, w := range map[string]core.Widget{
		"Radio":           Radio{Label: "x"},
		"選択済みの Radio":     Radio{Selected: true, Label: "x"},
		"RadioGroup":      RadioGroup[int]{Value: 1, Options: []RadioOption[int]{{Value: 1, Label: "a"}}},
		"無効の RadioOption": RadioGroup[int]{Options: []RadioOption[int]{{Value: 1, Label: "a", Disabled: true}}, OnChange: func(int) {}},
		"Tabs":            basic.SizedBox{Width: 200, Height: 100, Child: Tabs{Pages: []TabPage{{Label: "a"}, {Label: "b"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: w})
			var token bool
			for _, op := range wtest.Ops(tree, wtest.Screen) {
				if op.Color == th.Color.Ink3 || op.BorderColor == th.Color.Hair {
					token = true
				}
				if op.Color.A > 0 && op.Color.A < 1 && op.Color != th.Color.Fill && op.Color != th.Color.Hair {
					t.Errorf("a translucent %+v is not a token", op.Color)
				}
			}
			if !token {
				t.Error("a disabled control must draw in the disabled colors")
			}
		})
	}
}
