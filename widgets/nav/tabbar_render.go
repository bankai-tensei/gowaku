package nav

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// tabBarRow is the widget of renderTabBarRow.
type tabBarRow struct {
	ref    **renderTabBarRow
	line   geom.Color
	hasNew bool
	// children are the strip, the new tab button and the slot of the all
	// tabs button.
	children []core.Widget
}

// Key implements core.Widget.
func (tabBarRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabBarRow) CreateRenderObject() core.RenderObject {
	r := &renderTabBarRow{}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabBarRow) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTabBarRow)
	if r.line != t.line {
		r.line = t.line
		r.MarkNeedsPaint()
	}
	if r.hasNew != t.hasNew {
		r.hasNew = t.hasNew
		r.MarkNeedsLayout()
	}
	*t.ref = r
}

// ChildWidgets implements core.MultiChildWidget.
func (t tabBarRow) ChildWidgets() []core.Widget { return t.children }

// renderTabBarRow lays out the strip of a TabBar with the new tab button after
// the last tab, and the all tabs button at the end once the tabs no longer fit,
// over a hairline along the bottom that the selected tab covers.
type renderTabBarRow struct {
	core.RenderBase
	line     geom.Color
	hasNew   bool
	children []core.RenderObject
	// overflow is whether the tabs did not fit at the last layout.
	overflow bool
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTabBarRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderTabBarRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderTabBarRow) ChildAt(i int) core.RenderObject { return r.children[i] }

// run returns the strip below the first child.
func (r *renderTabBarRow) run() *renderTabRun {
	ro := r.children[0]
	for {
		if run, ok := ro.(*renderTabRun); ok {
			return run
		}
		if ro.ChildCount() == 0 {
			return nil
		}
		ro = ro.ChildAt(0)
	}
}

// Layout implements core.RenderObject.
func (r *renderTabBarRow) Layout(c geom.Constraints) geom.Size {
	if len(r.children) < 3 {
		return c.Constrain(geom.Size{H: tabBarHeight})
	}
	loose := geom.Constraints{MaxW: geom.Inf, MaxH: tabBarHeight}
	add := core.LayoutChild(r.children[1], loose)
	more := core.LayoutChild(r.children[2], loose)
	n := 0
	if run := r.run(); run != nil {
		n = run.shown()
	}
	width := c.MaxW
	avail := width - add.W
	r.overflow = width < geom.Inf && float32(n)*tabMinWidth > avail
	if r.overflow {
		avail -= more.W + tabButtonGap
	}
	strip := core.LayoutChild(r.children[0], geom.Constraints{MaxW: max(avail, 0), MinH: tabBarHeight, MaxH: tabBarHeight})
	r.children[0].SetOffset(geom.Offset{})
	r.children[1].SetOffset(geom.Offset{X: strip.W})
	if width >= geom.Inf {
		width = strip.W + add.W
	}
	r.children[2].SetOffset(geom.Offset{X: width - more.W - tabButtonGap})
	if slot, ok := r.children[2].(*renderToolbarSlot); ok && slot.hidden == r.overflow {
		slot.hidden = !r.overflow
		slot.MarkNeedsPaint()
	}
	return c.Constrain(geom.Size{W: width, H: tabBarHeight})
}

// Paint implements core.RenderObject.
func (r *renderTabBarRow) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: off.Y + sz.H - 1, W: sz.W, H: 1}}, paint.Paint{Color: r.line})
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// tabRun is the widget of renderTabRun.
type tabRun struct {
	ref **renderTabRun
	// slots are where the children are, in tabs; count is the number of
	// tabs, and a child after them is a tab of another bar on its way here.
	slots    []float32
	count    int
	selected int
	// hidden is the tab off the strip, and dragged the one moved along it
	// with its left edge at dragX; -1 for none.
	hidden, dragged int
	dragX           float32
	onDown          func(i int, pos geom.Offset)
	onMove, onUp    func(pos geom.Offset)
	onCancel        func()
	onMiddle        func(i int)
	children        []core.Widget
}

