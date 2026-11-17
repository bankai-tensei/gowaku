package control

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// meter is a State that only changes what its painter draws.
type meter struct {
	core.StatefulBase
	state **meterState
}

type meterState struct {
	core.StateBase
	signal  layout.RepaintSignal
	other   layout.RepaintSignal
	use     *layout.RepaintSignal
	level   float32
	drawn   float32
	builds  int
	layouts int
	paints  int
}

func (m meter) CreateState() core.State {
	s := &meterState{}
	s.use = &s.signal
	*m.state = s
	return s
}

func (s *meterState) Build(core.BuildContext) core.Widget {
	s.builds++
	return wtest.LayoutCount{N: &s.layouts, Child: basic.RepaintBoundary{Child: CustomPaint{
		Width: 40, Height: 20, Repaint: s.use,
		Painter: layout.PainterFunc(func(cv *paint.Canvas, sz geom.Size) {
			s.paints++
			s.drawn = s.level
			cv.DrawRRect(geom.RRectAll(geom.Rect{W: sz.W * s.level, H: sz.H}, 2), paint.Paint{Color: geom.Hex(0x34c759)})
		}),
	}}}
}

// meterTree returns a tree with a meter that has drawn its first frame, and a
// function that draws the next one.
func meterTree(t testing.TB) (*core.Tree, *meterState, *paint.Canvas, func() bool) {
	t.Helper()
	var s *meterState
	tree := core.NewTree(meter{state: &s})
	t.Cleanup(tree.Dispose)
	size := geom.Size{W: 100, H: 100}
	cv := &paint.Canvas{}
	frame := func() bool {
		tree.Build()
		tree.Layout(geom.Tight(size))
		return tree.Repaint(cv)
	}
	tree.Layout(geom.Tight(size))
	cv.Reset()
	tree.Paint(cv)
	return tree, s, cv, frame
}

func TestCustomPaint_RepaintSignalPaintsWithoutBuildOrLayout(t *testing.T) {
	tree, s, _, frame := meterTree(t)
	builds, layouts, paints := s.builds, s.layouts, s.paints
	if paints == 0 {
		t.Fatal("the first frame did not paint")
	}

	s.level = 0.5
	s.signal.Notify()
	if tree.NeedsBuild() {
		t.Error("Notify asked for a build")
	}
	if !tree.NeedsFrame() {
		t.Fatal("Notify asked for no frame")
	}
	if frame() {
		t.Error("the whole window was recorded again, not only the boundary")
	}
	if s.paints != paints+1 || s.drawn != 0.5 {
		t.Errorf("paints %d -> %d, drew %v", paints, s.paints, s.drawn)
	}
	if s.builds != builds || s.layouts != layouts {
		t.Errorf("builds %d -> %d, layouts %d -> %d", builds, s.builds, layouts, s.layouts)
	}
	if tree.NeedsFrame() {
		t.Error("a frame is still due after the paint")
	}
}

func TestCustomPaint_RepaintSignalSwapDetaches(t *testing.T) {
	cases := []struct {
		name string
		next func(s *meterState) *layout.RepaintSignal
	}{
		{"別の signal に替える", func(s *meterState) *layout.RepaintSignal { return &s.other }},
		{"nil にする", func(*meterState) *layout.RepaintSignal { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, s, _, frame := meterTree(t)
			s.SetState(func() { s.use = tc.next(s) })
			frame()
			paints := s.paints

			s.signal.Notify()
			if tree.NeedsFrame() {
				t.Fatal("the old signal still repaints")
			}
			if s.use != nil {
				s.use.Notify()
				if !tree.NeedsFrame() {
					t.Fatal("the new signal does not repaint")
				}
				frame()
				if s.paints != paints+1 {
					t.Errorf("paints %d -> %d", paints, s.paints)
				}
			}
		})
	}
}

func TestCustomPaint_RepaintSignalOutlivesThePaint(t *testing.T) {
	var s *wtest.HolderState
	var sig layout.RepaintSignal
	tree := core.NewTree(wtest.Holder{First: CustomPaint{Width: 10, Height: 10, Repaint: &sig}, State: &s})
	t.Cleanup(tree.Dispose)
	size := geom.Tight(geom.Size{W: 50, H: 50})
	tree.Layout(size)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)

	s.SetState(func() { s.Child = basic.SizedBox{} })
	tree.Build()
	tree.Layout(size)
	tree.Repaint(&cv)
	sig.Notify()
	if tree.NeedsFrame() {
		t.Error("a CustomPaint that left the tree asked for a frame")
	}
}

func BenchmarkCustomPaint_RepaintSignal(b *testing.B) {
	_, s, _, frame := meterTree(b)
	b.ReportAllocs()
	for b.Loop() {
		s.level += 0.001
		s.signal.Notify()
		frame()
	}
}
