package layout

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

const (
	progressBarHeight       = 6
	progressBarDefaultWidth = 200
	progressRingDefault     = 32
	// progressRingMinThickness keeps a small ring from turning into a hairline.
	progressRingMinThickness = 2
	// progressRingThicknessShare is the thickness of a ring as a share of its
	// diameter when none is given.
	progressRingThicknessShare = 1.0 / 12
	// busyRingShare is the length of the arc of a busy ring, and
	// busyRingTurn the time of one turn of it.
	busyRingShare = 0.28
	busyRingTurn  = 900 * time.Millisecond
	// busyBarShare is the length of the band of a busy bar, and busyBarPass
	// the time it takes to cross the bar.
	busyBarShare = 0.3
	busyBarPass  = 1400 * time.Millisecond
)

// busyClock runs the motion of a busy indicator while it is painted and seen,
// as the light of a skeleton does: an indicator scrolled out of view, clipped
// away, under a fully transparent Opacity or out of the tree asks for no
// frames.
type busyClock struct {
	ticker *core.Ticker
	// now is the time of the last tick, which places the motion.
	now time.Duration
	// painted says the indicator was painted and seen since the last tick.
	painted bool
}

// seen notes that the indicator of r was painted and seen, if the canvas took
// an op since before, and runs the clock.
func (b *busyClock) seen(r *core.RenderBase, cv *paint.Canvas, before int) {
	if len(cv.DisplayList().Ops) == before {
		return
	}
	b.painted = true
	if b.ticker == nil {
		b.ticker = r.NewTicker(func(now time.Duration) {
			if !r.Attached() || !b.painted {
				b.stop()
				return
			}
			b.painted = false
			b.now = now
			r.MarkNeedsPaint()
		})
	}
	if !b.ticker.Running() {
		b.ticker.Start()
	}
}

// stop stops the clock, if it runs.
func (b *busyClock) stop() {
	if b.ticker != nil {
		b.ticker.Stop()
	}
}

// phase returns how far into a period of length period the clock is, from 0
// to 1.
func (b *busyClock) phase(period time.Duration) float32 {
	return float32(b.now%period) / float32(period)
}

// RenderProgressBar draws a rounded track filled from the left by Value.
type RenderProgressBar struct {
	core.RenderBase
	// Value is the filled share, from 0 to 1.
	Value float32
	// Active fills the done part and Track the rest.
	Active, Track geom.Color
	// Busy shows work of unknown length instead of Value: a band of Active
	// that crosses the track again and again, or, with Still, stands in its
	// middle.
	Busy, Still bool

	clock busyClock
}

// Layout implements core.RenderObject. The bar takes the width it is allowed.
func (r *RenderProgressBar) Layout(c Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = progressBarDefaultWidth
	}
	return c.Constrain(geom.Size{W: w, H: progressBarHeight})
}

// Paint implements core.RenderObject.
func (r *RenderProgressBar) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	radius := sz.H / 2
	track := geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}
	before := len(cv.DisplayList().Ops)
	cv.DrawRRect(geom.RRectAll(track, radius), paint.Paint{Color: r.Track})
	if r.Busy {
		r.paintBusy(cv, track, before)
		return
	}
	r.clock.stop()
	done := sz.W * geom.Clamp32(r.Value, 0, 1)
	if done <= 0 {
		return
	}
	// A sliver narrower than the bar is tall stays a pill, not a square.
	track.W = max(done, sz.H)
	if done < sz.H {
		track.X -= sz.H - done
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
		cv.DrawRRect(geom.RRectAll(track, radius), paint.Paint{Color: r.Active})
		cv.Restore()
		return
	}
	cv.DrawRRect(geom.RRectAll(track, radius), paint.Paint{Color: r.Active})
}

// paintBusy draws the band of a busy bar over its track, and runs the clock
// unless the bar stands still.
func (r *RenderProgressBar) paintBusy(cv *paint.Canvas, track geom.Rect, before int) {
	band := max(track.W*busyBarShare, track.H)
	x := track.X + (track.W-band)/2
	if !r.Still {
		x = track.X - band + (track.W+band)*r.clock.phase(busyBarPass)
	}
	cv.Save()
	cv.ClipRRect(geom.RRectAll(track, track.H/2))
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: x, Y: track.Y, W: band, H: track.H}, track.H/2), paint.Paint{Color: r.Active})
	cv.Restore()
	if r.Still {
		r.clock.stop()
		return
	}
	r.clock.seen(&r.RenderBase, cv, before)
}

// RenderProgressRing draws a ring that fills clockwise from twelve o'clock by
// Value, with round ends, over a full ring of Track. It takes the size it is
// allowed and draws the largest circle that fits in the middle.
type RenderProgressRing struct {
	core.RenderBase
	// Value is the filled share, from 0 to 1.
	Value float32
	// Thickness is the width of the ring. Zero selects a twelfth of the
	// diameter.
	Thickness     float32
	Active, Track geom.Color
	// Busy shows work of unknown length instead of Value: an arc of Active
	// that turns around the ring, or, with Still, stands at twelve o'clock.
	Busy, Still bool

	// arc and track are rebuilt only when what they depend on changes.
	arc, track paint.Path
	built      ringKey
	clock      busyClock
}

type ringKey struct {
	value, diameter, thickness float32
	valid                      bool
}

// Layout implements core.RenderObject.
func (r *RenderProgressRing) Layout(c Constraints) geom.Size {
	w, h := c.MaxW, c.MaxH
	if w >= geom.Inf {
		w = progressRingDefault
	}
	if h >= geom.Inf {
		h = progressRingDefault
	}
	return c.Constrain(geom.Size{W: w, H: h})
}

