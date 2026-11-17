package nav

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// toolbarRig is a toolbar with buttons a to e, c disabled and a line after b,
// above a button outside it, and what its buttons did.
type toolbarRig struct {
	tree *core.Tree
	ran  []string
}

func newToolbarRig(t *testing.T, width float32, extra ...ToolbarItem) *toolbarRig {
	t.Helper()
	r := &toolbarRig{}
	act := func(label string) interact.Action {
		return interact.Action{Label: label, OnInvoke: func() { r.ran = append(r.ran, label) }}
	}
	items := []ToolbarItem{
		{Action: act("a")},
		{Action: act("b")},
		{Separator: true},
		{Action: interact.Action{Label: "c"}},
		{Action: act("d")},
		{Action: act("e")},
	}
	items = append(items, extra...)
	r.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Column{Cross: layout.CrossStart, Children: []core.Widget{
		basic.SizedBox{Width: width, Child: Toolbar{Label: "Tools", Items: items}},
		control.Button{Label: "after", OnPress: func() { r.ran = append(r.ran, "after") }},
	}}})
	wtest.Ops(r.tree, wtest.Screen)
	return r
}

// focused returns the label of the node in focus, or "".
func (r *toolbarRig) focused() string {
	wtest.Ops(r.tree, wtest.Screen)
	var out string
	r.tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.State&semantics.Focused != 0 {
			out = n.Label
			return false
		}
		return true
	})
	return out
}

func (r *toolbarRig) Key(k input.KeyCode, mods input.Modifiers) {
	wtest.Ops(r.tree, wtest.Screen)
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
}

func TestToolbar_ArrowKeysMoveBetweenTheButtons(t *testing.T) {
	r := newToolbarRig(t, 380)
	steps := []struct {
		name string
		key  input.KeyCode
		mods input.Modifiers
		want string
	}{
		{"Tab で最初のボタン", input.KeyTab, 0, "a"},
		{"右", input.KeyRight, 0, "b"},
		{"線と無効を飛ばす", input.KeyRight, 0, "d"},
		{"End", input.KeyEnd, 0, "e"},
		{"端で止まる", input.KeyRight, 0, "e"},
		{"左", input.KeyLeft, 0, "d"},
		{"Tab で外へ出る", input.KeyTab, 0, "after"},
		{"Shift+Tab で最後のボタンへ戻る", input.KeyTab, input.ModShift, "d"},
		{"Home", input.KeyHome, 0, "a"},
		{"端で止まる左", input.KeyLeft, 0, "a"},
	}
	for _, s := range steps {
		r.Key(s.key, s.mods)
		if got := r.focused(); got != s.want {
			t.Fatalf("%s: got %q, want %q", s.name, got, s.want)
		}
	}
	r.Key(input.KeyEnter, 0)
	r.Key(input.KeyRight, 0)
	r.Key(input.KeySpace, 0)
	if len(r.ran) != 2 || r.ran[0] != "a" || r.ran[1] != "b" {
		t.Errorf("Enter and Space press the button in focus: %q", r.ran)
	}
}

func TestToolbar_OneStopOfTab(t *testing.T) {
	r := newToolbarRig(t, 380)
	wtest.Ops(r.tree, wtest.Screen)
	var stops int
	r.tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.State&semantics.Focusable != 0 {
			stops++
		}
		return true
	})
	// The toolbar and the button after it.
	if stops != 2 {
		t.Errorf("got %d stops of Tab", stops)
	}
}

func TestToolbar_ClickRunsAndMovesTheStop(t *testing.T) {
	r := newToolbarRig(t, 380)
	bs := wtest.Buttons(r.tree.Semantics(), nil)
	var d *semantics.Node
	for _, b := range bs {
		if b.Label == "d" {
			d = b
		}
	}
	if d == nil {
		t.Fatal("no d")
	}
	wtest.Tap(r.tree, wtest.Center(d.Rect))
	if len(r.ran) != 1 || r.ran[0] != "d" {
		t.Fatalf("got %q", r.ran)
	}
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyTab, input.ModShift)
	if got := r.focused(); got != "d" {
		t.Errorf("Tab comes to the button used last: %q", got)
	}
}

