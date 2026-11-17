package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// paintCounter is a leaf that counts its paints.
type paintCounter struct {
	RenderBase
	paints int
}

func (p *paintCounter) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 10, H: 10})
}

func (p *paintCounter) Paint(cv *paint.Canvas, off geom.Offset) {
	p.paints++
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: off.Y, W: 10, H: 10}}, paint.Paint{Color: geom.Color{A: 1}})
}

// counterWidget places a given paintCounter in the tree.
type counterWidget struct{ ro *paintCounter }

func (counterWidget) Key() Key                           { return nil }
func (w counterWidget) CreateRenderObject() RenderObject { return w.ro }
func (counterWidget) UpdateRenderObject(RenderObject)    {}

// boundaryWidget is the widget of a RenderRepaintBoundary.
type boundaryWidget struct{ child Widget }

func (boundaryWidget) Key() Key                         { return nil }
func (boundaryWidget) CreateRenderObject() RenderObject { return NewRenderRepaintBoundary() }
func (boundaryWidget) UpdateRenderObject(RenderObject)  {}
func (w boundaryWidget) ChildWidget() Widget            { return w.child }

// transformed paints its child under a scale or an opacity.
type transformed struct {
	RenderBase
	child   RenderObject
	opacity float32
	paints  int
}

func (r *transformed) SetChild(c RenderObject)  { r.child = c }
func (r *transformed) ChildCount() int          { return 1 }
func (r *transformed) ChildAt(int) RenderObject { return r.child }
func (r *transformed) Layout(c geom.Constraints) geom.Size {
	return LayoutChild(r.child, c)
}

func (r *transformed) Paint(cv *paint.Canvas, off geom.Offset) {
	r.paints++
	cv.Save()
	cv.PushOpacity(r.opacity)
	r.child.Paint(cv, off)
	cv.Restore()
}

type transformedWidget struct {
	ro    *transformed
	child Widget
}

func (transformedWidget) Key() Key                           { return nil }
func (w transformedWidget) CreateRenderObject() RenderObject { return w.ro }
func (transformedWidget) UpdateRenderObject(RenderObject)    {}
func (w transformedWidget) ChildWidget() Widget              { return w.child }

func settled(t *testing.T, root Widget) (*Tree, *paint.Canvas) {
	t.Helper()
	tree := NewTree(root)
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	cv := &paint.Canvas{}
	cv.Reset()
	tree.Paint(cv)
	return tree, cv
}

func TestRepaintBoundary_RecordsOnlyItsLayer(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{counterWidget{leaf}})
	ops := cv.DisplayList().Ops
	if len(ops) != 1 || ops[0].Kind != paint.OpLayer {
		t.Fatalf("got %v, want one layer", ops)
	}
	version := ops[0].Layer.Version()

	leaf.MarkNeedsPaint()
	if !tree.NeedsFrame() {
		t.Fatal("a change in a layer needs a frame")
	}
	if tree.Repaint(cv) {
		t.Error("the picture around the layer did not change")
	}
	if leaf.paints != 2 {
		t.Errorf("painted %d times, want 2", leaf.paints)
	}
	if v := cv.DisplayList().Ops[0].Layer.Version(); v == version {
		t.Error("the version of the layer must change")
	}
	if tree.NeedsFrame() {
		t.Error("nothing is left to do")
	}
}

func TestRepaintBoundary_ParentReusesCleanLayer(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{counterWidget{leaf}})
	tree.MarkNeedsPaint()
	if !tree.Repaint(cv) {
		t.Error("the picture was asked for again")
	}
	if leaf.paints != 1 {
		t.Errorf("painted %d times, want the layer to stay", leaf.paints)
	}
}

func TestRepaintBoundary_MovingDoesNotRecord(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, transformedWidget{&transformed{opacity: 1}, boundaryWidget{counterWidget{leaf}}})
	b := tree.rootRO.ChildAt(0)
	b.SetOffset(geom.Offset{X: 5})
	if !tree.Repaint(cv) {
		t.Error("moving a layer changes the picture of its parent")
	}
	if leaf.paints != 1 {
		t.Errorf("painted %d times, want the layer to stay", leaf.paints)
	}
}

func TestRepaintBoundary_FadingDrawsDirectly(t *testing.T) {
	leaf := &paintCounter{}
	fade := &transformed{opacity: 0.5}
	tree, cv := settled(t, transformedWidget{fade, boundaryWidget{counterWidget{leaf}}})
	for _, op := range cv.DisplayList().Ops {
		if op.Kind == paint.OpLayer {
			t.Fatal("a layer cannot stand in under an opacity")
		}
	}
	leaf.MarkNeedsPaint()
	if !tree.Repaint(cv) {
		t.Fatal("without a layer, a change belongs to the parent")
	}
	if fade.paints != 2 {
		t.Errorf("the parent painted %d times", fade.paints)
	}

	// The fade ends, and the layer takes over with what is there by then.
	fade.opacity = 1
	fade.MarkNeedsPaint()
	tree.Repaint(cv)
	ops := cv.DisplayList().Ops
	if len(ops) != 1 || ops[0].Kind != paint.OpLayer || ops[0].Layer.Empty() {
		t.Fatalf("got %v, want a filled layer", ops)
	}
	leaf.MarkNeedsPaint()
	if tree.Repaint(cv) {
		t.Error("with the layer back, a change stays in it")
	}
}

func TestRepaintBoundary_NestedMeltsIntoTheOuterOne(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{boundaryWidget{counterWidget{leaf}}})
	outer := cv.DisplayList().Ops[0].Layer
	if k := outer.DisplayList().Ops[0].Kind; k != paint.OpRRect {
		t.Fatalf("the outer layer holds %v, want the commands of the inner one", k)
	}
	version := outer.Version()
	leaf.MarkNeedsPaint()
	if tree.Repaint(cv) {
		t.Error("the change stays in the outer layer")
	}
	if outer.Version() == version {
		t.Error("the outer layer must record again")
	}
}

func TestRepaintBoundary_DetachedLayerIsLeftAlone(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{counterWidget{leaf}})
	leaf.MarkNeedsPaint()
	r := tree.rootRO.(*RenderRepaintBoundary)
	r.detach()
	tree.Repaint(cv)
	if leaf.paints != 1 {
		t.Errorf("painted %d times after leaving the tree", leaf.paints)
	}
}

func BenchmarkMarkNeedsPaint_Repeated(b *testing.B) {
	leaf := &paintCounter{}
	tree := NewTree(boundaryWidget{counterWidget{leaf}})
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	b.ReportAllocs()
	for b.Loop() {
		leaf.MarkNeedsPaint()
	}
}
