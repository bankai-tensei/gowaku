package dbus

import (
	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/semantics"
)

// handler routes the calls on the accessibility bus to the objects of the
// application. It replaces the default handler of godbus, whose objects are
// fixed: here they come and go with every Update.
type handler struct{ a *A11y }

// objKind says what an object path stands for.
type objKind uint8

const (
	objGone objKind = iota
	objRoot
	objFrame
	objNode
)

// object is what one call is about, resolved from its path under the mutex.
// The tree it holds is never changed, so the call reads it without the lock.
type object struct {
	a    *A11y
	kind objKind
	path string
	// busName is the unique name of the application.
	busName string

	// For the root: the parent from the registry, and the frames.
	desktop ref
	frames  []string
	appID   int32

	// For frames and nodes.
	w      *A11yWindow
	t      *tree
	geo    WindowGeometry
	title  string
	active bool
	// index is the place of the frame among the windows.
	index int
	id    semantics.ID
	// n is the node, or the semantics root for a frame, which may be nil
	// before the first Update.
	n *semantics.Node
}

// LookupObject implements godbus.Handler. Every path is found: an unknown one
// answers every call with UnknownObject, as AT-SPI clients expect.
func (h handler) LookupObject(path godbus.ObjectPath) (godbus.ServerObject, bool) {
	return h.a.resolve(string(path)), true
}

// resolve finds the object of path.
func (a *A11y) resolve(path string) *object {
	a.mu.Lock()
	defer a.mu.Unlock()
	o := &object{a: a, path: path, busName: a.busName}
	if a.closed {
		return o
	}
	if path == rootPath {
		o.kind = objRoot
		o.desktop, o.appID = a.desktop, a.appID
		for _, w := range a.windows {
			o.frames = append(o.frames, w.path(0))
		}
		return o
	}
	number, id, ok := parseNodePath(path)
	if !ok {
		return o
	}
	for i, w := range a.windows {
		if w.number != number {
			continue
		}
		if !w.tree.has(id) {
			return o
		}
		o.w, o.t, o.geo, o.title, o.active, o.index, o.id = w, w.tree, w.geo, w.title, w.active, i, id
		o.n = w.tree.node(id)
		o.kind = objNode
		if id == 0 {
			o.kind = objFrame
		}
		return o
	}
	return o
}

// ifaces returns the AT-SPI interfaces of o.
func (o *object) ifaces() ifaceSet {
	switch o.kind {
	case objRoot:
		return ifaceAccessible | ifaceApplication
	case objFrame:
		return ifaceAccessible | ifaceComponent
	case objNode:
		return nodeIfaces(o.n)
	}
	return 0
}

// LookupInterface implements godbus.ServerObject.
func (o *object) LookupInterface(name string) (godbus.Interface, bool) {
	if o.kind == objGone {
		return goneInterface{o.path}, true
	}
	switch name {
	case introspectIface, propertiesIface, "":
		return boundInterface{o, name}, true
	}
	if ifaceOf(name)&o.ifaces() == 0 {
		return nil, false
	}
	return boundInterface{o, name}, true
}

// boundInterface is one interface of an object.
type boundInterface struct {
	o    *object
	name string
}

// LookupMethod implements godbus.Interface. Without an interface name, the
// first interface of the object with the method answers.
func (b boundInterface) LookupMethod(member string) (godbus.Method, bool) {
	o := b.o
	switch b.name {
	case introspectIface:
		if member == "Introspect" {
			return method{"", func([]any) ([]any, error) { return reply(introspect(o.ifaces())) }}, true
		}
		return nil, false
	case propertiesIface:
		return o.propertiesMethod(member)
	case "":
		for _, name := range append(o.ifaces().names(), propertiesIface, introspectIface) {
			if m, ok := (boundInterface{o, name}).LookupMethod(member); ok {
				return m, true
			}
		}
		return nil, false
	}
	return o.method(ifaceOf(b.name), member)
}

// goneInterface answers for an object that is not there.
type goneInterface struct{ path string }

// LookupMethod implements godbus.Interface.
func (g goneInterface) LookupMethod(string) (godbus.Method, bool) {
	return goneMethod(g), true
}

// goneMethod fails with UnknownObject, whatever its arguments.
type goneMethod struct{ path string }

// DecodeArguments implements godbus.ArgumentDecoder: the arguments do not
// matter.
func (goneMethod) DecodeArguments(*godbus.Conn, string, *godbus.Message, []any) ([]any, error) {
	return nil, nil
}

// Call implements godbus.Method.
func (g goneMethod) Call(...any) ([]any, error) {
	return nil, godbus.NewError(errUnknownObject, []any{"no object " + g.path})
}

// NumArguments implements godbus.Method.
func (goneMethod) NumArguments() int { return 0 }

// NumReturns implements godbus.Method.
func (goneMethod) NumReturns() int { return 0 }

// ArgumentValue implements godbus.Method.
func (goneMethod) ArgumentValue(int) any { return nil }

// ReturnValue implements godbus.Method.
func (goneMethod) ReturnValue(int) any { return nil }

// method is a method with the signature of its arguments. It takes the body
// of the call as godbus decoded it, after checking the signature, instead of
// the reflection of the default handler.
type method struct {
	in string
	fn func(args []any) ([]any, error)
}

