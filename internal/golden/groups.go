package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// groupCell is the size of a cell of the groups scene, in logical pixels.
const groupCell = 104

// groupStar returns a five-pointed star around the center of a cell.
func groupStar(r float32) *paint.Path {
	p := &paint.Path{}
	for i := range 10 {
		radius := r
		if i%2 == 1 {
			radius = r * 0.45
		}
		a := float32(i)*0.6283185 - 1.5707963
		x, y := 52+radius*geom.Cos32(a), 50+radius*geom.Sin32(a)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

// groupShapes draws what most cells filter: a red disc, a blue box over it and
// a green star, overlapping.
func groupShapes(cv *paint.Canvas, star *paint.Path) {
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: 14, Y: 12, W: 50, H: 50}, 25), paint.Paint{Color: geom.Hex(0xff453a)})
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: 40, Y: 34, W: 50, H: 40}, 6), paint.Paint{Color: geom.Hex(0x0a84ff)})
	cv.DrawPath(star, paint.Paint{Color: geom.Color{R: 0.19, G: 0.82, B: 0.35, A: 0.9}})
}

// groupFilterCell draws the shapes through filter.
func groupFilterCell(star *paint.Path, filter *paint.Filter) core.Widget {
	return control.CustomPaint{Width: groupCell, Height: groupCell, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		cv.PushGroup(paint.GroupStyle{Filter: filter})
		groupShapes(cv, star)
		cv.PopGroup()
	})}
}

func groupPaint(draw func(cv *paint.Canvas)) core.Widget {
	return control.CustomPaint{Width: groupCell, Height: groupCell, Painter: layout.PainterFunc(func(cv *paint.Canvas, _ geom.Size) {
		draw(cv)
	})}
}

