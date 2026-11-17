//go:build gowakudebug

package gowaku

import (
	"runtime/metrics"
	"sync/atomic"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/inspect"
	"github.com/bankai-tensei/gowaku/platform"
)

// inspecting is true with -tags gowakudebug: F12 opens the Inspector. See
// docs/ADR/0073-inspector.md.
const inspecting = true

// openAtStart is set by OpenInspector before Run.
var openAtStart atomic.Bool

// OpenInspector opens the Inspector, a window that shows the trees, the
// frames and the pictures of the other windows. F12 in any window does the
// same, and closes it again. Before Run, it opens the Inspector with the first
// window. It is safe to call from any goroutine.
//
// It does something only in builds with -tags gowakudebug; in other builds it
// does nothing, and no code of the Inspector is linked in.
func OpenInspector() {
	ap := currentApp.Load()
	if ap == nil {
		openAtStart.Store(true)
		return
	}
	ap.post(func() { ap.openInspector() })
}

// gpuTimer is what the D3D11 and Vulkan devices offer to measure frames.
type gpuTimer interface {
	EnableGPUTiming() error
	GPUFrameTimes(times []time.Duration) []time.Duration
}

// inspectorState is the part of the application that serves the Inspector.
type inspectorState struct {
	session *inspect.Session
	win     *window
	// seq numbers every frame of every window.
	seq   uint64
	timer gpuTimer
	times []time.Duration
	// allocs reads the count of heap allocations.
	allocs []metrics.Sample
}

// frameClock measures the stages of one frame.
type frameClock struct {
	on     bool
	last   time.Duration
	stages [inspect.StageCount]time.Duration
	allocs uint64
}

func (c *frameClock) lap(s inspect.Stage) {
	if !c.on {
		return
	}
	now := frameNow()
	c.stages[s] += now - c.last
	c.last = now
}

// Title implements inspect.Target.
func (w *window) Title() string { return w.title }

// Tree implements inspect.Target.
func (w *window) Tree() *core.Tree { return w.tree }

// Size implements inspect.Target.
func (w *window) Size() geom.Size { return w.size }

// Redraw implements inspect.Target.
func (w *window) Redraw() { w.tree.MarkNeedsPaint() }

// inspectHost is the inspect.Host of the application.
type inspectHost struct{ ap *app }

// Targets implements inspect.Host. The windows are in the order they opened.
func (h inspectHost) Targets() []inspect.Target {
	ap := h.ap
	var ts []inspect.Target
	for _, w := range ap.windowsInOrder() {
		if w != ap.inspector.win && !w.closing {
			ts = append(ts, w)
		}
	}
	return ts
}

// Activate implements inspect.Host.
func (h inspectHost) Activate(t inspect.Target) { t.(*window).native.Activate() }

// windowsInOrder returns the windows sorted by their ID, which grows with each
// window.
func (ap *app) windowsInOrder() []*window {
	ws := make([]*window, 0, len(ap.windows))
	for _, w := range ap.windows {
		ws = append(ws, w)
	}
	for i := 1; i < len(ws); i++ {
		for j := i; j > 0 && ws[j].native.ID() < ws[j-1].native.ID(); j-- {
			ws[j], ws[j-1] = ws[j-1], ws[j]
		}
	}
	return ws
}

// inspectStart opens the Inspector at the start of Run if OpenInspector asked.
func (ap *app) inspectStart() {
	if openAtStart.Swap(false) {
		ap.openInspector()
	}
}

func (ap *app) openInspector() {
	st := &ap.inspector
	if st.win != nil {
		st.win.native.Activate()
		return
	}
	s := inspect.NewSession(inspectHost{ap}, ap.shaper)
	if t, ok := ap.backend.(gpuTimer); ok && t.EnableGPUTiming() == nil {
		st.timer = t
		// Times of frames before the session would be taken for its own.
		st.times = t.GPUFrameTimes(st.times[:0])
		s.EnableGPU()
	}
	st.session = s
	if st.allocs == nil {
		st.allocs = []metrics.Sample{{Name: "/gc/heap/allocs:objects"}}
	}
	w, err := ap.open(WindowOptions{Title: "gowaku Inspector", Width: 1180, Height: 780}, inspect.Inspector{Session: s})
	if err != nil {
		st.session = nil
		panic("gowaku: open the Inspector: " + err.Error())
	}
	w.onClose = ap.inspectorClosed
	st.win = w
	s.SetActivator(w.native.Activate)
}

