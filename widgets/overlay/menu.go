package overlay

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The measures of a menu (docs/ADR/0046-menus-and-shortcuts.md).
const (
	// menuItemPadding is the room at the ends of an item.
	menuItemPadding = theme.Space8
	// menuCheckWidth is the column of the check marks, which every menu keeps
	// so that its labels line up whether or not an item is checked.
	menuCheckWidth    = theme.Space20
	menuCheckSize     = theme.Space12
	menuShortcutGap   = theme.Space28
	menuArrowWidth    = theme.Space16
	menuMinWidth      = 180
	menuMaxWidth      = 420
	menuSeparatorRoom = theme.Space4
)

// MenuItem is one entry of a menu: a command, a submenu or a separator.
//
//	MenuItem{Action: save}
//	MenuItem{Action: Action{Label: "最近使ったファイル(&R)"}, Items: recent}
//	MenuItem{Separator: true}
type MenuItem struct {
	// Action gives the label, the shortcut, the check mark and whether the
	// item is enabled.
	Action interact.Action
	// Items makes the item a submenu, titled by Action.Label. OnInvoke is not
	// used then.
	Items []MenuItem
	// Separator makes the item a line between groups, and the rest is unused.
	Separator bool
}

// enabled reports whether the item can be used: a submenu always can.
func (m *MenuItem) enabled() bool { return len(m.Items) > 0 || m.Action.Enabled() }

// menuShortcut runs the first enabled action below items whose shortcut e is,
// so that the shortcuts a menu shows work without an Actions around it.
func menuShortcut(items []MenuItem, e *core.KeyEvent) bool {
	for i := range items {
		it := &items[i]
		switch {
		case it.Separator:
		case len(it.Items) > 0:
			if menuShortcut(it.Items, e) {
				return true
			}
		case it.Action.Shortcut.Matches(e) && it.Action.OnInvoke != nil:
			it.Action.OnInvoke()
			return true
		}
	}
	return false
}

// menuSession is the menus open in one window at a time: a stack of panels,
// each a submenu of an item of the one before, shown in one overlay entry. A
// MenuBar or ShowPopupMenu starts it, and it ends when an item runs, on Escape,
// on a click beside the menus, or when the window loses the keyboard.
type menuSession struct {
	overlay *Host
	entry   *Entry
	layer   *menuLayerState
	panels  []*menuPanel
	// bar is the menu bar the menus belong to, or nil for a popup menu.
	bar *menuBarState
	// previous had the focus before the menus, and gets it back.
	previous core.FocusTarget
	closed   bool
}

// menuPanel is one open menu.
type menuPanel struct {
	items  []MenuItem
	labels []kit.Mnemonic
	title  string
	// anchor is what the menu opens from, in the coordinates of the overlay.
	anchor geom.Rect
	side   layout.PopupSide
	width  float32
	// hot is the item with the focus, and child the one whose submenu is
	// open. Both are -1 for none.
	hot, child int
	// focusBox and focusRow say what takes the focus when the panel first
	// appears: the panel itself, or a row. focusRow is -1 for none.
	focusBox bool
	focusRow int
	box      *layout.RenderFocus
	rows     []*layout.RenderFocus
}

// openMenu starts a session in the overlay above ctx, with items opening from
// anchor, a rectangle in the window. keyboard puts the focus on the first item.
func openMenu(ctx core.BuildContext, items []MenuItem, anchor geom.Rect, side layout.PopupSide,
	bar *menuBarState, previous core.FocusTarget, keyboard bool,
) *menuSession {
	s := &menuSession{overlay: Of(ctx), bar: bar, previous: previous}
	s.push(items, "", s.toOverlay(anchor), side, keyboard)
	s.entry = s.overlay.InsertFrom(ctx, func() core.Widget { return menuLayer{session: s} })
	return s
}

// toOverlay turns a rectangle in the window into one in the overlay.
func (s *menuSession) toOverlay(r geom.Rect) geom.Rect {
	if ro := s.overlay.Context().RenderObject(); ro != nil {
		o := core.LocalToGlobal(ro, geom.Offset{})
		r.X, r.Y = r.X-o.X, r.Y-o.Y
	}
	return r
}

