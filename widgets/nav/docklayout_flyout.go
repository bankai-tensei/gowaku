package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The parts of a DockLayout that hidden panels bring: the strips along the
// edges, their tabs, and the panel that slides out over the layout with its
// header and the handle on its inner edge. See
// docs/ADR/0102-dock-autohide-nesting.md.

// The measures of the hidden panels.
const (
	// dockStripTabIcon is the size of the icon on a tab of a strip, and
	// dockStripTabRadius its corners.
	dockStripTabIcon   = 14
	dockStripTabRadius = 6
)

// sideStrip returns the strip of the hidden panels at side, or nil without
// any.
func (s *dockState) sideStrip(th *theme.Theme, label string, side DockZone) core.Widget {
	c := s.ctrl
	var tabs []core.Widget
	for _, h := range c.hidden {
		p := s.panels[h.id]
		if h.side != side || p == nil {
			continue
		}
		tabs = append(tabs, core.Keyed(h.id, dockStripTab{
			dock: s, id: h.id, title: p.Title, icon: p.Icon, italic: p.Italic, side: side, open: c.flyout == h.id,
		}))
	}
	if len(tabs) == 0 {
		return nil
	}
	line := basic.Container{Width: 1, Height: 1, Color: th.Color.Hair}
	var body core.Widget
	if side == DockLeft || side == DockRight {
		list := basic.Expanded{Child: basic.Padding{Insets: geom.Insets{Top: theme.Space4}, Child: basic.Column{
			Cross: layout.CrossCenter, Spacing: theme.Space4, Children: tabs,
		}}}
		children := []core.Widget{list, line}
		if side == DockRight {
			children = []core.Widget{line, list}
		}
		body = basic.Row{Cross: layout.CrossStretch, Children: children}
	} else {
		list := basic.Expanded{Child: basic.Padding{Insets: geom.Insets{Left: theme.Space4}, Child: basic.Row{
			Cross: layout.CrossCenter, Spacing: theme.Space4, Children: tabs,
		}}}
		children := []core.Widget{list, line}
		if side == DockBottom {
			children = []core.Widget{line, list}
		}
		body = basic.Column{Cross: layout.CrossStretch, Children: children}
	}
	return dockSlotView{keep: true, child: interact.Semantics{Role: semantics.RoleTabList, Label: label, Child: body}}
}

// flyoutChrome returns the ground and header of the hidden panel id that slid
// out: its title, a close button if it is closable, and the pin button that
// docks it back.
func (s *dockState) flyoutChrome(th *theme.Theme, l *basic.Strings, id string) dockSlotView {
	c := &th.Color
	p := s.panels[id]
	var row []core.Widget
	if p.Icon != nil {
		row = append(row, basic.Padding{Insets: geom.Insets{Right: tabIconGap}, Child: basic.Icon{Path: p.Icon, Size: tabIconSize, Color: c.Ink2}})
	}
	title := kit.Colored(th.Text.Control, c.Ink)
	title.Italic = title.Italic || p.Italic
	row = append(row, basic.Expanded{Child: dockLine{child: basic.Text{Content: p.Title, Style: title}}})
	if p.Closable {
		row = append(row, dockIconButton(th, l.CloseNamed(p.Title), s.cross, false, false, s.closeFlyout))
	}
	row = append(row, dockIconButton(th, l.AutoHide, s.unpinned, true, true, func() { s.pinFlyout(id) }))
	return dockSlotView{keep: true, catch: true, bleed: true, onFocus: s.flyoutFocus, child: basic.Container{
		Color: c.Card, Shadows: th.Color.Shadows(theme.LiftFloat),
		Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
			basic.Container{Height: tabBarHeight - 1, Padding: geom.Insets{Left: tabInset, Right: tabButtonGap, Top: tabTop}, Child: basic.Row{
				Cross: layout.CrossCenter, Children: row,
			}},
			basic.Container{Height: 1, Color: c.Hair},
		}},
	}}
}

// dockIconButton returns a quiet square button with an icon, as those of a
// TabBar. A toggle is a checkbox to assistive technology, checked or not.
func dockIconButton(th *theme.Theme, label string, icon *paint.Path, toggle, checked bool, onPress func()) core.Widget {
	c := &th.Color
	pr := interact.Pressable{Label: label, OnPress: onPress, Build: func(p interact.PressState) core.Widget {
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
		return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
			Width: tabButtonSize, Height: tabButtonSize, Radius: th.Radius.Control, Color: ground,
			Padding: geom.All((tabButtonSize - tabIconSize) / 2),
			Child:   basic.Icon{Path: icon, Size: tabIconSize, Color: ink},
		}}
	}}
	if toggle {
		pr.Role, pr.Value = semantics.RoleCheckbox, "unchecked"
		if checked {
			pr.State, pr.Value = semantics.Checked, "checked"
		}
	}
	return pr
}

// pinPath returns a push pin on the 24 unit grid: upright for a docked panel,
// or lying on its side, the point to the left, for a hidden one.
func pinPath(lying bool) *paint.Path {
	pt := func(x, y float32) (float32, float32) {
		if lying {
			return 24 - y, x
		}
		return x, y
	}
	var p paint.Path
	move := func(x, y float32) { p.MoveTo(pt(x, y)) }
	line := func(x, y float32) { p.LineTo(pt(x, y)) }
	move(8, 4)
	line(16, 4)
	move(9, 4)
	line(9, 10)
	line(6, 14)
	line(18, 14)
	line(15, 10)
	line(15, 4)
	move(12, 14)
	line(12, 20)
	return p.Stroke(paint.StrokeStyle{Width: 1.75, Cap: paint.CapRound, Join: paint.JoinRound})
}

