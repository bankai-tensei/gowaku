package nav

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// toolbarSpacing is the gap between the items of a toolbar.
	toolbarSpacing = theme.Space4
	// toolbarIcon is the size of an icon on a toolbar button, and
	// toolbarIconGap the gap to its label.
	toolbarIcon    = 16
	toolbarIconGap = theme.Space6
	// toolbarPadding is the room at the ends of a button with a label: that
	// of a button without a ground (DESIGN §5).
	toolbarPadding = theme.Space8
	// toolbarSeparatorRoom is the room on each side of a separator, and
	// toolbarSeparatorLength the length of its line.
	toolbarSeparatorRoom   = theme.Space4
	toolbarSeparatorLength = 16
)

// ToolbarItem is one entry of a Toolbar: a button, a separator, a flexible space
// or a control of your own.
//
//	ToolbarItem{Action: bold, Icon: boldIcon}
//	ToolbarItem{Separator: true}
//	ToolbarItem{Spacer: true}
//	ToolbarItem{Child: searchField}
type ToolbarItem struct {
	// Action gives the label, whether the button is enabled and, when it is
	// Checkable, its state. The & of a mnemonic is taken out of the label, with
	// the parentheses of a Japanese one: "太字(&B)" shows 太字.
	Action interact.Action
	// Icon makes the button a square that shows the icon alone, drawn on a 24
	// unit grid; the label still names it to assistive technology and in the
	// menu of the items that do not fit.
	Icon *paint.Path
	// ShowLabel shows the label after the icon.
	ShowLabel bool
	// Separator makes the item a line between groups, and Spacer an empty
	// space that takes the room left over, which pushes the items after it to
	// the end. The rest is unused then.
	Separator, Spacer bool
	// Child is a control of your own, such as a search field, which takes the
	// place of the button. It keeps its own place in the order of Tab, and the
	// arrow keys pass it by. When it does not fit, the menu shows Action
	// instead if it has a label, and nothing otherwise.
	Child core.Widget
}

// button reports whether the item is a button of the toolbar.
func (it *ToolbarItem) button() bool { return !it.Separator && !it.Spacer && it.Child == nil }

// roving reports whether the arrow keys stop on the item.
func (it *ToolbarItem) roving() bool { return it.button() && it.Action.Enabled() }

// Toolbar is a row of buttons for the commands of a window or a pane, with
// separators between groups. It is one stop of Tab, as a native toolbar is: Tab
// comes to the button used last, and the arrow keys move between the buttons,
// Home and End to the first and the last. Disabled buttons are skipped.
//
// The buttons have no ground until the pointer comes near; a Checkable action
// shows its state with the accent tint. Items that do not fit go into a menu
// behind a … button at the end. The toolbar is 38 high, with room of 8 at its
// ends, and takes the whole width it is given.
type Toolbar struct {
	core.StatefulBase
	// Label names the toolbar to assistive technology.
	Label string
	Items []ToolbarItem
}

type toolbarState struct {
	core.StateBase
	// current is the item that Tab comes to, or len(Items) for the … button,
	// or -1 for none.
	current int
	// hidden is the first item that did not fit at the last layout.
	hidden int
	// refs are the focus objects of the buttons, the … button last.
	refs []*layout.RenderFocus
	more *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Toolbar) CreateState() core.State { return &toolbarState{} }

// InitState implements core.State.
func (s *toolbarState) InitState() {
	s.hidden = len(s.Widget().(Toolbar).Items)
	s.more = ellipsisPath(kit.DefaultIconSize)
}

// ellipsisPath returns three dots across the middle of a square of side box.
func ellipsisPath(box float32) *paint.Path {
	var p paint.Path
	r := box * 3 / 32
	for i := range 3 {
		x := box/2 + float32(i-1)*box*9/32
		p.AddRRect(geom.RRectAll(geom.Rect{X: x - r, Y: box/2 - r, W: 2 * r, H: 2 * r}, r))
	}
	return &p
}

// visible reports whether item i, or the … button for len(items), shows now.
func (s *toolbarState) visible(items []ToolbarItem, i int) bool {
	if i == len(items) {
		return s.hidden < len(items)
	}
	return i >= 0 && i < s.hidden
}

