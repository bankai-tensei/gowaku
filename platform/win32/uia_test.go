package win32

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// These test the UI Automation mapping and the text movement without the OS.

func TestControlType(t *testing.T) {
	tests := []struct {
		node semantics.Node
		want int32
	}{
		{semantics.Node{Role: semantics.RoleButton}, UIA_ButtonControlTypeId},
		{semantics.Node{Role: semantics.RoleSwitch}, UIA_ButtonControlTypeId},
		{semantics.Node{Role: semantics.RoleCheckbox}, UIA_CheckBoxControlTypeId},
		{semantics.Node{Role: semantics.RoleDialog}, UIA_WindowControlTypeId},
		{semantics.Node{Role: semantics.RoleHeading}, UIA_TextControlTypeId},
		{semantics.Node{Role: semantics.RoleStepper}, UIA_SpinnerControlTypeId},
		{semantics.Node{Role: semantics.RoleRadioGroup}, UIA_GroupControlTypeId},
		{semantics.Node{Role: semantics.RoleTextField}, UIA_EditControlTypeId},
		{semantics.Node{Role: semantics.RoleTabList}, UIA_TabControlTypeId},
		{semantics.Node{Role: semantics.RoleLink}, UIA_HyperlinkControlTypeId},
		{semantics.Node{Role: semantics.RoleProgress}, UIA_ProgressBarControlTypeId},
		{semantics.Node{Role: semantics.RoleTree}, UIA_TreeControlTypeId},
		{semantics.Node{Role: semantics.RoleTreeItem}, UIA_TreeItemControlTypeId},
		{semantics.Node{Children: []*semantics.Node{{ID: 2}}}, UIA_GroupControlTypeId},
		{semantics.Node{Label: "note"}, UIA_TextControlTypeId},
		{semantics.Node{}, UIA_CustomControlTypeId},
	}
	for _, tt := range tests {
		if got := controlType(&tt.node); got != tt.want {
			t.Errorf("%v (%d children, label %q): got %d, want %d",
				tt.node.Role, len(tt.node.Children), tt.node.Label, got, tt.want)
		}
	}
}

func TestPatterns(t *testing.T) {
	text := &semantics.TextInfo{Text: "abc"}
	tests := []struct {
		name string
		node semantics.Node
		want patternSet
	}{
		{"button", semantics.Node{Role: semantics.RoleButton, Actions: semantics.ActionTap}, patternInvoke},
		{"checkbox taps through Toggle", semantics.Node{Role: semantics.RoleCheckbox, Actions: semantics.ActionTap}, patternToggle},
		{"switch", semantics.Node{Role: semantics.RoleSwitch, Actions: semantics.ActionTap}, patternToggle},
		{"radio", semantics.Node{Role: semantics.RoleRadio, Actions: semantics.ActionTap}, patternSelectionItem},
		{"list item that taps", semantics.Node{Role: semantics.RoleListItem, Actions: semantics.ActionTap}, patternInvoke | patternSelectionItem},
		{"day of a calendar", semantics.Node{Role: semantics.RoleGridCell, Actions: semantics.ActionTap}, patternInvoke | patternSelectionItem | patternGridItem | patternTableItem},
		{"grid", semantics.Node{Role: semantics.RoleGrid}, patternSelection | patternGrid | patternTable},
		{"column header that sorts", semantics.Node{Role: semantics.RoleColumnHeader, Actions: semantics.ActionTap}, patternInvoke | patternGridItem},
		{"row that selects", semantics.Node{Role: semantics.RoleRow, Actions: semantics.ActionTap}, patternInvoke | patternSelectionItem},
		{"row of a calendar", semantics.Node{Role: semantics.RoleRow}, 0},
		{"list", semantics.Node{Role: semantics.RoleList}, patternSelection},
		{"anything multiselectable", semantics.Node{State: semantics.Multiselectable}, patternSelection},
		{"tree item", semantics.Node{Role: semantics.RoleTreeItem, State: semantics.Collapsed, Actions: semantics.ActionTap}, patternInvoke | patternSelectionItem | patternExpandCollapse},
		{"slider", semantics.Node{Role: semantics.RoleSlider, Value: "50%", Range: semantics.Range{Max: 1}}, patternRangeValue},
		{"progress", semantics.Node{Role: semantics.RoleProgress, Range: semantics.Range{Max: 100}}, patternRangeValue},
		{"text field", semantics.Node{Role: semantics.RoleTextField, Text: text}, patternValue | patternText},
		{"text field without text info", semantics.Node{Role: semantics.RoleTextField}, patternValue},
		{"value without range", semantics.Node{Role: semantics.RoleText, Value: "3 items"}, patternValue},
		{"disclosure", semantics.Node{Role: semantics.RoleButton, State: semantics.Collapsed, Actions: semantics.ActionTap}, patternInvoke | patternExpandCollapse},
		{"scrolls into view", semantics.Node{Actions: semantics.ActionScrollIntoView}, patternScrollItem},
		{"plain text", semantics.Node{Role: semantics.RoleText, Label: "x"}, 0},
	}
	for _, tt := range tests {
		if got := patterns(&tt.node); got != tt.want {
			t.Errorf("%s: got %b, want %b", tt.name, got, tt.want)
		}
	}
	if patternOf(UIA_TextPattern2Id) != patternText || patternOf(UIA_TextPatternId) != patternText || patternOf(1) != 0 {
		t.Error("patternOf")
	}
}

