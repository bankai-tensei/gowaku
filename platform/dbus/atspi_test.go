package dbus

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestAtspiRole(t *testing.T) {
	tests := []struct {
		name string
		n    semantics.Node
		want uint32
	}{
		{"button", semantics.Node{Role: semantics.RoleButton}, atspiRolePushButton},
		{"switch", semantics.Node{Role: semantics.RoleSwitch}, atspiRoleToggleBtn},
		{"stepper", semantics.Node{Role: semantics.RoleStepper}, atspiRoleSpinButton},
		{"radio group", semantics.Node{Role: semantics.RoleRadioGroup}, atspiRolePanel},
		{"heading", semantics.Node{Role: semantics.RoleHeading}, atspiRoleHeading},
		{"entry", semantics.Node{Role: semantics.RoleTextField}, atspiRoleEntry},
		{"multi-line", semantics.Node{Role: semantics.RoleTextField, State: semantics.Multiline}, atspiRoleText},
		{"text", semantics.Node{Role: semantics.RoleText}, atspiRoleLabel},
		{"none with children", semantics.Node{Children: []*semantics.Node{{}}}, atspiRolePanel},
		{"none with label", semantics.Node{Label: "x"}, atspiRoleLabel},
		{"none", semantics.Node{}, atspiRoleFiller},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := atspiRole(&tt.n); got != tt.want {
				t.Errorf("got %d (%s), want %d (%s)", got, atspiRoleName(got), tt.want, atspiRoleName(tt.want))
			}
		})
	}
	// Every role has a name.
	for r := semantics.RoleNone; r <= semantics.RoleStepper; r++ {
		if atspiRoleName(atspiRole(&semantics.Node{Role: r})) == "unknown" {
			t.Errorf("role %v has no AT-SPI name", r)
		}
	}
}

func TestNodeStates(t *testing.T) {
	has := func(s stateSet, states ...uint) bool {
		for _, st := range states {
			if !s.has(st) {
				return false
			}
		}
		return true
	}
	cb := &semantics.Node{Role: semantics.RoleCheckbox, State: semantics.Checked | semantics.Focusable}
	s := nodeStates(cb, true)
	if !has(s, atspiStateEnabled, atspiStateSensitive, atspiStateShowing, atspiStateVisible,
		atspiStateFocusable, atspiStateFocused, atspiStateCheckable, atspiStateChecked) {
		t.Errorf("checkbox states %x lack some", s)
	}
	off := nodeStates(&semantics.Node{Role: semantics.RoleButton, State: semantics.Disabled}, false)
	if off.has(atspiStateEnabled) || off.has(atspiStateSensitive) || off.has(atspiStateFocused) {
		t.Errorf("disabled button states %x", off)
	}
	radio := nodeStates(&semantics.Node{Role: semantics.RoleRadio, State: semantics.Selected}, false)
	if !has(radio, atspiStateChecked, atspiStateSelected, atspiStateSelectable) {
		t.Errorf("selected radio states %x", radio)
	}
	day := nodeStates(&semantics.Node{Role: semantics.RoleGridCell, State: semantics.Selected}, false)
	if !has(day, atspiStateSelected, atspiStateSelectable) {
		t.Errorf("selected day states %x", day)
	}
	item := nodeStates(&semantics.Node{Role: semantics.RoleTreeItem, State: semantics.Selected | semantics.Expanded}, false)
	if !has(item, atspiStateSelected, atspiStateSelectable, atspiStateExpanded) {
		t.Errorf("tree item states %x", item)
	}
	if row := nodeStates(&semantics.Node{Role: semantics.RoleRow, State: semantics.Selected, Actions: semantics.ActionTap}, false); !has(row, atspiStateSelected, atspiStateSelectable) {
		t.Errorf("selected row states %x", row)
	}
	if row := nodeStates(&semantics.Node{Role: semantics.RoleRow}, false); row.has(atspiStateSelectable) {
		t.Errorf("a row of a calendar is not selectable: %x", row)
	}
	if tree := nodeStates(&semantics.Node{Role: semantics.RoleTree, State: semantics.Multiselectable}, false); !tree.has(atspiStateMultiselect) {
		t.Errorf("tree states %x", tree)
	}
	bad := nodeStates(&semantics.Node{Role: semantics.RoleTextField, State: semantics.Invalid | semantics.Required}, false)
	if !has(bad, atspiStateInvalidEntry, atspiStateRequired) {
		t.Errorf("invalid required field states %x", bad)
	}
	field := nodeStates(&semantics.Node{Role: semantics.RoleTextField, Actions: semantics.ActionSetValue}, false)
	if !has(field, atspiStateEditable, atspiStateSingleLine) || field.has(atspiStateMultiLine) {
		t.Errorf("field states %x", field)
	}
	exp := nodeStates(&semantics.Node{State: semantics.Collapsed}, false)
	if !has(exp, atspiStateExpandable, atspiStateCollapsed) || exp.has(atspiStateExpanded) {
		t.Errorf("collapsed states %x", exp)
	}
	if field.has(atspiStateInvalidEntry) || field.has(atspiStateRequired) {
		t.Errorf("plain field states %x", field)
	}
	if !frameStates(true).has(atspiStateActive) || frameStates(false).has(atspiStateActive) {
		t.Error("frame active state")
	}
	// The second word holds the states from 32.
	if s := nodeStates(&semantics.Node{State: semantics.ReadOnly}, false); s[1]&(1<<(atspiStateReadOnly-32)) == 0 {
		t.Errorf("read-only must be in the second word: %x", s)
	}
}

