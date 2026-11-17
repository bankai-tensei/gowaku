package nav

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
)

// navRig is a Navigator in a window, driven frame by frame.
type navRig struct {
	tree *core.Tree
	ctl  *NavigatorController
	size geom.Size
	now  time.Duration
	cv   paint.Canvas
}

func newNavRig(t testing.TB, reduce bool, home core.Widget, routes []RoutePattern) *navRig {
	t.Helper()
	r := &navRig{ctl: &NavigatorController{}, size: geom.Size{W: 400, H: 300}, now: time.Second}
	r.tree = wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: reduce}, Child: overlay.Overlay{Child: Navigator{
		Home: home, Routes: routes, Controller: r.ctl,
	}}})
	r.tree.SetWindowFocused(true)
	r.frame()
	return r
}

func (r *navRig) frame() {
	r.tree.Tick(r.now)
	r.tree.Build()
	r.tree.Layout(geom.Tight(r.size))
	for r.tree.NeedsBuild() {
		r.tree.Build()
		r.tree.Layout(geom.Tight(r.size))
	}
	r.tree.Repaint(&r.cv)
}

// advance draws frames for d.
func (r *navRig) advance(d time.Duration) {
	for end := r.now + d; r.now < end; {
		r.now += 16 * time.Millisecond
		r.frame()
	}
}

func (r *navRig) settle(t testing.TB) {
	t.Helper()
	for range 1000 {
		if !r.tree.Ticking() && !r.tree.NeedsFrame() {
			return
		}
		r.advance(16 * time.Millisecond)
	}
	t.Fatal("the navigator never comes to rest")
}

func (r *navRig) state() *navigatorState { return r.ctl.s }

// heroPage is a page with a Hero of tag at a place and size.
func heroPage(tag string, x, y, side float32) core.Widget {
	return basic.Stack{Fit: 1, Children: []core.Widget{
		basic.Positioned{Left: x, Top: y, Child: Hero{Tag: tag, Child: basic.Container{Width: side, Height: side, Color: geom.RGBA(1, 0, 0, 1)}}},
	}}
}

func TestPageTransition_SlidesAndFadesOverThePageBelow(t *testing.T) {
	r := newNavRig(t, false, basic.Text{Content: "home"}, nil)
	s := r.state()
	r.ctl.Push(PageRoute{Name: "/two", Page: basic.Text{Content: "two"}})
	r.frame()
	home, two := s.entries[0], s.entries[1]
	if a := two.slot.ChildAt(0).(*core.RenderRepaintBoundary).Opacity(); a != 0 {
		t.Errorf("the first frame of a push shows the page at opacity %v, want 0", a)
	}
	r.advance(64 * time.Millisecond)
	if two.shift <= 0 || two.shift >= pageSlide {
		t.Errorf("mid-way the page is shifted by %v, want between 0 and %v", two.shift, pageSlide)
	}
	b := two.slot.ChildAt(0).(*core.RenderRepaintBoundary)
	if a := b.Opacity(); a <= 0 || a >= 1 {
		t.Errorf("mid-way the page has opacity %v", a)
	}
	if !home.painted || home.active || !two.active {
		t.Errorf("the page below shows but takes no input: painted %v active %v", home.painted, home.active)
	}
	r.settle(t)
	if home.painted || two.shift != 0 || b.Opacity() != 1 {
		t.Errorf("in place the page covers the one below: painted %v, shift %v, opacity %v", home.painted, two.shift, b.Opacity())
	}

	r.ctl.Pop()
	r.advance(48 * time.Millisecond)
	if !two.exiting || two.active || !home.active || !home.painted || two.shift <= 0 {
		t.Errorf("on the way out the page slides back over the one below: shift %v", two.shift)
	}
	r.settle(t)
	if len(s.entries) != 1 {
		t.Errorf("the page is gone once out: %d pages", len(s.entries))
	}
}

