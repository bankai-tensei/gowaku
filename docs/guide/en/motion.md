# Motion

[Guides](README.md) · [日本語](../ja/motion.md)

gowaku has animation built in, from widgets that move to new values on their own, to controllers and springs that you drive. This guide goes from the simplest to the most hands-on. The curves, springs and controllers are in package `motion`; the animated widgets are in `widgets/basic`, and page transitions in `widgets/nav`.

## Where time comes from

Every animation runs on a `core.Ticker`, which asks for a frame while it runs. When the motion arrives, the ticker stops and the window goes back to drawing nothing. Package `motion` never reads the clock itself: time comes from the tickers, which is what lets `gowakutest` move a fake clock in tests ([Testing and previews](testing-and-preview.md)).

Two rules hold for everything below:

- A new target starts from the value of the moment, so interrupting a motion never makes a jump.
- When the user asks the OS for less motion, `basic.ReduceMotionOf(ctx)` is true. The standard widgets then jump instead of moving or resizing, and keep their fades. Do the same in your own animations.

## Implicit animations

An implicit animation is a widget that, when one of its fields changes, moves to the new value over time instead of jumping. You change the field in `SetState` as usual; the widget does the rest.

| Widget | Animates |
|---|---|
| `AnimatedContainer` | Size, padding, color, corners, border and shadows. A width or height of zero does not animate. |
| `AnimatedOpacity` | Opacity. At 0 the child is out of hit testing, semantics and focus. |
| `AnimatedAlign` | The alignment of its child. |
| `AnimatedPositioned` | The place of a child of a `Stack`. |

Each has `Duration` (zero selects 200 ms) and `Curve` (nil selects `motion.Standard`).

```go
type panel struct{ gowaku.StatefulBase }

type panelState struct {
	gowaku.StateBase
	open bool
}

func (panel) CreateState() gowaku.State { return &panelState{} }

func (s *panelState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	height, color := float32(48), th.Color.Fill
	if s.open {
		height, color = 160, th.Color.AccentTint
	}
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space8, Children: []gowaku.Widget{
		control.Button{Label: "Toggle", OnPress: func() { s.SetState(func() { s.open = !s.open }) }},
		basic.AnimatedContainer{
			Duration: 250 * time.Millisecond,
			Curve:    motion.EaseOutCubic,
			Height:   height,
			Color:    color,
			Radius:   th.Radius.Group,
		},
		basic.AnimatedOpacity{Opacity: boolTo1(s.open), Child: basic.Text{Content: "Details"}},
	}}
}

func boolTo1(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
```

The standard controls animate in the same way: a switch slides its knob, a button sinks when pressed, a dialog fades and scales in.

## Switching children

`AnimatedSwitcher` moves from its old child to a new one. A child counts as new when it has another type or another key, so give children of the same type a key with `gowaku.Keyed`:

```go
type tabsState struct {
	gowaku.StateBase
	page, dir int // the page shown, and +1 or -1 for the way it went
}

func (s *tabsState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.AnimatedSwitcher{
		Duration:   250 * time.Millisecond,
		Transition: basic.SlideTransition(0.04*float32(s.dir), 0),
		Child:      gowaku.Keyed(s.page, s.body(s.page)),
	}
}
```

`Transition` decides the motion:

- `basic.FadeTransition`, the default, crossfades.
- `basic.SlideTransition(dx, dy)` slides the new child in from `dx, dy`, in fractions of its size, while it fades in, and the old one out the other way. 0.03 to 0.06 is a quiet shift; a negative `dx` goes back.
- `basic.ScaleTransition(from)` grows the new child from `from` to its size, as a window opens. 0.96 is gentle.

A transition of your own is a `basic.SwitcherTransition`: a function of the child, the progress `t` and whether the child is entering, which returns the child wrapped for that frame. It must return the same widget types for every `t`, or the child loses its state mid-motion. Build it from the paint-only widgets below.

## Curves

A `motion.Curve` maps the progress of a motion, from 0 to 1, to the progress of its value.

