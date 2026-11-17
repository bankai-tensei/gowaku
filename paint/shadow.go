package paint

import "github.com/bankai-tensei/gowaku/geom"

// BoxShadow is a shadow behind a box, with the meaning of CSS box-shadow: the
// box, grown by Spread on every side and moved by Offset, filled with Color
// and blurred by the blur radius Blur, as Canvas.DrawShadow draws it. It is the one type for
// shadows: theme.Colors.Shadows returns it, and basic.Container,
// layout.Decoration and the controls take it.
type BoxShadow struct {
	Color  geom.Color
	Offset geom.Offset
	Blur   float32
	Spread float32
}