// push opens a panel of items on top of the others.
func (s *menuSession) push(items []MenuItem, title string, anchor geom.Rect, side layout.PopupSide, keyboard bool) {
	p := &menuPanel{
		items: items, title: title, anchor: anchor, side: side, hot: -1, child: -1, focusRow: -1,
		labels: make([]kit.Mnemonic, len(items)), rows: make([]*layout.RenderFocus, len(items)),
	}
	for i := range items {
		p.labels[i] = kit.ParseMnemonic(items[i].Action.Label)
	}
	p.width = s.measure(p)
	if keyboard {
		p.focusRow = p.next(-1, 1)
	}
	p.focusBox = p.focusRow < 0
	s.panels = append(s.panels, p)
}

// measure returns the width of panel p: its widest label and shortcut, within
// the bounds of a menu. layout cannot ask the rows for a natural width.
func (s *menuSession) measure(p *menuPanel) float32 {
	ctx := s.overlay.Context()
	shaper, ok := core.Lookup[*text.Shaper](ctx)
	if !ok {
		return menuMinWidth
	}
	th := basic.ThemeOf(ctx)
	var label, key float32
	arrow := false
	for i := range p.items {
		it := &p.items[i]
		if it.Separator {
			continue
		}
		label = max(label, kit.TextWidth(shaper, p.labels[i].Text, th.Text.Body))
		if sc := it.Action.Shortcut.String(); sc != "" {
			key = max(key, kit.TextWidth(shaper, sc, th.Text.Body))
		}
		arrow = arrow || len(it.Items) > 0
	}
	w := 2*kit.MenuInset + 2*menuItemPadding + menuCheckWidth + label
	if key > 0 {
		w += menuShortcutGap + key
	}
	if arrow {
		w += menuArrowWidth
	}
	return geom.Clamp32(geom.Ceil32(w), menuMinWidth, menuMaxWidth)
}

// next returns the selectable item after i in direction dir, wrapping around,
// or -1 if there is none.
func (p *menuPanel) next(i, dir int) int {
	n := len(p.items)
	for k := 1; k <= n; k++ {
		j := ((i+dir*k)%n + n) % n
		if !p.items[j].Separator {
			return j
		}
	}
	return -1
}

// changed rebuilds the menus and the bar they belong to.
func (s *menuSession) changed() {
	if s.layer != nil && s.layer.Mounted() {
		kit.Rebuild(&s.layer.StateBase)
	}
	if s.bar != nil && s.bar.Mounted() {
		kit.Rebuild(&s.bar.StateBase)
	}
}

// close ends the session. restore gives the focus back to what had it before.
func (s *menuSession) close(restore bool) {
	if s.closed {
		return
	}
	s.closed = true
	s.entry.Remove()
	if restore && s.previous != nil {
		core.RestoreFocus(s.previous)
	}
	if s.bar != nil {
		s.bar.menuClosed(s, restore)
	}
}

// replace swaps the menus for a new first one, as moving along a menu bar does.
func (s *menuSession) replace(items []MenuItem, anchor geom.Rect, keyboard bool) {
	s.panels = s.panels[:0]
	s.push(items, "", s.toOverlay(anchor), layout.PopupBelow, keyboard)
	s.changed()
}

// truncate closes the panels from index p on.
func (s *menuSession) truncate(p int) {
	if p >= len(s.panels) {
		return
	}
	clear(s.panels[p:])
	s.panels = s.panels[:p]
	if p > 0 {
		s.panels[p-1].child = -1
	}
	s.changed()
}

// focusRow gives the keyboard to row i of panel p.
func (s *menuSession) focusRow(p, i int) {
	// The rows stay until the next build after the session ends, and the
	// pointer may still come over one.
	if s.closed || p >= len(s.panels) {
		return
	}
	if r := s.panels[p].rows[i]; r != nil && r.Attached() {
		r.RequestFocus()
	}
}

