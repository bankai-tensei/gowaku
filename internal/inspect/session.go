//go:build gowakudebug

// Package inspect is the Inspector of builds with -tags gowakudebug: a window
// that shows the trees, the frames and the pictures of the other windows of the
// application. Package gowaku drives it; nothing else imports it. See
// docs/ADR/0073-inspector.md.
package inspect

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

// Target is a window under inspection. Package gowaku implements it.
type Target interface {
	// Title returns the title of the window.
	Title() string
	// Tree returns the tree of the window.
	Tree() *core.Tree
	// Size returns the client area in logical pixels.
	Size() geom.Size
	// Redraw asks for a frame of the window.
	Redraw()
}

// Host is what the session needs of the application.
type Host interface {
	// Targets returns the open windows, the Inspector excluded, oldest first.
	Targets() []Target
	// Activate brings the window of t to the front.
	Activate(t Target)
}

// Stage is a step of a frame.
type Stage uint8

// The stages, in the order a frame runs them.
const (
	StageBuild Stage = iota
	StageLayout
	StagePaint
	// StageBatch is the conversion of the display list into GPU commands, with
	// the upload of new glyphs.
	StageBatch
	// StagePresent is the submission to the GPU, the wait for vsync included.
	StagePresent
	StageCount
)

// maxRecorded is how many elements and layers a frame record names. Beyond it
// the record only counts them.
const maxRecorded = 64

// Frame is the record of one frame of a target.
type Frame struct {
	// Seq numbers the frames of every window, the Inspector's included, in the
	// order they were presented.
	Seq    uint64
	Target Target
	// At is when the frame began, from the start of the session.
	At     time.Duration
	Stages [StageCount]time.Duration
	// GPU is the time the GPU spent on the frame, or 0 while it is unknown or
	// when the device cannot measure it.
	GPU time.Duration
	// Allocs counts the heap allocations of the frame, of the whole process.
	Allocs uint64
	// Rebuilt names the first elements that built; RebuiltCount counts them all.
	Rebuilt      []core.ElementView
	RebuiltCount int
	// LaidOut counts the render objects that ran Layout.
	LaidOut int
	// Recorded names the first layers that were recorded again; RecordedCount
	// counts them all.
	Recorded      []*core.RenderRepaintBoundary
	RecordedCount int
	// Repainted means that the picture of the window was recorded again, not
	// only some of its layers.
	Repainted bool
}

// Work returns the time of the stages that do the work of the frame: all but
// StagePresent, whose time is mostly the wait for vsync. It is what decides
// whether a frame fits its budget.
func (f *Frame) Work() time.Duration {
	var d time.Duration
	for s := range StagePresent {
		d += f.Stages[s]
	}
	return d
}

// CPU returns the sum of the stages.
func (f *Frame) CPU() time.Duration {
	var d time.Duration
	for _, s := range f.Stages {
		d += s
	}
	return d
}

// FrameRing is how many frames the session keeps.
const FrameRing = 240

// flashLife is how long a recorded layer stays tinted.
const flashLife = 500 * time.Millisecond

type flash struct {
	target Target
	layer  *core.RenderRepaintBoundary
	at     time.Duration
}

// Session is the state of the Inspector, shared by its window and the hooks in
// package gowaku. It lives on the UI goroutine.
type Session struct {
	host   Host
	shaper *text.Shaper
	start  time.Time

	frames [FrameRing]Frame
	// next is where the next frame goes, and count how many are kept.
	next, count int
	// current is the frame being recorded, or nil.
	current *Frame
	// Paused stops the recording, so that the frames stay to be read.
	Paused bool

	// gpu holds the frames that wait for their GPU time, by Seq; a nil entry
	// is a frame of the Inspector itself.
	gpuQueue []gpuWait
	gpuOn    bool

	// ShowBounds outlines every render object, and ShowRepaints tints the
	// layers that were recorded again.
	ShowBounds   bool
	ShowRepaints bool
	flashes      []flash

	// Picking means that a click in a target selects what is under it.
	Picking bool
	// pressed means that the press that picked is still down: the application
	// did not see it, so it must not see what follows it either.
	pressed bool
	// hovered is what the pointer is over while picking.
	hoverTarget Target
	hovered     core.RenderObject
	// selection is what the trees and the details show.
	selTarget  Target
	selElement core.ElementView
	selRO      core.RenderObject
	// SelectedFrame is the index in Frames of the frame whose details show, or
	// -1.
	SelectedFrame int

	// labels keeps the shaped labels of the overlay.
	labels [labelCount]overlayLabel

	// activate brings the Inspector window forward.
	activate func()

	listeners map[int]func()
	nextID    int
	// lastNotify throttles the notifications of frames, and notifyPending
	// says that frames came since the last one, which Tick tells about when
	// the throttle allows: the last frames of a burst must show too.
	lastNotify    time.Duration
	notifyPending bool
}

