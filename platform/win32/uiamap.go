package win32

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/semantics"
)

// This file maps semantics nodes to UI Automation without calling the OS, so
// that it is tested on any system. The values are those of the Windows SDK
// headers UIAutomationClient.h, UIAutomationCore.h and UIAutomationCoreApi.h.

// WM_GETOBJECT asks a window for its accessibility object.
const WM_GETOBJECT = 0x003D

// UI Automation values.
const (
	UiaRootObjectId    = -25
	UiaAppendRuntimeId = 3

	ProviderOptions_ServerSideProvider = 0x2

	NavigateDirection_Parent          = 0
	NavigateDirection_NextSibling     = 1
	NavigateDirection_PreviousSibling = 2
	NavigateDirection_FirstChild      = 3
	NavigateDirection_LastChild       = 4

	StructureChangeType_ChildrenInvalidated = 2

	ToggleState_Off           = 0
	ToggleState_On            = 1
	ToggleState_Indeterminate = 2

	ExpandCollapseState_Collapsed = 0
	ExpandCollapseState_Expanded  = 1
	ExpandCollapseState_LeafNode  = 3

	SupportedTextSelection_Single = 1

	TextUnit_Character = 0
	TextUnit_Format    = 1
	TextUnit_Word      = 2
	TextUnit_Line      = 3
	TextUnit_Paragraph = 4
	TextUnit_Page      = 5
	TextUnit_Document  = 6

	TextPatternRangeEndpoint_Start = 0
	TextPatternRangeEndpoint_End   = 1

	RowOrColumnMajor_RowMajor = 0

	HeadingLevel_None = 80050
	HeadingLevel1     = 80051
	HeadingLevel9     = 80059

	UIA_E_ELEMENTNOTENABLED   = 0x80040200
	UIA_E_ELEMENTNOTAVAILABLE = 0x80040201
	UIA_E_INVALIDOPERATION    = 0x80131509

	UIA_InvokePatternId         = 10000
	UIA_SelectionPatternId      = 10001
	UIA_ValuePatternId          = 10002
	UIA_RangeValuePatternId     = 10003
	UIA_ExpandCollapsePatternId = 10005
	UIA_GridPatternId           = 10006
	UIA_GridItemPatternId       = 10007
	UIA_SelectionItemPatternId  = 10010
	UIA_TablePatternId          = 10012
	UIA_TableItemPatternId      = 10013
	UIA_TextPatternId           = 10014
	UIA_TogglePatternId         = 10015
	UIA_ScrollItemPatternId     = 10017
	UIA_TextPattern2Id          = 10024

	UIA_StructureChangedEventId              = 20002
	UIA_AutomationFocusChangedEventId        = 20005
	UIA_SelectionItem_ElementSelectedEventId = 20012
	UIA_Text_TextSelectionChangedEventId     = 20014
	UIA_Text_TextChangedEventId              = 20015
	UIA_LiveRegionChangedEventId             = 20024

	UIA_ControlTypePropertyId                       = 30003
	UIA_NamePropertyId                              = 30005
	UIA_AcceleratorKeyPropertyId                    = 30006
	UIA_HasKeyboardFocusPropertyId                  = 30008
	UIA_IsKeyboardFocusablePropertyId               = 30009
	UIA_IsEnabledPropertyId                         = 30010
	UIA_AutomationIdPropertyId                      = 30011
	UIA_ClassNamePropertyId                         = 30012
	UIA_HelpTextPropertyId                          = 30013
	UIA_IsControlElementPropertyId                  = 30016
	UIA_IsContentElementPropertyId                  = 30017
	UIA_IsPasswordPropertyId                        = 30019
	UIA_FrameworkIdPropertyId                       = 30024
	UIA_ValueValuePropertyId                        = 30045
	UIA_RangeValueValuePropertyId                   = 30047
	UIA_ExpandCollapseExpandCollapseStatePropertyId = 30070
	UIA_SelectionItemIsSelectedPropertyId           = 30079
	UIA_ToggleToggleStatePropertyId                 = 30086
	UIA_LiveSettingPropertyId                       = 30135
	UIA_HeadingLevelPropertyId                      = 30173
	UIA_IsDialogPropertyId                          = 30174
	UIA_IsRequiredForFormPropertyId                 = 30025
	UIA_ItemStatusPropertyId                        = 30026
	UIA_AriaPropertiesPropertyId                    = 30102
	UIA_IsDataValidForFormPropertyId                = 30103
	UIA_PositionInSetPropertyId                     = 30152
	UIA_SizeOfSetPropertyId                         = 30153
	UIA_LevelPropertyId                             = 30154

	UIA_ButtonControlTypeId      = 50000
	UIA_CheckBoxControlTypeId    = 50002
	UIA_EditControlTypeId        = 50004
	UIA_HyperlinkControlTypeId   = 50005
	UIA_ImageControlTypeId       = 50006
	UIA_ListItemControlTypeId    = 50007
	UIA_ListControlTypeId        = 50008
	UIA_ProgressBarControlTypeId = 50012
	UIA_RadioButtonControlTypeId = 50013
	UIA_SliderControlTypeId      = 50015
	UIA_SpinnerControlTypeId     = 50016
	UIA_TabControlTypeId         = 50018
	UIA_TabItemControlTypeId     = 50019
	UIA_TextControlTypeId        = 50020
	UIA_CustomControlTypeId      = 50025
	UIA_GroupControlTypeId       = 50026
	UIA_WindowControlTypeId      = 50032
	UIA_MenuControlTypeId        = 50009
	UIA_MenuBarControlTypeId     = 50010
	UIA_MenuItemControlTypeId    = 50011
	UIA_SeparatorControlTypeId   = 50038
	UIA_ComboBoxControlTypeId    = 50003
	UIA_ToolBarControlTypeId     = 50021
	UIA_ToolTipControlTypeId     = 50022
	UIA_DataGridControlTypeId    = 50028
	UIA_DataItemControlTypeId    = 50029
	UIA_HeaderItemControlTypeId  = 50035
	UIA_TreeControlTypeId        = 50023
	UIA_TreeItemControlTypeId    = 50024
)

