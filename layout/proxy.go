package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Constraints are the sizes a parent allows its child to take.
type Constraints = geom.Constraints

// Insets are distances from the four edges of a rectangle.
type Insets = geom.Insets

// RenderProxy is a render object with at most one child, which it sizes itself
// after and paints in place. Objects that change only part of this embed it.
type RenderProxy struct {
	core.RenderBase
	child core.RenderObject
}

// SetChild implements core.SingleChildRenderObject.
func (r *RenderProxy) SetChild(child core.RenderObject) { r.child = child }

// ChildCount implements core.RenderObject.
func (r *RenderProxy) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements core.RenderObject.
func (r *RenderProxy) ChildAt(int) core.RenderObject { return r.child }

// Layout implements core.RenderObject.
func (r *RenderProxy) Layout(c Constraints) geom.Size {
	if r.child == nil {
		return c.Constrain(geom.Size{})
	}
	return core.LayoutChild(r.child, c)
}

// Paint implements core.RenderObject.
func (r *RenderProxy) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child != nil {
		r.child.Paint(cv, off.Add(r.child.Offset()))
	}
}

// RenderSemantics describes its subtree to assistive technology, and does what
// a screen reader asks of it with the callbacks it holds.
type RenderSemantics struct {
	RenderProxy
	Node semantics.Node
	// OnTap and the others do the actions. A nil one leaves its action out.
	OnTap      func()
	OnIncrease func()
	OnDecrease func()
	OnSetValue func(text string, number float64)
	OnExpand   func()
	OnCollapse func()
}

// Semantics implements core.RenderObject.
func (r *RenderSemantics) Semantics(sb *core.SemanticsBuilder) {
	n := r.Node
	n.Children = nil
	n.Actions |= r.actions()
	sb.Open(n, r.Size())
	if r.child != nil {
		sb.Visit(r.child)
	}
	sb.Close()
}

// RenderExcludeSemantics leaves its subtree out of the semantics tree. It is
// laid out, painted and hit tested as usual.
type RenderExcludeSemantics struct {
	RenderProxy
}

// Semantics implements core.RenderObject. It does not visit the child.
func (r *RenderExcludeSemantics) Semantics(*core.SemanticsBuilder) {}

func (r *RenderSemantics) actions() semantics.Actions {
	var a semantics.Actions
	for _, c := range [...]struct {
		set    bool
		action semantics.Actions
	}{
		{r.OnTap != nil, semantics.ActionTap},
		{r.OnIncrease != nil, semantics.ActionIncrease},
		{r.OnDecrease != nil, semantics.ActionDecrease},
		{r.OnSetValue != nil, semantics.ActionSetValue},
		{r.OnExpand != nil, semantics.ActionExpand},
		{r.OnCollapse != nil, semantics.ActionCollapse},
	} {
		if c.set {
			a |= c.action
		}
	}
	return a
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *RenderSemantics) PerformSemanticsAction(a semantics.Actions, arg core.SemanticsArg) bool {
	if r.Node.State&semantics.Disabled != 0 {
		return false
	}
	switch {
	case a == semantics.ActionTap && r.OnTap != nil:
		r.OnTap()
	case a == semantics.ActionIncrease && r.OnIncrease != nil:
		r.OnIncrease()
	case a == semantics.ActionDecrease && r.OnDecrease != nil:
		r.OnDecrease()
	case a == semantics.ActionSetValue && r.OnSetValue != nil:
		r.OnSetValue(arg.Text, arg.Number)
	case a == semantics.ActionExpand && r.OnExpand != nil:
		r.OnExpand()
	case a == semantics.ActionCollapse && r.OnCollapse != nil:
		r.OnCollapse()
	default:
		return false
	}
	return true
}
