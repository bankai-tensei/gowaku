package field

import (
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func fruits() []DropdownOption[string] {
	return []DropdownOption[string]{
		{Value: "apple", Label: "Apple"},
		{Value: "apricot", Label: "Apricot"},
		{Value: "banana", Label: "Banana"},
		{Value: "blueberry", Label: "Blueberry", Disabled: true},
		{Value: "cherry", Label: "Cherry"},
		{Value: "avocado", Label: "Avocado"},
	}
}

// dropdownRig is a window with a fruit dropdown that keeps its value, and a
// button after it.
func dropdownRig(t *testing.T, initial string, changes *[]string) *fieldRig {
	t.Helper()
	r := newFieldRig(t, formHolder[string]{Initial: initial, Build: func(v string, set func(string)) core.Widget {
		return basic.Column{Spacing: 8, Children: []core.Widget{
			DropdownMenu[string]{Label: "Fruit", Value: v, Placeholder: "Pick one", Options: fruits(), OnChange: func(s string) {
				*changes = append(*changes, s)
				set(s)
			}},
			control.Button{Label: "next", OnPress: func() {}},
		}}
	}})
	r.Key(input.KeyTab, 0)
	r.wantFocus(semantics.RoleComboBox, "Fruit")
	return r
}

func (r *fieldRig) wantValue(want string) {
	r.T.Helper()
	if got := r.must(semantics.RoleComboBox, "Fruit").Value; got != want {
		r.T.Errorf("value %q, want %q", got, want)
	}
}

func (r *fieldRig) wantOpen(open bool) {
	r.T.Helper()
	n := r.must(semantics.RoleComboBox, "Fruit")
	if got := n.State&semantics.Expanded != 0; got != open || n.State&semantics.Collapsed != 0 == open {
		r.T.Fatalf("expanded %v, want %v (state %b)", got, open, n.State)
	}
	if got := r.Find(semantics.RoleList, "Fruit") != nil; got != open {
		r.T.Fatalf("list shown %v, want %v", got, open)
	}
}

func TestDropdownMenu_KeysOpenTheList(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  input.KeyCode
		mods input.Modifiers
	}{
		{"Space", input.KeySpace, 0},
		{"Enter", input.KeyEnter, 0},
		{"Alt+Down", input.KeyDown, input.ModAlt},
		{"F4", input.KeyF4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var changes []string
			r := dropdownRig(t, "banana", &changes)
			r.Key(tc.key, tc.mods)
			r.wantOpen(true)
			r.wantFocus(semantics.RoleListItem, "Banana")
			if n := r.must(semantics.RoleListItem, "Banana"); n.State&semantics.Selected == 0 {
				t.Error("the current choice must be highlighted")
			}
			r.Key(input.KeyEscape, 0)
			r.wantOpen(false)
			r.wantFocus(semantics.RoleComboBox, "Fruit")
			if len(changes) != 0 {
				t.Errorf("Escape must keep the value: %q", changes)
			}
		})
	}
}

func TestDropdownMenu_ArrowsSkipDisabledAndEnterChooses(t *testing.T) {
	var changes []string
	r := dropdownRig(t, "banana", &changes)
	r.Key(input.KeySpace, 0)
	r.Key(input.KeyDown, 0)
	r.wantFocus(semantics.RoleListItem, "Cherry")
	r.Key(input.KeyUp, 0)
	r.Key(input.KeyUp, 0)
	r.wantFocus(semantics.RoleListItem, "Apricot")
	r.Key(input.KeyEnd, 0)
	r.wantFocus(semantics.RoleListItem, "Avocado")
	r.Key(input.KeyHome, 0)
	r.wantFocus(semantics.RoleListItem, "Apple")
	r.Key(input.KeyEnter, 0)
	r.wantOpen(false)
	r.wantFocus(semantics.RoleComboBox, "Fruit")
	r.wantValue("Apple")
	if len(changes) != 1 || changes[0] != "apple" {
		t.Errorf("got %q", changes)
	}
}