func TestNodeProperty(t *testing.T) {
	n := &semantics.Node{
		ID: 1<<32 | 7, Role: semantics.RoleHeading, Label: "Tasks", Hint: "All of them", Level: 2,
		State: semantics.Focusable | semantics.Disabled, Live: semantics.LivePolite,
	}
	tests := []struct {
		prop int32
		want propValue
	}{
		{UIA_ControlTypePropertyId, propInt(UIA_TextControlTypeId)},
		{UIA_NamePropertyId, propText("Tasks")},
		{UIA_HelpTextPropertyId, propText("All of them")},
		{UIA_IsEnabledPropertyId, propFlag(false)},
		{UIA_IsKeyboardFocusablePropertyId, propFlag(true)},
		{UIA_HasKeyboardFocusPropertyId, propFlag(true)},
		{UIA_LiveSettingPropertyId, propInt(1)},
		{UIA_IsControlElementPropertyId, propFlag(true)},
		{UIA_FrameworkIdPropertyId, propText("gowaku")},
		{UIA_AutomationIdPropertyId, propText("4294967303")},
		{UIA_IsDialogPropertyId, propFlag(false)},
		{UIA_HeadingLevelPropertyId, propInt(HeadingLevel1 + 1)},
		{UIA_IsPasswordPropertyId, propFlag(false)},
		{UIA_IsRequiredForFormPropertyId, propFlag(false)},
		{UIA_IsDataValidForFormPropertyId, propFlag(true)},
		{30001, propValue{}},
	}
	for _, tt := range tests {
		if got := nodeProperty(n, tt.prop, true); got != tt.want {
			t.Errorf("property %d: got %+v, want %+v", tt.prop, got, tt.want)
		}
	}
	if got := nodeProperty(&semantics.Node{Role: semantics.RoleText}, UIA_NamePropertyId, false); got.kind != propEmpty {
		t.Errorf("empty name must be VT_EMPTY, got %+v", got)
	}
	bad := &semantics.Node{Role: semantics.RoleTextField, State: semantics.Invalid | semantics.Required}
	if nodeProperty(bad, UIA_IsRequiredForFormPropertyId, false) != propFlag(true) ||
		nodeProperty(bad, UIA_IsDataValidForFormPropertyId, false) != propFlag(false) {
		t.Error("an invalid required field must say so")
	}
	item := &semantics.Node{Role: semantics.RoleTreeItem, Level: 3, PosInSet: 2, SetSize: 5}
	if nodeProperty(item, UIA_LevelPropertyId, false) != propInt(3) ||
		nodeProperty(item, UIA_PositionInSetPropertyId, false) != propInt(2) ||
		nodeProperty(item, UIA_SizeOfSetPropertyId, false) != propInt(5) {
		t.Error("a tree item must say its level and place among its siblings")
	}
	if got := nodeProperty(n, UIA_LevelPropertyId, false); got.kind != propEmpty {
		t.Errorf("a heading says its level with HeadingLevel, got %+v", got)
	}
	if got := nodeProperty(&semantics.Node{Role: semantics.RoleTreeItem}, UIA_PositionInSetPropertyId, false); got.kind != propEmpty {
		t.Errorf("an unknown place must be left to UIA, got %+v", got)
	}
	header := &semantics.Node{Role: semantics.RoleColumnHeader, Label: "Name", Sort: semantics.SortDescending}
	if nodeProperty(header, UIA_ItemStatusPropertyId, false) != propText("descending") ||
		nodeProperty(header, UIA_AriaPropertiesPropertyId, false) != propText("sort=descending") {
		t.Error("a sorted column header must say its order")
	}
	header.Sort = semantics.SortNone
	if got := nodeProperty(header, UIA_ItemStatusPropertyId, false); got.kind != propEmpty {
		t.Errorf("a header that is not sorted has no status, got %+v", got)
	}
	if got := headingLevel(&semantics.Node{Role: semantics.RoleHeading, Level: 12}); got != HeadingLevel9 {
		t.Errorf("heading level 12 = %d, want HeadingLevel9", got)
	}
	if rid := runtimeID(n.ID); rid != [3]int32{UiaAppendRuntimeId, 7, 1} {
		t.Errorf("runtime ID %v", rid)
	}
}

