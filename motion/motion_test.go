package motion

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// source is a tree with one state, which is all a controller needs.
type source struct {
	core.StatefulBase
	state **sourceState
}

type sourceState struct{ core.StateBase }

func (s source) CreateState() core.State {
	st := &sourceState{}
	*s.state = st
	return st
}

func (*sourceState) Build(core.BuildContext) core.Widget { return leaf{} }

type leaf struct{ core.StatelessBase }

func (leaf) CreateRenderObject() core.RenderObject { return &leafObject{} }
func (leaf) UpdateRenderObject(core.RenderObject)  {}

type leafObject struct{ core.RenderBase }

func (*leafObject) Layout(c geom.Constraints) geom.Size { return c.Biggest() }

func (*leafObject) Paint(*paint.Canvas, geom.Offset) {}

func newSource(t testing.TB) (*core.Tree, *sourceState) {
	t.Helper()
	var s *sourceState
	tree := core.NewTree(source{state: &s})
	t.Cleanup(tree.Dispose)
	return tree, s
}

func near(a, b float32) bool {
	d := a - b
	return d > -1e-3 && d < 1e-3
}

func TestCurves_Ends(t *testing.T) {
	curves := map[string]Curve{
		"Linear": Linear, "EaseInCubic": EaseInCubic, "EaseOutCubic": EaseOutCubic, "EaseInOutCubic": EaseInOutCubic,
		"EaseOutBack": EaseOutBack, "ElasticOut": ElasticOut, "BounceOut": BounceOut,
		"Standard": Standard, "Enter": Enter, "Leave": Leave, "SpringOut": SpringOut,
	}
	for name, c := range curves {
		t.Run(name, func(t *testing.T) {
			if got := c(0); !near(got, 0) {
				t.Errorf("c(0) = %v", got)
			}
			if got := c(1); !near(got, 1) {
				t.Errorf("c(1) = %v", got)
			}
		})
	}
}

