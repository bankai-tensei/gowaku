//go:build gowakudebug

package inspect

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// framesFixture is a session with one window whose tree has a keyed and an
// unkeyed element to name as rebuilt.
type framesFixture struct {
	ses   *Session
	tgt   *fakeTarget
	keyed core.ElementView
	plain core.ElementView
	seq   uint64
}

func newFramesFixture(t *testing.T) *framesFixture {
	t.Helper()
	root := basic.Column{Children: []core.Widget{
		core.Keyed("a", basic.SizedBox{Width: 10, Height: 10}),
		basic.SizedBox{Width: 10, Height: 10},
	}}
	tgt := newFakeTarget("Demo", root, geom.Size{W: 200, H: 100})
	ses := NewSession(&fakeHost{targets: []Target{tgt}}, nil)
	fx := &framesFixture{ses: ses, tgt: tgt}
	fx.keyed = findFrameElement(tgt.Tree().RootElement(), func(el core.ElementView) bool { return el.Key() == "a" })
	fx.plain = findFrameElement(tgt.Tree().RootElement(), func(el core.ElementView) bool {
		_, ok := el.Widget().(basic.SizedBox)
		return ok && el.Key() == nil
	})
	if fx.keyed.IsZero() || fx.plain.IsZero() {
		t.Fatal("the fixture tree lacks its elements")
	}
	return fx
}

func findFrameElement(el core.ElementView, match func(core.ElementView) bool) core.ElementView {
	if match(el) {
		return el
	}
	for i := range el.ChildCount() {
		if found := findFrameElement(el.ChildAt(i), match); !found.IsZero() {
			return found
		}
	}
	return core.ElementView{}
}

// add records a frame of k: its stages vary with k, and it rebuilds the keyed
// element once and the plain one rebuilt-1 times.
func (fx *framesFixture) add(k int, rebuilt int) {
	fx.seq++
	fx.ses.BeginFrame(fx.tgt, fx.seq)
	for i := range rebuilt {
		if i == 0 {
			fx.ses.Rebuilt(fx.keyed)
		} else {
			fx.ses.Rebuilt(fx.plain)
		}
	}
	for range k % 7 {
		fx.ses.LaidOut(nil)
	}
	ms := func(f float64) time.Duration { return time.Duration(f * float64(time.Millisecond)) }
	wave := float64(k%13) / 13
	stages := [StageCount]time.Duration{
		ms(0.4 + 1.2*wave), ms(0.3 + 0.5*wave), ms(0.6 + 0.8*wave), ms(0.3 + 0.2*wave), ms(4 + 3*wave),
	}
	if k%37 == 20 {
		stages[StageBuild] = ms(9)
		stages[StagePaint] = ms(4)
	}
	fx.ses.EndFrame(fx.tgt, stages, uint64(k%11*k%5), k%17 == 0)
	fx.ses.Frame(fx.ses.Frames() - 1).GPU = ms(1.5 + 2*wave)
}

func (fx *framesFixture) fill(n int) {
	for k := range n {
		fx.add(k, 1+k%3)
	}
}

func newFramesTester(t *testing.T, ses *Session, height float32, dark bool) *gowakutest.Tester {
	t.Helper()
	return gowakutest.New(t, listening{ses: ses}, gowakutest.Options{Width: 1180, Height: height, Dark: dark})
}

// listening rebuilds the panel when the session changes, as Inspector does.
type listening struct {
	core.StatefulBase
	ses *Session
}

func (listening) CreateState() core.State { return &listeningState{} }

type listeningState struct {
	core.StateOf[listening]
	cancel func()
}

func (s *listeningState) InitState() {
	s.cancel = s.Widget().ses.Listen(func() { s.SetState(func() {}) })
}

func (s *listeningState) Dispose() { s.cancel() }

func (s *listeningState) Build(core.BuildContext) core.Widget {
	return FramesPanel{Session: s.Widget().ses}
}

// barCenter returns where the bar of frame i is on the screen.
func barCenter(t *testing.T, tt *gowakutest.Tester, ses *Session, i int) geom.Offset {
	t.Helper()
	n, ok := tt.Find(framesEN.graph)
	if !ok {
		t.Fatalf("no graph in\n%s", tt.Dump())
	}
	w := (n.Rect.W - graphGutter) / FrameRing
	k := slotOf(i, ses.Frames())
	return geom.Offset{X: n.Rect.X + graphGutter + (float32(k)+0.5)*w, Y: n.Rect.Y + barsHeight - 2}
}

func TestFramesPanel_Empty(t *testing.T) {
	fx := newFramesFixture(t)
	tt := newFramesTester(t, fx.ses, 240, false)
	for _, label := range []string{framesEN.noFrames, stringsEN.noFrame} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no %q in\n%s", label, tt.Dump())
		}
	}
}

func TestFramesPanel_ClickSelects(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(30)
	tt := newFramesTester(t, fx.ses, 240, false)
	tt.TapAt(barCenter(t, tt, fx.ses, 12))
	if fx.ses.SelectedFrame != 12 {
		t.Fatalf("SelectedFrame = %d, want 12", fx.ses.SelectedFrame)
	}
	want := framesEN.frame + strconv.FormatUint(fx.ses.Frame(12).Seq, 10)
	if _, ok := tt.Find(want); !ok {
		t.Errorf("no %q in\n%s", want, tt.Dump())
	}
	// A click left of the oldest frame selects nothing new.
	tt.TapAt(barCenter(t, tt, fx.ses, -5))
	if fx.ses.SelectedFrame != 12 {
		t.Errorf("a click on an empty slot changed the selection to %d", fx.ses.SelectedFrame)
	}
}

