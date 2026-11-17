package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// splitKeyStep is how far an arrow key moves a divider.
const splitKeyStep = theme.Space10

// SplitCollapse says which pane of a SplitPane can fold away.
//
// It is SplitCollapseNone (the zero value), SplitCollapseFirst or
// SplitCollapseSecond.
type SplitCollapse uint8

// The panes that can collapse. The zero value is neither.
const (
	SplitCollapseNone SplitCollapse = iota
	SplitCollapseFirst
	SplitCollapseSecond
)

// String returns the name of the choice.
func (c SplitCollapse) String() string {
	switch c {
	case SplitCollapseFirst:
		return "first"
	case SplitCollapseSecond:
		return "second"
	}
	return "none"
}

// SplitPane shows two panes side by side, or one above the other with Axis
// layout.Vertical, with a divider between them that the pointer drags. Nest
// SplitPanes for more than two. It takes all the room it is given, which must be
// bounded along Axis.
//
// The divider is a hairline with a wider strip around it that takes the
// pointer, which shows a resize cursor. A double click puts it back at Default.
// It is a stop of Tab: the arrow keys move it, Home and End move it to the
// limits, and Enter folds and unfolds the pane that Collapsible names. A screen
// reader finds it as a separator whose value is the share of the first pane,
// from 0 to 100.
//
// The pane keeps the position as the user moves it, starting from Position, and
// reports each move to OnChange. A new Position from the caller replaces it, so
// a caller that keeps the value, to save it with the window, passes it back; a
// caller that does not need it leaves OnChange nil. Collapsed and OnCollapse
// work the same way.
type SplitPane struct {
	core.StatefulBase
	// Axis is the direction the panes are laid out in: Horizontal, the zero
	// value, puts First at the left.
	Axis          layout.Axis
	First, Second core.Widget
	// Position is the size of First: a fraction of the room from 0 to 1, or
	// logical pixels with Pixels. Zero selects Default.
	Position float32
	// Pixels makes Position and Default sizes in logical pixels, so that First
	// keeps its size when the window resizes, as a sidebar does. Otherwise
	// both panes grow in proportion.
	Pixels bool
	// Default is where a double click puts the divider, in the unit of
	// Position. Zero selects the middle.
	Default float32
	// MinFirst and MinSecond are the smallest sizes of the panes, in logical
	// pixels.
	MinFirst, MinSecond float32
	// Collapsible names the pane that Enter, or a drag past half its minimum
	// size, folds away.
	Collapsible SplitCollapse
	// Collapsed folds the pane Collapsible names.
	Collapsed bool
	// Label names the divider to assistive technology. Empty selects "Pane
	// divider" in the language of the window.
	Label string
	// OnChange reports the new position, in the unit of Position.
	OnChange func(position float32)
	// OnCollapse reports that the pane was folded or unfolded.
	OnCollapse func(collapsed bool)
}

type splitPaneState struct {
	core.StateOf[SplitPane]
	behavior.Control
	// position and collapsed are those shown, which follow the user and a
	// new value from the caller.
	position  float32
	collapsed bool
	// grip is the size of First when a drag started.
	grip     float32
	dragging bool
	layout   *renderSplit
}

// CreateState implements core.StatefulWidget.
func (SplitPane) CreateState() core.State { return &splitPaneState{} }

// InitState implements core.State.
func (s *splitPaneState) InitState() {
	s.Attach(&s.StateBase)
	w := s.Widget()
	s.position, s.collapsed = w.Position, w.Collapsed
}

// DidUpdateWidget implements core.State.
func (s *splitPaneState) DidUpdateWidget(old core.Widget) {
	w, o := s.Widget(), old.(SplitPane)
	if w.Position != o.Position {
		s.position = w.Position
	}
	if w.Collapsed != o.Collapsed {
		s.collapsed = w.Collapsed
	}
}