func TestToolbar_ClickTakesTheFocusAlongInside(t *testing.T) {
	r := newToolbarRig(t, 380)
	r.Key(input.KeyTab, 0)
	if got := r.focused(); got != "a" {
		t.Fatalf("got %q", got)
	}
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, r.tree.Semantics(), "e").Rect))
	if got := r.focused(); got != "e" {
		t.Errorf("a click moves the focus within the toolbar: %q", got)
	}
	r.Key(input.KeyLeft, 0)
	if got := r.focused(); got != "d" {
		t.Errorf("the arrows go on from there: %q", got)
	}
}

func TestToolbar_Semantics(t *testing.T) {
	bold := interact.Action{Label: "太字(&B)", Checkable: true, Checked: true, OnInvoke: func() {},
		Shortcut: interact.Shortcut{Key: input.KeyB, Modifiers: input.ModControl}}
	tree := wtest.TextTree(t, overlay.Overlay{Child: Toolbar{Label: "書式", Items: []ToolbarItem{
		{Action: bold, Icon: kit.CrossPath(24, 12, 2)},
		{Separator: true},
		{Action: interact.Action{Label: "保存"}},
	}}})
	wtest.Ops(tree, wtest.Screen)
	bar := wtest.FindRole(tree.Semantics(), semantics.RoleToolbar)
	if bar == nil || bar.Label != "書式" {
		t.Fatalf("got %+v", bar)
	}
	toggle := wtest.FindRole(bar, semantics.RoleCheckbox)
	if toggle == nil || toggle.Label != "太字" || toggle.State&semantics.Checked == 0 ||
		toggle.Value != "checked" || toggle.Shortcut != "Ctrl+B" || toggle.Actions&semantics.ActionTap == 0 {
		t.Errorf("a checkable action is a toggle: %+v", toggle)
	}
	if wtest.FindRole(bar, semantics.RoleSeparator) == nil {
		t.Error("no separator")
	}
	save := wtest.FindRole(bar, semantics.RoleButton)
	if save == nil || save.Label != "保存" || save.State&semantics.Disabled == 0 {
		t.Errorf("a button without OnInvoke is disabled: %+v", save)
	}
}

func TestToolbarLabel(t *testing.T) {
	for label, want := range map[string]string{
		"太字(&B)":       "太字",
		"&Bold":        "Bold",
		"Save && Quit": "Save & Quit",
		"(&X) only":    "(X) only",
		"保存":           "保存",
	} {
		if got := toolbarLabel(label); got != want {
			t.Errorf("%q: got %q, want %q", label, got, want)
		}
	}
}

// labels returns the labels of the buttons below n.
func labels(n *semantics.Node) []string {
	var out []string
	for _, b := range wtest.Buttons(n, nil) {
		out = append(out, b.Label)
	}
	return out
}

func TestToolbar_OverflowGoesIntoAMenu(t *testing.T) {
	// a, b, the line and c fit in 160 with the … button; d and e do not.
	r := newToolbarRig(t, 160)
	bar := wtest.FindRole(r.tree.Semantics(), semantics.RoleToolbar)
	got := labels(bar)
	want := []string{"a", "b", "c", "More"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	// The arrow keys reach the … button and pass the hidden ones by.
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnd, 0)
	if f := r.focused(); f != "More" {
		t.Fatalf("End goes to the … button: %q", f)
	}
	r.Key(input.KeyEnter, 0)
	wtest.Ops(r.tree, wtest.Screen)
	items := wtest.NodesOf(r.tree.Semantics(), semantics.RoleMenuItem, nil)
	if len(items) != 2 || items[0].Label != "d" || items[1].Label != "e" {
		var names []string
		for _, n := range items {
			names = append(names, n.Label)
		}
		t.Fatalf("the menu holds what did not fit: %q", names)
	}
	if items[0].State&semantics.Focused == 0 {
		t.Error("a menu opened from the keyboard focuses its first item")
	}
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyEnter, 0)
	wtest.Ops(r.tree, wtest.Screen)
	if len(r.ran) != 1 || r.ran[0] != "e" {
		t.Errorf("the menu runs the action: %q", r.ran)
	}
	if f := r.focused(); f != "More" {
		t.Errorf("the focus comes back to the … button: %q", f)
	}
}