| Curve | Shape |
|---|---|
| `Standard` | Motion that begins and ends on screen. The default of the implicit widgets. |
| `Enter`, `Leave` | What arrives: fast, then slow to settle. What goes away: slow to start. |
| `Linear` | Constant speed. |
| `EaseInCubic`, `EaseOutCubic`, `EaseInOutCubic` | The usual easings. |
| `EaseOutBack`, `SpringOut` | Overshoot the end a little and come back. |
| `ElasticOut`, `BounceOut` | Ring like a plucked string; bounce on the end. |

`motion.CubicBezier(x1, y1, x2, y2)` makes a curve from control points, as in CSS:

```go
var gentle = motion.CubicBezier(0.2, 0, 0, 1)
```

## Animation controllers

When a motion is not "move to the new value", such as something that pulses, spins or plays on command, drive it with a `motion.AnimationController`. It runs a value between 0 and 1 over time. Set its fields, `Attach` it in `InitState`, and `Dispose` it in `Dispose`:

```go
type pulse struct{ gowaku.StatefulBase }

type pulseState struct {
	gowaku.StateBase
	c motion.AnimationController
}

func (pulse) CreateState() gowaku.State { return &pulseState{} }

func (s *pulseState) InitState() {
	s.c.Duration = 800 * time.Millisecond
	s.c.Curve = motion.EaseInOutCubic
	s.c.OnChange = func() { s.SetState(func() {}) }
	s.c.Attach(&s.StateBase)
	s.c.Repeat(true)
}

func (s *pulseState) Dispose() { s.c.Dispose() }

func (s *pulseState) Build(gowaku.BuildContext) gowaku.Widget {
	scale := motion.LerpFloat(0.9, 1.1, s.c.Value())
	return basic.Scale{Scale: scale, Origin: layout.AlignCenter, Child: basic.Text{Content: "Live"}}
}
```

- `Forward()` runs to 1, `Reverse()` to 0, `Restart()` from 0 to 1, `Repeat(bounce)` until `Stop()`. `SetValue(v)` jumps without animating.
- `Value()` is the progress with the curve applied, `Progress()` without it, and `Status()` one of `motion.Dismissed`, `Forwarding`, `Reversing` and `Completed`.
- `OnChange` runs on every step; `OnStatus` when the status changes. `ReverseDuration` and `ReverseCurve` replace the forward ones on the way back.
- `Attach` takes a `motion.TickerSource`: the `StateBase` of a state, or the `RenderBase` of a render object.

Turn the value into what you draw with the lerp functions, `motion.LerpFloat`, `LerpColor`, `LerpOffset`, `LerpSize`, `LerpRect`, `LerpRRect`, `LerpInsets` and `LerpAffine`, or with a `motion.Tween[T]{Begin, End, Lerp}` and its `At(t)`.

## Springs

A spring moves like a mass on a spring rather than along a curve of fixed length. A new target keeps the velocity of the moment, so a quick press and release flows instead of jumping. `motion.SpringValue` is a number that follows its target this way:

```go
type knob struct{ gowaku.StatefulBase }

type knobState struct {
	gowaku.StateBase
	x  motion.SpringValue
	on bool
}

func (knob) CreateState() gowaku.State { return &knobState{} }

func (s *knobState) InitState() {
	s.x.Spring = motion.BouncySpring()
	s.x.OnChange = func() { s.SetState(func() {}) }
	s.x.Attach(&s.StateBase, 0)
}

func (s *knobState) Dispose() { s.x.Dispose() }

func (s *knobState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Column{Size: layout.MainMin, Children: []gowaku.Widget{
		control.Button{Label: "Move", OnPress: func() {
			s.on = !s.on
			s.x.Set(boolTo1(s.on)) // starts the motion; OnChange rebuilds
		}},
		basic.SizedBox{Width: 240, Height: 32, Child: basic.Stack{Children: []gowaku.Widget{
			basic.Positioned{Left: 200 * s.x.Value(), Child: basic.Container{
				Width: 32, Height: 32, Radius: 16, Color: th.Color.Accent,
			}},
		}}},
	}}
}
```

