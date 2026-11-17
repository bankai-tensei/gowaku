package field

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func TestActions_FromATextFieldButNotUnderADialog(t *testing.T) {
	var ran int
	var ctx core.BuildContext
	save := interact.Action{Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl}, OnInvoke: func() { ran++ }}
	tree := wtest.TextTree(t, overlay.Overlay{Child: interact.Actions{Actions: []interact.Action{save}, Child: wtest.Capture{Got: &ctx, Child: basic.Column{
		Children: []core.Widget{Input{Value: "abc", OnChange: func(string) {}}},
	}}}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() == nil {
		t.Fatal("the field must take the focus")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl})
	if ran != 1 {
		t.Fatalf("Ctrl+S in a field ran %d times, want 1", ran)
	}
	overlay.ShowDialog(ctx, func(func()) core.Widget { return overlay.Dialog{Title: "t"} })
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyS, Modifiers: input.ModControl})
	if ran != 1 {
		t.Error("a modal dialog must hold back the shortcuts of the window")
	}
}

// menuApp is a window with a menu bar over a text field, and what its items did.
type menuApp struct {
	tree  *core.Tree
	ran   []string
	field core.FocusTarget
}

func newMenuApp(t *testing.T) *menuApp {
	t.Helper()
	a := &menuApp{}
	act := func(label string, sc interact.Shortcut) interact.Action {
		return interact.Action{Label: label, Shortcut: sc, OnInvoke: func() { a.ran = append(a.ran, kit.ParseMnemonic(label).Text) }}
	}
	bar := overlay.MenuBar{Items: []overlay.MenuItem{
		{Action: interact.Action{Label: "ファイル(&F)"}, Items: []overlay.MenuItem{
			{Action: act("新規(&N)", ctrlN)},
			{Action: act("開く(&O)", interact.Shortcut{})},
			{Separator: true},
			{Action: interact.Action{Label: "最近(&R)"}, Items: []overlay.MenuItem{{Action: act("a.txt", interact.Shortcut{})}, {Action: act("b.txt", interact.Shortcut{})}}},
			{Action: interact.Action{Label: "印刷(&P)"}},
		}},
		{Action: interact.Action{Label: "編集(&E)"}, Items: []overlay.MenuItem{
			{Action: act("元に戻す(&U)", interact.Shortcut{Key: input.KeyZ, Modifiers: input.ModControl})},
		}},
	}}
	a.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Column{Cross: 0, Children: []core.Widget{
		bar,
		Input{Value: "abc", OnChange: func(string) {}},
	}}})
	wtest.Ops(a.tree, wtest.Screen)
	a.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	a.field = a.tree.Focused()
	if a.field == nil {
		t.Fatal("the field must take the focus")
	}
	return a
}

func (a *menuApp) Key(k input.KeyCode, m input.Modifiers) {
	a.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	wtest.Ops(a.tree, wtest.Screen)
}

func (a *menuApp) alt() {
	a.tree.DispatchKey(core.KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt})
	a.tree.DispatchKeyUp(core.KeyEvent{Key: input.KeyAlt})
	wtest.Ops(a.tree, wtest.Screen)
}

// focusedNode returns the node of what has the focus.
func (a *menuApp) focusedNode() *semantics.Node {
	root, focus := a.tree.SemanticsTree()
	return root.Find(focus)
}

func (a *menuApp) node(label string) *semantics.Node {
	root, _ := a.tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == label {
			found = n
			return false
		}
		return true
	})
	return found
}

func (a *menuApp) click(label string, button input.Button) {
	n := a.node(label)
	if n == nil {
		panic("no node " + label)
	}
	pos := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
	a.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: button})
	a.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: button})
	wtest.Ops(a.tree, wtest.Screen)
}

func (a *menuApp) wantFocus(t *testing.T, label string) {
	t.Helper()
	n := a.focusedNode()
	if n == nil || n.Label != label {
		got := "<nil>"
		if n != nil {
			got = n.Label
		}
		t.Fatalf("focus on %q, want %q", got, label)
	}
}