// dockStripTab is the tab of a hidden panel on its strip: a press slides the
// panel out or hides it again, and a drag takes it elsewhere. On the left and
// right strips it reads turned, from the bottom up on the left and from the
// top down on the right.
type dockStripTab struct {
	core.StatefulBase
	dock  *dockState
	id    string
	title string
	icon  *paint.Path
	// italic slants the title, as DockPanel.Italic.
	italic bool
	side   DockZone
	open   bool
}

type dockStripTabState struct {
	core.StateOf[dockStripTab]
	hovered, focused, shown bool
}

// CreateState implements core.StatefulWidget.
func (dockStripTab) CreateState() core.State { return &dockStripTabState{} }

// Dispose implements core.State. A tab that goes during its drag ends it.
func (s *dockStripTabState) Dispose() {
	if w := s.Widget(); w.dock.stripDrag == w.id {
		w.dock.stripDragEnded()
	}
}

// Build implements core.State.
func (s *dockStripTabState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	var ground geom.Color
	ink := c.Ink2
	switch {
	case w.open:
		ground, ink = c.FillStrong, c.Ink
	case s.hovered:
		ground, ink = c.Fill, c.Ink
	}
	var row []core.Widget
	if w.icon != nil {
		row = append(row, basic.Icon{Path: w.icon, Size: dockStripTabIcon, Color: ink})
	}
	style := kit.Colored(th.Text.Caption, ink)
	style.Italic = style.Italic || w.italic
	row = append(row, basic.Text{Content: w.title, Style: style})
	var face core.Widget = interact.FocusRing{Visible: s.focused && s.shown, Radius: dockStripTabRadius, Child: basic.Container{
		Height: dockStripTabBreadth, Radius: dockStripTabRadius, Color: ground, Padding: geom.Symmetric(theme.Space10, 0),
		Child: basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space6, Children: row},
	}}
	if w.side == DockLeft || w.side == DockRight {
		face = dockTurn{clockwise: w.side == DockRight, child: face}
	}
	var state semantics.State
	if w.open {
		state = semantics.Selected
	}
	dock, id := w.dock, w.id
	toggle := func() { dock.toggleFlyout(id) }
	return interact.Semantics{Role: semantics.RoleTab, Label: w.title, State: state, OnTap: toggle, Child: interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			if !kit.Activates(e) {
				return false
			}
			if !e.Repeat {
				toggle()
			}
			return true
		},
		OnFocusChange: func(f bool) { s.set(&s.focused, f) },
		OnShowFocus:   func(f bool) { s.set(&s.shown, f) },
		Child: interact.MouseRegion{
			OnEnter: func() { s.set(&s.hovered, true) },
			OnExit:  func() { s.set(&s.hovered, false) },
			Child: dockStripTabBox{
				id: id, onTap: toggle,
				onDragStart: func() { dock.stripDragStart(id) },
				onDrag:      func(at geom.Offset) { dock.stripDragged(id, at) },
				onDrop:      func(at geom.Offset) { dock.stripDropped(id, at) },
				onCancel:    dock.stripDragEnded,
				child:       face,
			},
		},
	}}
}

// set changes a flag and rebuilds when it changed.
func (s *dockStripTabState) set(flag *bool, v bool) {
	if *flag != v {
		*flag = v
		kit.Rebuild(&s.StateBase)
	}
}

// dockStripTabBox is the widget of renderDockStripTab.
type dockStripTabBox struct {
	id                 string
	onTap, onDragStart func()
	onDrag, onDrop     func(at geom.Offset)
	onCancel           func()
	child              core.Widget
}

// Key implements core.Widget.
func (dockStripTabBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b dockStripTabBox) CreateRenderObject() core.RenderObject {
	r := &renderDockStripTab{}
	b.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b dockStripTabBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDockStripTab)
	r.id, r.onTap, r.onDragStart, r.onDrag, r.onDrop, r.onCancel = b.id, b.onTap, b.onDragStart, b.onDrag, b.onDrop, b.onCancel
}

// ChildWidget implements core.SingleChildWidget.
func (b dockStripTabBox) ChildWidget() core.Widget { return b.child }

// renderDockStripTab turns the presses of a tab of a strip into a tap, or,
// once the pointer moves away, a drag, reported in the window.
type renderDockStripTab struct {
	layout.RenderProxy
	id                 string
	onTap, onDragStart func()
	onDrag, onDrop     func(at geom.Offset)
	onCancel           func()
	pressed, dragging  bool
	start              geom.Offset
}

// HandlePointer implements core.PointerTarget.
func (r *renderDockStripTab) HandlePointer(e *core.PointerEvent) {
	at := core.LocalToGlobal(r, e.Pos)
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		r.pressed, r.dragging, r.start = true, false, at
	case core.PointerMove:
		if !r.pressed {
			return
		}
		if !r.dragging {
			if geom.Abs32(at.X-r.start.X) < tabDragSlop && geom.Abs32(at.Y-r.start.Y) < tabDragSlop {
				return
			}
			r.dragging = true
			r.onDragStart()
		}
		r.onDrag(at)
	case core.PointerUp:
		if !r.pressed {
			return
		}
		dragging := r.dragging
		r.pressed, r.dragging = false, false
		sz := r.Size()
		switch {
		case dragging:
			r.onDrop(at)
		case e.Pos.X >= 0 && e.Pos.Y >= 0 && e.Pos.X < sz.W && e.Pos.Y < sz.H:
			r.onTap()
		}
	case core.PointerCancel:
		if r.pressed {
			r.pressed, r.dragging = false, false
			r.onCancel()
		}
	}
}