func TestPatternValues(t *testing.T) {
	slider := &semantics.Node{Range: semantics.Range{Min: 0, Max: 200, Value: 50, Step: 5}, Actions: semantics.ActionIncrease}
	if rangeReadOnly(slider) || smallChange(slider.Range) != 5 || largeChange(slider.Range) != 20 {
		t.Errorf("slider: read-only %v, small %v, large %v", rangeReadOnly(slider), smallChange(slider.Range), largeChange(slider.Range))
	}
	progress := &semantics.Node{Range: semantics.Range{Max: 100}}
	if !rangeReadOnly(progress) || smallChange(progress.Range) != 1 {
		t.Error("progress must be read-only with a step of a hundredth")
	}
	field := &semantics.Node{Role: semantics.RoleTextField, Value: "stale", Text: &semantics.TextInfo{Text: "fresh"}, Actions: semantics.ActionSetValue}
	if nodeValue(field) != "fresh" || valueReadOnly(field) {
		t.Errorf("field value %q, read-only %v", nodeValue(field), valueReadOnly(field))
	}
	field.State |= semantics.ReadOnly
	if !valueReadOnly(field) {
		t.Error("ReadOnly field")
	}
	for state, want := range map[semantics.State]int32{
		0: ToggleState_Off, semantics.Checked: ToggleState_On, semantics.Mixed | semantics.Checked: ToggleState_Indeterminate,
	} {
		if got := toggleState(&semantics.Node{State: state}); got != want {
			t.Errorf("toggle state of %b = %d, want %d", state, got, want)
		}
	}
	if expandCollapseState(&semantics.Node{State: semantics.Expanded}) != ExpandCollapseState_Expanded ||
		expandCollapseState(&semantics.Node{State: semantics.Collapsed}) != ExpandCollapseState_Collapsed {
		t.Error("expand collapse state")
	}
}