func TestMenuBar_AltArrowsEnterRunAndGiveTheFocusBack(t *testing.T) {
	a := newMenuApp(t)
	a.alt()
	a.wantFocus(t, "ファイル(F)")
	a.Key(input.KeyRight, 0)
	a.wantFocus(t, "編集(E)")
	a.Key(input.KeyLeft, 0)
	a.Key(input.KeyDown, 0)
	a.wantFocus(t, "新規(N)")
	if n := a.node("ファイル(F)"); n.State&semantics.Expanded == 0 {
		t.Error("the open title must say it is expanded")
	}
	a.Key(input.KeyDown, 0)
	a.wantFocus(t, "開く(O)")
	a.Key(input.KeyDown, 0)
	a.wantFocus(t, "最近(R)")
	a.Key(input.KeyUp, 0)
	a.Key(input.KeyEnter, 0)
	if len(a.ran) != 1 || a.ran[0] != "開く(O)" {
		t.Fatalf("ran %v", a.ran)
	}
	if a.tree.Focused() != a.field {
		t.Error("the field must get the focus back")
	}
	if a.node("新規(N)") != nil {
		t.Error("the menu must close")
	}
}

func TestMenuBar_AltLetterSubmenuAndEscape(t *testing.T) {
	a := newMenuApp(t)
	a.Key(input.KeyF, input.ModAlt)
	a.wantFocus(t, "新規(N)")
	a.Key(input.KeyR, 0)
	a.wantFocus(t, "a.txt")
	a.Key(input.KeyLeft, 0)
	a.wantFocus(t, "最近(R)")
	if a.node("a.txt") != nil {
		t.Error("Left must close the submenu")
	}
	a.Key(input.KeyRight, 0)
	a.wantFocus(t, "a.txt")
	a.Key(input.KeyRight, 0)
	a.wantFocus(t, "元に戻す(U)")
	a.Key(input.KeyEscape, 0)
	a.wantFocus(t, "編集(E)")
	a.Key(input.KeyEscape, 0)
	if a.tree.Focused() != a.field {
		t.Error("the second Escape must give the field the focus back")
	}
	if len(a.ran) != 0 {
		t.Errorf("nothing should run, got %v", a.ran)
	}
}

func TestMenuBar_DisabledItemDoesNothingAndAltCloses(t *testing.T) {
	a := newMenuApp(t)
	a.Key(input.KeyF, input.ModAlt)
	a.Key(input.KeyEnd, 0)
	a.wantFocus(t, "印刷(P)")
	if n := a.focusedNode(); n.State&semantics.Disabled == 0 {
		t.Error("an item without OnInvoke must be disabled")
	}
	a.Key(input.KeyEnter, 0)
	a.wantFocus(t, "印刷(P)")
	a.alt()
	if a.tree.Focused() != a.field || a.node("新規(N)") != nil {
		t.Error("Alt must close the menus and give the focus back")
	}
}

func TestMenuBar_PointerOpensSwitchesAndRuns(t *testing.T) {
	a := newMenuApp(t)
	a.click("ファイル(F)", input.ButtonPrimary)
	if a.node("新規(N)") == nil {
		t.Fatal("a press on the title must open its menu")
	}
	// The pointer moving over the next title opens its menu instead.
	e := a.node("編集(E)").Rect
	a.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: e.X + 4, Y: e.Y + 4}})
	wtest.Ops(a.tree, wtest.Screen)
	if a.node("新規(N)") != nil || a.node("元に戻す(U)") == nil {
		t.Fatal("moving onto the next title must switch menus")
	}
	a.click("元に戻す(U)", input.ButtonPrimary)
	if len(a.ran) != 1 || a.ran[0] != "元に戻す(U)" {
		t.Fatalf("ran %v", a.ran)
	}
	if a.tree.Focused() != a.field {
		t.Error("the field must keep the focus after a menu of the pointer")
	}
}

