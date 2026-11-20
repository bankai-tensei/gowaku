package gowaku

import (
	"errors"
	"testing"

	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/theme"
)

// pointer sends a press at a logical position of a window at scale 2.
func pointer(p *fakePlatform, kind platform.EventKind, x, y float32, button input.Button, ms uint32) {
	p.send(platform.Event{Kind: kind, Window: 1, X: x * 2, Y: y * 2, Button: button, Time: ms})
}

func click(p *fakePlatform, x, y float32, ms uint32) {
	pointer(p, platform.EventPointerMove, x, y, input.ButtonPrimary, ms)
	pointer(p, platform.EventPointerDown, x, y, input.ButtonPrimary, ms)
	pointer(p, platform.EventPointerUp, x, y, input.ButtonPrimary, ms+50)
}

// tapAt records where the content was tapped.
func tapAt(got *geom.Offset) core.Widget {
	return interact.GestureDetector{OnTapDownAt: func(t layout.Tap) { *got = t.Pos }, Child: basic.Container{}}
}

func TestRun_ClientDecoratedWindowDrawsItsTitleBar(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var tapped geom.Offset
	win := func() *fakeWindow { return p.windows[1] }
	// The window is 150x100 logical pixels. The buttons sit at the right of the
	// 36-pixel bar: close at x 126, maximize at 88, minimize at 50.
	p, b = newFakes(t,
		func() {
			win().csd = true
			// The platform reports a change of decoration with a resize.
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 300, PhysicalHeight: 200, Scale: 2})
		},
		func() {
			click(p, 40, 60, 100)
		},
		func() {
			if tapped != (geom.Offset{X: 40, Y: 24}) {
				t.Errorf("the content got a tap at %v, want it below the bar at (40, 24)", tapped)
			}
			click(p, 88, 18, 1000)
			click(p, 50, 18, 2000)
		},
		func() {
			if !win().maximized {
				t.Error("the maximize button did not maximize")
			}
			if win().minimizes != 1 {
				t.Errorf("the minimize button minimized %d times", win().minimizes)
			}
			// A click on the bar is not a move.
			click(p, 30, 18, 3000)
		},
		func() {
			if win().moves != 0 {
				t.Errorf("a click moved the window %d times", win().moves)
			}
			// The second press of a double click restores the window.
			pointer(p, platform.EventPointerDown, 30, 18, input.ButtonPrimary, 3200)
			pointer(p, platform.EventPointerUp, 30, 18, input.ButtonPrimary, 3250)
		},
		func() {
			if win().maximized {
				t.Error("a double click on the bar did not restore the window")
			}
			pointer(p, platform.EventPointerDown, 30, 18, input.ButtonPrimary, 5000)
			pointer(p, platform.EventPointerMove, 33, 18, input.ButtonPrimary, 5010)
			pointer(p, platform.EventPointerMove, 40, 20, input.ButtonPrimary, 5020)
			pointer(p, platform.EventPointerMove, 50, 22, input.ButtonPrimary, 5030)
		},
		func() {
			if win().moves != 1 {
				t.Errorf("a drag on the bar moved the window %d times, want once", win().moves)
			}
			// The compositor took the pointer, and the next press starts anew.
			p.send(platform.Event{Kind: platform.EventPointerLeave, Window: 1})
			pointer(p, platform.EventPointerMove, 30, 18, input.ButtonSecondary, 6000)
			pointer(p, platform.EventPointerDown, 30, 18, input.ButtonSecondary, 6000)
		},
		func() {
			if win().menuAt != [2]int{60, 36} {
				t.Errorf("the window menu opened at %v, want the press in physical pixels", win().menuAt)
			}
			click(p, 126, 18, 7000)
		},
		func() {
			t.Error("the close button did not close the window")
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Title: "t", Home: tapAt(&tapped)}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_DecoratedByTheOSHasNoTitleBar(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var tapped geom.Offset
	p, b = newFakes(t,
		func() { click(p, 40, 18, 100) },
		func() {
			if tapped != (geom.Offset{X: 40, Y: 18}) {
				t.Errorf("the content got a tap at %v, want (40, 18)", tapped)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: tapAt(&tapped)}); err != nil {
		t.Fatal(err)
	}
}

func TestShowFileDialog_AnswersOnTheUIGoroutine(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var got []string
	var seen []*theme.Theme
	var ctx BuildContext
	p, b = newFakes(t,
		func() {
			ShowFileDialog(ctx, FileDialog{Save: true, Name: "メモ.txt", Filters: []FileFilter{{Name: "Text", Patterns: []string{"*.txt"}}}},
				func(paths []string, err error) {
					if err != nil {
						t.Error(err)
					}
					got = paths
				})
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			if d := p.windows[1].dialogs; len(d) != 1 || !d[0].Save || len(d[0].Filters) != 1 || d[0].Filters[0].Patterns[0] != "*.txt" {
				t.Errorf("dialogs %+v", d)
			}
			if len(got) != 1 || got[0] != "/tmp/メモ.txt" {
				t.Errorf("got %q", got)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: themeRecorder{seen: &seen, ctx: &ctx}}); err != nil {
		t.Fatal(err)
	}
}

func TestShowFileDialog_WithoutADialogHostFails(t *testing.T) {
	var ctx BuildContext
	var err error
	ShowFileDialog(ctx, FileDialog{}, func(_ []string, e error) { err = e })
	if !errors.Is(err, ErrNoFileDialog) {
		t.Errorf("got %v, want ErrNoFileDialog", err)
	}
}

// A ctx can outlive its window, as that of a tray menu: the dialog then goes
// in front of a window still open.
func TestShowFileDialog_OutlivingItsWindowUsesAnother(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var first, second []*theme.Theme
	var ctx BuildContext
	var answered bool
	p, b = newFakes(t,
		func() {
			if _, err := OpenWindow(WindowOptions{}, themeRecorder{seen: &second, ctx: &ctx}); err != nil {
				t.Fatal(err)
			}
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			p.send(platform.Event{Kind: platform.EventClose, Window: 2})
		},
		func() {
			if _, ok := p.windows[2]; ok && !p.windows[2].closed {
				t.Fatal("the second window must be closed")
			}
			ShowFileDialog(ctx, FileDialog{Name: "a.txt"}, func([]string, error) { answered = true })
			p.send(platform.Event{Kind: platform.EventWake})
		},
		func() {
			if len(p.windows[1].dialogs) != 1 {
				t.Errorf("the open window must show the dialog, got %d", len(p.windows[1].dialogs))
			}
			if !answered {
				t.Error("done must run")
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: themeRecorder{seen: &first}}); err != nil {
		t.Fatal(err)
	}
}