// stops reports whether the arrow keys stop at i.
func (s *toolbarState) stops(items []ToolbarItem, i int) bool {
	if !s.visible(items, i) {
		return false
	}
	return i == len(items) || items[i].roving()
}

// step returns the next stop from i in direction dir, or -1.
func (s *toolbarState) step(items []ToolbarItem, i, dir int) int {
	for j := i + dir; j >= 0 && j <= len(items); j += dir {
		if s.stops(items, j) {
			return j
		}
	}
	return -1
}

// settle keeps current on a stop: the first one when it is on none.
func (s *toolbarState) settle(items []ToolbarItem) {
	if s.stops(items, s.current) {
		return
	}
	if s.current >= s.hidden && s.stops(items, len(items)) {
		s.current = len(items)
		return
	}
	s.current = s.step(items, -1, 1)
}

// key moves the focus between the buttons.
func (s *toolbarState) key(e *core.KeyEvent) bool {
	items := s.Widget().(Toolbar).Items
	if e.Modifiers != 0 {
		return false
	}
	var next int
	switch e.Key {
	case input.KeyLeft:
		next = s.step(items, s.current, -1)
	case input.KeyRight:
		next = s.step(items, s.current, 1)
	case input.KeyHome:
		next = s.step(items, -1, 1)
	case input.KeyEnd:
		next = s.step(items, len(items)+1, -1)
	default:
		return false
	}
	if next >= 0 {
		s.focusItem(next)
	}
	return true
}

// focusItem makes item i the stop of Tab and gives it the keyboard.
func (s *toolbarState) focusItem(i int) {
	s.current = i
	if r := s.refs[i]; r != nil && r.Attached() {
		// The next build enables it as well; the focus cannot wait for it.
		r.Enabled = true
		r.RequestFocus()
	}
	kit.Rebuild(&s.StateBase)
}

// moveStop makes item i the stop of Tab, as a click on it does. The keyboard
// follows when a button of the toolbar has it, since the one it is on stops
// taking it.
func (s *toolbarState) moveStop(i int) {
	if s.current == i {
		return
	}
	if s.focusInside() {
		s.focusItem(i)
		return
	}
	s.current = i
	kit.Rebuild(&s.StateBase)
}

// focusInside reports whether a button of the toolbar has the keyboard.
func (s *toolbarState) focusInside() bool {
	f := kit.FocusOf(s.Context())
	if f == nil {
		return false
	}
	for _, r := range s.refs {
		if r != nil && core.FocusTarget(r) == f {
			return true
		}
	}
	return false
}

// overflowed follows the layout, which finds what fits.
func (s *toolbarState) overflowed(first int) {
	if s.hidden != first {
		s.hidden = first
		kit.Rebuild(&s.StateBase)
	}
}

// overflowItems returns the menu of the items that do not fit.
func (s *toolbarState) overflowItems(items []ToolbarItem) []overlay.MenuItem {
	out := make([]overlay.MenuItem, 0, len(items)-s.hidden)
	for i := s.hidden; i < len(items); i++ {
		it := &items[i]
		switch {
		case it.Spacer:
		case it.Separator:
			if len(out) > 0 && !out[len(out)-1].Separator {
				out = append(out, overlay.MenuItem{Separator: true})
			}
		case it.Child != nil && it.Action.Label == "":
		default:
			out = append(out, overlay.MenuItem{Action: it.Action})
		}
	}
	if len(out) > 0 && out[len(out)-1].Separator {
		out = out[:len(out)-1]
	}
	return out
}

// openOverflow opens the menu of the … button.
func (s *toolbarState) openOverflow(keyboard bool) {
	items := s.Widget().(Toolbar).Items
	menu := s.overflowItems(items)
	r := s.refs[len(items)]
	if len(menu) == 0 || r == nil || !r.Attached() {
		return
	}
	ctx := s.Context()
	overlay.ShowMenuBelow(ctx, r, menu, keyboard)
}

