package dbus

import (
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file maps semantics nodes to AT-SPI without calling the bus, so that it
// is tested on any system (ADR 0043). The values are those of
// atspi-constants.h of at-spi2-core.

// toolkitName is the ToolkitName of the application and the toolkit attribute
// of every node.
const toolkitName = "gowaku"

// AtspiRole values.
const (
	atspiRoleInvalid     = 0
	atspiRoleCheckBox    = 7
	atspiRoleDialog      = 16
	atspiRoleFiller      = 20
	atspiRoleFrame       = 23
	atspiRoleImage       = 27
	atspiRoleLabel       = 29
	atspiRoleList        = 31
	atspiRoleListItem    = 32
	atspiRolePageTab     = 37
	atspiRolePageTabList = 38
	atspiRolePanel       = 39
	atspiRoleProgressBar = 42
	atspiRolePushButton  = 43
	atspiRoleRadioButton = 44
	atspiRoleSlider      = 51
	atspiRoleSpinButton  = 52
	atspiRoleText        = 61
	atspiRoleToggleBtn   = 62
	atspiRoleApplication = 75
	atspiRoleEntry       = 79
	atspiRoleHeading     = 83
	atspiRoleLink        = 88

	atspiRoleCheckMenuItem = 8
	atspiRoleMenu          = 33
	atspiRoleMenuBar       = 34
	atspiRoleMenuItem      = 35
	atspiRoleSeparator     = 50

	atspiRoleColumnHeader = 10
	atspiRoleComboBox     = 11
	atspiRoleTable        = 55
	atspiRoleTableCell    = 56
	atspiRoleToolBar      = 63
	atspiRoleToolTip      = 64
	atspiRoleTableRow     = 90
	atspiRoleTree         = 65
	atspiRoleTreeItem     = 91

	atspiRolePasswordText = 40
)

// atspiRoleNames are the names of atspi_role_get_name.
var atspiRoleNames = map[uint32]string{
	atspiRoleInvalid:     "invalid",
	atspiRoleCheckBox:    "check box",
	atspiRoleDialog:      "dialog",
	atspiRoleFiller:      "filler",
	atspiRoleFrame:       "frame",
	atspiRoleImage:       "image",
	atspiRoleLabel:       "label",
	atspiRoleList:        "list",
	atspiRoleListItem:    "list item",
	atspiRolePageTab:     "page tab",
	atspiRolePageTabList: "page tab list",
	atspiRolePanel:       "panel",
	atspiRoleProgressBar: "progress bar",
	atspiRolePushButton:  "push button",
	atspiRoleRadioButton: "radio button",
	atspiRoleSlider:      "slider",
	atspiRoleSpinButton:  "spin button",
	atspiRoleText:        "text",
	atspiRoleToggleBtn:   "toggle button",
	atspiRoleApplication: "application",
	atspiRoleEntry:       "entry",
	atspiRoleHeading:     "heading",
	atspiRoleLink:        "link",

	atspiRoleCheckMenuItem: "check menu item",
	atspiRoleMenu:          "menu",
	atspiRoleMenuBar:       "menu bar",
	atspiRoleMenuItem:      "menu item",
	atspiRoleSeparator:     "separator",

	atspiRoleColumnHeader: "column header",
	atspiRoleComboBox:     "combo box",
	atspiRoleTable:        "table",
	atspiRoleTableCell:    "table cell",
	atspiRoleToolBar:      "tool bar",
	atspiRoleToolTip:      "tool tip",
	atspiRoleTableRow:     "table row",
	atspiRoleTree:         "tree",
	atspiRoleTreeItem:     "tree item",

	atspiRolePasswordText: "password text",
}

// AtspiStateType values: bits of the two words of GetState.
const (
	atspiStateActive         = 1
	atspiStateChecked        = 4
	atspiStateCollapsed      = 5
	atspiStateEditable       = 7
	atspiStateEnabled        = 8
	atspiStateExpandable     = 9
	atspiStateExpanded       = 10
	atspiStateFocusable      = 11
	atspiStateFocused        = 12
	atspiStateModal          = 16
	atspiStateMultiLine      = 17
	atspiStateMultiselect    = 18
	atspiStateSelectable     = 22
	atspiStateSelected       = 23
	atspiStateSensitive      = 24
	atspiStateShowing        = 25
	atspiStateSingleLine     = 26
	atspiStateVisible        = 30
	atspiStateIndeterminate  = 32
	atspiStateRequired       = 33
	atspiStateInvalidEntry   = 36
	atspiStateSelectableText = 38
	atspiStateCheckable      = 41
	atspiStateReadOnly       = 43
)

// AtspiCoordType values.
const (
	atspiCoordScreen = 0
	atspiCoordWindow = 1
	atspiCoordParent = 2
)

// AtspiTextGranularity values.
const (
	atspiGranularityChar      = 0
	atspiGranularityWord      = 1
	atspiGranularitySentence  = 2
	atspiGranularityLine      = 3
	atspiGranularityParagraph = 4
)

// AtspiTextBoundaryType values.
const (
	atspiBoundaryChar          = 0
	atspiBoundaryWordStart     = 1
	atspiBoundaryWordEnd       = 2
	atspiBoundarySentenceStart = 3
	atspiBoundarySentenceEnd   = 4
	atspiBoundaryLineStart     = 5
	atspiBoundaryLineEnd       = 6
)

// atspiRoles maps the roles to AT-SPI (ADR 0042 §2). RoleNone and
// RoleTextField are decided by atspiRole.
var atspiRoles = [...]uint32{
	semantics.RoleButton:     atspiRolePushButton,
	semantics.RoleCheckbox:   atspiRoleCheckBox,
	semantics.RoleImage:      atspiRoleImage,
	semantics.RoleSlider:     atspiRoleSlider,
	semantics.RoleText:       atspiRoleLabel,
	semantics.RoleTextField:  atspiRoleEntry,
	semantics.RoleProgress:   atspiRoleProgressBar,
	semantics.RoleSwitch:     atspiRoleToggleBtn, // as GTK 4 maps its switch
	semantics.RoleRadio:      atspiRoleRadioButton,
	semantics.RoleRadioGroup: atspiRolePanel,
	semantics.RoleTab:        atspiRolePageTab,
	semantics.RoleTabList:    atspiRolePageTabList,
	semantics.RoleList:       atspiRoleList,
	semantics.RoleListItem:   atspiRoleListItem,
	semantics.RoleDialog:     atspiRoleDialog,
	semantics.RoleGroup:      atspiRolePanel,
	semantics.RoleHeading:    atspiRoleHeading, // with the level attribute
	semantics.RoleLink:       atspiRoleLink,
	semantics.RoleStepper:    atspiRoleSpinButton,

	semantics.RoleMenuBar:          atspiRoleMenuBar,
	semantics.RoleMenu:             atspiRoleMenu,
	semantics.RoleMenuItem:         atspiRoleMenuItem,
	semantics.RoleMenuItemCheckbox: atspiRoleCheckMenuItem,
	semantics.RoleSeparator:        atspiRoleSeparator,
	semantics.RoleToolbar:          atspiRoleToolBar,
	semantics.RoleTooltip:          atspiRoleToolTip,
	semantics.RoleComboBox:         atspiRoleComboBox,
	semantics.RoleGrid:             atspiRoleTable, // as GTK 4 maps a grid
	semantics.RoleRow:              atspiRoleTableRow,
	semantics.RoleGridCell:         atspiRoleTableCell,
	semantics.RoleColumnHeader:     atspiRoleColumnHeader,
	semantics.RoleTree:             atspiRoleTree,
	semantics.RoleTreeItem:         atspiRoleTreeItem,
}

// atspiRole returns the AT-SPI role of n. A field of several lines is a text
// rather than an entry. A node without a role is a panel when it holds others,
// a label when it only has a label, and a filler otherwise, as for UIA.
func atspiRole(n *semantics.Node) uint32 {
	if n.Role == semantics.RoleTextField && n.State&semantics.Obscured != 0 {
		return atspiRolePasswordText
	}
	if n.Role == semantics.RoleTextField && n.State&semantics.Multiline != 0 {
		return atspiRoleText
	}
	if n.Role != semantics.RoleNone && int(n.Role) < len(atspiRoles) {
		return atspiRoles[n.Role]
	}
	switch {
	case len(n.Children) > 0:
		return atspiRolePanel
	case n.Label != "" || n.Value != "":
		return atspiRoleLabel
	}
	return atspiRoleFiller
}

// atspiRoleName returns the name of an AT-SPI role.
func atspiRoleName(role uint32) string {
	if s, ok := atspiRoleNames[role]; ok {
		return s
	}
	return "unknown"
}

// stateSet is the two words of GetState.
type stateSet [2]uint32

func (s *stateSet) add(state uint) { s[state/32] |= 1 << (state % 32) }

func (s stateSet) has(state uint) bool { return s[state/32]&(1<<(state%32)) != 0 }

// atspiName returns the Name of n: its label, or for plain text without one,
// its value.
func atspiName(n *semantics.Node) string {
	if n.Label == "" && (n.Role == semantics.RoleText || n.Role == semantics.RoleHeading ||
		(n.Role == semantics.RoleNone && len(n.Children) == 0)) {
		return n.Value
	}
	return n.Label
}

func isEnabled(n *semantics.Node) bool { return n.State&semantics.Disabled == 0 }

func isCheckable(n *semantics.Node) bool {
	return n.Role == semantics.RoleCheckbox || n.Role == semantics.RoleSwitch || n.Role == semantics.RoleRadio ||
		n.Role == semantics.RoleMenuItemCheckbox
}

func isSelectable(n *semantics.Node) bool {
	return n.Role == semantics.RoleTab || n.Role == semantics.RoleListItem || n.Role == semantics.RoleRadio ||
		n.Role == semantics.RoleGridCell || n.Role == semantics.RoleTreeItem ||
		n.Role == semantics.RoleRow && n.Actions&semantics.ActionTap != 0
}

func hasRange(n *semantics.Node) bool { return n.Range.Max > n.Range.Min }

// isEditable reports whether the text of a field can be changed.
func isEditable(n *semantics.Node) bool {
	return n.Role == semantics.RoleTextField && n.State&(semantics.ReadOnly|semantics.Disabled) == 0 &&
		n.Actions&semantics.ActionSetValue != 0
}

// nodeStates returns the states of n, which has the keyboard focus if focused.
// Every node of the tree is on screen, so it is showing and visible.
func nodeStates(n *semantics.Node, focused bool) stateSet {
	var s stateSet
	s.add(atspiStateShowing)
	s.add(atspiStateVisible)
	if isEnabled(n) {
		s.add(atspiStateEnabled)
		s.add(atspiStateSensitive)
	}
	if n.State&semantics.Focusable != 0 {
		s.add(atspiStateFocusable)
	}
	if focused {
		s.add(atspiStateFocused)
	}
	if isCheckable(n) {
		s.add(atspiStateCheckable)
		// A radio button is checked when it is the chosen one.
		if n.State&semantics.Checked != 0 || (n.Role == semantics.RoleRadio && n.State&semantics.Selected != 0) {
			s.add(atspiStateChecked)
		}
		if n.State&semantics.Mixed != 0 {
			s.add(atspiStateIndeterminate)
		}
	}
	if isSelectable(n) {
		s.add(atspiStateSelectable)
		if n.State&semantics.Selected != 0 {
			s.add(atspiStateSelected)
		}
	}
	if n.State&(semantics.Expanded|semantics.Collapsed) != 0 {
		s.add(atspiStateExpandable)
		if n.State&semantics.Expanded != 0 {
			s.add(atspiStateExpanded)
		} else {
			s.add(atspiStateCollapsed)
		}
	}
	if n.State&semantics.ReadOnly != 0 {
		s.add(atspiStateReadOnly)
	}
	if n.Role == semantics.RoleTextField {
		if isEditable(n) {
			s.add(atspiStateEditable)
		}
		s.add(atspiStateSelectableText)
		if n.State&semantics.Multiline != 0 {
			s.add(atspiStateMultiLine)
		} else {
			s.add(atspiStateSingleLine)
		}
	}
	if n.Role == semantics.RoleDialog {
		s.add(atspiStateModal)
	}
	if n.State&semantics.Multiselectable != 0 {
		s.add(atspiStateMultiselect)
	}
	if n.State&semantics.Required != 0 {
		s.add(atspiStateRequired)
	}
	if n.State&semantics.Invalid != 0 {
		s.add(atspiStateInvalidEntry)
	}
	return s
}

// frameStates returns the states of the frame of a window.
func frameStates(active bool) stateSet {
	var s stateSet
	for _, st := range []uint{atspiStateEnabled, atspiStateSensitive, atspiStateShowing, atspiStateVisible, atspiStateFocusable} {
		s.add(st)
	}
	if active {
		s.add(atspiStateActive)
	}
	return s
}

// stateNames names the states that a change announces, as the detail of
// Object:StateChanged.
var stateNames = [...]struct {
	state uint
	name  string
}{
	{atspiStateChecked, "checked"},
	{atspiStateIndeterminate, "indeterminate"},
	{atspiStateSelected, "selected"},
	{atspiStateEnabled, "enabled"},
	{atspiStateSensitive, "sensitive"},
	{atspiStateExpanded, "expanded"},
	{atspiStateCollapsed, "collapsed"},
	{atspiStateEditable, "editable"},
	{atspiStateReadOnly, "read-only"},
}

// stateChange is one state that turned on or off.
type stateChange struct {
	name string
	on   bool
}

// changedStates returns the states that differ between before and after, of
// those that are announced. Focus is announced on its own.
func changedStates(before, after stateSet) []stateChange {
	var out []stateChange
	for _, s := range stateNames {
		if was, is := before.has(s.state), after.has(s.state); was != is {
			out = append(out, stateChange{s.name, is})
		}
	}
	return out
}

// atspiAction is one action of the Action interface.
type atspiAction struct {
	name, description string
	action            semantics.Actions
}

// nodeActions returns the actions of n, in the order of their indexes.
func nodeActions(n *semantics.Node) []atspiAction {
	var out []atspiAction
	for _, a := range [...]atspiAction{
		{"click", "Activates the element", semantics.ActionTap},
		{"increase", "Increases the value", semantics.ActionIncrease},
		{"decrease", "Decreases the value", semantics.ActionDecrease},
		{"expand", "Expands the element", semantics.ActionExpand},
		{"collapse", "Collapses the element", semantics.ActionCollapse},
	} {
		if n.Actions&a.action != 0 {
			out = append(out, a)
		}
	}
	return out
}

// ifaceSet is a set of the AT-SPI interfaces of an object.
type ifaceSet uint16

const (
	ifaceAccessible ifaceSet = 1 << iota
	ifaceApplication
	ifaceComponent
	ifaceAction
	ifaceValue
	ifaceText
	ifaceEditableText
)

// ifaceNames are the D-Bus names of the interfaces, in the order of the bits.
var ifaceNames = [...]string{
	"org.a11y.atspi.Accessible",
	"org.a11y.atspi.Application",
	"org.a11y.atspi.Component",
	"org.a11y.atspi.Action",
	"org.a11y.atspi.Value",
	"org.a11y.atspi.Text",
	"org.a11y.atspi.EditableText",
}

// names returns the D-Bus names of the interfaces of s.
func (s ifaceSet) names() []string {
	var out []string
	for i, name := range ifaceNames {
		if s&(1<<i) != 0 {
			out = append(out, name)
		}
	}
	return out
}

// ifaceOf returns the interface of a D-Bus name, or zero.
func ifaceOf(name string) ifaceSet {
	for i, n := range ifaceNames {
		if n == name {
			return 1 << i
		}
	}
	return 0
}

// nodeIfaces returns the interfaces of n.
func nodeIfaces(n *semantics.Node) ifaceSet {
	s := ifaceAccessible | ifaceComponent
	if len(nodeActions(n)) > 0 {
		s |= ifaceAction
	}
	if hasRange(n) {
		s |= ifaceValue
	}
	if n.Role == semantics.RoleTextField && n.Text != nil {
		s |= ifaceText
		if isEditable(n) {
			s |= ifaceEditableText
		}
	}
	return s
}

// nodeAttributes returns the object attributes of n (GetAttributes).
func nodeAttributes(n *semantics.Node) map[string]string {
	m := map[string]string{"toolkit": toolkitName}
	if n.Role == semantics.RoleHeading {
		m["level"] = strconv.Itoa(max(n.Level, 1))
	}
	if n.Role == semantics.RoleTreeItem {
		// The attributes of ARIA, as Chromium hands them to Orca.
		if n.Level > 0 {
			m["level"] = strconv.Itoa(n.Level)
		}
		if n.PosInSet > 0 {
			m["posinset"] = strconv.Itoa(n.PosInSet)
		}
		if n.SetSize > 0 {
			m["setsize"] = strconv.Itoa(n.SetSize)
		}
	}
	if n.Role == semantics.RoleColumnHeader && n.Sort != semantics.SortNone {
		// aria-sort, as Chromium hands it to Orca.
		m["sort"] = n.Sort.String()
	}
	if n.Shortcut != "" {
		// The attribute of ARIA, which Orca reads.
		m["keyshortcuts"] = n.Shortcut
	}
	switch n.Live {
	case semantics.LivePolite:
		m["container-live"], m["live"] = "polite", "polite"
	case semantics.LiveAssertive:
		m["container-live"], m["live"] = "assertive", "assertive"
	}
	// The size of a grid that has nodes only for the rows in view, and the
	// places in it, as the ARIA attributes that Orca reads, from 1.
	for _, a := range [...]struct {
		name string
		v    int
	}{{"rowcount", n.RowCount}, {"colcount", n.ColumnCount}, {"rowindex", n.RowIndex}, {"colindex", n.ColumnIndex}} {
		if a.v > 0 {
			m[a.name] = strconv.Itoa(a.v)
		}
	}
	return m
}

// atspiValueText is the Text of the Value interface: what the node says its
// value is, or else the number.
func atspiValueText(n *semantics.Node) string {
	if n.Value != "" {
		return n.Value
	}
	return strconv.FormatFloat(n.Range.Value, 'g', -1, 64)
}

// minimumIncrement is the step of a range, or a hundredth of it without one.
func minimumIncrement(r semantics.Range) float64 {
	if r.Step > 0 {
		return r.Step
	}
	return (r.Max - r.Min) / 100
}

// announcement returns what a live region says when it changes.
func announcement(n *semantics.Node) string {
	name := atspiName(n)
	value := n.Value
	if n.Text != nil {
		value = n.Text.Text
	}
	if value == "" || value == name {
		return name
	}
	if name == "" {
		return value
	}
	return name + " " + value
}

// Object paths. The application is the root, and every other object is a
// number under it: the window in the high bits and the semantics ID in the
// low ones, so that the IDs of two windows never meet. The semantics root,
// ID zero, is the frame of the window.
const (
	accessiblePrefix = "/org/a11y/atspi/accessible/"
	rootPath         = accessiblePrefix + "root"
	nullPath         = "/org/a11y/atspi/null"

	windowShift = 48
	idMask      = 1<<windowShift - 1
)

// nodePath returns the path of node id of window win, which counts from one.
func nodePath(win uint16, id semantics.ID) string {
	return accessiblePrefix + strconv.FormatUint(uint64(win)<<windowShift|uint64(id)&idMask, 10)
}

// parseNodePath returns the window and ID of a path of nodePath.
func parseNodePath(path string) (win uint16, id semantics.ID, ok bool) {
	s, found := strings.CutPrefix(path, accessiblePrefix)
	if !found {
		return 0, 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v>>windowShift == 0 {
		return 0, 0, false
	}
	return uint16(v >> windowShift), semantics.ID(v & idMask), true
}

// tree is the tree of the last Update of a window, indexed for the questions of
// AT-SPI. It is never changed once made, so readers may keep it without a lock.
// The root has the ID zero and stands for the window.
type tree struct {
	root    *semantics.Node
	nodes   map[semantics.ID]*semantics.Node
	parents map[semantics.ID]semantics.ID
	// focus is the node with the keyboard focus, or zero.
	focus semantics.ID
}

func newTree(root *semantics.Node, focus semantics.ID) *tree {
	t := &tree{
		root:    root,
		nodes:   make(map[semantics.ID]*semantics.Node),
		parents: make(map[semantics.ID]semantics.ID),
		focus:   focus,
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

// node returns the node of id, or nil if the tree lacks it. Zero is the root,
// which is nil before the first Update.
func (t *tree) node(id semantics.ID) *semantics.Node {
	if id == 0 {
		return t.root
	}
	return t.nodes[id]
}

// has reports whether the tree holds id. The root is always there.
func (t *tree) has(id semantics.ID) bool { return id == 0 || t.nodes[id] != nil }

// children returns the children of id.
func (t *tree) children(id semantics.ID) []*semantics.Node {
	if n := t.node(id); n != nil {
		return n.Children
	}
	return nil
}

// indexInParent returns where id is among the children of its parent, or -1.
func (t *tree) indexInParent(id semantics.ID) int {
	pid, ok := t.parents[id]
	if !ok {
		return -1
	}
	for i, c := range t.children(pid) {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// hitTest returns the deepest node below id whose box holds p, in logical
// pixels of the window. Of overlapping siblings the last, drawn on top, wins.
func (t *tree) hitTest(id semantics.ID, p geom.Offset) (semantics.ID, bool) {
	var found semantics.ID
	ok := false
	n := t.node(id)
	for n != nil {
		var next *semantics.Node
		for i := len(n.Children) - 1; i >= 0; i-- {
			if c := n.Children[i]; c.Rect.Contains(p) {
				next = c
				break
			}
		}
		if next != nil {
			found, ok = next.ID, true
		}
		n = next
	}
	return found, ok
}

// rect returns the box of id in logical pixels of the window. The root covers
// the window.
func (t *tree) rect(id semantics.ID) geom.Rect {
	if n := t.node(id); n != nil {
		return n.Rect
	}
	return geom.Rect{}
}

// WindowGeometry is where a window is, for the extents that readers ask for:
// its origin on the screen and its size in physical pixels, and the physical
// pixels of one logical pixel. The origin is zero where the window system
// does not tell, as on Wayland.
type WindowGeometry struct {
	X, Y, Width, Height int32
	Scale               float32
}

// scale returns the scale, one if unknown.
func (g WindowGeometry) scale() float32 {
	if g.Scale <= 0 {
		return 1
	}
	return g.Scale
}

// rect returns the window in logical pixels of itself.
func (g WindowGeometry) rect() geom.Rect {
	s := g.scale()
	return geom.Rect{W: float32(g.Width) / s, H: float32(g.Height) / s}
}

// extents is a box in AT-SPI coordinates.
type extents struct{ X, Y, W, H int32 }

// toAtspi turns a box in logical pixels of the window into the coordinates of
// coord. parent is the box of the parent, for atspiCoordParent.
func (g WindowGeometry) toAtspi(r, parent geom.Rect, coord uint32) extents {
	s := g.scale()
	e := extents{round(r.X * s), round(r.Y * s), round(r.W * s), round(r.H * s)}
	switch coord {
	case atspiCoordScreen:
		e.X += g.X
		e.Y += g.Y
	case atspiCoordParent:
		e.X -= round(parent.X * s)
		e.Y -= round(parent.Y * s)
	}
	return e
}

// fromAtspi turns a point in the coordinates of coord into logical pixels of
// the window. parent is the box of the parent, for atspiCoordParent.
func (g WindowGeometry) fromAtspi(x, y int32, parent geom.Rect, coord uint32) geom.Offset {
	s := g.scale()
	px, py := float32(x), float32(y)
	switch coord {
	case atspiCoordScreen:
		px -= float32(g.X)
		py -= float32(g.Y)
	case atspiCoordParent:
		px += parent.X * s
		py += parent.Y * s
	}
	return geom.Offset{X: px / s, Y: py / s}
}

func round(f float32) int32 {
	if f < 0 {
		return int32(f - 0.5)
	}
	return int32(f + 0.5)
}

// union returns the smallest box that holds both, where an empty a holds
// nothing.
func union(a, b geom.Rect) geom.Rect {
	if a.W <= 0 && a.H <= 0 {
		return b
	}
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