// uiaFrameworkID is the FrameworkId and ClassName of every node.
const uiaFrameworkID = "gowaku"

// controlTypes maps the roles to UIA control types (ADR 0042 §2). RoleNone is
// decided by controlType.
var controlTypes = [...]int32{
	semantics.RoleButton:     UIA_ButtonControlTypeId,
	semantics.RoleCheckbox:   UIA_CheckBoxControlTypeId,
	semantics.RoleImage:      UIA_ImageControlTypeId,
	semantics.RoleSlider:     UIA_SliderControlTypeId,
	semantics.RoleText:       UIA_TextControlTypeId,
	semantics.RoleTextField:  UIA_EditControlTypeId,
	semantics.RoleProgress:   UIA_ProgressBarControlTypeId,
	semantics.RoleSwitch:     UIA_ButtonControlTypeId, // with Toggle, as the Windows 11 toggle
	semantics.RoleRadio:      UIA_RadioButtonControlTypeId,
	semantics.RoleRadioGroup: UIA_GroupControlTypeId,
	semantics.RoleTab:        UIA_TabItemControlTypeId,
	semantics.RoleTabList:    UIA_TabControlTypeId,
	semantics.RoleList:       UIA_ListControlTypeId,
	semantics.RoleListItem:   UIA_ListItemControlTypeId,
	semantics.RoleDialog:     UIA_WindowControlTypeId, // with IsDialog
	semantics.RoleGroup:      UIA_GroupControlTypeId,
	semantics.RoleHeading:    UIA_TextControlTypeId, // with HeadingLevel
	semantics.RoleLink:       UIA_HyperlinkControlTypeId,
	semantics.RoleStepper:    UIA_SpinnerControlTypeId,

	semantics.RoleMenuBar:          UIA_MenuBarControlTypeId,
	semantics.RoleMenu:             UIA_MenuControlTypeId,
	semantics.RoleMenuItem:         UIA_MenuItemControlTypeId,
	semantics.RoleMenuItemCheckbox: UIA_MenuItemControlTypeId, // with Toggle
	semantics.RoleSeparator:        UIA_SeparatorControlTypeId,
	semantics.RoleToolbar:          UIA_ToolBarControlTypeId,
	semantics.RoleTooltip:          UIA_ToolTipControlTypeId,
	semantics.RoleComboBox:         UIA_ComboBoxControlTypeId, // with ExpandCollapse and Value
	semantics.RoleGrid:             UIA_DataGridControlTypeId,
	semantics.RoleRow:              UIA_DataItemControlTypeId, // as Chromium maps rows
	semantics.RoleGridCell:         UIA_DataItemControlTypeId,
	semantics.RoleColumnHeader:     UIA_HeaderItemControlTypeId,
	semantics.RoleTree:             UIA_TreeControlTypeId,
	semantics.RoleTreeItem:         UIA_TreeItemControlTypeId, // with ExpandCollapse and SelectionItem
}

