package core

import "reflect"

// Key identifies a widget among its siblings across rebuilds. It must be
// comparable. Keys are the one place where gowaku takes an any.
type Key = any

// Widget is an immutable description of part of the UI. Widgets are value
// types, created on every Build and thrown away. Besides Key, a widget
// implements one of StatelessWidget, StatefulWidget or RenderWidget.
//
// A widget of pointer type that a rebuild hands over again as the same pointer
// is taken to be unchanged: its element is not updated, so that nothing below
// it rebuilds but what depends on a changed Inherited or set its state. The
// root of a window holds its content this way, so that a resize rebuilds only
// what reads the size of the window. Ordinary widgets are values and do not do
// this.
type Widget interface {
	// Key returns the identity of the widget, or nil. Wrap a widget with Keyed
	// to give it one.
	Key() Key
}

// StatelessWidget builds its child from its own fields alone.
type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

// StatefulWidget keeps a State that survives rebuilds.
type StatefulWidget interface {
	Widget
	CreateState() State
}

// RenderWidget is backed by a RenderObject. To have children it also implements
// SingleChildWidget or MultiChildWidget.
type RenderWidget interface {
	Widget
	CreateRenderObject() RenderObject
	// UpdateRenderObject copies the fields of the widget to ro, which was made
	// by CreateRenderObject of a widget of the same type. It should mark ro
	// dirty only for fields that really changed.
	UpdateRenderObject(ro RenderObject)
}

// SingleChildWidget is a RenderWidget with at most one child. Its render object
// implements SingleChildRenderObject.
type SingleChildWidget interface {
	RenderWidget
	// ChildWidget returns the child, or nil.
	ChildWidget() Widget
}

// MultiChildWidget is a RenderWidget with a list of children. Its render object
// implements MultiChildRenderObject.
type MultiChildWidget interface {
	RenderWidget
	// ChildWidgets returns the children.
	ChildWidgets() []Widget
}

// StatelessBase gives a stateless widget its Key method.
type StatelessBase struct{}

// Key implements Widget.
func (StatelessBase) Key() Key { return nil }

// StatefulBase gives a stateful widget its Key method.
type StatefulBase struct{}

// Key implements Widget.
func (StatefulBase) Key() Key { return nil }

// keyed gives a widget an identity without its type having to know about keys.
// It is an ordinary stateless widget, so the reconciler needs no special case.
type keyed struct {
	key   Key
	child Widget
}

// Keyed returns w with the identity key. It panics if key is not comparable.
func Keyed(key Key, w Widget) Widget {
	if key == nil {
		panic("gowaku: Keyed called with a nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("gowaku: Keyed called with a key that is not comparable")
	}
	return keyed{key, w}
}

// Key implements Widget.
func (k keyed) Key() Key { return k.key }

// Build implements StatelessWidget.
func (k keyed) Build(BuildContext) Widget { return k.child }
