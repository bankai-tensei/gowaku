package core

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// turn wraps its child, placed at offset, in a fixed transform.
type turn struct {
	m      geom.Affine
	offset geom.Offset
	child  Widget
}

type renderTurn struct {
	RenderBase
	m      geom.Affine
	offset geom.Offset
	child  RenderObject
}

func (turn) Key() Key { return nil }

func (w turn) CreateRenderObject() RenderObject {
	return &renderTurn{m: w.m, offset: w.offset}
}

func (w turn) UpdateRenderObject(ro RenderObject) {
	r := ro.(*renderTurn)
	r.m, r.offset = w.m, w.offset
}

func (w turn) ChildWidget() Widget                     { return w.child }
func (r *renderTurn) SetChild(c RenderObject)          { r.child = c }
func (r *renderTurn) Paint(*paint.Canvas, geom.Offset) {}
func (r *renderTurn) ChildTransform() geom.Affine      { return r.m }

func (r *renderTurn) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

func (r *renderTurn) ChildAt(int) RenderObject { return r.child }

func (r *renderTurn) Layout(c geom.Constraints) geom.Size {
	if r.child != nil {
		LayoutChild(r.child, geom.Loose(c.Biggest()))
		r.child.SetOffset(r.offset)
	}
	return c.Biggest()
}

func near(a, b geom.Offset) bool {
	return geom.Abs32(a.X-b.X) < 1e-4 && geom.Abs32(a.Y-b.Y) < 1e-4
}

func TestTransform_LocalAndGlobalRoundTrip(t *testing.T) {
	m := geom.Translation(geom.Offset{X: 50, Y: 0}).Mul(geom.Rotation(math.Pi / 2))
	h := newHarness(turn{m: m, offset: geom.Offset{X: 10}, child: labeled{leaf{name: "ok", w: 20, h: 10}}})
	h.layout()
	child := h.rootRO().ChildAt(0)
	// The child's (0, 0) sits at (10, 0) before the turn: (0, 10) after it,
	// then 50 to the right.
	if got := LocalToGlobal(child, geom.Offset{}); !near(got, geom.Offset{X: 50, Y: 10}) {
		t.Errorf("LocalToGlobal: got %v, want (50, 10)", got)
	}
	p := geom.Offset{X: 3, Y: 4}
	back, ok := GlobalToLocal(child, LocalToGlobal(child, p))
	if !ok || !near(back, p) {
		t.Errorf("round trip: got %v, %v", back, ok)
	}
	// Turned a quarter, the 20x10 box is 10 wide and 20 tall.
	r := LocalRectToGlobal(child, geom.Rect{W: 20, H: 10})
	if geom.Abs32(r.W-10) > 1e-4 || geom.Abs32(r.H-20) > 1e-4 || geom.Abs32(r.X-40) > 1e-4 {
		t.Errorf("LocalRectToGlobal: got %+v, want x=40 w=10 h=20", r)
	}
}

func TestTransform_SemanticsRectIsTheTurnedBounds(t *testing.T) {
	m := geom.Translation(geom.Offset{X: 50, Y: 0}).Mul(geom.Rotation(math.Pi / 2))
	h := newHarness(turn{m: m, offset: geom.Offset{X: 10}, child: labeled{leaf{name: "ok", w: 20, h: 10}}})
	h.layout()
	n := h.tree.Semantics().Children[0]
	want := LocalRectToGlobal(h.rootRO().ChildAt(0), geom.Rect{W: 20, H: 10})
	if n.Rect != want {
		t.Errorf("got %+v, want %+v", n.Rect, want)
	}
}

func TestTransform_IdentityKeepsGlobalOffset(t *testing.T) {
	h := newHarness(turn{offset: geom.Offset{X: 10.3, Y: 7.1}, child: leaf{w: 20, h: 10}})
	h.layout()
	child := h.rootRO().ChildAt(0)
	p := geom.Offset{X: 1.7, Y: 2.9}
	if got, want := LocalToGlobal(child, p), p.Add(GlobalOffset(child)); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, ok := GlobalToLocal(child, p); !ok || got != p.Sub(GlobalOffset(child)) {
		t.Errorf("got %v", got)
	}
}

func TestTransform_FlatTransformHasNoLocalPoint(t *testing.T) {
	h := newHarness(turn{m: geom.Scaling(0, 1), child: leaf{w: 20, h: 10}})
	h.layout()
	if _, ok := GlobalToLocal(h.rootRO().ChildAt(0), geom.Offset{}); ok {
		t.Error("a flat transform must have no inverse")
	}
}