func TestCurves_Known(t *testing.T) {
	tests := []struct {
		name    string
		c       Curve
		t, want float32
	}{
		{"EaseOutCubic", EaseOutCubic, 0.5, 0.875},
		{"EaseInOutCubic は中央で 0.5", EaseInOutCubic, 0.5, 0.5},
		// CSS ease-in-out is symmetric, which pins down the solver.
		{"対称な Bézier は中央で 0.5", CubicBezier(0.42, 0, 0.58, 1), 0.5, 0.5},
		// A diagonal control polygon is the identity.
		{"対角線の Bézier は恒等", CubicBezier(1.0/3, 1.0/3, 2.0/3, 2.0/3), 0.3, 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c(tt.t); !near(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurves_BezierRisesOnFlatStart(t *testing.T) {
	// x1 = 0 makes the slope at the start zero, where Newton gives up.
	prev := float32(0)
	for i := 1; i < 100; i++ {
		got := Enter(float32(i) / 100)
		if got < prev {
			t.Fatalf("falls at %d: %v after %v", i, got, prev)
		}
		prev = got
	}
}

func TestEaseOutBack_Overshoots(t *testing.T) {
	if got := EaseOutBack(0.7); got <= 1 {
		t.Errorf("got %v", got)
	}
}

func TestLerpColor_FadeKeepsTheColor(t *testing.T) {
	red := geom.Hex(0xff0000)
	got := LerpColor(red.WithAlpha(0), red, 0.5)
	if !near(got.R, 1) || !near(got.A, 0.5) {
		t.Errorf("got %+v", got)
	}
	// The zero Color is transparent black, and must not darken the fade.
	got = LerpColor(geom.Color{}, red, 0.5)
	if !near(got.R, 1) || !near(got.A, 0.5) {
		t.Errorf("got %+v", got)
	}
}

func TestTween_At(t *testing.T) {
	w := Tween[geom.Offset]{Begin: geom.Offset{X: 10}, End: geom.Offset{X: 20, Y: 4}, Lerp: LerpOffset}
	if got := w.At(0.5); got != (geom.Offset{X: 15, Y: 2}) {
		t.Errorf("got %+v", got)
	}
	s := Tween[geom.Affine]{Begin: geom.Identity(), End: geom.Scaling(3, 3), Lerp: LerpAffine}
	if got := s.At(0.5).A; got != 2 {
		t.Errorf("got %v", got)
	}
}

func TestController_RunsOverItsDuration(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	var statuses []Status
	c.OnStatus = func(st Status) { statuses = append(statuses, st) }
	c.Forward()
	if !tree.Ticking() {
		t.Fatal("a running controller must ask for frames")
	}
	tree.Tick(time.Second) // what starts a run has no clock, so this is its start
	tree.Tick(time.Second + 50*time.Millisecond)
	if got := c.Value(); !near(got, 0.5) {
		t.Errorf("got %v", got)
	}
	tree.Tick(time.Second + 100*time.Millisecond)
	if got := c.Value(); got != 1 {
		t.Errorf("got %v", got)
	}
	if tree.Ticking() {
		t.Error("a finished controller must stop asking for frames")
	}
	if len(statuses) != 2 || statuses[0] != Forwarding || statuses[1] != Completed {
		t.Errorf("got %v", statuses)
	}
}

func TestController_ReverseFromTheMiddle(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	c.Forward()
	tree.Tick(0)
	tree.Tick(50 * time.Millisecond)
	c.Reverse()
	if got := c.Value(); !near(got, 0.5) {
		t.Fatalf("an interrupted motion must not jump, got %v", got)
	}
	tree.Tick(60 * time.Millisecond)
	tree.Tick(85 * time.Millisecond)
	if got := c.Value(); !near(got, 0.25) {
		t.Errorf("half the way must take half the time, got %v", got)
	}
	tree.Tick(110 * time.Millisecond)
	if c.Status() != Dismissed || c.Value() != 0 {
		t.Errorf("got %v at %v", c.Status(), c.Value())
	}
}

func TestController_Repeat(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	c.Repeat(true)
	tree.Tick(0)
	tree.Tick(100 * time.Millisecond)
	tree.Tick(150 * time.Millisecond)
	if got := c.Value(); !near(got, 0.5) {
		t.Errorf("got %v", got)
	}
	c.Stop()
	if tree.Ticking() {
		t.Error("Stop must end the frames")
	}
}

func TestController_StatusCallbackHasTheLastWord(t *testing.T) {
	tests := []struct {
		name string
		// start begins a run, and during is what OnStatus does on its news.
		start, during func(c *AnimationController)
		on            Status
		animating     bool
		status        Status
	}{
		{"Forward の通知で Stop", (*AnimationController).Forward, (*AnimationController).Stop, Forwarding, false, Forwarding},
		{"Forward の通知で Dispose", (*AnimationController).Forward, (*AnimationController).Dispose, Forwarding, false, Forwarding},
		{"Repeat の通知で Stop", func(c *AnimationController) { c.Repeat(true) }, (*AnimationController).Stop, Forwarding, false, Forwarding},
		{"Forward の通知で SetValue", (*AnimationController).Forward, func(c *AnimationController) { c.SetValue(1) }, Forwarding, false, Completed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, s := newSource(t)
			c := AnimationController{Duration: 100 * time.Millisecond}
			c.Attach(&s.StateBase)
			c.OnStatus = func(st Status) {
				if st == tt.on {
					tt.during(&c)
				}
			}
			tt.start(&c)
			if c.Animating() != tt.animating || tree.Ticking() != tt.animating {
				t.Errorf("Animating() = %v, Ticking() = %v, want %v", c.Animating(), tree.Ticking(), tt.animating)
			}
			if c.Status() != tt.status {
				t.Errorf("got %v, want %v", c.Status(), tt.status)
			}
		})
	}
}

func TestController_RunFromStatusCallbackReplacesTheRun(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	c.SetValue(0.5) // Forwarding at rest, so that Reversing is news
	var statuses []Status
	c.OnStatus = func(st Status) {
		statuses = append(statuses, st)
		if st == Reversing {
			c.Forward()
		}
	}
	c.Reverse()
	if !c.Animating() || c.Status() != Forwarding {
		t.Fatalf("got %v, animating %v", c.Status(), c.Animating())
	}
	tree.Tick(0)
	tree.Tick(25 * time.Millisecond)
	if got := c.Value(); !near(got, 0.75) {
		t.Errorf("the run of the callback must be the one that runs, got %v", got)
	}
	tree.Tick(50 * time.Millisecond)
	if c.Value() != 1 || c.Status() != Completed || c.Animating() {
		t.Errorf("got %v at %v", c.Status(), c.Value())
	}
	if len(statuses) != 3 || statuses[0] != Reversing || statuses[1] != Forwarding || statuses[2] != Completed {
		t.Errorf("got %v", statuses)
	}
}

func TestController_ReverseFromRepeatCallbackDoesNotRepeat(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	// Stopped half the way back, so that the Forwarding of Repeat is news.
	c.SetValue(0.5)
	c.Reverse()
	c.Stop()
	c.OnStatus = func(st Status) {
		if st == Forwarding {
			c.Reverse()
		}
	}
	c.Repeat(true)
	if c.Status() != Reversing || !c.Animating() {
		t.Fatalf("got %v, animating %v", c.Status(), c.Animating())
	}
	tree.Tick(0)
	tree.Tick(50 * time.Millisecond)
	if c.Value() != 0 || c.Status() != Dismissed || c.Animating() {
		t.Errorf("a Reverse must end at 0 instead of repeating: %v at %v, animating %v", c.Status(), c.Value(), c.Animating())
	}
}

func TestController_RunFromLastChangeKeepsItsStatus(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	var statuses []Status
	c.OnStatus = func(st Status) { statuses = append(statuses, st) }
	c.OnChange = func() {
		if c.Progress() == 1 && c.Status() != Reversing {
			c.Reverse()
		}
	}
	c.Forward()
	tree.Tick(0)
	tree.Tick(100 * time.Millisecond)
	if c.Status() != Reversing || !c.Animating() {
		t.Fatalf("the status must be that of the new run: %v, animating %v", c.Status(), c.Animating())
	}
	tree.Tick(110 * time.Millisecond)
	tree.Tick(160 * time.Millisecond)
	if got := c.Value(); !near(got, 0.5) {
		t.Errorf("got %v", got)
	}
	if len(statuses) != 2 || statuses[0] != Forwarding || statuses[1] != Reversing {
		t.Errorf("Completed must not come after the run that replaced it, got %v", statuses)
	}
}

func TestController_StopFromLastChangeStillComesToRest(t *testing.T) {
	tree, s := newSource(t)
	c := AnimationController{Duration: 100 * time.Millisecond}
	c.Attach(&s.StateBase)
	c.OnChange = func() {
		if c.Progress() == 1 {
			c.Stop()
		}
	}
	c.Forward()
	tree.Tick(0)
	tree.Tick(100 * time.Millisecond)
	if c.Status() != Completed || c.Animating() {
		t.Errorf("got %v, animating %v", c.Status(), c.Animating())
	}
}

func TestController_RunFromZeroDurationChange(t *testing.T) {
	_, s := newSource(t)
	var c AnimationController
	c.Attach(&s.StateBase)
	c.ReverseDuration = 100 * time.Millisecond
	c.OnChange = func() {
		if c.Progress() == 1 && c.Status() != Reversing {
			c.Reverse()
		}
	}
	c.Forward()
	if c.Status() != Reversing || !c.Animating() {
		t.Errorf("got %v, animating %v", c.Status(), c.Animating())
	}
}

func TestController_ZeroDurationJumps(t *testing.T) {
	tree, s := newSource(t)
	var c AnimationController
	c.Attach(&s.StateBase)
	c.Forward()
	if c.Value() != 1 || c.Status() != Completed || tree.Ticking() {
		t.Errorf("got %v, %v", c.Value(), c.Status())
	}
}

func TestImplicit_RetargetStartsFromTheMoment(t *testing.T) {
	tree, s := newSource(t)
	var m Implicit
	m.Attach(&s.StateBase, func() {})
	v := Track(&m, float32(0), LerpFloat)
	other := Track(&m, float32(10), LerpFloat)

	v.Set(100)
	m.Commit(100*time.Millisecond, nil)
	tree.Tick(0)
	tree.Tick(50 * time.Millisecond)
	if got := v.Get(); !near(got, 50) {
		t.Fatalf("got %v", got)
	}
	v.Set(0)
	other.Set(20)
	m.Commit(100*time.Millisecond, nil)
	if got := v.Get(); !near(got, 50) {
		t.Errorf("a new target must not jump, got %v", got)
	}
	tree.Tick(60 * time.Millisecond)
	tree.Tick(110 * time.Millisecond)
	if got := v.Get(); !near(got, 25) {
		t.Errorf("got %v", got)
	}
	if got := other.Get(); !near(got, 15) {
		t.Errorf("got %v", got)
	}
	tree.Tick(160 * time.Millisecond)
	if v.Get() != 0 || other.Get() != 20 || m.Animating() {
		t.Errorf("got %v, %v", v.Get(), other.Get())
	}
}

func TestImplicit_SameValueDoesNothing(t *testing.T) {
	tree, s := newSource(t)
	var m Implicit
	m.Attach(&s.StateBase, func() {})
	v := Track(&m, float32(5), LerpFloat)
	v.Set(5)
	m.Commit(time.Second, nil)
	if tree.Ticking() {
		t.Error("an unchanged value must not animate")
	}
}

func TestImplicit_ZeroDurationIsSilent(t *testing.T) {
	_, s := newSource(t)
	var m Implicit
	calls := 0
	m.Attach(&s.StateBase, func() { calls++ })
	v := Track(&m, float32(0), LerpFloat)
	v.Set(1)
	// Commit runs during a build, where the callback's SetState would panic.
	m.Commit(0, nil)
	if v.Get() != 1 || calls != 0 {
		t.Errorf("got %v after %d calls", v.Get(), calls)
	}
}

func BenchmarkController_Tick(b *testing.B) {
	tree, s := newSource(b)
	c := AnimationController{Duration: time.Hour, Curve: Standard, OnChange: func() {}}
	c.Attach(&s.StateBase)
	c.Forward()
	b.ReportAllocs()
	var now time.Duration
	for b.Loop() {
		now += time.Millisecond
		tree.Tick(now)
	}
}

func BenchmarkImplicit_Get(b *testing.B) {
	_, s := newSource(b)
	var m Implicit
	m.Attach(&s.StateBase, func() {})
	v := Track(&m, geom.Color{}, LerpColor)
	v.Set(geom.Hex(0x6750a4))
	m.Commit(time.Hour, Standard)
	b.ReportAllocs()
	var sink geom.Color
	for b.Loop() {
		sink = v.Get()
	}
	_ = sink
}

func TestSpringValue_ReachesItsTargetOnAScaleOfOne(t *testing.T) {
	tree, s := newSource(t)
	var v SpringValue
	v.Attach(&s.StateBase, 0)
	v.Set(1)
	var now time.Duration
	for i := 0; tree.Ticking(); i++ {
		if i > 500 {
			t.Fatal("the spring never rests")
		}
		tree.Tick(now)
		if tree.Ticking() && i > 3 && v.Value() > 0.999 {
			break
		}
		now += 8 * time.Millisecond
	}
	for tree.Ticking() {
		now += 8 * time.Millisecond
		tree.Tick(now)
	}
	if v.Value() != 1 {
		t.Errorf("got %v", v.Value())
	}
}

func TestSpringValue_KeepsVelocityAcrossTargets(t *testing.T) {
	tree, s := newSource(t)
	var v SpringValue
	v.Attach(&s.StateBase, 0)
	v.Set(1)
	tree.Tick(0)
	tree.Tick(30 * time.Millisecond)
	at := v.Value()
	v.Set(0)
	if v.Value() != at {
		t.Fatal("a new target must not jump")
	}
	tree.Tick(38 * time.Millisecond)
	tree.Tick(46 * time.Millisecond)
	if v.Value() <= at {
		t.Errorf("the motion must carry on for a moment: %v after %v", v.Value(), at)
	}
}

func TestSpringValue_BouncyOvershoots(t *testing.T) {
	tree, s := newSource(t)
	v := SpringValue{Spring: BouncySpring()}
	v.Attach(&s.StateBase, 0)
	v.Set(1)
	var top float32
	for now := time.Duration(0); tree.Ticking(); now += 8 * time.Millisecond {
		tree.Tick(now)
		top = max(top, v.Value())
	}
	if top <= 1 || top > 1.15 {
		t.Errorf("got a peak of %v", top)
	}
}

func TestSpringValue_KickReturns(t *testing.T) {
	tree, s := newSource(t)
	var v SpringValue
	v.Attach(&s.StateBase, 0)
	v.Kick(8)
	var top float32
	for now := time.Duration(0); tree.Ticking(); now += 8 * time.Millisecond {
		tree.Tick(now)
		top = max(top, v.Value())
	}
	if top <= 0 || v.Value() != 0 {
		t.Errorf("peak %v, end %v", top, v.Value())
	}
}

func BenchmarkSpringValue_Tick(b *testing.B) {
	tree, s := newSource(b)
	var v SpringValue
	v.Spring = Spring{Mass: 1, Stiffness: 1e-6, Damping: 1e-6}
	v.Attach(&s.StateBase, 0)
	v.Set(1)
	b.ReportAllocs()
	var now time.Duration
	for b.Loop() {
		now += time.Millisecond
		tree.Tick(now)
	}
}
