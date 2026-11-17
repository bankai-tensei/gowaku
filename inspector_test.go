//go:build gowakudebug

package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/inspect"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// The Inspector window is the second the fake platform creates.
const inspectorID platform.WindowID = 2

func pressKey(p *fakePlatform, id platform.WindowID, k input.KeyCode) {
	p.send(platform.Event{Kind: platform.EventKeyDown, Window: id, Key: k})
}

// clickAt sends a press and a release at x, y, in physical pixels.
func clickAt(p *fakePlatform, id platform.WindowID, x, y float32) {
	p.send(platform.Event{Kind: platform.EventPointerMove, Window: id, X: x, Y: y})
	p.send(platform.Event{Kind: platform.EventPointerDown, Window: id, X: x, Y: y})
	p.send(platform.Event{Kind: platform.EventPointerUp, Window: id, X: x, Y: y})
}

// wake lets the loop run once more, after a step that queued no event.
func wake(p *fakePlatform) { p.send(platform.Event{Kind: platform.EventWake}) }

func inspectorOf(t *testing.T) *inspectorState {
	t.Helper()
	ap := currentApp.Load()
	if ap == nil {
		t.Fatal("no application runs")
	}
	return &ap.inspector
}

func TestInspector_F12OpensAndCloses(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			st := inspectorOf(t)
			if st.win == nil || st.session == nil || p.windows[inspectorID] == nil {
				t.Fatal("F12 must open the Inspector")
			}
			if st.win.title != "gowaku Inspector" {
				t.Errorf("the Inspector window is titled %q", st.win.title)
			}
			pressKey(p, 1, input.KeyF12)
		},
		func() {
			st := inspectorOf(t)
			if st.win != nil || st.session != nil || !p.windows[inspectorID].closed {
				t.Error("F12 again must close the Inspector")
			}
			pressKey(p, 1, input.KeyF12)
		},
		// The Inspector opened again, as the third window: F12 in it closes it.
		func() { pressKey(p, 3, input.KeyF12) },
		func() {
			if inspectorOf(t).win != nil || !p.windows[3].closed {
				t.Error("F12 in the Inspector window must close it")
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_OpenBeforeRunOpensWithTheFirstWindow(t *testing.T) {
	t.Cleanup(func() { openAtStart.Store(false) })
	OpenInspector()
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() {
			if inspectorOf(t).win == nil || p.windows[inspectorID] == nil {
				t.Error("OpenInspector before Run must open the Inspector with the first window")
			}
			if openAtStart.Load() {
				t.Error("the request must be used up")
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: inspectorID})
		},
		func() { closeWindow(p, 1)() },
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_OpenDuringRunAndTwice(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() { OpenInspector() },
		func() {
			st := inspectorOf(t)
			if st.win == nil {
				t.Fatal("OpenInspector during Run must open the Inspector")
			}
			OpenInspector()
		},
		func() {
			if len(p.windows) != 2 {
				t.Errorf("a second OpenInspector opened another window: %d windows", len(p.windows))
			}
			if p.windows[inspectorID].activations == 0 {
				t.Error("a second OpenInspector must bring the Inspector forward")
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_PickTakesThePointerFromTheApplication(t *testing.T) {
	presses := 0
	var p *fakePlatform
	var b *fakeBackend
	// The window is 150 by 100 logical pixels at scale 2; the button is in
	// its middle.
	const x, y = 150, 100
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			inspectorOf(t).session.SetPicking(true)
			clickAt(p, 1, x, y)
		},
		func() {
			s := inspectorOf(t).session
			if presses != 0 {
				t.Errorf("the button saw the click that picked: %d presses", presses)
			}
			if s.Picking {
				t.Error("a pick must end the pick mode")
			}
			sel, el, ro := s.Selection()
			if sel != inspect.Target(currentApp.Load().windows[1]) || ro == nil || el.IsZero() {
				t.Errorf("the click must select what is under it, got %v %v", sel, ro)
			}
			if p.windows[inspectorID].activations == 0 {
				t.Error("a pick must bring the Inspector forward")
			}
			clickAt(p, 1, x, y)
		},
		func() {
			if presses != 1 {
				t.Errorf("after the pick the button must see clicks, got %d presses", presses)
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	home := basic.Center{Child: control.Button{Label: "Go", OnPress: func() { presses++ }}}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_EscapeCancelsThePick(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			inspectorOf(t).session.SetPicking(true)
			p.send(platform.Event{Kind: platform.EventPointerMove, Window: 1, X: 20, Y: 20})
			pressKey(p, 1, input.KeyEscape)
		},
		func() {
			s := inspectorOf(t).session
			if s.Picking {
				t.Error("Escape must cancel the pick")
			}
			if sel, _, _ := s.Selection(); sel != nil {
				t.Error("a canceled pick selects nothing")
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_RecordsTheApplicationOnly(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var frames int
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			frames = b.frames
			p.send(platform.Event{Kind: platform.EventRedraw, Window: 1})
			p.send(platform.Event{Kind: platform.EventRedraw, Window: inspectorID})
		},
		func() {
			st := inspectorOf(t)
			s := st.session
			app := inspect.Target(currentApp.Load().windows[1])
			if s.Frames() == 0 {
				t.Fatal("the frames of the application must be recorded")
			}
			for i := range s.Frames() {
				f := s.Frame(i)
				if f.Target != app {
					t.Errorf("frame %d is of %v, not of the application", f.Seq, f.Target)
				}
			}
			if b.frames-frames < 2 || st.seq < uint64(s.Frames()+1) {
				t.Errorf("both windows must draw and be counted: %d frames, seq %d", b.frames-frames, st.seq)
			}
			if st.win.tree.Probe() != nil {
				t.Error("the Inspector must not probe its own tree")
			}
			if currentApp.Load().windows[1].tree.Probe() == nil {
				t.Error("the application's tree must be probed")
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: inspectorID})
		},
		func() {
			if currentApp.Load().windows[1].tree.Probe() != nil {
				t.Error("closing the Inspector must remove the probes")
			}
			closeWindow(p, 1)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_ForgetsAClosingWindow(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var second *WindowHandle
	var s *inspect.Session
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			var err error
			if second, err = OpenWindow(WindowOptions{Title: "second"}, counter{}); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			st := inspectorOf(t)
			s = st.session
			w := currentApp.Load().windows[3]
			if w == nil || w.title != "second" {
				t.Fatal("the second window is not the third")
			}
			if ts := s.Host().Targets(); len(ts) != 2 || ts[0] != inspect.Target(currentApp.Load().windows[1]) || ts[1] != inspect.Target(w) {
				t.Errorf("got targets %v, want the two windows of the application, oldest first", ts)
			}
			s.Select(w, w.tree.RootElement())
			second.Close()
			wake(p)
		},
		func() {
			for i := range s.Frames() {
				if f := s.Frame(i); f.Target != nil && f.Target.Title() == "second" {
					t.Errorf("frame %d still refers to the closed window", f.Seq)
				}
			}
			if sel, _, _ := s.Selection(); sel != nil {
				t.Error("the selection in the closed window must go")
			}
			if len(s.Host().Targets()) != 1 {
				t.Error("the closed window must leave the targets")
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

func TestInspector_OverlayDrawsOverTheApplication(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var plain int
	ops := func() int { return len(currentApp.Load().windows[1].canvas.DisplayList().Ops) }
	p, b = newFakes(t,
		func() {
			plain = ops()
			pressKey(p, 1, input.KeyF12)
		},
		func() {
			inspectorOf(t).session.SetShowBounds(true)
			wake(p)
		},
		func() {
			if ops() <= plain {
				t.Errorf("the outlines must be drawn over the application: %d drawings, %d without", ops(), plain)
			}
			inspectorOf(t).session.SetShowBounds(false)
			wake(p)
		},
		func() {
			if ops() != plain {
				t.Errorf("turning the outlines off must take them away: %d drawings, want %d", ops(), plain)
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}

// The Inspector counts the allocations of the frames it records, so recording
// must not add any of its own once its ring has filled.
func TestInspector_RecordingDoesNotAllocate(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	p, b = newFakes(t,
		func() { pressKey(p, 1, input.KeyF12) },
		func() {
			ap := currentApp.Load()
			w := ap.windows[1]
			frame := func() {
				w.tree.MarkNeedsPaint()
				ap.frame(w)
			}
			for range inspect.FrameRing {
				frame()
			}
			if allocs := testing.AllocsPerRun(20, frame); allocs != 0 {
				t.Errorf("a recorded frame allocates %v times", allocs)
			}
			closeWindow(p, 1)()
			closeWindow(p, inspectorID)()
		},
	)
	if err := run(p, b, testutil.NewShaper(t), App{Home: counter{}}); err != nil {
		t.Fatal(err)
	}
}