// openSubmenu opens the submenu of item i of panel p.
func (s *menuSession) openSubmenu(p, i int, keyboard bool) {
	if s.closed || p >= len(s.panels) {
		return
	}
	panel := s.panels[p]
	if panel.child == i && p+1 < len(s.panels) {
		if keyboard {
			if j := s.panels[p+1].next(-1, 1); j >= 0 {
				s.focusRow(p+1, j)
			}
		}
		return
	}
	s.truncate(p + 1)
	row := panel.rows[i]
	if row == nil || !row.Attached() {
		return
	}
	s.push(panel.items[i].Items, panel.labels[i].Text, s.toOverlay(kit.GlobalRect(row)), layout.PopupBeside, keyboard)
	child := s.panels[len(s.panels)-1]
	// A submenu opened by the pointer leaves the keyboard where it is.
	child.focusBox = false
	panel.child = i
	s.changed()
}

// activate uses item i of panel p: opens its submenu, or closes the menus and
// runs its action.
func (s *menuSession) activate(p, i int, keyboard bool) {
	if s.closed || p >= len(s.panels) {
		return
	}
	it := s.panels[p].items[i]
	switch {
	case it.Separator || !it.enabled():
	case len(it.Items) > 0:
		s.openSubmenu(p, i, keyboard)
	default:
		// The action runs after the focus is back, so that a Copy finds the
		// field it copies from.
		s.close(true)
		it.Action.OnInvoke()
	}
}

// hover follows the pointer onto item i of panel p.
func (s *menuSession) hover(p, i int) {
	if s.closed || p >= len(s.panels) {
		return
	}
	panel := s.panels[p]
	if panel.child != i {
		s.truncate(p + 1)
	}
	s.focusRow(p, i)
	if len(panel.items[i].Items) > 0 {
		s.openSubmenu(p, i, false)
	}
}

// back closes panel p and puts the focus on the item that opened it.
func (s *menuSession) back(p int) {
	parent := s.panels[p-1].child
	s.truncate(p)
	if parent >= 0 {
		s.focusRow(p-1, parent)
	}
}

// key handles a key pressed while panel p or one of its rows has the focus.
// Every key stops here, as in a native menu.
func (s *menuSession) key(p int, e *core.KeyEvent) bool {
	if s.closed || p >= len(s.panels) {
		return false
	}
	panel := s.panels[p]
	plain := e.Modifiers&^input.ModShift == 0
	switch {
	case e.Key == input.KeyDown || e.Key == input.KeyUp:
		dir := 1
		if e.Key == input.KeyUp {
			dir = -1
		}
		if j := panel.next(panel.hot, dir); j >= 0 {
			s.focusRow(p, j)
		}
	case e.Key == input.KeyHome:
		if j := panel.next(-1, 1); j >= 0 {
			s.focusRow(p, j)
		}
	case e.Key == input.KeyEnd:
		if j := panel.next(0, -1); j >= 0 {
			s.focusRow(p, j)
		}
	case e.Key == input.KeyEnter || e.Key == input.KeySpace:
		if panel.hot >= 0 && !e.Repeat {
			s.activate(p, panel.hot, true)
		}
	case e.Key == input.KeyRight:
		if panel.hot >= 0 && len(panel.items[panel.hot].Items) > 0 && panel.items[panel.hot].enabled() {
			s.openSubmenu(p, panel.hot, true)
		} else if s.bar != nil {
			s.bar.step(1)
		}
	case e.Key == input.KeyLeft:
		if p > 0 {
			s.back(p)
		} else if s.bar != nil {
			s.bar.step(-1)
		}
	case e.Key == input.KeyEscape:
		switch {
		case p > 0:
			s.back(p)
		case s.bar != nil:
			s.bar.escape()
		default:
			s.close(true)
		}
	case e.Key == input.KeyAlt || e.Key == input.KeyF10 && e.Modifiers == 0:
		s.close(true)
	case plain || e.Modifiers == input.ModAlt:
		s.pickMnemonic(p, e.Key)
	}
	return true
}

// pickMnemonic uses the item of panel p whose mnemonic is key. Without one, it
// moves to the next item whose label starts with the letter.
func (s *menuSession) pickMnemonic(p int, key input.KeyCode) {
	panel := s.panels[p]
	if key == input.KeyNone {
		return
	}
	for i := range panel.items {
		if panel.labels[i].Key == key && !panel.items[i].Separator {
			s.focusRow(p, i)
			s.activate(p, i, true)
			return
		}
	}
	for k := 1; k <= len(panel.items); k++ {
		i := (panel.hot + k + len(panel.items)) % len(panel.items)
		t := panel.labels[i].Text
		if !panel.items[i].Separator && t != "" && kit.LetterKey(t[0]) == key {
			s.focusRow(p, i)
			return
		}
	}
}