func TestDropdownMenu_TypeAhead(t *testing.T) {
	var changes []string
	r := dropdownRig(t, "", &changes)
	r.wantValue("")
	r.Key(input.KeySpace, 0)
	r.wantFocus(semantics.RoleListItem, "Apple")
	// Letters in quick succession build a prefix.
	r.Key(input.KeyA, 0)
	r.Key(input.KeyV, 0)
	r.wantFocus(semantics.RoleListItem, "Avocado")
	// After a pause the search starts anew; one letter again and again
	// cycles through the labels with it.
	r.wait(2 * time.Second)
	r.wait(2 * time.Second)
	r.Key(input.KeyA, 0)
	r.wantFocus(semantics.RoleListItem, "Apple")
	r.Key(input.KeyA, 0)
	r.wantFocus(semantics.RoleListItem, "Apricot")
	r.Key(input.KeyA, 0)
	r.wantFocus(semantics.RoleListItem, "Avocado")
	r.wait(2 * time.Second)
	r.wait(2 * time.Second)
	// A disabled option is never a match.
	r.Key(input.KeyB, 0)
	r.Key(input.KeyL, 0)
	r.wantFocus(semantics.RoleListItem, "Banana")
	if len(changes) != 0 {
		t.Errorf("moving in the list must not choose: %q", changes)
	}
}

func TestDropdownMenu_ClosedFieldChangesWithArrowsAndLetters(t *testing.T) {
	var changes []string
	r := dropdownRig(t, "banana", &changes)
	r.Key(input.KeyDown, 0)
	r.wantValue("Cherry")
	r.Key(input.KeyUp, 0)
	r.wantValue("Banana")
	r.Key(input.KeyEnd, 0)
	r.wantValue("Avocado")
	r.Key(input.KeyHome, 0)
	r.wantValue("Apple")
	r.Key(input.KeyUp, 0)
	r.wantValue("Apple")
	r.Key(input.KeyC, 0)
	r.wantValue("Cherry")
	r.wantOpen(false)
	want := []string{"cherry", "banana", "avocado", "apple", "cherry"}
	if len(changes) != len(want) {
		t.Fatalf("got %q, want %q", changes, want)
	}
	for i := range want {
		if changes[i] != want[i] {
			t.Errorf("change %d: got %q, want %q", i, changes[i], want[i])
		}
	}
}

func TestDropdownMenu_PointerOpensAndChooses(t *testing.T) {
	var changes []string
	r := dropdownRig(t, "banana", &changes)
	r.click(r.must(semantics.RoleComboBox, "Fruit"))
	r.wantOpen(true)
	if l, f := r.must(semantics.RoleList, "Fruit").Rect, r.must(semantics.RoleComboBox, "Fruit").Rect; l.Y < f.Y+f.H {
		t.Errorf("the list must open below the field: %+v, %+v", l, f)
	}
	r.click(r.must(semantics.RoleListItem, "Blueberry"))
	r.wantOpen(true)
	r.click(r.must(semantics.RoleListItem, "Cherry"))
	r.wantOpen(false)
	r.wantValue("Cherry")
	// A press beside the list closes it and changes nothing.
	r.click(r.must(semantics.RoleComboBox, "Fruit"))
	r.wantOpen(true)
	wtest.Tap(r.tree, geom.Offset{X: 450, Y: 350})
	r.frame()
	r.wantOpen(false)
	if len(changes) != 1 {
		t.Errorf("got %q", changes)
	}
}

func TestDropdownMenu_OpensAboveAtTheBottomEdge(t *testing.T) {
	r := newFieldRig(t, basic.Column{Children: []core.Widget{
		basic.SizedBox{Height: 300},
		DropdownMenu[string]{Label: "Fruit", Options: fruits(), OnChange: func(string) {}},
	}})
	r.act(r.must(semantics.RoleComboBox, "Fruit"), semantics.ActionExpand)
	if l, f := r.must(semantics.RoleList, "Fruit").Rect, r.must(semantics.RoleComboBox, "Fruit").Rect; l.Y+l.H > f.Y {
		t.Errorf("the list must flip above the field: %+v, %+v", l, f)
	}
}

