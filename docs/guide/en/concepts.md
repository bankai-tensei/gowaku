# Concepts

[Guides](README.md) · [日本語](../ja/concepts.md)

gowaku is declarative, in a Flutter-like way: you describe the UI with widgets, and the library keeps the tree on the screen in step with that description. This guide explains the pieces and how a change reaches the screen.

## Three trees

| Tree | Lives | Does |
|---|---|---|
| **Widget** | One `Build`, then thrown away | Describes part of the UI. A plain value. |
| **Element** | As long as its place in the tree | Keeps the `State`, links a widget to its render object, and is what the diff compares. Internal to gowaku. |
| **RenderObject** | As long as its element | Does the layout, painting, hit testing and accessibility of a widget. |

You write widgets. Each `Build` returns new widget values. gowaku compares them with the elements that are already there: when an old and a new widget have the same type and the same key, the element stays, keeps its state and takes the new values. Otherwise the old element leaves and a new one is made.

Package `core` holds these types. The root package `gowaku` gives the ones every UI file writes another name, so a UI file rarely needs to import `core`: `gowaku.Widget`, `gowaku.BuildContext`, `gowaku.StatelessBase`, `gowaku.StatefulBase`, `gowaku.State`, `gowaku.StateBase`, `gowaku.StateOf`, `gowaku.Key` and `gowaku.Keyed`.

## Widgets are values

A widget is a struct, written as a literal. Its fields are its whole API:

```go
control.Button{Label: "Save", Style: control.ButtonPrimary, OnPress: save}
```

- **The zero value works.** `basic.Padding{}` draws its child without inset, `basic.Text{}` shows nothing, a `control.Button` without `OnPress` is disabled. Defaults are written in the doc comment of each field.
- **No builders, no options.** There are no `NewButton(...)` functions, functional options or method chains. A few one-line helpers exist for the most common shapes: `basic.VStack`, `basic.HStack`, `basic.Layers` and `basic.Pad`.
- **No reflection over your fields.** gowaku compares the type of a widget and its key, nothing else.

Widgets are cheap. Build them freely on every `Build`.

## Stateless widgets

A stateless widget builds its child from its own fields. Embed `gowaku.StatelessBase` and write `Build`:

```go
type greeting struct {
	gowaku.StatelessBase
	Name string
}

func (g greeting) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Text{Content: "Hello, " + g.Name, Style: th.Text.Title}
}
```

Use it as `greeting{Name: "Hanako"}`.

## Stateful widgets

A stateful widget keeps a `State` that survives rebuilds. The widget embeds `gowaku.StatefulBase` and returns a new state from `CreateState`. The state embeds `gowaku.StateBase`, or `gowaku.StateOf[W]` to read the fields of its widget without a type assertion:

```go
type stepper struct {
	gowaku.StatefulBase
	Step int
}

type stepperState struct {
	gowaku.StateOf[stepper]
	n int
}

func (stepper) CreateState() gowaku.State { return &stepperState{} }

func (s *stepperState) Build(gowaku.BuildContext) gowaku.Widget {
	step := s.Widget().Step // a stepper, no type assertion
	return control.Button{
		Label:   fmt.Sprintf("%d (+%d)", s.n, step),
		OnPress: func() { s.SetState(func() { s.n += step }) },
	}
}
```

The lifecycle of a state:

| Method | Runs |
|---|---|
| `InitState()` | Once, before the first `Build`. Start tickers and controllers here. |
| `Build(ctx)` | Whenever the state is dirty or its parent rebuilt. |
| `DidUpdateWidget(old)` | When a parent rebuild gives the element a new widget value. `Widget()` returns the new one. |
| `Dispose()` | Once, when the state leaves the tree. Stop what `InitState` started. |

`StateBase` gives empty versions of all but `Build`, so write only the ones you need.

`SetState(fn)` runs `fn` and schedules a rebuild of this state. Two rules, both checked with a panic:

- Do not call `SetState` during `Build`. Call it from a callback, such as `OnPress`, or from a ticker.
- Do not call `SetState` on a state that has left the tree. Check `Mounted()` when a result may arrive late.

## Tickers

A ticker calls a function once per frame while it runs. A state makes one with `NewTicker`, or with `NewPeriodicTicker` for something that changes at a fixed rate:

```go
type clock struct{ gowaku.StatefulBase }

type clockState struct {
	gowaku.StateBase
	ticker *core.Ticker
	now    time.Time
}

func (clock) CreateState() gowaku.State { return &clockState{} }

func (s *clockState) InitState() {
	s.now = time.Now()
	s.ticker = s.NewPeriodicTicker(time.Second, func(time.Duration) {
		s.SetState(func() { s.now = time.Now() })
	})
	s.ticker.Start()
}

func (s *clockState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: s.now.Format("15:04:05")}
}
```

A periodic ticker lets the window sleep between calls. Its first call comes right away, on the frame after `Start`, not one period later. Tickers made by a state stop on their own after its `Dispose`. The animation types of `motion` run on tickers too; see [Motion](motion.md).

### Running code after a delay

For a single call after a delay, as to take back a "Saved" message, a state calls `After`. It returns a function that cancels the call:

```go
type saveState struct {
	gowaku.StateBase
	saved bool
}

func (s *saveState) save() {
	s.SetState(func() { s.saved = true })
	s.After(1200*time.Millisecond, func() {
		s.SetState(func() { s.saved = false })
	})
}
```

`After` runs the function on the UI goroutine, so it may call `SetState`. It follows the clock of the tickers, so `gowakutest`'s `Advance` drives it like an animation (a tap's settle does not wait for it), and the window sleeps while it waits. When the state leaves the tree, the call is canceled with it. Use `After` rather than `time.AfterFunc` with `gowaku.Post`: that pair runs on the wall clock, which a test cannot move, and `Post` drops the function outside `Run`.

