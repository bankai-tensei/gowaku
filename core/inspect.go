package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file lets tools, such as the Inspector of -tags gowakudebug, read the
// element tree. Nothing in a frame calls it. See docs/ADR/0073-inspector.md.

// ElementKind says what kind of widget an element holds.
type ElementKind uint8

// The kinds of elements, after the kinds of widgets.
const (
	ElementStateless ElementKind = iota
	ElementStateful
	ElementInherited
	ElementRender
)

// String returns the name of the kind.
func (k ElementKind) String() string {
	switch k {
	case ElementStateless:
		return "Stateless"
	case ElementStateful:
		return "Stateful"
	case ElementInherited:
		return "Inherited"
	case ElementRender:
		return "Render"
	}
	return "ElementKind(?)"
}

// ElementView reads one element of a tree, for tools. The zero value refers to
// no element. Views are comparable, and two views of the same element are
// equal. A view stays valid after its element leaves the tree, but then
// reports Mounted false and no longer changes.
type ElementView struct{ el *element }

// RootElement returns the element at the root of the tree.
func (t *Tree) RootElement() ElementView { return ElementView{t.root} }

// IsZero reports whether v refers to no element.
func (v ElementView) IsZero() bool { return v.el == nil }

// Widget returns the widget the element was last built from.
func (v ElementView) Widget() Widget { return v.el.widget }

// Kind returns the kind of the element.
func (v ElementView) Kind() ElementKind {
	switch v.el.kind {
	case kindStateful:
		return ElementStateful
	case kindInherited:
		return ElementInherited
	case kindRender:
		return ElementRender
	}
	return ElementStateless
}

// Key returns the key of the widget, or nil.
func (v ElementView) Key() Key { return v.el.key }

// State returns the state of a stateful element, or nil.
func (v ElementView) State() State { return v.el.state }

// RenderObject returns the render object a render element owns, or nil for
// the other kinds.
func (v ElementView) RenderObject() RenderObject { return v.el.ro }

// NearestRenderObject returns the render object of the element or, for the
// other kinds, of the first render element below it.
func (v ElementView) NearestRenderObject() RenderObject { return renderObjectOf(v.el) }

// Depth returns the distance from the root, which has depth 0.
func (v ElementView) Depth() int { return int(v.el.depth) }

// Mounted reports whether the element is still in its tree.
func (v ElementView) Mounted() bool { return v.el.mounted }

// Dirty reports whether the element waits for the next Build.
func (v ElementView) Dirty() bool { return v.el.dirty }

// Dependencies returns how many inherited widgets the element depends on.
func (v ElementView) Dependencies() int { return len(v.el.dependencies) }

// Parent returns the element above, or the zero view at the root.
func (v ElementView) Parent() ElementView { return ElementView{v.el.parent} }

// ChildCount returns the number of children.
func (v ElementView) ChildCount() int {
	if v.el.kind == kindRender && v.el.multi {
		return len(v.el.children)
	}
	if v.el.child != nil {
		return 1
	}
	return 0
}

// ChildAt returns child i.
func (v ElementView) ChildAt(i int) ElementView {
	if v.el.kind == kindRender && v.el.multi {
		return ElementView{v.el.children[i]}
	}
	if i != 0 || v.el.child == nil {
		panic("gowaku: ElementView.ChildAt out of range")
	}
	return ElementView{v.el.child}
}

// ElementOf returns the render element that owns ro, or the zero view if ro is
// not in the tree. It walks the whole tree.
func (t *Tree) ElementOf(ro RenderObject) ElementView {
	if ro == nil || t.root == nil {
		return ElementView{}
	}
	return findElement(ElementView{t.root}, ro)
}

func findElement(v ElementView, ro RenderObject) ElementView {
	if v.el.ro == ro {
		return v
	}
	for i := range v.ChildCount() {
		if found := findElement(v.ChildAt(i), ro); !found.IsZero() {
			return found
		}
	}
	return ElementView{}
}

// ConstraintsOf returns the constraints of the last layout of ro.
func ConstraintsOf(ro RenderObject) geom.Constraints { return ro.base().constraints }

// NeedsLayoutOf reports whether ro waits for a layout.
func NeedsLayoutOf(ro RenderObject) bool { return ro.base().needsLayout }

// IsRelayoutBoundary reports whether a new layout of ro stops at ro, because
// its size does not depend on its contents.
func IsRelayoutBoundary(ro RenderObject) bool { return ro.base().boundary }

// HasLayer reports whether ro is a repaint boundary that keeps its picture in a
// layer at the moment.
func HasLayer(ro RenderObject) bool { return ro.base().repaint != nil }

// NeedsRepaint reports whether the picture of the tree, not only some of its
// layers, will be recorded again at the next paint.
func (t *Tree) NeedsRepaint() bool { return t.needsPaint }

// HostedBy reports whether ro is attached to t.
func (t *Tree) HostedBy(ro RenderObject) bool { return ro != nil && ro.base().tree == t }

// SemanticsIDOf returns the ID of the semantics nodes that ro opens, or zero if
// it has opened none yet. Tools match it against the IDs of Tree.Semantics.
func SemanticsIDOf(ro RenderObject) semantics.ID { return ro.base().semanticsID }
