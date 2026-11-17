package cocoa

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestAXRole_EveryRoleMaps(t *testing.T) {
	for r := semantics.RoleNone; r <= semantics.RoleTreeItem; r++ {
		role, _ := axRole(&semantics.Node{Role: r})
		if role == "" {
			t.Errorf("%v has no NSAccessibility role", r)
		}
	}
}

func TestAXRole_DependsOnNode(t *testing.T) {
	tests := []struct {
		name          string
		n             semantics.Node
		role, subrole string
	}{
		{"button", semantics.Node{Role: semantics.RoleButton}, axRoleButton, ""},
		{"switch", semantics.Node{Role: semantics.RoleSwitch}, axRoleCheckBox, axSubroleSwitch},
		{"tab", semantics.Node{Role: semantics.RoleTab}, axRoleRadioButton, axSubroleTabButton},
		{"field", semantics.Node{Role: semantics.RoleTextField}, axRoleTextField, ""},
		{"multiline field", semantics.Node{Role: semantics.RoleTextField, State: semantics.Multiline}, axRoleTextArea, ""},
		{"editable combo box", semantics.Node{Role: semantics.RoleComboBox, Text: &semantics.TextInfo{}}, axRoleComboBox, ""},
		{"dropdown", semantics.Node{Role: semantics.RoleComboBox}, axRolePopUpButton, ""},
		{"plain label", semantics.Node{Label: "Name"}, axRoleStaticText, ""},
		{"plain container", semantics.Node{Children: []*semantics.Node{{ID: 1}}}, axRoleGroup, ""},
		{"tree item", semantics.Node{Role: semantics.RoleTreeItem}, axRoleRow, axSubroleOutlineRow},
		{"dialog", semantics.Node{Role: semantics.RoleDialog}, axRoleGroup, axSubroleApplicationDialog},
	}
	for _, tt := range tests {
		role, subrole := axRole(&tt.n)
		if role != tt.role || subrole != tt.subrole {
			t.Errorf("%s: got %s/%s, want %s/%s", tt.name, role, subrole, tt.role, tt.subrole)
		}
	}
}

func TestAXLabelAndValue(t *testing.T) {
	tests := []struct {
		name  string
		n     semantics.Node
		label string
		value axValue
	}{
		{"button", semantics.Node{Role: semantics.RoleButton, Label: "OK"}, "OK", axValue{kind: axValueNone}},
		{"text is its value", semantics.Node{Role: semantics.RoleText, Label: "Hello"}, "", axValue{kind: axValueString, text: "Hello"}},
		{"text with both", semantics.Node{Role: semantics.RoleText, Label: "Total", Value: "3"}, "Total", axValue{kind: axValueString, text: "3"}},
		{"checked", semantics.Node{Role: semantics.RoleCheckbox, Label: "A", State: semantics.Checked}, "A", axValue{kind: axValueInt, number: 1}},
		{"mixed", semantics.Node{Role: semantics.RoleCheckbox, State: semantics.Checked | semantics.Mixed}, "", axValue{kind: axValueInt, number: 2}},
		{"selected tab", semantics.Node{Role: semantics.RoleTab, Label: "One", State: semantics.Selected}, "One", axValue{kind: axValueInt, number: 1}},
		{"slider", semantics.Node{Role: semantics.RoleSlider, Range: semantics.Range{Max: 10, Value: 4}}, "", axValue{kind: axValueNumber, number: 4}},
		{"heading", semantics.Node{Role: semantics.RoleHeading, Value: "Intro", Level: 2}, "Intro", axValue{kind: axValueInt, number: 2}},
		{"field", semantics.Node{Role: semantics.RoleTextField, Label: "Name", Text: &semantics.TextInfo{Text: "Ada"}}, "Name", axValue{kind: axValueString, text: "Ada"}},
	}
	for _, tt := range tests {
		if got := axLabel(&tt.n); got != tt.label {
			t.Errorf("%s: label %q, want %q", tt.name, got, tt.label)
		}
		if got := axNodeValue(&tt.n); got != tt.value {
			t.Errorf("%s: value %+v, want %+v", tt.name, got, tt.value)
		}
	}
}

func TestAXSetRangeAction_StepsWithoutSetValue(t *testing.T) {
	n := &semantics.Node{Range: semantics.Range{Max: 10, Value: 5}, Actions: semantics.ActionIncrease | semantics.ActionDecrease}
	if a, ok := axSetRangeAction(n, 7); !ok || a != semantics.ActionIncrease {
		t.Errorf("up: got %v %v", a, ok)
	}
	if a, ok := axSetRangeAction(n, 2); !ok || a != semantics.ActionDecrease {
		t.Errorf("down: got %v %v", a, ok)
	}
	if _, ok := axSetRangeAction(n, 11); ok {
		t.Error("out of range accepted")
	}
	n.Actions |= semantics.ActionSetValue
	if a, ok := axSetRangeAction(n, 7); !ok || a != semantics.ActionSetValue {
		t.Errorf("set: got %v %v", a, ok)
	}
}