// Key implements core.Widget.
func (tabRun) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabRun) CreateRenderObject() core.RenderObject {
	r := &renderTabRun{revealed: -1, middle: -1}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabRun) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTabRun)
	if !sameSlots(r.slots, t.slots) || r.count != t.count || r.selected != t.selected || r.hidden != t.hidden ||
		r.dragged != t.dragged || r.dragX != t.dragX {
		r.slots = append(r.slots[:0], t.slots...)
		r.count, r.selected, r.hidden, r.dragged, r.dragX = t.count, t.selected, t.hidden, t.dragged, t.dragX
		r.MarkNeedsLayout()
	}
	r.onDown, r.onMove, r.onUp, r.onCancel, r.onMiddle = t.onDown, t.onMove, t.onUp, t.onCancel, t.onMiddle
	*t.ref = r
}

func sameSlots(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ChildWidgets implements core.MultiChildWidget.
func (t tabRun) ChildWidgets() []core.Widget { return t.children }

// renderTabRun sets the tabs of a TabBar side by side, each in its slot, and
// scrolls them when they do not fit. It takes the presses of the tabs, which
// the TabBar turns into selection and drags, and the wheel.
type renderTabRun struct {
	core.RenderBase
	slots           []float32
	count, selected int
	hidden, dragged int
	dragX           float32
	onDown          func(i int, pos geom.Offset)
	onMove, onUp    func(pos geom.Offset)
	onCancel        func()
	onMiddle        func(i int)

	// tabW is the width of every tab, content that of all of them, and
	// scroll how far they are scrolled.
	tabW, content, scroll float32
	// revealed is the selected tab that was last scrolled into view.
	revealed int
	// pressed is a press of the primary button, and middle the tab of one of
	// the middle button, or -1.
	pressed  bool
	middle   int
	children []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTabRun) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderTabRun) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderTabRun) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *renderTabRun) Layout(c geom.Constraints) geom.Size {
	shown := r.shown()
	tw := float32(tabMaxWidth)
	if shown > 0 && c.MaxW < geom.Inf {
		tw = geom.Clamp32(geom.Floor32(c.MaxW/float32(shown)), tabMinWidth, tabMaxWidth)
	}
	r.tabW = tw
	r.content = float32(shown) * tw
	width := min(r.content, c.MaxW)
	maxScroll := max(r.content-width, 0)
	if r.selected != r.revealed && r.selected >= 0 && r.selected < r.count && r.selected < len(r.slots) {
		x := float32(r.selected) * tw
		if x < r.scroll {
			r.scroll = x
		} else if x+tw > r.scroll+width {
			r.scroll = x + tw - width
		}
		r.revealed = r.selected
	}
	r.scroll = geom.Clamp32(r.scroll, 0, maxScroll)
	tab := geom.Tight(geom.Size{W: tw, H: tabBarHeight})
	for i, child := range r.children {
		core.LayoutChild(child, tab)
		x := r.slots[i] * tw
		if i == r.dragged {
			x = r.dragX
		}
		child.SetOffset(geom.Offset{X: x - r.scroll})
	}
	return c.Constrain(geom.Size{W: width, H: tabBarHeight})
}

// shown returns how many tabs take a slot: those on the strip, and one of
// another bar on its way here. They share the width.
func (r *renderTabRun) shown() int {
	n := len(r.children)
	if r.hidden >= 0 {
		n--
	}
	return n
}

// ChildClip implements core.Clipper.
func (r *renderTabRun) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject. The dragged tab goes on top.
func (r *renderTabRun) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	for i, child := range r.children {
		if i != r.hidden && i != r.dragged {
			child.Paint(cv, off.Add(child.Offset()))
		}
	}
	if r.dragged >= 0 && r.dragged < len(r.children) {
		child := r.children[r.dragged]
		child.Paint(cv, off.Add(child.Offset()))
	}
	cv.Restore()
}

