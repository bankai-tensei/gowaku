package cocoa

import (
	"unicode/utf16"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file maps semantics nodes to NSAccessibility without calling AppKit,
// so that it is tested on any system (docs/ADR/0085-accessibility-macos.md).
// The role strings are the values of the NSAccessibility*Role constants of
// NSAccessibilityConstants.h, and of the roles that WebKit and Chromium add.

// NSAccessibility roles and subroles.
const (
	axRoleButton            = "AXButton"
	axRoleCell              = "AXCell"
	axRoleCheckBox          = "AXCheckBox"
	axRoleComboBox          = "AXComboBox"
	axRoleGroup             = "AXGroup"
	axRoleHeading           = "AXHeading"
	axRoleImage             = "AXImage"
	axRoleIncrementor       = "AXIncrementor"
	axRoleLink              = "AXLink"
	axRoleList              = "AXList"
	axRoleMenu              = "AXMenu"
	axRoleMenuBar           = "AXMenuBar"
	axRoleMenuItem          = "AXMenuItem"
	axRoleOutline           = "AXOutline"
	axRolePopUpButton       = "AXPopUpButton"
	axRoleProgressIndicator = "AXProgressIndicator"
	axRoleRadioButton       = "AXRadioButton"
	axRoleRadioGroup        = "AXRadioGroup"
	axRoleRow               = "AXRow"
	axRoleSlider            = "AXSlider"
	axRoleSplitter          = "AXSplitter"
	axRoleStaticText        = "AXStaticText"
	axRoleTabGroup          = "AXTabGroup"
	axRoleTable             = "AXTable"
	axRoleTextArea          = "AXTextArea"
	axRoleTextField         = "AXTextField"
	axRoleToolbar           = "AXToolbar"

	axSubroleApplicationDialog = "AXApplicationDialog"
	axSubroleContentList       = "AXContentList"
	axSubroleOutlineRow        = "AXOutlineRow"
	axSubroleSwitch            = "AXSwitch"
	axSubroleTabButton         = "AXTabButton"
	axSubroleTooltip           = "AXUserInterfaceTooltip"

	axSubroleSecureTextField = "AXSecureTextField"
)

// axRoles maps the roles to NSAccessibility (ADR 0042 §2, ADR 0085). The roles
// whose NSAccessibility role depends on more than the role are decided by
// axRole.
var axRoles = [...]struct{ role, subrole string }{
	semantics.RoleButton:     {axRoleButton, ""},
	semantics.RoleCheckbox:   {axRoleCheckBox, ""},
	semantics.RoleImage:      {axRoleImage, ""},
	semantics.RoleSlider:     {axRoleSlider, ""},
	semantics.RoleText:       {axRoleStaticText, ""},
	semantics.RoleProgress:   {axRoleProgressIndicator, ""},
	semantics.RoleSwitch:     {axRoleCheckBox, axSubroleSwitch},
	semantics.RoleRadio:      {axRoleRadioButton, ""},
	semantics.RoleRadioGroup: {axRoleRadioGroup, ""},
	semantics.RoleTab:        {axRoleRadioButton, axSubroleTabButton},
	semantics.RoleTabList:    {axRoleTabGroup, ""},
	semantics.RoleList:       {axRoleList, axSubroleContentList},
	semantics.RoleListItem:   {axRoleGroup, ""}, // as Chromium maps list items
	semantics.RoleDialog:     {axRoleGroup, axSubroleApplicationDialog},
	semantics.RoleGroup:      {axRoleGroup, ""},
	semantics.RoleHeading:    {axRoleHeading, ""},
	semantics.RoleLink:       {axRoleLink, ""},
	semantics.RoleStepper:    {axRoleIncrementor, ""},

	semantics.RoleMenuBar:          {axRoleMenuBar, ""},
	semantics.RoleMenu:             {axRoleMenu, ""},
	semantics.RoleMenuItem:         {axRoleMenuItem, ""},
	semantics.RoleMenuItemCheckbox: {axRoleMenuItem, ""}, // with a value of 0 or 1
	semantics.RoleSeparator:        {axRoleSplitter, ""},
	semantics.RoleToolbar:          {axRoleToolbar, ""},
	semantics.RoleTooltip:          {axRoleGroup, axSubroleTooltip},
	semantics.RoleGrid:             {axRoleTable, ""},
	semantics.RoleRow:              {axRoleRow, ""},
	semantics.RoleGridCell:         {axRoleCell, ""},
	semantics.RoleColumnHeader:     {axRoleCell, ""},
	semantics.RoleTree:             {axRoleOutline, ""},
	semantics.RoleTreeItem:         {axRoleRow, axSubroleOutlineRow},
}

// axRole returns the role and subrole of n. A text field is a text area when
// it has many lines, and a combo box a pop-up button when it cannot be typed
// into. A node without a role is a group when it holds others, static text
// when it only has a label, and an empty group otherwise.
func axRole(n *semantics.Node) (role, subrole string) {
	switch n.Role {
	case semantics.RoleTextField:
		if n.State&semantics.Obscured != 0 {
			return axRoleTextField, axSubroleSecureTextField
		}
		if n.State&semantics.Multiline != 0 {
			return axRoleTextArea, ""
		}
		return axRoleTextField, ""
	case semantics.RoleComboBox:
		if n.Text != nil {
			return axRoleComboBox, ""
		}
		return axRolePopUpButton, ""
	case semantics.RoleNone:
		if len(n.Children) == 0 && n.Label != "" {
			return axRoleStaticText, ""
		}
		return axRoleGroup, ""
	}
	if int(n.Role) < len(axRoles) && axRoles[n.Role].role != "" {
		r := axRoles[n.Role]
		return r.role, r.subrole
	}
	return axRoleGroup, ""
}

// axIsStaticText reports whether n is read as static text, which VoiceOver
// reads by its value.
func axIsStaticText(n *semantics.Node) bool {
	role, _ := axRole(n)
	return role == axRoleStaticText
}

// axLabel returns the AXDescription of n. Static text says its text as its
// value instead, unless it has both a label and a value; a heading without a
// label says its value, as its value is its level.
func axLabel(n *semantics.Node) string {
	switch {
	case axIsStaticText(n):
		if n.Value == "" {
			return ""
		}
		return n.Label
	case n.Role == semantics.RoleHeading && n.Label == "":
		return n.Value
	}
	return n.Label
}

// axValueKind is the type of the AXValue of a node.
type axValueKind uint8

const (
	axValueNone axValueKind = iota
	// axValueInt is the state of a checkbox, a radio button or a tab: 0, 1,
	// or 2 for mixed; or the level of a heading.
	axValueInt
	// axValueNumber is the value of a range.
	axValueNumber
	axValueString
)

// axValue is the AXValue of a node, before it becomes an NSNumber or an
// NSString.
type axValue struct {
	kind   axValueKind
	number float64
	text   string
}

func axIsToggle(n *semantics.Node) bool {
	return n.Role == semantics.RoleCheckbox || n.Role == semantics.RoleSwitch || n.Role == semantics.RoleMenuItemCheckbox
}

func axHasRange(n *semantics.Node) bool { return n.Range.Max > n.Range.Min }

// axNodeValue returns the AXValue of n.
func axNodeValue(n *semantics.Node) axValue {
	switch {
	case axIsToggle(n):
		switch {
		case n.State&semantics.Mixed != 0:
			return axValue{kind: axValueInt, number: 2}
		case n.State&semantics.Checked != 0:
			return axValue{kind: axValueInt, number: 1}
		}
		return axValue{kind: axValueInt}
	case n.Role == semantics.RoleRadio || n.Role == semantics.RoleTab:
		if n.State&(semantics.Selected|semantics.Checked) != 0 {
			return axValue{kind: axValueInt, number: 1}
		}
		return axValue{kind: axValueInt}
	case axHasRange(n):
		return axValue{kind: axValueNumber, number: n.Range.Value}
	case n.Role == semantics.RoleHeading:
		if n.Level > 0 {
			return axValue{kind: axValueInt, number: float64(n.Level)}
		}
		return axValue{}
	case n.Text != nil:
		return axValue{kind: axValueString, text: n.Text.Text}
	case n.Role == semantics.RoleTextField:
		return axValue{kind: axValueString, text: n.Value}
	case axIsStaticText(n):
		if n.Value != "" {
			return axValue{kind: axValueString, text: n.Value}
		}
		return axValue{kind: axValueString, text: n.Label}
	case n.Value != "":
		return axValue{kind: axValueString, text: n.Value}
	}
	return axValue{}
}

// axIsSelectionItem reports whether n is one of the choices of a selection
// container. A row is one when it can be tapped, as the rows of a DataTable
// that select.
func axIsSelectionItem(n *semantics.Node) bool {
	return n.Role == semantics.RoleRadio || n.Role == semantics.RoleTab || n.Role == semantics.RoleListItem ||
		n.Role == semantics.RoleGridCell || n.Role == semantics.RoleTreeItem ||
		n.Role == semantics.RoleRow && n.Actions&semantics.ActionTap != 0
}

// axIsSelectionContainer reports whether n holds choices that can be selected.
func axIsSelectionContainer(n *semantics.Node) bool {
	switch n.Role {
	case semantics.RoleRadioGroup, semantics.RoleTabList, semantics.RoleList, semantics.RoleGrid, semantics.RoleTree:
		return true
	}
	return n.State&semantics.Multiselectable != 0
}

// The values of NSAccessibilitySortDirection.
const (
	axSortUnknown    = 0
	axSortAscending  = 1
	axSortDescending = 2
)

// axSortDirection returns the AXSortDirection of n, a column header, as
// WebKit says aria-sort to VoiceOver.
func axSortDirection(n *semantics.Node) int64 {
	switch n.Sort {
	case semantics.SortAscending:
		return axSortAscending
	case semantics.SortDescending:
		return axSortDescending
	}
	return axSortUnknown
}

func axExpandable(n *semantics.Node) bool {
	return n.State&(semantics.Expanded|semantics.Collapsed) != 0
}

func axEnabled(n *semantics.Node) bool { return n.State&semantics.Disabled == 0 }

// axCanSetValue reports whether a reader may write the AXValue of n.
func axCanSetValue(n *semantics.Node) bool {
	if n.State&(semantics.ReadOnly|semantics.Disabled) != 0 {
		return false
	}
	if n.Actions&semantics.ActionSetValue != 0 {
		return true
	}
	return axHasRange(n) && n.Actions&(semantics.ActionIncrease|semantics.ActionDecrease) != 0
}

// axSetRangeAction returns the action that moves the range of n to v: a
// SetValue, or for a node that only steps, one step toward v. It reports false
// when v is out of the range or there is nothing to do.
func axSetRangeAction(n *semantics.Node, v float64) (semantics.Actions, bool) {
	if v != v || v < n.Range.Min || v > n.Range.Max {
		return 0, false
	}
	switch {
	case n.Actions&semantics.ActionSetValue != 0:
		return semantics.ActionSetValue, true
	case v > n.Range.Value && n.Actions&semantics.ActionIncrease != 0:
		return semantics.ActionIncrease, true
	case v < n.Range.Value && n.Actions&semantics.ActionDecrease != 0:
		return semantics.ActionDecrease, true
	}
	return 0, false
}

// axExpandAction returns the action that opens (or closes) n: Expand or
// Collapse, or a Tap where only that is offered, as UIA does.
func axExpandAction(n *semantics.Node, open bool) (semantics.Actions, bool) {
	action, from := semantics.ActionCollapse, semantics.Expanded
	if open {
		action, from = semantics.ActionExpand, semantics.Collapsed
	}
	switch {
	case n.State&from == 0:
		return 0, false
	case n.Actions&action != 0:
		return action, true
	case n.Actions&semantics.ActionTap != 0:
		return semantics.ActionTap, true
	}
	return 0, false
}

// axTree is the tree of the last Update, indexed for the questions of
// NSAccessibility. The root has the ID zero and stands for the window.
type axTree struct {
	root    *semantics.Node
	nodes   map[semantics.ID]*semantics.Node
	parents map[semantics.ID]semantics.ID
}

func newAXTree(root *semantics.Node) axTree {
	t := axTree{
		root:    root,
		nodes:   make(map[semantics.ID]*semantics.Node),
		parents: make(map[semantics.ID]semantics.ID),
	}
	if root != nil {
		var walk func(n *semantics.Node)
		walk = func(n *semantics.Node) {
			for _, c := range n.Children {
				t.nodes[c.ID] = c
				t.parents[c.ID] = n.ID
				walk(c)
			}
		}
		walk(root)
	}
	return t
}

// node returns the node of id, the root for zero, or nil.
func (t *axTree) node(id semantics.ID) *semantics.Node {
	if id == 0 {
		return t.root
	}
	return t.nodes[id]
}

// has reports whether the tree holds id, which is not the root.
func (t *axTree) has(id semantics.ID) bool { return id != 0 && t.nodes[id] != nil }

// hitTest returns the deepest node whose box holds p, in logical pixels of
// the window, or zero. Of overlapping siblings the last, drawn on top, wins.
func (t *axTree) hitTest(p geom.Offset) semantics.ID {
	var found semantics.ID
	n := t.root
	for n != nil {
		var next *semantics.Node
		for i := len(n.Children) - 1; i >= 0; i-- {
			if c := n.Children[i]; c.Rect.Contains(p) {
				next = c
				break
			}
		}
		if next != nil {
			found = next.ID
		}
		n = next
	}
	return found
}

// selectionContainer returns the nearest node above id that holds choices.
func (t *axTree) selectionContainer(id semantics.ID) (semantics.ID, bool) {
	for {
		pid, ok := t.parents[id]
		if !ok || pid == 0 {
			return 0, false
		}
		if parent := t.node(pid); parent != nil && axIsSelectionContainer(parent) {
			return pid, true
		}
		id = pid
	}
}

// selection returns the selected choices of the container id, in the order of
// the tree, without looking inside a choice or another container.
func (t *axTree) selection(id semantics.ID) []semantics.ID {
	n := t.node(id)
	if n == nil {
		return nil
	}
	var out []semantics.ID
	var walk func(n *semantics.Node)
	walk = func(n *semantics.Node) {
		for _, c := range n.Children {
			switch {
			case axIsSelectionItem(c):
				if c.State&semantics.Selected != 0 {
					out = append(out, c.ID)
				}
			case !axIsSelectionContainer(c):
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

// rows returns the AXRows of a table or an outline: the rows of a grid, or the
// items of a tree at every depth, in the order of the tree. It does not look
// inside another table or outline.
func (t *axTree) rows(id semantics.ID) []semantics.ID {
	n := t.node(id)
	if n == nil {
		return nil
	}
	want := semantics.RoleRow
	if n.Role == semantics.RoleTree {
		want = semantics.RoleTreeItem
	}
	var out []semantics.ID
	var walk func(n *semantics.Node)
	walk = func(n *semantics.Node) {
		for _, c := range n.Children {
			if c.Role == semantics.RoleGrid || c.Role == semantics.RoleTree {
				continue
			}
			if c.Role == want {
				out = append(out, c.ID)
			}
			walk(c)
		}
	}
	walk(n)
	return out
}

// disclosedBy returns the tree item that holds the tree item id, or zero for
// one at the top of its tree.
func (t *axTree) disclosedBy(id semantics.ID) semantics.ID {
	for {
		pid, ok := t.parents[id]
		if !ok || pid == 0 {
			return 0
		}
		switch t.node(pid).Role {
		case semantics.RoleTreeItem:
			return pid
		case semantics.RoleTree:
			return 0
		}
		id = pid
	}
}

// disclosed returns the tree items right below the tree item id.
func (t *axTree) disclosed(id semantics.ID) []semantics.ID {
	n := t.node(id)
	if n == nil {
		return nil
	}
	var out []semantics.ID
	var walk func(n *semantics.Node)
	walk = func(n *semantics.Node) {
		for _, c := range n.Children {
			switch c.Role {
			case semantics.RoleTreeItem:
				out = append(out, c.ID)
			case semantics.RoleTree:
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

// axClamp clamps a range of UTF-16 code units to a text of n units.
func axClamp(start, length, n int) (int, int) {
	start = min(max(start, 0), n)
	end := start + min(max(length, 0), n-start)
	return start, end
}

// axSubstring returns the code units from start to end of s.
func axSubstring(s []uint16, start, end int) string {
	start, end = axClamp(start, end-start, len(s))
	return string(utf16.Decode(s[start:end]))
}

// axSelection returns the selection of a field as a location and a
// length, the selection running forward.
func axSelection(t *semantics.TextInfo, n int) (start, length int) {
	a, b := t.SelectionStart, t.SelectionEnd
	if a > b {
		a, b = b, a
	}
	a, b = axClamp(a, b-a, n)
	return a, b - a
}

// axRangeRect returns the box of a range of a field, in logical pixels of the
// window: the union of the boxes of the lines that it touches, or the box of
// the node where the field leaves them out.
func axRangeRect(n *semantics.Node, start, length int) geom.Rect {
	t := n.Text
	if t == nil || len(t.LineRects) == 0 {
		return n.Rect
	}
	first, last := t.Line(start), t.Line(start+max(length-1, 0))
	var r geom.Rect
	found := false
	for i := first; i <= last && i < len(t.LineRects); i++ {
		lr := t.LineRects[i]
		if !found {
			r, found = lr, true
			continue
		}
		x0, y0 := min(r.X, lr.X), min(r.Y, lr.Y)
		x1, y1 := max(r.X+r.W, lr.X+lr.W), max(r.Y+r.H, lr.Y+lr.H)
		r = geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	if !found {
		return n.Rect
	}
	return r
}

// axNoteKind is a notification of NSAccessibility.
type axNoteKind uint8

const (
	// noteLayout is AXLayoutChanged, on the window.
	noteLayout axNoteKind = iota
	noteValue
	noteTitle
	noteSelectedText
	// noteSelectedChildren is AXSelectedChildrenChanged, on the container.
	noteSelectedChildren
	noteRowExpanded
	noteRowCollapsed
	noteFocus
	// noteAnnounce is AXAnnouncementRequested with text, for a live region.
	noteAnnounce
)

// axNote is a notification to post about a node.
type axNote struct {
	kind      axNoteKind
	id        semantics.ID
	text      string
	assertive bool
}

// axNotes returns the notifications that tell VoiceOver what d changed, from
// the tree before to the tree after, with the focus now on focus.
func axNotes(before, after *axTree, d *semantics.Diff, focus semantics.ID) []axNote {
	var notes []axNote
	layout := len(d.Added) > 0 || len(d.Removed) > 0
	for _, c := range d.Changed {
		if c.What&semantics.ChangedChildren != 0 {
			layout = true
		}
	}
	if layout {
		notes = append(notes, axNote{kind: noteLayout})
	}
	containers := map[semantics.ID]bool{}
	for _, c := range d.Changed {
		n, old := c.Node, before.node(c.Node.ID)
		if old == nil || n.ID == 0 {
			continue
		}
		titleChanged := axLabel(old) != axLabel(n)
		valueChanged := axNodeValue(old) != axNodeValue(n)
		if titleChanged {
			notes = append(notes, axNote{kind: noteTitle, id: n.ID})
		}
		if valueChanged {
			notes = append(notes, axNote{kind: noteValue, id: n.ID})
		}
		if n.Text != nil && c.What&semantics.ChangedSelection != 0 {
			notes = append(notes, axNote{kind: noteSelectedText, id: n.ID})
		}
		if axIsSelectionItem(n) && (old.State^n.State)&semantics.Selected != 0 {
			if cid, ok := after.selectionContainer(n.ID); ok && !containers[cid] {
				containers[cid] = true
				notes = append(notes, axNote{kind: noteSelectedChildren, id: cid})
			}
		}
		if n.Role == semantics.RoleTreeItem && (old.State^n.State)&(semantics.Expanded|semantics.Collapsed) != 0 {
			switch {
			case n.State&semantics.Expanded != 0:
				notes = append(notes, axNote{kind: noteRowExpanded, id: n.ID})
			case n.State&semantics.Collapsed != 0:
				notes = append(notes, axNote{kind: noteRowCollapsed, id: n.ID})
			}
		}
		// A live region speaks when it changes, not when it appears (ADR 0043).
		if n.Live != semantics.LiveOff && (titleChanged || valueChanged) {
			if text := axLiveText(n); text != "" {
				notes = append(notes, axNote{kind: noteAnnounce, id: n.ID, text: text, assertive: n.Live == semantics.LiveAssertive})
			}
		}
	}
	if d.FocusMoved && focus != 0 && after.has(focus) {
		notes = append(notes, axNote{kind: noteFocus, id: focus})
	}
	return notes
}

// axLiveText is what a live region says when it changes: its label and value.
func axLiveText(n *semantics.Node) string {
	label, value := axLabel(n), ""
	if v := axNodeValue(n); v.kind == axValueString {
		value = v.text
	}
	switch {
	case label == "":
		return value
	case value == "" || value == label:
		return label
	}
	return label + " " + value
}
