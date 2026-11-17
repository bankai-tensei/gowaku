package dbus

import (
	"strconv"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file holds the methods of the AT-SPI interfaces (ADR 0043 §4).

// AtspiComponentLayer values.
const (
	atspiLayerWidget = 3
	atspiLayerWindow = 7
)

// relation is one entry of GetRelationSet.
type relation struct {
	Type    uint32
	Targets []ref
}

// actionInfo is one entry of GetActions.
type actionInfo struct {
	Name, Description, KeyBinding string
}

// method returns member of interface s of o.
func (o *object) method(s ifaceSet, member string) (godbus.Method, bool) {
	switch s {
	case ifaceAccessible:
		return o.accessibleMethod(member)
	case ifaceApplication:
		return o.applicationMethod(member)
	case ifaceComponent:
		return o.componentMethod(member)
	case ifaceAction:
		return o.actionMethod(member)
	case ifaceText:
		return o.textMethod(member)
	case ifaceEditableText:
		return o.editableTextMethod(member)
	}
	return nil, false
}

// post asks the event loop to do action ev to the node.
func (o *object) post(ev platform.Event) {
	ev.Window, ev.Node = o.w.id, o.id
	o.a.post(ev)
}

// Accessible

func (o *object) name() string {
	switch o.kind {
	case objRoot:
		return o.a.appName
	case objFrame:
		return o.title
	}
	return atspiName(o.n)
}

func (o *object) parent() ref {
	switch o.kind {
	case objRoot:
		if o.desktop.Path == "" {
			return o.null()
		}
		return o.desktop
	case objFrame:
		return o.ref(rootPath)
	}
	return o.ref(o.w.path(o.t.parents[o.id]))
}

// children returns the paths of the children of o.
func (o *object) children() []string {
	if o.kind == objRoot {
		return o.frames
	}
	nodes := o.t.children(o.id)
	out := make([]string, len(nodes))
	for i, c := range nodes {
		out[i] = o.w.path(c.ID)
	}
	return out
}

func (o *object) role() uint32 {
	switch o.kind {
	case objRoot:
		return atspiRoleApplication
	case objFrame:
		return atspiRoleFrame
	}
	return atspiRole(o.n)
}

func (o *object) states() stateSet {
	switch o.kind {
	case objRoot:
		return stateSet{}
	case objFrame:
		return frameStates(o.active)
	}
	return nodeStates(o.n, o.t.focus == o.id)
}

func (o *object) accessibleProperty(name string) any {
	switch name {
	case "Name":
		return o.name()
	case "Description":
		if o.kind == objNode {
			return o.n.Hint
		}
		return ""
	case "Parent":
		return o.parent()
	case "ChildCount":
		return int32(len(o.children()))
	case "Locale":
		return o.a.locale
	case "AccessibleId":
		if o.kind == objNode {
			return strconv.FormatUint(uint64(o.id), 10)
		}
		return ""
	case "HelpText":
		return ""
	}
	return nil
}

func (o *object) accessibleMethod(member string) (godbus.Method, bool) {
	var m method
	switch member {
	case "GetChildAtIndex":
		m = method{"i", func(args []any) ([]any, error) {
			children := o.children()
			if i := int(argI(args, 0)); i >= 0 && i < len(children) {
				return reply(o.ref(children[i]))
			}
			return reply(o.null())
		}}
	case "GetChildren":
		m = method{"", func([]any) ([]any, error) {
			children := o.children()
			out := make([]ref, len(children))
			for i, c := range children {
				out[i] = o.ref(c)
			}
			return reply(out)
		}}
	case "GetIndexInParent":
		m = method{"", func([]any) ([]any, error) {
			switch o.kind {
			case objRoot:
				return reply(int32(-1))
			case objFrame:
				return reply(int32(o.index))
			}
			return reply(int32(o.t.indexInParent(o.id)))
		}}
	case "GetRelationSet":
		m = method{"", func([]any) ([]any, error) { return reply([]relation{}) }}
	case "GetRole":
		m = method{"", func([]any) ([]any, error) { return reply(o.role()) }}
	case "GetRoleName", "GetLocalizedRoleName":
		m = method{"", func([]any) ([]any, error) { return reply(atspiRoleName(o.role())) }}
	case "GetState":
		m = method{"", func([]any) ([]any, error) {
			s := o.states()
			return reply([]uint32{s[0], s[1]})
		}}
	case "GetAttributes":
		m = method{"", func([]any) ([]any, error) {
			if o.kind == objNode {
				return reply(nodeAttributes(o.n))
			}
			return reply(map[string]string{"toolkit": toolkitName})
		}}
	case "GetApplication":
		m = method{"", func([]any) ([]any, error) { return reply(o.ref(rootPath)) }}
	case "GetInterfaces":
		m = method{"", func([]any) ([]any, error) { return reply(o.ifaces().names()) }}
	default:
		return nil, false
	}
	return m, true
}

// Application

func (o *object) applicationMethod(member string) (godbus.Method, bool) {
	var m method
	switch member {
	case "GetLocale":
		m = method{"u", func([]any) ([]any, error) { return reply(o.a.locale) }}
	case "RegisterEventListener", "DeregisterEventListener":
		m = method{"s", func([]any) ([]any, error) { return reply() }}
	case "GetApplicationBusAddress":
		// No peer-to-peer connection: everything goes over the bus.
		m = method{"", func([]any) ([]any, error) { return reply("") }}
	default:
		return nil, false
	}
	return m, true
}

// Component

// rect returns the box of o in logical pixels of the window, and that of its
// parent.
func (o *object) rect() (r, parent geom.Rect) {
	if o.kind == objFrame {
		// The semantics root may not know its size; the window does.
		r = o.geo.rect()
		if r.W <= 0 && o.n != nil {
			r = geom.Rect{W: o.n.Rect.W, H: o.n.Rect.H}
		}
		return r, geom.Rect{}
	}
	return o.n.Rect, o.t.rect(o.t.parents[o.id])
}

// extents returns the box of o in the coordinates of coord. The parent of a
// frame is the screen.
func (o *object) extents(coord uint32) extents {
	r, parent := o.rect()
	if o.kind == objFrame && coord == atspiCoordParent {
		coord = atspiCoordScreen
	}
	return o.geo.toAtspi(r, parent, coord)
}

// point turns a point in the coordinates of coord into logical pixels of the
// window.
func (o *object) point(x, y int32, coord uint32) geom.Offset {
	_, parent := o.rect()
	if o.kind == objFrame && coord == atspiCoordParent {
		coord = atspiCoordScreen
	}
	return o.geo.fromAtspi(x, y, parent, coord)
}

func (o *object) componentMethod(member string) (godbus.Method, bool) {
	var m method
	no := func([]any) ([]any, error) { return reply(false) }
	switch member {
	case "Contains":
		m = method{"iiu", func(args []any) ([]any, error) {
			r, _ := o.rect()
			return reply(r.Contains(o.point(argI(args, 0), argI(args, 1), argU(args, 2))))
		}}
	case "GetAccessibleAtPoint":
		m = method{"iiu", func(args []any) ([]any, error) {
			if id, ok := o.t.hitTest(o.id, o.point(argI(args, 0), argI(args, 1), argU(args, 2))); ok {
				return reply(o.ref(o.w.path(id)))
			}
			return reply(o.null())
		}}
	case "GetExtents":
		m = method{"u", func(args []any) ([]any, error) { return reply(o.extents(argU(args, 0))) }}
	case "GetPosition":
		m = method{"u", func(args []any) ([]any, error) {
			e := o.extents(argU(args, 0))
			return reply(e.X, e.Y)
		}}
	case "GetSize":
		m = method{"", func([]any) ([]any, error) {
			e := o.extents(atspiCoordWindow)
			return reply(e.W, e.H)
		}}
	case "GetLayer":
		m = method{"", func([]any) ([]any, error) {
			if o.kind == objFrame {
				return reply(uint32(atspiLayerWindow))
			}
			return reply(uint32(atspiLayerWidget))
		}}
	case "GetMDIZOrder":
		m = method{"", func([]any) ([]any, error) { return reply(int16(0)) }}
	case "GrabFocus":
		m = method{"", func([]any) ([]any, error) {
			if o.kind != objNode || !isEnabled(o.n) {
				return reply(false)
			}
			o.post(platform.Event{Action: semantics.ActionFocus})
			return reply(true)
		}}
	case "GetAlpha":
		m = method{"", func([]any) ([]any, error) { return reply(1.0) }}
	case "SetExtents":
		m = method{"iiiiu", no}
	case "SetPosition":
		m = method{"iiu", no}
	case "SetSize":
		m = method{"ii", no}
	case "ScrollTo":
		m = method{"u", func([]any) ([]any, error) {
			if o.kind != objNode || o.n.Actions&semantics.ActionScrollIntoView == 0 {
				return reply(false)
			}
			o.post(platform.Event{Action: semantics.ActionScrollIntoView})
			return reply(true)
		}}
	case "ScrollToPoint":
		m = method{"uii", no}
	default:
		return nil, false
	}
	return m, true
}

// Action

func (o *object) actionMethod(member string) (godbus.Method, bool) {
	actions := nodeActions(o.n)
	at := func(args []any) (atspiAction, bool) {
		i := int(argI(args, 0))
		if i < 0 || i >= len(actions) {
			return atspiAction{}, false
		}
		return actions[i], true
	}
	field := func(get func(atspiAction) string) method {
		return method{"i", func(args []any) ([]any, error) {
			a, _ := at(args)
			return reply(get(a))
		}}
	}
	var m method
	switch member {
	case "GetName", "GetLocalizedName":
		m = field(func(a atspiAction) string { return a.name })
	case "GetDescription":
		m = field(func(a atspiAction) string { return a.description })
	case "GetKeyBinding":
		m = field(func(atspiAction) string { return "" })
	case "GetActions":
		m = method{"", func([]any) ([]any, error) {
			out := make([]actionInfo, len(actions))
			for i, a := range actions {
				out[i] = actionInfo{a.name, a.description, ""}
			}
			return reply(out)
		}}
	case "DoAction":
		m = method{"i", func(args []any) ([]any, error) {
			a, ok := at(args)
			if !ok || !isEnabled(o.n) {
				return reply(false)
			}
			o.post(platform.Event{Action: a.action})
			return reply(true)
		}}
	default:
		return nil, false
	}
	return m, true
}

// Value

func (o *object) valueProperty(name string) any {
	r := o.n.Range
	switch name {
	case "MinimumValue":
		return r.Min
	case "MaximumValue":
		return r.Max
	case "MinimumIncrement":
		return minimumIncrement(r)
	case "CurrentValue":
		return r.Value
	case "Text":
		return atspiValueText(o.n)
	}
	return nil
}

// setValue asks for the value f of a range. A node that cannot take a value
// but can step takes one step toward it.
func (o *object) setValue(f float64) bool {
	n := o.n
	if !isEnabled(n) || n.State&semantics.ReadOnly != 0 {
		return false
	}
	switch {
	case n.Actions&semantics.ActionSetValue != 0:
		o.post(platform.Event{Action: semantics.ActionSetValue, Number: f})
	case f > n.Range.Value && n.Actions&semantics.ActionIncrease != 0:
		o.post(platform.Event{Action: semantics.ActionIncrease})
	case f < n.Range.Value && n.Actions&semantics.ActionDecrease != 0:
		o.post(platform.Event{Action: semantics.ActionDecrease})
	case f == n.Range.Value:
	default:
		return false
	}
	return true
}

// Text

// selectText asks for the selection from start to end, in characters.
func (o *object) selectText(t atspiText, start, end int32) bool {
	if o.n.Actions&semantics.ActionSetSelection == 0 || !isEnabled(o.n) {
		return false
	}
	o.post(platform.Event{
		Action:         semantics.ActionSetSelection,
		SelectionStart: t.toU16(int(start)),
		SelectionEnd:   t.toU16(int(end)),
	})
	return true
}

func (o *object) textMethod(member string) (godbus.Method, bool) {
	t := newAtspiText(o.n.Text)
	around := func(which int) method {
		return method{"iu", func(args []any) ([]any, error) {
			s, start, end := t.textAround(int(argI(args, 0)), argU(args, 1), which)
			return reply(s, int32(start), int32(end))
		}}
	}
	box := func(r geom.Rect, ok bool, coord uint32) ([]any, error) {
		if !ok {
			r = o.n.Rect
		}
		_, parent := o.rect()
		e := o.geo.toAtspi(r, parent, coord)
		return reply(e.X, e.Y, e.W, e.H)
	}
	noAttributes := func([]any) ([]any, error) {
		return reply(map[string]string{}, int32(0), int32(t.count()))
	}
	var m method
	switch member {
	case "GetStringAtOffset":
		m = method{"iu", func(args []any) ([]any, error) {
			s, start, end := t.stringAt(int(argI(args, 0)), argU(args, 1))
			return reply(s, int32(start), int32(end))
		}}
	case "GetText":
		m = method{"ii", func(args []any) ([]any, error) {
			return reply(t.slice(int(argI(args, 0)), int(argI(args, 1))))
		}}
	case "SetCaretOffset":
		m = method{"i", func(args []any) ([]any, error) {
			off := argI(args, 0)
			return reply(o.selectText(t, off, off))
		}}
	case "GetTextBeforeOffset":
		m = around(pieceBefore)
	case "GetTextAtOffset":
		m = around(pieceAtOff)
	case "GetTextAfterOffset":
		m = around(pieceAfter)
	case "GetCharacterAtOffset":
		m = method{"i", func(args []any) ([]any, error) {
			if i := int(argI(args, 0)); i >= 0 && i < t.count() {
				return reply(t.runes[i])
			}
			return reply(int32(0))
		}}
	case "GetAttributeValue":
		m = method{"is", func([]any) ([]any, error) { return reply("") }}
	case "GetAttributes":
		m = method{"i", noAttributes}
	case "GetAttributeRun":
		m = method{"ib", noAttributes}
	case "GetDefaultAttributes", "GetDefaultAttributeSet":
		m = method{"", func([]any) ([]any, error) { return reply(map[string]string{}) }}
	case "GetCharacterExtents":
		m = method{"iu", func(args []any) ([]any, error) {
			off := int(argI(args, 0))
			r, ok := t.lineRect(t.line(off))
			return box(r, ok, argU(args, 1))
		}}
	case "GetRangeExtents":
		m = method{"iiu", func(args []any) ([]any, error) {
			r, ok := t.rangeRect(int(argI(args, 0)), int(argI(args, 1)))
			return box(r, ok, argU(args, 2))
		}}
	case "GetOffsetAtPoint":
		m = method{"iiu", func(args []any) ([]any, error) {
			p := o.point(argI(args, 0), argI(args, 1), argU(args, 2))
			return reply(int32(t.offsetAtPoint(p)))
		}}
	case "GetNSelections":
		m = method{"", func([]any) ([]any, error) {
			if _, _, ok := t.selection(); ok {
				return reply(int32(1))
			}
			return reply(int32(0))
		}}
	case "GetSelection":
		m = method{"i", func(args []any) ([]any, error) {
			start, end, ok := t.selection()
			if !ok || argI(args, 0) != 0 {
				return reply(int32(0), int32(0))
			}
			return reply(int32(start), int32(end))
		}}
	case "AddSelection":
		m = method{"ii", func(args []any) ([]any, error) {
			// A field has one selection, which can only be added while there
			// is none.
			if _, _, ok := t.selection(); ok {
				return reply(false)
			}
			return reply(o.selectText(t, argI(args, 0), argI(args, 1)))
		}}
	case "RemoveSelection":
		m = method{"i", func(args []any) ([]any, error) {
			if _, _, ok := t.selection(); !ok || argI(args, 0) != 0 {
				return reply(false)
			}
			caret := int32(t.caret())
			return reply(o.selectText(t, caret, caret))
		}}
	case "SetSelection":
		m = method{"iii", func(args []any) ([]any, error) {
			if argI(args, 0) != 0 {
				return reply(false)
			}
			return reply(o.selectText(t, argI(args, 1), argI(args, 2)))
		}}
	case "GetBoundedRanges":
		m = method{"iiiiuuu", func([]any) ([]any, error) {
			return reply([]struct {
				Start, End int32
				Text       string
				Value      godbus.Variant
			}{})
		}}
	case "ScrollSubstringTo":
		m = method{"iiu", func([]any) ([]any, error) { return reply(false) }}
	case "ScrollSubstringToPoint":
		m = method{"iiuii", func([]any) ([]any, error) { return reply(false) }}
	default:
		return nil, false
	}
	return m, true
}

// EditableText

func (o *object) editableTextMethod(member string) (godbus.Method, bool) {
	no := func([]any) ([]any, error) { return reply(false) }
	var m method
	switch member {
	case "SetTextContents":
		m = method{"s", func(args []any) ([]any, error) {
			if !isEditable(o.n) {
				return reply(false)
			}
			o.post(platform.Event{Action: semantics.ActionSetValue, Text: argS(args, 0)})
			return reply(true)
		}}
	case "InsertText":
		m = method{"isi", no}
	case "CopyText":
		m = method{"ii", func([]any) ([]any, error) { return reply() }}
	case "CutText", "DeleteText":
		m = method{"ii", no}
	case "PasteText":
		m = method{"i", no}
	default:
		return nil, false
	}
	return m, true
}
