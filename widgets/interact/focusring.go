package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// FocusRing draws the focus ring of DESIGN §5, the pale 3px ring that the
// library's own controls show, just outside its child while Visible. It fades
// in and out as theirs does. A Pressable shows it with the flag it is given:
//
//	Pressable{Label: "Album", OnPress: open, Build: func(s PressState) core.Widget {
//		return FocusRing{Visible: s.FocusVisible, Radius: 8, Child: cover}
//	}}
//
// The ring is drawn outside the child's box and takes no room, so leave a few
// pixels around the child where a neighbor would hide it.
type FocusRing struct {
	core.StatefulBase
	// Visible shows the ring.
	Visible bool
	// Radius is the corner radius of the child, which the ring follows.
	Radius float32
	// Color is the hue of the ring; the ring sets its alpha. The zero Color
	// selects the theme's AccentInk, as the controls use.
	Color geom.Color
	Child core.Widget
}

type focusRingState struct {
	core.StateOf[FocusRing]
	fade motion.Implicit
	lit  *motion.Value[float32]
}

// CreateState implements core.StatefulWidget.
func (FocusRing) CreateState() core.State { return &focusRingState{} }

// InitState implements core.State.
func (s *focusRingState) InitState() {
	s.fade.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.lit = motion.Track(&s.fade, kit.OnOff(s.Widget().Visible), motion.LerpFloat)
}

// DidUpdateWidget implements core.State.
func (s *focusRingState) DidUpdateWidget(core.Widget) {
	s.lit.Set(kit.OnOff(s.Widget().Visible))
	s.fade.Commit(kit.HoverDuration, motion.Linear)
}

// Dispose implements core.State.
func (s *focusRingState) Dispose() { s.fade.Dispose() }

// Build implements core.State.
func (s *focusRingState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	color := w.Color
	if color == (geom.Color{}) {
		color = basic.ThemeOf(ctx).Color.AccentInk
	}
	return kit.Ring{Radius: w.Radius, Color: color.WithAlpha(kit.RingOpacity * s.lit.Get()), Child: w.Child}
}
