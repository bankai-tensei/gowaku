package motion

import (
	"math"
	"testing"
	"time"
)

func springs() map[string]Spring {
	return map[string]Spring{
		"臨界減衰": CriticalSpring(400),
		"不足減衰": {Mass: 1, Stiffness: 400, Damping: 10},
		"過減衰":  {Mass: 1, Stiffness: 400, Damping: 90},
	}
}

func TestSpring_StartsWhereItWasTold(t *testing.T) {
	for name, s := range springs() {
		t.Run(name, func(t *testing.T) {
			sim := NewSpringSimulation(s, 10, 300, 100)
			p, v := sim.At(0)
			if math.Abs(p-10) > 1e-9 || math.Abs(v-300) > 1e-9 {
				t.Errorf("got %v, %v", p, v)
			}
		})
	}
}

func TestSpring_ReachesTheTarget(t *testing.T) {
	for name, s := range springs() {
		t.Run(name, func(t *testing.T) {
			sim := NewSpringSimulation(s, 0, 0, 100)
			if sim.Settled(0) {
				t.Error("must not start settled")
			}
			if !sim.Settled(10 * time.Second) {
				p, v := sim.At(10 * time.Second)
				t.Errorf("got %v, %v", p, v)
			}
		})
	}
}

// The velocity must be the derivative of the position, or a change of target
// would jerk.
func TestSpring_VelocityMatchesPosition(t *testing.T) {
	const h = time.Microsecond
	for name, s := range springs() {
		t.Run(name, func(t *testing.T) {
			sim := NewSpringSimulation(s, 5, -200, 100)
			for _, at := range []time.Duration{10 * time.Millisecond, 100 * time.Millisecond, 300 * time.Millisecond} {
				before, _ := sim.At(at - h)
				after, _ := sim.At(at + h)
				_, v := sim.At(at)
				want := (after - before) / (2 * h).Seconds()
				if math.Abs(v-want) > 0.05+math.Abs(want)*1e-3 {
					t.Errorf("at %v: got %v, want %v", at, v, want)
				}
			}
		})
	}
}

func TestSpring_CriticalDoesNotOvershoot(t *testing.T) {
	sim := NewSpringSimulation(CriticalSpring(400), 0, 0, 100)
	for at := time.Duration(0); at < 2*time.Second; at += time.Millisecond {
		if p, _ := sim.At(at); p > 100 {
			t.Fatalf("at %v: %v", at, p)
		}
	}
}

func TestSpring_UnderdampedOvershoots(t *testing.T) {
	sim := NewSpringSimulation(Spring{Mass: 1, Stiffness: 400, Damping: 10}, 0, 0, 100)
	peak := 0.0
	for at := time.Duration(0); at < 2*time.Second; at += time.Millisecond {
		p, _ := sim.At(at)
		peak = math.Max(peak, p)
	}
	if peak <= 100 {
		t.Errorf("peak %v", peak)
	}
}