// Build implements core.State.
func (s *toolbarState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Toolbar)
	n := len(w.Items)
	s.hidden = min(s.hidden, n)
	if len(s.refs) != n+1 {
		s.refs = make([]*layout.RenderFocus, n+1)
	}
	s.settle(w.Items)

	children := make([]core.Widget, 0, n+1)
	for i := range w.Items {
		it := &w.Items[i]
		var child core.Widget
		var kind slotKind
		switch {
		case it.Spacer:
			kind, child = slotSpacer, basic.SizedBox{}
		case it.Separator:
			kind = slotSeparator
			child = basic.Padding{Insets: geom.Insets{Left: toolbarSeparatorRoom, Right: toolbarSeparatorRoom}, Child: basic.Divider{
				Axis: layout.Vertical, Indent: (theme.ControlHeight - toolbarSeparatorLength) / 2,
				EndIndent: (theme.ControlHeight - toolbarSeparatorLength) / 2,
			}}
		case it.Child != nil:
			child = it.Child
		default:
			child = s.button(i, it)
		}
		children = append(children, toolbarSlot{kind: kind, child: child})
	}
	children = append(children, toolbarSlot{child: toolbarButton{
		label: basic.StringsOf(ctx).More, icon: s.more, enabled: true,
		tabStop: s.current == n, ref: &s.refs[n], onKey: s.key,
		onPress: func(keyboard bool) {
			s.moveStop(len(s.Widget().(Toolbar).Items))
			s.openOverflow(keyboard)
		},
	}})
	return interact.Semantics{Role: semantics.RoleToolbar, Label: w.Label, Child: basic.Padding{
		Insets: geom.Symmetric(theme.Space8, theme.Space4),
		Child:  toolbarRow{onOverflow: s.overflowed, children: children},
	}}
}

// button returns the button of item i.
func (s *toolbarState) button(i int, it *ToolbarItem) core.Widget {
	act := it.Action
	b := toolbarButton{
		label: toolbarLabel(act.Label), icon: it.Icon, showLabel: it.ShowLabel || it.Icon == nil,
		enabled: act.Enabled(), checkable: act.Checkable, checked: act.Checked,
		shortcut: act.Shortcut.String(), tabStop: s.current == i, ref: &s.refs[i], onKey: s.key,
	}
	if b.enabled {
		b.onPress = func(bool) {
			s.moveStop(i)
			if f := s.Widget().(Toolbar).Items[i].Action.OnInvoke; f != nil {
				f()
			}
		}
	}
	return b
}

// toolbarLabel takes the mnemonic out of label, with the parentheses that hold
// it in a Japanese label.
func toolbarLabel(label string) string {
	if i := strings.Index(label, "(&"); i >= 0 && len(label) == i+4 && label[i+3] == ')' {
		label = label[:i]
	}
	return kit.ParseMnemonic(label).Text
}

// toolbarButton is a button of a toolbar: no ground until the pointer comes, a
// square for an icon alone, and a focus that only the stop of Tab takes.
type toolbarButton struct {
	core.StatefulBase
	label     string
	icon      *paint.Path
	showLabel bool
	enabled   bool
	checkable bool
	checked   bool
	shortcut  string
	// tabStop makes the button the one Tab comes to.
	tabStop bool
	ref     **layout.RenderFocus
	onKey   func(e *core.KeyEvent) bool
	onPress func(keyboard bool)
}

type toolbarButtonState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (toolbarButton) CreateState() core.State { return &toolbarButtonState{} }

