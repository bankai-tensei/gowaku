//go:build gowakudebug

package inspect

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// sampleTree is a padded box, a box in a layer of its own, and a box hidden by
// a fully transparent Opacity, top to bottom in a 200 by 150 window.
func sampleTree() core.Widget {
	return basic.Column{Children: []core.Widget{
		basic.Padding{Insets: geom.All(10), Child: basic.SizedBox{Width: 50, Height: 30}},
		basic.RepaintBoundary{Child: basic.SizedBox{Width: 40, Height: 20}},
		basic.Opacity{Opacity: 0, Child: basic.SizedBox{Width: 40, Height: 20}},
	}}
}

func newSampleSession(t *testing.T) (*Session, *fakeTarget, *fakeHost) {
	t.Helper()
	target := newFakeTarget("app", sampleTree(), geom.Size{W: 200, H: 150})
	t.Cleanup(target.tree.Dispose)
	host := &fakeHost{targets: []Target{target}}
	return NewSession(host, nil), target, host
}

// record records a frame of t with sequence number seq.
func record(s *Session, t Target, seq uint64) {
	s.BeginFrame(t, seq)
	s.EndFrame(t, [StageCount]time.Duration{}, 0, false)
}

// age moves the clock of s forward by d.
func age(s *Session, d time.Duration) { s.start = s.start.Add(-d) }

// findRO returns the first render object of type T in the tree of t.
func findRO[T core.RenderObject](t *fakeTarget) T {
	var found T
	var walk func(ro core.RenderObject) bool
	walk = func(ro core.RenderObject) bool {
		if r, ok := ro.(T); ok {
			found = r
			return true
		}
		for i := range ro.ChildCount() {
			if c := ro.ChildAt(i); c != nil && walk(c) {
				return true
			}
		}
		return false
	}
	walk(t.tree.RootRenderObject())
	return found
}

func TestSession_RingWraps(t *testing.T) {
	s, target, _ := newSampleSession(t)
	for i := range 10 {
		record(s, target, uint64(i+1))
	}
	if s.Frames() != 10 || s.Frame(0).Seq != 1 || s.Frame(9).Seq != 10 {
		t.Fatalf("before the wrap: %d frames, %d..%d", s.Frames(), s.Frame(0).Seq, s.Frame(s.Frames()-1).Seq)
	}
	for i := 10; i < FrameRing+25; i++ {
		record(s, target, uint64(i+1))
	}
	if s.Frames() != FrameRing {
		t.Fatalf("got %d frames, want %d", s.Frames(), FrameRing)
	}
	if got := s.Frame(0).Seq; got != 26 {
		t.Errorf("the oldest frame is %d, want 26", got)
	}
	if got := s.Frame(FrameRing - 1).Seq; got != FrameRing+25 {
		t.Errorf("the newest frame is %d, want %d", got, FrameRing+25)
	}
	for i := 1; i < FrameRing; i++ {
		if s.Frame(i).Seq != s.Frame(i-1).Seq+1 {
			t.Fatalf("frame %d is %d after %d", i, s.Frame(i).Seq, s.Frame(i-1).Seq)
		}
	}
}

func TestSession_FrameOutOfRangePanics(t *testing.T) {
	s, _, _ := newSampleSession(t)
	defer func() {
		if recover() == nil {
			t.Error("Frame(0) of an empty session must panic")
		}
	}()
	s.Frame(0)
}

func TestSession_SelectedFrameFollowsTheWrap(t *testing.T) {
	s, target, _ := newSampleSession(t)
	for i := range FrameRing - 1 {
		record(s, target, uint64(i+1))
	}
	s.SelectFrame(5)
	record(s, target, FrameRing)
	if s.SelectedFrame != 5 {
		t.Fatalf("filling the ring moved the selection to %d", s.SelectedFrame)
	}
	seq := s.Frame(5).Seq
	record(s, target, FrameRing+1)
	if s.SelectedFrame != 4 || s.Frame(s.SelectedFrame).Seq != seq {
		t.Errorf("after the wrap the selection is %d, want 4 holding frame %d", s.SelectedFrame, seq)
	}
	s.SelectFrame(0)
	record(s, target, FrameRing+2)
	if s.SelectedFrame != -1 {
		t.Errorf("a selected frame that is overwritten must go, got %d", s.SelectedFrame)
	}
	record(s, target, FrameRing+3)
	if s.SelectedFrame != -1 {
		t.Errorf("no selection must stay none, got %d", s.SelectedFrame)
	}
}