func TestFramesPanel_Details(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(5)
	fx.add(5, 70) // more than a record names
	fx.ses.SelectFrame(5)
	tt := newFramesTester(t, fx.ses, 1700, false)
	f := fx.ses.Frame(5)
	for _, label := range []string{
		"Demo",
		`Keyed (key "a")`,
		"+6 more",
		framesEN.cpu,
		msText(f.CPU(), &framesEN),
		msText(f.GPU, &framesEN),
	} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no %q in\n%s", label, tt.Dump())
		}
	}
	if got := len(tt.FindAll("basic.SizedBox")); got != 63 {
		t.Errorf("%d unkeyed rows, want 63", got)
	}
	tt.Tap(`Keyed (key "a")`)
	if target, el, _ := fx.ses.Selection(); target != fx.tgt || el != fx.keyed {
		t.Errorf("Selection = %v, %v; want the keyed element of the window", target, el)
	}
}

func TestFramesPanel_UnmountedIsDisabled(t *testing.T) {
	fx := newFramesFixture(t)
	fx.add(0, 2)
	fx.ses.SelectFrame(0)
	fx.ses.Forget(fx.tgt)
	tt := newFramesTester(t, fx.ses, 240, false)
	if _, ok := tt.Find(framesEN.closedWindow); !ok {
		t.Errorf("no %q in\n%s", framesEN.closedWindow, tt.Dump())
	}
}

func TestFramesPanel_Hover(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(40)
	tt := newFramesTester(t, fx.ses, 240, false)
	pos := barCenter(t, tt, fx.ses, 20)
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
	tt.Pump()
	want := frameReadout(fx.ses.Frame(20), &stringsEN, &framesEN)
	if _, ok := tt.Find(want); !ok {
		t.Errorf("no readout %q in\n%s", want, tt.Dump())
	}
}

func TestFramesPanel_Wrap(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(FrameRing)
	fx.ses.SelectFrame(100)
	seq := fx.ses.Frame(100).Seq
	for k := range 10 {
		fx.add(FrameRing+k, 1)
	}
	if got := fx.ses.SelectedFrame; got != 90 || fx.ses.Frame(got).Seq != seq {
		t.Fatalf("SelectedFrame = %d, want 90 with seq %d", got, seq)
	}
	tt := newFramesTester(t, fx.ses, 240, false)
	want := framesEN.frame + strconv.FormatUint(seq, 10)
	if _, ok := tt.Find(want); !ok {
		t.Errorf("no %q in\n%s", want, tt.Dump())
	}
	tt.TapAt(barCenter(t, tt, fx.ses, FrameRing-1))
	if fx.ses.SelectedFrame != FrameRing-1 {
		t.Errorf("SelectedFrame = %d after a click on the newest", fx.ses.SelectedFrame)
	}
	// A selected frame that leaves the ring goes.
	fx.ses.SelectFrame(0)
	fx.add(0, 1)
	if fx.ses.SelectedFrame != -1 {
		t.Errorf("SelectedFrame = %d after its frame left", fx.ses.SelectedFrame)
	}
	tt.Pump()
	if _, ok := tt.Find(stringsEN.noFrame); !ok {
		t.Errorf("no placeholder after the selection went in\n%s", tt.Dump())
	}
}

func TestFramesPanel_Paused(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(10)
	fx.ses.SetPaused(true)
	fx.add(10, 1)
	if fx.ses.Frames() != 10 {
		t.Fatalf("%d frames while paused, want 10", fx.ses.Frames())
	}
	tt := newFramesTester(t, fx.ses, 240, false)
	want := summaryOf(fx.ses, &framesEN) + "  ·  " + framesEN.paused
	if _, ok := tt.Find(want); !ok {
		t.Errorf("no %q in\n%s", want, tt.Dump())
	}
}

func TestFramesGraph_PaintDoesNotAllocate(t *testing.T) {
	fx := newFramesFixture(t)
	fx.fill(FrameRing)
	g := framesGraph{ses: fx.ses, hover: 3, selected: 7}
	g.top, g.maxAllocs = scaleOf(fx.ses)
	var cv paint.Canvas
	sz := geom.Size{W: 800, H: graphHeight}
	g.Paint(&cv, sz)
	allocs := testing.AllocsPerRun(10, func() {
		cv.Reset()
		g.Paint(&cv, sz)
	})
	if allocs != 0 {
		t.Errorf("Paint allocates %v times", allocs)
	}
}

// TestFramesPanel_Screenshot saves the panel in both themes to the directory
// in GOWAKU_FRAMES_SHOTS, for a look by eye.
func TestFramesPanel_Screenshot(t *testing.T) {
	dir := os.Getenv("GOWAKU_FRAMES_SHOTS")
	if dir == "" {
		t.Skip("GOWAKU_FRAMES_SHOTS is not set")
	}
	for _, dark := range []bool{false, true} {
		fx := newFramesFixture(t)
		fx.fill(FrameRing + 30)
		tt := newFramesTester(t, fx.ses, 240, dark)
		tt.TapAt(barCenter(t, tt, fx.ses, 200))
		tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: barCenter(t, tt, fx.ses, 150)})
		tt.Pump()
		name := "frames-light.png"
		if dark {
			name = "frames-dark.png"
		}
		tt.SaveScreenshot(filepath.Join(dir, name))
	}
	fx := newFramesFixture(t)
	fx.fill(40)
	tt := newFramesTester(t, fx.ses, 240, false)
	tt.SaveScreenshot(filepath.Join(dir, "frames-few.png"))
}
