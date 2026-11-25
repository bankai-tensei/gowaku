package gowaku

import (
	"errors"
	"testing"

	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
)

func TestOnQuitRequest_NilAsksEveryWindow(t *testing.T) {
	p, b := newFakes(t)
	asked := map[string]int{}
	allow := false
	ask := func(name string) func() bool {
		return func() bool { asked[name]++; return allow }
	}
	p.script = []func(){
		func() {
			if _, err := OpenWindow(WindowOptions{OnCloseRequest: ask("second")}, counter{}); err != nil {
				t.Fatal(err)
			}
			p.send(platform.Event{Kind: platform.EventQuit})
		},
		func() {
			if asked["first"] != 1 || asked["second"] != 1 || !alive(1) || !alive(2) {
				t.Fatalf("a refused quit: asked %v, alive %v %v", asked, alive(1), alive(2))
			}
			allow = true
			p.send(platform.Event{Kind: platform.EventQuit})
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}, OnCloseRequest: ask("first")}); err != nil {
		t.Fatal(err)
	}
	if asked["first"] != 2 || asked["second"] != 2 {
		t.Errorf("asked %v, want each window twice", asked)
	}
}

func TestOnQuitRequest_DecidesForTheApplication(t *testing.T) {
	p, b := newFakes(t)
	quits, closes, closed := 0, 0, 0
	allow := false
	p.script = []func(){
		func() { p.send(platform.Event{Kind: platform.EventQuit}) },
		func() {
			if quits != 1 || closes != 0 || !alive(1) {
				t.Fatalf("quits %d, closes %d, alive %v; a refused quit must keep the window, asking it nothing", quits, closes, alive(1))
			}
			// The close button still asks the window, which refuses.
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
		func() {
			if closes != 1 || !alive(1) {
				t.Fatalf("closes %d, alive %v", closes, alive(1))
			}
			allow = true
			p.send(platform.Event{Kind: platform.EventQuit})
		},
	}
	app := App{
		Home:           counter{},
		OnClose:        func() { closed++ },
		OnCloseRequest: func() bool { closes++; return false },
		OnQuitRequest:  func() bool { quits++; return allow },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if quits != 2 || closes != 1 || closed != 1 {
		t.Errorf("quits %d, closes %d, closed %d; want 2, 1, 1", quits, closes, closed)
	}
}

func TestHide_KeepsTheWindowAndRunGoesOn(t *testing.T) {
	p, b := newFakes(t)
	var win *WindowHandle
	frames := 0
	p.script = []func(){
		func() {
			win = WindowOf(lastCounter.Context())
			Post(func() { lastCounter.SetState(func() { lastCounter.n = 3 }) })
			// The close button hides the window.
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
		func() {
			if !alive(1) || !win.Hidden() || !p.windows[1].hidden {
				t.Fatalf("alive %v, hidden %v / %v; the close request must have hidden the window", alive(1), win.Hidden(), p.windows[1].hidden)
			}
			frames = b.frames
			Post(func() { lastCounter.SetState(func() { lastCounter.n = 4 }) })
		},
		func() {
			if b.frames != frames {
				t.Errorf("a hidden window drew %d frames", b.frames-frames)
			}
			activations := p.windows[1].activations
			win.Show()
			if win.Hidden() || p.windows[1].hidden || p.windows[1].activations != activations+1 {
				t.Errorf("Show must put the window back and activate it")
			}
			p.Wake()
		},
		func() {
			if b.frames == frames {
				t.Error("a shown window must draw what changed while it was hidden")
			}
			if lastCounter.n != 4 {
				t.Error("the tree must stay while the window is hidden")
			}
			win.Hide()
			p.send(platform.Event{Kind: platform.EventReopen})
		},
		func() {
			if win.Hidden() {
				t.Error("a reopen from the Dock must show the hidden window")
			}
			Quit()
		},
	}
	app := App{
		Home:           counter{},
		OnCloseRequest: func() bool { win.Hide(); return false },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
}

func TestHide_TakesTheModalWindowsAlong(t *testing.T) {
	p, b := newFakes(t)
	var win, modal *WindowHandle
	p.script = []func(){
		func() {
			win = WindowOf(lastCounter.Context())
			var err error
			if modal, err = ShowModal(lastCounter.Context(), WindowOptions{}, counter{}); err != nil {
				t.Fatal(err)
			}
			win.Hide()
			if !modal.Hidden() || !p.windows[2].hidden {
				t.Error("Hide must hide the modal window over the window")
			}
			win.Show()
			if modal.Hidden() || p.windows[2].hidden {
				t.Error("Show must show the modal window again")
			}
			Quit()
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestRequestNotifications_AnswersOnTheUIGoroutineAfterward(t *testing.T) {
	p, b := newFakes(t)
	var answers []error
	returned := false
	p.script = []func(){
		func() {
			ctx := lastCounter.Context()
			RequestNotifications(ctx, func(err error) {
				if !returned {
					t.Error("done must run after RequestNotifications returns")
				}
				answers = append(answers, err)
			})
			returned = true
			p.noNotify = true
			RequestNotifications(ctx, func(err error) { answers = append(answers, err) })
			if err := Notify(ctx, "題", "本文"); !errors.Is(err, ErrNoNotifications) {
				t.Errorf("Notify refused = %v, want ErrNoNotifications", err)
			}
			p.Wake()
		},
		closeWindow(p, 1),
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if p.asked != 2 || len(answers) != 2 || answers[0] != nil || !errors.Is(answers[1], ErrNoNotifications) {
		t.Errorf("asked %d, answers %v", p.asked, answers)
	}
}

// startTray shows a tray icon and asks for leave to notify from InitState,
// as an application does at start-up.
type startTray struct {
	StatefulBase
	shown *error
	asked *error
}

func (startTray) CreateState() State { return &startTrayState{} }

type startTrayState struct{ StateBase }

func (s *startTrayState) InitState() {
	w := s.Widget().(startTray)
	_, *w.shown = ShowTray(s.Context(), TrayOptions{Tooltip: "tip"})
	RequestNotifications(s.Context(), func(err error) { *w.asked = err })
}

func (s *startTrayState) Build(BuildContext) Widget { return counter{} }

func TestShowTray_FromInitStateOfTheFirstBuild(t *testing.T) {
	p, b := newFakes(t)
	shown, asked := errors.New("not run"), errors.New("not run")
	p.script = []func(){closeWindow(p, 1)}
	if err := run(p, b, testutil.NewShaper(t), App{Home: startTray{shown: &shown, asked: &asked}}); err != nil {
		t.Fatal(err)
	}
	if shown != nil || p.tray == nil || asked != nil {
		t.Errorf("ShowTray %v, RequestNotifications %v; the first build must reach the host", shown, asked)
	}
}
