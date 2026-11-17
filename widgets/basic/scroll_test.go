package basic

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
)

// runUntilIdleWith plays a motion with frame in place of RenderTree.
func tallChild(height float32) core.Widget {
	return Container{Height: height, Color: geom.Hex(0x123456)}
}

func viewportOf(t testing.TB, tree *core.Tree) *layout.RenderViewport {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 10, Y: 10}).Path {
		if v, ok := ro.(*layout.RenderViewport); ok {
			return v
		}
	}
	t.Fatal("no viewport")
	return nil
}

func TestScroll_WheelMovesWithInertiaAndStops(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -1)
	if !tree.Ticking() {
		t.Fatal("the wheel must start the motion")
	}
	var steps []float64
	frames := wtest.RunUntilIdle(t, tree, size, func() { steps = append(steps, v.Position.Value()) })
	if got := v.Position.Value(); got != kit.NotchDistance {
		t.Errorf("rests at %v, want %v", got, kit.NotchDistance)
	}
	if frames < 5 {
		t.Errorf("a jump, not a motion: %d frames", frames)
	}
	for i := 1; i < len(steps); i++ {
		if steps[i] < steps[i-1] {
			t.Fatalf("a critically damped scroll must not turn back: %v", steps)
		}
	}
	if tree.NeedsFrame() {
		t.Error("at rest there must be nothing to draw")
	}
}

func TestScroll_SecondNotchAddsToTheMotion(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -1)
	tree.Tick(time.Second)
	tree.Tick(time.Second + 16*time.Millisecond)
	wtest.Wheel(tree, -1)
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := v.Position.Value(); got != 2*kit.NotchDistance {
		t.Errorf("rests at %v", got)
	}
}

func TestScroll_BouncesAtTheEnds(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(400)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -5)
	var most float64
	wtest.RunUntilIdle(t, tree, size, func() { most = max(most, v.Position.Value()) })
	if most <= 100 {
		t.Errorf("must pull past the end, got to %v", most)
	}
	if most > 100+overscroll+1 {
		t.Errorf("pulled too far: %v", most)
	}
	if got := v.Position.Value(); got != 100 {
		t.Errorf("rests at %v, want the end", got)
	}
	wtest.Wheel(tree, 9)
	var least float64
	wtest.RunUntilIdle(t, tree, size, func() { least = min(least, v.Position.Value()) })
	if least >= 0 {
		t.Errorf("must pull past the start, got to %v", least)
	}
	if got := v.Position.Value(); got != 0 {
		t.Errorf("rests at %v, want the start", got)
	}
}

func TestScroll_ShortContentStaysPut(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(100)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -2)
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := v.Position.Value(); got != 0 {
		t.Errorf("rests at %v", got)
	}
}

func TestScroll_OnScrollAndStickToEndDoNotAllocate(t *testing.T) {
	var told ScrollPosition
	tree := wtest.TextTree(t, Scroll{StickToEnd: true, OnScroll: func(p ScrollPosition) { told = p }, Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	var content float64 = 1000
	allocs := testing.AllocsPerRun(20, func() {
		// A step of a touchpad, then content that grows under a view at
		// the end, then a step back.
		v.OnScroll(0.5)
		content += 20
		v.OnMeasured(100, content)
		v.OnScroll(-0.5)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
	if told.MaxOffset != content-100 {
		t.Errorf("OnScroll was last told %+v, want the largest offset %v", told, content-100)
	}
}