## BuildContext

A `BuildContext` is the place of a widget in the tree. `Build` gets one, and a state also has `Context()`. Functions that read what lies above a place take it:

- `basic.ThemeOf(ctx)`: the theme.
- `basic.MediaQueryOf(ctx)`: the scale, the brightness the OS asks for, reduced motion, the locale and the size of the window. `basic.WindowSizeOf`, `basic.ReduceMotionOf` and `basic.LocaleOf` read one of them, and rebuild the caller only when that one changes.
- `nav.NavigatorOf(ctx)`: the nearest `nav.Navigator`.
- `gowaku.WindowOf(ctx)`: the window.

A context sees only what is above it. To read something that a widget in the same `Build` provides, put the reading part in a `basic.Builder`, which builds with a context of its own at its place:

```go
basic.Builder{Content: func(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: "Hi", Style: basic.ThemeOf(ctx).Text.Title}
}}
```

## Inherited data

`core.Inherited[T]` makes a value available to everything below it. `core.DependOn[T]` finds the nearest one and rebuilds the caller when the value changes; `core.Lookup[T]` finds it without that, for callbacks. This is how `ThemeOf` and `MediaQueryOf` work.

```go
type user struct{ Name string }

func withUser(u user, child gowaku.Widget) gowaku.Widget {
	return core.Inherited[user]{Value: u, Child: child}
}

type badge struct{ gowaku.StatelessBase }

func (badge) Build(ctx gowaku.BuildContext) gowaku.Widget {
	u, ok := core.DependOn[user](ctx)
	if !ok {
		return basic.Text{Content: "Signed out"}
	}
	return basic.Text{Content: u.Name}
}
```

`T` must be comparable, and "changed" means `!=`. Use a type of your own, not a bare `string` or `int`, so that two packages do not find each other's values. For a large value, pass a pointer and replace the pointer when the data changes.

## Keys

The identity of a widget among its siblings is its type and its key. Without keys, children are matched by position. That is right for most trees, but not for a list whose items move: a removed row at the top would hand its state, such as hover, focus or text being typed, to the row below.

Give each item a key with `gowaku.Keyed`:

```go
type todo struct {
	ID    int
	Title string
	Done  bool
}

func todoRows(items []todo, toggle func(id int, done bool)) []gowaku.Widget {
	rows := make([]gowaku.Widget, len(items))
	for i, it := range items {
		rows[i] = gowaku.Keyed(it.ID, control.Checkbox{
			Label:    it.Title,
			Value:    it.Done,
			OnChange: func(v bool) { toggle(it.ID, v) },
		})
	}
	return rows
}
```

A key can be any comparable value, such as an ID or a string; `Keyed` panics on a value that is not comparable. Keys must be unique among siblings. `basic.AnimatedSwitcher` also uses keys: a child with a new key counts as new and is animated in.

## How a change reaches the screen

1. A callback calls `SetState`. The state is marked dirty. Nothing is built yet.
2. On the next frame, the dirty states build, parents first. A parent's new widgets update its children: render widgets copy their fields to their render objects, and stateful children get `DidUpdateWidget` and build again.
3. A render object whose layout changed is laid out again, starting from the nearest object whose size cannot affect its parent.
4. What changed is painted. A `basic.RepaintBoundary` keeps its child's picture apart, so that a change inside paints only the child and a change outside reuses its pixels. `collection.List` puts one around each row.
5. The GPU draws the frame.

Many `SetState` calls before a frame make one build. Keep state low in the tree: a `SetState` rebuilds that state and what its `Build` returns, so a small stateful widget near the change costs less than one at the root.

### Idle draws nothing

A window draws a frame only when something asks for it: input, a `SetState`, a change of the window, or a running ticker. The ticker is the only way to ask for frames over time. When no ticker runs and no event comes, the UI goroutine sleeps in the event loop and uses no CPU. Animations stop their tickers when they arrive, so a window goes quiet by itself.

## The UI goroutine

The goroutine that calls `gowaku.Run` is the UI goroutine. `Build`, layout, paint, callbacks and `SetState` all run there, so state needs no locks. From another goroutine, hand work back with `gowaku.Post`:

```go
func (s *loaderState) InitState() {
	go func() {
		r := slowFetch() // runs on another goroutine
		gowaku.Post(func() {
			if s.Mounted() {
				s.SetState(func() { s.result = r })
			}
		})
	}()
}
```

`Post` is safe from any goroutine. Outside `Run` it drops the function, so code that only waits for time to pass uses `After` (see [Tickers](#running-code-after-a-delay)), which tests can drive.

## Render objects

Most applications never write one. Compose the standard widgets, draw with `control.CustomPaint`, and arrange children with `basic.CustomLayout` ([Layout](layout.md#a-custom-layout)).

When you do need one, a widget implements `core.RenderWidget` (`CreateRenderObject` and `UpdateRenderObject`), and `core.SingleChildWidget` or `core.MultiChildWidget` for children. The render object embeds `core.RenderBase` and implements `Layout` and `Paint`. `UpdateRenderObject` should mark the object dirty only for fields that changed: `MarkNeedsLayout` for a change of size, `MarkNeedsPaint` for a change of look. `Paint` must not allocate. The render objects of package `layout` are examples to read.

## Further reading

- [Layout](layout.md) for the constraints that `Layout` receives.
- [ADR 0006](../../ADR/0006-core-tree-api.md) (Japanese) on the API of the tree, and [ADR 0029](../../ADR/0029-typed-state-widget.md) on `StateOf`.
