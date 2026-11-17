package overlay

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// PopupList is the card of choices that DropdownMenu and Autocomplete open
// below themselves. It looks like a menu, with the rows of menu items, and
// scrolls the highlighted row into view.
type PopupList struct {
	core.StatefulBase
	Label  string
	Labels []string
	// Disabled marks the rows that cannot be chosen; rows past its end can.
	Disabled []bool
	// Hot is the highlighted row, and Checked the row with the check mark.
	// Either is -1 for none. Checks keeps a column for the mark.
	Hot, Checked int
	Checks       bool
	// Rows is how many rows show before the list scrolls; zero selects eight.
	Rows  int
	Width float32
	// Empty is shown when there are no rows.
	Empty string
	// Refs, when not nil, makes every row take the focus, with its object
	// stored at its index. Autofocus is the row that takes it first.
	Refs       []*layout.RenderFocus
	Autofocus  int
	OnHover    func(i int)
	OnTap      func(i int)
	OnKey      func(e *core.KeyEvent) bool
	OnRowFocus func(i int, focused bool)
}

type popupListState struct {
	basic.ScrollState
	shown int
}

// CreateState implements core.StatefulWidget.
func (PopupList) CreateState() core.State { return &popupListState{} }

// InitState implements core.State.
func (s *popupListState) InitState() {
	s.ScrollState.InitState()
	s.reveal()
}

// DidUpdateWidget implements core.State.
func (s *popupListState) DidUpdateWidget(core.Widget) {
	if s.Widget().(PopupList).Hot != s.shown {
		s.reveal()
	}
}

// reveal scrolls the highlighted row into view.
func (s *popupListState) reveal() {
	p := s.Widget().(PopupList)
	s.shown = p.Hot
	if p.Hot < 0 || len(p.Labels) <= p.visible() {
		return
	}
	view := float64(p.visible() * kit.MenuItemHeight)
	top := float64(p.Hot * kit.MenuItemHeight)
	at := s.Offset.Value()
	switch {
	case top < at:
		s.Jump(top)
	case top+kit.MenuItemHeight > at+view:
		s.Jump(top + kit.MenuItemHeight - view)
	}
}

// Build implements core.State.
func (s *popupListState) Build(ctx core.BuildContext) core.Widget {
	p := s.Widget().(PopupList)
	th := basic.ThemeOf(ctx)
	var body core.Widget
	switch n := len(p.Labels); {
	case n == 0:
		body = basic.Container{
			Height:  kit.MenuItemHeight,
			Padding: geom.Insets{Left: menuItemPadding, Right: menuItemPadding},
			Child:   basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{Content: p.Empty, Style: kit.Colored(th.Text.Body, th.Color.Ink2)}},
		}
	case n <= p.visible():
		body = s.rows(th, p)
	default:
		view := basic.Viewport{State: &s.ScrollState, Child: s.rows(th, p)}
		body = basic.SizedBox{Height: float32(p.visible() * kit.MenuItemHeight), Child: basic.ScrollFrame{
			Axis: layout.Vertical, State: &s.ScrollState, View: view,
		}}
	}
	return interact.Semantics{Role: semantics.RoleList, Label: p.Label, Child: basic.Container{
		Width:   p.Width,
		Color:   th.Color.Raised,
		Radius:  th.Radius.Control,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.All(kit.MenuInset),
		Child:   body,
	}}
}

// visible returns how many rows show before the list scrolls.
func (p PopupList) visible() int {
	if p.Rows > 0 {
		return p.Rows
	}
	return kit.PopupMaxRows
}

func (s *popupListState) rows(th *theme.Theme, p PopupList) core.Widget {
	rows := make([]core.Widget, len(p.Labels))
	for i := range p.Labels {
		rows[i] = s.row(th, p, i)
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: rows}
}

func (s *popupListState) row(th *theme.Theme, p PopupList, i int) core.Widget {
	enabled := i >= len(p.Disabled) || !p.Disabled[i]
	lit := p.Hot == i
	ink := th.Color.Ink
	if !enabled {
		ink = th.Color.Ink3
	}
	var ground geom.Color
	switch {
	case lit && enabled:
		ground = th.Color.AccentTint
	case lit:
		ground = th.Color.Fill
	}
	parts := make([]core.Widget, 0, 2)
	if p.Checks {
		var check core.Widget = basic.SizedBox{Width: menuCheckWidth}
		if i == p.Checked {
			check = basic.SizedBox{Width: menuCheckWidth, Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.SizedBox{
				Width: menuCheckSize, Height: menuCheckSize, Child: kit.Checkmark{Color: ink, Progress: 1},
			}}}
		}
		parts = append(parts, check)
	}
	parts = append(parts, basic.Expanded{Child: basic.Text{Content: p.Labels[i], Style: kit.Colored(th.Text.Body, ink)}})
	sem := interact.Semantics{Role: semantics.RoleListItem, Label: p.Labels[i]}
	if lit {
		sem.State |= semantics.Selected
	}
	var tap func()
	if enabled {
		tap = func() { p.OnTap(i) }
		sem.OnTap = tap
	} else {
		sem.State |= semantics.Disabled
	}
	var child core.Widget = interact.MouseRegion{
		OnEnter: func() { p.OnHover(i) },
		Child: interact.GestureDetector{
			// The press stops here, so that it does not reach what lies
			// under the list or move the keyboard.
			OnTapDown: func() {},
			OnTap:     tap,
			Child: basic.Container{
				Height:  kit.MenuItemHeight,
				Color:   ground,
				Radius:  th.Radius.Check,
				Padding: geom.Insets{Left: menuItemPadding, Right: menuItemPadding},
				Child:   basic.Row{Cross: layout.CrossCenter, Children: parts},
			},
		},
	}
	if p.Refs != nil {
		child = focusRef{
			ref:           &p.Refs[i],
			enabled:       enabled,
			autofocus:     p.Autofocus == i,
			onKey:         p.OnKey,
			onFocusChange: func(focused bool) { p.OnRowFocus(i, focused) },
			child:         child,
		}
	}
	sem.Child = child
	return sem
}

// PopupWidth returns the width of a PopupList of labels: the widest label,
// with room for check marks if checks is set, or at least least.
func PopupWidth(ctx core.BuildContext, labels []string, checks bool, least float32) float32 {
	shaper, ok := core.Lookup[*text.Shaper](ctx)
	if !ok {
		return least
	}
	style := basic.ThemeOf(ctx).Text.Body
	var widest float32
	for _, l := range labels {
		widest = max(widest, kit.TextWidth(shaper, l, style))
	}
	w := 2*kit.MenuInset + 2*menuItemPadding + widest
	if checks {
		w += menuCheckWidth
	}
	return min(max(geom.Ceil32(w), least), menuMaxWidth)
}
