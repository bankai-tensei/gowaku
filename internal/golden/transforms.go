package golden

import (
	"image"
	"image/color"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// checker returns a picture of colored squares, whose edges show how an image is
// filtered when turned.
func checker() *paint.Image {
	const n, cell = 32, 8
	m := image.NewNRGBA(image.Rect(0, 0, n, n))
	colors := [...]color.NRGBA{{0xef, 0x47, 0x6f, 0xff}, {0xff, 0xd1, 0x66, 0xff}, {0x06, 0xd6, 0xa0, 0xff}, {0x11, 0x8a, 0xb2, 0xff}}
	for y := range n {
		for x := range n {
			m.SetNRGBA(x, y, colors[(x/cell+y/cell)%len(colors)])
		}
	}
	return paint.NewImage(m)
}

// star returns a five-pointed star in a box of size, filled even-odd so that
// its middle stays empty.
func star(size float32) *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	r := size / 2
	for i := range 5 {
		a := float32(i)*4*3.14159265/5 - 3.14159265/2
		x, y := r+r*geom.Cos32(a), r+r*geom.Sin32(a)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

// transforms turns and slants every kind of drawing: a card with a gradient,
// a border and a shadow, text, paths filled and stroked, an image with rounded
// corners, which clips, a rectangular clip, and a layer.
func transforms() core.Widget {
	shape, line := star(80), &paint.Path{}
	line.MoveTo(4, 70)
	line.QuadTo(40, 20, 76, 70)
	pictured := checker()
	card := basic.Container{
		Width: 100, Height: 64, Radius: 12,
		Gradient: &paint.Gradient{
			End: geom.Offset{X: 100, Y: 64},
			Stops: [paint.MaxGradientStops]paint.GradientStop{
				{Offset: 0, Color: geom.Hex(0x7b2ff7)}, {Offset: 0.5, Color: geom.Hex(0xf107a3)}, {Offset: 1, Color: geom.Hex(0xff8a00)},
			},
			StopCount: 3,
		},
		Border:  paint.Stroke{Width: 2, Color: geom.Hex(0x222222)},
		Shadows: []paint.BoxShadow{{Color: geom.Hex(0x000000).WithAlpha(0.35), Blur: 14, Offset: geom.Offset{Y: 5}}},
	}
	paths := control.CustomPaint{Width: 80, Height: 80, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.DrawPath(shape, paint.Paint{Color: geom.Hex(0x2f5fd0)})
		cv.DrawPath(line, paint.Paint{Stroke: &paint.Stroke{Width: 3, Color: geom.Hex(0xef476f)}})
	})}
	clipped := control.CustomPaint{Width: 70, Height: 70, Painter: layout.PainterFunc(func(cv *paint.Canvas, sz geom.Size) {
		cv.Save()
		cv.ClipRect(geom.Rect{X: 8, Y: 8, W: sz.W - 16, H: sz.H - 16})
		cv.DrawRRect(geom.RRectAll(geom.Rect{W: sz.W, H: sz.H}, sz.W/2), paint.Paint{Color: geom.Hex(0x06d6a0)})
		cv.Restore()
	})}
	label := basic.Container{
		Color: geom.Hex(0xfff3c4), Radius: 6, Padding: geom.Symmetric(8, 4),
		Child: basic.Text{Content: "Turned text"},
	}
	return basic.Center{Child: basic.Row{Size: layout.MainMin, Spacing: 30, Cross: layout.CrossCenter, Children: []core.Widget{
		basic.Rotate(0.35, card),
		basic.Rotate(-0.5, label),
		basic.Skew(0.35, 0, paths),
		basic.Rotate(0.6, basic.Image{Source: pictured, Width: 64, Height: 64, Radius: 12}),
		basic.Rotate(0.4, clipped),
		basic.Rotate(-0.3, basic.RepaintBoundary{Child: basic.Container{
			Color: geom.Hex(0xdde6ff), Radius: 4, Padding: geom.All(6), Child: basic.Text{Content: "Layer"},
		}}),
	}}}
}
