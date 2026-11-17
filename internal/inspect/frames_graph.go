//go:build gowakudebug

package inspect

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// The geometry of the frame graph, in logical pixels.
const (
	// graphHeight is the height of the graph, its allocation band included.
	graphHeight = 156
	// graphGutter is the room at the left for the labels of the lines.
	graphGutter = 52
	// allocBand is the height of the band of allocation counts under the bars,
	// and allocGap the room between them.
	allocBand = 16
	allocGap  = 6
	// barsHeight is the height of the stacked bars.
	barsHeight = graphHeight - allocBand - allocGap
)

// The reference lines: the frame budgets at 120 and 60 frames a second.
const (
	budget120 = time.Second / 120
	budget60  = time.Second / 60
)

// graphTops are the times the top of the graph can stand for. The graph takes
// the first that holds its tallest bar, so that the scale stays put while the
// frames vary a little.
var graphTops = [...]time.Duration{
	20 * time.Millisecond, time.Second / 30, 50 * time.Millisecond,
	100 * time.Millisecond, 200 * time.Millisecond, 500 * time.Millisecond,
}

// graphTop returns the time the top of the graph stands for, given the
// tallest bar.
func graphTop(tallest time.Duration) time.Duration {
	for _, t := range graphTops {
		if tallest <= t {
			return t
		}
	}
	return graphTops[len(graphTops)-1]
}

// stageColors returns the colors of the stages. They are the system colors of
// a quiet palette, a step lighter in the dark, with present grey: most of it
// is the wait for vsync, not work.
func stageColors(b theme.Brightness) [StageCount]geom.Color {
	if b == theme.BrightnessDark {
		return [StageCount]geom.Color{
			geom.Hex(0x4c9aff), geom.Hex(0x3cc8c0), geom.Hex(0xffb340),
			geom.Hex(0xbf7af0), geom.Hex(0x6e6e78),
		}
	}
	return [StageCount]geom.Color{
		geom.Hex(0x2f7cf6), geom.Hex(0x1ea7a0), geom.Hex(0xf29a1f),
		geom.Hex(0xa25ad9), geom.Hex(0xb4b4bc),
	}
}

// graphPaint holds the colors of the graph.
type graphPaint struct {
	stages            [StageCount]geom.Color
	gpu, allocs       geom.Color
	hover, selected   geom.Color
	baseline, line120 geom.Color
	line60            geom.Color
	selectedEdge      geom.Color
}

// framesGraph paints the frames of a session as stacked bars, one slot per
// record of the ring, the newest at the right. It is a layout.Painter.
type framesGraph struct {
	ses *Session
	// top is the time at the top of the bars.
	top time.Duration
	// maxAllocs scales the allocation band.
	maxAllocs uint64
	// hover and selected are indices in Session.Frames, or -1.
	hover, selected int
	colors          graphPaint
	// width is the width of the last paint, which hit testing needs.
	width float32
}

// slot returns the left and right edge of slot k of the ring, snapped to
// whole pixels so that the bars keep sharp edges.
func (g *framesGraph) slot(k int, width float32) (x0, x1 float32) {
	w := (width - graphGutter) / FrameRing
	return geom.Floor32(graphGutter + float32(k)*w), geom.Floor32(graphGutter + float32(k+1)*w)
}

// slotOf returns the slot of frame i of n.
func slotOf(i, n int) int { return FrameRing - n + i }

// indexAt returns the frame under x, or -1.
func (g *framesGraph) indexAt(x float32) int {
	n := g.ses.Frames()
	if g.width <= graphGutter || x < graphGutter || n == 0 {
		return -1
	}
	w := (g.width - graphGutter) / FrameRing
	i := int((x-graphGutter)/w) - (FrameRing - n)
	if i < 0 || i >= n {
		return -1
	}
	return i
}

// y returns the height in the bars of a time, from the top of the graph.
func (g *framesGraph) y(d time.Duration) float32 {
	return barsHeight - float32(d)/float32(g.top)*barsHeight
}

// Paint implements layout.Painter. It allocates nothing: every shape is a
// rectangle.
func (g *framesGraph) Paint(cv *paint.Canvas, sz geom.Size) {
	g.width = sz.W
	if g.ses == nil || g.top <= 0 {
		return
	}
	c := &g.colors
	n := g.ses.Frames()
	fill := func(x0, y0, x1, y1 float32, col geom.Color) {
		if x1 > x0 && y1 > y0 {
			cv.DrawRRect(geom.RRectAll(geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, 0), paint.Paint{Color: col})
		}
	}
	// The columns of the hovered and the selected frame.
	column := func(i int, col geom.Color) {
		if i >= 0 && i < n {
			x0, x1 := g.slot(slotOf(i, n), sz.W)
			fill(x0, 0, x1, graphHeight, col)
		}
	}
	if g.hover != g.selected {
		column(g.hover, c.hover)
	}
	column(g.selected, c.selected)
	// The reference lines, under the bars.
	right := sz.W
	fill(graphGutter, barsHeight, right, barsHeight+1, c.baseline)
	if budget120 < g.top {
		y := geom.Floor32(g.y(budget120))
		fill(graphGutter, y, right, y+1, c.line120)
	}
	if budget60 < g.top {
		y := geom.Floor32(g.y(budget60))
		fill(graphGutter, y, right, y+1, c.line60)
	}
	cv.Save()
	cv.ClipRect(geom.Rect{X: graphGutter, Y: 0, W: right - graphGutter, H: graphHeight})
	allocScale := float32(0)
	if g.maxAllocs > 0 {
		allocScale = allocBand / float32(math.Log1p(float64(g.maxAllocs)))
	}
	for i := range n {
		f := g.ses.Frame(i)
		x0, x1 := g.slot(slotOf(i, n), sz.W)
		// A pixel between bars when they are wide enough to spare it.
		if x1-x0 >= 3 {
			x1--
		}
		bottom := float32(barsHeight)
		var sum time.Duration
		for s, d := range f.Stages {
			if d <= 0 {
				continue
			}
			sum += d
			top := g.y(sum)
			fill(x0, top, x1, bottom, c.stages[s])
			bottom = top
		}
		if sum > g.top {
			fill(x0, 0, x1, 2, c.gpu)
		}
		if f.GPU > 0 {
			y := g.y(f.GPU)
			fill(x0, y-1, x1, y+1, c.gpu)
		}
		if f.Allocs > 0 {
			h := max(1, float32(math.Log1p(float64(f.Allocs)))*allocScale)
			fill(x0, graphHeight-h, x1, graphHeight, c.allocs)
		}
	}
	cv.Restore()
	// The edge of the selected column, over the bars.
	if g.selected >= 0 && g.selected < n {
		x0, x1 := g.slot(slotOf(g.selected, n), sz.W)
		fill(x0-1, 0, x1+1, 3, c.selectedEdge)
		fill(x0-1, barsHeight+1, x1+1, barsHeight+3, c.selectedEdge)
	}
}