// notifyEvery is the shortest time between two notifications of frames.
const notifyEvery = 100 * time.Millisecond

type gpuWait struct {
	seq   uint64
	frame *Frame
}

// NewSession returns a session for the windows of host. shaper draws the labels
// of the overlay.
func NewSession(host Host, shaper *text.Shaper) *Session {
	// The first frame is told about at once.
	return &Session{host: host, shaper: shaper, start: time.Now(), SelectedFrame: -1, lastNotify: -notifyEvery}
}

// Host returns the host of the session.
func (s *Session) Host() Host { return s.host }

// Now returns the time since the session began.
func (s *Session) Now() time.Duration { return time.Since(s.start) }

// Listen calls fn when what the Inspector shows changed, until the returned
// function is called.
func (s *Session) Listen(fn func()) (cancel func()) {
	if s.listeners == nil {
		s.listeners = make(map[int]func())
	}
	id := s.nextID
	s.nextID++
	s.listeners[id] = fn
	return func() { delete(s.listeners, id) }
}

// Changed tells the listeners that something changed.
func (s *Session) Changed() {
	for _, fn := range s.listeners {
		fn()
	}
}

// notifyThrottled tells the listeners about new frames, ten times a second at
// most, so that the Inspector does not take the time it measures.
func (s *Session) notifyThrottled() {
	now := s.Now()
	if now-s.lastNotify < notifyEvery {
		s.notifyPending = true
		return
	}
	s.lastNotify, s.notifyPending = now, false
	s.Changed()
}

// Frames returns how many frames are kept.
func (s *Session) Frames() int { return s.count }

// Frame returns frame i, counted from the oldest kept.
func (s *Session) Frame(i int) *Frame {
	if i < 0 || i >= s.count {
		panic("gowaku: inspect.Session.Frame out of range")
	}
	return &s.frames[(s.next-s.count+i+FrameRing)%FrameRing]
}

// BeginFrame starts the record of a frame of t. seq numbers the frame among
// all frames.
func (s *Session) BeginFrame(t Target, seq uint64) {
	if s.Paused {
		s.current = nil
		t.Tree().SetProbe(nil)
		return
	}
	f := &s.frames[s.next]
	rebuilt, recorded := f.Rebuilt[:0], f.Recorded[:0]
	if rebuilt == nil {
		rebuilt = make([]core.ElementView, 0, maxRecorded)
		recorded = make([]*core.RenderRepaintBoundary, 0, maxRecorded)
	}
	// With the ring full, the slot holds the oldest frame, which leaves the
	// ring now rather than at EndFrame: Frame never shows a record half
	// written, and the indices of the others, a selection among them, shift
	// by one at once.
	if s.count == FrameRing {
		s.count--
		if s.SelectedFrame >= 0 {
			s.SelectedFrame--
		}
	}
	*f = Frame{Seq: seq, Target: t, At: s.Now(), Rebuilt: rebuilt, Recorded: recorded}
	s.current = f
	t.Tree().SetProbe(s)
}

// EndFrame finishes the record begun by BeginFrame.
func (s *Session) EndFrame(t Target, stages [StageCount]time.Duration, allocs uint64, repainted bool) {
	f := s.current
	if f == nil {
		if s.gpuOn {
			s.gpuQueue = append(s.gpuQueue, gpuWait{})
		}
		return
	}
	s.current = nil
	f.Stages, f.Allocs, f.Repainted = stages, allocs, repainted
	s.next = (s.next + 1) % FrameRing
	if s.count < FrameRing {
		s.count++
	}
	if s.gpuOn {
		s.gpuQueue = append(s.gpuQueue, gpuWait{seq: f.Seq, frame: f})
	}
	// A picture recorded again in full does not flash: it would light up the
	// whole window on every change. The frame details say so instead.
	if s.ShowRepaints {
		now := s.Now()
		for _, l := range f.Recorded {
			s.flashes = append(s.flashes, flash{target: t, layer: l, at: now})
		}
	}
	s.notifyThrottled()
}

