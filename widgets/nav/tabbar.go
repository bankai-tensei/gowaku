package nav

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a TabBar.
const (
	// tabBarHeight is that of the strip, and tabTop the room above the tabs,
	// which stand on its bottom edge.
	tabBarHeight = 34
	tabTop       = tabBarHeight - theme.ControlHeight
	// tabMinWidth and tabMaxWidth bound the width of a tab, which is the
	// same for all of them: they share the strip up to the maximum, and
	// scroll once they reach the minimum.
	tabMinWidth = 120
	tabMaxWidth = 208
	// tabInset is the room before the icon or title, and tabEndInset that
	// after the close button.
	tabInset    = theme.Space12
	tabEndInset = theme.Space6
	tabIconSize = 16
	tabIconGap  = theme.Space8
	// tabCloseSize is the square of the close button, and tabCloseGap the room
	// between it and the title.
	tabCloseSize = 20
	tabCloseGap  = theme.Space4
	// tabDotSize is the dot of a modified tab, in the place of the close
	// button.
	tabDotSize = 8
	// tabSeparator is the height of the line between two tabs.
	tabSeparator = 16
	// tabButtonSize is the square of the new tab and all tabs buttons, and
	// tabButtonGap the room around them.
	tabButtonSize = 28
	tabButtonGap  = theme.Space4
	// tabDragSlop is how far the pointer moves before a press becomes a drag.
	tabDragSlop = 4
	// tabTearDistance is how far from the strip the pointer goes before the
	// tab comes off it.
	tabTearDistance = 32
	// tabScrollStep is how far one notch of the wheel scrolls the strip.
	tabScrollStep = 48
	// tabGhostOpacity is that of a tab from another bar that would drop here.
	tabGhostOpacity = 0.75
)

// TabItem is one tab of a TabBar.
type TabItem struct {
	// Key tells the tab apart from the others, across moves and between the
	// bars of a TabGroup, such as the ID of the document it shows. It must be
	// comparable and unique within the group. Without it the tabs are told
	// apart by their places, and do not slide when they move.
	Key core.Key
	// Label is the title the tab shows, which also names it to assistive
	// technology.
	Label string
	// Icon is drawn before the title, on the 24 unit grid of Icon.
	Icon *paint.Path
	// Permanent keeps the tab from closing: it has no close button, and
	// neither the middle button nor Ctrl+W close it.
	Permanent bool
	// Italic slants the title, as editors show a preview tab that the next
	// document opened takes over. It changes only the look: the tab measures
	// as it would upright, and assistive technology reads the title as it is.
	Italic bool
	// Modified marks a document with changes that are not saved, as editors
	// do: a dot takes the place of the close button, and turns back into it
	// while the pointer is over the tab. Assistive technology reads
	// Strings.Modified as the value of the tab. A Permanent tab shows the dot
	// too, with nothing behind it.
	Modified bool
}