// Dispose implements core.State.
func (s *splitPaneState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *splitPaneState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	active := max(s.Hot.Get(), kit.OnOff(s.dragging))
	lit := s.Lit.Get()
	line := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, active), c.AccentInk, lit)
	label := w.Label
	if label == "" {
		label = basic.StringsOf(ctx).PaneDivider
	}
	sem := interact.Semantics{
		Role: semantics.RoleSeparator, Label: label,
		OnIncrease: func() { s.moveBy(splitKeyStep) },
		OnDecrease: func() { s.moveBy(-splitKeyStep) },
		OnSetValue: func(_ string, number float64) { s.setPercent(float32(number)) },
	}
	if r := s.layout; r != nil && r.room > 0 {
		sem.Range = semantics.Range{
			Max: 100, Value: float64(r.first / r.room * 100), Step: float64(splitKeyStep / r.room * 100),
		}
	}
	if w.Collapsible != SplitCollapseNone {
		sem.State = semantics.Expanded
		if s.collapsed {
			sem.State = semantics.Collapsed
		}
		sem.OnExpand = func() { s.collapse(false) }
		sem.OnCollapse = func() { s.collapse(true) }
	}
	sem.Child = interact.Focus{
		Enabled:       true,
		OnKey:         s.key,
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: kit.SplitHandle{
			Axis: w.Axis, Line: line, Ring: c.AccentInk.WithAlpha(kit.RingOpacity * lit),
			OnHover: s.Hover,
			OnStart: s.dragStart,
			OnDrag:  s.drag,
			OnEnd:   s.dragEnd,
			OnReset: s.reset,
		},
	}
	collapsed := SplitCollapseNone
	if s.collapsed {
		collapsed = w.Collapsible
	}
	return splitLayout{
		axis: w.Axis, position: s.position, pixels: w.Pixels, def: w.Default,
		minFirst: w.MinFirst, minSecond: w.MinSecond, collapsed: collapsed,
		ref: &s.layout, onLayout: s.laidOut,
		children: [3]core.Widget{
			kit.PaneSlot{Child: orEmpty(w.First)},
			sem,
			kit.PaneSlot{Child: orEmpty(w.Second)},
		},
	}
}

// laidOut follows a layout that moved the divider, so that the value a screen
// reader reads is that of the line it sees.
func (s *splitPaneState) laidOut() { kit.Rebuild(&s.StateBase) }

// orEmpty returns w, or an empty box for nil.
func orEmpty(w core.Widget) core.Widget {
	if w == nil {
		return basic.SizedBox{}
	}
	return w
}

// key moves the divider with the arrow keys along its axis, Home and End, and
// folds the collapsible pane with Enter.
func (s *splitPaneState) key(e *core.KeyEvent) bool {
	w := s.Widget()
	if e.Modifiers != 0 {
		return false
	}
	back, forth := input.KeyLeft, input.KeyRight
	if w.Axis == layout.Vertical {
		back, forth = input.KeyUp, input.KeyDown
	}
	r := s.layout
	switch e.Key {
	case back:
		s.moveBy(-splitKeyStep)
	case forth:
		s.moveBy(splitKeyStep)
	case input.KeyHome:
		if r != nil {
			s.setFirst(r.lo, true)
		}
	case input.KeyEnd:
		if r != nil {
			s.setFirst(r.hi, true)
		}
	case input.KeyEnter:
		if w.Collapsible == SplitCollapseNone || e.Repeat {
			return w.Collapsible != SplitCollapseNone
		}
		s.collapse(!s.collapsed)
	default:
		return false
	}
	return true
}

// moveBy moves the divider by d logical pixels from where it shows.
func (s *splitPaneState) moveBy(d float32) {
	if r := s.layout; r != nil {
		s.setFirst(r.first+d, false)
	}
}

// setPercent moves the divider to give First p percent of the room.
func (s *splitPaneState) setPercent(p float32) {
	if r := s.layout; r != nil {
		s.setFirst(geom.Clamp32(p, 0, 100)/100*r.room, false)
	}
}

func (s *splitPaneState) dragStart() {
	s.dragging = true
	if r := s.layout; r != nil {
		s.grip = r.first
	}
	kit.Rebuild(&s.StateBase)
}

func (s *splitPaneState) drag(delta float32) { s.setFirst(s.grip+delta, false) }

func (s *splitPaneState) dragEnd() {
	s.dragging = false
	kit.Rebuild(&s.StateBase)
}