func TestSession_GPUTimesMatchTheirFrames(t *testing.T) {
	s, target, _ := newSampleSession(t)
	s.EnableGPU()
	record(s, target, 1)
	s.SkipFrame() // a frame of the Inspector
	record(s, target, 3)
	s.SetPaused(true)
	record(s, target, 4) // paused: not kept, but timed
	s.SetPaused(false)
	record(s, target, 5)
	s.GPUTimes([]time.Duration{10, 20})
	s.GPUTimes([]time.Duration{30, 40, 50, 60}) // 60 has no frame
	want := map[uint64]time.Duration{1: 10, 3: 30, 5: 50}
	if s.Frames() != 3 {
		t.Fatalf("got %d frames, want 3", s.Frames())
	}
	for i := range s.Frames() {
		f := s.Frame(i)
		if f.GPU != want[f.Seq] {
			t.Errorf("frame %d: GPU %v, want %v", f.Seq, f.GPU, want[f.Seq])
		}
	}
	if len(s.gpuQueue) != 0 {
		t.Errorf("%d frames still wait", len(s.gpuQueue))
	}
}

func TestSession_GPUTimesSkipReusedRecords(t *testing.T) {
	s, target, _ := newSampleSession(t)
	s.EnableGPU()
	for i := range FrameRing + 1 {
		record(s, target, uint64(i+1))
	}
	times := make([]time.Duration, FrameRing+1)
	for i := range times {
		times[i] = time.Duration(i + 1)
	}
	s.GPUTimes(times)
	// The record of frame 1 now holds frame FrameRing+1: the time of frame 1
	// must not land on it.
	for i := range s.Frames() {
		f := s.Frame(i)
		if f.GPU != time.Duration(f.Seq) {
			t.Fatalf("frame %d: GPU %v, want %v", f.Seq, f.GPU, time.Duration(f.Seq))
		}
	}
}

func TestSession_GPUTimesWithoutTimingAreIgnored(t *testing.T) {
	s, target, _ := newSampleSession(t)
	record(s, target, 1)
	s.SkipFrame()
	s.GPUTimes([]time.Duration{10})
	if s.Frame(0).GPU != 0 || len(s.gpuQueue) != 0 {
		t.Error("a session without GPU timing must not queue frames")
	}
}

func TestSession_ProbeCountsTheFrame(t *testing.T) {
	s, target, _ := newSampleSession(t)
	var cv paint.Canvas
	cv.Reset()
	s.BeginFrame(target, 1)
	target.tree.MarkNeedsPaint()
	target.tree.Repaint(&cv)
	target.tree.Layout(geom.Tight(geom.Size{W: 120, H: 150}))
	s.EndFrame(target, [StageCount]time.Duration{StageBuild: 1, StagePaint: 2}, 7, true)
	f := s.Frame(0)
	if f.RecordedCount != 1 || len(f.Recorded) != 1 {
		t.Errorf("got %d layers recorded, want 1", f.RecordedCount)
	}
	if f.LaidOut == 0 {
		t.Error("a new layout must be counted")
	}
	if f.CPU() != 3 || f.Allocs != 7 || !f.Repainted || f.Target != target {
		t.Errorf("got %+v", f)
	}
	// Between frames the probe hears nothing.
	laidOut := f.LaidOut
	target.tree.Layout(geom.Tight(geom.Size{W: 100, H: 150}))
	if f.LaidOut != laidOut || s.current != nil {
		t.Error("a layout outside a frame must not be counted")
	}
}

func TestSession_PausedFramesAreNotProbed(t *testing.T) {
	s, target, _ := newSampleSession(t)
	s.BeginFrame(target, 1)
	s.SetPaused(true)
	s.EndFrame(target, [StageCount]time.Duration{}, 0, false)
	s.BeginFrame(target, 2)
	target.tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	s.EndFrame(target, [StageCount]time.Duration{}, 0, false)
	if s.Frames() != 1 {
		t.Errorf("got %d frames, want only the one before the pause", s.Frames())
	}
}

