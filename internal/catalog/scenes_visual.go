package catalog

import (
	"math"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

// The color picker, the skeletons and the sparklines (ADR 0094 and 0095).

func sceneFieldColorPicker() core.Widget {
	return field.ColorPicker{
		Value: geom.Hex(0x3a7bf0), Label: "Accent color", OnChange: func(geom.Color) {},
		Swatches: []geom.Color{
			geom.Hex(0x3a7bf0), geom.Hex(0x5e5ce6), geom.Hex(0xbf5af2), geom.Hex(0xff375f),
			geom.Hex(0xff9f0a), geom.Hex(0x30d158), geom.Hex(0x8e8e93),
		},
	}
}

func sceneControlSkeleton() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{Width: 320, Color: th.Color.Card, Radius: th.Radius.Group, Padding: geom.All(theme.Space16), Child: control.SkeletonGroup{
			Label: "Loading messages",
			Child: basic.Row{Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
				control.Skeleton{Shape: control.SkeletonCircle, Width: 40},
				basic.Expanded{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
					control.Skeleton{Width: 96, Height: 14},
					control.SkeletonText{Lines: 3},
				}}},
			}},
		}}
	})
	// catalog:end
}

func sceneControlSparkline() core.Widget {
	trend, change := make([]float64, 24), make([]float64, 12)
	for i := range trend {
		trend[i] = 20 + 6*math.Sin(float64(i)/3.5)
	}
	trend[9] = math.NaN() // a gap
	for i := range change {
		change[i] = 4 * math.Sin(float64(i)/1.7)
	}
	// catalog:begin
	return basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space24, Children: []core.Widget{
		basic.SizedBox{Width: 140, Height: 28, Child: control.Sparkline{
			Data: trend, Fill: true, MarkLast: true, Label: "Temperature, now 21.4 degrees",
		}},
		basic.SizedBox{Width: 120, Height: 28, Child: control.Sparkline{Kind: control.SparklineBars, Data: change}},
	}}
	// catalog:end
}
