package layout

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// paintWidget places a given RenderCustomPaint in a tree.
type paintWidget struct{ r *RenderCustomPaint }

func (paintWidget) Key() core.Key                           { return nil }
func (w paintWidget) CreateRenderObject() core.RenderObject { return w.r }
func (paintWidget) UpdateRenderObject(core.RenderObject)    {}

func newCanvas() *paint.Canvas {
	cv := &paint.Canvas{}
	cv.Reset()
	return cv
}

// mountedPaint returns a RenderCustomPaint in a tree that was laid out.
func mountedPaint(t testing.TB) (*RenderCustomPaint, *core.Tree) {
	t.Helper()
	r := &RenderCustomPaint{Width: 10, Height: 10}
	tree := core.NewTree(paintWidget{r})
	tree.Layout(geom.Tight(geom.Size{W: 10, H: 10}))
	return r, tree
}

func TestRepaintSignal_ZeroValueNotifies(t *testing.T) {
	var s RepaintSignal
	s.Notify()
	if len(s.views) != 0 {
		t.Fatalf("got %d views", len(s.views))
	}
}

func TestRepaintSignal_SetRepaint(t *testing.T) {
	var a, b RepaintSignal
	r := &RenderCustomPaint{}
	cases := []struct {
		name   string
		set    *RepaintSignal
		inA    int
		inB    int
		listen *RepaintSignal
	}{
		{"a を付ける", &a, 1, 0, &a},
		{"同じ a をもう一度付けても 1 つ", &a, 1, 0, &a},
		{"b に替えると a から外れる", &b, 0, 1, &b},
		{"nil で b からも外れる", nil, 0, 0, nil},
		{"nil をもう一度でも何も起きない", nil, 0, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.SetRepaint(tc.set)
			if len(a.views) != tc.inA || len(b.views) != tc.inB {
				t.Errorf("a has %d, b has %d; want %d, %d", len(a.views), len(b.views), tc.inA, tc.inB)
			}
			if r.Repaint() != tc.listen {
				t.Errorf("Repaint() = %p, want %p", r.Repaint(), tc.listen)
			}
		})
	}
}

func TestRepaintSignal_SeveralViews(t *testing.T) {
	var s RepaintSignal
	r1, t1 := mountedPaint(t)
	r2, t2 := mountedPaint(t)
	r1.SetRepaint(&s)
	r2.SetRepaint(&s)
	t1.Paint(newCanvas())
	t2.Paint(newCanvas())
	s.Notify()
	if !t1.NeedsFrame() || !t2.NeedsFrame() {
		t.Fatalf("frames: %v, %v", t1.NeedsFrame(), t2.NeedsFrame())
	}
	r1.SetRepaint(nil)
	if len(s.views) != 1 || s.views[0] != repaintView(r2) {
		t.Fatalf("views after detaching the first: %v", s.views)
	}
}

func TestRepaintSignal_DropsObjectsThatLeftTheTree(t *testing.T) {
	var s RepaintSignal
	r, tree := mountedPaint(t)
	r.SetRepaint(&s)
	tree.Dispose()
	s.Notify()
	if len(s.views) != 0 {
		t.Fatalf("a detached object stayed: %d views", len(s.views))
	}
}

func BenchmarkRepaintSignal_Notify(b *testing.B) {
	var s RepaintSignal
	r, tree := mountedPaint(b)
	r.SetRepaint(&s)
	cv := newCanvas()
	b.ReportAllocs()
	for b.Loop() {
		s.Notify()
		cv.Reset()
		tree.Paint(cv)
	}
}
