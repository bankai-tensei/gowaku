package layout

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// hiddenPoll is how often a paused animation in a hidden subtree looks
// whether it shows again. Nothing tells a render object that an ancestor
// stopped hiding it, and asking costs a walk up the tree, without a frame.
const hiddenPoll = 200 * time.Millisecond

// RenderAnimatedImage draws the frames of an animation one after the other,
// at their delays. It is a RenderImage whose Source is the current frame.
//
// A ticker of its own moves it on, and sleeps between frames, so that the
// window draws only when the frame changes; a change repaints the object and
// nothing else. It pauses while an ancestor hides it (core.Hider) and stops
// when it leaves the tree or its loops are over. See
// docs/ADR/0045-shaders-svg-animated-images.md.
type RenderAnimatedImage struct {
	RenderImage
	anim *paint.Animation
	// still holds the first frame and keeps the ticker stopped.
	still bool

	ticker *core.Ticker
	frame  int
	// elapsed is how far the animation has played, counted while it runs.
	elapsed time.Duration
	// anchor is the ticker time at which the animation would have started
	// had it never paused, valid while anchored.
	anchor   time.Duration
	anchored bool
	ended    bool
}

// Animation returns the animation that SetAnimation set.
func (r *RenderAnimatedImage) Animation() *paint.Animation { return r.anim }

// Frame returns the index of the frame on show.
func (r *RenderAnimatedImage) Frame() int { return r.frame }

// SetAnimation starts a over from its first frame, unless it is the
// animation already shown.
func (r *RenderAnimatedImage) SetAnimation(a *paint.Animation) {
	if r.anim == a {
		return
	}
	old := r.Source
	r.anim = a
	r.restart()
	if old == nil || r.Source == nil || old.Size() != r.Source.Size() {
		r.MarkNeedsLayout()
	}
}

// SetStill makes the object hold the first frame, as for a window that asks
// for less motion, or lets it play again from the start.
func (r *RenderAnimatedImage) SetStill(still bool) {
	if r.still == still {
		return
	}
	r.still = still
	r.restart()
}

// restart shows the first frame and lets the next paint start the ticker.
func (r *RenderAnimatedImage) restart() {
	r.frame, r.elapsed, r.anchored, r.ended = 0, 0, false, false
	r.Source = nil
	if r.anim != nil && len(r.anim.Frames) > 0 {
		r.Source = r.anim.Frames[0]
	}
	if r.ticker != nil {
		r.ticker.Stop()
	}
	r.MarkNeedsPaint()
}

// Paint implements core.RenderObject. The first paint starts the ticker, and
// so does the first after a restart.
func (r *RenderAnimatedImage) Paint(cv *paint.Canvas, off geom.Offset) {
	r.RenderImage.Paint(cv, off)
	if r.still || r.ended || r.anim == nil || len(r.anim.Frames) < 2 {
		return
	}
	if r.ticker == nil {
		r.ticker = r.NewTicker(r.tick)
	}
	if !r.ticker.Running() {
		r.anchored = false
		r.ticker.Start()
	}
}

func (r *RenderAnimatedImage) tick(now time.Duration) {
	if !r.Attached() || r.still || r.anim == nil {
		r.ticker.Stop()
		return
	}
	if r.hidden() {
		// Keep the place in the animation, and look again later. The time
		// until the ticker noticed counts as played.
		if r.anchored {
			r.elapsed = now - r.anchor
		}
		r.anchored = false
		r.ticker.SleepUntil(now + hiddenPoll)
		return
	}
	if !r.anchored {
		r.anchor, r.anchored = now-r.elapsed, true
	}
	r.elapsed = now - r.anchor
	frame, next, ended := r.anim.FrameAt(r.elapsed)
	if frame != r.frame {
		r.frame = frame
		r.Source = r.anim.Frames[frame]
		r.MarkNeedsPaint()
	}
	if ended {
		r.ended = true
		r.ticker.Stop()
		return
	}
	r.ticker.SleepUntil(r.anchor + next)
}

// hidden reports whether an ancestor hides the object.
func (r *RenderAnimatedImage) hidden() bool {
	for p := core.RenderObject(r); p != nil; p = p.Parent() {
		if h, ok := p.(core.Hider); ok && h.Hidden() {
			return true
		}
	}
	return false
}
