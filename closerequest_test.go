package gowaku

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// alive reports whether window id is still open.
func alive(id platform.WindowID) bool {
	_, ok := currentApp.Load().windows[id]
	return ok
}

func TestOnCloseRequest_RefusalKeepsTheWindowAndItsTree(t *testing.T) {
	p, b := newFakes(t)
	asked, closed := 0, 0
	allow := false
	p.script = []func(){
		func() {
			Post(func() { lastCounter.SetState(func() { lastCounter.n = 3 }) })
		},
		closeWindow(p, 1),
		func() {
			if asked != 1 || !alive(1) || closed != 0 || p.windows[1].closed {
				t.Fatalf("a refused close went ahead: asked %d, closed %d", asked, closed)
			}
			if lastCounter.n != 3 {
				t.Error("the tree must survive a refused close")
			}
			// Queued twice: the second request comes after the first was
			// refused, and is asked again.
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
		func() {
			if asked != 3 || !alive(1) {
				t.Fatalf("asked %d times, want 3", asked)
			}
			allow = true
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	}
	app := App{
		Home:           counter{},
		OnClose:        func() { closed++ },
		OnCloseRequest: func() bool { asked++; return allow },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if asked != 4 || closed != 1 || b.destroyed != 1 {
		t.Errorf("asked %d, closed %d, destroyed %d; want 4, 1, 1", asked, closed, b.destroyed)
	}
}

func TestOnCloseRequest_TheProgramClosesWithoutAsking(t *testing.T) {
	p, b := newFakes(t)
	asked := 0
	refuse := func() bool { asked++; return false }
	var second, third *WindowHandle
	p.script = []func(){
		func() {
			var err error
			if second, err = OpenWindow(WindowOptions{OnCloseRequest: refuse}, counter{}); err != nil {
				t.Fatal(err)
			}
			if third, err = OpenWindow(WindowOptions{OnCloseRequest: refuse}, counter{}); err != nil {
				t.Fatal(err)
			}
			second.Close()
			p.Wake()
		},
		func() {
			if alive(2) {
				t.Error("Close must close a window that refuses the user")
			}
			// Someone else destroys the third window: it cannot refuse.
			p.send(platform.Event{Kind: platform.EventDestroyed, Window: third.window.native.ID()})
		},
		func() {
			if alive(3) {
				t.Error("a destroyed window must go")
			}
			Quit()
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}, OnCloseRequest: refuse}); err != nil {
		t.Fatal(err)
	}
	if asked != 0 || b.destroyed != 3 {
		t.Errorf("asked %d times, destroyed %d; want 0, 3", asked, b.destroyed)
	}
}

func TestOnCloseRequest_TitleBarCloseButtonAsks(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	asked := 0
	allow := false
	p, b = newFakes(t,
		func() {
			p.windows[1].csd = true
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 300, PhysicalHeight: 200, Scale: 2})
		},
		func() {
			// The close button sits at x 126 of the 150-pixel window.
			click(p, 126, 18, 1000)
		},
		func() {
			if asked != 1 || !alive(1) {
				t.Fatalf("the close button of the title bar: asked %d, window there %v", asked, alive(1))
			}
			allow = true
			click(p, 126, 18, 2000)
		},
	)
	app := App{Title: "t", Home: counter{}, OnCloseRequest: func() bool { asked++; return allow }}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if asked != 2 {
		t.Errorf("asked %d times, want 2", asked)
	}
}

// requestOnTap asks for its window to close from a callback of its tree.
type requestOnTap struct {
	StatelessBase
	during func()
}

// Build implements core.StatelessWidget.
func (r requestOnTap) Build(ctx BuildContext) Widget {
	return interact.GestureDetector{
		OnTap: func() {
			WindowOf(ctx).RequestClose()
			r.during()
		},
		Child: basic.Container{Width: 150, Height: 100, Color: geom.Hex(0x808080)},
	}
}

