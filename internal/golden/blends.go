package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

const (
	// blendCellWidth and blendCellHeight are the size of a cell of the grid of
	// blend modes, in logical pixels.
	blendCellWidth  = 100
	blendCellHeight = 64
	blendColumns    = 6
)

// rainbow is the backdrop of the blend scenes: colors of every hue and
// lightness, so that each mode shows what it does.
func rainbow(width float32) *paint.Gradient {
	return &paint.Gradient{
		End: geom.Offset{X: width, Y: 12},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0, Color: geom.Hex(0x1d3fbf)}, {Offset: 0.35, Color: geom.Hex(0x10b981)},
			{Offset: 0.7, Color: geom.Hex(0xfde047)}, {Offset: 1, Color: geom.Hex(0xffffff)},
		},
		StopCount: 4,
	}
}

// backdrop is a box of the rainbow with a dark label on it.
func backdrop(label string, width, height float32) core.Widget {
	return basic.Container{
		Width: width, Height: height, Radius: 8, Gradient: rainbow(width),
		Padding: geom.Symmetric(6, 4),
		Child:   basic.Text{Content: label, Style: text.Style{Size: 13, Color: geom.Hex(0x111111)}},
	}
}

// source is the shape that blends: a disc of a translucent gradient from pink
// to cyan, which reaches past the backdrop at the bottom right.
func source(mode paint.BlendMode) core.Widget {
	return control.CustomPaint{Width: blendCellWidth, Height: blendCellHeight + 8, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		disc := geom.RRectAll(geom.Rect{X: 44, Y: 14, W: 56, H: 56}, 28)
		cv.DrawRRect(disc, paint.Paint{
			Gradient: &paint.Gradient{
				Start: geom.Offset{X: 44, Y: 14}, End: geom.Offset{X: 100, Y: 70},
				StartColor: geom.Color{R: 0.95, G: 0.25, B: 0.55, A: 0.85},
				EndColor:   geom.Color{R: 0.1, G: 0.8, B: 0.95, A: 0.85},
			},
			Blend: mode,
		})
	})}
}

// blends draws a cell for every blend mode: the disc blends over the rainbow
// and its label, and past the backdrop over the canvas. Nothing in it needs
// multisampling, so the modes that read the frame copy it directly.
func blends() core.Widget {
	var rows []core.Widget
	row := make([]core.Widget, 0, blendColumns)
	for m := range paint.BlendModeCount {
		mode := paint.BlendMode(m)
		row = append(row, basic.SizedBox{Width: blendCellWidth, Height: blendCellHeight + 8, Child: basic.Stack{Children: []core.Widget{
			backdrop(mode.String(), blendCellWidth-8, blendCellHeight),
			source(mode),
		}}})
		if len(row) == blendColumns || m == paint.BlendModeCount-1 {
			rows = append(rows, basic.Row{Spacing: 8, Children: row})
			row = make([]core.Widget, 0, blendColumns)
		}
	}
	return basic.Padding{Insets: geom.All(12), Child: basic.Column{Spacing: 6, Children: rows}}
}

// pair is two overlapping translucent discs, whose overlap shows whether they
// blend as one picture or one by one.
func pair(mode paint.BlendMode) core.Widget {
	return control.CustomPaint{Width: 120, Height: 90, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 10, Y: 20, W: 60, H: 60}, 30), paint.Paint{Color: geom.Color{R: 0.1, G: 0.75, B: 0.95, A: 0.9}, Blend: mode})
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 50, Y: 20, W: 60, H: 60}, 30), paint.Paint{Color: geom.Color{R: 0.95, G: 0.3, B: 0.5, A: 0.9}, Blend: mode})
	})}
}

// blendStar is a five-pointed star path, filled non-zero.
func blendStar(size float32) *paint.Path {
	p := star(size)
	p.Rule = paint.FillNonZero
	return p
}

// blends2 blends groups: a subtree with Blend, the same under Opacity, a layer
// of a RepaintBoundary that blends inside itself, where the backdrop is
// transparent around its own rainbow, and paths, which make the frame
// multisampled so that the copies resolve.
func blends2() core.Widget {
	cell := func(label string, child core.Widget) core.Widget {
		return basic.SizedBox{Width: 140, Height: 110, Child: basic.Stack{Children: []core.Widget{
			backdrop(label, 130, 90),
			basic.Positioned{Left: 6, Top: 14, Child: child},
		}}}
	}
	path := blendStar(70)
	paths := control.CustomPaint{Width: 120, Height: 90, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.Save()
		cv.Translate(4, 10)
		cv.DrawPath(path, paint.Paint{Color: geom.Color{R: 0.9, G: 0.2, B: 0.6, A: 0.9}, Blend: paint.BlendOverlay})
		cv.Translate(50, 0)
		cv.DrawPath(path, paint.Paint{Color: geom.Color{R: 0.2, G: 0.3, B: 0.9, A: 0.8}, Blend: paint.BlendScreen})
		cv.Restore()
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 96, Y: 4, W: 20, H: 20}, 4), paint.Paint{Color: geom.Hex(0x000000), Blend: paint.BlendClear})
	})}
	layered := basic.RepaintBoundary{Child: control.CustomPaint{Width: 140, Height: 110, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 10, Y: 10, W: 90, H: 70}, 8), paint.Paint{Gradient: rainbow(90)})
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 60, Y: 40, W: 70, H: 60}, 30), paint.Paint{Color: geom.Color{R: 0.95, G: 0.4, B: 0.2, A: 0.8}, Blend: paint.BlendDifference})
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 30, H: 30}, 6), paint.Paint{Color: geom.Color{A: 0.7}, Blend: paint.BlendDstOut})
	})}}
	return basic.Padding{Insets: geom.All(12), Child: basic.Row{Spacing: 10, Children: []core.Widget{
		cell("group", basic.Blend{Mode: paint.BlendMultiply, Child: pair(paint.BlendSrcOver)}),
		cell("each", pair(paint.BlendMultiply)),
		cell("faded", basic.Opacity{Opacity: 0.6, Child: basic.Blend{Mode: paint.BlendHue, Child: pair(paint.BlendSrcOver)}}),
		layered,
		cell("paths", paths),
	}}}
}
