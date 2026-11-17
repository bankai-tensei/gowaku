package control

import (
	"strconv"

	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	switchHeight = 22
	// switchStretch is how much wider the thumb gets under a press.
	switchStretch = 5

	sliderStepDivisor = 20
	sliderGrow        = 0.1
	// sliderPress takes back what hover grew, so that a held thumb feels pressed.
	sliderPress = 0.14
	// plateSink shrinks the plate of a segmented control under a press.
	plateSink = 0.05
)

// Switch shows Value and asks for the opposite through OnChange. Without OnChange
// it is disabled.
type Switch struct {
	core.StatefulBase
	Value bool
	// Label describes the switch to assistive technology.
	Label    string
	OnChange func(bool)
}

type switchState struct {
	core.StateBase
	behavior.Control
	turn motion.SpringValue
}

// CreateState implements core.StatefulWidget.
func (Switch) CreateState() core.State { return &switchState{} }

// InitState implements core.State.
func (s *switchState) InitState() {
	s.Attach(&s.StateBase)
	s.turn.Spring = motion.BouncySpring()
	s.turn.OnChange = func() { kit.Rebuild(&s.StateBase) }
	s.turn.Attach(&s.StateBase, kit.OnOff(s.Widget().(Switch).Value))
}

// Dispose implements core.State.
func (s *switchState) Dispose() {
	s.Control.Dispose()
	s.turn.Dispose()
}

// Build implements core.State.
func (s *switchState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Switch)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil

	if basic.ReduceMotionOf(s.Context()) {
		s.turn.Jump(kit.OnOff(w.Value))
	} else {
		s.turn.Set(kit.OnOff(w.Value))
	}
	on := s.turn.Value()
	lit := geom.Clamp32(on, 0, 1)
	track := motion.LerpColor(theme.Over(c.FillStrong, c.Ink, kit.HoverStrength*s.Hot.Get()), theme.Over(c.Accent, c.OnAccent, kit.HoverStrength*s.Hot.Get()), lit)
	thumb, lift := geom.Hex(0xffffff), theme.LiftKnob
	if !enabled {
		track = motion.LerpColor(c.Fill, c.Ink3, lit)
		thumb, lift = motion.LerpColor(c.Ink3, c.Field, lit), theme.LiftNone
	}
	width := kit.SwitchThumb + switchStretch*geom.Clamp32(s.Sink.Value(), 0, 1)
	left := motion.LerpFloat(kit.SwitchEdge, kit.SwitchWidth-kit.SwitchEdge-width, on)
	body := basic.SizedBox{Width: kit.SwitchWidth, Height: switchHeight, Child: basic.Stack{Children: []core.Widget{
		basic.Container{Width: kit.SwitchWidth, Height: switchHeight, Radius: switchHeight / 2, Color: track},
		basic.Positioned{Left: left, Top: kit.SwitchEdge, Width: width, Height: kit.SwitchThumb, Child: basic.Container{
			Radius: kit.SwitchThumb / 2, Color: thumb, Shadows: th.Color.Shadows(lift),
		}},
	}}}
	value := "off"
	if w.Value {
		value = "on"
	}
	var state semantics.State
	if w.Value {
		state = semantics.Checked
	}
	press := func() { w.OnChange(!w.Value) }
	return s.Wrap(interact.Semantics{Role: semantics.RoleSwitch, Label: w.Label, Value: value, State: state}, enabled, press,
		s.Ring(switchHeight/2, c.AccentInk, body))
}

// Slider picks a value between Min and Max, which default to 0 and 1. It takes the
// width it is given, and moves with the pointer or the arrow keys. Without
// OnChange it is disabled.
type Slider struct {
	core.StatefulBase
	Value, Min, Max float32
	// Step is the amount an arrow key moves the value by, and the grid it snaps
	// to if Snap is set. Zero selects a twentieth of the range.
	Step float32
	Snap bool
	// Label describes the slider to assistive technology.
	Label string
	// Format spells the value for assistive technology, which reads it as
	// the value of the slider: "50%", "3 minutes", "Loud". The slider shows
	// no number itself. Nil reads the number with two decimals, "0.50".
	Format   func(float32) string
	OnChange func(float32)
}

func (w Slider) limits() (lo, hi float32) {
	if w.Max > w.Min {
		return w.Min, w.Max
	}
	return w.Min, w.Min + 1
}

func (w Slider) step() float32 {
	if w.Step > 0 {
		return w.Step
	}
	lo, hi := w.limits()
	return (hi - lo) / sliderStepDivisor
}

// fraction returns the place of v along the slider.
func (w Slider) fraction(v float32) float32 {
	lo, hi := w.limits()
	return geom.Clamp32((v-lo)/(hi-lo), 0, 1)
}

// valueAt returns the value at fraction, snapped if the slider snaps.
func (w Slider) valueAt(fraction float32) float32 {
	lo, hi := w.limits()
	v := lo + (hi-lo)*fraction
	if w.Snap {
		step := w.step()
		v = lo + geom.Floor32((v-lo)/step+0.5)*step
	}
	return geom.Clamp32(v, lo, hi)
}

