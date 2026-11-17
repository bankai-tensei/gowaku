// Package semantics describes a window to assistive technology: the nodes of
// the accessibility tree, their roles, states and actions, and the difference
// between two trees. core builds the tree, and the platform layer hands it to
// the screen readers of the OS (docs/ADR/0042-accessibility-windows.md).
//
// It sits below both and imports nothing but geom, so that platform can know
// the tree without knowing core (CODING_STYLE §3.1). It must not import core,
// platform or anything above them.
package semantics

import "github.com/bankai-tensei/gowaku/geom"

// ID identifies a node across trees, for as long as the object that described it
// stays in the window. Zero is never a valid ID.
type ID uint64

// Role says what a node is to assistive technology.
type Role uint8

// The roles. New roles go at the end, so that the values of the others stay.
const (
	RoleNone Role = iota
	RoleButton
	RoleCheckbox
	RoleImage
	RoleSlider
	RoleText
	RoleTextField
	// RoleProgress is a progress indicator. Its value is a percentage.
	RoleProgress
	// RoleSwitch is an on/off toggle. The Checked state says it is on.
	RoleSwitch
	// RoleRadio is one choice among those of a RoleRadioGroup.
	RoleRadio
	RoleRadioGroup
	// RoleTab is one page of a RoleTabList.
	RoleTab
	RoleTabList
	RoleList
	RoleListItem
	// RoleDialog is a window within the window, such as Dialog.
	RoleDialog
	// RoleGroup gathers related controls under a label.
	RoleGroup
	RoleHeading
	RoleLink
	// RoleStepper is a value with buttons that step it up and down, as
	// control.NumberStepper and field.NumberField: a spin button to UIA and
	// AT-SPI, an incrementor to NSAccessibility.
	RoleStepper
	// RoleMenuBar is the row of menus at the top of a window, and RoleMenu one
	// open menu. See docs/ADR/0046-menus-and-shortcuts.md.
	RoleMenuBar
	RoleMenu
	// RoleMenuItem is a command in a menu, or the title of a menu in the menu
	// bar, which is Expanded while its menu is open.
	RoleMenuItem
	// RoleMenuItemCheckbox is a menu item that is on or off. The Checked
	// state says it is on.
	RoleMenuItemCheckbox
	// RoleSeparator is a line between groups of menu items.
	RoleSeparator
	// RoleToolbar is a row of controls, such as Toolbar.
	RoleToolbar
	// RoleTooltip is the short text that a pointer resting on a control shows.
	RoleTooltip
	// RoleComboBox is a field or a button that opens a list of choices, such as
	// DropdownMenu and Autocomplete. Expanded says the list is open, and Value
	// is the choice.
	RoleComboBox
	// RoleGrid is a table of rows of cells that the keyboard moves through, such
	// as the days of a calendar. RoleRow is one row, RoleGridCell one cell and
	// RoleColumnHeader the title of a column.
	RoleGrid
	RoleRow
	RoleGridCell
	RoleColumnHeader
	// RoleTree is a list of items that open to show items below them, such as
	// TreeView. RoleTreeItem is one item: Level is its depth from 1, and
	// Expanded or Collapsed says whether it shows its children.
	RoleTree
	RoleTreeItem
)

// String returns the name of the role.
func (r Role) String() string {
	if int(r) < len(roleNames) {
		return roleNames[r]
	}
	return "unknown"
}

var roleNames = [...]string{
	RoleNone:       "none",
	RoleButton:     "button",
	RoleCheckbox:   "checkbox",
	RoleImage:      "image",
	RoleSlider:     "slider",
	RoleText:       "text",
	RoleTextField:  "textfield",
	RoleProgress:   "progress",
	RoleSwitch:     "switch",
	RoleRadio:      "radio",
	RoleRadioGroup: "radiogroup",
	RoleTab:        "tab",
	RoleTabList:    "tablist",
	RoleList:       "list",
	RoleListItem:   "listitem",
	RoleDialog:     "dialog",
	RoleGroup:      "group",
	RoleHeading:    "heading",
	RoleLink:       "link",
	RoleStepper:    "stepper",

	RoleMenuBar:          "menubar",
	RoleMenu:             "menu",
	RoleMenuItem:         "menuitem",
	RoleMenuItemCheckbox: "menuitemcheckbox",
	RoleSeparator:        "separator",
	RoleToolbar:          "toolbar",
	RoleTooltip:          "tooltip",
	RoleComboBox:         "combobox",
	RoleGrid:             "grid",
	RoleRow:              "row",
	RoleGridCell:         "gridcell",
	RoleColumnHeader:     "columnheader",
	RoleTree:             "tree",
	RoleTreeItem:         "treeitem",
}

// State is a set of conditions of a node.
type State uint16