// TabBar is a strip of tabs that the user selects, closes, reorders and drags
// out, for documents in one window, as a browser has. It is only the strip:
// the caller shows what the selected tab holds below it, and owns the list,
// which the callbacks ask it to change.
//
// The selected tab is a raised surface, the color of a Card, that joins the
// content below it; the others have no ground until the pointer is over them.
// Tabs share the width up to 208 each, and below 96 the strip scrolls with the
// wheel, and a button at its end lists all of them. A close button shows on
// the selected tab and the one under the pointer; a middle click closes too.
// A Modified tab shows a dot there instead until the pointer is over it.
//
// A tab dragged along the strip moves, and the others slide out of its way.
// Dragged off the strip, it comes away: dropped on another TabBar of the same
// Group, in this window or another, it moves there; dropped anywhere else, it
// asks OnTearOff for a window of its own. gowaku.TearOffWindow opens one.
//
// The strip is one stop of Tab. The left and right arrow keys select the
// previous and next tab, Home and End the first and last, and Ctrl+Shift with
// an arrow moves the selected tab. From anywhere in the window, Ctrl+Tab and
// Ctrl+Shift+Tab, or Ctrl+PageDown and Ctrl+PageUp, select the next and
// previous tab, Ctrl+Shift+PageDown and Ctrl+Shift+PageUp move it, and Ctrl+W
// or Ctrl+F4 close it. Without OnSelect the bar is disabled.
type TabBar struct {
	core.StatefulBase
	Tabs []TabItem
	// Selected is the index of the selected tab. Out of range selects none.
	Selected int
	// Label names the strip to assistive technology.
	Label    string
	OnSelect func(i int)
	// OnClose asks to close tab i, from its close button, the middle button
	// or Ctrl+W and Ctrl+F4; without it the tabs have no close button, and
	// neither have those that are Permanent.
	// The bar takes nothing out itself: the tab goes when Tabs no longer has
	// it. OnClose may first ask, as whether to save, and take the tab out
	// later or not at all. The bar does not change the selection either:
	// select the tab that should follow, as the neighbor on the right.
	OnClose func(i int)
	// OnMove asks to move the tab at from to the index to. OnSelect follows
	// with the new index of the selected tab, if it changed.
	OnMove func(from, to int)
	// OnNew adds a button after the tabs that asks for a new tab.
	OnNew func()

	// Group joins bars between which tabs move by dragging. Share one
	// *TabGroup between them.
	Group *TabGroup
	// OnReceive asks to insert the tab with key at index: a tab of another
	// bar of Group was dropped here. Without it the bar takes no tabs.
	OnReceive func(key core.Key, index int)
	// OnDetach asks to take out tab i, without closing what it shows, because
	// it was dropped on another bar of Group, whose OnReceive has run.
	OnDetach func(i int)
	// OnTearOff asks for a window for a tab dropped away from every bar of
	// Group. Take the tab out of this bar, and open the window with
	// gowaku.TearOffWindow, or, when the tab was alone, move this window.
	// Without it a tab cannot leave the bar but for another bar.
	OnTearOff func(e TabTearOff)
}

// TabTearOff is a tab dropped away from every TabBar of its group.
type TabTearOff struct {
	// Index and Key are those of the tab in the bar it left.
	Index int
	Key   core.Key
	// Screen is where the pointer let go, in physical pixels of the screen.
	// Origin is where a window should have the top-left corner of its client
	// area for the tab to come out under the pointer, if the new window has
	// its TabBar where this one is. Placed is false where the window system
	// does not tell where windows are, as on Wayland; the new window then
	// goes where the system puts it.
	Screen, Origin geom.Offset
	Placed         bool
	// Alone means the tab was the only one of its bar, whose window can then
	// move to Origin instead.
	Alone bool
}

// tabIndexKey tells apart tabs without a Key, by their places.
type tabIndexKey int

type tabBarState struct {
	core.StateOf[TabBar]
	// focused and shown are those of the strip; the ring goes around the
	// selected tab.
	focused, shown bool
	// hovered is the tab under the pointer, or -1.
	hovered int
	// slides hold where each tab is, in slots, so that tabs slide to new
	// places when they move, close or make room.
	slides map[core.Key]*motion.SpringValue
	// drag is the press or drag of a tab of this bar, and incoming a tab of
	// another bar that would drop here.
	drag     tabDrag
	incoming *tabIncoming
	// group is the one the bar joined, which may lag Widget().Group by a
	// build.
	group *TabGroup

	strip *layout.RenderFocus
	run   *renderTabRun
	row   *renderTabBarRow
	more  *layout.RenderProxy
	// moreFocused means the all tabs button has the keyboard, so that its
	// menu opens on the first item.
	moreFocused bool

	cross, plus, chevron *paint.Path
}

// CreateState implements core.StatefulWidget.
func (TabBar) CreateState() core.State { return &tabBarState{} }

// InitState implements core.State.
func (s *tabBarState) InitState() {
	s.hovered = -1
	s.slides = make(map[core.Key]*motion.SpringValue)
	s.cross = kit.CrossPath(tabCloseSize, theme.Space8, 1.5)
	s.plus = plusPath(kit.DefaultIconSize)
	s.chevron = kit.ChevronGlyph()
}

// Dispose implements core.State.
func (s *tabBarState) Dispose() {
	s.cancelDrag()
	if s.group != nil {
		s.group.leave(s)
	}
	for k, v := range s.slides {
		v.Dispose()
		delete(s.slides, k)
	}
}

