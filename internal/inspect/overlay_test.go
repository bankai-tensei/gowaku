//go:build gowakudebug

package inspect

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// outlines counts the outlines that ShowBounds draws over t.
func outlines(s *Session, t Target) int {
	var cv paint.Canvas
	cv.Reset()
	s.Overlay(t, &cv)
	return len(cv.DisplayList().Ops)
}

func TestOverlay_BoundsSkipHiddenEmptyAndOffscreen(t *testing.T) {
	s, target, _ := newSampleSession(t)
	s.ShowBounds = true
	// Column, Padding, box, RepaintBoundary, box: the Opacity of 0 and its
	// box are hidden.
	if got := outlines(s, target); got != 5 {
		t.Errorf("got %d outlines, want 5", got)
	}

	empty := newFakeTarget("empty", basic.Column{Children: []core.Widget{
		basic.SizedBox{Width: 50, Height: 0},
		basic.SizedBox{Width: 50, Height: 20},
	}}, geom.Size{W: 100, H: 100})
	defer empty.tree.Dispose()
	if got := outlines(s, empty); got != 2 {
		t.Errorf("got %d outlines, want 2: a box without height has none", got)
	}

	far := newFakeTarget("far", basic.Stack{Children: []core.Widget{
		basic.Positioned{Left: 500, Top: 10, Width: 20, Height: 20, Child: basic.SizedBox{}},
		basic.Positioned{Left: 10, Top: 10, Width: 20, Height: 20, Child: basic.SizedBox{}},
	}}, geom.Size{W: 100, H: 100})
	defer far.tree.Dispose()
	before := outlines(s, far)
	s.ShowBounds = false
	if outlines(s, far) != 0 {
		t.Fatal("no outlines without ShowBounds")
	}
	s.ShowBounds = true
	// The stack, the box inside the window, and what Positioned adds around
	// it; the box out of the window is left out.
	stackAndInside := 0
	walkAll(far.tree.RootRenderObject(), func(ro core.RenderObject) {
		r := globalRect(ro)
		if r.W > 0 && r.H > 0 && r.X < 100 {
			stackAndInside++
		}
	})
	if before != stackAndInside {
		t.Errorf("got %d outlines, want %d: the box past the right edge has none", before, stackAndInside)
	}
}

func walkAll(ro core.RenderObject, fn func(core.RenderObject)) {
	fn(ro)
	for i := range ro.ChildCount() {
		if c := ro.ChildAt(i); c != nil {
			walkAll(c, fn)
		}
	}
}

func TestOverlay_TurnedObjectsAreOutlinedTurned(t *testing.T) {
	target := newFakeTarget("turned", basic.Center{Child: basic.Transform{
		Transform: geom.Rotation(math.Pi / 6),
		Child:     basic.SizedBox{Width: 40, Height: 20},
	}}, geom.Size{W: 200, H: 200})
	defer target.tree.Dispose()
	s := NewSession(&fakeHost{targets: []Target{target}}, nil)
	var leaf core.RenderObject
	walkAll(target.tree.RootRenderObject(), func(ro core.RenderObject) {
		if ro.ChildCount() == 0 {
			leaf = ro
		}
	})
	m := globalTransform(leaf)
	if m.IsScaleTranslate() {
		t.Fatal("the box under the Transform must be turned")
	}
	// The transform of the walk agrees with the one of core.
	got := m.ApplyRect(geom.Rect{W: 40, H: 20})
	want := globalRect(leaf)
	if geom.Abs32(got.X-want.X) > 0.01 || geom.Abs32(got.W-want.W) > 0.01 {
		t.Errorf("got bounds %v, core says %v", got, want)
	}
	s.SelectRenderObject(target, leaf)
	var cv paint.Canvas
	cv.Reset()
	s.Overlay(target, &cv)
	turned := false
	for _, op := range cv.DisplayList().Ops {
		if !op.Transform.IsIdentity() {
			turned = true
		}
	}
	if !turned {
		t.Error("the outline of a turned selection must be turned")
	}
}

