package control

import (
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

// rangePage is how many steps PageUp and PageDown move a thumb by.
const rangePage = 10

// RangeSlider picks a range, from Low to High, between Min and Max, which
// default to 0 and 1. It looks like a Slider with a second thumb, and takes the
// width it is given. Without OnChange it is disabled.
//
// A thumb moves with the pointer, and the stretch between the thumbs moves
// both. The thumbs never cross: each stops MinGap short of the other, by the
// pointer and by the keys alike. The values it asks for are always within Min
// and Max, and a thumb never pushes the other: where the gap cannot be kept
// inside the limits, as when the parent gives a Low and a High closer than
// MinGap near an end, the moving thumb stops at the end, and the range stays
// narrower than MinGap until the other thumb moves away. Each thumb is a stop of Tab of its own, the
// low one first, where the arrow keys move it by Step, PageUp and PageDown by
// ten steps, and Home and End to as far as it can go. Assistive technology
// sees a group named Label with a slider for each thumb.
//
// The parent keeps Low and High, and OnChange asks for both at once. While a
// drag lasts the thumbs follow the pointer by themselves, without a build, so
// the slider allocates nothing per frame unless the parent builds again.
type RangeSlider struct {
	core.StatefulBase
	Low, High, Min, Max float32
	// Step is the amount an arrow key moves a thumb by, and the grid they
	// snap to if Snap is set. Zero selects a twentieth of the range.
	Step float32
	Snap bool
	// MinGap is the least distance between Low and High. Zero lets the thumbs
	// meet.
	MinGap float32
	// RTL mirrors the slider for a right-to-left layout: Min is at the right
	// end, and Left and Right move a thumb the way they point on the screen.
	// Up and PageUp still raise it.
	RTL bool
	// Label describes the slider to assistive technology.
	Label string
	// Format spells the value of a thumb for assistive technology, as for
	// Slider: "$20", "18:00". Nil reads the number with two decimals.
	Format   func(float32) string
	OnChange func(low, high float32)
}

func (w RangeSlider) limits() (lo, hi float32) {
	if w.Max > w.Min {
		return w.Min, w.Max
	}
	return w.Min, w.Min + 1
}

func (w RangeSlider) step() float32 {
	if w.Step > 0 {
		return w.Step
	}
	lo, hi := w.limits()
	return (hi - lo) / sliderStepDivisor
}

// gap returns MinGap kept within the range.
func (w RangeSlider) gap() float32 {
	lo, hi := w.limits()
	return geom.Clamp32(w.MinGap, 0, hi-lo)
}

// values returns Low and High within the limits and in order.
func (w RangeSlider) values() (low, high float32) {
	lo, hi := w.limits()
	low, high = geom.Clamp32(w.Low, lo, hi), geom.Clamp32(w.High, lo, hi)
	if low > high {
		low, high = high, low
	}
	return low, high
}

// fraction returns the place of v along the slider.
func (w RangeSlider) fraction(v float32) float32 {
	lo, hi := w.limits()
	return geom.Clamp32((v-lo)/(hi-lo), 0, 1)
}

// valueAt returns the value at fraction, snapped if the slider snaps.
func (w RangeSlider) valueAt(fraction float32) float32 {
	lo, hi := w.limits()
	v := lo + (hi-lo)*fraction
	if w.Snap {
		step := w.step()
		v = lo + geom.Floor32((v-lo)/step+0.5)*step
	}
	return geom.Clamp32(v, lo, hi)
}

// bounds returns how far thumb i may go, given where the other one is.
func (w RangeSlider) bounds(i int) (from, to float32) {
	lo, hi := w.limits()
	low, high := w.values()
	if i == 0 {
		return lo, max(high-w.gap(), lo)
	}
	return min(low+w.gap(), hi), hi
}

type rangeSliderState struct {
	core.StateOf[RangeSlider]
	behavior.Control
	low, high motion.SpringValue
	// focus is the thumb that has the keyboard, or -1, and held the part a
	// press holds, while dragging.
	focus    int
	held     layout.RangePart
	dragging bool
	// reported is what OnChange last asked for during the drag.
	reported [2]float32
}

// CreateState implements core.StatefulWidget.
func (RangeSlider) CreateState() core.State { return &rangeSliderState{focus: -1} }

// InitState implements core.State.
func (s *rangeSliderState) InitState() {
	s.Attach(&s.StateBase)
	w := s.Widget()
	low, high := w.values()
	rebuild := func() { kit.Rebuild(&s.StateBase) }
	s.low.OnChange, s.high.OnChange = rebuild, rebuild
	s.low.Attach(&s.StateBase, w.fraction(low))
	s.high.Attach(&s.StateBase, w.fraction(high))
}

// DidUpdateWidget implements core.State.
func (s *rangeSliderState) DidUpdateWidget(core.Widget) {
	w := s.Widget()
	low, high := w.values()
	if s.dragging || basic.ReduceMotionOf(s.Context()) {
		// While dragging, the thumbs stay under the pointer.
		s.low.Jump(w.fraction(low))
		s.high.Jump(w.fraction(high))
		return
	}
	s.low.Set(w.fraction(low))
	s.high.Set(w.fraction(high))
}

// Dispose implements core.State.
func (s *rangeSliderState) Dispose() {
	s.Control.Dispose()
	s.low.Dispose()
	s.high.Dispose()
}

// set asks for the range low to high, if it is another.
func (s *rangeSliderState) set(low, high float32) {
	w := s.Widget()
	if l, h := w.values(); l != low || h != high {
		w.OnChange(low, high)
	}
}

// move puts thumb i at v, kept within its bounds.
func (s *rangeSliderState) move(i int, v float32) {
	w := s.Widget()
	from, to := w.bounds(i)
	v = geom.Clamp32(v, from, to)
	low, high := w.values()
	if i == 0 {
		s.set(v, high)
	} else {
		s.set(low, v)
	}
}

// key moves thumb i with the keyboard.
func (s *rangeSliderState) key(i int, e *core.KeyEvent) bool {
	w := s.Widget()
	step := w.step()
	left, right := -step, step
	if w.RTL {
		left, right = step, -step
	}
	low, high := w.values()
	v := low
	if i == 1 {
		v = high
	}
	from, to := w.bounds(i)
	switch e.Key {
	case input.KeyLeft:
		v += left
	case input.KeyRight:
		v += right
	case input.KeyDown:
		v -= step
	case input.KeyUp:
		v += step
	case input.KeyPageDown:
		v -= rangePage * step
	case input.KeyPageUp:
		v += rangePage * step
	case input.KeyHome:
		v = from
	case input.KeyEnd:
		v = to
	default:
		return false
	}
	s.move(i, v)
	return true
}

// thumbFocus follows the keyboard onto and off thumb i.
func (s *rangeSliderState) thumbFocus(i int, focused bool) {
	switch {
	case focused:
		s.focus = i
	case s.focus == i:
		s.focus = -1
	}
	s.Focus(s.focus >= 0)
	kit.Rebuild(&s.StateBase)
}

// grabbed follows a press onto a part of the slider, and gives the keyboard
// to the thumb it holds.
func (s *rangeSliderState) grabbed(part layout.RangePart) {
	if !s.dragging {
		s.dragging = true
		s.reported[0], s.reported[1] = s.Widget().values()
		s.Press()
	}
	s.held = part
	i := s.focus
	switch part {
	case layout.RangeLow:
		i = 0
	case layout.RangeHigh:
		i = 1
	}
	if i < 0 {
		i = 0
	}
	if r := findRangeSlider(s.Context().RenderObject()); r != nil && i < r.ChildCount() {
		if f, ok := kit.FirstFocusable(r.ChildAt(i)).(interface{ RequestFocus() }); ok {
			f.RequestFocus()
		}
	}
}

// dragged turns the places of a drag into values. It does not build again:
// the render object has moved the thumbs.
func (s *rangeSliderState) dragged(lowAt, highAt float32) {
	w := s.Widget()
	lo, hi := w.limits()
	low, high := w.valueAt(lowAt), w.valueAt(highAt)
	gap := w.gap()
	// As in bounds: the gap gives way to the limits, never the other thumb.
	switch s.held {
	case layout.RangeLow:
		low = min(low, max(high-gap, lo))
	case layout.RangeHigh:
		high = max(high, min(low+gap, hi))
	}
	if low == s.reported[0] && high == s.reported[1] {
		return
	}
	s.reported[0], s.reported[1] = low, high
	w.OnChange(low, high)
}

func (s *rangeSliderState) released() {
	s.dragging = false
	s.Release()
}

// Build implements core.State.
func (s *rangeSliderState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	words := basic.StringsOf(ctx)
	enabled := w.OnChange != nil
	lo, hi := w.limits()
	low, high := w.values()

	grow := sliderGrow * s.Hot.Get()
	p := rangePaint{
		low: s.low.Value(), high: s.high.Value(), mirrored: w.RTL,
		gap: w.gap() / (hi - lo), active: c.Accent, inactive: c.FillStrong,
		thumb: geom.Hex(0xffffff), edge: c.Hair, shadows: th.Color.Shadows(theme.LiftKnob),
		grow: [2]float32{grow, grow},
	}
	if w.Snap {
		p.snap = w.step() / (hi - lo)
	}
	if s.dragging && s.held != layout.RangeBar {
		p.grow[s.held] -= sliderPress * geom.Clamp32(s.Sink.Value(), 0, 1)
	}
	if s.focus >= 0 {
		p.ring[s.focus] = c.AccentInk.WithAlpha(kit.RingOpacity * s.Lit.Get())
	}
	thumb := func(i int, label string, v float32) core.Widget {
		from, to := w.bounds(i)
		sem := interact.Semantics{
			Role: semantics.RoleSlider, Label: label, Value: sliderValue(w.Format, v),
			Range: semantics.Range{Min: float64(from), Max: float64(to), Value: float64(v), Step: float64(w.step())},
			Child: basic.SizedBox{},
		}
		if !enabled {
			sem.State = semantics.Disabled
			return sem
		}
		sem.OnIncrease = func() { s.key(i, &core.KeyEvent{Key: input.KeyUp}) }
		sem.OnDecrease = func() { s.key(i, &core.KeyEvent{Key: input.KeyDown}) }
		sem.OnSetValue = func(_ string, v float64) { s.move(i, float32(v)) }
		sem.Child = interact.Focus{
			Enabled:       true,
			OnKey:         func(e *core.KeyEvent) bool { return s.key(i, e) },
			OnFocusChange: func(focused bool) { s.thumbFocus(i, focused) },
			OnShowFocus:   s.ShowFocus,
			Child:         basic.SizedBox{},
		}
		return sem
	}
	p.children = []core.Widget{thumb(0, words.RangeMinimum, low), thumb(1, words.RangeMaximum, high)}
	group := interact.Semantics{Role: semantics.RoleGroup, Label: w.Label}
	if !enabled {
		p.active, p.thumb, p.shadows = c.Ink3, c.Ink3, nil
		group.State, group.Child = semantics.Disabled, p
		return group
	}
	p.onDrag, p.onStart, p.onEnd = s.dragged, s.grabbed, s.released
	group.Child = interact.MouseRegion{
		OnEnter: func() { s.Hover(true) },
		OnExit:  func() { s.Hover(false) },
		Child:   p,
	}
	return group
}

// findRangeSlider returns the render object of the slider at or below ro.
func findRangeSlider(ro core.RenderObject) *layout.RenderRangeSlider {
	for ro != nil {
		if r, ok := ro.(*layout.RenderRangeSlider); ok {
			return r
		}
		if ro.ChildCount() == 0 {
			return nil
		}
		ro = ro.ChildAt(0)
	}
	return nil
}

// rangePaint is the widget of layout.RenderRangeSlider.
type rangePaint struct {
	low, high, gap, snap          float32
	mirrored                      bool
	active, inactive, thumb, edge geom.Color
	shadows                       []paint.BoxShadow
	grow                          [2]float32
	ring                          [2]geom.Color
	onDrag                        func(low, high float32)
	onStart                       func(layout.RangePart)
	onEnd                         func()
	children                      []core.Widget
}

// Key implements core.Widget.
func (rangePaint) Key() core.Key { return nil }

func (p rangePaint) apply(r *layout.RenderRangeSlider) {
	if r.Low != p.low || r.High != p.high || r.Mirrored != p.mirrored {
		r.MarkNeedsLayout()
	}
	r.Low, r.High, r.Mirrored, r.Gap, r.Snap = p.low, p.high, p.mirrored, p.gap, p.snap
	r.Active, r.Inactive, r.Thumb, r.Edge = p.active, p.inactive, p.thumb, p.edge
	r.Shadows, r.Grow, r.Ring = p.shadows, p.grow, p.ring
	r.OnDrag, r.OnDragStart, r.OnDragEnd = p.onDrag, p.onStart, p.onEnd
}

// CreateRenderObject implements core.RenderWidget.
func (p rangePaint) CreateRenderObject() core.RenderObject {
	r := &layout.RenderRangeSlider{}
	p.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p rangePaint) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderRangeSlider)
	p.apply(r)
	r.MarkNeedsPaint()
}

// ChildWidgets implements core.MultiChildWidget.
func (p rangePaint) ChildWidgets() []core.Widget { return p.children }