// InitState implements core.State.
func (s *toolbarButtonState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *toolbarButtonState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *toolbarButtonState) Build(ctx core.BuildContext) core.Widget {
	b := s.Widget().(toolbarButton)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	hot := s.Hot.Get()

	fill := c.Fill.WithAlpha(c.Fill.A * hot)
	fill = theme.Over(fill, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	ink, tint := c.Ink, theme.Over(c.Ink2, c.Ink, hot)
	if b.checked {
		fill = theme.Over(c.AccentTint, c.Ink, kit.HoverStrength*hot+(kit.PressStrength-kit.HoverStrength)*sink)
		on := c.OnTint()
		ink, tint = on.AccentInk, on.AccentInk
	}
	if !b.enabled {
		fill, ink, tint = geom.Color{}, c.Ink3, c.Ink3
		if b.checked {
			fill = c.Fill
		}
	}
	style := kit.Colored(th.Text.Control, ink)
	row := make([]core.Widget, 0, 2)
	if b.icon != nil {
		row = append(row, basic.Icon{Path: b.icon, Size: toolbarIcon, Color: tint})
	}
	if b.showLabel {
		row = append(row, basic.Text{Content: b.label, Style: style})
	}
	box := basic.Container{
		Height: theme.ControlHeight, Radius: th.Radius.Control, Color: fill,
		Sink:    kit.ButtonSink * sink,
		Padding: geom.Symmetric(toolbarPadding, 0),
		Child:   basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: toolbarIconGap, Children: row},
	}
	if !b.showLabel {
		box.Width = theme.ControlHeight
		box.Padding = geom.Insets{}
		box.Child = basic.Center{Child: basic.Icon{Path: b.icon, Size: toolbarIcon, Color: tint}}
	}
	sem := interact.Semantics{Role: semantics.RoleButton, Label: b.label, Shortcut: b.shortcut}
	if b.checkable {
		sem.Role, sem.Value = semantics.RoleCheckbox, "unchecked"
		if b.checked {
			sem.State, sem.Value = semantics.Checked, "checked"
		}
	}
	body := s.Ring(th.Radius.Control, c.AccentInk, box)
	if !b.enabled {
		sem.State |= semantics.Disabled
		// The focus object stays, disabled, so that the toolbar can tell it
		// apart from a button that is gone.
		sem.Child = toolbarFocus{ref: b.ref, child: body}
		return sem
	}
	sem.OnTap = func() { s.Widget().(toolbarButton).onPress(false) }
	sem.Child = toolbarFocus{
		ref:     b.ref,
		enabled: b.tabStop,
		onKey: func(e *core.KeyEvent) bool {
			if kit.Activates(e) {
				if !e.Repeat {
					s.Sink.Kick(kit.KeyKick)
					s.Widget().(toolbarButton).onPress(true)
				}
				return true
			}
			return s.Widget().(toolbarButton).onKey(e)
		},
		onFocusChange: s.Focus,
		onShowFocus:   s.ShowFocus,
		child: interact.MouseRegion{
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   s.Press,
				OnTapCancel: s.Release,
				OnTap: func() {
					s.Release()
					s.Widget().(toolbarButton).onPress(false)
				},
				Child: body,
			},
		},
	}
	return sem
}

// toolbarFocus is Focus that hands its render object to the toolbar, which
// moves the keyboard between its buttons.
type toolbarFocus struct {
	ref           **layout.RenderFocus
	enabled       bool
	onKey         func(e *core.KeyEvent) bool
	onFocusChange func(focused bool)
	onShowFocus   func(show bool)
	child         core.Widget
}

// Key implements core.Widget.
func (toolbarFocus) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f toolbarFocus) CreateRenderObject() core.RenderObject {
	r := &layout.RenderFocus{}
	f.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f toolbarFocus) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFocus)
	r.Enabled, r.OnKey, r.OnFocusChange, r.OnShowFocus = f.enabled, f.onKey, f.onFocusChange, f.onShowFocus
	*f.ref = r
}

// ChildWidget implements core.SingleChildWidget.
func (f toolbarFocus) ChildWidget() core.Widget { return f.child }

// slotKind is what a slot of a toolbar holds, which decides how it fits.
type slotKind uint8

const (
	slotItem slotKind = iota
	slotSeparator
	slotSpacer
)

// toolbarSlot is the widget of renderToolbarSlot.
type toolbarSlot struct {
	kind  slotKind
	child core.Widget
}

// Key implements core.Widget.
func (toolbarSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t toolbarSlot) CreateRenderObject() core.RenderObject { return &renderToolbarSlot{kind: t.kind} }

// UpdateRenderObject implements core.RenderWidget.
func (t toolbarSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderToolbarSlot)
	if r.kind != t.kind {
		r.kind = t.kind
		r.Parent().MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t toolbarSlot) ChildWidget() core.Widget { return t.child }

// renderToolbarSlot holds one item of a toolbar, which the row hides when it
// does not fit: the pointer, Tab and assistive technology then pass it by.
type renderToolbarSlot struct {
	layout.RenderProxy
	kind   slotKind
	hidden bool
}

