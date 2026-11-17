package gowaku

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/platform"
)

func TestRun_DecorationsReachThePlatform(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var h *WindowHandle
	gray := image.NewGray(image.Rect(0, 0, 64, 64))
	gray.Set(1, 1, color.Gray{Y: 200})
	minSize, maxSize := geom.Size{W: 200, H: 100}, geom.Size{W: 900}
	p, b = newFakes(t,
		func() {
			win := p.windows[1]
			o := win.opts
			if o.AppID != "com.example.Test" || o.MinSize != minSize || o.MaxSize != maxSize || !o.AlwaysOnTop || !o.Fullscreen {
				t.Errorf("platform options %+v miss what App asked for", o)
			}
			if len(o.Icon) != 1 || o.Icon[0].Rect.Dx() != 64 || o.Icon[0].NRGBAAt(1, 1).R != 200 {
				t.Errorf("the icon did not arrive as an NRGBA of the gray image")
			}
			h.SetFullscreen(false)
			h.SetAlwaysOnTop(false)
			h.SetSizeLimits(geom.Size{}, geom.Size{})
			h.SetIcon(nil)
			if win.fullscreen || win.topmost || win.minSize != (geom.Size{}) || win.icon != nil || win.icons != 1 {
				t.Errorf("the setters of the handle did not reach the window")
			}
			if h.Fullscreen() || h.Transparent() {
				t.Errorf("Fullscreen %v, Transparent %v; want both false", h.Fullscreen(), h.Transparent())
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	home := windowGrab{handle: &h}
	a := App{
		Home: home, AppID: "com.example.Test", Icon: []image.Image{gray}, MinSize: minSize, MaxSize: maxSize,
		AlwaysOnTop: true, Fullscreen: true,
	}
	if err := run(p, b, testutil.NewShaper(t), a); err != nil {
		t.Fatal(err)
	}
}

// windowGrab hands out the handle of its window.
type windowGrab struct {
	StatelessBase
	handle **WindowHandle
}

// Build implements StatelessWidget.
func (g windowGrab) Build(ctx BuildContext) Widget {
	*g.handle = WindowOf(ctx)
	return basic.Container{}
}

func TestRun_TransparentWindowClearsToNothing(t *testing.T) {
	half := geom.Color{R: 1, G: 0.5, A: 0.5}
	tests := []struct {
		name       string
		background geom.Color
		opaque     bool
		want       geom.Color
	}{
		{"背景なし: 透明", geom.Color{}, false, geom.Color{}},
		{"半透明の背景: 乗算済み", half, false, geom.Color{R: 0.5, G: 0.25, A: 0.5}},
		{"OS が透過できない: テーマの地", geom.Color{}, true, geom.Color{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, b := newFakes(t)
			p.noAlpha = tt.opaque
			p.script = []func(){closeWindow(p, 1)}
			a := App{Home: counter{}, Transparent: true, Background: tt.background}
			if err := run(p, b, testutil.NewShaper(t), a); err != nil {
				t.Fatal(err)
			}
			want := tt.want
			if tt.opaque {
				want = (&app{}).theme().Color.Canvas
			}
			if b.clear != want {
				t.Errorf("cleared to %v, want %v", b.clear, want)
			}
		})
	}
}

func TestRun_FullscreenHidesTheTitleBar(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var tapped geom.Offset
	resize := func() {
		p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 300, PhysicalHeight: 200, Scale: 2})
	}
	p, b = newFakes(t,
		func() { click(p, 40, 60, 100) },
		func() {
			if tapped != (geom.Offset{X: 40, Y: 24}) {
				t.Errorf("windowed: the content got a tap at %v, want (40, 24) below the bar", tapped)
			}
			p.windows[1].fullscreen = true
			resize()
		},
		func() { click(p, 40, 60, 1000) },
		func() {
			if tapped != (geom.Offset{X: 40, Y: 60}) {
				t.Errorf("fullscreen: the content got a tap at %v, want (40, 60) with no bar", tapped)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	home := basic.Column{Cross: layout.CrossStretch, Children: []Widget{TitleBar{}, basic.Expanded{Child: tapAt(&tapped)}}}
	if err := run(p, b, testutil.NewShaper(t), App{CustomFrame: true, Home: home}); err != nil {
		t.Fatal(err)
	}
}