// plusPath returns the + of the new tab button, in a square of side box.
func plusPath(box float32) *paint.Path {
	arm := box / 3
	var p paint.Path
	p.MoveTo(box/2, box/2-arm)
	p.LineTo(box/2, box/2+arm)
	p.MoveTo(box/2-arm, box/2)
	p.LineTo(box/2+arm, box/2)
	return p.Stroke(paint.StrokeStyle{Width: 1.75, Cap: paint.CapRound})
}

// keyOf returns the key that tells tab i apart.
func (s *tabBarState) keyOf(i int) core.Key {
	if k := s.Widget().Tabs[i].Key; k != nil {
		return k
	}
	return tabIndexKey(i)
}

// selectTab asks for tab i, unless it is the selected one.
func (s *tabBarState) selectTab(i int) {
	if w := s.Widget(); w.OnSelect != nil && i != w.Selected && i >= 0 && i < len(w.Tabs) {
		w.OnSelect(i)
	}
}

// step selects the tab d places from the selected one, wrapping.
func (s *tabBarState) step(d int) {
	w := s.Widget()
	n := len(w.Tabs)
	if n == 0 {
		return
	}
	from := w.Selected
	if from < 0 || from >= n {
		from = 0
		if d > 0 {
			from = -1
		}
	}
	s.selectTab(((from+d)%n + n) % n)
}

// moveTab moves the tab at from to the index to, and the selection with it.
func (s *tabBarState) moveTab(from, to int) {
	w := s.Widget()
	n := len(w.Tabs)
	if w.OnMove == nil || from == to || from < 0 || from >= n || to < 0 || to >= n {
		return
	}
	sel := movedIndex(w.Selected, from, to)
	w.OnMove(from, to)
	if sel != w.Selected && w.OnSelect != nil {
		w.OnSelect(sel)
	}
}

// movedIndex returns where index i is after the item at from moves to to.
func movedIndex(i, from, to int) int {
	switch {
	case i == from:
		return to
	case from < i && i <= to:
		return i - 1
	case to <= i && i < from:
		return i + 1
	}
	return i
}

// closeTab asks to close tab i.
func (s *tabBarState) closeTab(i int) {
	if w := s.Widget(); w.OnClose != nil && i >= 0 && i < len(w.Tabs) && !w.Tabs[i].Permanent {
		w.OnClose(i)
	}
}

// key is that of the strip.
func (s *tabBarState) key(e *core.KeyEvent) bool {
	w := s.Widget()
	if e.Modifiers&input.ModAlt != 0 {
		return false
	}
	if e.Key == input.KeyEscape && s.drag.phase != dragNone {
		s.cancelDrag()
		return true
	}
	if e.Modifiers == input.ModControl|input.ModShift {
		switch e.Key {
		case input.KeyLeft:
			s.moveTab(w.Selected, w.Selected-1)
		case input.KeyRight:
			s.moveTab(w.Selected, w.Selected+1)
		default:
			return false
		}
		return true
	}
	if e.Modifiers != 0 {
		return false
	}
	switch e.Key {
	case input.KeyLeft:
		s.step(-1)
	case input.KeyRight:
		s.step(1)
	case input.KeyHome:
		s.selectTab(0)
	case input.KeyEnd:
		s.selectTab(len(w.Tabs) - 1)
	default:
		return false
	}
	return true
}

// shortcut takes the keys of the bar from anywhere in the window.
func (s *tabBarState) shortcut(e *core.KeyEvent) bool {
	w := s.Widget()
	if w.OnSelect == nil || len(w.Tabs) == 0 || e.Modifiers&input.ModControl == 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	shift := e.Modifiers&input.ModShift != 0
	switch {
	case e.Key == input.KeyTab && shift, e.Key == input.KeyPageUp && !shift:
		s.step(-1)
	case e.Key == input.KeyTab, e.Key == input.KeyPageDown && !shift:
		s.step(1)
	case e.Key == input.KeyPageUp:
		s.moveTab(w.Selected, w.Selected-1)
	case e.Key == input.KeyPageDown:
		s.moveTab(w.Selected, w.Selected+1)
	case (e.Key == input.KeyW || e.Key == input.KeyF4) && !shift && w.OnClose != nil:
		if !e.Repeat {
			s.closeTab(w.Selected)
		}
	default:
		return false
	}
	return true
}

