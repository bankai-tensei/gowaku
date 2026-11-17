package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	groupInset     = theme.Space16
	groupRowHeight = 44
	groupTitleGap  = theme.Space6

	sidebarWidth   = 208
	sidebarIcon    = 16
	sidebarIconGap = theme.Space8
	sidebarInset   = theme.Space10
	// sidebarDetailRow is the height of a row with a Detail: its two lines,
	// of 20 and 18, and 4 above and below.
	sidebarDetailRow = 46
	// sidebarRingRoom is the room the focus ring of a row takes outside it,
	// and sidebarTitleGap what is left of the gap under the title, so that
	// the rows start sidebarTitleGap+sidebarRingRoom below it.
	sidebarRingRoom = theme.Space4
	sidebarTitleGap = theme.Space6
)

// Group is a card of rows: a settings page or a form is a few of these. The lines
// between rows stop short of the left edge.
type Group struct {
	core.StatelessBase
	// Title heads the group, outside the card.
	Title string
	// Children are the rows, usually GroupRows.
	Children []core.Widget
}

// Build implements core.StatelessWidget.
func (g Group) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	lines := make([]core.Widget, 0, 2*len(g.Children))
	for i, row := range g.Children {
		if i > 0 {
			lines = append(lines, basic.Padding{Insets: geom.Insets{Left: groupInset}, Child: basic.Container{Height: 1, Color: th.Color.Hair}})
		}
		lines = append(lines, row)
	}
	card := basic.Container{
		Color:  th.Color.Card,
		Radius: th.Radius.Group,
		Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
		Child:  basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: lines},
	}
	if g.Title == "" {
		return card
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: g.Title, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: groupTitleGap, Children: []core.Widget{
			basic.Padding{Insets: geom.Insets{Left: groupInset}, Child: basic.Text{Content: g.Title, Style: kit.Colored(th.Text.Section, th.Color.Ink2)}},
			card,
		},
	}}
}

// GroupRow is a line of a Group: a name on the left and a control on the right.
// The label also names the control to assistive technology when the control
// has no label of its own, as a Input or a DropdownMenu without a Label; a
// screen reader then hears the name once, on the control.
type GroupRow struct {
	core.StatelessBase
	Label string
	// Detail is a smaller line below the label.
	Detail string
	Child  core.Widget
}

// Build implements core.StatelessWidget.
func (r GroupRow) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	name := core.Widget(basic.Text{Content: r.Label, Style: kit.Colored(th.Text.Body, th.Color.Ink)})
	if r.Label != "" && r.Child != nil {
		name = caption{child: name}
	}
	if r.Detail != "" {
		name = basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Children: []core.Widget{
			name, basic.Text{Content: r.Detail, Style: kit.Colored(th.Text.Caption, th.Color.Ink2)},
		}}
	}
	children := []core.Widget{name}
	if r.Child != nil {
		children = append(children, propertyName{name: r.Label, child: r.Child})
	}
	return basic.ConstrainedBox{MinHeight: groupRowHeight, MaxWidth: geom.Inf, MaxHeight: geom.Inf, Child: basic.Padding{
		Insets: geom.Symmetric(groupInset, theme.Space6),
		Child:  basic.Row{Main: layout.MainSpaceBetween, Cross: layout.CrossCenter, Children: children},
	}}
}

// Sidebar is the column of navigation at the left of a window: a panel that
// draws its own Sidebar ground and the hairline at its right edge, with the
// Title at the top, an optional Header under it, the rows, and an optional
// Footer pinned at the bottom.
//
// Put it first in a Row whose Cross is layout.CrossStretch, so that it runs
// the full height of the window, and give the content beside it an Expanded:
//
//	basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
//		collection.Sidebar{Title: "Library", Children: items},
//		basic.Expanded{Child: page},
//	}}
//
// Its height must be bounded. It is Width wide, 208 when zero, hairline
// included. A parent that fixes the width wins over Width: inside a
// control.SplitPane, size the pane in Pixels and the panel fills it.
//
// The rows scroll when they do not fit, with a scroll bar that shows only
// then, as in basic.Scroll; the Title, Header and Footer stay where they are.
// A screen reader hears the rows as a list named by the Title, which it says
// once (ADR 0065).
type Sidebar struct {
	core.StatelessBase
	Title string
	// Width is the width of the panel. Zero is the designed 208.
	Width float32
	// Header sits under the Title, above the rows, such as a field.Search.
	Header core.Widget
	// Children are the rows, usually SidebarItems.
	Children []core.Widget
	// Footer is pinned at the bottom, under the rows, such as a few buttons.
	Footer core.Widget
}

