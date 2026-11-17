package golden

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// busy is the backdrop of the blur scenes: a gradient, shapes of every color and
// lines of dark text, sharp details that a blur visibly smooths.
func busy(width, height float32) core.Widget {
	const line = "Blur smooths every sharp edge of text and shapes below it. "
	shapes := make([]core.Widget, 0, 23)
	shapes = append(shapes, basic.Container{Width: width, Height: height, Gradient: &paint.Gradient{
		End: geom.Offset{X: width, Y: height},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0, Color: geom.Hex(0x1d3fbf)}, {Offset: 0.4, Color: geom.Hex(0x10b981)},
			{Offset: 0.75, Color: geom.Hex(0xfde047)}, {Offset: 1, Color: geom.Hex(0xf43f5e)},
		},
		StopCount: 4,
	}})
	colors := []uint32{0xff453a, 0xffffff, 0x0a84ff, 0x1d1d1f, 0xbf5af2, 0x30d158}
	for i := range 14 {
		x, y := float32(i%7)*92+10, float32(i/7)*150+30
		radius := float32(6)
		if i%3 == 0 {
			radius = 22
		}
		shapes = append(shapes, basic.Positioned{Left: x, Top: y, Child: basic.Container{
			Width: 44, Height: 44, Radius: radius, Color: geom.Hex(colors[i%len(colors)]),
		}})
	}
	for i := range 8 {
		shapes = append(shapes, basic.Positioned{Left: 8, Top: float32(i)*42 + 8, Width: width - 16, Child: basic.Text{
			Content: line + line, Style: text.Style{Size: 13, Color: geom.Hex(0x111111)},
		}})
	}
	return basic.Stack{Children: shapes}
}

// glassCard is a translucent card over a backdrop blur of sigma.
func glassCard(label string, sigma float32) core.Widget {
	return basic.BackdropFilter{Sigma: sigma, Radius: 16, Child: basic.Container{
		Width: 140, Height: 120, Radius: 16, Color: geom.Color{R: 1, G: 1, B: 1, A: 0.35},
		Border:  paint.Stroke{Width: 1, Color: geom.Color{R: 1, G: 1, B: 1, A: 0.6}},
		Padding: geom.All(12),
		Child:   basic.Text{Content: label, Style: text.Style{Size: 15, Color: geom.Hex(0x111111)}},
	}}
}

// blurs draws backdrop blurs of two sigmas over the busy backdrop, one inside a
// RepaintBoundary, which must draw it inline, and the blur of a child: text and
// a shape, one of them turned and faded.
func blurs() core.Widget {
	const width, height = 640, 360
	return basic.Stack{Children: []core.Widget{
		busy(width, height),
		basic.Positioned{Left: 20, Top: 40, Child: glassCard("sigma 3", 3)},
		basic.Positioned{Left: 180, Top: 40, Child: glassCard("sigma 14", 14)},
		basic.Positioned{Left: 340, Top: 40, Child: basic.RepaintBoundary{Child: glassCard("boundary", 8)}},
		basic.Positioned{Left: 20, Top: 200, Child: basic.ImageFiltered{Sigma: 2, Child: basic.Text{
			Content: "ImageFiltered", Style: text.Style{Size: 32, Color: geom.Hex(0xffffff)},
		}}},
		basic.Positioned{Left: 300, Top: 190, Child: basic.Transform{
			Transform: geom.Rotation(0.25), Origin: layout.AlignCenter,
			Child: basic.Opacity{Opacity: 0.8, Child: basic.ImageFiltered{Sigma: 6, Child: basic.Container{
				Width: 120, Height: 80, Radius: 12, Color: geom.Hex(0x1d1d1f),
				Padding: geom.All(10),
				Child:   basic.Text{Content: "soft", Style: text.Style{Size: 20, Color: geom.Hex(0xfde047)}},
			}}},
		}},
		basic.Positioned{Left: 480, Top: 210, Child: glassCard("sigma 40", 40)},
	}}
}

// glassRow is a row of a colorful list, the page behind the glass dialogs.
func glassRow(i int) core.Widget {
	colors := []uint32{0xff453a, 0xff9f0a, 0xffd60a, 0x30d158, 0x64d2ff, 0x0a84ff, 0x5e5ce6, 0xbf5af2}
	c := geom.Hex(colors[i%len(colors)])
	return basic.Padding{Insets: geom.Symmetric(12, 4), Child: basic.Container{
		Height: 40, Radius: 10, Color: c.WithAlpha(0.85), Padding: geom.Symmetric(12, 10),
		Child: basic.Text{Content: fmt.Sprintf("Item %d of a colorful list", i+1), Style: text.Style{Size: 14, Color: geom.Hex(0x111111)}},
	}}
}

func glassPage() core.Widget {
	return collection.List{ItemCount: 100, ItemExtent: 48, ItemBuilder: glassRow}
}

// showGlassDialog opens a glass dialog over the page.
func showGlassDialog(_ *core.Tree, ctx core.BuildContext) {
	overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
		return overlay.Dialog{
			Title:   "Glass dialog",
			Message: "The page behind shows through, blurred, while the text stays easy to read.",
			Actions: []core.Widget{
				control.Button{Label: "Cancel", OnPress: dismiss},
				control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: dismiss},
			},
			Width: 420,
			Glass: true,
		}
	})
}