// focus and showFocus follow the focus of the strip.
func (s *tabBarState) focus(focused bool) {
	if s.focused != focused {
		s.focused = focused
		kit.Rebuild(&s.StateBase)
	}
}

func (s *tabBarState) showFocus(show bool) {
	if s.shown != show {
		s.shown = show
		kit.Rebuild(&s.StateBase)
	}
}

// hover follows the pointer over tab i.
func (s *tabBarState) hover(i int, inside bool) {
	switch {
	case inside && s.hovered != i:
		s.hovered = i
	case !inside && s.hovered == i:
		s.hovered = -1
	default:
		return
	}
	kit.Rebuild(&s.StateBase)
}

// allTabs opens the menu of every tab.
func (s *tabBarState) allTabs() {
	w := s.Widget()
	if s.more == nil || !s.more.Attached() || len(w.Tabs) == 0 {
		return
	}
	items := make([]overlay.MenuItem, len(w.Tabs))
	for i := range w.Tabs {
		items[i] = overlay.MenuItem{Action: interact.Action{
			// A & in a title is not a mnemonic.
			Label:     strings.ReplaceAll(w.Tabs[i].Label, "&", "&&"),
			Checkable: true, Checked: i == w.Selected,
			OnInvoke: func() { s.selectTab(i) },
		}}
	}
	ctx := s.Context()
	overlay.ShowMenuBelow(ctx, s.more, items, s.moreFocused)
}

// slides and targets

// targetSlot returns where tab i goes now, in slots: its index, moved by the
// drag of a tab of this bar and the room made for one of another bar.
func (s *tabBarState) targetSlot(i int) int {
	slot := i
	switch d := &s.drag; d.phase {
	case dragDragging:
		slot = movedIndex(i, d.index, d.target)
	case dragDetached:
		if i > d.index {
			slot = i - 1
		}
	}
	if in := s.incoming; in != nil && slot >= in.index {
		slot++
	}
	return slot
}

// slide returns the spring of the tab with key, made at slot if it is new.
func (s *tabBarState) slide(k core.Key, slot int) *motion.SpringValue {
	v := s.slides[k]
	if v == nil {
		v = &motion.SpringValue{Spring: motion.SnappySpring(), OnChange: func() { kit.Rebuild(&s.StateBase) }}
		v.Attach(&s.StateBase, float32(slot))
		s.slides[k] = v
	}
	return v
}

// retarget sends every tab to its slot, and returns where each is now. Springs
// of tabs that left go.
func (s *tabBarState) retarget(keys []core.Key, ghost core.Key, slots []float32) []float32 {
	reduce := basic.ReduceMotionOf(s.Context())
	for i, k := range keys {
		v := s.slide(k, s.targetSlot(i))
		to := float32(s.targetSlot(i))
		if reduce {
			v.Jump(to)
		} else {
			v.Set(to)
		}
		slots = append(slots, v.Value())
	}
	if in := s.incoming; in != nil {
		v := s.slide(ghost, in.index)
		v.Jump(float32(in.index))
		slots = append(slots, float32(in.index))
	}
	for k, v := range s.slides {
		if k == ghost && s.incoming != nil {
			continue
		}
		if !containsKey(keys, k) {
			v.Dispose()
			delete(s.slides, k)
		}
	}
	return slots
}

func containsKey(keys []core.Key, k core.Key) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

