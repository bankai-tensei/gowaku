package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// cardPadding is the room inside a card, that of the sides of a group
	// (DESIGN §4).
	cardPadding = theme.Space16
	// cardSink is how far a press shrinks a pressable card. A card is far
	// wider than a button, so the sink of a button would move its edges by
	// several pixels: half of it reads as the same press.
	cardSink = kit.ButtonSink / 2
)

// Card is a surface that holds one piece of content, such as a note in a grid or
// a summary on a dashboard: the theme's Card ground, corners of 12, a hairline
// and the contact shadow of a control (DESIGN §6). Padding defaults to 16 on
// every side.
//
// With OnPress the whole card is a button: it runs OnPress when clicked, or when
// Space or Enter is pressed while it has the focus, and it reacts as the
// controls do, with a wash on hover, a spring sink on a press and the focus
// ring. Put no other control inside a pressable card, or give that control's
// action to the card. Without OnPress it is a plain surface that assistive
// technology sees as a group named Label.
type Card struct {
	core.StatefulBase
	// Label names the card to assistive technology: the name of the group, or
	// of the button when OnPress is set.
	Label string
	// Padding is the room around the child. The zero value selects 16 on every
	// side.
	Padding layout.Insets
	// OnPress makes the card a button.
	OnPress func()
	Child   core.Widget
}

type cardState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (Card) CreateState() core.State { return &cardState{} }

// InitState implements core.State.
func (s *cardState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *cardState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *cardState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Card)
	th := basic.ThemeOf(ctx)
	c := &th.Color
	padding := w.Padding
	if padding == (layout.Insets{}) {
		padding = geom.All(cardPadding)
	}
	fill := c.Card
	pressable := w.OnPress != nil
	var sink float32
	if pressable {
		sink = geom.Clamp32(s.Sink.Value(), 0, 1)
		fill = theme.Over(fill, c.Ink, kit.HoverStrength*s.Hot.Get()+(kit.PressStrength-kit.HoverStrength)*sink)
	}
	body := basic.Container{
		Color:   fill,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: c.Hair},
		Shadows: th.Color.Shadows(theme.LiftControl),
		Sink:    cardSink * sink,
		Padding: padding,
		Child:   w.Child,
	}
	if !pressable {
		if w.Label == "" {
			return body
		}
		return interact.Semantics{Role: semantics.RoleGroup, Label: w.Label, Child: body}
	}
	return s.Wrap(interact.Semantics{Role: semantics.RoleButton, Label: w.Label}, true, w.OnPress,
		s.Ring(th.Radius.Group, c.AccentInk, body))
}
