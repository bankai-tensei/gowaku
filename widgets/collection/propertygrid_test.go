package collection

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// gridRig is a PropertyGrid of two categories in a 400×500 window, with the
// values it holds.
type gridRig struct {
	tree    *core.Tree
	name    string
	width   int
	opacity float64
	visible bool
	mode    string
	widths  []float32
}

var gridScreen = geom.Size{W: 400, H: 500}

func newGridRig(t *testing.T, filter bool) *gridRig {
	t.Helper()
	r := &gridRig{name: "Box", width: 120, opacity: 0.5, mode: "fill"}
	var state *wtest.HolderState
	r.tree = wtest.TextTree(t, overlay.Overlay{Child: wtest.Holder{First: r.grid(filter), State: &state}})
	r.render()
	return r
}

func (r *gridRig) grid(filter bool) PropertyGrid {
	return PropertyGrid{Filter: filter, Label: "Inspector", OnNameWidthChange: func(w float32) { r.widths = append(r.widths, w) },
		Categories: []PropertyCategory{
			{Name: "General", Properties: []Property{
				{Name: "Name", Description: "What the box is called.", Editor: PropertyText{Value: r.name, OnChange: func(s string) { r.name = s }}},
				{Name: "Kind", Editor: PropertyText{Value: "Rectangle"}},
				{Name: "Visible", Description: "Shows the box.", Editor: PropertyBool{Value: r.visible, OnChange: func(v bool) { r.visible = v }}},
				{Name: "Mode", Editor: PropertyChoice[string]{Value: r.mode, Options: []fieldw.DropdownOption[string]{
					{Value: "fill", Label: "Fill"}, {Value: "fit", Label: "Fit"},
				}, OnChange: func(v string) { r.mode = v }}},
			}},
			{Name: "Layout", Properties: []Property{
				{Name: "Width", Description: "In pixels.", Editor: PropertyInt{Value: r.width, Max: 1000, OnChange: func(v int) { r.width = v }}},
				{Name: "Opacity", Editor: PropertyNumber{Value: r.opacity, Min: 0, Max: 1, OnChange: func(v float64) { r.opacity = v }}},
				{Name: "Tint", Editor: PropertyColor{Value: geom.Hex(0x2f5fd0)}},
			}},
			{Name: "Advanced", Collapsed: true, Properties: []Property{
				{Name: "Cache", Editor: PropertyBool{Value: true, OnChange: func(bool) {}}},
			}},
		}}
}

func (r *gridRig) render() { wtest.Ops(r.tree, gridScreen) }

func (r *gridRig) sem() *semantics.Node {
	r.render()
	return r.tree.Semantics()
}

func (r *gridRig) Key(k input.KeyCode, mods input.Modifiers) {
	r.render()
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	r.render()
}

func (r *gridRig) focused() *semantics.Node {
	var out *semantics.Node
	r.sem().Walk(func(n *semantics.Node) bool {
		if n.State&semantics.Focused != 0 {
			out = n
		}
		return out == nil
	})
	return out
}

// named returns the node labeled name that is not a group.
func named(root *semantics.Node, name string) *semantics.Node {
	var out *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == name && n.Role != semantics.RoleGroup && n.Role != semantics.RoleButton {
			out = n
		}
		return out == nil
	})
	return out
}