// controlType returns the UIA control type of n. A node without a role is a
// Group when it holds others, a Text when it only has a label, and Custom
// otherwise.
func controlType(n *semantics.Node) int32 {
	if n.Role != semantics.RoleNone && int(n.Role) < len(controlTypes) {
		return controlTypes[n.Role]
	}
	switch {
	case len(n.Children) > 0:
		return UIA_GroupControlTypeId
	case n.Label != "":
		return UIA_TextControlTypeId
	}
	return UIA_CustomControlTypeId
}

// patternSet is a set of the control patterns a node supports.
type patternSet uint16

const (
	patternInvoke patternSet = 1 << iota
	patternToggle
	patternValue
	patternRangeValue
	patternSelectionItem
	patternExpandCollapse
	patternScrollItem
	patternText
	patternSelection
	patternGrid
	patternGridItem
	patternTable
	patternTableItem
)

// patternOf returns the pattern of a UIA pattern ID, or zero. Text2 is the
// Text pattern.
func patternOf(id int32) patternSet {
	switch id {
	case UIA_InvokePatternId:
		return patternInvoke
	case UIA_TogglePatternId:
		return patternToggle
	case UIA_ValuePatternId:
		return patternValue
	case UIA_RangeValuePatternId:
		return patternRangeValue
	case UIA_SelectionItemPatternId:
		return patternSelectionItem
	case UIA_ExpandCollapsePatternId:
		return patternExpandCollapse
	case UIA_ScrollItemPatternId:
		return patternScrollItem
	case UIA_TextPatternId, UIA_TextPattern2Id:
		return patternText
	case UIA_SelectionPatternId:
		return patternSelection
	case UIA_GridPatternId:
		return patternGrid
	case UIA_GridItemPatternId:
		return patternGridItem
	case UIA_TablePatternId:
		return patternTable
	case UIA_TableItemPatternId:
		return patternTableItem
	}
	return 0
}

func isToggle(n *semantics.Node) bool {
	return n.Role == semantics.RoleCheckbox || n.Role == semantics.RoleSwitch || n.Role == semantics.RoleMenuItemCheckbox
}

// isSelectionItem reports whether n is one of the choices of a selection
// container. A row is one when it can be tapped, as the rows of a DataTable
// that select.
func isSelectionItem(n *semantics.Node) bool {
	return n.Role == semantics.RoleRadio || n.Role == semantics.RoleTab || n.Role == semantics.RoleListItem ||
		n.Role == semantics.RoleGridCell || n.Role == semantics.RoleTreeItem ||
		n.Role == semantics.RoleRow && n.Actions&semantics.ActionTap != 0
}

// isSelectionContainer reports whether n holds choices that can be selected:
// a group, list, grid or tree, or any node that says that many of its items
// can be selected together.
func isSelectionContainer(n *semantics.Node) bool {
	switch n.Role {
	case semantics.RoleRadioGroup, semantics.RoleTabList, semantics.RoleList, semantics.RoleGrid, semantics.RoleTree:
		return true
	}
	return n.State&semantics.Multiselectable != 0
}