func TestMenuBar_PressBesideClosesAndShortcutsWork(t *testing.T) {
	a := newMenuApp(t)
	a.click("ファイル(F)", input.ButtonPrimary)
	a.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 390, Y: 290}})
	wtest.Ops(a.tree, wtest.Screen)
	if a.node("新規(N)") != nil {
		t.Fatal("a press beside the menu must close it")
	}
	a.Key(input.KeyN, input.ModControl)
	if len(a.ran) != 1 || a.ran[0] != "新規(N)" {
		t.Errorf("the shortcut of a menu item must work without Actions, ran %v", a.ran)
	}
}

func TestMenuBar_WindowBlurCloses(t *testing.T) {
	a := newMenuApp(t)
	a.Key(input.KeyF, input.ModAlt)
	a.tree.SetWindowFocused(false)
	wtest.Ops(a.tree, wtest.Screen)
	if a.node("新規(N)") != nil {
		t.Fatal("losing the keyboard must close the menus")
	}
	a.tree.SetWindowFocused(true)
	if a.tree.Focused() != a.field {
		t.Error("the field must have the focus when the window comes back")
	}
}

func TestMenuBar_AltTapNeedsNothingInBetween(t *testing.T) {
	a := newMenuApp(t)
	a.tree.DispatchKey(core.KeyEvent{Key: input.KeyAlt, Modifiers: input.ModAlt})
	a.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModAlt})
	a.tree.DispatchKeyUp(core.KeyEvent{Key: input.KeyAlt})
	wtest.Ops(a.tree, wtest.Screen)
	if n := a.focusedNode(); n != nil && n.Role == semantics.RoleMenuItem {
		t.Error("Alt with another key is a chord, not a way to the menu bar")
	}
}

func TestContextMenu_SecondaryClickAndMenuKey(t *testing.T) {
	var ran []string
	items := []overlay.MenuItem{
		{Action: interact.Action{Label: "コピー(&C)", Shortcut: interact.Shortcut{Key: input.KeyC, Modifiers: input.ModControl},
			OnInvoke: func() { ran = append(ran, "copy") }}},
		{Action: interact.Action{Label: "折り返し", Checkable: true, Checked: true, OnInvoke: func() {}}},
	}
	tree := wtest.TextTree(t, overlay.Overlay{Child: overlay.ContextMenu{Items: items, Child: basic.Column{Children: []core.Widget{
		control.Button{Label: "target", OnPress: func() {}},
	}}}})
	a := &menuApp{tree: tree}
	wtest.Ops(tree, wtest.Screen)
	a.click("target", input.ButtonSecondary)
	copyNode := a.node("コピー(C)")
	if copyNode == nil {
		t.Fatal("a secondary click must open the menu")
	}
	if copyNode.Shortcut != "Ctrl+C" {
		t.Errorf("the item must tell readers its shortcut, got %q", copyNode.Shortcut)
	}
	if n := a.node("折り返し"); n.Role != semantics.RoleMenuItemCheckbox || n.State&semantics.Checked == 0 {
		t.Error("a checked action must be a checked menu checkbox")
	}
	a.Key(input.KeyEscape, 0)
	if a.node("コピー(C)") != nil {
		t.Fatal("Escape must close the menu")
	}

	a.Key(input.KeyTab, 0)
	button := tree.Focused()
	a.Key(input.KeyF10, input.ModShift)
	a.wantFocus(t, "コピー(C)")
	a.Key(input.KeyC, 0)
	if len(ran) != 1 {
		t.Fatalf("the mnemonic must run the item, ran %v", ran)
	}
	if tree.Focused() != button {
		t.Error("the button must get the focus back")
	}
}

var ctrlN = interact.Shortcut{Key: input.KeyN, Modifiers: input.ModControl}
