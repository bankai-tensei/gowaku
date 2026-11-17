package field

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// DropdownMenu stays out of the shared press table of design_test.go, which
// wraps its controls in an Overlay: there the release would open the list.

func TestDropdownMenu_PressMoves(t *testing.T) {
	// Without an Overlay the release opens nothing, as the shared table of
	// design_test.go needs.
	tree := wtest.TextTree(t, basic.Align{Child: DropdownMenu[string]{Options: fruits(), OnChange: func(string) {}}})
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
}

func TestDropdownMenu_ShowsTheFocusRingFromTheKeyboard(t *testing.T) {
	r := newFieldRig(t, DropdownMenu[string]{Label: "Fruit", Options: fruits(), OnChange: func(string) {}})
	rest := r.frame()
	r.Key(input.KeyTab, 0)
	r.settle()
	if r.frame() == rest {
		t.Error("the keyboard focus must show")
	}
	r.wantFocus(semantics.RoleComboBox, "Fruit")
}

func TestFormField_AutocompleteJudgedOnBlurNotOnAClick(t *testing.T) {
	var ctrl FormController
	submitted := 0
	r := newFieldRig(t, formHolder[string]{Build: func(v string, set func(string)) core.Widget {
		return Form{Controller: &ctrl, OnSubmit: func() { submitted++ }, Child: basic.Column{Spacing: 8, Children: []core.Widget{
			FormItem[string]{Value: v, Required: true, Child: basic.SizedBox{Width: 240, Child: Autocomplete[string]{
				Label: "City", Value: v, Options: wtest.CityNames(), OnChange: set,
			}}},
			Input{Label: "Next"},
		}}}
	}})
	r.focusField("City")
	r.typeText("k")
	r.click(r.must(semantics.RoleListItem, "Kobe"))
	r.wantFocus(semantics.RoleTextField, "City")
	r.wantError("City", "")
	r.Key(input.KeyEnter, 0)
	if submitted != 1 {
		t.Errorf("Enter without a highlight must submit the form, got %d", submitted)
	}
	r.Key(input.KeyA, input.ModControl)
	r.Key(input.KeyBackspace, 0)
	r.wantError("City", "")
	r.Key(input.KeyTab, 0)
	r.wantError("City", "Required")
	if n := r.must(semantics.RoleComboBox, "City"); n.Hint != "Required" {
		t.Errorf("the combo box must say the error too: %+v", n)
	}
}
