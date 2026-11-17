package control

import (
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// badgeHeight is the height of a pill with a number or a word, which holds
	// one line of Caption.
	badgeHeight = 18
	// badgePadding is the room at the ends of the text of a pill, and
	// badgePaddingOne that of a single character, small enough that one digit
	// makes a circle.
	badgePadding    = theme.Space6
	badgePaddingOne = theme.Space4
	// badgeDot is the size of a dot.
	badgeDot = theme.Space8
	// badgeInset is how far the middle of a badge on a child sits inside the
	// child's top right corner, so that it overlaps the corner and stays near.
	badgeInset = theme.Space2
	// badgeMax is the largest count a badge spells out unless Max says
	// otherwise.
	badgeMax = 99
	// badgeRoom is the widest a badge on a child may grow: "999+" in Caption.
	badgeRoom = 48
)

// Badge is a small mark that something needs attention: a count of unread
// items, a word such as "New", or a dot. Alone it is a pill; with a Child it sits
// on the child's top right corner, as on an icon or an avatar:
//
//	Badge{Count: unread, Child: Icon{Path: inbox}}
//
// It shows Label if set, else Count if above zero, else a dot if Dot is set, and
// otherwise nothing: the zero value draws only its child, so that a count that
// falls to zero takes the badge away without an if around it.
//
// The pill is filled with the accent and carries white text; Quiet puts the
// text in Ink2 on the theme's Fill instead, for counts that are information
// rather than a call, such as the number of items in a sidebar folder. A screen
// reader reads the text of the pill after the child. A dot says nothing unless
// it has a Label of its own, which then shows as a pill.
type Badge struct {
	core.StatelessBase
	// Count is the number to show, when above zero.
	Count int
	// Max is the largest count spelled out; above it the badge shows Max and a
	// plus, as "99+". Zero selects 99.
	Max int
	// Label is a word to show instead of the count.
	Label string
	// Dot shows a dot when there is neither a count nor a label.
	Dot bool
	// Quiet draws a gray pill instead of an accent one.
	Quiet bool
	// Child is what the badge sits on. Without it the badge stands alone.
	Child core.Widget
}

// text returns what the pill says, or "" for a dot or nothing.
func (b Badge) text() string {
	if b.Label != "" {
		return b.Label
	}
	if b.Count <= 0 {
		return ""
	}
	limit := b.Max
	if limit <= 0 {
		limit = badgeMax
	}
	if b.Count > limit {
		return kit.Itoa(limit) + "+"
	}
	return kit.Itoa(b.Count)
}

// Build implements core.StatelessWidget.
func (b Badge) Build(ctx core.BuildContext) core.Widget {
	mark := b.mark(basic.ThemeOf(ctx))
	switch {
	case mark == nil && b.Child == nil:
		return basic.SizedBox{}
	case mark == nil:
		return b.Child
	case b.Child == nil:
		return mark
	}
	// The badge hangs over the corner of the child: it takes no room of its
	// own, so that a badge coming and going moves nothing around it. A round
	// one is centered near the corner, and a longer count grows outward,
	// away from the child, from the same left edge.
	size := float32(badgeHeight)
	if b.text() == "" {
		size = badgeDot
	}
	left := badgeInset + size/2
	return basic.Stack{Children: []core.Widget{
		b.Child,
		basic.Positioned{
			AnchorRight: true, Right: left - badgeRoom, Top: badgeInset - size/2,
			Width: badgeRoom, Height: size,
			Child: basic.Align{Alignment: layout.Alignment{}, Child: mark},
		},
	}}
}

// mark returns the pill or the dot, or nil when there is nothing to show.
func (b Badge) mark(th *theme.Theme) core.Widget {
	c := &th.Color
	fill, ink := c.Accent, c.OnAccent
	if b.Quiet {
		fill, ink = c.Fill, c.Ink2
	}
	label := b.text()
	if label == "" {
		if !b.Dot {
			return nil
		}
		return basic.Container{Width: badgeDot, Height: badgeDot, Radius: badgeDot / 2, Color: fill}
	}
	style := kit.Colored(th.Text.Caption, ink)
	style.Weight = th.Text.Section.Weight
	padding := float32(badgePadding)
	if utf8.RuneCountInString(label) == 1 {
		padding = badgePaddingOne
	}
	return interact.Semantics{Role: semantics.RoleText, Label: label, Child: basic.ConstrainedBox{
		MinWidth: badgeHeight,
		Child: basic.Container{
			Height: badgeHeight, Radius: badgeHeight / 2, Color: fill,
			Padding: geom.Symmetric(padding, max((badgeHeight-kit.LabelHeight(&style))/2, 0)),
			Child: basic.Row{Size: layout.MainMin, Main: layout.MainCenter, Cross: layout.CrossCenter, Children: []core.Widget{
				basic.Text{Content: label, Style: style},
			}},
		},
	}}
}