func TestToolbar_OverflowWithPointer(t *testing.T) {
	r := newToolbarRig(t, 160)
	var more *semantics.Node
	for _, b := range wtest.Buttons(r.tree.Semantics(), nil) {
		if b.Label == "More" {
			more = b
		}
	}
	if more == nil {
		t.Fatal("no … button")
	}
	if want := float32(160 - theme.Space8 - theme.ControlHeight); more.Rect.X != want {
		t.Errorf("the … button sits at the end: %v, want %v", more.Rect.X, want)
	}
	wtest.Tap(r.tree, wtest.Center(more.Rect))
	wtest.Ops(r.tree, wtest.Screen)
	if items := wtest.NodesOf(r.tree.Semantics(), semantics.RoleMenuItem, nil); len(items) != 2 {
		t.Errorf("a click opens the menu: %d items", len(items))
	}
}

func TestToolbar_WideEnoughHasNoMoreButton(t *testing.T) {
	r := newToolbarRig(t, 380)
	for _, l := range labels(r.tree.Semantics()) {
		if l == "More" {
			t.Error("everything fits; no … button")
		}
	}
}

func TestToolbar_ShrinkingMovesTheFocusToMore(t *testing.T) {
	var swap func(core.Widget)
	items := []ToolbarItem{
		{Action: interact.Action{Label: "a", OnInvoke: func() {}}},
		{Action: interact.Action{Label: "b", OnInvoke: func() {}}},
		{Action: interact.Action{Label: "c", OnInvoke: func() {}}},
		{Action: interact.Action{Label: "d", OnInvoke: func() {}}},
	}
	bar := func(w float32) core.Widget {
		return overlay.Overlay{Child: basic.Align{Child: basic.SizedBox{Width: w, Child: Toolbar{Items: items}}}}
	}
	var tree *core.Tree
	tree, swap = wtest.SwapTree(t, bar(400))
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnd})
	swap(bar(100))
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	var got string
	tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.State&semantics.Focused != 0 {
			got = n.Label
			return false
		}
		return true
	})
	if got != "More" {
		t.Errorf("a hidden stop gives its place to the … button: %q", got)
	}
}

func TestToolbar_SpacerPushesToTheEnd(t *testing.T) {
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{Toolbar{Items: []ToolbarItem{
		{Action: interact.Action{Label: "a", OnInvoke: func() {}}},
		{Spacer: true},
		{Action: interact.Action{Label: "z", OnInvoke: func() {}}, Icon: kit.CrossPath(24, 12, 2)},
	}}}}})
	wtest.Ops(tree, wtest.Screen)
	bs := wtest.Buttons(tree.Semantics(), nil)
	if len(bs) != 2 {
		t.Fatalf("got %d buttons", len(bs))
	}
	if want := float32(400 - theme.Space8 - theme.ControlHeight); bs[1].Rect.X != want || bs[1].Rect.W != theme.ControlHeight {
		t.Errorf("an icon button is square at the end: %+v, want X %v", bs[1].Rect, want)
	}
	if bs[0].Rect.X != theme.Space8 || bs[0].Rect.Y != theme.Space4 || bs[0].Rect.H != theme.ControlHeight {
		t.Errorf("got %+v", bs[0].Rect)
	}
}