type sliderState struct {
	core.StateBase
	behavior.Control
	dragging bool
	slide    motion.SpringValue
}

// CreateState implements core.StatefulWidget.
func (Slider) CreateState() core.State { return &sliderState{} }

// InitState implements core.State.
func (s *sliderState) InitState() {
	s.Attach(&s.StateBase)
	w := s.Widget().(Slider)
	s.slide.OnChange = func() { kit.Rebuild(&s.StateBase) }
	s.slide.Attach(&s.StateBase, w.fraction(w.Value))
}

// DidUpdateWidget implements core.State.
func (s *sliderState) DidUpdateWidget(core.Widget) {
	w := s.Widget().(Slider)
	if s.dragging {
		// The thumb stays under the pointer.
		s.slide.Jump(w.fraction(w.Value))
		return
	}
	if basic.ReduceMotionOf(s.Context()) {
		s.slide.Jump(w.fraction(w.Value))
		return
	}
	s.slide.Set(w.fraction(w.Value))
}

// Dispose implements core.State.
func (s *sliderState) Dispose() {
	s.Control.Dispose()
	s.slide.Dispose()
}

func (s *sliderState) key(e *core.KeyEvent) bool {
	w := s.Widget().(Slider)
	lo, hi := w.limits()
	v := w.Value
	switch e.Key {
	case input.KeyLeft, input.KeyDown:
		v -= w.step()
	case input.KeyRight, input.KeyUp:
		v += w.step()
	case input.KeyHome:
		v = lo
	case input.KeyEnd:
		v = hi
	default:
		return false
	}
	w.OnChange(geom.Clamp32(v, lo, hi))
	return true
}

// Build implements core.State.
func (s *sliderState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Slider)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil

	p := sliderPaint{
		fraction: s.slide.Value(), active: c.Accent, inactive: c.FillStrong,
		thumb: geom.Hex(0xffffff), edge: c.Hair, shadows: th.Color.Shadows(theme.LiftKnob),
		grow: sliderGrow*s.Hot.Get() - sliderPress*geom.Clamp32(s.Sink.Value(), 0, 1),
		ring: c.AccentInk.WithAlpha(kit.RingOpacity * s.Lit.Get()),
	}
	lo, hi := w.limits()
	sem := interact.Semantics{
		Role: semantics.RoleSlider, Label: w.Label, Value: sliderValue(w.Format, w.Value),
		Range: semantics.Range{Min: float64(lo), Max: float64(hi), Value: float64(w.Value), Step: float64(w.step())},
	}
	if !enabled {
		p.active, p.thumb, p.shadows = c.Ink3, c.Ink3, nil
		sem.State, sem.Child = semantics.Disabled, p
		return sem
	}
	sem.OnIncrease = func() { s.key(&core.KeyEvent{Key: input.KeyRight}) }
	sem.OnDecrease = func() { s.key(&core.KeyEvent{Key: input.KeyLeft}) }
	sem.OnSetValue = func(_ string, v float64) {
		w := s.Widget().(Slider)
		lo, hi := w.limits()
		w.OnChange(geom.Clamp32(float32(v), lo, hi))
	}
	p.onDrag = func(fraction float32) {
		if !s.dragging {
			s.dragging = true
			s.Press()
		}
		w := s.Widget().(Slider)
		w.OnChange(w.valueAt(fraction))
		kit.Rebuild(&s.StateBase)
	}
	p.onEnd = func() {
		s.dragging = false
		s.Release()
	}
	sem.Child = interact.Focus{
		Enabled:       true,
		OnKey:         s.key,
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child:   p,
		},
	}
	return sem
}

// sliderValue spells a value for assistive technology with format, or with two
// decimals without it.
func sliderValue(format func(float32) string, v float32) string {
	if format != nil {
		return format(v)
	}
	return strconv.FormatFloat(float64(v), 'f', 2, 32)
}

// sliderPaint is the widget of layout.RenderSlider.
type sliderPaint struct {
	fraction, grow                float32
	active, inactive, thumb, edge geom.Color
	ring                          geom.Color
	shadows                       []paint.BoxShadow
	onDrag                        func(fraction float32)
	onEnd                         func()
}

// Key implements core.Widget.
func (sliderPaint) Key() core.Key { return nil }

func (p sliderPaint) apply(r *layout.RenderSlider) {
	r.Fraction, r.Active, r.Inactive, r.Thumb, r.Edge = p.fraction, p.active, p.inactive, p.thumb, p.edge
	r.Shadows, r.Grow, r.Ring = p.shadows, p.grow, p.ring
	r.OnDrag, r.OnDragEnd = p.onDrag, p.onEnd
}

// CreateRenderObject implements core.RenderWidget.
func (p sliderPaint) CreateRenderObject() core.RenderObject {
	r := &layout.RenderSlider{}
	p.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p sliderPaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderSlider)
	p.apply(r)
	r.MarkNeedsPaint()
}

// SegmentedControl picks one of a few choices, with a plate that slides to it. It
// takes the place of a row of radio buttons. Without OnChange it is disabled.
type SegmentedControl struct {
	core.StatefulBase
	Options  []string
	Selected int
	// Label describes the control to assistive technology.
	Label    string
	OnChange func(int)
}

