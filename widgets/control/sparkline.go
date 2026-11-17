package control

import (
	"math"
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The measures of a sparkline.
const (
	// sparklineWidth and sparklineHeight are the size of a sparkline whose
	// parent leaves it unbounded.
	sparklineWidth  = 64
	sparklineHeight = 20
	// sparklineThickness is the width of the line when none is given.
	sparklineThickness = 1.5
	// sparklineBarShare is the share of its slot that a bar fills; the rest is
	// the gap to the next one.
	sparklineBarShare = 0.7
	// sparklineAreaAlpha is the opacity of the area fill at the top.
	sparklineAreaAlpha = 0.22
)

// SparklineKind is the shape of a Sparkline.
//
// It is SparklineLine (the zero value) or SparklineBars.
type SparklineKind uint8

// The kinds of sparkline.
const (
	// SparklineLine joins the values with a line.
	SparklineLine SparklineKind = iota
	// SparklineBars draws a bar per value, up from zero for positive values
	// and down from it for negative ones.
	SparklineBars
)

// String returns the name of the kind.
func (k SparklineKind) String() string {
	switch k {
	case SparklineLine:
		return "line"
	case SparklineBars:
		return "bars"
	}
	return "unknown"
}

// Sparkline is a chart the size of a word or a table cell: a line or bars
// over Data, without axes, legend or tooltips. It is Width by Height; a zero
// length takes all the room its parent allows, or 64 by 20 where that is
// unbounded.
//
// The values span the height from the smallest to the largest, or from Min to
// Max when Max is greater than Min, outside which they are clamped. Bars span
// zero too, so that they rise and fall from a baseline. A NaN or infinite value
// leaves a gap: the line breaks there and the bar is missing. Empty Data draws
// nothing.
//
// A screen reader sees an image named by Label, or by a summary of the
// smallest, the largest and the latest value without one.
//
// Painting does not allocate: the line is built when the widget or the size
// changes and reused on every paint after that, which keeps a scrolled table of
// a thousand sparklines free of garbage. Data is read, not copied, and must
// not change until the next build.
type Sparkline struct {
	core.StatelessBase
	Data []float64
	Kind SparklineKind
	// Min and Max fix the range when Max is greater than Min.
	Min, Max float64
	// Width and Height force a size. Zero takes what the parent allows.
	Width, Height float32
	// Color draws the line and the bars of positive values, and Negative the
	// bars of negative values. The zero values select the accent ink and the
	// danger color of the theme.
	Color, Negative geom.Color
	// Thickness is the width of the line. Zero selects 1.5.
	Thickness float32
	// Fill shades the area under the line, fading downwards.
	Fill bool
	// MarkLast puts a dot on the last value of a line.
	MarkLast bool
	// Label describes the chart to assistive technology. Empty gives a summary
	// of the smallest, largest and latest value.
	Label string
}

// Build implements core.StatelessWidget.
func (w Sparkline) Build(ctx core.BuildContext) core.Widget {
	c := &basic.ThemeOf(ctx).Color
	l := basic.StringsOf(ctx)
	v := sparklineView{w: w, color: w.Color, negative: w.Negative, words: [3]string{l.Minimum, l.Maximum, l.Latest}}
	if v.color == (geom.Color{}) {
		v.color = c.AccentInk
	}
	if v.negative == (geom.Color{}) {
		v.negative = c.Danger
	}
	v.baseline = c.Hair
	return v
}

// sparklineView is the render widget of a Sparkline. It holds the widget in a
// field: embedding it would bring its Build along.
type sparklineView struct {
	w                         Sparkline
	color, negative, baseline geom.Color
	words                     [3]string
}

// Key implements core.Widget.
func (sparklineView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v sparklineView) CreateRenderObject() core.RenderObject {
	r := &renderSparkline{}
	v.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget. The data may have changed
// in place, so every update paints again.
func (v sparklineView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderSparkline)
	if r.Width != v.w.Width || r.Height != v.w.Height {
		r.MarkNeedsLayout()
	}
	v.apply(r)
	r.MarkNeedsPaint()
}

func (v sparklineView) apply(r *renderSparkline) {
	w := &v.w
	r.Data, r.Kind, r.Min, r.Max = w.Data, w.Kind, w.Min, w.Max
	r.Width, r.Height, r.Thickness = w.Width, w.Height, w.Thickness
	r.Color, r.Negative, r.Baseline = v.color, v.negative, v.baseline
	r.Fill, r.MarkLast, r.Label = w.Fill, w.MarkLast, w.Label
	r.words = v.words
	r.built = false
}

// renderSparkline draws a Sparkline. Its fields are those of the widget, with
// the colors resolved.
type renderSparkline struct {
	core.RenderBase
	Data          []float64
	Kind          SparklineKind
	Min, Max      float64
	Width, Height float32
	Thickness     float32
	// Color, Negative and Baseline draw the line or positive bars, the
	// negative bars, and the zero line under bars that go below it.
	Color, Negative, Baseline geom.Color
	Fill, MarkLast            bool
	Label                     string

	// words are Minimum, Maximum and Latest of the Strings, for the summary.
	words [3]string

	// line, area and the rest are what Paint draws, built for builtSize
	// while built holds.
	line, area paint.Path
	areaFill   paint.Gradient
	stroke     paint.Stroke
	dot        geom.RRect
	hasDot     bool
	built      bool
	builtSize  geom.Size
}

// Layout implements core.RenderObject.
func (r *renderSparkline) Layout(c geom.Constraints) geom.Size {
	pick := func(want, limit, fallback float32) float32 {
		switch {
		case want > 0:
			return want
		case limit < geom.Inf:
			return limit
		}
		return fallback
	}
	return c.Constrain(geom.Size{W: pick(r.Width, c.MaxW, sparklineWidth), H: pick(r.Height, c.MaxH, sparklineHeight)})
}

func (r *renderSparkline) thickness() float32 {
	if r.Thickness > 0 {
		return r.Thickness
	}
	return sparklineThickness
}

// dotRadius is the radius of the dot on the last value.
func (r *renderSparkline) dotRadius() float32 { return r.thickness() + 1 }

// inset is the room kept free at each edge, so that the line and its dot are
// not cut off.
func (r *renderSparkline) inset() float32 {
	if r.Kind == SparklineBars {
		return 0
	}
	if r.MarkLast {
		return r.dotRadius()
	}
	return r.thickness() / 2
}

// finite reports whether v can be drawn.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// span returns the values at the bottom and the top of the box.
func (r *renderSparkline) span() (lo, hi float64) {
	if r.Max > r.Min {
		return r.Min, r.Max
	}
	first := true
	for _, v := range r.Data {
		if !finite(v) {
			continue
		}
		if first {
			lo, hi, first = v, v, false
			continue
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	if r.Kind == SparklineBars {
		lo, hi = min(lo, 0), max(hi, 0)
	}
	return lo, hi
}

// sparkFrame maps values to the box.
type sparkFrame struct {
	lo, hi        float64
	top, height   float32
	left, width   float32
	count         int
	bars          bool
	middle, floor float32
}

func (r *renderSparkline) frame(sz geom.Size) sparkFrame {
	in := r.inset()
	lo, hi := r.span()
	f := sparkFrame{
		lo: lo, hi: hi, top: in, height: max(sz.H-2*in, 0), left: in, width: max(sz.W-2*in, 0),
		count: len(r.Data), bars: r.Kind == SparklineBars,
	}
	f.middle = f.top + f.height/2
	f.floor = f.top + f.height
	return f
}

// y returns the height in the box of v.
func (f *sparkFrame) y(v float64) float32 {
	if f.hi <= f.lo {
		return f.middle
	}
	v = min(max(v, f.lo), f.hi)
	return f.top + float32((f.hi-v)/(f.hi-f.lo))*f.height
}

// x returns the middle of value i.
func (f *sparkFrame) x(i int) float32 {
	if f.bars {
		slot := f.width / float32(f.count)
		return f.left + slot*(float32(i)+0.5)
	}
	if f.count == 1 {
		return f.left + f.width/2
	}
	return f.left + f.width*float32(i)/float32(f.count-1)
}

// build makes the line, the area and the dot for sz.
func (r *renderSparkline) build(sz geom.Size) {
	r.built, r.builtSize = true, sz
	r.line.Reset()
	r.area.Reset()
	r.hasDot = false
	if r.Kind != SparklineLine || len(r.Data) == 0 {
		return
	}
	f := r.frame(sz)
	pen := false
	// runStart is the first point of the run of finite values that the pen
	// draws, for the area under it.
	runStart := 0
	closeArea := func(end int) {
		if !r.Fill || end-runStart < 2 {
			return
		}
		r.area.MoveTo(f.x(runStart), f.floor)
		for k := runStart; k < end; k++ {
			r.area.LineTo(f.x(k), f.y(r.Data[k]))
		}
		r.area.LineTo(f.x(end-1), f.floor)
		r.area.Close()
	}
	last := -1
	for i, v := range r.Data {
		if !finite(v) {
			if pen {
				closeArea(i)
			}
			pen = false
			continue
		}
		x, y := f.x(i), f.y(v)
		if pen {
			r.line.LineTo(x, y)
		} else {
			r.line.MoveTo(x, y)
			pen, runStart = true, i
		}
		last = i
	}
	if pen {
		closeArea(len(r.Data))
	}
	if r.MarkLast && last >= 0 {
		rad := r.dotRadius()
		x, y := f.x(last), f.y(r.Data[last])
		r.dot = geom.RRectAll(geom.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, rad)
		r.hasDot = true
	}
	top := r.Color.WithAlpha(r.Color.A * sparklineAreaAlpha)
	r.areaFill = paint.Gradient{
		Start: geom.Offset{Y: f.top}, End: geom.Offset{Y: f.floor},
		StartColor: top, EndColor: r.Color.WithAlpha(0),
	}
}

// Paint implements core.RenderObject.
func (r *renderSparkline) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if sz.W <= 0 || sz.H <= 0 || len(r.Data) == 0 {
		return
	}
	if r.Kind == SparklineBars {
		r.paintBars(cv, off, sz)
		return
	}
	if !r.built || r.builtSize != sz {
		r.build(sz)
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	if !r.area.Empty() {
		cv.DrawPath(&r.area, paint.Paint{Gradient: &r.areaFill})
	}
	if !r.line.Empty() {
		r.stroke = paint.Stroke{Width: r.thickness(), Color: r.Color}
		cv.DrawPath(&r.line, paint.Paint{Stroke: &r.stroke})
	}
	if r.hasDot {
		cv.DrawRRect(r.dot, paint.Paint{Color: r.Color})
	}
	cv.Restore()
}

// paintBars draws a bar per finite value, from zero.
func (r *renderSparkline) paintBars(cv *paint.Canvas, off geom.Offset, sz geom.Size) {
	f := r.frame(sz)
	zero := f.y(0)
	if f.lo < 0 && f.hi > 0 && r.Baseline.A > 0 {
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: off.Y + zero - 0.5, W: sz.W, H: 1}}, paint.Paint{Color: r.Baseline})
	}
	slot := f.width / float32(f.count)
	bw := slot * sparklineBarShare
	if slot-bw < 1 {
		// Keep a gap of a pixel between bars while there is room for both.
		bw = max(slot-1, slot/2)
	}
	radius := min(1, bw/2)
	for i, v := range r.Data {
		if !finite(v) {
			continue
		}
		y := f.y(v)
		top, bottom := min(y, zero), max(y, zero)
		if bottom-top <= 0 {
			continue
		}
		color := r.Color
		if v < 0 {
			color = r.Negative
		}
		rect := geom.Rect{X: off.X + f.x(i) - bw/2, Y: off.Y + top, W: bw, H: bottom - top}
		cv.DrawRRect(geom.RRectAll(rect, min(radius, rect.H/2)), paint.Paint{Color: color})
	}
}

// Semantics implements core.RenderObject. The summary is spelled only here,
// when a screen reader asks, never while painting.
func (r *renderSparkline) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleImage, Label: r.summary()}, r.Size())
	sb.Close()
}

// summary returns the Label, or the smallest, largest and latest value.
func (r *renderSparkline) summary() string {
	if r.Label != "" {
		return r.Label
	}
	var lo, hi, last float64
	found := false
	for _, v := range r.Data {
		if !finite(v) {
			continue
		}
		if !found {
			lo, hi, found = v, v, true
		}
		lo, hi, last = min(lo, v), max(hi, v), v
	}
	if !found {
		return ""
	}
	return r.words[0] + " " + sparkNumber(lo) + ", " + r.words[1] + " " + sparkNumber(hi) + ", " +
		r.words[2] + " " + sparkNumber(last)
}

// sparkNumber spells v with at most two decimals.
func sparkNumber(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}
