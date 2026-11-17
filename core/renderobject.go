package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// RenderObject does the layout, painting and hit testing of a RenderWidget.
// Implementations embed RenderBase and provide Layout and Paint. With children,
// they also override ChildCount and ChildAt.
type RenderObject interface {
	// Layout decides the size of the object within c, and positions its
	// children. Children must be laid out with LayoutChild, never directly.
	Layout(c geom.Constraints) geom.Size
	// Paint draws the object at off, the position of its top left corner on
	// the canvas. It must not allocate.
	Paint(cv *paint.Canvas, off geom.Offset)
	// HitTest adds the object and what is under pos to r, if pos is inside.
	// pos is relative to the object.
	HitTest(pos geom.Offset, r *HitTestResult) bool
	// Semantics describes the object for accessibility tools.
	Semantics(sb *SemanticsBuilder)

	// Parent returns the object above this one, or nil.
	Parent() RenderObject
	// ChildCount returns the number of children.
	ChildCount() int
	// ChildAt returns child i, counted from the one painted first.
	ChildAt(i int) RenderObject
	// Size returns the result of the last layout.
	Size() geom.Size
	// Offset returns the position relative to the parent.
	Offset() geom.Offset
	// SetOffset is called by the parent during its Layout.
	SetOffset(off geom.Offset)
	// MarkNeedsLayout schedules a new layout.
	MarkNeedsLayout()
	// MarkNeedsPaint schedules a new paint, for changes that keep the size.
	MarkNeedsPaint()

	base() *RenderBase
}

// SingleChildRenderObject is the render object of a SingleChildWidget.
type SingleChildRenderObject interface {
	RenderObject
	// SetChild stores the child, which can be nil.
	SetChild(child RenderObject)
}

// MultiChildRenderObject is the render object of a MultiChildWidget.
type MultiChildRenderObject interface {
	RenderObject
	// SetChildren stores the children. The slice stays owned by core, and is
	// valid until the next SetChildren.
	SetChildren(children []RenderObject)
}

// RenderBase is embedded by every RenderObject.
type RenderBase struct {
	self   RenderObject
	tree   *Tree
	parent RenderObject
	depth  int32

	offset      geom.Offset
	size        geom.Size
	constraints geom.Constraints
	needsLayout bool
	// boundary means that a new layout of this object cannot change anything
	// above it, because its size does not depend on its contents.
	boundary bool

	// repaint is the repaint boundary that this object is, if it is one that
	// keeps its picture in a layer at the moment.
	repaint *RenderRepaintBoundary
	// markedPaint is the paint generation of the tree in which this object last
	// asked for a paint. Asking again changes nothing, and stops the climb.
	markedPaint uint64
	// semanticsID is the ID of the semantics node the object opens, given the
	// first time it opens one.
	semanticsID semantics.ID
}

func (b *RenderBase) base() *RenderBase { return b }

func (b *RenderBase) attach(self RenderObject, t *Tree, depth int32) {
	b.self, b.tree, b.depth = self, t, depth
	b.needsLayout = true
}

func (b *RenderBase) detach() {
	if b.tree != nil {
		b.tree.detached(b.self)
	}
	b.tree, b.parent = nil, nil
}

// Parent implements RenderObject.
func (b *RenderBase) Parent() RenderObject { return b.parent }

// ChildCount implements RenderObject for objects without children.
func (b *RenderBase) ChildCount() int { return 0 }

// ChildAt implements RenderObject for objects without children.
func (b *RenderBase) ChildAt(int) RenderObject {
	panic("gowaku: ChildAt called on a render object without children")
}

// Size implements RenderObject.
func (b *RenderBase) Size() geom.Size { return b.size }

// Offset implements RenderObject.
func (b *RenderBase) Offset() geom.Offset { return b.offset }

// SetOffset implements RenderObject.
func (b *RenderBase) SetOffset(off geom.Offset) {
	if b.offset != off {
		b.offset = off
		// The picture of the object stays what it was. What changes is where the
		// parent puts it, which matters when the object is a repaint boundary.
		if b.parent != nil {
			b.parent.MarkNeedsPaint()
		} else {
			b.MarkNeedsPaint()
		}
	}
}