// setFirst gives First size px, within the limits of the panes. Away from a
// limit, a collapsible pane folds when it would be less than half its minimum
// size, and unfolds otherwise; exact keeps it as it is, for Home and End.
func (s *splitPaneState) setFirst(px float32, exact bool) {
	r := s.layout
	if r == nil || r.room <= 0 {
		return
	}
	w := s.Widget()
	fold := s.collapsed
	if !exact {
		switch w.Collapsible {
		case SplitCollapseFirst:
			fold = px < r.lo/2 || (r.lo == 0 && px <= 0)
		case SplitCollapseSecond:
			fold = r.room-px < (r.room-r.hi)/2 || (r.hi == r.room && px >= r.room)
		}
	} else {
		fold = false
	}
	if !fold {
		px = geom.Clamp32(px, r.lo, r.hi)
		if pos := r.toPosition(px); pos != s.position {
			s.position = pos
			if w.OnChange != nil {
				w.OnChange(pos)
			}
		}
	}
	s.setCollapsed(fold)
	kit.Rebuild(&s.StateBase)
}

// collapse folds or unfolds the collapsible pane, which comes back where it was.
func (s *splitPaneState) collapse(fold bool) {
	if s.Widget().Collapsible == SplitCollapseNone {
		return
	}
	s.setCollapsed(fold)
	kit.Rebuild(&s.StateBase)
}

func (s *splitPaneState) setCollapsed(fold bool) {
	w := s.Widget()
	if w.Collapsible == SplitCollapseNone || fold == s.collapsed {
		return
	}
	s.collapsed = fold
	if w.OnCollapse != nil {
		w.OnCollapse(fold)
	}
}

// reset puts the divider back at Default.
func (s *splitPaneState) reset() {
	w := s.Widget()
	pos := w.Default
	if pos <= 0 {
		pos = 0.5
		if w.Pixels {
			pos = 0
			if r := s.layout; r != nil {
				pos = geom.Floor32(r.room/2 + 0.5)
			}
		}
	}
	changed := pos != s.position
	s.position = pos
	s.setCollapsed(false)
	if changed && w.OnChange != nil {
		w.OnChange(pos)
	}
	kit.Rebuild(&s.StateBase)
}

// splitLayout is the widget of renderSplit.
type splitLayout struct {
	axis                layout.Axis
	position, def       float32
	pixels              bool
	minFirst, minSecond float32
	collapsed           SplitCollapse
	ref                 **renderSplit
	onLayout            func()
	children            [3]core.Widget
}

// Key implements core.Widget.
func (splitLayout) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l splitLayout) CreateRenderObject() core.RenderObject {
	r := &renderSplit{}
	l.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (l splitLayout) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderSplit)
	if r.axis != l.axis || r.position != l.position || r.pixels != l.pixels || r.def != l.def ||
		r.minFirst != l.minFirst || r.minSecond != l.minSecond || r.collapsed != l.collapsed {
		r.axis, r.position, r.pixels, r.def = l.axis, l.position, l.pixels, l.def
		r.minFirst, r.minSecond, r.collapsed = l.minFirst, l.minSecond, l.collapsed
		r.MarkNeedsLayout()
	}
	r.onLayout = l.onLayout
	*l.ref = r
}

// ChildWidgets implements core.MultiChildWidget.
func (l splitLayout) ChildWidgets() []core.Widget { return l.children[:] }

// splitUnbounded are the panics of a split without a length, by its axis.
var splitUnbounded = [...]string{
	layout.Horizontal: "gowaku: SplitPane has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or Expanded",
	layout.Vertical:   "gowaku: SplitPane has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or Expanded",
}

// renderSplit lays out the two panes and the strip of the divider over the line
// between them. It remembers the limits of the last layout, which the keys and
// the drag of the divider work within.
type renderSplit struct {
	core.RenderBase
	axis                layout.Axis
	position, def       float32
	pixels              bool
	minFirst, minSecond float32
	collapsed           SplitCollapse
	children            []core.RenderObject
	// onLayout reports a layout that changed room or first.
	onLayout func()

	// room is the length shared by the panes, first the size of First, and lo
	// and hi its limits, all in logical pixels at the last layout.
	room, first, lo, hi float32
	// reported are room and first as onLayout last saw them.
	reported [2]float32
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderSplit) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderSplit) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderSplit) ChildAt(i int) core.RenderObject { return r.children[i] }

