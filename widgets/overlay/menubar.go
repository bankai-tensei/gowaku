package overlay

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	menuBarHeight      = theme.ControlHeight
	menuTitleHeight    = 26
	menuTitlePadding   = theme.Space10
	menuBarInset       = theme.Space4
	menuBarTitlesSpace = theme.Space2
)

// MenuBar is the row of menus of a window. It is an ordinary widget: put it at
// the top of a Column, or in a title bar. Each item is a menu titled by its
// Action.Label; an item without Items runs its action from the bar.
//
// The pointer opens a menu with a press, and moves between menus while one is
// open. From the keyboard, Alt alone or F10 moves the focus to the bar, and
// Alt with the mnemonic of a title opens its menu; arrows, Enter and Escape
// move through the menus as in Windows. The shortcuts of the items work from
// anywhere in the window, without an Actions. On macOS the bar becomes the menu
// of the system (docs/ADR/0083-macos-integration.md). See
// docs/ADR/0046-menus-and-shortcuts.md.
type MenuBar struct {
	core.StatefulBase
	Items []MenuItem
}

type menuBarState struct {
	core.StateBase
	titles []*layout.RenderFocus
	labels []kit.Mnemonic
	// active means the keyboard is on the bar: the titles take the focus, and
	// previous gets it back when the bar lets go.
	active   bool
	previous core.FocusTarget
	// focused and hovered are titles, or -1; open is the title whose menu is
	// open, or -1.
	focused, hovered, open int
	session                *menuSession
	// native is the host that shows the menus, if the window system does.
	native core.NativeMenuHost
}

// CreateState implements core.StatefulWidget.
func (MenuBar) CreateState() core.State {
	return &menuBarState{focused: -1, hovered: -1, open: -1}
}

// Dispose implements core.State.
func (s *menuBarState) Dispose() {
	if s.session != nil {
		s.session.close(false)
	}
	if s.native != nil {
		s.native.SetNativeMenu(nil)
	}
}

// nativeMenu converts items for the menus of the window system.
func nativeMenu(items []MenuItem) []core.NativeMenuItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]core.NativeMenuItem, len(items))
	for i := range items {
		it := &items[i]
		a := &it.Action
		out[i] = core.NativeMenuItem{
			Label: a.Label, Key: a.Shortcut.Key, Modifiers: a.Shortcut.Modifiers,
			Enabled: it.enabled(), Checkable: a.Checkable, Checked: a.Checked,
			Separator: it.Separator, Items: nativeMenu(it.Items), OnInvoke: a.OnInvoke,
		}
	}
	return out
}

func (s *menuBarState) items() []MenuItem { return s.Widget().(MenuBar).Items }

// Build implements core.State. Where the window system shows the menus of
// the application, as macOS does, the bar hands them over and takes no room;
// its shortcuts still work.
func (s *menuBarState) Build(ctx core.BuildContext) core.Widget {
	items := s.items()
	if host, ok := ctx.Host().(core.NativeMenuHost); ok && host.SetNativeMenu(nativeMenu(items)) {
		s.native = host
		return shortcuts{onShortcut: s.shortcut, child: basic.SizedBox{}}
	}
	th := basic.ThemeOf(ctx)
	if len(s.titles) != len(items) {
		s.titles = make([]*layout.RenderFocus, len(items))
	}
	s.labels = s.labels[:0]
	titles := make([]core.Widget, len(items))
	for i := range items {
		s.labels = append(s.labels, kit.ParseMnemonic(items[i].Action.Label))
		titles[i] = s.title(th, i)
	}
	return interact.Semantics{Role: semantics.RoleMenuBar, Child: shortcuts{
		onShortcut: s.shortcut,
		child: interact.Focus{
			// The bar itself never takes the focus; it hears the keys of its
			// titles.
			OnKey: s.key,
			Child: basic.Container{
				Height:  menuBarHeight,
				Padding: geom.Insets{Left: menuBarInset, Right: menuBarInset},
				Child:   basic.Row{Cross: layout.CrossCenter, Spacing: menuBarTitlesSpace, Children: titles},
			},
		},
	}}
}