func TestWindowHandle_RequestCloseAsksAfterTheCallback(t *testing.T) {
	p, b := newFakes(t)
	var log []string
	allow := false
	tap := func() {
		p.send(platform.Event{Kind: platform.EventPointerMove, Window: 1, X: 40, Y: 40})
		p.send(platform.Event{Kind: platform.EventPointerDown, Window: 1, X: 40, Y: 40})
		p.send(platform.Event{Kind: platform.EventPointerUp, Window: 1, X: 40, Y: 40})
	}
	p.script = []func(){
		tap,
		func() {
			if strings.Join(log, " ") != "tap ask" || !alive(1) {
				t.Fatalf("got %v, want the request asked after the tap and refused", log)
			}
			allow = true
			tap()
		},
	}
	home := requestOnTap{during: func() { log = append(log, "tap") }}
	app := App{Home: home, OnCloseRequest: func() bool { log = append(log, "ask"); return allow }}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if strings.Join(log, " ") != "tap ask tap ask" {
		t.Errorf("got %v", log)
	}
}

func TestOnCloseRequest_ModalWindows(t *testing.T) {
	p, b := newFakes(t)
	var ctx BuildContext
	var log []string
	var modal *WindowHandle
	p.script = []func(){
		func() {
			// The owner refuses and asks with a modal window of its own,
			// opened from the request.
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
		func() {
			if modal == nil || !alive(1) {
				t.Fatal("the request did not open its modal window")
			}
			// The owner is asked nothing while the modal window is open: the
			// modal window comes to the front.
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
			Post(func() { WindowOf(ctx).RequestClose() })
		},
		func() {
			if n := p.windows[2].activations; n != 2 {
				t.Errorf("requests to the owner activated the modal window %d times, want 2", n)
			}
			// The × of the modal window asks the modal window.
			p.send(platform.Event{Kind: platform.EventClose, Window: 2})
		},
		func() {
			if !alive(2) {
				t.Fatal("the modal window must have refused")
			}
			// Closing the owner from the program closes its modal window
			// without asking it.
			closeWindowNow(p, 1)()
		},
	}
	home := basic.Builder{Content: func(c BuildContext) Widget {
		ctx = c
		return counter{}
	}}
	app := App{
		Home: home,
		OnCloseRequest: func() bool {
			log = append(log, "owner")
			opts := WindowOptions{
				OnCloseRequest: func() bool { log = append(log, "modal"); return false },
				OnClose:        func() { log = append(log, "modal closed") },
			}
			var err error
			if modal, err = ShowModal(ctx, opts, counter{}); err != nil {
				t.Fatal(err)
			}
			return false
		},
		OnClose: func() { log = append(log, "owner closed") },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(log, ", "); got != "owner, modal, modal closed, owner closed" {
		t.Errorf("got %s", got)
	}
}

// guard puts its own close request on its window, as a State that knows what
// the window holds does.
type guard struct {
	StatelessBase
	ask func() bool
}

// Build implements core.StatelessWidget.
func (g guard) Build(ctx BuildContext) Widget {
	WindowOf(ctx).SetOnCloseRequest(g.ask)
	return basic.SizedBox{}
}

func TestWindowHandle_SetOnCloseRequestTakesThePlaceOfTheOption(t *testing.T) {
	p, b := newFakes(t)
	var log []string
	allow := false
	p.script = []func(){
		closeWindow(p, 1),
		func() {
			if !alive(1) {
				t.Fatal("the request of the widget did not refuse")
			}
			allow = true
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	}
	app := App{
		Home:           guard{ask: func() bool { log = append(log, "widget"); return allow }},
		OnCloseRequest: func() bool { log = append(log, "option"); return true },
	}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(log, " "); got != "widget widget" {
		t.Errorf("got %s", got)
	}
}

// The request may close the window itself, and say true or false.
func TestOnCloseRequest_ClosingFromTheRequest(t *testing.T) {
	p, b := newFakes(t)
	closed := 0
	var second *WindowHandle
	p.script = []func(){
		func() {
			var err error
			opts := WindowOptions{OnClose: func() { closed++ }, OnCloseRequest: func() bool { second.Close(); return true }}
			if second, err = OpenWindow(opts, counter{}); err != nil {
				t.Fatal(err)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 2})
		},
		func() {
			if alive(2) || closed != 1 {
				t.Errorf("window there %v, closed %d times; want gone once", alive(2), closed)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	}
	h := func() bool { Quit(); return false }
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}, OnCloseRequest: h}); err != nil {
		t.Fatal(err)
	}
	if b.destroyed != 2 {
		t.Errorf("destroyed %d, want 2", b.destroyed)
	}
}