// toPosition turns a size of First into the unit of Position.
func (r *renderSplit) toPosition(px float32) float32 {
	if r.pixels || r.room <= 0 {
		return px
	}
	return px / r.room
}

// want returns the size First asks for, before the limits.
func (r *renderSplit) want() float32 {
	pos := r.position
	if pos <= 0 {
		pos = r.def
	}
	switch {
	case pos <= 0:
		return r.room / 2
	case r.pixels:
		return pos
	}
	return pos * r.room
}

// Layout implements core.RenderObject.
func (r *renderSplit) Layout(c geom.Constraints) geom.Size {
	main, cross := c.MaxW, c.MaxH
	if r.axis == layout.Vertical {
		main, cross = c.MaxH, c.MaxW
	}
	if main >= geom.Inf {
		panic(splitUnbounded[r.axis])
	}
	if cross >= geom.Inf {
		// Without a bound across, the panes are as long as the longer one.
		cross = 0
		for i := range 2 {
			sz := core.LayoutChild(r.children[i], r.box(geom.Inf, geom.Inf))
			cross = max(cross, r.acrossOf(sz))
		}
	}
	r.room = max(main-kit.DividerThickness, 0)
	r.lo = min(r.minFirst, r.room)
	r.hi = max(r.room-r.minSecond, r.lo)
	r.first = geom.Floor32(geom.Clamp32(r.want(), r.lo, r.hi) + 0.5)
	switch r.collapsed {
	case SplitCollapseFirst:
		r.first = 0
	case SplitCollapseSecond:
		r.first = r.room
	}
	first, second := r.slot(0), r.slot(2)
	first.Folded = r.collapsed == SplitCollapseFirst
	second.Folded = r.collapsed == SplitCollapseSecond
	core.LayoutChild(first, r.tight(r.first, cross))
	first.SetOffset(geom.Offset{})
	core.LayoutChild(second, r.tight(r.room-r.first, cross))
	second.SetOffset(r.at(r.first + kit.DividerThickness))
	handle := r.children[1]
	core.LayoutChild(handle, r.tight(kit.SplitHit, cross))
	handle.SetOffset(r.at(r.first - (kit.SplitHit-kit.DividerThickness)/2))
	if seen := [2]float32{r.room, r.first}; seen != r.reported {
		r.reported = seen
		if r.onLayout != nil {
			r.onLayout()
		}
	}
	if r.axis == layout.Vertical {
		return c.Constrain(geom.Size{W: cross, H: main})
	}
	return c.Constrain(geom.Size{W: main, H: cross})
}

func (r *renderSplit) slot(i int) *kit.RenderPaneSlot { return r.children[i].(*kit.RenderPaneSlot) }

// tight returns constraints of length along and width across the axis.
func (r *renderSplit) tight(along, across float32) geom.Constraints {
	if r.axis == layout.Vertical {
		return geom.Tight(geom.Size{W: across, H: along})
	}
	return geom.Tight(geom.Size{W: along, H: across})
}

// box returns loose constraints of at most along and across the axis.
func (r *renderSplit) box(along, across float32) geom.Constraints {
	if r.axis == layout.Vertical {
		return geom.Constraints{MaxW: across, MaxH: along}
	}
	return geom.Constraints{MaxW: along, MaxH: across}
}

func (r *renderSplit) acrossOf(sz geom.Size) float32 {
	if r.axis == layout.Vertical {
		return sz.W
	}
	return sz.H
}

// at returns the offset of a point d along the axis.
func (r *renderSplit) at(d float32) geom.Offset {
	if r.axis == layout.Vertical {
		return geom.Offset{Y: d}
	}
	return geom.Offset{X: d}
}

// Paint implements core.RenderObject. The divider comes last, over the edges
// of the panes that its focus band reaches.
func (r *renderSplit) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, i := range [3]int{0, 2, 1} {
		child := r.children[i]
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// HitTest implements core.RenderObject. The strip of the divider overlaps the
// panes, and takes the pointer first.
func (r *renderSplit) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	for _, i := range [3]int{1, 2, 0} {
		child := r.children[i]
		if h, ok := child.(core.Hider); ok && h.Hidden() {
			continue
		}
		if child.HitTest(pos.Sub(child.Offset()), res) {
			break
		}
	}
	res.Path = append(res.Path, r)
	return true
}