// Build implements core.StatelessWidget.
func (s Sidebar) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	width := s.Width
	if width <= 0 {
		width = sidebarWidth
	}
	side := func(bottom float32, child core.Widget) core.Widget {
		return basic.Padding{Insets: geom.Insets{Left: sidebarInset, Right: sidebarInset, Bottom: bottom}, Child: child}
	}
	parts := make([]core.Widget, 0, 4)
	if s.Title != "" {
		// The list says the title; the text would say it twice.
		parts = append(parts, silent{child: basic.Padding{
			Insets: geom.Insets{Left: 2 * sidebarInset, Right: sidebarInset, Bottom: sidebarTitleGap},
			Child:  basic.Text{Content: s.Title, Style: kit.Colored(th.Text.Headline, th.Color.Ink2)},
		}})
	}
	if s.Header != nil {
		parts = append(parts, side(sidebarTitleGap, s.Header))
	}
	// The rows keep sidebarRingRoom inside the scrolling view, so that the
	// focus ring of the first and the last row is not cut off.
	bottom := float32(sidebarInset)
	if s.Footer != nil {
		bottom = sidebarRingRoom
	}
	parts = append(parts, basic.Expanded{Child: interact.Semantics{Role: semantics.RoleList, Label: s.Title, Child: basic.Scroll{
		Child: basic.Padding{
			Insets: geom.Insets{Left: sidebarInset, Top: sidebarRingRoom, Right: sidebarInset, Bottom: bottom},
			Child:  basic.Column{Size: layout.MainMin, Spacing: theme.Space2, Cross: layout.CrossStretch, Children: s.Children},
		},
	}}})
	if s.Footer != nil {
		parts = append(parts, basic.Padding{
			Insets: geom.Insets{Left: sidebarInset, Top: sidebarTitleGap, Right: sidebarInset, Bottom: sidebarInset},
			Child:  s.Footer,
		})
	}
	// The SizedBox gives way to a tight width, as that of a SplitPane pane, and
	// the panel then fills it.
	return basic.SizedBox{Width: width, Child: basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Expanded{Child: basic.Container{
			Color: th.Color.Sidebar, Padding: geom.Insets{Top: theme.Space28},
			Child: basic.Column{Cross: layout.CrossStretch, Children: parts},
		}},
		basic.Container{Width: 1, Color: th.Color.Hair},
	}}}
}

// SidebarItem is a row of a Sidebar: an icon, a label, an optional second
// line of detail and, at the end, a count such as that of the unread messages
// of a mailbox.
type SidebarItem struct {
	core.StatefulBase
	Label string
	// Detail is a smaller second line under the label in Ink2, such as the
	// last message of a chat, as GroupRow.Detail is. It is cut to one line
	// with an ellipsis, and so is the label then. A row with a Detail is 46
	// high instead of 30. A screen reader hears the whole detail once, as the
	// description of the row.
	Detail string
	// Icon is drawn before the label on a 24 unit grid, such as one of
	// package icons.
	Icon *paint.Path
	// Leading, if set, is drawn before the label in place of Icon, such as
	// the colored dot of a label in a mail client:
	//
	//	basic.Container{Width: 10, Height: 10, Radius: 5, Color: red}
	//
	// It keeps its own size and is centered across the row; one 16 wide, as
	// an Icon is, lines its label up with the rows that have an Icon. On the
	// Selected row it is built under the theme of theme.Theme.OnTint, so
	// that what takes its colors from the theme reads on the tint, while a
	// color it sets itself, as that of the dot, stays.
	Leading core.Widget
	// Count, when above zero, shows at the end of the row in a quiet pill,
	// as control.Badge with Quiet does, spelled out up to 99 and as "99+"
	// above. A screen reader hears the whole number once, as the value of
	// the row, after its label.
	Count int
	// Selected marks the chosen row with AccentTint. What the row sets on
	// it, the Detail and the Count included, takes the colors of
	// theme.Colors.OnTint, so that it reads at 4.5:1 on the tint.
	Selected bool
	OnPress  func()
}

