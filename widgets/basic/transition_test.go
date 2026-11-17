package basic_test

import (
	"image/color"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

var motionLinear = motion.Linear

var red = geom.RGBA(1, 0, 0, 1)

func isRed(c color.NRGBA) bool { return c.R > 200 && c.G < 60 && c.B < 60 }

func TestTranslate_MovesThePictureBySize(t *testing.T) {
	box := basic.Container{Width: 50, Height: 50, Color: red}
	tt := gowakutest.New(t, basic.Align{Child: basic.Translate{X: 1, Child: box}}, pressableFonts)
	img := tt.Screenshot()
	if isRed(img.NRGBAAt(25, 25)) || !isRed(img.NRGBAAt(75, 25)) {
		t.Errorf("X: 1 must move the box one width right: at 25 %v, at 75 %v", img.NRGBAAt(25, 25), img.NRGBAAt(75, 25))
	}
	clipped := gowakutest.New(t, basic.Align{Child: basic.Translate{X: 1, Clip: true, Child: box}}, pressableFonts)
	if img := clipped.Screenshot(); isRed(img.NRGBAAt(75, 25)) || isRed(img.NRGBAAt(25, 25)) {
		t.Error("a clipped Translate must not paint outside its box")
	}
}

func TestSlideTransition_MovesInAndOutOppositeWays(t *testing.T) {
	slide := basic.SlideTransition(0.1, 0)
	in := slide(basic.SizedBox{}, 0.25, true).(basic.Opacity)
	out := slide(basic.SizedBox{}, 0.25, false).(basic.Opacity)
	if in.Opacity != 0.25 || out.Opacity != 0.25 {
		t.Errorf("opacity in %v, out %v", in.Opacity, out.Opacity)
	}
	if x := in.Child.(basic.Translate).X; x < 0.0749 || x > 0.0751 {
		t.Errorf("entering at %v, want 0.075 (still to the right)", x)
	}
	if x := out.Child.(basic.Translate).X; x > -0.0749 || x < -0.0751 {
		t.Errorf("leaving at %v, want -0.075 (already to the left)", x)
	}
	if x := slide(basic.SizedBox{}, 1, true).(basic.Opacity).Child.(basic.Translate).X; x != 0 {
		t.Errorf("at rest at %v, want 0", x)
	}
}

// pager switches its page through an AnimatedSwitcher with a recording
// Transition.
type pager struct {
	core.StatefulBase
	transition basic.SwitcherTransition
	next       *func()
}

type pagerState struct {
	core.StateOf[pager]
	page int
}

func (pager) CreateState() core.State { return &pagerState{} }

func (s *pagerState) InitState() {
	*s.Widget().next = func() { s.SetState(func() { s.page++ }) }
}

func (s *pagerState) Build(core.BuildContext) core.Widget {
	return basic.AnimatedSwitcher{
		Duration:   200 * time.Millisecond,
		Transition: s.Widget().transition,
		Child:      core.Keyed(s.page, page{}),
	}
}

// page counts how often its State is made, to catch a remount mid-motion.
type page struct{ core.StatefulBase }

type pageState struct{ core.StateBase }

var pagesMade int

func (page) CreateState() core.State { pagesMade++; return &pageState{} }

func (*pageState) Build(core.BuildContext) core.Widget {
	return basic.SizedBox{Width: 10, Height: 10}
}

func TestAnimatedSwitcher_TransitionSeesBothChildren(t *testing.T) {
	type call struct {
		t        float32
		entering bool
	}
	var calls []call
	record := func(child core.Widget, v float32, entering bool) core.Widget {
		calls = append(calls, call{v, entering})
		return basic.Translate{X: 1 - v, Child: child}
	}
	var next func()
	pagesMade = 0
	tt := gowakutest.New(t, pager{transition: record, next: &next}, pressableFonts)
	calls = nil
	next()
	tt.Advance(100 * time.Millisecond)
	var in, out int
	for _, c := range calls {
		if c.entering {
			in++
		} else {
			out++
		}
	}
	if in == 0 || out == 0 {
		t.Fatalf("mid-motion the transition saw %d entering and %d leaving calls", in, out)
	}
	tt.Settle()
	next()
	tt.Settle()
	if pagesMade != 3 {
		t.Errorf("made %d pages, want 3: the first and one per switch, none remade mid-motion", pagesMade)
	}
}

func TestAnimatedSwitcher_DefaultIsAFade(t *testing.T) {
	w := basic.FadeTransition(basic.SizedBox{}, 0.4, true)
	if o, ok := w.(basic.Opacity); !ok || o.Opacity != 0.4 {
		t.Errorf("got %#v", w)
	}
}

// colorPager switches between two full-size colored pages with a slide of
// half a width, to look at the picture mid-motion.
type colorPager struct {
	core.StatefulBase
	next *func()
}

type colorPagerState struct {
	core.StateOf[colorPager]
	page int
}

func (colorPager) CreateState() core.State { return &colorPagerState{} }

func (s *colorPagerState) InitState() {
	*s.Widget().next = func() { s.SetState(func() { s.page++ }) }
}

func (s *colorPagerState) Build(core.BuildContext) core.Widget {
	colors := []geom.Color{red, geom.RGBA(0, 0, 1, 1)}
	return basic.AnimatedSwitcher{
		Duration:   200 * time.Millisecond,
		Curve:      motionLinear,
		Transition: basic.SlideTransition(0.5, 0),
		Child:      core.Keyed(s.page, basic.Container{Width: 200, Height: 100, Color: colors[s.page%2]}),
	}
}

func TestSlideTransition_PicturesMidMotion(t *testing.T) {
	var next func()
	tt := gowakutest.New(t, colorPager{next: &next}, gowakutest.Options{Width: 200, Height: 100, Scale: 1, Fonts: pressableFonts.Fonts})
	next()
	tt.Advance(100 * time.Millisecond)
	img := tt.Screenshot()
	// Halfway, the blue page is a quarter width right of its place and half
	// faded in, the red one a quarter width left and half faded out: the left
	// edge shows only red over the background, the right edge only blue.
	left, right := img.NRGBAAt(10, 50), img.NRGBAAt(190, 50)
	if left.R <= left.B || right.B <= right.R {
		t.Errorf("mid-motion left %v, right %v: want red leaving left and blue coming from the right", left, right)
	}
	tt.Settle()
	if c := tt.Screenshot().NRGBAAt(10, 50); c.B < 200 || c.R > 60 {
		t.Errorf("at rest %v, want blue", c)
	}
}

// pressableFonts keeps the tests independent of the fonts of the machine.
var pressableFonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}