func TestPageTransition_ReduceMotionKeepsTheFadeOnly(t *testing.T) {
	r := newNavRig(t, true, heroPage("a", 10, 10, 20), nil)
	s := r.state()
	r.ctl.Push(PageRoute{Page: heroPage("a", 200, 100, 80)})
	r.frame()
	r.advance(64 * time.Millisecond)
	two := s.entries[1]
	b := two.slot.ChildAt(0).(*core.RenderRepaintBoundary)
	if two.shift != 0 {
		t.Errorf("less motion: shift %v, want none", two.shift)
	}
	if a := b.Opacity(); a <= 0 || a >= 1 {
		t.Errorf("less motion keeps the fade: opacity %v", a)
	}
	if len(two.flights) != 0 {
		t.Error("less motion: no hero flies")
	}
}

func TestPageTransition_InterruptedPushGoesBackWithoutAJump(t *testing.T) {
	r := newNavRig(t, false, basic.Text{Content: "home"}, nil)
	s := r.state()
	r.ctl.Push(PageRoute{Page: basic.Text{Content: "two"}})
	r.frame()
	r.advance(48 * time.Millisecond)
	two := s.entries[1]
	before := two.controller.Value()
	r.ctl.Pop()
	if after := two.controller.Value(); after != before {
		t.Errorf("value %v jumped to %v", before, after)
	}
	r.settle(t)
	if len(s.entries) != 1 {
		t.Error("the page goes")
	}
}

func TestHero_FliesBetweenThePages(t *testing.T) {
	r := newNavRig(t, false, heroPage("note", 10, 20, 20), nil)
	s := r.state()
	r.ctl.Push(PageRoute{Page: heroPage("note", 200, 100, 80)})
	r.frame()
	two := s.entries[1]
	if len(two.flights) != 1 {
		t.Fatalf("got %d flights, want 1", len(two.flights))
	}
	f := two.flights[0]
	if f.from.flying == 0 || f.to.flying == 0 {
		t.Error("both heroes hide while they fly")
	}
	from, to := s.heroRect(f.from), s.heroRect(f.to)
	if from != (geom.Rect{X: 10, Y: 20, W: 20, H: 20}) || to != (geom.Rect{X: 200, Y: 100, W: 80, H: 80}) {
		t.Errorf("from %v to %v", from, to)
	}
	mid := flightRect(from, to, 0.5)
	if mid != (geom.Rect{X: 105, Y: 60, W: 50, H: 50}) {
		t.Errorf("half-way %v", mid)
	}
	r.settle(t)
	if len(two.flights) != 0 || f.from.flying != 0 || f.to.flying != 0 {
		t.Error("the heroes land when the page is in place")
	}

	r.ctl.Pop()
	if len(two.flights) != 1 {
		t.Fatal("the heroes fly back on the way out")
	}
	r.settle(t)
	if f.from.flying != 0 {
		t.Error("and land again")
	}
}

func TestHero_PaintsInFlightOverThePages(t *testing.T) {
	r := newNavRig(t, false, heroPage("note", 10, 20, 20), nil)
	r.ctl.Push(PageRoute{Page: heroPage("note", 200, 100, 80)})
	r.frame()
	r.advance(80 * time.Millisecond)
	r.tree.MarkNeedsPaint()
	r.tree.Repaint(&r.cv)
	// The heroes are red squares; in flight two of them are painted over the
	// layers of the pages, between the two places.
	var flying int
	for _, op := range r.cv.DisplayList().Ops {
		if op.Kind == paint.OpRRect && op.Color.R == 1 && op.Color.G == 0 {
			flying++
			if op.RRect.Rect.X <= 10 || op.RRect.Rect.X >= 200 {
				t.Errorf("a hero in flight at %v", op.RRect.Rect)
			}
		}
	}
	if flying != 2 {
		t.Errorf("got %d heroes painted over the pages, want the two of the crossfade", flying)
	}
}

