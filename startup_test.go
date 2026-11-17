package gowaku

import (
	"errors"
	"testing"

	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/theme"
)

func deliver(b *fakeBackend, err error) <-chan deviceResult {
	c := make(chan deviceResult, 1)
	c <- deviceResult{b, err}
	return c
}

func deliverFonts(t *testing.T, err error) <-chan shaperResult {
	c := make(chan shaperResult, 1)
	if err != nil {
		c <- shaperResult{err: err}
	} else {
		c <- shaperResult{shaper: testutil.NewShaper(t)}
	}
	return c
}

func TestStart_ClearsTheWindowAndRunsTheApp(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){closeWindow(p, 1)}
	if err := start(p, deliver(b, nil), deliverFonts(t, nil), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	// The background while the fonts load, then the first frame.
	if b.frames != 2 || b.surfaces != 1 || b.instances != 1 {
		t.Errorf("got %d frames on %d surfaces, %d instances", b.frames, b.surfaces, b.instances)
	}
	if b.clear != theme.Default().Light.Color.Canvas {
		t.Errorf("cleared to %+v", b.clear)
	}
	if !p.windows[1].closed || b.destroyed != 1 {
		t.Error("the window and its surface must be closed")
	}
}

func TestStart_FailingDeviceClosesTheWindow(t *testing.T) {
	p, b := newFakes(t)
	want := errors.New("no gpu")
	if err := start(p, deliver(b, want), deliverFonts(t, nil), App{Home: counter{}}); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if !p.windows[1].closed || b.surfaces != 0 {
		t.Error("the window must be closed, without a surface")
	}
}

func TestStart_FailingFontsCloseTheWindow(t *testing.T) {
	p, b := newFakes(t)
	want := errors.New("no fonts")
	if err := start(p, deliver(b, nil), deliverFonts(t, want), App{Home: counter{}}); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if !p.windows[1].closed || b.destroyed != 1 {
		t.Error("the window and its surface must be closed")
	}
}

func TestStart_DarkAppGetsADarkTitleBarAtOnce(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){closeWindow(p, 1)}
	if err := start(p, deliver(b, nil), deliverFonts(t, nil), App{Home: counter{}, ThemeMode: ThemeDark}); err != nil {
		t.Fatal(err)
	}
	if !p.windows[1].opts.DarkTitleBar {
		t.Error("the window must be created dark")
	}
}
