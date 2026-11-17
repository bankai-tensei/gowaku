package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// stopsOf returns a gradient of the given kind whose stops are spread evenly,
// unless offsets says where they go. Up to four go in Stops, more in MoreStops.
func stopsOf(kind paint.GradientKind, start, end geom.Offset, offsets []float32, colors ...geom.Color) *paint.Gradient {
	stops := make([]paint.GradientStop, len(colors))
	for i, c := range colors {
		at := float32(i) / float32(max(len(colors)-1, 1))
		if offsets != nil {
			at = offsets[i]
		}
		stops[i] = paint.GradientStop{Offset: at, Color: c}
	}
	g := &paint.Gradient{Kind: kind, Start: start, End: end}
	if len(stops) > paint.MaxGradientStops {
		g.MoreStops = &stops
	} else {
		g.StopCount = copy(g.Stops[:], stops)
	}
	return g
}

// hue returns a saturated color at turn h of the color wheel.
func hue(h float32) geom.Color {
	channel := func(shift float32) float32 {
		k := h*6 + shift
		k -= 6 * geom.Floor32(k/6)
		return 1 - max(0, min(1, min(k, 4-k)))
	}
	return geom.Color{R: channel(5) * 0.9, G: channel(3) * 0.9, B: channel(1) * 0.9, A: 1}
}

// gradients2 draws what goes beyond four stops in an instance: eight stops
// with hard edges, a color wheel and a pie that sweep, ellipses through a
// gradient transform, a gradient on a turned card, and a grid of boxes that
// each need a row of the ramps.
func gradients2() core.Widget {
	const size = 120
	center := geom.Offset{X: size / 2, Y: size / 2}
	bands := stopsOf(paint.GradientLinear, geom.Offset{}, geom.Offset{X: size, Y: size / 3},
		[]float32{0, 0.25, 0.25, 0.5, 0.5, 0.75, 0.75, 1},
		geom.Hex(0x264653), geom.Hex(0x2a9d8f), geom.Hex(0xe9c46a), geom.Hex(0xe9c46a),
		geom.Hex(0xf4a261), geom.Hex(0xe76f51), geom.Hex(0x9b2226), geom.Hex(0x3a0ca3))
	wheel := stopsOf(paint.GradientSweep, center, geom.Offset{X: size, Y: size / 2}, nil,
		hue(0), hue(1.0/6), hue(2.0/6), hue(3.0/6), hue(4.0/6), hue(5.0/6), hue(0))
	pie := stopsOf(paint.GradientSweep, center, geom.Offset{X: size / 2}, []float32{0, 0.35, 0.35, 0.6, 0.6, 1},
		geom.Hex(0x4361ee), geom.Hex(0x4361ee), geom.Hex(0xf72585), geom.Hex(0xf72585), geom.Hex(0xffd166), geom.Hex(0xff8a00))
	// A unit circle, stretched to the box: an ellipse of four stops in the
	// instance, and one of six through the ramps.
	oval := stopsOf(paint.GradientRadial, geom.Offset{}, geom.Offset{X: 1}, []float32{0, 0.4, 0.8, 1},
		geom.Hex(0xffffff), geom.Hex(0x4cc9f0), geom.Hex(0x4361ee), geom.Hex(0x10002b))
	oval.Transform = geom.Translation(geom.Offset{X: 80, Y: size / 2}).Mul(geom.Scaling(80, size/2))
	rings := stopsOf(paint.GradientRadial, geom.Offset{}, geom.Offset{X: 1}, nil,
		geom.Hex(0xfefae0), geom.Hex(0xdda15e), geom.Hex(0xbc6c25), geom.Hex(0x606c38), geom.Hex(0x283618), geom.Hex(0xfefae0))
	rings.Transform = geom.Translation(geom.Offset{X: 50, Y: 30}).Mul(geom.Rotation(0.5)).Mul(geom.Scaling(60, 30))
	turned := stopsOf(paint.GradientLinear, geom.Offset{}, geom.Offset{X: 110}, nil,
		geom.Hex(0x7b2ff7), geom.Hex(0xf107a3), geom.Hex(0xff8a00), geom.Hex(0xffd166), geom.Hex(0x06d6a0))

	box := func(w, h, radius float32, g *paint.Gradient) core.Widget {
		return basic.Container{Width: w, Height: h, Radius: radius, Gradient: g}
	}
	// Twenty-four boxes of five stops each, no two alike.
	tiles := make([]core.Widget, 0, 24)
	for i := range 24 {
		h := float32(i) / 24
		tiles = append(tiles, box(28, 28, 6, stopsOf(paint.GradientLinear, geom.Offset{}, geom.Offset{X: 28, Y: 28}, nil,
			hue(h), hue(h+0.1), geom.Hex(0xffffff), hue(h+0.5), hue(h+0.6))))
	}
	return basic.Padding{Insets: geom.All(16), Child: basic.Column{Spacing: 20, Children: []core.Widget{
		basic.Row{Spacing: 16, Children: []core.Widget{
			basic.Container{
				Width: size, Height: size, Radius: 12, Gradient: bands,
				Border: paint.Stroke{Width: 2, Color: geom.Hex(0x222222)},
			},
			box(size, size, size/2, wheel),
			box(size, size, size/2, pie),
			box(160, size, 16, oval),
		}},
		basic.Row{Spacing: 28, Cross: layout.CrossCenter, Children: []core.Widget{
			basic.Rotate(0.4, basic.Container{
				Width: 110, Height: 64, Radius: 12, Gradient: turned,
				Border: paint.Stroke{Width: 2, Color: geom.Hex(0x222222)},
			}),
			basic.Rotate(-0.25, box(100, 60, 8, rings)),
			basic.SizedBox{Width: 8*28 + 7*6, Child: basic.Wrap{Spacing: 6, RunSpacing: 6, Children: tiles}},
		}},
	}}}
}