func TestOverlay_InsetsOfThePaddingAreShaded(t *testing.T) {
	s, target, _ := newSampleSession(t)
	var padding core.RenderObject
	walkAll(target.tree.RootRenderObject(), func(ro core.RenderObject) {
		if padding == nil && TypeName(ro) == "layout.RenderPadding" {
			padding = ro
		}
	})
	if padding == nil {
		t.Fatal("no padding in the sample")
	}
	s.SelectRenderObject(target, padding)
	var cv paint.Canvas
	cv.Reset()
	s.Overlay(target, &cv)
	shaded := 0
	for _, op := range cv.DisplayList().Ops {
		if op.Color == insetFill {
			shaded++
		}
	}
	if shaded != 4 {
		t.Errorf("got %d shaded strips, want the 4 sides of the padding", shaded)
	}
}

func TestOverlay_DoesNotAllocate(t *testing.T) {
	shaper := testutil.NewShaper(t)
	target := newFakeTarget("app", sampleTree(), geom.Size{W: 200, H: 150})
	defer target.tree.Dispose()
	s := NewSession(&fakeHost{targets: []Target{target}}, shaper)
	s.ShowBounds = true
	s.SelectRenderObject(target, findRO[*core.RenderRepaintBoundary](target))
	s.SetPicking(true)
	s.Pointer(target, geom.Offset{X: 20, Y: 20}, PointerMove)
	var cv paint.Canvas
	// The first draw shapes the labels and grows the canvas.
	s.Overlay(target, &cv)
	allocs := testing.AllocsPerRun(20, func() {
		cv.Reset()
		s.Overlay(target, &cv)
	})
	if allocs != 0 {
		t.Errorf("the overlay allocates %v times a frame; it draws inside the frames it measures", allocs)
	}
}

func TestLabelBox_StaysInTheWindow(t *testing.T) {
	win := geom.Rect{W: 200, H: 100}
	ts := geom.Size{W: 60, H: 14} // a box of 70 by 18
	tests := []struct {
		name string
		r    geom.Rect
		want geom.Offset
	}{
		{"room above", geom.Rect{X: 20, Y: 50, W: 40, H: 20}, geom.Offset{X: 20, Y: 30}},
		{"below at the top", geom.Rect{X: 20, Y: 5, W: 40, H: 20}, geom.Offset{X: 20, Y: 27}},
		{"inside when it fills the window", geom.Rect{X: 0, Y: 0, W: 200, H: 100}, geom.Offset{X: 0, Y: 2}},
		{"pushed in from the right", geom.Rect{X: 180, Y: 50, W: 40, H: 20}, geom.Offset{X: 130, Y: 30}},
		{"pushed in from the left", geom.Rect{X: -30, Y: 50, W: 40, H: 20}, geom.Offset{X: 0, Y: 30}},
		{"out of the window below", geom.Rect{X: 20, Y: 300, W: 40, H: 20}, geom.Offset{X: 20, Y: 82}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box := labelBox(tt.r, ts, win)
			if box.Origin() != tt.want || box.W != 70 || box.H != 18 {
				t.Errorf("got %v, want at %v", box, tt.want)
			}
		})
	}
}

func TestIcons_FitTheGrid(t *testing.T) {
	for name, p := range map[string]*paint.Path{"pick": iconPick, "bounds": iconBounds, "repaints": iconRepaints, "pause": iconPause} {
		if p == nil || p.Empty() {
			t.Errorf("%s: no icon", name)
			continue
		}
		b := p.Bounds()
		if b.X < 0 || b.Y < 0 || b.X+b.W > 24 || b.Y+b.H > 24 || b.W < 10 || b.H < 10 {
			t.Errorf("%s: bounds %v do not fill the 24 unit grid", name, b)
		}
	}
}
