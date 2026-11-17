package motion

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
)

const (
	// springUnits scales a value of about 1 up to the pixels that Settled is
	// measured in. Without it, a value from 0 to 1 would count as at rest at 0.75.
	springUnits = 1000

	snappyStiffness = 700
	bouncyStiffness = 500
	// bouncyDamping is a fraction of critical damping. A little below 1 gives a
	// single small overshoot.
	bouncyDamping = 0.72
)

// SnappySpring settles fast without overshooting, for what the pointer presses.
func SnappySpring() Spring { return CriticalSpring(snappyStiffness) }

// BouncySpring overshoots a little, for parts that travel.
func BouncySpring() Spring {
	s := CriticalSpring(bouncyStiffness)
	s.Damping *= bouncyDamping
	return s
}

// SpringValue is a number that follows its target like a mass on a spring. A new
// target keeps the velocity of the moment, so that a quick press and release
// flows rather than jumps. Set the fields, call Attach in InitState and Dispose in
// Dispose.
type SpringValue struct {
	// Spring is the physics. The zero value is SnappySpring.
	Spring Spring
	// OnChange runs on every step of the motion.
	OnChange func()

	ticker *core.Ticker
	sim    SpringSimulation
	// started is when sim began, from the first tick after it was made, because
	// what sets a target has no clock.
	started time.Duration
	fresh   bool
	// position and velocity are in springUnits.
	position, velocity float64
	target             float64
}

// Attach connects the value to its source of frames. It starts at v, at rest.
func (s *SpringValue) Attach(src TickerSource, v float32) {
	s.ticker = src.NewTicker(s.tick)
	s.position = float64(v) * springUnits
	s.target = s.position
}

// Dispose stops the motion for good.
func (s *SpringValue) Dispose() { s.ticker.Stop() }

// Value returns the value of the moment, which can overshoot the target.
func (s *SpringValue) Value() float32 { return float32(s.position / springUnits) }

// Moving reports whether the value is still on its way.
func (s *SpringValue) Moving() bool { return s.ticker.Running() }

// Set makes target the place to go. It does not call OnChange, so it is safe
// during a Build.
func (s *SpringValue) Set(target float32) {
	t := float64(target) * springUnits
	if t == s.target {
		return
	}
	s.target = t
	s.launch()
}

// Kick pushes the value, as if it had been struck, and lets it return to the
// target. velocity is in units per second.
func (s *SpringValue) Kick(velocity float32) {
	s.velocity += float64(velocity) * springUnits
	s.launch()
}

// Jump goes to v at once and stops there.
func (s *SpringValue) Jump(v float32) {
	s.ticker.Stop()
	s.position = float64(v) * springUnits
	s.target = s.position
	s.velocity = 0
}

func (s *SpringValue) launch() {
	spring := s.Spring
	if spring == (Spring{}) {
		spring = SnappySpring()
	}
	s.sim = NewSpringSimulation(spring, s.position, s.velocity, s.target)
	s.fresh = true
	s.ticker.Start()
}

func (s *SpringValue) tick(now time.Duration) {
	if s.fresh {
		s.fresh = false
		s.started = now
	}
	elapsed := now - s.started
	s.position, s.velocity = s.sim.At(elapsed)
	if s.sim.Settled(elapsed) {
		s.position, s.velocity = s.target, 0
		s.ticker.Stop()
	}
	if s.OnChange != nil {
		s.OnChange()
	}
}