// Hidden implements core.Hider.
func (r *renderToolbarSlot) Hidden() bool { return r.hidden }

// Paint implements core.RenderObject.
func (r *renderToolbarSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	if !r.hidden {
		r.RenderProxy.Paint(cv, off)
	}
}

// toolbarRow is the widget of renderToolbarRow.
type toolbarRow struct {
	onOverflow func(first int)
	children   []core.Widget
}

// Key implements core.Widget.
func (toolbarRow) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t toolbarRow) CreateRenderObject() core.RenderObject {
	return &renderToolbarRow{onOverflow: t.onOverflow, first: -1}
}

// UpdateRenderObject implements core.RenderWidget.
func (t toolbarRow) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderToolbarRow).onOverflow = t.onOverflow
}

// ChildWidgets implements core.MultiChildWidget.
func (t toolbarRow) ChildWidgets() []core.Widget { return t.children }

// renderToolbarRow lays out the slots of a toolbar in a row 30 high, the last
// one the … button. When the items do not all fit, it shows the … button at the
// end and hides the items from the first that does not fit before it, and
// reports that one.
type renderToolbarRow struct {
	core.RenderBase
	onOverflow func(first int)
	children   []core.RenderObject
	widths     []float32
	// first is the first hidden item at the last layout, or -1 before one.
	first int
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderToolbarRow) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderToolbarRow) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderToolbarRow) ChildAt(i int) core.RenderObject { return r.children[i] }

func (r *renderToolbarRow) slot(i int) *renderToolbarSlot { return r.children[i].(*renderToolbarSlot) }

// Layout implements core.RenderObject.
func (r *renderToolbarRow) Layout(c geom.Constraints) geom.Size {
	const height = theme.ControlHeight
	n := len(r.children) - 1
	if n < 0 {
		return c.Constrain(geom.Size{H: height})
	}
	if cap(r.widths) < n {
		r.widths = make([]float32, n)
	}
	r.widths = r.widths[:n]
	loose := geom.Constraints{MaxW: geom.Inf, MaxH: height}
	var total float32
	spacers := 0
	for i := range n {
		w := core.LayoutChild(r.children[i], loose).W
		r.widths[i] = w
		total += w
		if i > 0 {
			total += toolbarSpacing
		}
		if r.slot(i).kind == slotSpacer {
			spacers++
		}
	}
	limit := c.MaxW
	first, moreW := n, float32(0)
	if limit < geom.Inf && total > limit {
		moreW = core.LayoutChild(r.children[n], loose).W
		room := limit - moreW - toolbarSpacing
		var x float32
		first = 0
		for i := range n {
			need := r.widths[i]
			if i > 0 {
				need += toolbarSpacing
			}
			if x+need > room {
				break
			}
			x += need
			first = i + 1
		}
		// A line or a space has nothing to go with at the end.
		for first > 0 && r.slot(first-1).kind != slotItem {
			first--
		}
	}

	width := total
	if limit < geom.Inf {
		width = limit
	}
	// The space left over goes to the spacers.
	var share float32
	if first == n && spacers > 0 && limit < geom.Inf {
		share = (limit - total) / float32(spacers)
	}
	var x float32
	for i := range n {
		s := r.slot(i)
		s.hidden = i >= first
		if s.hidden {
			continue
		}
		if s.kind == slotSpacer && share > 0 {
			r.widths[i] = core.LayoutChild(s, geom.Tight(geom.Size{W: share, H: height})).W
		}
		sz := s.Size()
		s.SetOffset(geom.Offset{X: geom.Floor32(x + 0.5), Y: geom.Floor32((height-sz.H)/2 + 0.5)})
		x += r.widths[i] + toolbarSpacing
	}
	more := r.slot(n)
	more.hidden = first == n
	if !more.hidden {
		more.SetOffset(geom.Offset{X: geom.Floor32(width - moreW + 0.5), Y: geom.Floor32((height-more.Size().H)/2 + 0.5)})
	}
	if first != r.first {
		r.first = first
		if r.onOverflow != nil {
			r.onOverflow(first)
		}
	}
	return c.Constrain(geom.Size{W: width, H: height})
}

// Paint implements core.RenderObject.
func (r *renderToolbarRow) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