// rowFocus follows the focus of row i of panel p.
func (s *menuSession) rowFocus(p, i int, focused bool) {
	if p >= len(s.panels) {
		return
	}
	panel := s.panels[p]
	switch {
	case focused && panel.hot != i:
		panel.hot = i
		s.changed()
	case !focused && panel.hot == i:
		panel.hot = -1
		s.changed()
	}
	if !focused {
		s.focusLeft(panel.rows[i])
	}
}

// focusLeft closes the menus when the focus that ro lost went outside them, or
// the window lost the keyboard: ro still has the focus then.
func (s *menuSession) focusLeft(ro core.RenderObject) {
	if ro == nil || s.closed {
		return
	}
	now := kit.FocusFrom(ro)
	switch {
	case now == ro:
		s.close(true)
	case !s.contains(now):
		s.close(false)
	}
}

// contains reports whether ro is part of the open menus.
func (s *menuSession) contains(ro core.RenderObject) bool {
	if ro == nil || s.layer == nil {
		return false
	}
	root := s.layer.Context().RenderObject()
	for ; ro != nil; ro = ro.Parent() {
		if ro == root {
			return true
		}
	}
	return false
}

// menuLayer shows the menus of a session over the window: a barrier that ends
// them on a press beside them, the panels, and over a menu bar, a stand-in for
// each title that moves to its menu as the pointer goes over it.
type menuLayer struct {
	core.StatefulBase
	session *menuSession
}

type menuLayerState struct {
	core.StateBase
	session *menuSession
}

// CreateState implements core.StatefulWidget.
func (m menuLayer) CreateState() core.State { return &menuLayerState{session: m.session} }

// InitState implements core.State.
func (s *menuLayerState) InitState() { s.session.layer = s }

// Build implements core.State.
func (s *menuLayerState) Build(ctx core.BuildContext) core.Widget {
	ms := s.session
	th := basic.ThemeOf(ctx)
	children := make([]core.Widget, 0, len(ms.panels)+8)
	children = append(children, kit.Barrier{OnPress: func() { ms.close(true) }})
	if ms.bar != nil {
		children = ms.bar.standIns(ms, children)
	}
	for p := range ms.panels {
		panel := ms.panels[p]
		var shift float32
		if panel.side == layout.PopupBeside {
			shift = kit.MenuInset
		}
		children = append(children, core.Keyed(panel, kit.Popup{
			Anchor: panel.anchor, Side: panel.side, Shift: shift,
			Child: ms.panelWidget(th, p),
		}))
	}
	return kit.ModalScope{Active: true, Child: basic.Stack{Fit: layout.StackExpand, Children: children}}
}

// panelWidget builds panel p.
func (s *menuSession) panelWidget(th *theme.Theme, p int) core.Widget {
	panel := s.panels[p]
	rows := make([]core.Widget, len(panel.items))
	for i := range panel.items {
		if panel.items[i].Separator {
			rows[i] = basic.Padding{
				Insets: geom.Insets{Top: menuSeparatorRoom, Bottom: menuSeparatorRoom - 1, Left: menuItemPadding, Right: menuItemPadding},
				Child:  interact.Semantics{Role: semantics.RoleSeparator, Child: basic.Container{Height: 1, Color: th.Color.Hair}},
			}
			continue
		}
		rows[i] = s.rowWidget(th, p, i)
	}
	card := basic.Container{
		Width:   panel.width,
		Color:   th.Color.Raised,
		Radius:  th.Radius.Control,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.All(kit.MenuInset),
		Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: rows},
	}
	return interact.Semantics{Role: semantics.RoleMenu, Label: panel.title, Child: focusRef{
		ref:       &panel.box,
		enabled:   true,
		autofocus: panel.focusBox,
		onKey:     func(e *core.KeyEvent) bool { return s.key(p, e) },
		onFocusChange: func(focused bool) {
			if !focused {
				s.focusLeft(panel.box)
			}
		},
		child: card,
	}}
}