// The states. Focusable and Focused are set by core; widgets leave them alone.
const (
	// Checked means a checkbox is ticked, a switch is on.
	Checked State = 1 << iota
	// Mixed means a checkbox is neither ticked nor clear.
	Mixed
	// Selected means the node is the chosen one of its group or list.
	Selected
	// Expanded and Collapsed are the two states of what can open and close.
	// A node with neither cannot.
	Expanded
	Collapsed
	Disabled
	Focusable
	Focused
	ReadOnly
	Multiline
	// Multiselectable is set on a list, grid or tree whose items can be
	// selected together.
	Multiselectable
	// Invalid means the value of a field fails its checks; the Hint says why.
	Invalid
	// Required means a form needs the field to be filled.
	Required
	// Obscured marks a password field. Its Value and Text are the marks it
	// shows, one for each character, never what was typed. The platforms
	// announce it as a password field: UIA IsPassword, the AT-SPI role
	// password text and the NSAccessibility secure text field.
	Obscured
)

// Actions is a set of things assistive technology can do to a node.
type Actions uint16

// The actions. New actions go at the end, so that the bits of the others stay.
const (
	ActionTap Actions = 1 << iota
	ActionFocus
	ActionIncrease
	ActionDecrease
	// ActionSetValue replaces the text of a field, or the number of a range.
	ActionSetValue
	ActionExpand
	ActionCollapse
	// ActionSetSelection selects a range of the text of a field.
	ActionSetSelection
	ActionScrollIntoView
)

// Live says how a node that changes is announced.
type Live uint8

// The kinds of live region.
const (
	// LiveOff announces nothing.
	LiveOff Live = iota
	// LivePolite announces changes once the reader is idle.
	LivePolite
	// LiveAssertive interrupts the reader.
	LiveAssertive
)

// Sort is the order a column header says its column is sorted in, as
// aria-sort. The zero value is no sort.
type Sort uint8

// The orders.
const (
	// SortNone means the column is not sorted, or the node is no header.
	SortNone Sort = iota
	SortAscending
	SortDescending
)

// String returns the name of the order, as aria-sort spells it.
func (s Sort) String() string {
	switch s {
	case SortAscending:
		return "ascending"
	case SortDescending:
		return "descending"
	}
	return "none"
}

// Range is the number of a slider, stepper or progress indicator. A node has
// one when Max > Min.
type Range struct {
	Min, Max, Value float64
	// Step is how far Increase and Decrease go. Zero leaves it to the reader.
	Step float64
}

// TextInfo describes the text of a field, for readers that follow the caret.
// Offsets count UTF-16 code units, as the UIA and AT-SPI interfaces do.
type TextInfo struct {
	// Text is the whole text, which is also the Value of the node.
	Text string
	// SelectionStart and SelectionEnd are the selection, or the caret where
	// they are equal. Start can be after End when the selection runs backward.
	SelectionStart, SelectionEnd int
	// LineStarts holds the offset at which each line after the first starts.
	LineStarts []int
	// LineRects holds the box of each line, in logical pixels of the window. It
	// may be shorter than the lines, as for a long field scrolled out of view.
	LineRects []geom.Rect
}

// Node is one entry of the accessibility tree.
type Node struct {
	// ID is set by core.
	ID      ID
	Role    Role
	Label   string
	Value   string
	Hint    string
	State   State
	Actions Actions
	Live    Live
	Range   Range
	// Text is set for text fields, and nil otherwise.
	Text *TextInfo
	// Level is the level of a heading or the depth of a tree item, from 1.
	Level int
	// PosInSet and SetSize place a tree or list item among its siblings, both from 1:
	// the third of five children is 3 of 5. A virtualized tree knows them when
	// the platform cannot count them, as only the rows in view have nodes. Zero
	// leaves them to the platform.
	PosInSet, SetSize int
	// Shortcut is the key that does what the node does from anywhere in the
	// window, as a menu shows it: "Ctrl+S".
	Shortcut string
	// RowCount and ColumnCount are the size of a grid, its header row
	// included. A grid that gives nodes only to the rows in view, such as
	// DataTable, sets them so that readers know how many rows there are;
	// zero means that the rows and cells below the node are all there are.
	RowCount, ColumnCount int
	// RowIndex and ColumnIndex place a row, a cell or a column header in its
	// grid, from 1, the header row being row 1. Zero means that the place is
	// that of the node among its siblings.
	RowIndex, ColumnIndex int
	// Sort is the order the column of a RoleColumnHeader is sorted in. The
	// platforms announce it: UIA ItemStatus and AriaProperties, the AT-SPI
	// attribute "sort", and NSAccessibility AXSortDirection.
	Sort Sort
	// Rect is in logical pixels from the top left of the window.
	Rect     geom.Rect
	Children []*Node
}

// Walk calls fn for n and every node below it, parents first, until fn returns
// false.
func (n *Node) Walk(fn func(*Node) bool) bool {
	if !fn(n) {
		return false
	}
	for _, c := range n.Children {
		if !c.Walk(fn) {
			return false
		}
	}
	return true
}

// Find returns the node of id below n, n included, or nil.
func (n *Node) Find(id ID) *Node {
	var found *Node
	n.Walk(func(m *Node) bool {
		if m.ID == id {
			found = m
			return false
		}
		return true
	})
	return found
}