// title builds the title of menu i.
func (s *menuBarState) title(th *theme.Theme, i int) core.Widget {
	it := &s.items()[i]
	ink := th.Color.Ink
	if !it.enabled() {
		ink = th.Color.Ink3
	}
	var ground geom.Color
	switch {
	case s.open == i, s.active && s.focused == i:
		ground = th.Color.FillStrong
	case s.hovered == i && it.enabled():
		ground = th.Color.Fill
	}
	sem := interact.Semantics{Role: semantics.RoleMenuItem, Label: s.labels[i].Text, Shortcut: altShortcut(s.labels[i].Key)}
	switch {
	case !it.enabled():
		sem.State |= semantics.Disabled
	case len(it.Items) > 0:
		sem.OnExpand = func() { s.openMenu(i, true) }
		sem.OnCollapse = func() { s.closeMenu() }
		if s.open == i {
			sem.State |= semantics.Expanded
		} else {
			sem.State |= semantics.Collapsed
		}
	default:
		sem.OnTap = func() { s.openMenu(i, true) }
	}
	sem.Child = focusRef{
		ref:           &s.titles[i],
		enabled:       s.active,
		onFocusChange: func(focused bool) { s.titleFocus(i, focused) },
		child: interact.MouseRegion{
			OnEnter: func() { s.setHovered(i) },
			OnExit: func() {
				if s.hovered == i {
					s.setHovered(-1)
				}
			},
			Child: interact.GestureDetector{
				OnTapDown: func() { s.press(i) },
				Child: basic.Container{
					Height:  menuTitleHeight,
					Color:   ground,
					Radius:  th.Radius.Check,
					Padding: geom.Insets{Left: menuTitlePadding, Right: menuTitlePadding},
					Child:   basic.Center{Child: basic.Text{Content: s.labels[i].Text, Style: kit.Colored(th.Text.Body, ink)}},
				},
			},
		},
	}
	return sem
}

// altShortcut is how a reader names the key that opens a menu.
func altShortcut(key input.KeyCode) string {
	if key == input.KeyNone {
		return ""
	}
	return interact.Shortcut{Key: key, Modifiers: input.ModAlt}.String()
}

func (s *menuBarState) setHovered(i int) {
	if s.hovered != i {
		s.hovered = i
		kit.Rebuild(&s.StateBase)
	}
}

// shortcut handles what reaches the bar from anywhere in the window: Alt and
// F10, Alt with a mnemonic, and the shortcuts of the items.
func (s *menuBarState) shortcut(e *core.KeyEvent) bool {
	switch {
	case e.Key == input.KeyAlt && e.Modifiers == 0, e.Key == input.KeyF10 && e.Modifiers == 0:
		if s.active || s.session != nil {
			s.leave()
		} else {
			s.enter(0)
		}
		return true
	case e.Modifiers == input.ModAlt:
		if i := s.mnemonicTitle(e.Key); i >= 0 {
			if !s.active {
				s.enter(i)
			}
			s.openMenu(i, true)
			return true
		}
	}
	return menuShortcut(s.items(), e)
}

// key handles the keys of a focused title.
func (s *menuBarState) key(e *core.KeyEvent) bool {
	if !s.active || s.focused < 0 {
		return false
	}
	switch {
	case e.Key == input.KeyLeft:
		s.step(-1)
	case e.Key == input.KeyRight:
		s.step(1)
	case e.Key == input.KeyDown, e.Key == input.KeyUp, e.Key == input.KeyEnter, e.Key == input.KeySpace:
		if !e.Repeat {
			s.openMenu(s.focused, true)
		}
	case e.Key == input.KeyEscape, e.Key == input.KeyAlt, e.Key == input.KeyF10 && e.Modifiers == 0:
		s.leave()
	case e.Key == input.KeyTab:
	case e.Modifiers&^input.ModAlt == 0:
		i := s.mnemonicTitle(e.Key)
		if i < 0 {
			return false
		}
		s.openMenu(i, true)
	default:
		return false
	}
	return true
}

// mnemonicTitle returns the enabled title whose mnemonic is key, or -1.
func (s *menuBarState) mnemonicTitle(key input.KeyCode) int {
	if key == input.KeyNone {
		return -1
	}
	items := s.items()
	for i := range s.labels {
		if s.labels[i].Key == key && i < len(items) && items[i].enabled() {
			return i
		}
	}
	return -1
}

// enter moves the keyboard to title i.
func (s *menuBarState) enter(i int) {
	if len(s.titles) == 0 {
		return
	}
	if !s.active {
		s.previous = kit.FocusOf(s.Context())
		s.active = true
		for _, t := range s.titles {
			if t != nil {
				// The focus must be taken now, not after the rebuild.
				t.Enabled = true
			}
		}
	}
	s.focusTitle(i)
	kit.Rebuild(&s.StateBase)
}

func (s *menuBarState) focusTitle(i int) {
	if t := s.titles[i]; t != nil && t.Attached() {
		t.RequestFocus()
	}
}

// leave closes the menus and gives the keyboard back.
func (s *menuBarState) leave() {
	if s.session != nil {
		s.session.close(true)
		return
	}
	s.deactivate(true)
}

// deactivate takes the keyboard off the bar. restore gives it back to what had
// it before.
func (s *menuBarState) deactivate(restore bool) {
	if !s.active {
		return
	}
	s.active = false
	for _, t := range s.titles {
		if t != nil {
			t.Enabled = false
		}
	}
	if restore && s.previous != nil {
		core.RestoreFocus(s.previous)
	}
	s.previous = nil
	kit.Rebuild(&s.StateBase)
}