// rowWidget builds item i of panel p.
func (s *menuSession) rowWidget(th *theme.Theme, p, i int) core.Widget {
	panel := s.panels[p]
	it := &panel.items[i]
	enabled := it.enabled()
	lit := panel.hot == i || panel.child == i
	ink, ink2 := th.Color.Ink, th.Color.Ink2
	if !enabled {
		ink, ink2 = th.Color.Ink3, th.Color.Ink3
	}
	var ground geom.Color
	switch {
	case lit && enabled:
		// The shortcut in Ink2 falls below 4.5:1 on the tint.
		ground, ink2 = th.Color.AccentTint, th.Color.OnTint().Ink2
	case lit:
		ground = th.Color.Fill
	}
	var check core.Widget = basic.SizedBox{Width: menuCheckWidth}
	if it.Action.Checkable && it.Action.Checked {
		check = basic.SizedBox{Width: menuCheckWidth, Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.SizedBox{
			Width: menuCheckSize, Height: menuCheckSize, Child: kit.Checkmark{Color: ink, Progress: 1},
		}}}
	}
	parts := []core.Widget{check, basic.Expanded{Child: basic.Text{Content: panel.labels[i].Text, Style: kit.Colored(th.Text.Body, ink)}}}
	shortcut := it.Action.Shortcut.String()
	if shortcut != "" && len(it.Items) == 0 {
		parts = append(parts, basic.Padding{
			Insets: geom.Insets{Left: menuShortcutGap},
			Child:  basic.Text{Content: shortcut, Style: kit.Colored(th.Text.Body, ink2)},
		})
	}
	if len(it.Items) > 0 {
		parts = append(parts, basic.SizedBox{Width: menuArrowWidth, Child: basic.Align{Alignment: layout.AlignCenterRight, Child: basic.Text{
			Content: "›", Style: kit.Colored(th.Text.Body, ink2),
		}}})
	}
	sem := interact.Semantics{Role: semantics.RoleMenuItem, Label: panel.labels[i].Text, Shortcut: shortcut}
	switch {
	case !enabled:
		sem.State |= semantics.Disabled
	case len(it.Items) > 0:
		sem.OnExpand = func() { s.openSubmenu(p, i, true) }
		sem.OnCollapse = func() { s.truncate(p + 1) }
		if panel.child == i {
			sem.State |= semantics.Expanded
		} else {
			sem.State |= semantics.Collapsed
		}
	default:
		sem.OnTap = func() { s.activate(p, i, true) }
	}
	if it.Action.Checkable {
		sem.Role = semantics.RoleMenuItemCheckbox
		if it.Action.Checked {
			sem.State |= semantics.Checked
		}
	}
	sem.Child = focusRef{
		ref:           &panel.rows[i],
		enabled:       true,
		autofocus:     panel.focusRow == i,
		onFocusChange: func(focused bool) { s.rowFocus(p, i, focused) },
		child: interact.MouseRegion{
			OnEnter: func() { s.hover(p, i) },
			Child: interact.GestureDetector{
				OnTapDown: func() {},
				OnTap:     func() { s.activate(p, i, false) },
				Child: basic.Container{
					Height:  kit.MenuItemHeight,
					Color:   ground,
					Radius:  th.Radius.Check,
					Padding: geom.Insets{Left: menuItemPadding, Right: menuItemPadding},
					Child:   basic.Row{Cross: layout.CrossCenter, Children: parts},
				},
			},
		},
	}
	return sem
}

// focusRef is Focus that hands its render object to the state that builds it,
// which needs it to move the keyboard.
type focusRef struct {
	ref           **layout.RenderFocus
	enabled       bool
	autofocus     bool
	onKey         func(e *core.KeyEvent) bool
	onFocusChange func(focused bool)
	child         core.Widget
}

// Key implements core.Widget.
func (focusRef) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f focusRef) CreateRenderObject() core.RenderObject {
	r := &layout.RenderFocus{Autofocus: f.autofocus}
	f.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f focusRef) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFocus)
	r.Enabled, r.OnKey, r.OnFocusChange = f.enabled, f.onKey, f.onFocusChange
	*f.ref = r
}

// ChildWidget implements core.SingleChildWidget.
func (f focusRef) ChildWidget() core.Widget { return f.child }