func TestDropdownMenu_Semantics(t *testing.T) {
	var changes []string
	r := dropdownRig(t, "cherry", &changes)
	n := r.must(semantics.RoleComboBox, "Fruit")
	if n.Value != "Cherry" || n.Actions&semantics.ActionExpand == 0 || n.Actions&semantics.ActionCollapse == 0 {
		t.Errorf("got %+v", n)
	}
	r.wantOpen(false)
	r.act(n, semantics.ActionExpand)
	r.wantOpen(true)
	items := r.all(semantics.RoleListItem)
	if len(items) != len(fruits()) {
		t.Fatalf("got %d items", len(items))
	}
	if items[3].State&semantics.Disabled == 0 {
		t.Error("a disabled option must say so")
	}
	r.act(r.must(semantics.RoleComboBox, "Fruit"), semantics.ActionCollapse)
	r.wantOpen(false)
	r.act(r.must(semantics.RoleComboBox, "Fruit"), semantics.ActionExpand)
	r.act(r.must(semantics.RoleListItem, "Apple"), semantics.ActionTap)
	r.wantValue("Apple")
}

func TestDropdownMenu_WithoutOnChangeIsDisabled(t *testing.T) {
	th := theme.Default().Light
	r := newFieldRig(t, DropdownMenu[string]{Label: "Fruit", Value: "apple", Options: fruits()})
	n := r.must(semantics.RoleComboBox, "Fruit")
	if n.State&semantics.Disabled == 0 || n.Actions&semantics.ActionExpand != 0 || n.State&semantics.Focusable != 0 {
		t.Errorf("got %+v", n)
	}
	r.click(n)
	if r.Find(semantics.RoleList, "") != nil {
		t.Error("a disabled dropdown must not open")
	}
	var fill, ink bool
	for _, op := range wtest.Ops(r.tree, r.Size) {
		fill = fill || op.Color == th.Color.Fill
		ink = ink || op.Color == th.Color.Ink3
	}
	if !fill || !ink {
		t.Error("a disabled dropdown must draw in the disabled colors")
	}
}

func TestDropdownMenu_LongListScrollsToTheChoice(t *testing.T) {
	opts := make([]DropdownOption[int], 40)
	for i := range opts {
		opts[i] = DropdownOption[int]{Value: i, Label: "Item " + strconv.Itoa(i)}
	}
	r := newFieldRig(t, DropdownMenu[int]{Label: "Many", Value: 33, Options: opts, OnChange: func(int) {}})
	r.act(r.must(semantics.RoleComboBox, "Many"), semantics.ActionExpand)
	list := r.must(semantics.RoleList, "Many").Rect
	row := r.must(semantics.RoleListItem, "Item 33").Rect
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Errorf("the choice must be in view: row %+v, list %+v", row, list)
	}
	if list.H > float32(kit.PopupMaxRows*kit.MenuItemHeight+2*kit.MenuInset) {
		t.Errorf("the list must scroll rather than grow: %+v", list)
	}
}

func TestDropdownMenu_GoingAwayClosesTheList(t *testing.T) {
	r := newFieldRig(t, formHolder[bool]{Initial: true, Build: func(shown bool, set func(bool)) core.Widget {
		children := []core.Widget{control.Button{Label: "hide", OnPress: func() { set(false) }}}
		if shown {
			children = append(children, DropdownMenu[string]{Label: "Fruit", Options: fruits(), OnChange: func(string) {}})
		}
		return basic.Column{Children: children}
	}})
	r.act(r.must(semantics.RoleComboBox, "Fruit"), semantics.ActionExpand)
	r.act(r.must(semantics.RoleButton, "hide"), semantics.ActionTap)
	r.frame()
	if r.Find(semantics.RoleList, "") != nil {
		t.Error("the list must go with its dropdown")
	}
}
