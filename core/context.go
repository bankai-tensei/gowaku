package core

// BuildContext is the place of a widget in the tree.
type BuildContext struct {
	el *element
}

// Inherited makes Value available to everything below it through DependOn.
// Widgets that depend on it rebuild when Value changes.
type Inherited[T comparable] struct {
	Value T
	Child Widget
}

// inheritedWidget is how the element sees any instantiation of Inherited.
type inheritedWidget interface {
	Widget
	child() Widget
	changedFrom(old Widget) bool
}

// Key implements Widget.
func (Inherited[T]) Key() Key { return nil }

func (i Inherited[T]) child() Widget { return i.Child }

func (i Inherited[T]) changedFrom(old Widget) bool { return old.(Inherited[T]).Value != i.Value }

// DependOn returns the Value of the nearest Inherited[T] above ctx and
// rebuilds ctx when it changes. It reports false if there is none.
func DependOn[T comparable](ctx BuildContext) (T, bool) {
	for p := ctx.el.parent; p != nil; p = p.parent {
		if p.kind != kindInherited {
			continue
		}
		if w, ok := p.widget.(Inherited[T]); ok {
			ctx.el.dependOn(p)
			return w.Value, true
		}
	}
	var zero T
	return zero, false
}

// DependOnAt returns the Value of the nearest Inherited[T] above at, and
// rebuilds ctx when it changes. It serves a widget shown away from the place
// it belongs to, such as an overlay entry that takes the theme of the widget
// that opened it. If at has left the tree, it still finds what was above at
// when it left, and follows the part of that which stays in the tree. It
// reports false if there is none.
func DependOnAt[T comparable](ctx, at BuildContext) (T, bool) {
	for p := at.el.parent; p != nil; p = p.parent {
		if p.kind != kindInherited {
			continue
		}
		if w, ok := p.widget.(Inherited[T]); ok {
			ctx.el.dependOn(p)
			return w.Value, true
		}
	}
	var zero T
	return zero, false
}

// Lookup is DependOn for callbacks: it finds the Value without rebuilding ctx
// when it changes.
func Lookup[T comparable](ctx BuildContext) (T, bool) {
	for p := ctx.el.parent; p != nil; p = p.parent {
		if w, ok := p.widget.(Inherited[T]); ok {
			return w.Value, true
		}
	}
	var zero T
	return zero, false
}

// RenderObject returns the render object of the widget of ctx: its own for a
// render widget, or else the first one below it. It is nil before the first
// Build of the subtree. Read its size and place after a layout, in a callback,
// never during Build.
func (ctx BuildContext) RenderObject() RenderObject {
	for el := ctx.el; el != nil; el = el.child {
		if el.kind == kindRender {
			return el.ro
		}
	}
	return nil
}
