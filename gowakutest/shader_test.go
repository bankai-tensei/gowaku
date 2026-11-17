package gowakutest_test

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// clockShader paints red by the time it sees, from 0 at 0 s to 1 at 1 s, and
// adds the green of what it masks.
func clockShader() *paint.Shader {
	return paint.NewShader(paint.Shader{Name: "clock", Fallback: func(in *paint.ShaderInput) geom.Color {
		c := in.Tex0.Sample(in.UV)
		return geom.Color{R: min(in.Time, 1), G: c.G, A: 1}
	}})
}

// shaderPage shows a CustomPaint and a ShaderMask of a shader, hides them under
// an Opacity of 0 on demand.
type shaderPage struct {
	gowaku.StatefulBase
	shader  *paint.Shader
	animate bool
	state   **shaderPageState
}

type shaderPageState struct {
	gowaku.StateBase
	hidden bool
}

func (p shaderPage) CreateState() gowaku.State {
	s := &shaderPageState{}
	*p.state = s
	return s
}

func (s *shaderPageState) Build(gowaku.BuildContext) gowaku.Widget {
	p := s.Widget().(shaderPage)
	opacity := float32(1)
	if s.hidden {
		opacity = 0
	}
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Opacity{Opacity: opacity, Child: basic.Row{Children: []gowaku.Widget{
		control.CustomPaint{Shader: p.shader, Width: 20, Height: 20, Animate: p.animate},
		basic.ShaderMask{Shader: p.shader, Animate: p.animate, Child: basic.Container{Width: 20, Height: 20, Color: geom.Hex(0x00ff00)}},
	}}}}
}

func newShaderPage(t *testing.T, s *paint.Shader, animate bool, opts gowakutest.Options) (*gowakutest.Tester, *shaderPageState) {
	t.Helper()
	var st *shaderPageState
	opts.Width, opts.Height = 40, 20
	tt := gowakutest.New(t, shaderPage{shader: s, animate: animate, state: &st}, opts)
	return tt, st
}

func findCustomPaint(t *testing.T, tt *gowakutest.Tester) *layout.RenderCustomPaint {
	t.Helper()
	var found *layout.RenderCustomPaint
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if r, ok := ro.(*layout.RenderCustomPaint); ok {
			found = r
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(tt.Tree().RootRenderObject())
	if found == nil {
		t.Fatal("no CustomPaint")
	}
	return found
}

func TestShader_AnimateRunsTheClock(t *testing.T) {
	tt, _ := newShaderPage(t, clockShader(), true, gowakutest.Options{})
	r := findCustomPaint(t, tt)
	start := tt.Now() + gowakutest.FrameTime
	tt.Advance(start + 500*time.Millisecond - tt.Now())
	if got := r.ShaderTime(); got < 490*time.Millisecond || got > 510*time.Millisecond {
		t.Errorf("time %v after 500 ms", got)
	}
	img := tt.Screenshot()
	paintRed, maskRed, maskGreen := img.NRGBAAt(10, 10).R, img.NRGBAAt(30, 10).R, img.NRGBAAt(30, 10).G
	if paintRed < 120 || paintRed > 135 || maskRed != paintRed || maskGreen != 255 {
		t.Errorf("red %d and %d, green %d: the screen does not show the time, or the mask lost its child", paintRed, maskRed, maskGreen)
	}
	if !tt.Tree().Ticking() {
		t.Error("an animated shader asks for no frames")
	}
}

func TestShader_ReduceMotionStopsTheClock(t *testing.T) {
	tt, _ := newShaderPage(t, clockShader(), true, gowakutest.Options{ReduceMotion: true})
	tt.Advance(2 * time.Second)
	if got := findCustomPaint(t, tt).ShaderTime(); got != 0 {
		t.Errorf("time %v, want it to stay at 0", got)
	}
	if _, ok := tt.Tree().NextWake(); ok || tt.Tree().Ticking() {
		t.Error("a still shader keeps a ticker")
	}
}

func TestShader_PausesInAHiddenSubtree(t *testing.T) {
	tt, s := newShaderPage(t, clockShader(), true, gowakutest.Options{})
	r := findCustomPaint(t, tt)
	tt.Advance(300 * time.Millisecond)
	s.SetState(func() { s.hidden = true })
	tt.Advance(time.Second)
	held := r.ShaderTime()
	tt.Advance(5 * time.Second)
	if r.ShaderTime() != held || held > 600*time.Millisecond {
		t.Errorf("time %v, then %v while hidden; want it to hold", held, r.ShaderTime())
	}
	if tt.Tree().Ticking() {
		t.Error("a hidden shader asks for frames")
	}
	s.SetState(func() { s.hidden = false })
	tt.Advance(time.Second)
	if r.ShaderTime() <= held {
		t.Error("the clock does not go on after showing again")
	}
}

func TestShader_StillByDefault(t *testing.T) {
	tt, _ := newShaderPage(t, clockShader(), false, gowakutest.Options{})
	tt.Advance(time.Second)
	if tt.Tree().Ticking() {
		t.Error("a shader without Animate asks for frames")
	}
}

func TestWarnings_ShaderWithoutFallback(t *testing.T) {
	gpuOnly := paint.NewShader(paint.Shader{Name: "gpu-only"})
	tt, _ := newShaderPage(t, gpuOnly, false, gowakutest.Options{})
	var shaders []gowakutest.Warning
	for _, w := range tt.Warnings() {
		if w.Kind == gowakutest.WarnShader {
			shaders = append(shaders, w)
		}
	}
	if len(shaders) != 2 || shaders[0].Text != "gpu-only" {
		t.Fatalf("warnings %v, want one for the CustomPaint and one for the ShaderMask", shaders)
	}
	if shaders[1].Rect != (geom.Rect{X: 20, W: 20, H: 20}) {
		t.Errorf("the mask is at %v", shaders[1].Rect)
	}
	tt2, _ := newShaderPage(t, clockShader(), false, gowakutest.Options{})
	for _, w := range tt2.Warnings() {
		if w.Kind == gowakutest.WarnShader {
			t.Errorf("a shader with a Fallback: %v", w)
		}
	}
}