func TestAXExpandAction_FallsBackToTap(t *testing.T) {
	n := &semantics.Node{State: semantics.Collapsed, Actions: semantics.ActionTap}
	if a, ok := axExpandAction(n, true); !ok || a != semantics.ActionTap {
		t.Errorf("expand by tap: got %v %v", a, ok)
	}
	if _, ok := axExpandAction(n, false); ok {
		t.Error("collapsing what is collapsed")
	}
	n.Actions |= semantics.ActionExpand
	if a, _ := axExpandAction(n, true); a != semantics.ActionExpand {
		t.Errorf("expand: got %v", a)
	}
}

func TestAXSortDirection(t *testing.T) {
	for sort, want := range map[semantics.Sort]int64{
		semantics.SortNone: axSortUnknown, semantics.SortAscending: axSortAscending, semantics.SortDescending: axSortDescending,
	} {
		if got := axSortDirection(&semantics.Node{Role: semantics.RoleColumnHeader, Sort: sort}); got != want {
			t.Errorf("%v: AXSortDirection %d, want %d", sort, got, want)
		}
	}
}

// tree is a window with a list of two items, a field and a slider.
func tree() *semantics.Node {
	return &semantics.Node{Rect: geom.Rect{W: 200, H: 200}, Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleList, Rect: geom.Rect{W: 200, H: 100}, Children: []*semantics.Node{
			{ID: 2, Role: semantics.RoleListItem, Label: "a", State: semantics.Selected, Rect: geom.Rect{W: 200, H: 50}},
			{ID: 3, Role: semantics.RoleListItem, Label: "b", Rect: geom.Rect{Y: 50, W: 200, H: 50}},
		}},
		{ID: 4, Role: semantics.RoleTextField, Label: "Name", Rect: geom.Rect{Y: 100, W: 200, H: 50},
			Text: &semantics.TextInfo{Text: "ab\ncd", SelectionStart: 4, SelectionEnd: 1, LineStarts: []int{3},
				LineRects: []geom.Rect{{X: 2, Y: 100, W: 20, H: 10}, {X: 2, Y: 110, W: 30, H: 10}}}},
		{ID: 5, Role: semantics.RoleSlider, Range: semantics.Range{Max: 1}, Rect: geom.Rect{Y: 150, W: 200, H: 50}},
	}}
}

func TestAXTree_HitTestAndSelection(t *testing.T) {
	tr := newAXTree(tree())
	if got := tr.hitTest(geom.Offset{X: 10, Y: 60}); got != 3 {
		t.Errorf("hit test: got %d, want 3", got)
	}
	if got := tr.hitTest(geom.Offset{X: 10, Y: 300}); got != 0 {
		t.Errorf("outside: got %d", got)
	}
	if got := tr.selection(1); len(got) != 1 || got[0] != 2 {
		t.Errorf("selection: got %v", got)
	}
	if c, ok := tr.selectionContainer(3); !ok || c != 1 {
		t.Errorf("container: got %d %v", c, ok)
	}
	if tr.parents[2] != 1 || tr.parents[1] != 0 {
		t.Errorf("parents: %v", tr.parents)
	}
}

func TestAXText_RangesInUTF16(t *testing.T) {
	n := tree().Children[1]
	s := semantics.UTF16(n.Text.Text)
	start, length := axSelection(n.Text, len(s))
	if start != 1 || length != 3 {
		t.Errorf("selection: %d+%d", start, length)
	}
	if got := axSubstring(s, start, start+length); got != "b\nc" {
		t.Errorf("substring %q", got)
	}
	if got := axSubstring(semantics.UTF16("a😀b"), 1, 3); got != "😀" {
		t.Errorf("surrogates: %q", got)
	}
	if r := axRangeRect(n, 1, 3); r != (geom.Rect{X: 2, Y: 100, W: 30, H: 20}) {
		t.Errorf("rect of two lines: %+v", r)
	}
	if r := axRangeRect(n, 3, 1); r != n.Text.LineRects[1] {
		t.Errorf("rect of second line: %+v", r)
	}
}