func TestSession_FlashesFade(t *testing.T) {
	s, target, host := newSampleSession(t)
	other := newFakeTarget("other", basic.SizedBox{}, geom.Size{W: 10, H: 10})
	host.targets = append(host.targets, other)
	s.SetShowRepaints(true)
	if s.Overlaid(target) {
		t.Fatal("no flash yet, yet the overlay draws")
	}
	if _, ok := s.Wake(); ok {
		t.Fatal("no flash yet, yet the loop must wake")
	}
	var cv paint.Canvas
	cv.Reset()
	s.BeginFrame(target, 1)
	target.tree.MarkNeedsPaint()
	target.tree.Repaint(&cv)
	s.EndFrame(target, [StageCount]time.Duration{}, 0, true)
	// The window recorded in full does not flash, only its layer.
	if len(s.flashes) != 1 {
		t.Fatalf("got %d flashes, want the layer only", len(s.flashes))
	}
	if !s.Overlaid(target) || s.Overlaid(other) {
		t.Error("only the window that flashes is overlaid")
	}
	if _, ok := s.Wake(); !ok {
		t.Error("a flash must wake the loop until it fades")
	}
	redraws := target.redraws
	s.Tick()
	if len(s.flashes) != 1 || target.redraws == redraws {
		t.Error("a fresh flash must stay and draw again")
	}
	cv.Reset()
	s.Overlay(target, &cv)
	if len(cv.DisplayList().Ops) == 0 {
		t.Error("the flashes must be drawn")
	}
	age(s, flashLife)
	s.Tick()
	if len(s.flashes) != 0 {
		t.Errorf("%d flashes outlived their life", len(s.flashes))
	}
	if s.Overlaid(target) {
		t.Error("a faded flash must not keep the overlay")
	}
	if _, ok := s.Wake(); ok {
		t.Error("nothing to fade, yet the loop wakes")
	}
	// Turning the tints off drops them at once.
	record(s, target, 2)
	s.BeginFrame(target, 3)
	s.EndFrame(target, [StageCount]time.Duration{}, 0, true)
	s.SetShowRepaints(false)
	if len(s.flashes) != 0 {
		t.Error("turning the tints off must drop the flashes")
	}
}

func TestSession_ForgetDropsAClosedWindow(t *testing.T) {
	s, target, host := newSampleSession(t)
	other := newFakeTarget("other", basic.SizedBox{}, geom.Size{W: 10, H: 10})
	host.targets = append(host.targets, other)
	s.SetShowRepaints(true)
	var cv paint.Canvas
	cv.Reset()
	s.BeginFrame(target, 1)
	target.tree.MarkNeedsPaint()
	target.tree.Repaint(&cv)
	s.EndFrame(target, [StageCount]time.Duration{}, 0, true)
	record(s, other, 2)
	s.SetPicking(true)
	s.Pointer(target, geom.Offset{X: 20, Y: 20}, PointerMove)
	s.SelectRenderObject(target, findRO[*core.RenderRepaintBoundary](target))
	changes := 0
	cancel := s.Listen(func() { changes++ })
	defer cancel()

	s.Forget(target)
	if changes == 0 {
		t.Error("forgetting a window must tell the Inspector")
	}
	if f := s.Frame(0); f.Target != nil || len(f.Rebuilt) != 0 || len(f.Recorded) != 0 {
		t.Errorf("the frame of the closed window still refers to it: %+v", f)
	}
	if s.Frame(1).Target != other {
		t.Error("the frames of the other window must stay")
	}
	if sel, el, ro := s.Selection(); sel != nil || !el.IsZero() || ro != nil {
		t.Error("the selection in the closed window must go")
	}
	if s.hoverTarget != nil || s.hovered != nil {
		t.Error("the hover in the closed window must go")
	}
	if s.hasFlash(target) {
		t.Error("the flashes of the closed window must go")
	}
	if s.Overlaid(target) {
		t.Error("nothing is left to draw over the closed window")
	}
}

func TestSession_PickHoversThenSelects(t *testing.T) {
	s, target, _ := newSampleSession(t)
	activated := 0
	s.SetActivator(func() { activated++ })
	at := geom.Offset{X: 20, Y: 20} // inside the padded box

	if s.Pointer(target, at, PointerMove) {
		t.Fatal("the pointer must reach the application while not picking")
	}
	s.SetPicking(true)
	if s.Overlaid(target) {
		t.Error("nothing is hovered yet")
	}
	redraws := target.redraws
	if !s.Pointer(target, at, PointerMove) {
		t.Fatal("picking must take the pointer")
	}
	if s.hovered == nil || s.hoverTarget != target || target.redraws != redraws+1 {
		t.Fatalf("hover %v, %d redraws", s.hovered, target.redraws-redraws)
	}
	if !s.Overlaid(target) {
		t.Error("the hovered object must be drawn over")
	}
	s.Pointer(target, at, PointerMove)
	if target.redraws != redraws+1 {
		t.Error("the same hover must not draw again")
	}
	s.Pointer(target, geom.Offset{}, PointerLeave)
	if s.hovered != nil || s.hoverTarget != nil {
		t.Error("leaving the window must clear the hover")
	}
	if s.Overlaid(target) {
		t.Error("nothing is hovered after the pointer left")
	}

	s.Pointer(target, at, PointerMove)
	want := s.hovered
	if !s.Pointer(target, at, PointerDown) {
		t.Fatal("the press that picks must not reach the application")
	}
	sel, el, ro := s.Selection()
	if sel != target || ro != want || el.IsZero() || el.RenderObject() != want {
		t.Errorf("got selection %v %v %v, want %v", sel, el, ro, want)
	}
	if s.Picking || s.hovered != nil {
		t.Error("a pick must end the pick mode")
	}
	if activated != 1 {
		t.Errorf("the Inspector was brought forward %d times, want 1", activated)
	}
	if !s.Overlaid(target) {
		t.Error("the selection must be drawn over its window")
	}
	// The rest of the press belongs to the pick too.
	if !s.Pointer(target, at, PointerMove) || !s.Pointer(target, at, PointerUp) {
		t.Error("the application must not see the release of the press that picked")
	}
	if s.Pointer(target, at, PointerMove) {
		t.Error("after the release the pointer is the application's again")
	}
}