// MarkNeedsLayout implements RenderObject. It climbs to the nearest relayout
// boundary, which is where the next layout starts.
func (b *RenderBase) MarkNeedsLayout() {
	if b.tree == nil || b.needsLayout {
		return
	}
	b.needsLayout = true
	if b.boundary {
		b.tree.layoutDirty = append(b.tree.layoutDirty, b)
		return
	}
	b.parent.MarkNeedsLayout()
}

// MarkNeedsPaint implements RenderObject. It climbs to the nearest repaint
// boundary, whose layer the next frame records again. Without one on the way,
// the picture of the window itself is what changed.
func (b *RenderBase) MarkNeedsPaint() {
	t := b.tree
	if t == nil {
		return
	}
	for cur := b; ; {
		if cur.markedPaint == t.paintGeneration {
			return
		}
		cur.markedPaint = t.paintGeneration
		if r := cur.repaint; r != nil {
			t.addDirtyLayer(r)
			return
		}
		if cur.parent == nil {
			t.needsPaint = true
			return
		}
		cur = cur.parent.base()
	}
}

// HitTest implements RenderObject. The child painted last comes first, and the
// first child hit ends the search. A hidden Hider is skipped with its subtree.
//
// Where no child is hit, the object is hit anywhere inside its bounds, unless
// it is a SelfHitTester that says otherwise: the objects of package layout that
// only place or paint around their children (Align, Padding, Flex, Stack and
// the like) are hit only where a child is, so that the empty space of a layer
// lets the pointer through to what is under it.
func (b *RenderBase) HitTest(pos geom.Offset, r *HitTestResult) bool {
	if pos.X < 0 || pos.Y < 0 || pos.X >= b.size.W || pos.Y >= b.size.H {
		return false
	}
	hit := false
	for i := b.self.ChildCount() - 1; i >= 0; i-- {
		c := b.self.ChildAt(i)
		if isHidden(c) {
			continue
		}
		if c.HitTest(pos.Sub(c.Offset()), r) {
			hit = true
			break
		}
	}
	if !hit {
		if s, ok := b.self.(SelfHitTester); ok && !s.HitTestSelf(pos) {
			return false
		}
	}
	r.Path = append(r.Path, b.self)
	return true
}

// SelfHitTester is a RenderObject that decides whether it is hit at a point of
// its bounds where none of its children is. RenderBase.HitTest asks it; an
// object that is not one is hit anywhere inside its bounds, which is right for
// what draws or takes the pointer itself (text, images, painted controls,
// Listener, GestureDetector, Pressable).
type SelfHitTester interface {
	RenderObject
	// HitTestSelf reports whether the object is hit at pos, relative to it,
	// where no child is hit.
	HitTestSelf(pos geom.Offset) bool
}

// Semantics implements RenderObject. It describes nothing itself, and lets the
// children speak.
func (b *RenderBase) Semantics(sb *SemanticsBuilder) {
	for i := range b.self.ChildCount() {
		sb.Visit(b.self.ChildAt(i))
	}
}

// LayoutChild lays out ro under c and returns its size. A child that is clean
// under the same constraints keeps its result, which is what limits a relayout
// to the part of the tree that changed.
func LayoutChild(ro RenderObject, c geom.Constraints) geom.Size {
	b := ro.base()
	if !b.needsLayout && b.constraints == c {
		return b.size
	}
	b.constraints = c
	b.boundary = b.parent == nil || c.IsTight()
	b.size = ro.Layout(c)
	b.needsLayout = false
	b.MarkNeedsPaint()
	if probing && b.tree != nil && b.tree.probe != nil {
		b.tree.probe.LaidOut(ro)
	}
	return b.size
}

// HitTestResult lists the render objects under a point, deepest first.
type HitTestResult struct {
	Path []RenderObject
}