- `Set(target)` names where to go. It does not call `OnChange`, so it is safe during `Build`.
- `Jump(v)` goes there at once; `Kick(velocity)` strikes the value and lets it come back; `Moving()` reports whether it is on its way.
- `Spring` holds the physics as `motion.Spring{Mass, Stiffness, Damping}`. `motion.SnappySpring()`, the zero value's choice, settles fast without overshoot, for what the pointer presses. `motion.BouncySpring()` overshoots a little, for parts that travel. `motion.CriticalSpring(stiffness)` reaches the target fastest without overshoot.

`motion.SpringSimulation` is the exact motion underneath, from a position and velocity to a target, for code that keeps its own time.

## Your own implicit animation

`motion.Implicit` is what the implicit widgets are made of. A state attaches it, tracks one `motion.Value` per animated field, sets the targets in `DidUpdateWidget`, and calls `Commit`:

```go
type glow struct {
	gowaku.StatefulBase
	Level float32
	Child gowaku.Widget
}

type glowState struct {
	gowaku.StateOf[glow]
	m     motion.Implicit
	level *motion.Value[float32]
}

func (glow) CreateState() gowaku.State { return &glowState{} }

func (s *glowState) InitState() {
	s.m.Attach(&s.StateBase, func() { s.SetState(func() {}) })
	s.level = motion.Track(&s.m, s.Widget().Level, motion.LerpFloat)
}

func (s *glowState) DidUpdateWidget(gowaku.Widget) {
	s.level.Set(s.Widget().Level)
	s.m.Commit(300*time.Millisecond, motion.Standard)
}

func (s *glowState) Dispose() { s.m.Dispose() }

func (s *glowState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color: th.Color.Accent.WithAlpha(s.level.Get()),
		Child: s.Widget().Child,
	}
}
```

`motion.Track` compares values with `==`; `motion.TrackFunc` takes a comparison for types such as slices. `Commit` does nothing when no target changed.

## Paint-only transforms

These widgets move only the picture. Layout, and in most cases the pointer, see the child in place, so they suit motions that end at rest, and they cost no layout per frame:

- `basic.Opacity{Opacity, Child}`.
- `basic.Translate{X, Y, Child}`, in fractions of the child's size.
- `basic.Scale{Scale, Origin, Child}`.
- `basic.Transform` and `basic.Rotate(radians, child)`. A turned widget is hit only inside its turned shape (`examples/rotate`).

## Pages and heroes

`nav.Navigator` moves between pages. By default a page that comes in slides in from the trailing edge by a few pixels while it fades in. `nav.PageRoute.Transition` takes a `nav.PageTransition` of your own, and `Instant: true` turns the motion off.

`nav.Hero` makes a child fly from one page to the next: give the small icon in a list row and the big icon on the detail page the same `Tag`, and the icon lifts off one page and lands on the other while the page moves.

```go
func noteRow(ctx gowaku.BuildContext, id int, color geom.Color) gowaku.Widget {
	return basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
		nav.Hero{Tag: id, Child: basic.Container{Width: 24, Height: 24, Radius: 6, Color: color}},
		control.Button{Label: "Open", OnPress: func() {
			nav.NavigatorOf(ctx).Push(nav.PageRoute{Page: notePage{id: id, color: color}})
		}},
	}}
}

type notePage struct {
	gowaku.StatelessBase
	id    int
	color geom.Color
}

func (p notePage) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Column{Size: layout.MainMin, Children: []gowaku.Widget{
		nav.BackButton{},
		nav.Hero{Tag: p.id, Child: basic.Container{Width: 96, Height: 96, Radius: 16, Color: p.color}},
	}}
}
```

Tags must be comparable and unique on a page. A window that asks for less motion has no flights. `examples/navigator` shows the whole thing.

## Further reading

- The [animation page](../../catalog/en/animation.md) of the widget catalog.
- [ADR 0010](../../ADR/0010-motion-and-theme.md) (Japanese) on the design of `motion`, and [ADR 0037](../../ADR/0037-switcher-transitions.md) on switcher transitions.