func testTree() *semantics.Node {
	return &semantics.Node{Rect: geom.Rect{W: 400, H: 300}, Children: []*semantics.Node{
		{ID: 1, Role: semantics.RoleButton, Rect: geom.Rect{X: 10, Y: 10, W: 100, H: 30}},
		{ID: 2, Role: semantics.RoleRadioGroup, Rect: geom.Rect{X: 10, Y: 50, W: 200, H: 30}, Children: []*semantics.Node{
			{ID: 3, Role: semantics.RoleRadio, Rect: geom.Rect{X: 10, Y: 50, W: 100, H: 30}},
			{ID: 4, Role: semantics.RoleRadio, Rect: geom.Rect{X: 110, Y: 50, W: 100, H: 30}},
		}},
		{ID: 5, Role: semantics.RoleText},
	}}
}

func TestSnapshot_Navigate(t *testing.T) {
	s := newSnapshot(testTree())
	tests := []struct {
		id   semantics.ID
		dir  int32
		want semantics.ID
		ok   bool
	}{
		{0, NavigateDirection_FirstChild, 1, true},
		{0, NavigateDirection_LastChild, 5, true},
		{0, NavigateDirection_Parent, 0, false},
		{0, NavigateDirection_NextSibling, 0, false},
		{1, NavigateDirection_Parent, 0, true},
		{1, NavigateDirection_NextSibling, 2, true},
		{1, NavigateDirection_PreviousSibling, 0, false},
		{3, NavigateDirection_Parent, 2, true},
		{4, NavigateDirection_PreviousSibling, 3, true},
		{4, NavigateDirection_NextSibling, 0, false},
		{5, NavigateDirection_FirstChild, 0, false},
		{9, NavigateDirection_Parent, 0, false},
	}
	for _, tt := range tests {
		got, ok := s.navigate(tt.id, tt.dir)
		if got != tt.want || ok != tt.ok {
			t.Errorf("navigate(%d, %d) = %d, %v; want %d, %v", tt.id, tt.dir, got, ok, tt.want, tt.ok)
		}
	}
	if !s.has(0) || !s.has(4) || s.has(9) {
		t.Error("has")
	}
	if id, ok := s.selectionContainer(4); id != 2 || !ok {
		t.Errorf("selection container of a radio = %d, %v", id, ok)
	}
	if _, ok := s.selectionContainer(1); ok {
		t.Error("a button has no selection container")
	}
	empty := newSnapshot(nil)
	if _, ok := empty.navigate(0, NavigateDirection_FirstChild); ok {
		t.Error("an empty tree has no children")
	}
}

func TestSnapshot_HitTest(t *testing.T) {
	s := newSnapshot(testTree())
	for _, tt := range []struct {
		p    geom.Offset
		want semantics.ID
	}{
		{geom.Offset{X: 20, Y: 20}, 1},
		{geom.Offset{X: 150, Y: 60}, 4},
		{geom.Offset{X: 300, Y: 200}, 0},
	} {
		if got := s.hitTest(tt.p); got != tt.want {
			t.Errorf("hitTest(%v) = %d, want %d", tt.p, got, tt.want)
		}
	}
}