// SkipFrame accounts for a frame that is not recorded, such as one of the
// Inspector, so that the GPU times still match their frames.
func (s *Session) SkipFrame() {
	if s.gpuOn {
		s.gpuQueue = append(s.gpuQueue, gpuWait{})
	}
}

// EnableGPU says that the device measures frames, from the next one on.
func (s *Session) EnableGPU() { s.gpuOn = true }

// GPUTimes hands over the GPU times of the oldest frames that wait for them.
func (s *Session) GPUTimes(times []time.Duration) {
	for _, d := range times {
		if len(s.gpuQueue) == 0 {
			return
		}
		w := s.gpuQueue[0]
		s.gpuQueue = s.gpuQueue[1:]
		// The ring may have reused the record since.
		if w.frame != nil && w.frame.Seq == w.seq {
			w.frame.GPU = d
		}
	}
}

// Rebuilt implements core.Probe.
func (s *Session) Rebuilt(el core.ElementView) {
	if f := s.current; f != nil {
		f.RebuiltCount++
		if len(f.Rebuilt) < maxRecorded {
			f.Rebuilt = append(f.Rebuilt, el)
		}
	}
}

// LaidOut implements core.Probe.
func (s *Session) LaidOut(core.RenderObject) {
	if f := s.current; f != nil {
		f.LaidOut++
	}
}

// Recorded implements core.Probe.
func (s *Session) Recorded(r *core.RenderRepaintBoundary) {
	if f := s.current; f != nil {
		f.RecordedCount++
		if len(f.Recorded) < maxRecorded {
			f.Recorded = append(f.Recorded, r)
		}
	}
}

// Overlaid reports whether the overlay draws anything over t, which then
// repaints in full each frame.
func (s *Session) Overlaid(t Target) bool {
	return s.ShowBounds || s.ShowRepaints && s.hasFlash(t) ||
		s.Picking && s.hoverTarget == t || s.selTarget == t && s.selRO != nil
}

// Wake returns how long until the session needs Tick: while a flash fades,
// or when frames wait to be told about.
func (s *Session) Wake() (time.Duration, bool) {
	wait, ok := time.Duration(0), false
	if s.notifyPending {
		wait, ok = max(0, notifyEvery-(s.Now()-s.lastNotify)), true
	}
	if len(s.flashes) > 0 && (!ok || wait > 16*time.Millisecond) {
		wait, ok = 16*time.Millisecond, true
	}
	return wait, ok
}

// Tick tells about frames the throttle held back, drops the flashes that
// faded, and asks the targets that show flashes for a frame.
func (s *Session) Tick() {
	if s.notifyPending && s.Now()-s.lastNotify >= notifyEvery {
		s.notifyThrottled()
	}
	if len(s.flashes) == 0 {
		return
	}
	now := s.Now()
	kept := s.flashes[:0]
	for _, f := range s.flashes {
		if now-f.at < flashLife {
			kept = append(kept, f)
		}
	}
	clear(s.flashes[len(kept):])
	s.flashes = kept
	for _, t := range s.host.Targets() {
		t.Redraw()
	}
}

// Forget drops what refers to t, a window that closed.
func (s *Session) Forget(t Target) {
	if s.hoverTarget == t {
		s.hoverTarget, s.hovered = nil, nil
	}
	if s.selTarget == t {
		s.selTarget, s.selElement, s.selRO = nil, core.ElementView{}, nil
	}
	s.pressed = false
	s.labels = [labelCount]overlayLabel{}
	for i := range s.frames {
		f := &s.frames[i]
		if f.Target != t {
			continue
		}
		f.Target = nil
		clear(f.Rebuilt)
		f.Rebuilt = f.Rebuilt[:0]
		clear(f.Recorded)
		f.Recorded = f.Recorded[:0]
	}
	s.flashes = deleteFlashes(s.flashes, t)
	s.Changed()
}

func deleteFlashes(fs []flash, t Target) []flash {
	kept := fs[:0]
	for _, f := range fs {
		if f.target != t {
			kept = append(kept, f)
		}
	}
	clear(fs[len(kept):])
	return kept
}

// Selection returns the selected element and render object, and their window.
// The element is the zero view when only a render object is selected that no
// element owns, which does not happen in practice.
func (s *Session) Selection() (Target, core.ElementView, core.RenderObject) {
	return s.selTarget, s.selElement, s.selRO
}

// Select selects el of t, and the render object nearest it.
func (s *Session) Select(t Target, el core.ElementView) {
	s.selTarget, s.selElement = t, el
	s.selRO = nil
	if !el.IsZero() && el.Mounted() {
		s.selRO = el.NearestRenderObject()
	}
	s.redrawAll()
	s.Changed()
}

