package core

import (
	"fmt"
	"reflect"
)

type elementKind uint8

const (
	kindStateless elementKind = iota
	kindStateful
	kindInherited
	kindRender
)

// element is the long-lived node between a widget and its render object. One
// struct serves all kinds, which keeps reconciling free of interface calls.
type element struct {
	widget Widget
	// typ is the dynamic type of widget, cached for the identity check.
	typ    reflect.Type
	key    Key
	parent *element
	tree   *Tree
	depth  int32
	kind   elementKind

	dirty   bool
	mounted bool

	// child is the only child of every kind except a multi-child render.
	child *element
	state State

	// The fields below belong to kindRender.
	ro       RenderObject
	children []*element
	// spare is the other half of the double buffer of children.
	spare      []*element
	multi      bool
	single     bool
	roChild    RenderObject
	roChildren []RenderObject
	// updating is set while children change, when they cannot be synced.
	updating  bool
	needsSync bool

	// dependents are the elements using this kindInherited element.
	dependents map[*element]struct{}
	// dependencies are the kindInherited elements this element uses.
	dependencies []*element
}

// inflate creates and mounts the element of w.
func (t *Tree) inflate(w Widget, parent *element) *element {
	if w == nil {
		panic("gowaku: a nil Widget was placed in the tree; use SizedBox for an empty space")
	}
	el := &element{widget: w, typ: reflect.TypeOf(w), key: w.Key(), parent: parent, tree: t}
	if parent != nil {
		el.depth = parent.depth + 1
	}
	switch x := w.(type) {
	case inheritedWidget:
		el.kind = kindInherited
	case RenderWidget:
		el.kind = kindRender
		el.ro = x.CreateRenderObject()
		el.ro.base().attach(el.ro, t, el.depth)
		_, el.single = w.(SingleChildWidget)
		_, el.multi = w.(MultiChildWidget)
	case StatefulWidget:
		el.kind = kindStateful
		el.state = x.CreateState()
	case StatelessWidget:
		el.kind = kindStateless
	default:
		panic(fmt.Sprintf("gowaku: %T is not a StatelessWidget, StatefulWidget or RenderWidget", w))
	}
	el.mounted = true

	switch el.kind {
	case kindRender:
		el.updateRenderChildren()
	case kindStateful:
		el.state.attach(el)
		el.state.InitState()
		el.rebuild()
	default:
		el.rebuild()
	}
	return el
}

// update moves el to w, a widget of the same type and key.
func (el *element) update(w Widget) {
	// The same pointer to a widget is the same description: what is below it
	// rebuilds only where it asked to. See Widget.
	if el.typ.Kind() == reflect.Pointer && el.widget == w {
		return
	}
	old := el.widget
	el.widget = w
	switch el.kind {
	case kindRender:
		w.(RenderWidget).UpdateRenderObject(el.ro)
		el.updateRenderChildren()
		if probing && el.tree.probe != nil {
			el.tree.probe.Rebuilt(ElementView{el})
		}
	case kindStateful:
		el.state.DidUpdateWidget(old)
		el.rebuild()
	case kindInherited:
		if w.(inheritedWidget).changedFrom(old) {
			for d := range el.dependents {
				d.markNeedsBuild()
			}
		}
		el.rebuild()
	default:
		el.rebuild()
	}
}

// rebuild runs Build and reconciles its result. It serves every kind but
// kindRender.
func (el *element) rebuild() {
	el.dirty = false
	t := el.tree
	var next Widget
	switch el.kind {
	case kindStateful:
		t.inBuild = true
		next = el.state.Build(BuildContext{el})
		t.inBuild = false
	case kindInherited:
		next = el.widget.(inheritedWidget).child()
	default:
		t.inBuild = true
		next = el.widget.(StatelessWidget).Build(BuildContext{el})
		t.inBuild = false
	}
	if next == nil {
		panic("gowaku: a nil Widget was placed in the tree; use SizedBox for an empty space")
	}
	before := renderObjectOf(el.child)
	el.child = t.updateChild(el, el.child, next)
	// The children of a render element are render objects, so a new object
	// far down a chain of components changes that element's list.
	if before != nil && renderObjectOf(el.child) != before {
		el.renderObjectChanged()
	}
	if probing && t.probe != nil {
		t.probe.Rebuilt(ElementView{el})
	}
}

func (el *element) updateRenderChildren() {
	el.updating = true
	switch {
	case el.multi:
		el.updateChildren(el.widget.(MultiChildWidget).ChildWidgets())
	case el.single:
		if w := el.widget.(SingleChildWidget).ChildWidget(); w != nil || el.child != nil {
			el.child = el.tree.updateChild(el, el.child, w)
		}
	}
	el.updating = false
	el.needsSync = false
	el.syncRenderChildren()
}

// renderObjectChanged tells the nearest render element above el that its list
// of render objects is out of date.
func (el *element) renderObjectChanged() {
	for p := el.parent; p != nil; p = p.parent {
		if p.kind == kindRender {
			if p.updating {
				p.needsSync = true
			} else {
				p.syncRenderChildren()
			}
			return
		}
	}
	el.tree.rootChanged()
}

// syncRenderChildren hands the render objects of the child elements to the
// render object of el, if they differ from what it has.
func (el *element) syncRenderChildren() {
	switch {
	case el.multi:
		list := el.roChildren
		old := len(list)
		changed := old != len(el.children)
		list = list[:0]
		for i, c := range el.children {
			ro := renderObjectOf(c)
			if !changed && el.roChildren[i] != ro {
				changed = true
			}
			list = append(list, ro)
		}
		// The children that left must not stay reachable from the end of
		// the array.
		if len(list) < old {
			clear(list[len(list):old])
		}
		el.roChildren = list
		if changed {
			for _, ro := range list {
				ro.base().parent = el.ro
			}
			el.ro.(MultiChildRenderObject).SetChildren(list)
			el.ro.MarkNeedsLayout()
		}
	case el.single:
		ro := renderObjectOf(el.child)
		if ro == el.roChild {
			return
		}
		el.roChild = ro
		if ro != nil {
			ro.base().parent = el.ro
		}
		el.ro.(SingleChildRenderObject).SetChild(ro)
		el.ro.MarkNeedsLayout()
	}
}

// renderObjectOf returns the render object that represents el in the render
// tree: its own, or that of the first render element below it.
func renderObjectOf(el *element) RenderObject {
	for el != nil {
		if el.kind == kindRender {
			return el.ro
		}
		el = el.child
	}
	return nil
}

func (el *element) markNeedsBuild() {
	if el.dirty || el.kind == kindRender {
		return
	}
	el.dirty = true
	el.tree.dirty = append(el.tree.dirty, el)
}

func (el *element) dependOn(inherited *element) {
	if inherited.dependents == nil {
		inherited.dependents = make(map[*element]struct{})
	}
	if _, ok := inherited.dependents[el]; ok {
		return
	}
	inherited.dependents[el] = struct{}{}
	el.dependencies = append(el.dependencies, inherited)
}

// unmount removes el and everything below it. Children go first, so that a
// Dispose can rely on its ancestors.
func (el *element) unmount() {
	if el.child != nil {
		el.child.unmount()
	}
	for _, c := range el.children {
		c.unmount()
	}
	for _, d := range el.dependencies {
		delete(d.dependents, el)
	}
	if el.state != nil {
		el.state.Dispose()
		el.tree.stopTickersOf(el)
	}
	if el.ro != nil {
		el.ro.base().detach()
	}
	el.mounted = false
}