func TestPropertyGrid_EditorsAreNamedByTheirProperty(t *testing.T) {
	r := newGridRig(t, false)
	root := r.sem()
	cases := []struct {
		name string
		role semantics.Role
		hint string
	}{
		{"Name", semantics.RoleTextField, "What the box is called."},
		{"Kind", semantics.RoleText, ""},
		{"Visible", semantics.RoleCheckbox, "Shows the box."},
		{"Mode", semantics.RoleComboBox, ""},
		{"Width", semantics.RoleStepper, "In pixels."},
		{"Opacity", semantics.RoleTextField, ""},
		{"Tint", semantics.RoleText, ""},
	}
	for _, tc := range cases {
		n := named(root, tc.name)
		if n == nil {
			t.Errorf("%s: no node", tc.name)
			continue
		}
		if n.Role != tc.role || n.Hint != tc.hint {
			t.Errorf("%s: got role %v hint %q", tc.name, n.Role, n.Hint)
		}
	}
	if n := named(root, "Kind"); n.Value != "Rectangle" || n.State&semantics.ReadOnly == 0 {
		t.Errorf("read-only text: %+v", n)
	}
	if n := named(root, "Tint"); n.Value != "#2F5FD0" {
		t.Errorf("a color reads its code: %+v", n)
	}
	// The name beside each editor is not read a second time.
	count := 0
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == "Width" || n.Value == "Width" {
			count++
		}
		return true
	})
	if count != 1 {
		t.Errorf("Width is read %d times", count)
	}
}

func TestPropertyGrid_Categories(t *testing.T) {
	r := newGridRig(t, false)
	root := r.sem()
	var groups []string
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleGroup {
			groups = append(groups, n.Label)
		}
		return true
	})
	if want := []string{"Inspector", "General", "Layout", "Advanced"}; !slices.Equal(groups, want) {
		t.Errorf("groups %q", groups)
	}
	adv := wtest.ByLabel(t, root, "Advanced")
	header := wtest.FindRole(adv, semantics.RoleButton)
	if header == nil || header.State&semantics.Collapsed == 0 || header.Actions&semantics.ActionExpand == 0 {
		t.Fatalf("a folded category: %+v", header)
	}
	if named(root, "Cache") != nil {
		t.Error("a folded category hides its properties")
	}
	if h := wtest.FindRole(wtest.ByLabel(t, root, "General"), semantics.RoleButton); h.State&semantics.Expanded == 0 {
		t.Errorf("an open category: %+v", h)
	}
	wtest.Tap(r.tree, wtest.Center(header.Rect))
	if named(r.sem(), "Cache") == nil {
		t.Error("a click unfolds it")
	}
}

func TestPropertyGrid_Keyboard(t *testing.T) {
	r := newGridRig(t, false)
	var order []string
	for range 6 {
		r.Key(input.KeyTab, 0)
		order = append(order, r.focused().Label)
	}
	// The stepper of Width is named Width; its first button takes the focus.
	if want := []string{"General", "Name", "Visible", "Mode", "Layout", "−"}; !slices.Equal(order[:6], want) {
		t.Errorf("Tab goes through titles and editors: %q", order)
	}
	// Back to the title of Layout, which Space folds.
	for r.focused().Label != "Layout" {
		r.Key(input.KeyTab, input.ModShift)
	}
	r.Key(input.KeySpace, 0)
	if named(r.sem(), "Width") != nil {
		t.Fatal("Space folds the category")
	}
	r.Key(input.KeyRight, 0)
	if named(r.sem(), "Width") == nil {
		t.Fatal("Right unfolds it")
	}
	r.Key(input.KeyLeft, 0)
	r.Key(input.KeyEnter, 0)
	if named(r.sem(), "Width") == nil {
		t.Error("Left folds and Enter unfolds")
	}
}

func TestPropertyGrid_DescriptionFollowsTheFocus(t *testing.T) {
	r := newGridRig(t, false)
	if findTextAnywhere(r.tree, "In pixels.") {
		t.Fatal("no description before a focus")
	}
	// Tab to the minus button of the stepper of Width.
	for range 6 {
		r.Key(input.KeyTab, 0)
	}
	if !findTextAnywhere(r.tree, "In pixels.") {
		t.Error("the area shows the description of the property in focus")
	}
}

