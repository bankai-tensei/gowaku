package motion

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
)

// TickerSource makes tickers. core.StateBase and core.RenderBase are the two.
type TickerSource interface {
	NewTicker(fn func(now time.Duration)) *core.Ticker
}

// Status says where a controller stands.
type Status uint8

// The statuses.
const (
	// Dismissed is at rest at 0.
	Dismissed Status = iota
	// Forwarding is on its way to 1.
	Forwarding
	// Reversing is on its way to 0.
	Reversing
	// Completed is at rest at 1.
	Completed
)

// String returns the name of the status.
func (s Status) String() string {
	switch s {
	case Dismissed:
		return "dismissed"
	case Forwarding:
		return "forwarding"
	case Reversing:
		return "reversing"
	case Completed:
		return "completed"
	}
	return "unknown"
}

// AnimationController runs a value between 0 and 1 over time. Set the fields,
// call Attach in InitState and Dispose in Dispose.
//
// A new motion starts from the value of the moment, so that interrupting one
// never makes a jump. A full run takes Duration, and part of the way takes that
// part of it.
type AnimationController struct {
	// Duration is how long a full run takes. Zero jumps to the end.
	Duration time.Duration
	// ReverseDuration replaces Duration on the way back, unless it is zero.
	ReverseDuration time.Duration
	// Curve shapes Value. Nil is Linear.
	Curve Curve
	// ReverseCurve replaces Curve on the way back, unless it is nil.
	ReverseCurve Curve
	// OnChange runs on every step of the motion, including the last.
	OnChange func()
	// OnStatus runs when the status changes, after OnChange.
	OnStatus func(s Status)

	ticker *core.Ticker
	// raw is the linear progress, and from and to the ends of this run.
	raw, from, to float32
	span          time.Duration
	// started is when the run began. It comes from the first tick, because
	// what starts a run has no clock.
	started time.Duration
	fresh   bool
	status  Status
	// repeat runs for ever, and bounce goes back and forth while doing so.
	repeat, bounce bool
	// epoch counts the runs, jumps and disposals. A callback that starts one
	// changes it, which tells the code that called the callback to leave the
	// state alone.
	epoch uint32
}

// Attach connects the controller to its source of frames.
func (c *AnimationController) Attach(src TickerSource) { c.ticker = src.NewTicker(c.tick) }

// Dispose stops the motion for good.
func (c *AnimationController) Dispose() {
	c.epoch++
	c.repeat = false
	c.ticker.Stop()
}

// Value returns the curved progress.
func (c *AnimationController) Value() float32 {
	curve := c.Curve
	if c.status == Reversing && c.ReverseCurve != nil {
		curve = c.ReverseCurve
	}
	if curve == nil || c.raw <= 0 || c.raw >= 1 {
		return c.raw
	}
	return curve(c.raw)
}

// Progress returns the linear progress, without the curve.
func (c *AnimationController) Progress() float32 { return c.raw }

// Status returns where the controller stands.
func (c *AnimationController) Status() Status { return c.status }

// Animating reports whether the value is moving.
func (c *AnimationController) Animating() bool { return c.ticker.Running() }

// SetValue jumps to v without any animation. The status follows, but neither
// callback runs.
func (c *AnimationController) SetValue(v float32) {
	c.epoch++
	c.ticker.Stop()
	c.repeat = false
	c.raw = min(max(v, 0), 1)
	switch {
	case c.raw <= 0:
		c.status = Dismissed
	case c.raw >= 1:
		c.status = Completed
	default:
		c.status = Forwarding
	}
}

// Forward runs to 1.
func (c *AnimationController) Forward() {
	c.repeat = false
	c.run(1, c.Duration, Forwarding)
}

// Reverse runs to 0.
func (c *AnimationController) Reverse() {
	c.repeat = false
	d := c.Duration
	if c.ReverseDuration > 0 {
		d = c.ReverseDuration
	}
	c.run(0, d, Reversing)
}

// Restart runs from 0 to 1, whatever the value was.
func (c *AnimationController) Restart() {
	c.raw = 0
	c.Forward()
}

// Repeat runs until Stop. With bounce it goes back and forth, and otherwise it
// starts over from 0 each time.
func (c *AnimationController) Repeat(bounce bool) {
	// Set before the run, whose callbacks may stop it or start another: what
	// they did must stand. A run that jumps to its end clears repeat itself.
	c.repeat, c.bounce = true, bounce
	c.run(1, c.Duration, Forwarding)
}

// Stop freezes the value where it is.
func (c *AnimationController) Stop() {
	c.repeat = false
	c.ticker.Stop()
}

func (c *AnimationController) run(to float32, full time.Duration, status Status) {
	distance := to - c.raw
	if distance < 0 {
		distance = -distance
	}
	c.epoch++
	if full <= 0 || distance == 0 {
		c.repeat = false
		c.ticker.Stop()
		c.raw = to
		c.rest(to)
		return
	}
	c.from, c.to = c.raw, to
	c.span = time.Duration(float64(full) * float64(distance))
	c.fresh = true
	// The ticker starts before OnStatus hears of the run, so that a Stop, a
	// Dispose or another run from that callback is the last word.
	c.ticker.Start()
	c.setStatus(status)
}

// rest reports the last step of a run that came to rest at at, with its ticker
// stopped. If OnChange starts another run, jumps or disposes, the status is
// that of the new state already and stays as it is.
func (c *AnimationController) rest(at float32) {
	epoch := c.epoch
	c.changed()
	if c.epoch != epoch {
		return
	}
	c.finish(at)
}

func (c *AnimationController) tick(now time.Duration) {
	if c.fresh {
		c.fresh = false
		c.started = now
	}
	t := float32(float64(now-c.started) / float64(c.span))
	if t < 1 {
		c.raw = c.from + (c.to-c.from)*t
		c.changed()
		return
	}
	c.raw = c.to
	if c.repeat {
		c.again(now)
		c.changed()
		return
	}
	c.ticker.Stop()
	c.rest(c.to)
}

// again starts the next round of Repeat at now.
func (c *AnimationController) again(now time.Duration) {
	c.started = now
	if c.bounce {
		c.from, c.to = c.to, c.from
		return
	}
	c.raw = c.from
}

func (c *AnimationController) finish(at float32) {
	if at >= 1 {
		c.setStatus(Completed)
	} else {
		c.setStatus(Dismissed)
	}
}

func (c *AnimationController) changed() {
	if c.OnChange != nil {
		c.OnChange()
	}
}

func (c *AnimationController) setStatus(s Status) {
	if c.status == s {
		return
	}
	c.status = s
	if c.OnStatus != nil {
		c.OnStatus(s)
	}
}
