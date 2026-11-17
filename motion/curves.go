package motion

import "math"

const (
	// bezierSteps bounds the Newton iterations of a cubic Bézier. The curves of
	// UI easing converge in far fewer.
	bezierSteps = 8
	// bezierHalvings is the fallback for a flat stretch, down to float32 noise.
	bezierHalvings = 24
	bezierEpsilon  = 1e-5
)

// Curve maps the progress of an animation, from 0 to 1, to the progress of its
// value. It should map 0 to 0 and 1 to 1, and can overshoot in between. A nil
// Curve is Linear.
type Curve func(t float32) float32

// Linear is the identity.
func Linear(t float32) float32 { return t }

// EaseInCubic starts slowly.
func EaseInCubic(t float32) float32 { return t * t * t }

// EaseOutCubic ends slowly.
func EaseOutCubic(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOutCubic starts and ends slowly.
func EaseInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := 2 - 2*t
	return 1 - u*u*u/2
}

// EaseOutBack overshoots the end a little and comes back.
func EaseOutBack(t float32) float32 {
	const overshoot = 1.70158
	u := t - 1
	return 1 + (overshoot+1)*u*u*u + overshoot*u*u
}

// ElasticOut overshoots the end and rings like a plucked string.
func ElasticOut(t float32) float32 {
	if t <= 0 || t >= 1 {
		return min(max(t, 0), 1)
	}
	x := float64(t)
	return float32(math.Pow(2, -10*x)*math.Sin((x*10-0.75)*2*math.Pi/3) + 1)
}

// BounceOut falls to the end and bounces on it.
func BounceOut(t float32) float32 {
	const n, d = 7.5625, 2.75
	switch {
	case t < 1/d:
		return n * t * t
	case t < 2/d:
		t -= 1.5 / d
		return n*t*t + 0.75
	case t < 2.5/d:
		t -= 2.25 / d
		return n*t*t + 0.9375
	}
	t -= 2.625 / d
	return n*t*t + 0.984375
}

// Standard is the easing of motion that begins and ends on the screen.
func Standard(t float32) float32 { return bezier(0.2, 0, 0, 1, t) }

// Enter is the easing of what arrives, fast at first and slow to settle.
func Enter(t float32) float32 { return bezier(0, 0, 0, 1, t) }

// Leave is the easing of what goes away, slow to start.
func Leave(t float32) float32 { return bezier(0.3, 0, 1, 1, t) }

// SpringOut has the shape of a spring that overshoots a little. It is for
// controllers, which have a fixed duration; see SpringValue for the real physics.
func SpringOut(t float32) float32 {
	if t <= 0 || t >= 1 {
		return min(max(t, 0), 1)
	}
	x := float64(t)
	return float32(1 - math.Exp(-7*x)*math.Cos(9*x))
}

// CubicBezier returns the curve with the control points (x1, y1) and (x2, y2),
// as in CSS. x1 and x2 must lie between 0 and 1.
func CubicBezier(x1, y1, x2, y2 float32) Curve {
	return func(t float32) float32 { return bezier(x1, y1, x2, y2, t) }
}

// bezier solves x(s) = t for s and returns y(s).
func bezier(x1, y1, x2, y2, t float32) float32 {
	if t <= 0 || t >= 1 {
		return min(max(t, 0), 1)
	}
	s := t
	for range bezierSteps {
		e := bezierAt(x1, x2, s) - t
		if e > -bezierEpsilon && e < bezierEpsilon {
			return bezierAt(y1, y2, s)
		}
		slope := bezierSlope(x1, x2, s)
		if slope > -bezierEpsilon && slope < bezierEpsilon {
			break
		}
		s -= e / slope
	}
	// Newton lost its way. x(s) only rises, so halving cannot.
	lo, hi := float32(0), float32(1)
	s = t
	for range bezierHalvings {
		if bezierAt(x1, x2, s) < t {
			lo = s
		} else {
			hi = s
		}
		s = (lo + hi) / 2
	}
	return bezierAt(y1, y2, s)
}

func bezierAt(a, b, s float32) float32 {
	u := 1 - s
	return 3*u*u*s*a + 3*u*s*s*b + s*s*s
}

func bezierSlope(a, b, s float32) float32 {
	u := 1 - s
	return 3*u*u*a + 6*u*s*(b-a) + 3*s*s*(1-b)
}