func TestSession_PickSkipsHiddenObjects(t *testing.T) {
	s, target, _ := newSampleSession(t)
	s.SetPicking(true)
	// The Opacity of 0 holds the third box, at y 70 to 90.
	s.Pointer(target, geom.Offset{X: 20, Y: 80}, PointerMove)
	if s.hovered == nil {
		t.Fatal("the column must be hit")
	}
	if hiddenAt(s.hovered) {
		t.Errorf("picked %s, which is hidden", TypeName(s.hovered))
	}
}

func TestSession_EscapeLikeCancelKeepsTheSelection(t *testing.T) {
	s, target, _ := newSampleSession(t)
	box := findRO[*core.RenderRepaintBoundary](target)
	s.SelectRenderObject(target, box)
	s.SetPicking(true)
	s.Pointer(target, geom.Offset{X: 20, Y: 20}, PointerMove)
	s.SetPicking(false)
	if s.hovered != nil {
		t.Error("ending the pick mode must clear the hover")
	}
	if _, _, ro := s.Selection(); ro != box {
		t.Error("ending the pick mode must keep the selection")
	}
	if s.Pointer(target, geom.Offset{X: 20, Y: 20}, PointerDown) {
		t.Error("without a pick the press is the application's")
	}
}

func TestSession_OverlaidConditions(t *testing.T) {
	s, target, host := newSampleSession(t)
	other := newFakeTarget("other", basic.SizedBox{}, geom.Size{W: 10, H: 10})
	host.targets = append(host.targets, other)
	if s.Overlaid(target) || s.Overlaid(other) {
		t.Fatal("a fresh session draws over nothing")
	}
	s.SetShowBounds(true)
	if !s.Overlaid(target) || !s.Overlaid(other) {
		t.Error("the outlines go over every window")
	}
	if target.redraws == 0 || other.redraws == 0 {
		t.Error("turning the outlines on must draw every window again")
	}
	s.SetShowBounds(false)
	s.Select(target, target.tree.RootElement())
	if !s.Overlaid(target) || s.Overlaid(other) {
		t.Error("a selection goes over its window only")
	}
	s.Select(nil, core.ElementView{})
	if s.Overlaid(target) {
		t.Error("no selection, nothing drawn")
	}
	s.SetPicking(true)
	if s.Overlaid(target) {
		t.Error("picking without a hover draws nothing")
	}
}

func TestSession_SelectFindsBothSides(t *testing.T) {
	s, target, _ := newSampleSession(t)
	root := target.tree.RootElement()
	s.Select(target, root)
	if _, _, ro := s.Selection(); ro != root.NearestRenderObject() || ro == nil {
		t.Error("selecting an element must select its render object")
	}
	box := findRO[*core.RenderRepaintBoundary](target)
	s.SelectRenderObject(target, box)
	if _, el, _ := s.Selection(); el.RenderObject() != box {
		t.Error("selecting a render object must select its element")
	}
}

func TestSession_TellsAboutTheLastFramesOfABurst(t *testing.T) {
	ft := newFakeTarget("a", basic.SizedBox{Width: 10, Height: 10}, geom.Size{W: 10, H: 10})
	s := NewSession(&fakeHost{targets: []Target{ft}}, nil)
	told := 0
	s.Listen(func() { told++ })
	frame := func() {
		s.BeginFrame(ft, 1)
		s.EndFrame(ft, [StageCount]time.Duration{}, 0, false)
	}
	frame()
	frame()
	if told != 1 {
		t.Fatalf("two quick frames told %d times, want 1", told)
	}
	wait, ok := s.Wake()
	if !ok || wait > notifyEvery {
		t.Fatalf("Wake = %v, %v: the held frame needs a tick", wait, ok)
	}
	s.lastNotify -= notifyEvery
	s.Tick()
	if told != 2 {
		t.Errorf("Tick told %d times, want 2", told)
	}
	if _, ok := s.Wake(); ok {
		t.Error("Wake still asks for a tick")
	}
}
