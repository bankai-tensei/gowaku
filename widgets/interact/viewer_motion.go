package interact

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
)

const (
	// flingStiffness sets how fast a fling slows down. A critical spring aimed
	// at velocity/ω ahead, ω = √stiffness, decays like friction: speed falls
	// by e every 1/ω seconds and the view travels velocity/ω in all.
	flingStiffness = 12
	// minFlingSpeed and maxFlingSpeed bound the speed of a release that
	// flings, in logical pixels per second.
	minFlingSpeed = 60
	maxFlingSpeed = 8000
	// tweenStiffness sets the speed of an animated move of the controller or
	// the keyboard: about 0.4 s.
	tweenStiffness = 300
	// tweenUnits is the length of the spring of a tween, so that its rest
	// distance of 0.25 is a small part of the way.
	tweenUnits = 1000
	// viewerGlideTime and viewerGlideLead are those of Scroll: a wheel notch
	// covers 63% of its way in viewerGlideTime, and its first frame has
	// already run for viewerGlideLead.
	viewerGlideTime = 40 * time.Millisecond
	viewerGlideLead = time.Second / 60
	// velocityWindow is how far back a release looks for its speed, and
	// velocityStale how long the pointer may rest before a release that then
	// does not fling, in milliseconds.
	velocityWindow = 100
	velocityStale  = 40
)

// viewerMotionKind is what moves a viewer.
type viewerMotionKind uint8

const (
	motionNone viewerMotionKind = iota
	// motionGlide is a wheel notch or an arrow key: the offset approaches its
	// target fastest at first.
	motionGlide
	// motionFling glides on after a release, slowing down.
	motionFling
	// motionTween moves scale and offset together to a target, for the
	// controller, the keyboard and a double click.
	motionTween
)

// viewerMotion is the motion of a viewer under way. It holds no pointers and
// allocates nothing.
type viewerMotion struct {
	kind viewerMotionKind
	// fresh says that the motion takes its start from the next tick, as the
	// pointer has no clock of the frames.
	fresh   bool
	started time.Duration

	// from and to are the ends of a glide.
	from, to geom.Offset
	// x and y run a fling; xDone and yDone say that it hit an edge.
	x, y         motion.SpringSimulation
	xDone, yDone bool
	// A tween runs progress from 0 to tweenUnits. The scale changes evenly in
	// proportion, and the point of the child in the middle of the viewer
	// moves in a straight line, which reads as one zoom rather than a swoop.
	progress           motion.SpringSimulation
	fromScale, toScale float32
	fromMid, toMid     geom.Offset
	toOffset           geom.Offset
}

func (m *viewerMotion) begin(kind viewerMotionKind) {
	m.kind, m.fresh = kind, true
}

// glide starts a glide from cur to target.
func (m *viewerMotion) glide(cur, target geom.Offset) {
	m.begin(motionGlide)
	m.from, m.to = cur, target
}

// fling starts a fling from off at velocity v.
func (m *viewerMotion) fling(off, v geom.Offset) {
	m.begin(motionFling)
	spring := motion.CriticalSpring(flingStiffness)
	omega := math.Sqrt(flingStiffness)
	px, py := float64(off.X), float64(off.Y)
	vx, vy := float64(v.X), float64(v.Y)
	m.x = motion.NewSpringSimulation(spring, px, vx, px+vx/omega)
	m.y = motion.NewSpringSimulation(spring, py, vy, py+vy/omega)
	m.xDone, m.yDone = false, false
}

// tween starts a move from one scale and offset to another in a view.
func (m *viewerMotion) tween(view geom.Size, s0 float32, o0 geom.Offset, s1 float32, o1 geom.Offset) {
	m.begin(motionTween)
	m.progress = motion.NewSpringSimulation(motion.CriticalSpring(tweenStiffness), 0, 0, tweenUnits)
	mid := geom.Offset{X: view.W / 2, Y: view.H / 2}
	m.fromScale, m.toScale = s0, s1
	m.fromMid = geom.Offset{X: (mid.X - o0.X) / s0, Y: (mid.Y - o0.Y) / s0}
	m.toMid = geom.Offset{X: (mid.X - o1.X) / s1, Y: (mid.Y - o1.Y) / s1}
	m.toOffset = o1
}

