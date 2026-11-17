package dbus

import "strings"

// The introspection data of the interfaces gowaku implements, from the XML of
// at-spi2-core, cut to what gowaku answers.

const introspectHeader = `<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
"http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
`

const introspectStandard = `<interface name="org.freedesktop.DBus.Introspectable">
<method name="Introspect"><arg name="xml_data" type="s" direction="out"/></method>
</interface>
<interface name="org.freedesktop.DBus.Properties">
<method name="Get"><arg name="interface_name" type="s" direction="in"/><arg name="property_name" type="s" direction="in"/><arg name="value" type="v" direction="out"/></method>
<method name="GetAll"><arg name="interface_name" type="s" direction="in"/><arg name="props" type="a{sv}" direction="out"/></method>
<method name="Set"><arg name="interface_name" type="s" direction="in"/><arg name="property_name" type="s" direction="in"/><arg name="value" type="v" direction="in"/></method>
<signal name="PropertiesChanged"><arg name="interface_name" type="s"/><arg name="changed_properties" type="a{sv}"/><arg name="invalidated_properties" type="as"/></signal>
</interface>
`

// introspectInterfaces holds the XML of each AT-SPI interface, in the order of
// ifaceNames.
var introspectInterfaces = [...]string{
	`<interface name="org.a11y.atspi.Accessible">
<property name="Name" type="s" access="read"/>
<property name="Description" type="s" access="read"/>
<property name="Parent" type="(so)" access="read"/>
<property name="ChildCount" type="i" access="read"/>
<property name="Locale" type="s" access="read"/>
<property name="AccessibleId" type="s" access="read"/>
<property name="HelpText" type="s" access="read"/>
<method name="GetChildAtIndex"><arg direction="in" name="index" type="i"/><arg direction="out" type="(so)"/></method>
<method name="GetChildren"><arg direction="out" type="a(so)"/></method>
<method name="GetIndexInParent"><arg direction="out" type="i"/></method>
<method name="GetRelationSet"><arg direction="out" type="a(ua(so))"/></method>
<method name="GetRole"><arg direction="out" type="u"/></method>
<method name="GetRoleName"><arg direction="out" type="s"/></method>
<method name="GetLocalizedRoleName"><arg direction="out" type="s"/></method>
<method name="GetState"><arg direction="out" type="au"/></method>
<method name="GetAttributes"><arg direction="out" type="a{ss}"/></method>
<method name="GetApplication"><arg direction="out" type="(so)"/></method>
<method name="GetInterfaces"><arg direction="out" type="as"/></method>
</interface>
`,
	`<interface name="org.a11y.atspi.Application">
<property name="ToolkitName" type="s" access="read"/>
<property name="Version" type="s" access="read"/>
<property name="AtspiVersion" type="s" access="read"/>
<property name="Id" type="i" access="readwrite"/>
<method name="GetLocale"><arg direction="in" name="lctype" type="u"/><arg direction="out" type="s"/></method>
<method name="RegisterEventListener"><arg direction="in" name="event" type="s"/></method>
<method name="DeregisterEventListener"><arg direction="in" name="event" type="s"/></method>
<method name="GetApplicationBusAddress"><arg direction="out" type="s"/></method>
</interface>
`,
	`<interface name="org.a11y.atspi.Component">
<method name="Contains"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="coord_type" type="u"/><arg direction="out" type="b"/></method>
<method name="GetAccessibleAtPoint"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="coord_type" type="u"/><arg direction="out" type="(so)"/></method>
<method name="GetExtents"><arg direction="in" name="coord_type" type="u"/><arg direction="out" type="(iiii)"/></method>
<method name="GetPosition"><arg direction="in" name="coord_type" type="u"/><arg direction="out" name="x" type="i"/><arg direction="out" name="y" type="i"/></method>
<method name="GetSize"><arg direction="out" name="width" type="i"/><arg direction="out" name="height" type="i"/></method>
<method name="GetLayer"><arg direction="out" type="u"/></method>
<method name="GetMDIZOrder"><arg direction="out" type="n"/></method>
<method name="GrabFocus"><arg direction="out" type="b"/></method>
<method name="GetAlpha"><arg direction="out" type="d"/></method>
<method name="SetExtents"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="width" type="i"/><arg direction="in" name="height" type="i"/><arg direction="in" name="coord_type" type="u"/><arg direction="out" type="b"/></method>
<method name="SetPosition"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="coord_type" type="u"/><arg direction="out" type="b"/></method>
<method name="SetSize"><arg direction="in" name="width" type="i"/><arg direction="in" name="height" type="i"/><arg direction="out" type="b"/></method>
<method name="ScrollTo"><arg direction="in" name="type" type="u"/><arg direction="out" type="b"/></method>
<method name="ScrollToPoint"><arg direction="in" name="type" type="u"/><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="out" type="b"/></method>
</interface>
`,
	`<interface name="org.a11y.atspi.Action">
<property name="NActions" type="i" access="read"/>
<method name="GetDescription"><arg direction="in" name="index" type="i"/><arg direction="out" type="s"/></method>
<method name="GetName"><arg direction="in" name="index" type="i"/><arg direction="out" type="s"/></method>
<method name="GetLocalizedName"><arg direction="in" name="index" type="i"/><arg direction="out" type="s"/></method>
<method name="GetKeyBinding"><arg direction="in" name="index" type="i"/><arg direction="out" type="s"/></method>
<method name="GetActions"><arg direction="out" type="a(sss)"/></method>
<method name="DoAction"><arg direction="in" name="index" type="i"/><arg direction="out" type="b"/></method>
</interface>
`,
	`<interface name="org.a11y.atspi.Value">
<property name="MinimumValue" type="d" access="read"/>
<property name="MaximumValue" type="d" access="read"/>
<property name="MinimumIncrement" type="d" access="read"/>
<property name="CurrentValue" type="d" access="readwrite"/>
<property name="Text" type="s" access="read"/>
</interface>
`,
	`<interface name="org.a11y.atspi.Text">
<property name="CharacterCount" type="i" access="read"/>
<property name="CaretOffset" type="i" access="read"/>
<method name="GetStringAtOffset"><arg direction="in" name="offset" type="i"/><arg direction="in" name="granularity" type="u"/><arg direction="out" type="s"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetText"><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="out" type="s"/></method>
<method name="SetCaretOffset"><arg direction="in" name="offset" type="i"/><arg direction="out" type="b"/></method>
<method name="GetTextBeforeOffset"><arg direction="in" name="offset" type="i"/><arg direction="in" name="type" type="u"/><arg direction="out" type="s"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetTextAtOffset"><arg direction="in" name="offset" type="i"/><arg direction="in" name="type" type="u"/><arg direction="out" type="s"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetTextAfterOffset"><arg direction="in" name="offset" type="i"/><arg direction="in" name="type" type="u"/><arg direction="out" type="s"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetCharacterAtOffset"><arg direction="in" name="offset" type="i"/><arg direction="out" type="i"/></method>
<method name="GetAttributeValue"><arg direction="in" name="offset" type="i"/><arg direction="in" name="attributeName" type="s"/><arg direction="out" type="s"/></method>
<method name="GetAttributes"><arg direction="in" name="offset" type="i"/><arg direction="out" type="a{ss}"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetDefaultAttributes"><arg direction="out" type="a{ss}"/></method>
<method name="GetCharacterExtents"><arg direction="in" name="offset" type="i"/><arg direction="in" name="coordType" type="u"/><arg direction="out" name="x" type="i"/><arg direction="out" name="y" type="i"/><arg direction="out" name="width" type="i"/><arg direction="out" name="height" type="i"/></method>
<method name="GetOffsetAtPoint"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="coordType" type="u"/><arg direction="out" type="i"/></method>
<method name="GetNSelections"><arg direction="out" type="i"/></method>
<method name="GetSelection"><arg direction="in" name="selectionNum" type="i"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="AddSelection"><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="out" type="b"/></method>
<method name="RemoveSelection"><arg direction="in" name="selectionNum" type="i"/><arg direction="out" type="b"/></method>
<method name="SetSelection"><arg direction="in" name="selectionNum" type="i"/><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="out" type="b"/></method>
<method name="GetRangeExtents"><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="in" name="coordType" type="u"/><arg direction="out" name="x" type="i"/><arg direction="out" name="y" type="i"/><arg direction="out" name="width" type="i"/><arg direction="out" name="height" type="i"/></method>
<method name="GetBoundedRanges"><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="in" name="width" type="i"/><arg direction="in" name="height" type="i"/><arg direction="in" name="coordType" type="u"/><arg direction="in" name="xClipType" type="u"/><arg direction="in" name="yClipType" type="u"/><arg direction="out" type="a(iisv)"/></method>
<method name="GetAttributeRun"><arg direction="in" name="offset" type="i"/><arg direction="in" name="includeDefaults" type="b"/><arg direction="out" type="a{ss}"/><arg direction="out" name="startOffset" type="i"/><arg direction="out" name="endOffset" type="i"/></method>
<method name="GetDefaultAttributeSet"><arg direction="out" type="a{ss}"/></method>
<method name="ScrollSubstringTo"><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="in" name="type" type="u"/><arg direction="out" type="b"/></method>
<method name="ScrollSubstringToPoint"><arg direction="in" name="startOffset" type="i"/><arg direction="in" name="endOffset" type="i"/><arg direction="in" name="type" type="u"/><arg direction="in" name="x" type="i"/><arg direction="in" name="y" type="i"/><arg direction="out" type="b"/></method>
</interface>
`,
	`<interface name="org.a11y.atspi.EditableText">
<method name="SetTextContents"><arg direction="in" name="newContents" type="s"/><arg direction="out" type="b"/></method>
<method name="InsertText"><arg direction="in" name="position" type="i"/><arg direction="in" name="text" type="s"/><arg direction="in" name="length" type="i"/><arg direction="out" type="b"/></method>
<method name="CopyText"><arg direction="in" name="startPos" type="i"/><arg direction="in" name="endPos" type="i"/></method>
<method name="CutText"><arg direction="in" name="startPos" type="i"/><arg direction="in" name="endPos" type="i"/><arg direction="out" type="b"/></method>
<method name="DeleteText"><arg direction="in" name="startPos" type="i"/><arg direction="in" name="endPos" type="i"/><arg direction="out" type="b"/></method>
<method name="PasteText"><arg direction="in" name="position" type="i"/><arg direction="out" type="b"/></method>
</interface>
`,
}

// introspect returns the introspection data of an object with the interfaces
// s.
func introspect(s ifaceSet) string {
	var b strings.Builder
	b.WriteString(introspectHeader)
	b.WriteString(introspectStandard)
	for i, x := range introspectInterfaces {
		if s&(1<<i) != 0 {
			b.WriteString(x)
		}
	}
	b.WriteString("</node>\n")
	return b.String()
}