type segmentedState struct {
	core.StateBase
	behavior.Control
	plate motion.SpringValue
}

// CreateState implements core.StatefulWidget.
func (SegmentedControl) CreateState() core.State { return &segmentedState{} }

// InitState implements core.State.
func (s *segmentedState) InitState() {
	s.Attach(&s.StateBase)
	s.plate.Spring = motion.BouncySpring()
	s.plate.OnChange = func() { kit.Rebuild(&s.StateBase) }
	s.plate.Attach(&s.StateBase, float32(s.Widget().(SegmentedControl).Selected))
}

// DidUpdateWidget implements core.State.
func (s *segmentedState) DidUpdateWidget(core.Widget) {
	if basic.ReduceMotionOf(s.Context()) {
		s.plate.Jump(float32(s.Widget().(SegmentedControl).Selected))
		return
	}
	s.plate.Set(float32(s.Widget().(SegmentedControl).Selected))
}

// Dispose implements core.State.
func (s *segmentedState) Dispose() {
	s.Control.Dispose()
	s.plate.Dispose()
}

func (s *segmentedState) key(e *core.KeyEvent) bool {
	w := s.Widget().(SegmentedControl)
	next := w.Selected
	switch e.Key {
	case input.KeyLeft:
		next--
	case input.KeyRight:
		next++
	default:
		return false
	}
	if next >= 0 && next < len(w.Options) {
		w.OnChange(next)
	}
	return true
}

// Build implements core.State.
func (s *segmentedState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(SegmentedControl)
	th := basic.ThemeOf(ctx)
	kit.ReportField(ctx, kit.FieldChoice)
	c := &th.Color
	enabled := w.OnChange != nil
	labels := make([]core.Widget, len(w.Options))
	for i, name := range w.Options {
		style := kit.Colored(th.Text.Body, motion.LerpColor(c.Ink2, c.Ink, s.Hot.Get()))
		switch {
		case !enabled:
			style = kit.Colored(th.Text.Body, c.Ink3)
		case i == w.Selected:
			style = kit.Colored(th.Text.Control, c.Ink)
		}
		var state semantics.State
		if i == w.Selected {
			state = semantics.Selected
		}
		if !enabled {
			state |= semantics.Disabled
		}
		var pick func()
		if enabled {
			pick = func() {
				if w := s.Widget().(SegmentedControl); i != w.Selected {
					w.OnChange(i)
				}
			}
		}
		labels[i] = interact.Semantics{Role: semantics.RoleRadio, Label: name, State: state, OnTap: pick, Child: basic.Text{Content: name, Style: style}}
	}
	plate := c.Card
	if th.Brightness == theme.BrightnessDark {
		plate = c.Control
	}
	body := segmentedPaint{
		plate: s.plate.Value(), track: c.Fill, color: plate, shadows: th.Color.Shadows(theme.LiftKnob),
		radius: th.Radius.Control, children: labels,
		sink: plateSink * geom.Clamp32(s.Sink.Value(), 0, 1),
	}
	if !enabled {
		body.color, body.shadows = c.Fill, nil
	}
	value := ""
	if w.Selected >= 0 && w.Selected < len(w.Options) {
		value = w.Options[w.Selected]
	}
	if !enabled {
		return interact.Semantics{Role: semantics.RoleRadioGroup, Label: w.Label, Value: value, State: semantics.Disabled, Child: body}
	}
	body.onSelect = func(i int) {
		if i != s.Widget().(SegmentedControl).Selected {
			s.Widget().(SegmentedControl).OnChange(i)
		}
	}
	body.onPress = func(down bool) {
		if down {
			s.Press()
		} else {
			s.Release()
		}
	}
	return interact.Semantics{Role: semantics.RoleRadioGroup, Label: w.Label, Value: value, Child: interact.Focus{
		Enabled:       true,
		OnKey:         s.key,
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child:   s.Ring(th.Radius.Control, c.AccentInk, body),
		},
	}}
}

// segmentedPaint is the widget of layout.RenderSegmented.
type segmentedPaint struct {
	plate, radius, sink float32
	track, color        geom.Color
	shadows             []paint.BoxShadow
	onSelect            func(int)
	onPress             func(bool)
	children            []core.Widget
}

// Key implements core.Widget.
func (segmentedPaint) Key() core.Key { return nil }

func (p segmentedPaint) apply(r *layout.RenderSegmented) {
	r.Plate, r.Track, r.PlateColor, r.Shadows, r.Radius, r.Sink = p.plate, p.track, p.color, p.shadows, p.radius, p.sink
	r.OnSelect, r.OnPress = p.onSelect, p.onPress
}

// CreateRenderObject implements core.RenderWidget.
func (p segmentedPaint) CreateRenderObject() core.RenderObject {
	r := &layout.RenderSegmented{}
	p.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p segmentedPaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderSegmented)
	p.apply(r)
	r.MarkNeedsPaint()
}

// ChildWidgets implements core.MultiChildWidget.
func (p segmentedPaint) ChildWidgets() []core.Widget { return p.children }