func TestAXNotes(t *testing.T) {
	before := tree()
	after := tree()
	// The selection moves to b, the field's text changes, the slider moves,
	// and the focus goes to the field.
	after.Children[0].Children[0].State = 0
	after.Children[0].Children[1].State = semantics.Selected
	after.Children[1].Text.Text = "abc"
	after.Children[2].Range.Value = 0.5
	after.Children[2].Live = semantics.LiveAssertive
	before.Children[2].Live = semantics.LiveAssertive

	var d semantics.Diff
	semantics.Compare(before, after, 0, 4, &d)
	b, a := newAXTree(before), newAXTree(after)
	got := axNotes(&b, &a, &d, 4)
	want := map[axNote]bool{
		{kind: noteSelectedChildren, id: 1}: true,
		{kind: noteValue, id: 4}:            true,
		{kind: noteValue, id: 5}:            true,
		{kind: noteFocus, id: 4}:            true,
	}
	for _, n := range got {
		if n.kind == noteLayout {
			t.Errorf("layout without a change of structure")
		}
		delete(want, n)
	}
	if len(want) != 0 {
		t.Errorf("missing %v in %v", want, got)
	}

	// A new child changes the layout.
	after2 := tree()
	after2.Children = append(after2.Children, &semantics.Node{ID: 9, Role: semantics.RoleText, Label: "new", Live: semantics.LivePolite})
	semantics.Compare(before, after2, 0, 0, &d)
	a2 := newAXTree(after2)
	got = axNotes(&b, &a2, &d, 0)
	if len(got) == 0 || got[0].kind != noteLayout {
		t.Errorf("added node: got %v", got)
	}
	for _, n := range got {
		if n.kind == noteAnnounce {
			t.Errorf("a live region spoke on appearing: %v", n)
		}
	}
}

func TestAXNotes_LiveRegionSpeaks(t *testing.T) {
	before := &semantics.Node{Children: []*semantics.Node{{ID: 1, Role: semantics.RoleText, Label: "Saving", Live: semantics.LivePolite}}}
	after := &semantics.Node{Children: []*semantics.Node{{ID: 1, Role: semantics.RoleText, Label: "Saved", Live: semantics.LivePolite}}}
	var d semantics.Diff
	semantics.Compare(before, after, 0, 0, &d)
	b, a := newAXTree(before), newAXTree(after)
	found := false
	for _, n := range axNotes(&b, &a, &d, 0) {
		if n.kind == noteAnnounce {
			found = true
			if n.text != "Saved" || n.assertive {
				t.Errorf("announcement %+v", n)
			}
		}
	}
	if !found {
		t.Error("no announcement")
	}
}

func TestAXTree_RowsOfTree(t *testing.T) {
	root := &semantics.Node{Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleTree, Children: []*semantics.Node{
			{ID: 2, Role: semantics.RoleTreeItem, Level: 1, State: semantics.Expanded, Children: []*semantics.Node{
				{ID: 3, Role: semantics.RoleTreeItem, Level: 2},
			}},
			{ID: 4, Role: semantics.RoleTreeItem, Level: 1},
		}},
	}}
	tr := newAXTree(root)
	if got := tr.rows(1); len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Errorf("rows %v", got)
	}
	if tr.disclosedBy(3) != 2 || tr.disclosedBy(2) != 0 {
		t.Error("disclosedBy")
	}
	if got := tr.disclosed(2); len(got) != 1 || got[0] != 3 {
		t.Errorf("disclosed %v", got)
	}
}

func TestAXFlags(t *testing.T) {
	tests := []struct {
		name                          string
		n                             semantics.Node
		expandable, enabled, settable bool
	}{
		{"plain", semantics.Node{}, false, true, false},
		{"collapsed", semantics.Node{State: semantics.Collapsed}, true, true, false},
		{"expanded", semantics.Node{State: semantics.Expanded}, true, true, false},
		{"field", semantics.Node{Actions: semantics.ActionSetValue}, false, true, true},
		{"disabled field", semantics.Node{State: semantics.Disabled, Actions: semantics.ActionSetValue}, false, false, false},
		{"read-only field", semantics.Node{State: semantics.ReadOnly, Actions: semantics.ActionSetValue}, false, true, false},
		{"stepper", semantics.Node{Range: semantics.Range{Max: 10}, Actions: semantics.ActionIncrease}, false, true, true},
		{"steps without a range", semantics.Node{Actions: semantics.ActionIncrease}, false, true, false},
	}
	for _, tt := range tests {
		if got := axExpandable(&tt.n); got != tt.expandable {
			t.Errorf("%s: expandable %v, want %v", tt.name, got, tt.expandable)
		}
		if got := axEnabled(&tt.n); got != tt.enabled {
			t.Errorf("%s: enabled %v, want %v", tt.name, got, tt.enabled)
		}
		if got := axCanSetValue(&tt.n); got != tt.settable {
			t.Errorf("%s: settable %v, want %v", tt.name, got, tt.settable)
		}
	}
}
