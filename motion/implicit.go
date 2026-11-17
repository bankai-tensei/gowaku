package motion

import "time"

// Implicit is what makes "when the value changes, animate from the old one"
// work: one controller and the values that it moves together.
//
// A widget state calls Attach in InitState and makes a Value per animated field.
// In DidUpdateWidget it sets every Value to the field of the new widget, and then
// calls Commit. Build reads the Values. See docs/ADR/0010-motion-and-theme.md.
type Implicit struct {
	controller AnimationController
	tracks     []track
	moved      bool
}

// track is what Implicit sees of any Value[T].
type track interface {
	rebase()
}

// Value is one animated value of an Implicit.
type Value[T any] struct {
	owner *Implicit
	// from and to are the ends of the motion, and next the end of the one that
	// Commit starts.
	from, to, next T
	lerp           Lerp[T]
	same           func(a, b T) bool
}

// Attach connects the animation. onChange runs on every step of it, and should
// call SetState.
func (m *Implicit) Attach(src TickerSource, onChange func()) {
	m.controller.Attach(src)
	m.controller.OnChange = onChange
	m.controller.SetValue(1)
}

// Dispose stops the animation for good.
func (m *Implicit) Dispose() { m.controller.Dispose() }

// Animating reports whether any value is moving.
func (m *Implicit) Animating() bool { return m.controller.Animating() }

// Commit starts the motion toward the targets set since the last call, if any of
// them changed. Every value starts from where it is, also those whose target
// stayed, so that nothing jumps. A zero duration goes there at once.
func (m *Implicit) Commit(d time.Duration, curve Curve) {
	if !m.moved {
		return
	}
	m.moved = false
	for _, t := range m.tracks {
		t.rebase()
	}
	if d <= 0 {
		// Commit runs during a build, where the callback must not run.
		m.controller.SetValue(1)
		return
	}
	m.controller.Duration, m.controller.Curve = d, curve
	m.controller.Restart()
}

// Track adds a value that starts at initial and compares with ==.
func Track[T comparable](m *Implicit, initial T, lerp Lerp[T]) *Value[T] {
	return TrackFunc(m, initial, lerp, func(a, b T) bool { return a == b })
}

// TrackFunc adds a value of a type that == cannot compare.
func TrackFunc[T any](m *Implicit, initial T, lerp Lerp[T], same func(a, b T) bool) *Value[T] {
	v := &Value[T]{owner: m, from: initial, to: initial, next: initial, lerp: lerp, same: same}
	m.tracks = append(m.tracks, v)
	return v
}

// Get returns the value of the moment.
func (v *Value[T]) Get() T {
	t := v.owner.controller.Value()
	if t == 1 {
		return v.to
	}
	return v.lerp(v.from, v.to, t)
}

// Set names the target, which Commit starts moving to.
func (v *Value[T]) Set(target T) {
	if !v.same(v.next, target) {
		v.next = target
		v.owner.moved = true
	}
}

func (v *Value[T]) rebase() {
	v.from = v.Get()
	v.to = v.next
}