// HitTest implements core.RenderObject. The tab off the strip and one coming
// from another bar take no pointer.
func (r *renderTabRun) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	sz := r.Size()
	if pos.X < 0 || pos.Y < 0 || pos.X >= sz.W || pos.Y >= sz.H {
		return false
	}
	if i := r.tabAt(pos); i >= 0 {
		child := r.children[i]
		child.HitTest(pos.Sub(child.Offset()), res)
	}
	res.Path = append(res.Path, r)
	return true
}

// tabAt returns the tab at pos, the dragged one first, or -1.
func (r *renderTabRun) tabAt(pos geom.Offset) int {
	inside := func(i int) bool {
		off, sz := r.children[i].Offset(), r.children[i].Size()
		return pos.X >= off.X && pos.X < off.X+sz.W && pos.Y >= off.Y && pos.Y < off.Y+sz.H
	}
	if r.dragged >= 0 && r.dragged < r.count && inside(r.dragged) {
		return r.dragged
	}
	for i := min(r.count, len(r.children)) - 1; i >= 0; i-- {
		if i != r.hidden && inside(i) {
			return i
		}
	}
	return -1
}

// HandlePointer implements core.PointerTarget.
func (r *renderTabRun) HandlePointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		i := r.tabAt(e.Pos)
		if i < 0 || !pressButton(e.Button) {
			return
		}
		if e.Button == input.ButtonMiddle {
			if r.onMiddle != nil {
				r.middle = i
				e.Handled = true
			}
			return
		}
		if r.onDown != nil {
			r.pressed = true
			e.Handled = true
			r.onDown(i, e.Pos)
		}
	case core.PointerMove:
		if r.pressed && r.onMove != nil {
			r.onMove(e.Pos)
		}
	case core.PointerUp:
		if r.middle >= 0 {
			if i := r.tabAt(e.Pos); i == r.middle && r.onMiddle != nil {
				r.onMiddle(i)
			}
			r.middle = -1
		}
		if r.pressed && e.Button == input.ButtonPrimary {
			r.pressed = false
			if r.onUp != nil {
				r.onUp(e.Pos)
			}
		}
	case core.PointerCancel:
		r.middle = -1
		if r.pressed {
			r.pressed = false
			if r.onCancel != nil {
				r.onCancel()
			}
		}
	case core.PointerScroll:
		maxScroll := max(r.content-r.Size().W, 0)
		if maxScroll <= 0 {
			return
		}
		d := e.Scroll.X
		if d == 0 {
			d = e.Scroll.Y
		}
		r.scroll = geom.Clamp32(r.scroll-d*tabScrollStep, 0, maxScroll)
		r.MarkNeedsLayout()
		e.Handled = true
	}
}

// tabView is one tab of a TabBar.
type tabView struct {
	core.StatefulBase
	title       string
	icon, cross *paint.Path
	selected    bool
	enabled     bool
	// italic slants the title.
	italic bool
	// modified draws a dot in the place of the close button, which the
	// semantics read as modifiedValue.
	modified      bool
	modifiedValue string
	// separator draws a line at the right edge, and ghost shows a tab of
	// another bar that would drop here.
	separator, ghost bool
	// quiet keeps the tab from lighting up under the pointer, while another
	// is dragged over it.
	quiet          bool
	ring           geom.Color
	onTap, onClose func()
	onHover        func(inside bool)
	closeLabel     string
}