type sidebarItemState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (SidebarItem) CreateState() core.State { return &sidebarItemState{} }

// InitState implements core.State.
func (s *sidebarItemState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *sidebarItemState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *sidebarItemState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(SidebarItem)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	style, tint := kit.Colored(th.Text.Body, c.Ink), c.Ink2
	fill := motion.LerpColor(c.Fill.WithAlpha(c.Fill.A*s.Hot.Get()), c.FillStrong, sink)
	if w.Selected {
		// The row lies on the tint, where Ink2 and AccentInk fall below
		// 4.5:1: what it sets on it, the count included, takes the colors
		// of OnTint.
		fill = theme.Over(c.AccentTint, c.Ink, kit.PressStrength*sink/2)
		th = th.OnTint()
		c = &th.Color
		style, tint = kit.Colored(th.Text.Control, c.Ink), c.AccentInk
	}
	row := make([]core.Widget, 0, 3)
	switch {
	case w.Leading != nil:
		lead := w.Leading
		if w.Selected {
			lead = basic.Theme{Data: th, Child: lead}
		}
		row = append(row, lead)
	case w.Icon != nil:
		row = append(row, basic.Icon{Path: w.Icon, Size: sidebarIcon, Color: tint})
	}
	textHeight, rowHeight := kit.LabelHeight(&style), float32(theme.ControlHeight)
	label := core.Widget(basic.Text{Content: w.Label, Style: style})
	if w.Detail != "" {
		style.MaxLines = 1
		detail := kit.Colored(th.Text.Caption, c.Ink2)
		detail.MaxLines = 1
		textHeight += kit.LabelHeight(&detail)
		rowHeight = sidebarDetailRow
		// The row says the detail as its description; the text would say
		// it again.
		label = basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
			basic.Text{Content: w.Label, Style: style},
			silent{child: basic.Text{Content: w.Detail, Style: detail}},
		}}
	}
	var value string
	if w.Count > 0 {
		// The row says the count as its value; the pill would say it again.
		value = kit.Itoa(w.Count)
	}
	if w.Count > 0 || w.Detail != "" {
		label = basic.Expanded{Child: label}
	}
	row = append(row, label)
	if w.Count > 0 {
		var pill core.Widget = control.Badge{Count: w.Count, Quiet: true}
		if w.Selected {
			pill = basic.Theme{Data: th, Child: pill}
		}
		row = append(row, silent{child: pill})
	}
	body := s.Ring(th.Radius.Control, c.AccentInk, basic.Container{
		Color: fill, Radius: th.Radius.Control,
		Padding: geom.Symmetric(sidebarInset, max((rowHeight-textHeight)/2, 0)),
		Child:   basic.Row{Spacing: sidebarIconGap, Cross: layout.CrossCenter, Children: row},
	})
	var state semantics.State
	if w.Selected {
		state = semantics.Selected
	}
	return s.Wrap(interact.Semantics{Role: semantics.RoleListItem, Label: w.Label, Value: value, Hint: w.Detail, State: state}, w.OnPress != nil, w.OnPress, body)
}

// caption marks the first node inside it as the visible name of the control
// that a propertyName beside it names. See core.SemanticsBuilder.Caption.
type caption struct {
	child core.Widget
}

// Key implements core.Widget.
func (caption) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (caption) CreateRenderObject() core.RenderObject { return &renderCaption{} }

// UpdateRenderObject implements core.RenderWidget.
func (caption) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (c caption) ChildWidget() core.Widget { return c.child }

type renderCaption struct {
	layout.RenderProxy
}

// Semantics implements core.RenderObject.
func (r *renderCaption) Semantics(sb *core.SemanticsBuilder) {
	sb.Caption()
	r.RenderProxy.Semantics(sb)
}