// SelectRenderObject selects ro of t, and the element that owns it.
func (s *Session) SelectRenderObject(t Target, ro core.RenderObject) {
	s.selTarget, s.selRO = t, ro
	s.selElement = core.ElementView{}
	if t != nil && ro != nil {
		s.selElement = t.Tree().ElementOf(ro)
	}
	s.redrawAll()
	s.Changed()
}

// SetPicking turns the pick mode on or off.
func (s *Session) SetPicking(on bool) {
	s.Picking = on
	if !on {
		s.hoverTarget, s.hovered = nil, nil
	}
	s.redrawAll()
	s.Changed()
}

// SetShowBounds turns the outlines on or off.
func (s *Session) SetShowBounds(on bool) {
	s.ShowBounds = on
	s.redrawAll()
	s.Changed()
}

// SetShowRepaints turns the tints of recorded layers on or off.
func (s *Session) SetShowRepaints(on bool) {
	s.ShowRepaints = on
	if !on {
		clear(s.flashes)
		s.flashes = s.flashes[:0]
	}
	s.redrawAll()
	s.Changed()
}

// SetPaused stops or resumes the recording.
func (s *Session) SetPaused(on bool) {
	s.Paused = on
	s.Changed()
}

// SelectFrame shows the details of frame i, or none with -1.
func (s *Session) SelectFrame(i int) {
	s.SelectedFrame = i
	s.Changed()
}

func (s *Session) redrawAll() {
	for _, t := range s.host.Targets() {
		t.Redraw()
	}
}

// PointerAction is what a pointer event of a target does.
type PointerAction uint8

// The pointer actions.
const (
	PointerMove PointerAction = iota
	PointerDown
	PointerUp
	// PointerLeave is the pointer leaving the window.
	PointerLeave
)

// Pointer takes a pointer event of t while picking, at pos in logical pixels,
// and reports whether it took it, so that the application does not see it.
// After the press that picks, it also takes the events up to the release, as
// the application never saw that press.
func (s *Session) Pointer(t Target, pos geom.Offset, act PointerAction) bool {
	if !s.Picking {
		if !s.pressed {
			return false
		}
		if act == PointerUp || act == PointerLeave {
			s.pressed = false
		}
		return true
	}
	if act == PointerLeave {
		if s.hoverTarget == t {
			s.hoverTarget, s.hovered = nil, nil
			t.Redraw()
		}
		return true
	}
	ro := deepest(t.Tree(), pos)
	if ro != s.hovered || s.hoverTarget != t {
		s.hoverTarget, s.hovered = t, ro
		t.Redraw()
	}
	if act == PointerDown && ro != nil {
		s.Picking, s.pressed = false, true
		s.hoverTarget, s.hovered = nil, nil
		s.SelectRenderObject(t, ro)
		s.Activate()
	}
	return true
}

// SetActivator gives the function that brings the Inspector window forward,
// which a pick calls.
func (s *Session) SetActivator(fn func()) { s.activate = fn }

// Activate brings the Inspector window forward.
func (s *Session) Activate() {
	if s.activate != nil {
		s.activate()
	}
}

// deepest returns the deepest render object whose box holds pos, or nil. It
// goes by geometry, not by the hit test of the pointer: a pick shows what is
// drawn there, also a Padding or an Align, which take no press over their
// empty space (ADR 0142). What a Hider hides, and what a Clipper cuts away at
// pos, is skipped; of overlapping children the one painted last wins.
func deepest(t *core.Tree, pos geom.Offset) core.RenderObject {
	root := t.RootRenderObject()
	if root == nil {
		return nil
	}
	return deepestIn(root, pos)
}

func deepestIn(ro core.RenderObject, pos geom.Offset) core.RenderObject {
	if hides(ro) {
		return nil
	}
	children := true
	if c, ok := ro.(core.Clipper); ok {
		if clip, ok := c.ChildClip(); ok && !core.LocalRectToGlobal(ro, clip).Contains(pos) {
			children = false
		}
	}
	if children {
		for i := ro.ChildCount() - 1; i >= 0; i-- {
			if c := ro.ChildAt(i); c != nil {
				if hit := deepestIn(c, pos); hit != nil {
					return hit
				}
			}
		}
	}
	size := ro.Size()
	if core.LocalRectToGlobal(ro, geom.Rect{W: size.W, H: size.H}).Contains(pos) {
		return ro
	}
	return nil
}