func (r *RenderProgressRing) thickness(diameter float32) float32 {
	t := r.Thickness
	if t <= 0 {
		t = max(diameter*progressRingThicknessShare, progressRingMinThickness)
	}
	return min(t, diameter/2)
}

// Paint implements core.RenderObject.
func (r *RenderProgressRing) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	d := min(sz.W, sz.H)
	if d <= 0 {
		return
	}
	value := geom.Clamp32(r.Value, 0, 1)
	if r.Busy {
		value = busyRingShare
	}
	key := ringKey{value: value, diameter: d, thickness: r.thickness(d), valid: true}
	if key != r.built {
		r.built = key
		r.track.Reset()
		RingPath(&r.track, d, key.thickness, 1)
		r.arc.Reset()
		RingPath(&r.arc, d, key.thickness, key.value)
	}
	before := len(cv.DisplayList().Ops)
	cv.Save()
	cv.Translate(off.X+(sz.W-d)/2, off.Y+(sz.H-d)/2)
	cv.DrawPath(&r.track, paint.Paint{Color: r.Track})
	if r.Busy {
		// The arc is centered on where it stands, so that a still one reads
		// as no share filled from twelve o'clock.
		turn := float32(-busyRingShare / 2)
		if !r.Still {
			turn += r.clock.phase(busyRingTurn)
		}
		cv.Translate(d/2, d/2)
		cv.Transform(geom.Rotation(turn * 2 * math.Pi))
		cv.Translate(-d/2, -d/2)
	}
	cv.DrawPath(&r.arc, paint.Paint{Color: r.Active})
	cv.Restore()
	if !r.Busy || r.Still {
		r.clock.stop()
		return
	}
	r.clock.seen(&r.RenderBase, cv, before)
}

// RingPath adds to p a ring of the given diameter and thickness, in a square
// with its corner at the origin, filled clockwise from twelve o'clock by
// value. Below 1 the ends are round; at 1 or more it is a whole ring, and at 0
// or less nothing. The path fills with the non-zero rule.
func RingPath(p *paint.Path, diameter, thickness, value float32) {
	if value <= 0 || diameter <= 0 || thickness <= 0 {
		return
	}
	c := diameter / 2
	center := geom.Offset{X: c, Y: c}
	outer := c
	inner := c - min(thickness, diameter/2)
	const start = -math.Pi / 2
	if value >= 1 {
		// The inner circle runs the other way, which leaves the hole.
		p.AddArc(center, outer, start, 2*math.Pi)
		p.Close()
		p.AddArc(center, inner, start+2*math.Pi, -2*math.Pi)
		p.Close()
		return
	}
	sweep := float32(2 * math.Pi * float64(value))
	end := start + sweep
	mid, round := (outer+inner)/2, (outer-inner)/2
	cosE, sinE := float32(math.Cos(float64(end))), float32(math.Sin(float64(end)))
	p.AddArc(center, outer, start, sweep)
	p.ArcTo(geom.Offset{X: c + mid*cosE, Y: c + mid*sinE}, round, end, math.Pi)
	p.ArcTo(center, inner, end, -sweep)
	p.ArcTo(geom.Offset{X: c, Y: c - mid}, round, start+math.Pi, math.Pi)
	p.Close()
}

// Painter draws the content of a RenderCustomPaint. A painter that draws
// text gets it laid out: the widget that makes the painter finds the
// text.Shaper with core.DependOn[*text.Shaper](ctx) and lays the text out, or
// keeps a text.Labels, and the painter draws the text.Layout with
// paint.Canvas.DrawText. paint.Canvas.DrawRect and DrawRRect fill boxes, and
// paint.Path.AddArc, ArcTo and AddOval make round shapes.
type Painter interface {
	// Paint draws into a box of size sz with its corner at the origin of cv.
	// It runs on every paint, so it should not allocate. A Painter can keep one
	// paint.Path and reset and refill it for each shape it draws: the canvas
	// records a copy of the path, in storage it reuses frame after frame.
	Paint(cv *paint.Canvas, sz geom.Size)
}

// PainterFunc lets a function be a Painter.
type PainterFunc func(cv *paint.Canvas, sz geom.Size)

// Paint implements Painter.
func (f PainterFunc) Paint(cv *paint.Canvas, sz geom.Size) { f(cv, sz) }

// RenderCustomPaint hands its drawing to a Painter. It is Width by Height,
// within its constraints; a zero length takes all that is allowed, or nothing
// if that is unbounded. With a RepaintSignal set by SetRepaint, it also paints
// again on each Notify of the signal. With a shader set by SetShader, it fills
// its box with the shader before the Painter draws.
type RenderCustomPaint struct {
	core.RenderBase
	Painter       Painter
	Width, Height float32
	repaint       *RepaintSignal
	shader        ShaderPaint
	clock         shaderClock
}

// Layout implements core.RenderObject.
func (r *RenderCustomPaint) Layout(c Constraints) geom.Size {
	pick := func(want, limit float32) float32 {
		switch {
		case want > 0:
			return want
		case limit < geom.Inf:
			return limit
		}
		return 0
	}
	return c.Constrain(geom.Size{W: pick(r.Width, c.MaxW), H: pick(r.Height, c.MaxH)})
}

// Paint implements core.RenderObject.
func (r *RenderCustomPaint) Paint(cv *paint.Canvas, off geom.Offset) {
	r.paintShader(cv, off)
	if r.Painter == nil {
		return
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	r.Painter.Paint(cv, r.Size())
	cv.Restore()
}