// press handles a press on title i: it opens the menu, or closes it if it is
// open.
func (s *menuBarState) press(i int) {
	if s.open == i {
		s.closeMenu()
		return
	}
	s.openMenu(i, false)
}

// closeMenu closes the open menu, giving the keyboard back.
func (s *menuBarState) closeMenu() {
	if s.session != nil {
		s.session.close(true)
	}
}

// openMenu opens the menu of title i, or runs its action if it has no menu.
func (s *menuBarState) openMenu(i int, keyboard bool) {
	it := s.items()[i]
	if !it.enabled() || s.titles[i] == nil || !s.titles[i].Attached() {
		return
	}
	if len(it.Items) == 0 {
		s.leave()
		it.Action.OnInvoke()
		return
	}
	anchor := kit.GlobalRect(s.titles[i])
	s.open = i
	if s.active {
		s.focused = i
	}
	if s.session != nil {
		s.session.replace(it.Items, anchor, keyboard)
		return
	}
	previous := s.previous
	if !s.active {
		previous = kit.FocusOf(s.Context())
	}
	s.session = openMenu(s.Context(), it.Items, anchor, layout.PopupBelow, s, previous, keyboard)
	kit.Rebuild(&s.StateBase)
}

// step moves along the bar from the focused or open title, opening the next
// menu if one is open.
func (s *menuBarState) step(dir int) {
	n := len(s.items())
	from := s.focused
	if s.open >= 0 {
		from = s.open
	}
	if n == 0 || from < 0 {
		return
	}
	i := from
	for range n {
		i = ((i+dir)%n + n) % n
		if s.items()[i].enabled() {
			break
		}
	}
	if s.session != nil {
		s.openMenu(i, true)
		return
	}
	s.focusTitle(i)
}

// escape closes the open menu and leaves the keyboard on its title.
// A menu opened by the pointer just closes.
func (s *menuBarState) escape() {
	i := s.open
	if !s.active {
		s.closeMenu()
		return
	}
	if s.session != nil {
		s.session.close(false)
	}
	if i >= 0 {
		s.focusTitle(i)
	}
}

// menuClosed hears that the session of the bar ended.
func (s *menuBarState) menuClosed(ms *menuSession, restore bool) {
	if s.session != ms {
		return
	}
	s.session = nil
	s.open = -1
	if restore {
		// The session gave the focus back already.
		s.previous = nil
		s.deactivate(false)
	}
	kit.Rebuild(&s.StateBase)
}

// titleFocus follows the focus of title i.
func (s *menuBarState) titleFocus(i int, focused bool) {
	if focused {
		s.focused = i
		kit.Rebuild(&s.StateBase)
		return
	}
	if s.focused == i {
		s.focused = -1
		kit.Rebuild(&s.StateBase)
	}
	if !s.active {
		return
	}
	now := kit.FocusFrom(s.titles[i])
	switch {
	case now == s.titles[i]:
		// The window lost the keyboard.
		s.leave()
	case s.isTitle(now), s.session != nil && s.session.contains(now):
	default:
		s.deactivate(false)
	}
}

func (s *menuBarState) isTitle(ro core.RenderObject) bool {
	for _, t := range s.titles {
		if t != nil && core.RenderObject(t) == ro {
			return true
		}
	}
	return false
}

// standIns adds to children, for a session open from the bar, a stand-in over
// each title that opens its menu when the pointer comes over it, and closes
// the menus on a press of the open one.
func (s *menuBarState) standIns(ms *menuSession, children []core.Widget) []core.Widget {
	for i, t := range s.titles {
		if t == nil || !t.Attached() {
			continue
		}
		r := ms.toOverlay(kit.GlobalRect(t))
		children = append(children, basic.Positioned{
			Left: r.X, Top: r.Y, Width: r.W, Height: r.H,
			Child: interact.MouseRegion{
				OnEnter: func() {
					if !ms.closed && s.open != i && len(s.items()[i].Items) > 0 && s.items()[i].enabled() {
						s.openMenu(i, s.active)
					}
				},
				Child: interact.GestureDetector{OnTapDown: func() {
					if !ms.closed {
						s.press(i)
					}
				}},
			},
		})
	}
	return children
}

// shortcuts is the widget of layout.RenderShortcuts.
type shortcuts struct {
	onShortcut func(e *core.KeyEvent) bool
	child      core.Widget
}

// Key implements core.Widget.
func (shortcuts) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (k shortcuts) CreateRenderObject() core.RenderObject {
	return &layout.RenderShortcuts{OnShortcut: k.onShortcut}
}

// UpdateRenderObject implements core.RenderWidget.
func (k shortcuts) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderShortcuts).OnShortcut = k.onShortcut
}

// ChildWidget implements core.SingleChildWidget.
func (k shortcuts) ChildWidget() core.Widget { return k.child }