// DecodeArguments implements godbus.ArgumentDecoder.
func (m method) DecodeArguments(_ *godbus.Conn, _ string, msg *godbus.Message, body []any) ([]any, error) {
	var sig string
	if v, ok := msg.Headers[godbus.FieldSignature]; ok {
		if s, ok := v.Value().(godbus.Signature); ok {
			sig = s.String()
		}
	}
	if sig != m.in {
		return nil, godbus.NewError(errInvalidArgs, []any{"want arguments (" + m.in + "), got (" + sig + ")"})
	}
	return body, nil
}

// Call implements godbus.Method.
func (m method) Call(args ...any) ([]any, error) { return m.fn(args) }

// NumArguments implements godbus.Method; DecodeArguments makes it unused.
func (method) NumArguments() int { return 0 }

// NumReturns implements godbus.Method.
func (method) NumReturns() int { return 0 }

// ArgumentValue implements godbus.Method.
func (method) ArgumentValue(int) any { return nil }

// ReturnValue implements godbus.Method.
func (method) ReturnValue(int) any { return nil }

func reply(values ...any) ([]any, error) { return values, nil }

// argument helpers: the signature has been checked, so the types hold.
func argI(args []any, i int) int32  { v, _ := args[i].(int32); return v }
func argU(args []any, i int) uint32 { v, _ := args[i].(uint32); return v }
func argS(args []any, i int) string { v, _ := args[i].(string); return v }

// ref returns the reference of a path of the application.
func (o *object) ref(path string) ref { return ref{o.busName, godbus.ObjectPath(path)} }

func (o *object) null() ref { return o.ref(nullPath) }

// propertiesMethod answers org.freedesktop.DBus.Properties.
func (o *object) propertiesMethod(member string) (godbus.Method, bool) {
	switch member {
	case "Get":
		return method{"ss", func(args []any) ([]any, error) {
			v, err := o.property(argS(args, 0), argS(args, 1))
			if err != nil {
				return nil, err
			}
			return reply(godbus.MakeVariant(v))
		}}, true
	case "GetAll":
		return method{"s", func(args []any) ([]any, error) {
			name := argS(args, 0)
			all := map[string]godbus.Variant{}
			for _, p := range propertyNames(ifaceOf(name) & o.ifaces()) {
				if v, err := o.property(name, p); err == nil {
					all[p] = godbus.MakeVariant(v)
				}
			}
			return reply(all)
		}}, true
	case "Set":
		return method{"ssv", func(args []any) ([]any, error) {
			v, _ := args[2].(godbus.Variant)
			return nil, o.setProperty(argS(args, 0), argS(args, 1), v)
		}}, true
	}
	return nil, false
}

// propertyNames lists the properties of an interface.
func propertyNames(s ifaceSet) []string {
	switch s {
	case ifaceAccessible:
		return []string{"Name", "Description", "Parent", "ChildCount", "Locale", "AccessibleId", "HelpText"}
	case ifaceApplication:
		return []string{"ToolkitName", "Version", "AtspiVersion", "Id"}
	case ifaceAction:
		return []string{"NActions"}
	case ifaceValue:
		return []string{"MinimumValue", "MaximumValue", "MinimumIncrement", "CurrentValue", "Text"}
	case ifaceText:
		return []string{"CharacterCount", "CaretOffset"}
	}
	return nil
}

func unknownProperty(iface, name string) error {
	return godbus.NewError(errUnknownProp, []any{"no property " + iface + "." + name})
}

// property returns a property of o.
func (o *object) property(iface, name string) (any, error) {
	s := ifaceOf(iface)
	if s&o.ifaces() == 0 {
		return nil, unknownProperty(iface, name)
	}
	var v any
	switch s {
	case ifaceAccessible:
		v = o.accessibleProperty(name)
	case ifaceApplication:
		switch name {
		case "ToolkitName":
			v = toolkitName
		case "Version":
			v = gowakuVersion
		case "AtspiVersion":
			v = atspiVersion
		case "Id":
			v = o.appID
		}
	case ifaceAction:
		if name == "NActions" {
			v = int32(len(nodeActions(o.n)))
		}
	case ifaceValue:
		v = o.valueProperty(name)
	case ifaceText:
		t := newAtspiText(o.n.Text)
		switch name {
		case "CharacterCount":
			v = int32(t.count())
		case "CaretOffset":
			v = int32(t.caret())
		}
	}
	if v == nil {
		return nil, unknownProperty(iface, name)
	}
	return v, nil
}

// setProperty changes Id of the application, which the registry sets, or the
// value of a range.
func (o *object) setProperty(iface, name string, v godbus.Variant) error {
	switch {
	case ifaceOf(iface) == ifaceApplication && name == "Id" && o.kind == objRoot:
		id, ok := v.Value().(int32)
		if !ok {
			return godbus.NewError(errInvalidArgs, []any{"Id is an int32"})
		}
		o.a.mu.Lock()
		o.a.appID = id
		o.a.mu.Unlock()
		return nil
	case ifaceOf(iface) == ifaceValue && name == "CurrentValue" && o.ifaces()&ifaceValue != 0:
		f, ok := v.Value().(float64)
		if !ok {
			return godbus.NewError(errInvalidArgs, []any{"CurrentValue is a double"})
		}
		if !o.setValue(f) {
			return godbus.NewError(errPropReadOnly, []any{"the value cannot be set"})
		}
		return nil
	}
	if _, err := o.property(iface, name); err != nil {
		return err
	}
	return godbus.NewError(errPropReadOnly, []any{iface + "." + name + " is read-only"})
}
