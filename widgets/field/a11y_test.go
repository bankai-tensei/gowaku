package field

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestA11y_ButtonSpeaksForItsCaption(t *testing.T) {
	pressed := 0
	tree := wtest.A11yTree(t, control.Button{Label: "保存", OnPress: func() { pressed++ }})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "保存")
	if n.Role != semantics.RoleButton || len(n.Children) != 0 {
		t.Errorf("the caption must not be read again: %+v", n.Children)
	}
	if !tree.PerformSemanticsAction(n.ID, semantics.ActionTap, core.SemanticsArg{}) || pressed != 1 {
		t.Errorf("Tap pressed %d times", pressed)
	}
}

func TestA11y_DisabledButton(t *testing.T) {
	tree := wtest.A11yTree(t, control.Button{Label: "保存"})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "保存")
	if n.State&semantics.Disabled == 0 || n.State&semantics.Focusable != 0 || n.Actions&semantics.ActionTap != 0 {
		t.Errorf("state %b actions %b", n.State, n.Actions)
	}
}

func TestA11y_CheckboxSwitchSegments(t *testing.T) {
	var picked int
	tree := wtest.A11yTree(t, basic.Column{Children: []core.Widget{
		control.Checkbox{Label: "完了", Value: true, OnChange: func(bool) {}},
		control.Switch{Label: "通知", OnChange: func(bool) {}},
		control.SegmentedControl{Label: "表示", Options: []string{"日", "週"}, Selected: 1, OnChange: func(i int) { picked = i + 10 }},
	}})
	root, _ := tree.SemanticsTree()
	if n := wtest.ByLabel(t, root, "完了"); n.Role != semantics.RoleCheckbox || n.State&semantics.Checked == 0 {
		t.Errorf("checkbox: %+v", n)
	}
	if n := wtest.ByLabel(t, root, "通知"); n.Role != semantics.RoleSwitch || n.State&semantics.Checked != 0 {
		t.Errorf("switch: %+v", n)
	}
	group := wtest.ByLabel(t, root, "表示")
	if group.Role != semantics.RoleRadioGroup || len(group.Children) != 2 {
		t.Fatalf("segments: %+v", group)
	}
	day, week := group.Children[0], group.Children[1]
	if day.Role != semantics.RoleRadio || day.State&semantics.Selected != 0 || week.State&semantics.Selected == 0 {
		t.Errorf("radios: %+v %+v", day, week)
	}
	tree.PerformSemanticsAction(day.ID, semantics.ActionTap, core.SemanticsArg{})
	if picked != 10 {
		t.Errorf("picking 日 gave %d", picked)
	}
}

func TestA11y_SliderRange(t *testing.T) {
	var v float32 = 0.5
	tree := wtest.A11yTree(t, control.Slider{Label: "音量", Value: v, OnChange: func(x float32) { v = x }})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "音量")
	if n.Range.Max != 1 || n.Range.Value != 0.5 {
		t.Errorf("range %+v", n.Range)
	}
	tree.PerformSemanticsAction(n.ID, semantics.ActionSetValue, core.SemanticsArg{Number: 2})
	if v != 1 {
		t.Errorf("SetValue past Max gave %v, want 1", v)
	}
	tree.PerformSemanticsAction(n.ID, semantics.ActionDecrease, core.SemanticsArg{})
	if v >= 0.5 {
		t.Errorf("Decrease left %v", v)
	}
}

func TestA11y_TextField(t *testing.T) {
	var got string
	tree := wtest.A11yTree(t, Input{Label: "件名", Value: "日本語 text", OnChange: func(s string) { got = s }})
	root, _ := tree.SemanticsTree()
	n := wtest.FindRole(root, semantics.RoleTextField)
	if n == nil || n.Label != "件名" || n.Text == nil || n.Text.Text != "日本語 text" || n.State&semantics.Focusable == 0 {
		t.Fatalf("field: %+v", n)
	}
	if len(n.Text.LineRects) != 1 || n.Text.LineRects[0].W <= 0 {
		t.Errorf("line rects %+v", n.Text.LineRects)
	}
	// Select "text", which starts after 3 kanji and a space: 4 UTF-16 units.
	tree.PerformSemanticsAction(n.ID, semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 4, SelectionEnd: 8})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	root, _ = tree.SemanticsTree()
	if ti := wtest.FindRole(root, semantics.RoleTextField).Text; ti.SelectionStart != 4 || ti.SelectionEnd != 8 {
		t.Errorf("selection %d..%d", ti.SelectionStart, ti.SelectionEnd)
	}
	tree.PerformSemanticsAction(n.ID, semantics.ActionSetValue, core.SemanticsArg{Text: "新しい"})
	if got != "新しい" {
		t.Errorf("SetValue gave %q", got)
	}
}

func TestA11y_DialogAndHeading(t *testing.T) {
	tree := wtest.A11yTree(t, overlay.Dialog{Title: "削除しますか", Message: "元に戻せません"})
	root, _ := tree.SemanticsTree()
	d := wtest.ByLabel(t, root, "削除しますか")
	if d.Role != semantics.RoleDialog || d.Hint != "元に戻せません" {
		t.Fatalf("dialog %+v", d)
	}
	var heading *semantics.Node
	d.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleHeading {
			heading = n
		}
		return heading == nil
	})
	if heading == nil || heading.Level != 1 || len(heading.Children) != 0 {
		t.Errorf("heading %+v", heading)
	}
}

func TestA11y_FocusTraversalOrder(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Column{Children: []core.Widget{
		control.Button{Label: "a", OnPress: func() {}},
		interact.FocusTraversalOrder{Order: 1, Child: control.Button{Label: "b", OnPress: func() {}}},
	}})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	root, focus := tree.SemanticsTree()
	if b := wtest.ByLabel(t, root, "b"); focus != b.ID {
		t.Errorf("Tab went to %d, want b (%d)", focus, b.ID)
	}
}

func TestA11y_FocusFollowsTheChoice(t *testing.T) {
	tree := wtest.A11yTree(t, control.SegmentedControl{Label: "表示", Options: []string{"日", "週"}, Selected: 1, OnChange: func(int) {}})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	root, focus := tree.SemanticsTree()
	week := wtest.ByLabel(t, root, "週")
	if focus != week.ID || week.State&semantics.Focused == 0 {
		t.Errorf("focus %d, want the chosen radio %d", focus, week.ID)
	}
	if wtest.ByLabel(t, root, "表示").State&semantics.Focused != 0 {
		t.Error("the group must hand its focus to the choice")
	}
}
