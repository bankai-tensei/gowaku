package collection

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/semantics"
)

// countLabel counts the nodes below root that say label.
func countLabel(root *semantics.Node, label string) int {
	n := 0
	root.Walk(func(m *semantics.Node) bool {
		if m.Label == label {
			n++
		}
		return true
	})
	return n
}

func TestGroupRow_LabelNamesTheControlOnce(t *testing.T) {
	nothing := func(string) {}
	cases := []struct {
		name  string
		child core.Widget
		role  semantics.Role
	}{
		{"field.Input", basic.SizedBox{Width: 160, Child: field.Input{OnChange: nothing}}, semantics.RoleTextField},
		{"DropdownMenu", field.DropdownMenu[string]{Value: "b", Options: []field.DropdownOption[string]{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}, OnChange: nothing}, semantics.RoleComboBox},
		{"Switch", control.Switch{OnChange: func(bool) {}}, semantics.RoleSwitch},
		{"Autocomplete", basic.SizedBox{Width: 160, Child: field.Autocomplete[string]{Options: []string{"Oslo", "Osaka"}, OnChange: nothing}}, semantics.RoleComboBox},
		{"same label of its own", control.Slider{Label: "Volume", OnChange: func(float32) {}}, semantics.RoleSlider},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label := "Volume"
			tree := wtest.A11yTree(t, Group{Title: "Settings", Children: []core.Widget{GroupRow{Label: label, Child: tc.child}}})
			root, _ := tree.SemanticsTree()
			n := wtest.ByLabel(t, root, label)
			if n.Role != tc.role {
				t.Errorf("the label went to a %v, want the %v", n.Role, tc.role)
			}
			if got := countLabel(root, label); got != 1 {
				t.Errorf("%q is said %d times, want once", label, got)
			}
		})
	}
}

func TestGroupRow_ControlWithAnotherLabelKeepsTheText(t *testing.T) {
	tree := wtest.A11yTree(t, GroupRow{Label: "Sounds", Detail: "A short chime", Child: control.Switch{Label: "Chime", OnChange: func(bool) {}}})
	root, _ := tree.SemanticsTree()
	if n := wtest.ByLabel(t, root, "Sounds"); n.Role != semantics.RoleText {
		t.Errorf("the row text is a %v, want text", n.Role)
	}
	if n := wtest.ByLabel(t, root, "Chime"); n.Role != semantics.RoleSwitch {
		t.Errorf("the switch is a %v", n.Role)
	}
	wtest.ByLabel(t, root, "A short chime")
}

func TestGroupRow_WithoutAControlIsText(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Column{Children: []core.Widget{
		GroupRow{Label: "Version"},
		GroupRow{Label: "Build", Child: basic.Text{Content: "1.2.0"}},
		// A control after them is not named by an earlier row.
		control.Switch{Label: "Version", OnChange: func(bool) {}},
	}})
	root, _ := tree.SemanticsTree()
	if got := countLabel(root, "Version"); got != 2 {
		t.Errorf("Version is said %d times, want the text and the switch", got)
	}
	if n := wtest.ByLabel(t, root, "Build"); n.Role != semantics.RoleText {
		t.Errorf("Build is a %v", n.Role)
	}
	wtest.ByLabel(t, root, "1.2.0")
}

func TestGroup_TitleNamesTheGroup(t *testing.T) {
	tree := wtest.A11yTree(t, Group{Title: "Account", Children: []core.Widget{GroupRow{Label: "Name"}}})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "Account")
	if n.Role != semantics.RoleGroup {
		t.Errorf("the title names a %v", n.Role)
	}
	if got := countLabel(root, "Account"); got != 1 {
		t.Errorf("the title is said %d times", got)
	}
}

func TestSidebar_ListOfItems(t *testing.T) {
	pressed := 0
	tree := wtest.A11yTree(t, Sidebar{Title: "Library", Children: []core.Widget{
		SidebarItem{Label: "Songs", Selected: true, OnPress: func() { pressed++ }},
		SidebarItem{Label: "Albums", OnPress: func() { pressed++ }},
	}})
	root, _ := tree.SemanticsTree()
	list := wtest.ByLabel(t, root, "Library")
	if list.Role != semantics.RoleList || countLabel(root, "Library") != 1 {
		t.Fatalf("the title must name the list once: %v, %d times", list.Role, countLabel(root, "Library"))
	}
	songs, albums := wtest.ByLabel(t, list, "Songs"), wtest.ByLabel(t, list, "Albums")
	if songs.Role != semantics.RoleListItem || songs.State&semantics.Selected == 0 {
		t.Errorf("Songs: %v %b", songs.Role, songs.State)
	}
	if albums.State&semantics.Selected != 0 {
		t.Error("Albums is selected")
	}
	if !tree.PerformSemanticsAction(albums.ID, semantics.ActionTap, core.SemanticsArg{}) || pressed != 1 {
		t.Errorf("Tap pressed %d times", pressed)
	}
}