func (ap *app) closeInspector() {
	if w := ap.inspector.win; w != nil {
		ap.requestClose(w)
	}
}

// inspectorClosed forgets the session when its window goes.
func (ap *app) inspectorClosed() {
	st := &ap.inspector
	for _, w := range ap.windows {
		w.tree.SetProbe(nil)
		w.tree.MarkNeedsPaint()
	}
	st.session, st.win = nil, nil
}

// inspectKey handles F12, and Escape while picking. It reports whether it took
// the key.
func (ap *app) inspectKey(_ *window, ev *platform.Event) bool {
	switch {
	case ev.Key == input.KeyF12 && ev.Modifiers == 0:
		if ap.inspector.win != nil {
			ap.closeInspector()
		} else {
			ap.openInspector()
		}
		return true
	case ev.Key == input.KeyEscape && ap.inspector.session != nil && ap.inspector.session.Picking:
		ap.inspector.session.SetPicking(false)
		return true
	}
	return false
}

// inspectPointer gives a pointer event of w to the pick mode, and reports
// whether it took it.
func (ap *app) inspectPointer(w *window, ev *platform.Event) bool {
	s := ap.inspector.session
	if s == nil || w == ap.inspector.win {
		return false
	}
	act := inspect.PointerMove
	switch ev.Kind {
	case platform.EventPointerMove:
	case platform.EventPointerDown:
		act = inspect.PointerDown
	case platform.EventPointerUp:
		act = inspect.PointerUp
	case platform.EventPointerLeave:
		act = inspect.PointerLeave
	default:
		return false
	}
	return s.Pointer(w, geom.Offset{X: ev.X / w.scale, Y: ev.Y / w.scale}, act)
}

// beginInspected starts the clock of a frame of w, and the record of it.
func (ap *app) beginInspected(w *window) frameClock {
	st := &ap.inspector
	st.seq++
	if st.session == nil {
		return frameClock{}
	}
	if w == st.win {
		st.session.SkipFrame()
		return frameClock{}
	}
	st.session.BeginFrame(w, st.seq)
	metrics.Read(st.allocs)
	return frameClock{on: true, last: frameNow(), allocs: st.allocs[0].Value.Uint64()}
}

// endInspected finishes the record of a frame of w.
func (ap *app) endInspected(w *window, c *frameClock, repainted bool) {
	st := &ap.inspector
	if !c.on || st.session == nil {
		return
	}
	metrics.Read(st.allocs)
	st.session.EndFrame(w, c.stages, st.allocs[0].Value.Uint64()-c.allocs, repainted)
}

// overlaid reports whether the Inspector draws over w this frame.
func (ap *app) overlaid(w *window) bool {
	s := ap.inspector.session
	return s != nil && w != ap.inspector.win && s.Overlaid(w)
}

// drawOverlay draws what the Inspector shows over w on its canvas.
func (ap *app) drawOverlay(w *window) {
	ap.inspector.session.Overlay(w, &w.canvas)
}

// inspectPumped runs after each pump: it collects the GPU times and fades the
// tints.
func (ap *app) inspectPumped() {
	st := &ap.inspector
	if st.session == nil {
		return
	}
	if st.timer != nil {
		st.times = st.timer.GPUFrameTimes(st.times[:0])
		st.session.GPUTimes(st.times)
	}
	st.session.Tick()
}

// inspectWake returns when the Inspector needs the loop to wake.
func (ap *app) inspectWake() (time.Duration, bool) {
	if s := ap.inspector.session; s != nil {
		return s.Wake()
	}
	return 0, false
}

// inspectForget drops what the Inspector knows of w, a window that closes.
func (ap *app) inspectForget(w *window) {
	if s := ap.inspector.session; s != nil && w != ap.inspector.win {
		s.Forget(w)
	}
}

const (
	stageBuild   = inspect.StageBuild
	stageLayout  = inspect.StageLayout
	stagePaint   = inspect.StagePaint
	stageBatch   = inspect.StageBatch
	stagePresent = inspect.StagePresent
)