// Build implements core.State.
func (s *tabBarState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	l := basic.StringsOf(ctx)
	s.joinGroup(w.Group)
	n := len(w.Tabs)
	enabled := w.OnSelect != nil
	s.drag.clamp(n)
	if s.hovered >= n {
		s.hovered = -1
	}

	keys := make([]core.Key, n)
	for i := range n {
		keys[i] = s.keyOf(i)
	}
	var ghost core.Key
	if in := s.incoming; in != nil {
		ghost = in.key
		if containsKey(keys, ghost) {
			s.incoming, ghost = nil, nil
		}
	}
	slots := s.retarget(keys, ghost, make([]float32, 0, n+1))

	ring := geom.Color{}
	if s.focused && s.shown {
		ring = c.AccentInk.WithAlpha(kit.RingOpacity)
	}
	tabs := make([]core.Widget, 0, n+1)
	for i := range n {
		it := &w.Tabs[i]
		t := tabView{
			title: it.Label, icon: it.Icon, cross: s.cross, italic: it.Italic, modified: it.Modified,
			selected: i == w.Selected, enabled: enabled, modifiedValue: l.Modified,
			separator: s.separated(i), quiet: s.drag.phase >= dragDragging && i != s.drag.index,
		}
		if t.selected {
			t.ring = ring
		}
		if enabled {
			t.onTap = func() { s.selectTab(i) }
			t.onHover = func(inside bool) { s.hover(i, inside) }
			if w.OnClose != nil && !it.Permanent {
				t.onClose = func() { s.closeTab(i) }
				t.closeLabel = l.CloseNamed(it.Label)
			}
		}
		tabs = append(tabs, core.Keyed(keys[i], t))
	}
	if in := s.incoming; in != nil {
		tabs = append(tabs, core.Keyed(ghost, tabView{
			title: in.item.Label, icon: in.item.Icon, cross: s.cross, italic: in.item.Italic, selected: true, ghost: true,
		}))
	}

	run := tabRun{
		ref: &s.run, slots: slots, count: n, selected: w.Selected,
		hidden: -1, dragged: -1, children: tabs,
	}
	switch s.drag.phase {
	case dragDragging:
		run.dragged, run.dragX = s.drag.index, s.drag.x
	case dragDetached:
		run.hidden = s.drag.index
	}
	if enabled {
		run.onDown, run.onMove, run.onUp, run.onCancel, run.onMiddle = s.down, s.move, s.release, s.cancelDrag, s.closeTab
	}
	list := interact.Semantics{Role: semantics.RoleTabList, Label: w.Label, Child: kit.TabListFocus{
		Ref: &s.strip, Enabled: enabled && n > 0, OnKey: s.key,
		OnFocusChange: s.focus, OnShowFocus: s.showFocus, Child: run,
	}}
	if !enabled {
		list.State = semantics.Disabled
	}

	var add core.Widget = basic.SizedBox{}
	if w.OnNew != nil && enabled {
		add = s.iconButton(th, l.NewTab, s.plus, w.OnNew, nil)
	}
	more := tabRef{ref: &s.more, child: s.iconButton(th, l.AllTabs, s.chevron, s.allTabs, func(f bool) { s.moreFocused = f })}
	return tabShortcuts{onShortcut: s.shortcut, child: tabBarRow{
		ref: &s.row, line: c.Hair, hasNew: w.OnNew != nil && enabled,
		children: []core.Widget{list, add, toolbarSlot{child: more}},
	}}
}

// separated reports whether a line parts tab i from the next one: not beside
// the selected tab, the one under the pointer, or one being dragged.
func (s *tabBarState) separated(i int) bool {
	w := s.Widget()
	if i >= len(w.Tabs)-1 {
		return false
	}
	busy := func(j int) bool {
		return j == w.Selected || j == s.hovered || (s.drag.phase != dragNone && j == s.drag.index)
	}
	return s.drag.phase < dragDragging && !busy(i) && !busy(i+1)
}

// iconButton returns a quiet square button with an icon.
func (s *tabBarState) iconButton(th *theme.Theme, label string, icon *paint.Path, onPress func(), onFocus func(bool)) core.Widget {
	c := &th.Color
	return basic.Padding{Insets: geom.Insets{Left: tabButtonGap, Top: tabTop + (theme.ControlHeight-tabButtonSize)/2}, Child: interact.Pressable{
		Label: label, OnPress: onPress, OnFocusChange: onFocus,
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
			return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
				Width: tabButtonSize, Height: tabButtonSize, Radius: th.Radius.Control, Color: ground,
				Padding: geom.All((tabButtonSize - tabIconSize) / 2),
				Child:   basic.Icon{Path: icon, Size: tabIconSize, Color: ink},
			}}
		},
	}}
}
