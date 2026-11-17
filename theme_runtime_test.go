package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/theme"
)

// themeRecorder records the theme of each build, and hands out its context.
type themeRecorder struct {
	StatelessBase
	seen *[]*theme.Theme
	ctx  *BuildContext
}

func (r themeRecorder) Build(ctx BuildContext) Widget {
	*r.seen = append(*r.seen, basic.ThemeOf(ctx))
	if r.ctx != nil {
		*r.ctx = ctx
	}
	return basic.SizedBox{}
}

func last(seen []*theme.Theme) *theme.Theme {
	if len(seen) == 0 {
		return nil
	}
	return seen[len(seen)-1]
}

func TestSetTheme_RebuildsEveryWindowAndFollowsTheOSAgain(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var first, second []*theme.Theme
	red := theme.FromAccent(geom.Hex(0xc0392b))
	wake := func() { p.send(platform.Event{Kind: platform.EventWake}) }
	p, b = newFakes(t,
		func() {
			if _, err := OpenWindow(WindowOptions{}, themeRecorder{seen: &second}); err != nil {
				t.Fatal(err)
			}
			SetTheme(red, ThemeDark)
			wake()
		},
		func() {
			if last(first) != red.Dark || last(second) != red.Dark {
				t.Errorf("every window must take the new theme, got %p and %p", last(first), last(second))
			}
			for id, w := range p.windows {
				if !w.dark {
					t.Errorf("window %d must get a dark title bar", id)
				}
			}
			if b.clear != red.Dark.Color.Canvas {
				t.Errorf("the background must follow, got %+v", b.clear)
			}
			if set, mode := CurrentTheme(); set != red || mode != ThemeDark {
				t.Errorf("CurrentTheme: got %v %v", set, mode)
			}
			// Back to the OS, which is light.
			SetTheme(red, ThemeSystem)
			wake()
		},
		func() {
			if last(first) != red.Light {
				t.Errorf("ThemeSystem must follow the light OS, got %p", last(first))
			}
			if p.windows[1].dark {
				t.Error("the title bar must follow the OS again")
			}
			p.brightness = platform.BrightnessDark
			p.send(platform.Event{Kind: platform.EventBrightness})
		},
		func() {
			if last(first) != red.Dark {
				t.Errorf("ThemeSystem must keep following the OS, got %p", last(first))
			}
			// The same theme again changes nothing.
			n := len(first)
			SetTheme(red, ThemeSystem)
			wake()
			p.script = append([]func(){func() {
				if len(first) != n {
					t.Errorf("setting the same theme rebuilt %d times", len(first)-n)
				}
				p.send(platform.Event{Kind: platform.EventClose, Window: 1})
				p.send(platform.Event{Kind: platform.EventClose, Window: 2})
			}}, p.script...)
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: themeRecorder{seen: &first}}); err != nil {
		t.Fatal(err)
	}
}

func TestSetTheme_ReachesAShowingDialog(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var home, dialog []*theme.Theme
	var ctx BuildContext
	red := theme.FromAccent(geom.Hex(0xc0392b))
	p, b = newFakes(t,
		func() {
			overlay.ShowDialog(ctx, func(func()) core.Widget { return themeRecorder{seen: &dialog} })
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			if last(dialog) != theme.Default().Light {
				t.Errorf("the dialog must open in the theme of the app, got %p", last(dialog))
			}
			SetTheme(red, ThemeDark)
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			if last(dialog) != red.Dark {
				t.Errorf("the dialog must take the new theme, got %p", last(dialog))
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: themeRecorder{seen: &home, ctx: &ctx}}); err != nil {
		t.Fatal(err)
	}
}

func TestSetTheme_OutsideRunPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("SetTheme outside Run must panic")
		}
	}()
	SetTheme(theme.Set{}, ThemeDark)
}