// isGridItem reports whether n is a cell of a grid, the title of a column
// included.
func isGridItem(n *semantics.Node) bool {
	return n.Role == semantics.RoleGridCell || n.Role == semantics.RoleColumnHeader
}

func hasRange(n *semantics.Node) bool { return n.Range.Max > n.Range.Min }

// patterns returns the control patterns of n (ADR 0042 §4).
func patterns(n *semantics.Node) patternSet {
	var p patternSet
	toggle, selection, ranged := isToggle(n), isSelectionItem(n), hasRange(n)
	if n.Actions&semantics.ActionTap != 0 && !toggle && n.Role != semantics.RoleRadio && n.Role != semantics.RoleTab {
		p |= patternInvoke
	}
	if toggle {
		p |= patternToggle
	}
	if n.Role == semantics.RoleTextField || (n.Value != "" && !ranged && !toggle && !selection) {
		p |= patternValue
	}
	if ranged {
		p |= patternRangeValue
	}
	if selection {
		p |= patternSelectionItem
	}
	if n.State&(semantics.Expanded|semantics.Collapsed) != 0 {
		p |= patternExpandCollapse
	}
	if n.Actions&semantics.ActionScrollIntoView != 0 {
		p |= patternScrollItem
	}
	if n.Role == semantics.RoleTextField && n.Text != nil {
		p |= patternText
	}
	if isSelectionContainer(n) {
		p |= patternSelection
	}
	if n.Role == semantics.RoleGrid {
		p |= patternGrid | patternTable
	}
	if isGridItem(n) {
		p |= patternGridItem
	}
	if n.Role == semantics.RoleGridCell {
		p |= patternTableItem
	}
	return p
}

// nodeName returns the Name of n: its label, or for plain text without one,
// its value.
func nodeName(n *semantics.Node) string {
	if n.Label == "" && (n.Role == semantics.RoleText || n.Role == semantics.RoleHeading) {
		return n.Value
	}
	return n.Label
}

// nodeValue returns the Value of the Value pattern.
func nodeValue(n *semantics.Node) string {
	if n.Text != nil {
		return n.Text.Text
	}
	return n.Value
}

func enabled(n *semantics.Node) bool { return n.State&semantics.Disabled == 0 }

// valueReadOnly is IsReadOnly of the Value pattern.
func valueReadOnly(n *semantics.Node) bool {
	return n.State&(semantics.ReadOnly|semantics.Disabled) != 0 || n.Actions&semantics.ActionSetValue == 0
}

// rangeReadOnly is IsReadOnly of the RangeValue pattern.
func rangeReadOnly(n *semantics.Node) bool {
	const change = semantics.ActionIncrease | semantics.ActionDecrease | semantics.ActionSetValue
	return n.State&(semantics.ReadOnly|semantics.Disabled) != 0 || n.Actions&change == 0
}

// smallChange is the step of a range, or a hundredth of it without one.
func smallChange(r semantics.Range) float64 {
	if r.Step > 0 {
		return r.Step
	}
	return (r.Max - r.Min) / 100
}

// largeChange is a tenth of a range, and never less than its step.
func largeChange(r semantics.Range) float64 {
	return max(smallChange(r), (r.Max-r.Min)/10)
}

func toggleState(n *semantics.Node) int32 {
	switch {
	case n.State&semantics.Mixed != 0:
		return ToggleState_Indeterminate
	case n.State&semantics.Checked != 0:
		return ToggleState_On
	}
	return ToggleState_Off
}

func expandCollapseState(n *semantics.Node) int32 {
	switch {
	case n.State&semantics.Expanded != 0:
		return ExpandCollapseState_Expanded
	case n.State&semantics.Collapsed != 0:
		return ExpandCollapseState_Collapsed
	}
	return ExpandCollapseState_LeafNode
}

// headingLevel returns the HeadingLevel of n.
func headingLevel(n *semantics.Node) int32 {
	if n.Role != semantics.RoleHeading {
		return HeadingLevel_None
	}
	return HeadingLevel1 + int32(min(max(n.Level, 1), HeadingLevel9-HeadingLevel1+1)) - 1
}

