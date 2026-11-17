package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// gradients fills boxes with gradients of three and four stops: a linear one
// as on an album cover, a radial one, and one whose offsets leave room at
// both ends and meet in a hard edge.
func gradients() core.Widget {
	const size = 120
	purple, pink, orange := geom.Hex(0x7b2ff7), geom.Hex(0xf107a3), geom.Hex(0xff8a00)
	linear := &paint.Gradient{
		End: geom.Offset{X: size, Y: size},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0, Color: purple}, {Offset: 0.5, Color: pink}, {Offset: 1, Color: orange},
		},
		StopCount: 3,
	}
	radial := &paint.Gradient{
		Kind:  paint.GradientRadial,
		Start: geom.Offset{X: size / 2, Y: size / 2},
		End:   geom.Offset{X: size, Y: size / 2},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0, Color: geom.Hex(0xffffff)},
			{Offset: 0.3, Color: geom.Hex(0x4cc9f0)},
			{Offset: 0.7, Color: geom.Hex(0x4361ee)},
			{Offset: 1, Color: geom.Hex(0x3a0ca3)},
		},
		StopCount: 4,
	}
	banded := &paint.Gradient{
		End: geom.Offset{X: size},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0.2, Color: geom.Hex(0x06d6a0)},
			{Offset: 0.5, Color: geom.Hex(0xffd166)},
			{Offset: 0.5, Color: geom.Hex(0xef476f)},
			{Offset: 0.8, Color: geom.Hex(0x118ab2)},
		},
		StopCount: 4,
	}
	box := func(g *paint.Gradient, radius float32) core.Widget {
		return basic.Container{Width: size, Height: size, Radius: radius, Gradient: g}
	}
	return basic.Padding{Insets: geom.All(16), Child: basic.Row{Spacing: 16, Children: []core.Widget{
		box(linear, 16),
		box(radial, size/2),
		basic.Container{
			Width: size, Height: size, Radius: 8, Gradient: banded,
			Border: paint.Stroke{Width: 2, Color: geom.Hex(0x222222)},
		},
	}}}
}
