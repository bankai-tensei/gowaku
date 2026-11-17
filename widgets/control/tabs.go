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

const (
	// tabPadding is the room on each side of the label of a tab.
	tabPadding = theme.Space12
	// tabGap parts the tabs of a strip.
	tabGap = theme.Space4
	// tabRuleGap is the room between the tabs and the line below them.
	tabRuleGap = theme.Space8
	// tabHoverGround is how much of the plate a tab under the pointer shows.
	tabHoverGround = 0.6
)

// TabPage is one page of Tabs: the label of its tab and what it shows.
type TabPage struct {
	// Label is the title the tab shows, which also names it to assistive
	// technology, as TabItem.Label of nav.TabBar does.
	Label string
	Child core.Widget
}

// Tabs shows one of Pages, picked by a strip of tabs above it. The selected tab
// sits on a plate that slides to the next one on a spring, as the plate of a
// SegmentedControl does; a hairline parts the strip from the page, which fills
// the rest of the height. Without OnChange the tabs are disabled.
//
// The strip is one stop for Tab. The left and right arrow keys select the
// previous and next tab, wrapping at the ends, and Home and End the first and
// last. Ctrl+Tab and Ctrl+Shift+Tab, or Ctrl+PageDown and Ctrl+PageUp, step
// through the tabs from anywhere inside, and bring the keyboard to the strip.
//
// The page crossfades into the next one, which starts with a new State, so
// keep what a page must remember, such as its scroll position or its inputs,
// in the State above the Tabs and pass it down. Tabs
// takes the height it is given, so inside a vertical Scroll give it one with a
// SizedBox. It is the plain kind of tab; tabs that close, move and tear off
// into windows are another widget.
type Tabs struct {
	core.StatefulBase
	Pages []TabPage
	// Selected is the index of the page shown. Out of range shows none.
	Selected int
	// Label names the strip of tabs to assistive technology.
	Label    string
	OnChange func(int)
}

type tabsState struct {
	core.StateOf[Tabs]
	behavior.Control
	plate motion.SpringValue
	// list is the focus of the strip, which the keys of a page move to.
	list *layout.RenderFocus
}

// CreateState implements core.StatefulWidget.
func (Tabs) CreateState() core.State { return &tabsState{} }

// InitState implements core.State.
func (s *tabsState) InitState() {
	s.Attach(&s.StateBase)
	s.plate.Spring = motion.BouncySpring()
	s.plate.OnChange = func() { kit.Rebuild(&s.StateBase) }
	s.plate.Attach(&s.StateBase, float32(s.Widget().Selected))
}

// DidUpdateWidget implements core.State.
func (s *tabsState) DidUpdateWidget(core.Widget) {
	to := float32(s.Widget().Selected)
	if basic.ReduceMotionOf(s.Context()) {
		s.plate.Jump(to)
		return
	}
	s.plate.Set(to)
}

// Dispose implements core.State.
func (s *tabsState) Dispose() {
	s.Control.Dispose()
	s.plate.Dispose()
}

// choose asks for page i, unless it is the one shown.
func (s *tabsState) choose(i int) {
	if w := s.Widget(); w.OnChange != nil && i != w.Selected && i >= 0 && i < len(w.Pages) {
		w.OnChange(i)
	}
}

// step selects the tab d places from the selected one, wrapping.
func (s *tabsState) step(d int) {
	w := s.Widget()
	n := len(w.Pages)
	from := w.Selected
	if from < 0 || from >= n {
		from = 0
		if d < 0 {
			from = n
		}
		if d > 0 {
			from = -1
		}
	}
	s.choose(((from+d)%n + n) % n)
}

// key is that of the strip.
func (s *tabsState) key(e *core.KeyEvent) bool {
	if e.Modifiers&(input.ModControl|input.ModAlt) != 0 {
		return false
	}
	switch e.Key {
	case input.KeyLeft:
		s.step(-1)
	case input.KeyRight:
		s.step(1)
	case input.KeyHome:
		s.choose(0)
	case input.KeyEnd:
		s.choose(len(s.Widget().Pages) - 1)
	default:
		return false
	}
	return true
}

