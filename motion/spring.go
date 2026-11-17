package motion

import (
	"math"
	"time"
)

const (
	// settledDistance and settledSpeed say when a spring counts as at rest, in
	// logical pixels and pixels per second.
	settledDistance = 0.25
	settledSpeed    = 2
)

// Spring describes the physics of a damped spring.
type Spring struct {
	Mass, Stiffness, Damping float64
}

// CriticalSpring returns the spring that reaches its target fastest without
// overshooting, for a given stiffness and a mass of 1.
func CriticalSpring(stiffness float64) Spring {
	return Spring{Mass: 1, Stiffness: stiffness, Damping: 2 * math.Sqrt(stiffness)}
}

// SpringSimulation is the exact motion of a spring from a position and velocity
// to a target. Keeping the velocity across a change of target is what makes
// the motion continuous.
type SpringSimulation struct {
	target float64
	// The solution has one of three forms, depending on the damping. In all
	// of them the distance from the target is
	// e^(-a t) times something built from b, c and w.
	kind springKind
	a, w float64
	b, c float64
}

type springKind uint8

const (
	springCritical springKind = iota
	springUnder
	springOver
)

// NewSpringSimulation starts s at position with velocity, heading for target.
func NewSpringSimulation(s Spring, position, velocity, target float64) SpringSimulation {
	x0 := position - target
	a := s.Damping / (2 * s.Mass)
	w0 := s.Stiffness / s.Mass
	disc := a*a - w0
	sim := SpringSimulation{target: target, a: a}
	switch {
	case math.Abs(disc) < 1e-9:
		sim.kind = springCritical
		sim.b, sim.c = x0, velocity+a*x0
	case disc < 0:
		sim.kind = springUnder
		sim.w = math.Sqrt(-disc)
		sim.b, sim.c = x0, (velocity+a*x0)/sim.w
	default:
		sim.kind = springOver
		sim.w = math.Sqrt(disc)
		r1, r2 := -a+sim.w, -a-sim.w
		sim.c = (velocity - r2*x0) / (r1 - r2) // the slow root
		sim.b = x0 - sim.c                     // the fast root
	}
	return sim
}

// At returns the position and velocity t after the start.
func (s *SpringSimulation) At(t time.Duration) (position, velocity float64) {
	x := t.Seconds()
	decay := math.Exp(-s.a * x)
	switch s.kind {
	case springUnder:
		sin, cos := math.Sincos(s.w * x)
		position = decay * (s.b*cos + s.c*sin)
		velocity = decay * ((s.c*s.w-s.a*s.b)*cos - (s.b*s.w+s.a*s.c)*sin)
	case springOver:
		r1, r2 := -s.a+s.w, -s.a-s.w
		e1, e2 := math.Exp(r1*x), math.Exp(r2*x)
		position = s.c*e1 + s.b*e2
		velocity = s.c*r1*e1 + s.b*r2*e2
	default:
		position = decay * (s.b + s.c*x)
		velocity = decay * (s.c - s.a*(s.b+s.c*x))
	}
	return position + s.target, velocity
}

// Settled reports whether the motion has come to rest t after the start.
func (s *SpringSimulation) Settled(t time.Duration) bool {
	p, v := s.At(t)
	return math.Abs(p-s.target) < settledDistance && math.Abs(v) < settledSpeed
}

// Target returns where the spring is heading.
func (s *SpringSimulation) Target() float64 { return s.target }