func TestChangedStates(t *testing.T) {
	before := nodeStates(&semantics.Node{Role: semantics.RoleCheckbox}, false)
	after := nodeStates(&semantics.Node{Role: semantics.RoleCheckbox, State: semantics.Checked | semantics.Disabled}, true)
	got := changedStates(before, after)
	want := []stateChange{{"checked", true}, {"enabled", false}, {"sensitive", false}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNodeActionsAndIfaces(t *testing.T) {
	n := &semantics.Node{Role: semantics.RoleSlider, Actions: semantics.ActionIncrease | semantics.ActionDecrease | semantics.ActionFocus,
		Range: semantics.Range{Max: 10}}
	var names []string
	for _, a := range nodeActions(n) {
		names = append(names, a.name)
	}
	if !slices.Equal(names, []string{"increase", "decrease"}) {
		t.Errorf("actions %v", names)
	}
	if got := nodeIfaces(n); got != ifaceAccessible|ifaceComponent|ifaceAction|ifaceValue {
		t.Errorf("slider interfaces %v", got.names())
	}
	field := &semantics.Node{Role: semantics.RoleTextField, Actions: semantics.ActionSetValue, Text: &semantics.TextInfo{}}
	if got := nodeIfaces(field); got != ifaceAccessible|ifaceComponent|ifaceText|ifaceEditableText {
		t.Errorf("field interfaces %v", got.names())
	}
	field.State = semantics.ReadOnly
	if nodeIfaces(field)&ifaceEditableText != 0 {
		t.Error("a read-only field is not editable")
	}
	if ifaceOf("org.a11y.atspi.Text") != ifaceText || ifaceOf("org.example") != 0 {
		t.Error("ifaceOf")
	}
}

func TestNodeAttributes(t *testing.T) {
	m := nodeAttributes(&semantics.Node{Role: semantics.RoleHeading, Level: 2, Live: semantics.LiveAssertive})
	if m["level"] != "2" || m["container-live"] != "assertive" || m["live"] != "assertive" || m["toolkit"] != "gowaku" {
		t.Errorf("got %v", m)
	}
	item := nodeAttributes(&semantics.Node{Role: semantics.RoleTreeItem, Level: 3, PosInSet: 2, SetSize: 5})
	if item["level"] != "3" || item["posinset"] != "2" || item["setsize"] != "5" {
		t.Errorf("tree item attributes %v", item)
	}
	if bare := nodeAttributes(&semantics.Node{Role: semantics.RoleTreeItem}); bare["level"] != "" || bare["posinset"] != "" || bare["setsize"] != "" {
		t.Errorf("unset tree item attributes must be absent, got %v", bare)
	}
	if _, ok := m["rowindex"]; ok {
		t.Error("a heading has no place in a grid")
	}
	grid := nodeAttributes(&semantics.Node{Role: semantics.RoleGrid, RowCount: 1001, ColumnCount: 5})
	cell := nodeAttributes(&semantics.Node{Role: semantics.RoleGridCell, RowIndex: 502, ColumnIndex: 3})
	if grid["rowcount"] != "1001" || grid["colcount"] != "5" || cell["rowindex"] != "502" || cell["colindex"] != "3" {
		t.Errorf("grid %v, cell %v", grid, cell)
	}
	header := nodeAttributes(&semantics.Node{Role: semantics.RoleColumnHeader, Sort: semantics.SortAscending})
	if header["sort"] != "ascending" {
		t.Errorf("a sorted header: %v, want sort ascending", header)
	}
	if _, ok := nodeAttributes(&semantics.Node{Role: semantics.RoleColumnHeader})["sort"]; ok {
		t.Error("a header that is not sorted has no sort attribute")
	}
}

func TestNodePath(t *testing.T) {
	p := nodePath(3, 42)
	w, id, ok := parseNodePath(p)
	if !ok || w != 3 || id != 42 {
		t.Errorf("parse %q = %d %d %v", p, w, id, ok)
	}
	if nodePath(1, 0) == nodePath(2, 0) || nodePath(1, 5) == nodePath(2, 5) {
		t.Error("windows must not share paths")
	}
	for _, bad := range []string{rootPath, accessiblePrefix + "7", accessiblePrefix + "x", "/other/1"} {
		if _, _, ok := parseNodePath(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
	}
}

func testTree() *tree {
	root := &semantics.Node{Rect: geom.Rect{W: 200, H: 100}, Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleGroup, Rect: geom.Rect{X: 10, Y: 10, W: 100, H: 50}, Children: []*semantics.Node{
			{ID: 2, Role: semantics.RoleButton, Rect: geom.Rect{X: 20, Y: 20, W: 30, H: 20}},
			{ID: 3, Role: semantics.RoleButton, Rect: geom.Rect{X: 60, Y: 20, W: 30, H: 20}},
		}},
	}}
	return newTree(root, 3)
}

func TestTree(t *testing.T) {
	tr := testTree()
	if tr.indexInParent(3) != 1 || tr.indexInParent(1) != 0 || tr.indexInParent(0) != -1 {
		t.Error("indexInParent")
	}
	if id, ok := tr.hitTest(0, geom.Offset{X: 65, Y: 25}); !ok || id != 3 {
		t.Errorf("hit = %d %v, want 3", id, ok)
	}
	if id, ok := tr.hitTest(0, geom.Offset{X: 15, Y: 15}); !ok || id != 1 {
		t.Errorf("hit = %d %v, want 1", id, ok)
	}
	if _, ok := tr.hitTest(2, geom.Offset{X: 25, Y: 25}); ok {
		t.Error("a leaf has nothing below")
	}
	if tr.has(9) || !tr.has(0) {
		t.Error("has")
	}
}

func TestValueHelpers(t *testing.T) {
	if s := atspiValueText(&semantics.Node{Range: semantics.Range{Max: 10, Value: 2.5}}); s != "2.5" {
		t.Errorf("value text %q", s)
	}
	if s := atspiValueText(&semantics.Node{Value: "25%", Range: semantics.Range{Max: 1, Value: 0.25}}); s != "25%" {
		t.Errorf("value text %q", s)
	}
	if v := minimumIncrement(semantics.Range{Max: 200}); v != 2 {
		t.Errorf("increment %v, want a hundredth", v)
	}
	if v := minimumIncrement(semantics.Range{Max: 200, Step: 5}); v != 5 {
		t.Errorf("increment %v, want the step", v)
	}
}

func TestGeometry(t *testing.T) {
	if r := (WindowGeometry{Width: 800, Height: 600, Scale: 2}).rect(); r != (geom.Rect{W: 400, H: 300}) {
		t.Errorf("window rect %v", r)
	}
	if r := testTree().rect(2); r != (geom.Rect{X: 20, Y: 20, W: 30, H: 20}) {
		t.Errorf("node rect %v", r)
	}
	g := WindowGeometry{X: 100, Y: 50, Scale: 2}
	r := geom.Rect{X: 20, Y: 20, W: 30, H: 20}
	parent := geom.Rect{X: 10, Y: 10}
	if e := g.toAtspi(r, parent, atspiCoordScreen); e != (extents{140, 90, 60, 40}) {
		t.Errorf("screen %v", e)
	}
	if e := g.toAtspi(r, parent, atspiCoordWindow); e != (extents{40, 40, 60, 40}) {
		t.Errorf("window %v", e)
	}
	if e := g.toAtspi(r, parent, atspiCoordParent); e != (extents{20, 20, 60, 40}) {
		t.Errorf("parent %v", e)
	}
	for _, coord := range []uint32{atspiCoordScreen, atspiCoordWindow, atspiCoordParent} {
		e := g.toAtspi(r, parent, coord)
		if p := g.fromAtspi(e.X, e.Y, parent, coord); p != r.Origin() {
			t.Errorf("coord %d: round trip %v, want %v", coord, p, r.Origin())
		}
	}
}

func info(s string, start, end int, lineStarts ...int) *semantics.TextInfo {
	return &semantics.TextInfo{Text: s, SelectionStart: start, SelectionEnd: end, LineStarts: lineStarts}
}

func TestAtspiText_Offsets(t *testing.T) {
	// 😀 is two UTF-16 units and one character.
	tx := newAtspiText(info("a😀b", 3, 3))
	if tx.count() != 3 {
		t.Errorf("count %d", tx.count())
	}
	for u, r := range []int{0, 1, 1, 2, 3} {
		if got := tx.toRune(u); got != r {
			t.Errorf("toRune(%d) = %d, want %d", u, got, r)
		}
	}
	for r, u := range []int{0, 1, 3, 4} {
		if got := tx.toU16(r); got != u {
			t.Errorf("toU16(%d) = %d, want %d", r, got, u)
		}
	}
	if tx.caret() != 2 {
		t.Errorf("caret %d", tx.caret())
	}
	if tx.slice(1, -1) != "😀b" || tx.slice(2, 1) != "" {
		t.Error("slice")
	}
	sel := newAtspiText(info("a😀b", 4, 1))
	if s, e, ok := sel.selection(); !ok || s != 1 || e != 3 {
		t.Errorf("selection %d %d %v", s, e, ok)
	}
}

func TestAtspiText_StringAt(t *testing.T) {
	tx := newAtspiText(info("こんにちは world", 0, 0))
	tests := []struct {
		off        int
		g          uint32
		want       string
		start, end int
	}{
		{2, atspiGranularityWord, "こんにちは", 0, 5},
		{5, atspiGranularityWord, "こんにちは", 0, 5},
		{7, atspiGranularityWord, "world", 6, 11},
		{11, atspiGranularityWord, "world", 6, 11},
		{3, atspiGranularityChar, "ち", 3, 4},
		{3, atspiGranularityLine, "こんにちは world", 0, 11},
		{3, atspiGranularitySentence, "こんにちは world", 0, 11},
	}
	for _, tt := range tests {
		s, start, end := tx.stringAt(tt.off, tt.g)
		if s != tt.want || start != tt.start || end != tt.end {
			t.Errorf("stringAt(%d, %d) = %q %d %d, want %q %d %d", tt.off, tt.g, s, start, end, tt.want, tt.start, tt.end)
		}
	}
	lines := newAtspiText(info("ab\ncd\nef", 0, 0, 3, 6))
	if s, start, end := lines.stringAt(4, atspiGranularityLine); s != "cd\n" || start != 3 || end != 6 {
		t.Errorf("line = %q %d %d", s, start, end)
	}
	if s, _, _ := lines.stringAt(7, atspiGranularityParagraph); s != "ef" {
		t.Errorf("paragraph = %q", s)
	}
}

func TestAtspiText_Around(t *testing.T) {
	tx := newAtspiText(info("hello big world", 0, 0))
	tests := []struct {
		off, which int
		b          uint32
		want       string
	}{
		{7, pieceAtOff, atspiBoundaryWordStart, "big "},
		{7, pieceBefore, atspiBoundaryWordStart, "hello "},
		{7, pieceAfter, atspiBoundaryWordStart, "world"},
		{7, pieceAtOff, atspiBoundaryWordEnd, " big"},
		{0, pieceBefore, atspiBoundaryWordStart, ""},
		{14, pieceAfter, atspiBoundaryWordStart, ""},
		{1, pieceAtOff, atspiBoundaryChar, "e"},
		{1, pieceAfter, atspiBoundaryChar, "l"},
	}
	for _, tt := range tests {
		if s, _, _ := tx.textAround(tt.off, tt.b, tt.which); s != tt.want {
			t.Errorf("textAround(%d, %d, %d) = %q, want %q", tt.off, tt.b, tt.which, s, tt.want)
		}
	}
}

func TestAtspiText_Rects(t *testing.T) {
	ti := info("ab\ncd", 0, 0, 3)
	ti.LineRects = []geom.Rect{{X: 0, Y: 0, W: 50, H: 10}, {X: 0, Y: 10, W: 40, H: 10}}
	tx := newAtspiText(ti)
	if r, ok := tx.rangeRect(1, 5); !ok || r != (geom.Rect{W: 50, H: 20}) {
		t.Errorf("rangeRect %v %v", r, ok)
	}
	if tx.offsetAtPoint(geom.Offset{X: 5, Y: 15}) != 3 || tx.offsetAtPoint(geom.Offset{X: 5, Y: 50}) != -1 {
		t.Error("offsetAtPoint")
	}
}

func TestTextChange(t *testing.T) {
	tests := []struct {
		before, after   string
		start           int
		removed, insert string
	}{
		{"hello", "help", 3, "lo", "p"},
		{"日本", "日本語", 2, "", "語"},
		{"abc", "abc", 3, "", ""},
		{"aXa", "aa", 1, "X", ""},
	}
	for _, tt := range tests {
		start, removed, inserted := textChange(tt.before, tt.after)
		if start != tt.start || string(removed) != tt.removed || string(inserted) != tt.insert {
			t.Errorf("textChange(%q, %q) = %d %q %q", tt.before, tt.after, start, string(removed), string(inserted))
		}
	}
}

func TestAnnouncement(t *testing.T) {
	if s := announcement(&semantics.Node{Label: "Saved", Value: "3 items"}); s != "Saved 3 items" {
		t.Errorf("got %q", s)
	}
	if s := announcement(&semantics.Node{Role: semantics.RoleText, Value: "Done"}); s != "Done" {
		t.Errorf("got %q", s)
	}
}