func TestHero_LiftsOffWhereAScrolledListShowsIt(t *testing.T) {
	list := basic.Scroll{Child: basic.Column{Children: []core.Widget{
		basic.SizedBox{Height: 200},
		Hero{Tag: 7, Child: basic.Container{Width: 40, Height: 40, Color: geom.RGBA(1, 0, 0, 1)}},
		basic.SizedBox{Height: 600},
	}}}
	r := newNavRig(t, false, list, nil)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 50, Y: 50}, Scroll: geom.Offset{Y: -2}})
	r.settle(t)
	r.ctl.Push(PageRoute{Page: basic.Center{Child: Hero{Tag: 7, Child: basic.Container{Width: 80, Height: 80}}}})
	r.frame()
	two := r.state().entries[1]
	if len(two.flights) != 1 {
		t.Fatal("no flight")
	}
	if y := r.state().heroRect(two.flights[0].from).Y; y >= 200 {
		t.Errorf("the hero lifts off at %v, where the list shows it, not at 200", y)
	}
}

func TestHero_DuplicateTagsPanic(t *testing.T) {
	r := newNavRig(t, false, basic.Column{Children: []core.Widget{
		Hero{Tag: "x", Child: basic.SizedBox{Width: 10, Height: 10}},
		Hero{Tag: "x", Child: basic.SizedBox{Width: 10, Height: 10}},
	}}, nil)
	wtest.ExpectPanic(t, "two Heroes on one page", func() {
		r.ctl.Push(PageRoute{Page: Hero{Tag: "x", Child: basic.SizedBox{Width: 10, Height: 10}}})
		r.frame()
	})
}

func TestNavigator_UnboundedPanics(t *testing.T) {
	wtest.ExpectPanic(t, "Navigator has an unbounded size", func() {
		tree := wtest.TextTree(t, basic.Column{Children: []core.Widget{Navigator{Home: basic.SizedBox{}}}})
		tree.Build()
		tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	})
}

func TestNavigator_LowerPagesAreOutOfSemanticsAndHitTesting(t *testing.T) {
	var taps int
	home := interact.GestureDetector{OnTap: func() { taps++ }, Child: basic.SizedBox{Width: 400, Height: 300}}
	r := newNavRig(t, false, home, nil)
	r.ctl.Push(PageRoute{Page: basic.Text{Content: "two"}})
	r.frame()
	at := geom.Offset{X: 100, Y: 100}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonPrimary})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonPrimary})
	if taps != 0 {
		t.Error("the page below takes no taps, even while the new one comes in")
	}
	r.settle(t)
	r.ctl.Pop()
	r.frame()
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonPrimary})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonPrimary})
	if taps != 1 {
		t.Error("the page that comes back takes taps at once, while the other one leaves")
	}
}

// transitionRig is a push held in the middle of its motion, with a hero in
// flight.
func transitionRig(t testing.TB) *navRig {
	t.Helper()
	r := newNavRig(t, false, heroPage("note", 10, 20, 20), nil)
	r.ctl.Push(PageRoute{Page: heroPage("note", 200, 100, 80)})
	r.frame()
	e := r.state().top()
	if e.controller.Status() != motion.Forwarding || len(e.flights) != 1 {
		t.Fatal("the push must be moving, with a hero in flight")
	}
	// A slow motion back and forth keeps the frames in the middle.
	e.controller.Duration = 2 * time.Second
	e.controller.Repeat(true)
	r.advance(500 * time.Millisecond)
	return r
}

func TestPageTransition_FramesDoNotAllocate(t *testing.T) {
	r := transitionRig(t)
	allocs := testing.AllocsPerRun(100, func() { r.advance(16 * time.Millisecond) })
	if allocs != 0 {
		t.Errorf("a frame of a page transition with a hero in flight allocates %v times", allocs)
	}
}

// BenchmarkPageTransition_Frame is one frame of a push in motion with a hero
// in flight: tick, build, layout and paint. The pages are layers, so the frame
// only moves and fades them.
func BenchmarkPageTransition_Frame(b *testing.B) {
	r := transitionRig(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.advance(16 * time.Millisecond)
	}
}