// at returns the scale and offset at now, and whether the motion goes on.
// scale and off are where the view is.
func (m *viewerMotion) at(now time.Duration, view geom.Size, scale float32, off geom.Offset) (float32, geom.Offset, bool) {
	if m.fresh {
		m.fresh, m.started = false, now
		if m.kind == motionGlide {
			m.started -= viewerGlideLead
		}
	}
	elapsed := now - m.started
	switch m.kind {
	case motionGlide:
		k := float32(math.Exp(-elapsed.Seconds() / viewerGlideTime.Seconds()))
		left := geom.Offset{X: (m.from.X - m.to.X) * k, Y: (m.from.Y - m.to.Y) * k}
		if geom.Abs32(left.X) < 0.25 && geom.Abs32(left.Y) < 0.25 {
			return scale, m.to, false
		}
		return scale, m.to.Add(left), true
	case motionFling:
		px, _ := m.x.At(elapsed)
		py, _ := m.y.At(elapsed)
		if !m.xDone {
			off.X = float32(px)
		}
		if !m.yDone {
			off.Y = float32(py)
		}
		more := (!m.xDone && !m.x.Settled(elapsed)) || (!m.yDone && !m.y.Settled(elapsed))
		return scale, off, more
	case motionTween:
		if m.progress.Settled(elapsed) {
			return m.toScale, m.toOffset, false
		}
		p, _ := m.progress.At(elapsed)
		t := geom.Clamp32(float32(p/tweenUnits), 0, 1)
		s := m.fromScale * float32(math.Pow(float64(m.toScale/m.fromScale), float64(t)))
		mid := geom.Offset{X: m.fromMid.X + (m.toMid.X-m.fromMid.X)*t, Y: m.fromMid.Y + (m.toMid.Y-m.fromMid.Y)*t}
		return s, geom.Offset{X: view.W/2 - mid.X*s, Y: view.H/2 - mid.Y*s}, true
	}
	return scale, off, false
}

// velocityTracker keeps the last places of a drag, to measure the speed of
// its release.
type velocityTracker struct {
	pos  [8]geom.Offset
	at   [8]uint32
	n    int
	next int
}

func (v *velocityTracker) reset() { v.n, v.next = 0, 0 }

func (v *velocityTracker) add(p geom.Offset, t uint32) {
	v.pos[v.next], v.at[v.next] = p, t
	v.next = (v.next + 1) % len(v.pos)
	v.n = min(v.n+1, len(v.pos))
}

// velocity returns the speed of the drag at now, in pixels per second, over
// the last velocityWindow milliseconds. A pointer that rested before the
// release has none.
func (v *velocityTracker) velocity(now uint32) geom.Offset {
	if v.n < 2 {
		return geom.Offset{}
	}
	newest := (v.next + len(v.pos) - 1) % len(v.pos)
	if age(now, v.at[newest]) > velocityStale {
		return geom.Offset{}
	}
	oldest := newest
	for k := 1; k < v.n; k++ {
		i := (newest + len(v.pos) - k) % len(v.pos)
		if age(now, v.at[i]) > velocityWindow {
			break
		}
		oldest = i
	}
	// The millisecond clock wraps around, so the difference is taken in uint32.
	dt := float32(v.at[newest]-v.at[oldest]) / 1000
	if dt <= 0 {
		return geom.Offset{}
	}
	d := v.pos[newest].Sub(v.pos[oldest])
	return geom.Offset{X: d.X / dt, Y: d.Y / dt}
}

// age returns how long before now t was, on the wrapping millisecond clock.
func age(now, t uint32) int32 { return int32(now - t) }