type tabViewState struct {
	core.StateOf[tabView]
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (tabView) CreateState() core.State { return &tabViewState{} }

// InitState implements core.State.
func (s *tabViewState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *tabViewState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *tabViewState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	hot := s.Hot.Get()
	if w.selected || w.ghost || w.quiet {
		hot = 0
	}
	ink := motion.LerpColor(c.Ink2, c.Ink, hot)
	switch {
	case !w.enabled && !w.ghost:
		ink = c.Ink3
	case w.selected:
		ink = c.Ink
	}
	style := kit.Colored(th.Text.Control, ink)
	style.Italic = style.Italic || w.italic

	row := make([]core.Widget, 0, 3)
	if w.icon != nil {
		row = append(row, basic.Padding{Insets: geom.Insets{Right: tabIconGap}, Child: basic.Icon{Path: w.icon, Size: tabIconSize, Color: ink}})
	}
	row = append(row, basic.Expanded{Child: tabTitle{text: w.title, style: style}})
	switch {
	case w.onClose != nil || w.ghost:
		row = append(row, s.closeButton(th, w))
	case w.modified:
		row = append(row, basic.Padding{Insets: geom.Insets{Left: tabCloseGap}, Child: tabDot(ink)})
	}
	body := tabSurface{
		selected: w.selected, ghost: w.ghost, hot: hot, radius: th.Radius.Control,
		card: c.Card, hair: c.Hair, fill: c.Fill, ring: w.ring, separator: w.separator,
		child: basic.Padding{
			Insets: geom.Insets{Left: tabInset, Right: tabEndInset, Top: tabTop},
			Child:  basic.Row{Cross: layout.CrossCenter, Children: row},
		},
	}
	if w.ghost {
		return body
	}
	sem := interact.Semantics{Role: semantics.RoleTab, Label: w.title}
	if w.modified {
		sem.Value = w.modifiedValue
	}
	if w.selected {
		sem.State = semantics.Selected
	}
	if !w.enabled {
		sem.State |= semantics.Disabled
		sem.Child = body
		return sem
	}
	sem.OnTap = w.onTap
	sem.Child = interact.MouseRegion{
		OnEnter: func() { s.hoverTab(true) },
		OnExit:  func() { s.hoverTab(false) },
		Child:   body,
	}
	return sem
}

// hoverTab follows the pointer, and tells the bar.
func (s *tabViewState) hoverTab(inside bool) {
	s.Hover(inside)
	if f := s.Widget().onHover; f != nil {
		f(inside)
	}
}

// closeButton returns the × of a tab. It shows on the selected tab and the one
// under the pointer; elsewhere it keeps its room but neither shows nor takes
// the pointer.
func (s *tabViewState) closeButton(th *theme.Theme, w tabView) core.Widget {
	c := &th.Color
	hovered := s.Hovered && !w.quiet
	shown := w.selected || hovered
	if w.ghost {
		return basic.SizedBox{Width: tabCloseSize + tabCloseGap}
	}
	// A modified tab shows its dot until the pointer is over it, when the
	// dot turns into the close button, as in editors. The dot takes the
	// pointer like the button it stands for on the selected tab.
	dot := w.modified && !hovered
	button := interact.Pressable{
		Label: w.closeLabel, OnPress: w.onClose, SkipTab: true,
		Build: func(p interact.PressState) core.Widget {
			var ground geom.Color
			switch {
			case p.Pressed:
				ground = c.FillStrong
			case p.Hovered:
				ground = c.Fill
			}
			ink := c.Ink2
			if p.Hovered || p.Pressed {
				ink = c.Ink
			}
			if !shown {
				ink = ink.WithAlpha(0)
			}
			if dot && !p.Hovered && !p.Pressed {
				return basic.Container{Width: tabCloseSize, Height: tabCloseSize, Color: ground, Child: tabDot(c.Ink2)}
			}
			return basic.Container{
				Width: tabCloseSize, Height: tabCloseSize, Radius: th.Radius.Check + 2, Color: ground,
				Child: basic.Icon{Path: s.Widget().cross, ViewBox: tabCloseSize, Size: tabCloseSize, Color: ink},
			}
		},
	}
	var child core.Widget = button
	if !shown {
		child = basic.IgnorePointer{Child: button}
	}
	return basic.Padding{Insets: geom.Insets{Left: tabCloseGap}, Child: child}
}

// tabDot is the dot of a modified tab, centered in the square of the close
// button.
func tabDot(ink geom.Color) core.Widget {
	return basic.SizedBox{Width: tabCloseSize, Height: tabCloseSize, Child: basic.Center{Child: basic.Container{
		Width: tabDotSize, Height: tabDotSize, Radius: tabDotSize / 2, Color: ink,
	}}}
}

// tabSurface is the widget of renderTabSurface.
type tabSurface struct {
	selected, ghost, separator bool
	hot, radius                float32
	card, hair, fill, ring     geom.Color
	child                      core.Widget
}

// Key implements core.Widget.
func (tabSurface) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabSurface) CreateRenderObject() core.RenderObject {
	r := &renderTabSurface{}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabSurface) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTabSurface)
	if r.selected != t.selected || r.ghost != t.ghost || r.separator != t.separator || r.hot != t.hot ||
		r.radius != t.radius || r.card != t.card || r.hair != t.hair || r.fill != t.fill || r.ring.Color != t.ring {
		r.selected, r.ghost, r.separator, r.hot, r.radius = t.selected, t.ghost, t.separator, t.hot, t.radius
		r.card, r.hair, r.fill = t.card, t.hair, t.fill
		r.ring = paint.Stroke{Width: kit.TabRingWidth, Color: t.ring}
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t tabSurface) ChildWidget() core.Widget { return t.child }