// groups draws groups of the canvas (ADR 0114): an opacity over shapes that
// overlap, nested groups that blend, masks of alpha and luminance, every filter
// primitive, and groups inside a RepaintBoundary, inside a layer that has to
// be drawn inline, and under an opacity and a turn.
func groups() core.Widget {
	star := groupStar(30)
	region := geom.Rect{X: 0, Y: 0, W: groupCell, H: groupCell}
	saturate := float32(0.2)
	filters := []*paint.Filter{
		{Region: region, Primitives: []paint.FilterPrimitive{{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 1.2, Y: 1.2}}}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 5, Y: 2}},
			{Kind: paint.FilterOffset, Offset: geom.Offset{X: 6, Y: 4}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterDropShadow, StdDev: geom.Offset{X: 3, Y: 3}, Offset: geom.Offset{X: 4, Y: 5}, Color: geom.Color{R: 0.2, A: 0.6}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterFlood, Color: geom.Color{R: 1, G: 0.8, A: 1}, Subregion: geom.Rect{X: 20, Y: 20, W: 60, H: 60}},
			{Kind: paint.FilterComposite, In: paint.FilterSourceGraphic, In2: paint.FilterResult(0), Operator: paint.CompositeIn},
			{Kind: paint.FilterComposite, In: paint.FilterResult(0), In2: paint.FilterSourceAlpha, Operator: paint.CompositeArithmetic,
				K: [4]float32{0.5, 0.3, 0.2, 0}},
			{Kind: paint.FilterMerge, Inputs: []paint.FilterInput{paint.FilterResult(2), paint.FilterResult(1)}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterColorMatrix, Matrix: [20]float32{
				0.213 + 0.787*saturate, 0.715 - 0.715*saturate, 0.072 - 0.072*saturate, 0, 0,
				0.213 - 0.213*saturate, 0.715 + 0.285*saturate, 0.072 - 0.072*saturate, 0, 0,
				0.213 - 0.213*saturate, 0.715 - 0.715*saturate, 0.072 + 0.928*saturate, 0, 0,
				0, 0, 0, 1, 0,
			}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterComponentTransfer, SRGB: true, Transfer: [4]paint.TransferFunc{
				{Kind: paint.TransferTable, Table: []float32{1, 0.2, 0.8}},
				{Kind: paint.TransferDiscrete, Table: []float32{0, 0.5, 1}},
				{Kind: paint.TransferGamma, Amplitude: 1, Exponent: 0.5, Offset: 0.05},
				{Kind: paint.TransferLinear, Slope: 0.8},
			}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterFlood, Color: geom.Hex(0x5e5ce6)},
			{Kind: paint.FilterBlend, In: paint.FilterSourceGraphic, In2: paint.FilterResult(0), Mode: paint.BlendScreen,
				Subregion: geom.Rect{X: 0, Y: 0, W: 52, H: groupCell}},
			{Kind: paint.FilterBlend, In: paint.FilterSourceGraphic, In2: paint.FilterResult(1), Mode: paint.BlendHue,
				Subregion: geom.Rect{X: 52, Y: 0, W: 52, H: groupCell}},
			{Kind: paint.FilterMerge, Inputs: []paint.FilterInput{paint.FilterResult(1), paint.FilterResult(2)}},
		}},
		{Region: region, Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterMorphology, Radius: geom.Offset{X: 3, Y: 1}, Dilate: true},
			{Kind: paint.FilterMorphology, In: paint.FilterSourceGraphic, Radius: geom.Offset{X: 2, Y: 2}},
			{Kind: paint.FilterComposite, In: paint.FilterResult(1), In2: paint.FilterResult(0), Operator: paint.CompositeXor},
		}},
	}
	cells := make([]core.Widget, 0, 7+len(filters))
	cells = append(cells,
		groupPaint(func(cv *paint.Canvas) {
			cv.Save()
			cv.PushOpacity(0.6)
			cv.PushGroup(paint.GroupStyle{})
			groupShapes(cv, star)
			cv.PopGroup()
			cv.Restore()
		}),
		groupPaint(func(cv *paint.Canvas) {
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 4, Y: 4, W: 96, H: 96}}, paint.Paint{Gradient: rainbow(96)})
			cv.PushGroup(paint.GroupStyle{Blend: paint.BlendMultiply})
			cv.Save()
			cv.PushOpacity(0.7)
			cv.PushGroup(paint.GroupStyle{})
			groupShapes(cv, star)
			cv.PopGroup()
			cv.Restore()
			cv.PopGroup()
		}),
		groupPaint(func(cv *paint.Canvas) {
			cv.PushGroup(paint.GroupStyle{Mask: paint.MaskAlpha})
			groupShapes(cv, star)
			cv.BeginMask()
			cv.DrawPath(groupStar(44), paint.Paint{Color: geom.Color{A: 1}})
			cv.PopGroup()
		}),
		groupPaint(func(cv *paint.Canvas) {
			cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance})
			groupShapes(cv, star)
			cv.BeginMask()
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 0, Y: 0, W: groupCell, H: groupCell}}, paint.Paint{Gradient: &paint.Gradient{
				End: geom.Offset{X: groupCell}, StartColor: geom.Color{R: 1, G: 1, B: 1, A: 1}, EndColor: geom.Color{G: 0.3, A: 1},
			}})
			cv.PopGroup()
		}),
	)
	for _, f := range filters {
		cells = append(cells, groupFilterCell(star, f))
	}
	blurred := &paint.Filter{Region: region, Primitives: []paint.FilterPrimitive{{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 2.5, Y: 2.5}}}}
	cells = append(cells,
		basic.RepaintBoundary{Child: groupFilterCell(star, blurred)},
		// The path of the clip makes the layer inline.
		basic.RepaintBoundary{Child: groupPaint(func(cv *paint.Canvas) {
			cv.Save()
			cv.ClipPath(groupStar(48))
			cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance, Filter: blurred})
			groupShapes(cv, star)
			cv.BeginMask()
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: 20, Y: 10, W: 64, H: 80}, 30), paint.Paint{Color: geom.Color{R: 1, G: 1, B: 1, A: 1}})
			cv.PopGroup()
			cv.Restore()
		})},
		basic.Opacity{Opacity: 0.7, Child: basic.Transform{
			Transform: geom.Rotation(0.3), Origin: layout.AlignCenter,
			Child: groupFilterCell(star, filters[2]),
		}},
	)
	const cols = 8
	var rows []core.Widget
	for i := 0; i < len(cells); i += cols {
		rows = append(rows, basic.Row{Spacing: 4, Children: cells[i:min(i+cols, len(cells))]})
	}
	return basic.Container{
		Color:   geom.Hex(0xf2f2f7),
		Padding: geom.All(6),
		Child:   basic.Column{Spacing: 4, Children: rows},
	}
}