// cycle is the key of the whole: Ctrl+Tab and its kin, wherever the focus is
// inside.
func (s *tabsState) cycle(e *core.KeyEvent) bool {
	if e.Modifiers&input.ModControl == 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	w := s.Widget()
	if w.OnChange == nil || len(w.Pages) == 0 {
		return false
	}
	switch {
	case e.Key == input.KeyTab && e.Modifiers&input.ModShift != 0, e.Key == input.KeyPageUp:
		s.step(-1)
	case e.Key == input.KeyTab, e.Key == input.KeyPageDown:
		s.step(1)
	default:
		return false
	}
	// The page with the focus is going away.
	if !s.Focused && s.list != nil && s.list.Attached() {
		s.list.RequestFocus()
	}
	return true
}

// Build implements core.State.
func (s *tabsState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	n := len(w.Pages)
	enabled := w.OnChange != nil && n > 0
	shown := w.Selected >= 0 && w.Selected < n

	tabs := make([]core.Widget, n)
	for i := range w.Pages {
		t := tabButton{label: w.Pages[i].Label, selected: i == w.Selected}
		if enabled {
			t.onDown = func() {
				s.Press()
				s.choose(i)
			}
			t.onUp = s.Release
			t.onTap = func() { s.choose(i) }
		}
		tabs[i] = t
	}
	strip := tabStrip{
		plate: s.plate.Value(), shown: shown, color: c.Fill, radius: th.Radius.Control,
		sink:     plateSink * geom.Clamp32(s.Sink.Value(), 0, 1),
		ring:     c.AccentInk.WithAlpha(kit.RingOpacity * s.Lit.Get()),
		children: tabs,
	}
	list := interact.Semantics{Role: semantics.RoleTabList, Label: w.Label, Child: kit.TabListFocus{
		Ref: &s.list, Enabled: enabled, OnKey: s.key, OnFocusChange: s.Focus, OnShowFocus: s.ShowFocus,
		Child: strip,
	}}
	if !enabled {
		list.State = semantics.Disabled
	}

	var page core.Widget = basic.SizedBox{}
	label := ""
	if shown {
		label = w.Pages[w.Selected].Label
		if child := w.Pages[w.Selected].Child; child != nil {
			page = child
		}
	}
	// Fades stay under ReduceMotion (ADR 0042); only the plate jumps.
	content := interact.Semantics{Role: semantics.RoleGroup, Label: label, Child: basic.AnimatedSwitcher{
		Duration:  kit.ToggleDuration,
		Alignment: layout.AlignTopLeft,
		Child:     core.Keyed(w.Selected, page),
	}}
	return interact.Focus{OnKey: s.cycle, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		list,
		basic.SizedBox{Height: tabRuleGap},
		basic.Container{Height: 1, Color: c.Hair},
		basic.Expanded{Child: content},
	}}}
}

// tabButton is one tab of a strip. It shows the pointer over it; the strip
// draws the plate, its press and the focus.
type tabButton struct {
	core.StatefulBase
	label    string
	selected bool
	// onDown, onUp and onTap are the press of the pointer and the tap of a
	// screen reader. Without onTap the tab is disabled.
	onDown, onUp, onTap func()
}