// renderTabSurface draws the ground of a tab: the selected one is a card with
// its top corners rounded, standing on the bottom edge, outlined by a
// hairline; one under the pointer has a soft fill.
type renderTabSurface struct {
	layout.RenderProxy
	selected, ghost, separator bool
	hot, radius                float32
	card, hair, fill           geom.Color
	// ring is kept here so that painting takes its address without
	// allocating.
	ring paint.Stroke
}

// Paint implements core.RenderObject.
func (r *renderTabSurface) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	body := geom.Rect{X: off.X, Y: off.Y + tabTop, W: sz.W, H: sz.H - tabTop}
	if r.ghost {
		cv.Save()
		cv.PushOpacity(tabGhostOpacity)
	}
	if r.separator {
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{
			X: off.X + sz.W - 1, Y: body.Y + (body.H-tabSeparator)/2, W: 1, H: tabSeparator,
		}}, paint.Paint{Color: r.hair})
	}
	switch {
	case r.selected:
		if r.ring.Color.A > 0 {
			ring := geom.Rect{X: body.X - kit.TabRingWidth, Y: body.Y - kit.TabRingWidth, W: body.W + 2*kit.TabRingWidth, H: body.H + kit.TabRingWidth}
			cv.DrawRRect(topRounded(ring, r.radius+kit.TabRingWidth), paint.Paint{Stroke: &r.ring})
		}
		outline := geom.Rect{X: body.X - 1, Y: body.Y - 1, W: body.W + 2, H: body.H + 1}
		cv.DrawRRect(topRounded(outline, r.radius+1), paint.Paint{Color: r.hair})
		cv.DrawRRect(topRounded(body, r.radius), paint.Paint{Color: r.card})
	case r.hot > 0:
		ground := geom.Rect{X: body.X + 2, Y: body.Y, W: body.W - 4, H: body.H - 4}
		cv.DrawRRect(geom.RRectAll(ground, r.radius), paint.Paint{Color: r.fill.WithAlpha(r.fill.A * r.hot)})
	}
	r.RenderProxy.Paint(cv, off)
	if r.ghost {
		cv.Restore()
	}
}

// topRounded returns r with its top corners rounded by radius.
func topRounded(r geom.Rect, radius float32) geom.RRect {
	return geom.RRect{Rect: r, TopLeft: radius, TopRight: radius}
}

// tabTitle is the title of a tab, on one line, cut short with an ellipsis.
type tabTitle struct {
	core.StatelessBase
	text  string
	style text.Style
}

// Build implements core.StatelessWidget.
func (t tabTitle) Build(ctx core.BuildContext) core.Widget {
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: TabBar needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	return tabTitleText{shaper: shaper, text: t.text, style: kit.Themed(ctx, t.style)}
}