func TestToolbar_ChildKeepsItsOwnFocus(t *testing.T) {
	var typed int
	tree := wtest.TextTree(t, overlay.Overlay{Child: Toolbar{Items: []ToolbarItem{
		{Action: interact.Action{Label: "a", OnInvoke: func() {}}},
		{Child: control.Button{Label: "own", OnPress: func() { typed++ }}},
		{Action: interact.Action{Label: "b", OnInvoke: func() {}}},
	}}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyRight})
	wtest.Ops(tree, wtest.Screen)
	n := wtest.ByLabel(t, tree.Semantics(), "b")
	if n.State&semantics.Focused == 0 {
		t.Fatal("the arrows pass a control of one's own by")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
	wtest.Ops(tree, wtest.Screen)
	if n := wtest.ByLabel(t, tree.Semantics(), "own"); n.State&semantics.Focused == 0 {
		t.Error("a control of one's own is a stop of Tab")
	}
}

func TestToolbar_ButtonStates(t *testing.T) {
	th := theme.Default().Light
	for _, tt := range []struct {
		name string
		item ToolbarItem
		fill geom.Color
		ink  geom.Color
	}{
		{"オン", ToolbarItem{Action: interact.Action{Label: "x", Checkable: true, Checked: true, OnInvoke: func() {}}}, th.Color.AccentTint, th.Color.OnTint().AccentInk},
		{"無効", ToolbarItem{Action: interact.Action{Label: "x"}}, geom.Color{}, th.Color.Ink3},
		{"通常", ToolbarItem{Action: interact.Action{Label: "x", OnInvoke: func() {}}}, geom.Color{}, th.Color.Ink},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: basic.SizedBox{Width: 200, Child: Toolbar{Items: []ToolbarItem{tt.item}}}}})
			var fill, ink geom.Color
			for _, op := range wtest.Ops(tree, wtest.Screen) {
				switch op.Kind {
				case paint.OpRRect:
					if op.RRect.Rect.H == theme.ControlHeight {
						fill = op.Color
					}
				case paint.OpGlyphs:
					ink = op.Color
				}
			}
			if fill.A > 0 != (tt.fill.A > 0) || tt.fill.A > 0 && fill != tt.fill {
				t.Errorf("fill %+v, want %+v", fill, tt.fill)
			}
			if ink != tt.ink {
				t.Errorf("ink %+v, want %+v", ink, tt.ink)
			}
		})
	}
}

func TestToolbar_HoverAndPressMoveAButton(t *testing.T) {
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: basic.SizedBox{Width: 200, Child: Toolbar{Items: []ToolbarItem{
		{Action: interact.Action{Label: "x", OnInvoke: func() {}}},
	}}}}})
	rest := testutil.RenderTree(tree, wtest.Screen)
	pos := geom.Offset{X: 14, Y: 14}
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
}

func benchmarkToolbar(b *testing.B, widths [2]float32) {
	items := make([]ToolbarItem, 0, 24)
	for i := range 20 {
		if i%5 == 4 {
			items = append(items, ToolbarItem{Separator: true})
			continue
		}
		items = append(items, ToolbarItem{Action: interact.Action{Label: "item", OnInvoke: func() {}}})
	}
	items = append(items, ToolbarItem{Spacer: true}, ToolbarItem{Action: interact.Action{Label: "end", OnInvoke: func() {}}})
	tree := wtest.TextTree(b, overlay.Overlay{Child: Toolbar{Items: items}})
	sizes := [2]geom.Size{{W: widths[0], H: 100}, {W: widths[1], H: 100}}
	wtest.Ops(tree, sizes[0])
	wtest.Ops(tree, sizes[1])
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		// The two widths hide the same items, so only the layout runs.
		tree.Layout(geom.Tight(sizes[i%2]))
	}
	if tree.NeedsBuild() {
		b.Fatal("the widths must hide the same items")
	}
}

func BenchmarkToolbar_LayoutFits(b *testing.B) { benchmarkToolbar(b, [2]float32{1000, 1400}) }

func BenchmarkToolbar_LayoutOverflows(b *testing.B) { benchmarkToolbar(b, [2]float32{300, 302}) }