// findTextAnywhere reports whether the tree paints s, which a silent area does
// not tell assistive technology.
func findTextAnywhere(tree *core.Tree, s string) bool {
	found := false
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if p, ok := ro.(*basic.RenderParagraph); ok && p.Text() == s {
			found = true
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(tree.RootRenderObject())
	return found
}

func TestPropertyGrid_Filter(t *testing.T) {
	r := newGridRig(t, true)
	r.Key(input.KeyTab, 0)
	if f := r.focused(); f == nil || f.Label != "Filter properties" {
		t.Fatalf("the search field comes first: %+v", f)
	}
	r.tree.DispatchText("CAC")
	root := r.sem()
	if named(root, "Cache") == nil {
		t.Error("a filter shows what it finds in a folded category")
	}
	if named(root, "Width") != nil || wtest.FindRole(root, semantics.RoleButton) == nil {
		t.Error("and hides the rest")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyBackspace})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyBackspace})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyBackspace})
	r.tree.DispatchText("layout")
	root = r.sem()
	if named(root, "Width") == nil || named(root, "Opacity") == nil || named(root, "Name") != nil {
		t.Error("the name of a category keeps all its properties")
	}
}

func TestPropertyGrid_NumberCommitsOnEnterAndLeaving(t *testing.T) {
	r := newGridRig(t, false)
	field := named(r.sem(), "Opacity")
	wtest.Tap(r.tree, wtest.Center(field.Rect))
	r.render()
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyA, Modifiers: input.ModControl})
	r.tree.DispatchText("0.25")
	r.render()
	if r.opacity != 0.5 {
		t.Fatal("typing alone changes nothing")
	}
	r.Key(input.KeyEnter, 0)
	if r.opacity != 0.25 {
		t.Errorf("Enter commits: %v", r.opacity)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyA, Modifiers: input.ModControl})
	r.tree.DispatchText("7")
	r.Key(input.KeyTab, 0)
	if r.opacity != 1 {
		t.Errorf("leaving commits, within Max: %v", r.opacity)
	}
}

func TestPropertyNumber_InvalidTextGoesBack(t *testing.T) {
	v := 2.5
	tree := wtest.TextTree(t, basic.Column{Children: []core.Widget{
		PropertyNumber{Value: v, OnChange: func(n float64) { v = n }}.editor(),
		control.Button{Label: "next", OnPress: func() {}},
	}})
	wtest.Ops(tree, wtest.Screen)
	field := wtest.FindRole(tree.Semantics(), semantics.RoleTextField)
	wtest.Tap(tree, wtest.Center(field.Rect))
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyA, Modifiers: input.ModControl})
	tree.DispatchText("abc")
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FindRole(tree.Semantics(), semantics.RoleTextField).Value; got != "2.5" || v != 2.5 {
		t.Errorf("got %q and %v", got, v)
	}
}

func TestPropertyGrid_DragTheColumnBoundary(t *testing.T) {
	r := newGridRig(t, false)
	// The names take two fifths of 400.
	at := geom.Offset{X: 160, Y: 120}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 200, Y: 130}})
	r.render()
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 200, Y: 130}})
	r.render()
	if len(r.widths) == 0 || r.widths[len(r.widths)-1] != 200 {
		t.Fatalf("widths %v", r.widths)
	}
	if n := named(r.sem(), "Width"); n.Rect.X < 200 {
		t.Errorf("the editors move with it: %+v", n.Rect)
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 2000, Y: 130}})
	at = geom.Offset{X: 200, Y: 120}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1000})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 1010})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1100})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 1110})
	r.render()
	if got := r.widths[len(r.widths)-1]; got != 160 {
		t.Errorf("a double click goes back: %v", got)
	}
}

func TestPropertyNameWidth(t *testing.T) {
	for _, tc := range []struct{ asked, width, want float32 }{
		{0, 400, 160},
		{50, 400, 80},
		{350, 400, 280},
		{0, 100, 80},
		{0, 60, 60},
	} {
		if got := propertyNameWidth(tc.asked, tc.width); got != tc.want {
			t.Errorf("%v of %v: got %v, want %v", tc.asked, tc.width, got, tc.want)
		}
	}
}

func TestHexOf(t *testing.T) {
	if got := hexOf(geom.Hex(0x2f5fd0)); got != "#2F5FD0" {
		t.Error(got)
	}
	if got := hexOf(geom.Color{R: 1, A: 0.5}); got != "#FF000080" {
		t.Error(got)
	}
}