func TestFieldText_Units(t *testing.T) {
	ti := &semantics.TextInfo{Text: "こんにちは world\nnext line", LineStarts: []int{12}}
	f := newFieldText(ti)
	n := len(f.s)

	if s, e := f.expand(TextUnit_Word, 2); f.text(s, e, -1) != "こんにちは " {
		t.Errorf("word around 2: %q", f.text(s, e, -1))
	}
	if s, e := f.expand(TextUnit_Line, 13); s != 12 || e != n {
		t.Errorf("line around 13: %d..%d", s, e)
	}
	if s, e := f.expand(TextUnit_Paragraph, 0); f.text(s, e, -1) != "こんにちは world\n" {
		t.Errorf("first paragraph: %q", f.text(s, e, -1))
	}
	if s, e := f.expand(TextUnit_Document, 5); s != 0 || e != n {
		t.Error("document")
	}
	if s, e := f.expand(TextUnit_Format, 5); s != 0 || e != n {
		t.Error("format is the document")
	}
	if s, e := f.expand(TextUnit_Character, n); s != n-1 || e != n {
		t.Errorf("character at the end: %d..%d", s, e)
	}

	// A word range moves to the next word.
	s, e := f.expand(TextUnit_Word, 0)
	s, e, moved := f.move(TextUnit_Word, 1, s, e)
	if moved != 1 || f.text(s, e, -1) != "world\n" {
		t.Errorf("move by word: %d, %q", moved, f.text(s, e, -1))
	}
	// Past the last word it stops: "next " and "line" are left.
	_, _, moved = f.move(TextUnit_Word, 10, s, e)
	if moved != 2 {
		t.Errorf("move by 10 words from the second: moved %d, want 2", moved)
	}
	s, _, moved = f.move(TextUnit_Word, -5, s, e)
	if moved != -1 || s != 0 {
		t.Errorf("move back: moved %d, at %d", moved, s)
	}
	// A caret moves by boundaries and stays empty, up to the end.
	s, e, moved = f.move(TextUnit_Line, 3, 0, 0)
	if moved != 2 || s != n || e != n {
		t.Errorf("caret by lines: moved %d, %d..%d", moved, s, e)
	}

	s, e, moved = f.moveEndpoint(TextPatternRangeEndpoint_End, TextUnit_Character, 3, 0, 0)
	if moved != 3 || f.text(s, e, -1) != "こんに" {
		t.Errorf("end by 3 characters: %d, %q", moved, f.text(s, e, -1))
	}
	s, e, moved = f.moveEndpoint(TextPatternRangeEndpoint_Start, TextUnit_Document, 1, s, e)
	if moved != 1 || s != n || e != n {
		t.Errorf("start past the end collapses: %d, %d..%d", moved, s, e)
	}
	if got := f.text(0, n, 7); got != "こんにちは w" {
		t.Errorf("text limited to 7: %q", got)
	}
}

func TestFieldText_Surrogates(t *testing.T) {
	f := newFieldText(&semantics.TextInfo{Text: "a😀b"})
	if s, e, moved := f.move(TextUnit_Character, 1, 0, 1); moved != 1 || s != 1 || e != 3 {
		t.Errorf("the emoji is one character: %d..%d moved %d", s, e, moved)
	}
	if got := f.text(0, 3, 2); got != "a" {
		t.Errorf("a limit inside the pair drops the half: %q", got)
	}
	if s, e := f.expand(TextUnit_Character, 2); s != 1 || e != 3 {
		t.Errorf("expand inside the pair: %d..%d", s, e)
	}
}

func TestFieldText_Empty(t *testing.T) {
	f := newFieldText(&semantics.TextInfo{})
	if s, e := f.expand(TextUnit_Word, 0); s != 0 || e != 0 {
		t.Error("expand on empty text")
	}
	if _, _, moved := f.move(TextUnit_Character, 1, 0, 0); moved != 0 {
		t.Error("move on empty text")
	}
}

func TestFieldText_LineRects(t *testing.T) {
	f := newFieldText(&semantics.TextInfo{
		Text: "ab\ncd\nef", LineStarts: []int{3, 6},
		LineRects: []geom.Rect{{Y: 0, H: 10, W: 50}, {Y: 10, H: 10, W: 50}},
	})
	if got := f.lineRects(1, 4); len(got) != 2 {
		t.Errorf("two lines: %v", got)
	}
	if got := f.lineRects(6, 8); len(got) != 0 {
		t.Errorf("a line without a box: %v", got)
	}
	if got := f.lineRects(2, 2); got != nil {
		t.Error("an empty range has no box")
	}
	if f.lineAt(15) != 3 || f.lineAt(-5) != 0 || f.lineAt(100) != 3 {
		t.Errorf("lineAt: %d %d %d", f.lineAt(15), f.lineAt(-5), f.lineAt(100))
	}
}