// runtimeID returns the part of the runtime ID of a node after
// UiaAppendRuntimeId: the low and high halves of its ID.
func runtimeID(id semantics.ID) [3]int32 {
	return [3]int32{UiaAppendRuntimeId, int32(uint32(id)), int32(uint32(id >> 32))}
}

// propKind is the type of a property value.
type propKind uint8

const (
	propEmpty propKind = iota
	propI4
	propBool
	propString
)

// propValue is the value of a UIA property, before it becomes a VARIANT.
type propValue struct {
	kind propKind
	i    int32
	b    bool
	s    string
}

func propInt(i int32) propValue { return propValue{kind: propI4, i: i} }
func propFlag(b bool) propValue { return propValue{kind: propBool, b: b} }

func propText(s string) propValue {
	if s == "" {
		return propValue{}
	}
	return propValue{kind: propString, s: s}
}

// nodeProperty returns property prop of n, a node other than the root, which
// has the keyboard focus if focused. Properties gowaku does not know are
// empty, which leaves them to UI Automation's defaults.
func nodeProperty(n *semantics.Node, prop int32, focused bool) propValue {
	switch prop {
	case UIA_ControlTypePropertyId:
		return propInt(controlType(n))
	case UIA_NamePropertyId:
		return propText(nodeName(n))
	case UIA_HelpTextPropertyId:
		return propText(n.Hint)
	case UIA_AcceleratorKeyPropertyId:
		return propText(n.Shortcut)
	case UIA_IsEnabledPropertyId:
		return propFlag(enabled(n))
	case UIA_IsKeyboardFocusablePropertyId:
		return propFlag(n.State&semantics.Focusable != 0)
	case UIA_HasKeyboardFocusPropertyId:
		return propFlag(focused)
	case UIA_LiveSettingPropertyId:
		return propInt(int32(n.Live))
	case UIA_IsControlElementPropertyId, UIA_IsContentElementPropertyId:
		return propFlag(true)
	case UIA_ClassNamePropertyId, UIA_FrameworkIdPropertyId:
		return propText(uiaFrameworkID)
	case UIA_AutomationIdPropertyId:
		return propText(strconv.FormatUint(uint64(n.ID), 10))
	case UIA_IsDialogPropertyId:
		return propFlag(n.Role == semantics.RoleDialog)
	case UIA_HeadingLevelPropertyId:
		return propInt(headingLevel(n))
	case UIA_IsPasswordPropertyId:
		return propFlag(n.State&semantics.Obscured != 0)
	case UIA_IsRequiredForFormPropertyId:
		return propFlag(n.State&semantics.Required != 0)
	case UIA_IsDataValidForFormPropertyId:
		return propFlag(n.State&semantics.Invalid == 0)
	case UIA_ItemStatusPropertyId:
		// As Chromium does for aria-sort, which Narrator reads.
		return propText(sortStatus(n))
	case UIA_AriaPropertiesPropertyId:
		if s := sortStatus(n); s != "" {
			return propText("sort=" + s)
		}
	case UIA_PositionInSetPropertyId:
		return positive(n.PosInSet)
	case UIA_SizeOfSetPropertyId:
		return positive(n.SetSize)
	case UIA_LevelPropertyId:
		// A heading says its level with HeadingLevel instead.
		if n.Role == semantics.RoleTreeItem {
			return positive(n.Level)
		}
	}
	return propValue{}
}

// sortStatus returns the order the column of a header is sorted in, as
// aria-sort spells it, or "" when it is not sorted.
func sortStatus(n *semantics.Node) string {
	if n.Role != semantics.RoleColumnHeader || n.Sort == semantics.SortNone {
		return ""
	}
	return n.Sort.String()
}

// positive returns v as an integer property, or empty when it is not set, which
// leaves the property to UI Automation.
func positive(v int) propValue {
	if v <= 0 {
		return propValue{}
	}
	return propInt(int32(v))
}
