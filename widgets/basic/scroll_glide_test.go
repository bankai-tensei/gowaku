package basic

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
)

// TestScroll_WheelAnswersAtOnce pins the feel of the wheel at 60 frames a
// second: a notch shows on the first frame, is mostly there within 100 ms, and
// rests well within a quarter of a second. A spring from rest, used before,
// moved 0 px on the first frame and 3 px on the second.
func TestScroll_WheelAnswersAtOnce(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(3000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -1)
	var at []float64
	for now := time.Second; tree.Ticking() && len(at) < 60; now += time.Second / 60 {
		tree.Tick(now)
		testutil.RenderTree(tree, size)
		at = append(at, v.Position.Value())
	}
	if at[0] < 0.25*kit.NotchDistance {
		t.Errorf("first frame at %v px, want at least a quarter of the notch", at[0])
	}
	if at[6] < 0.9*kit.NotchDistance {
		t.Errorf("100 ms in at %v px, want 90%% of the notch", at[6])
	}
	if len(at) > 16 || at[len(at)-1] != kit.NotchDistance {
		t.Errorf("rests after %d frames at %v px, want %v within 16", len(at), at[len(at)-1], kit.NotchDistance)
	}
	// The last frame snaps the final fraction of a pixel.
	for i := 1; i < len(at)-1; i++ {
		if at[i]-at[i-1] > at[i-1]-prev(at, i-2)+1e-3 {
			t.Fatalf("the glide speeds up at frame %d: %v", i, at)
		}
	}
}

// prev returns at[i], or 0 before the start.
func prev(at []float64, i int) float64 {
	if i < 0 {
		return 0
	}
	return at[i]
}

// TestScroll_TouchpadFollowsAtOnce: the small steps of a touchpad move the
// content by exactly their distance at once, with no motion left to run, and
// stop at the ends without a bounce.
func TestScroll_TouchpadFollowsAtOnce(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	for i := range 10 {
		wtest.Wheel(tree, -0.1)
		// No tick: the step is there before any motion could run. (The scroll
		// bar fading in may still tick.)
		testutil.RenderTree(tree, size)
		if got, want := v.Position.Value(), float64(i+1)*kit.NotchDistance/10; got < want-0.01 || got > want+0.01 {
			t.Fatalf("step %d at %v px, want %v at once", i, got, want)
		}
	}
	if got := v.Position.Value(); got < 99.9 || got > 100.1 {
		t.Errorf("ten steps of a tenth of a notch moved %v px, want %v", got, kit.NotchDistance)
	}
	for range 100 {
		wtest.Wheel(tree, -0.5)
	}
	testutil.RenderTree(tree, size)
	if got := v.Position.Value(); got != 700 {
		t.Errorf("past the end at %v, want the end 700", got)
	}
}

// TestScroll_TouchpadAfterWheelKeepsTheNotch: a touchpad step during a glide
// starts from the glide's target, so the notch still counts.
func TestScroll_TouchpadAfterWheelKeepsTheNotch(t *testing.T) {
	tree := wtest.TextTree(t, Scroll{Child: tallChild(1000)})
	size := geom.Size{W: 200, H: 300}
	testutil.RenderTree(tree, size)
	v := viewportOf(t, tree)
	wtest.Wheel(tree, -1)
	tree.Tick(time.Second)
	wtest.Wheel(tree, -0.25)
	testutil.RenderTree(tree, size)
	if got := v.Position.Value(); got != 125 {
		t.Errorf("at %v, want 125", got)
	}
}
