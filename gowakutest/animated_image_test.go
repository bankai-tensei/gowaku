package gowakutest_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

const ms = time.Millisecond

// frameColors are the colors of the frames of colorAnimation.
var frameColors = []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}}

// colorAnimation shows red for 100 ms, green for 200 ms and blue for 300 ms,
// 20x20 each, loops times.
func colorAnimation(loops int) *paint.Animation {
	a := &paint.Animation{Loops: loops}
	for i, c := range frameColors {
		m := image.NewRGBA(image.Rect(0, 0, 20, 20))
		for p := 0; p < len(m.Pix); p += 4 {
			m.Pix[p], m.Pix[p+1], m.Pix[p+2], m.Pix[p+3] = c.R, c.G, c.B, c.A
		}
		a.Frames = append(a.Frames, paint.NewImage(m))
		a.Delays = append(a.Delays, time.Duration(i+1)*100*ms)
	}
	return a
}

// player shows an animation, hides it under an Opacity of 0 or takes it out of
// the tree on demand.
type player struct {
	gowaku.StatefulBase
	anim  *paint.Animation
	state **playerState
}

type playerState struct {
	gowaku.StateBase
	hidden, gone bool
}

func (p player) CreateState() gowaku.State {
	s := &playerState{}
	*p.state = s
	return s
}

func (s *playerState) Build(gowaku.BuildContext) gowaku.Widget {
	if s.gone {
		return basic.SizedBox{Width: 20, Height: 20}
	}
	opacity := float32(1)
	if s.hidden {
		opacity = 0
	}
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Opacity{
		Opacity: opacity,
		Child:   basic.AnimatedImage{Source: s.Widget().(player).anim, Label: "clip"},
	}}
}

func newPlayer(t *testing.T, loops int, opts gowakutest.Options) (*gowakutest.Tester, *playerState, *layout.RenderAnimatedImage) {
	t.Helper()
	var s *playerState
	opts.Width, opts.Height = 40, 40
	tt := gowakutest.New(t, player{anim: colorAnimation(loops), state: &s}, opts)
	return tt, s, findAnimated(t, tt)
}

func findAnimated(t *testing.T, tt *gowakutest.Tester) *layout.RenderAnimatedImage {
	t.Helper()
	var found *layout.RenderAnimatedImage
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if r, ok := ro.(*layout.RenderAnimatedImage); ok {
			found = r
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(tt.Tree().RootRenderObject())
	if found == nil {
		t.Fatal("no animated image")
	}
	return found
}

// shownColor returns the color in the middle of the picture on screen.
func shownColor(tt *gowakutest.Tester) color.NRGBA {
	return tt.Screenshot().NRGBAAt(10, 10)
}

func TestAnimatedImage_FramesAdvanceAtTheirDelays(t *testing.T) {
	tt, _, r := newPlayer(t, paint.LoopForever, gowakutest.Options{})
	// The first tick, a frame after the first paint, starts the clock.
	start := tt.Now() + gowakutest.FrameTime
	steps := []struct {
		at    time.Duration
		frame int
	}{
		{99 * ms, 0}, {100 * ms, 1}, {299 * ms, 1}, {300 * ms, 2}, {599 * ms, 2}, {600 * ms, 0}, {700 * ms, 1},
	}
	for _, s := range steps {
		tt.Advance(start + s.at - tt.Now())
		if r.Frame() != s.frame {
			t.Fatalf("at %v: frame %d, want %d", s.at, r.Frame(), s.frame)
		}
		want := frameColors[s.frame]
		if got := shownColor(tt); got.R != want.R || got.G != want.G || got.B != want.B {
			t.Fatalf("at %v: the screen shows %v, want %v", s.at, got, want)
		}
	}
}

func TestAnimatedImage_LoopsEndOnTheLastFrame(t *testing.T) {
	tt, _, r := newPlayer(t, 2, gowakutest.Options{})
	tt.Advance(1300 * ms)
	if r.Frame() != 2 {
		t.Errorf("frame %d after two plays, want the last", r.Frame())
	}
	if tt.Tree().Ticking() {
		t.Error("the ticker still runs")
	}
	if _, ok := tt.Tree().NextWake(); ok {
		t.Error("the ticker still sleeps, instead of stopping")
	}
	tt.Advance(time.Second)
	if r.Frame() != 2 {
		t.Errorf("frame %d later", r.Frame())
	}
}

func TestAnimatedImage_ReduceMotionHoldsTheFirstFrame(t *testing.T) {
	tt, _, r := newPlayer(t, paint.LoopForever, gowakutest.Options{ReduceMotion: true})
	tt.Advance(2 * time.Second)
	if r.Frame() != 0 {
		t.Errorf("frame %d, want the first", r.Frame())
	}
	if _, ok := tt.Tree().NextWake(); ok || tt.Tree().Ticking() {
		t.Error("a still animation keeps a ticker")
	}
	if got := shownColor(tt); got.R != 255 || got.G != 0 {
		t.Errorf("the screen shows %v", got)
	}
}

func TestAnimatedImage_PausesInAHiddenSubtree(t *testing.T) {
	tt, s, r := newPlayer(t, paint.LoopForever, gowakutest.Options{})
	start := tt.Now() + gowakutest.FrameTime
	tt.Advance(start + 150*ms - tt.Now())
	if r.Frame() != 1 {
		t.Fatalf("frame %d", r.Frame())
	}
	s.SetState(func() { s.hidden = true })
	tt.Advance(5 * time.Second)
	if r.Frame() != 1 {
		t.Errorf("frame %d while hidden, want it to hold", r.Frame())
	}
	if tt.Tree().Ticking() {
		t.Error("a hidden animation asks for frames")
	}
	if wake, ok := tt.Tree().NextWake(); !ok || wake-tt.Now() > 200*ms {
		t.Errorf("the hidden animation looks again at %v, %v", wake, ok)
	}
	// It noticed at its next wake, 300 ms in, and goes on from there within
	// 200 ms of showing again.
	s.SetState(func() { s.hidden = false })
	tt.Advance(250 * ms)
	if r.Frame() != 2 {
		t.Errorf("frame %d after showing again", r.Frame())
	}
	// 600 ms after showing, it has played 700 to 900 ms: the second play.
	tt.Advance(350 * ms)
	if r.Frame() != 0 && r.Frame() != 1 {
		t.Errorf("frame %d 600 ms after showing again", r.Frame())
	}
}

func TestAnimatedImage_StopsWhenRemoved(t *testing.T) {
	tt, s, r := newPlayer(t, paint.LoopForever, gowakutest.Options{})
	tt.Advance(150 * ms)
	s.SetState(func() { s.gone = true })
	tt.Advance(time.Second)
	if r.Attached() {
		t.Fatal("still in the tree")
	}
	if _, ok := tt.Tree().NextWake(); ok || tt.Tree().Ticking() {
		t.Error("the ticker of a removed animation still runs")
	}
}
