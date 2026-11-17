package wtest

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
)

// TextTree returns a tree for w with a real shaper, disposed when the test
// ends.
func TextTree(t testing.TB, w core.Widget) *core.Tree {
	t.Helper()
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), w))
	t.Cleanup(tree.Dispose)
	return tree
}

// A11yTree renders w once at 400×300 and returns its tree.
func A11yTree(t *testing.T, w core.Widget) *core.Tree {
	t.Helper()
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), w))
	t.Cleanup(tree.Dispose)
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	return tree
}

// Advance runs the frames of d from *now in 8 ms steps.
func Advance(tree *core.Tree, size geom.Size, now *time.Duration, d time.Duration) {
	for end := *now + d; *now < end; *now += 8 * time.Millisecond {
		tree.Tick(*now)
		testutil.RenderTree(tree, size)
	}
}

// Settled runs frames from *now until nothing ticks.
func Settled(t testing.TB, tree *core.Tree, size geom.Size, now *time.Duration) {
	t.Helper()
	for i := 0; tree.Ticking(); i++ {
		if i > 2000 {
			t.Fatal("the motion never ends")
		}
		tree.Tick(*now)
		testutil.RenderTree(tree, size)
		*now += 8 * time.Millisecond
	}
}

// RunUntilIdle runs frames until nothing ticks, calling visit after each, and
// returns how many it ran.
func RunUntilIdle(t testing.TB, tree *core.Tree, size geom.Size, visit func()) int {
	t.Helper()
	frames := 0
	for now := time.Second; tree.Ticking(); now += 8 * time.Millisecond {
		tree.Tick(now)
		testutil.RenderTree(tree, size)
		if visit != nil {
			visit()
		}
		frames++
		if frames > 2000 {
			t.Fatal("the motion never ends")
		}
	}
	return frames
}

// SwapTree returns a text tree showing first and a function that replaces
// what it shows.
func SwapTree(t testing.TB, first core.Widget) (*core.Tree, func(core.Widget)) {
	t.Helper()
	var state *HolderState
	tree := TextTree(t, Holder{First: first, State: &state})
	return tree, func(w core.Widget) { state.SetState(func() { state.Child = w }) }
}

// Holder shows First until its state is given another child.
type Holder struct {
	core.StatefulBase
	First core.Widget
	State **HolderState
}

// HolderState is the state of a Holder.
type HolderState struct {
	core.StateBase
	Child core.Widget
}

// CreateState implements core.StatefulWidget.
func (h Holder) CreateState() core.State {
	*h.State = &HolderState{Child: h.First}
	return *h.State
}

// Build implements core.State.
func (s *HolderState) Build(core.BuildContext) core.Widget { return s.Child }

// Swap rebuilds through Build each time its state is set.
type Swap struct {
	core.StatefulBase
	Build func() core.Widget
	State **SwapState
}

// SwapState is the state of a Swap.
type SwapState struct{ core.StateBase }

// CreateState implements core.StatefulWidget.
func (w Swap) CreateState() core.State {
	s := &SwapState{}
	*w.State = s
	return s
}

// Build implements core.State.
func (s *SwapState) Build(core.BuildContext) core.Widget { return s.Widget().(Swap).Build() }

// Capture stores the BuildContext it was built in.
type Capture struct {
	core.StatelessBase
	Got   *core.BuildContext
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (c Capture) Build(ctx core.BuildContext) core.Widget {
	*c.Got = ctx
	return c.Child
}

// WaitUntil waits, in real time, for work posted through
// core.StateBase.Poster until done reports true: it ticks and renders a frame
// at once and each time work is posted, and fails the test after 10 seconds.
func WaitUntil(t testing.TB, tree *core.Tree, size geom.Size, done func() bool) {
	t.Helper()
	posted := make(chan struct{}, 1)
	tree.SetWake(func() {
		select {
		case posted <- struct{}{}:
		default:
		}
	})
	defer tree.SetWake(nil)
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		tree.Tick(time.Second)
		testutil.RenderTree(tree, size)
		if done() {
			return
		}
		select {
		case <-posted:
		case <-deadline.C:
			t.Fatal("wtest: WaitUntil gave up after 10 seconds")
		}
	}
}