type tabButtonState struct {
	core.StateOf[tabButton]
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (tabButton) CreateState() core.State { return &tabButtonState{} }

// InitState implements core.State.
func (s *tabButtonState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *tabButtonState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *tabButtonState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	hot := s.Hot.Get()
	ink := motion.LerpColor(c.Ink2, c.Ink, hot)
	switch {
	case w.onTap == nil:
		ink, hot = c.Ink3, 0
	case w.selected:
		ink = c.Ink
	}
	style := kit.Colored(th.Text.Control, ink)
	body := basic.Container{
		Height:  theme.ControlHeight,
		Padding: geom.Symmetric(tabPadding, max((theme.ControlHeight-kit.LabelHeight(&style))/2, 0)),
		Radius:  th.Radius.Control,
		Color:   c.Fill.WithAlpha(c.Fill.A * tabHoverGround * hot),
		Child:   basic.Text{Content: w.label, Style: style},
	}
	sem := interact.Semantics{Role: semantics.RoleTab, Label: w.label}
	if w.selected {
		sem.State = semantics.Selected
	}
	if w.onTap == nil {
		sem.State |= semantics.Disabled
		sem.Child = body
		return sem
	}
	sem.OnTap = w.onTap
	sem.Child = interact.MouseRegion{
		OnEnter: func() { s.Hover(true) },
		OnExit:  func() { s.Hover(false) },
		Child: interact.GestureDetector{
			OnTapDown:   w.onDown,
			OnTapCancel: w.onUp,
			OnTap:       w.onUp,
			Child:       body,
		},
	}
	return sem
}

// tabStrip is the widget of renderTabStrip.
type tabStrip struct {
	plate, radius, sink float32
	shown               bool
	color, ring         geom.Color
	children            []core.Widget
}

// Key implements core.Widget.
func (tabStrip) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabStrip) CreateRenderObject() core.RenderObject {
	r := &renderTabStrip{}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabStrip) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTabStrip)
	if r.plate != t.plate || r.shown != t.shown || r.color != t.color || r.radius != t.radius ||
		r.sink != t.sink || r.ring.Color != t.ring {
		r.plate, r.shown, r.color, r.radius, r.sink = t.plate, t.shown, t.color, t.radius, t.sink
		r.ring = paint.Stroke{Width: kit.TabRingWidth, Color: t.ring}
		r.MarkNeedsPaint()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (t tabStrip) ChildWidgets() []core.Widget { return t.children }

// renderTabStrip sets tabs side by side from the left, and draws the plate of
// the selected one under them. The plate moves between tabs of different
// widths, so it lives here, where the boxes of the tabs are known; layout has
// no such object, and a strip is only for Tabs.
type renderTabStrip struct {
	core.RenderBase
	// plate is the place of the plate, in tabs; it moves between whole numbers.
	plate        float32
	shown        bool
	color        geom.Color
	radius, sink float32
	// ring is the focus ring around the plate; transparent draws none. It is
	// kept here so that painting takes its address without allocating.
	ring paint.Stroke

	children []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTabStrip) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderTabStrip) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderTabStrip) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderTabStrip) Layout(c geom.Constraints) geom.Size {
	free := geom.Constraints{MaxW: geom.Inf, MaxH: c.MaxH}
	var x, height float32
	for i, child := range r.children {
		if i > 0 {
			x += tabGap
		}
		sz := core.LayoutChild(child, free)
		child.SetOffset(geom.Offset{X: x})
		x += sz.W
		height = max(height, sz.H)
	}
	return c.Constrain(geom.Size{W: x, H: height})
}

// box returns the box of tab i.
func (r *renderTabStrip) box(i int) geom.Rect {
	off, sz := r.children[i].Offset(), r.children[i].Size()
	return geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}
}

// plateRect returns where the plate is, between the boxes of the two tabs
// around it. Past the ends, as a spring overshoots, it goes on along the line
// of the last two.
func (r *renderTabStrip) plateRect() geom.Rect {
	n := len(r.children)
	if n == 1 {
		return r.box(0)
	}
	i := min(max(int(geom.Floor32(r.plate)), 0), n-2)
	f := r.plate - float32(i)
	a, b := r.box(i), r.box(i+1)
	return geom.Rect{
		X: motion.LerpFloat(a.X, b.X, f), Y: a.Y,
		W: motion.LerpFloat(a.W, b.W, f), H: max(a.H, b.H),
	}
}

// Paint implements core.RenderObject.
func (r *renderTabStrip) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.shown && len(r.children) > 0 {
		p := r.plateRect().Translate(off)
		if r.ring.Color.A > 0 {
			ring := geom.Rect{X: p.X - kit.TabRingWidth, Y: p.Y - kit.TabRingWidth, W: p.W + 2*kit.TabRingWidth, H: p.H + 2*kit.TabRingWidth}
			cv.DrawRRect(geom.RRectAll(ring, r.radius+kit.TabRingWidth), paint.Paint{Stroke: &r.ring})
		}
		sunk := geom.Rect{X: p.X + p.W*r.sink/2, Y: p.Y + p.H*r.sink/2, W: p.W * (1 - r.sink), H: p.H * (1 - r.sink)}
		cv.DrawRRect(geom.RRectAll(sunk, r.radius), paint.Paint{Color: r.color})
	}
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