// tabTitleText is the widget of renderTabTitle.
type tabTitleText struct {
	shaper *text.Shaper
	text   string
	style  text.Style
}

// Key implements core.Widget.
func (tabTitleText) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabTitleText) CreateRenderObject() core.RenderObject {
	return &renderTabTitle{shaper: t.shaper, text: t.text, style: t.style}
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabTitleText) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTabTitle)
	if r.text != t.text || !r.style.SameLayout(t.style) {
		r.text, r.style = t.text, t.style
		r.full, r.shown = nil, nil
		r.MarkNeedsLayout()
		return
	}
	if r.style.PaintColor() != t.style.PaintColor() {
		r.style = t.style
		r.MarkNeedsPaint()
	}
}

// renderTabTitle lays out a title on one line. When it does not fit, it shows
// as much as fits before an ellipsis. Both layouts are kept, so that only a
// new width cuts the title again.
type renderTabTitle struct {
	core.RenderBase
	shaper *text.Shaper
	text   string
	style  text.Style
	// full is the whole title, and shown what fits in width.
	full, shown *text.Layout
	width       float32
}

// Layout implements core.RenderObject.
func (r *renderTabTitle) Layout(c geom.Constraints) geom.Size {
	if r.full == nil {
		r.full = r.shaper.Shape(r.text, r.style).Layout(geom.Inf)
		r.shown = nil
	}
	switch {
	case r.full.Size().W <= c.MaxW:
		r.shown = r.full
	case r.shown == nil || r.shown == r.full || r.width != c.MaxW:
		r.shown = r.cut(c.MaxW)
	}
	r.width = c.MaxW
	sz := r.shown.Size()
	return c.Constrain(geom.Size{W: min(sz.W, c.MaxW), H: sz.H})
}

// cut returns the longest start of the title with an ellipsis that fits in
// width, or the ellipsis alone.
func (r *renderTabTitle) cut(width float32) *text.Layout {
	ellipsis := r.shaper.Shape("…", r.style).Layout(geom.Inf)
	at := r.full.HitTest(geom.Offset{X: width - ellipsis.Size().W, Y: r.full.Size().H / 2})
	for at > 0 {
		l := r.shaper.Shape(strings.TrimRight(r.text[:at], " ")+"…", r.style).Layout(geom.Inf)
		if l.Size().W <= width {
			return l
		}
		at = r.full.PrevCaret(at)
	}
	return ellipsis
}

// Paint implements core.RenderObject.
func (r *renderTabTitle) Paint(cv *paint.Canvas, off geom.Offset) {
	color := r.style.PaintColor()
	runs := r.shown.Runs()
	for i := range runs {
		cv.DrawGlyphs(&runs[i], off, color)
	}
}

// tabRef hands its render object to the state that builds it, which opens a
// menu from it.
type tabRef struct {
	ref   **layout.RenderProxy
	child core.Widget
}

// Key implements core.Widget.
func (tabRef) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabRef) CreateRenderObject() core.RenderObject {
	r := &layout.RenderProxy{}
	*t.ref = r
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabRef) UpdateRenderObject(ro core.RenderObject) { *t.ref = ro.(*layout.RenderProxy) }

// ChildWidget implements core.SingleChildWidget.
func (t tabRef) ChildWidget() core.Widget { return t.child }

// tabShortcuts takes the keys of a TabBar from anywhere in the window.
type tabShortcuts struct {
	onShortcut func(e *core.KeyEvent) bool
	child      core.Widget
}

// Key implements core.Widget.
func (tabShortcuts) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t tabShortcuts) CreateRenderObject() core.RenderObject {
	return &layout.RenderShortcuts{OnShortcut: t.onShortcut}
}

// UpdateRenderObject implements core.RenderWidget.
func (t tabShortcuts) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderShortcuts).OnShortcut = t.onShortcut
}

// ChildWidget implements core.SingleChildWidget.
func (t tabShortcuts) ChildWidget() core.Widget { return t.child }
