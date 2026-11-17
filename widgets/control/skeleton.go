package control

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures and timing of a skeleton.
const (
	// skeletonLineHeight is the height of a line of placeholder text, about
	// the x-height and a bit of a line of body text.
	skeletonLineHeight = 12
	// skeletonWidth is the width of a line or a block whose parent leaves it
	// unbounded, and skeletonBlock the height of a block.
	skeletonWidth  = 160
	skeletonBlock  = 64
	skeletonCircle = 32
	// skeletonLineGap is the room between the lines of a SkeletonText.
	skeletonLineGap = theme.Space8
	// skeletonLines is the number of lines of a SkeletonText that sets none.
	skeletonLines = 3
	// skeletonPeriod is the time the light takes to cross the window once.
	skeletonPeriod = 1600 * time.Millisecond
	// skeletonBand is the width of the band of light.
	skeletonBand = 240
)

// skeletonLastShare is the width of the last line of a SkeletonText.
const skeletonLastShare = 0.6

// skeletonShare returns the width of line i of a SkeletonText as a share of
// the width it is given. The widths vary a little, in a cycle, as the lines of
// a paragraph do.
func skeletonShare(i int) float32 {
	switch i % 5 {
	case 1:
		return 0.94
	case 2:
		return 0.86
	case 3:
		return 0.97
	case 4:
		return 0.9
	}
	return 1
}

// SkeletonShape is the shape of a Skeleton.
//
// It is SkeletonLine (the zero value), SkeletonBlock or SkeletonCircle.
type SkeletonShape uint8

// The shapes of a skeleton.
const (
	// SkeletonLine stands for a line of text: a pill 12 high and as wide as
	// it may be.
	SkeletonLine SkeletonShape = iota
	// SkeletonBlock stands for an image or a card: a rounded rectangle.
	SkeletonBlock
	// SkeletonCircle stands for an avatar or an icon.
	SkeletonCircle
)

// String returns the name of the shape.
func (s SkeletonShape) String() string {
	switch s {
	case SkeletonLine:
		return "line"
	case SkeletonBlock:
		return "block"
	case SkeletonCircle:
		return "circle"
	}
	return "unknown"
}

// Skeleton is a placeholder for content that is still loading: a quiet shape
// with a band of light that sweeps across it. Wrap the skeletons of one place
// in a SkeletonGroup, which tells a screen reader that it is loading; the
// shapes themselves say nothing.
//
// The light belongs to the window rather than to each shape: all skeletons
// read one clock and one position across the window, so the band passes over
// a card of them as one. It runs only while the shape is painted and seen, so
// a skeleton scrolled out of view, clipped away, under a fully transparent
// Opacity or out of the tree asks for no frames; and it stands still when the
// window asks for less motion.
//
// The zero value is a line as wide as its parent allows.
type Skeleton struct {
	core.StatelessBase
	Shape SkeletonShape
	// Width and Height force a size. Zero selects a line 12 high, a block 64
	// high, or a circle 32 across, as wide as the parent allows for a line or
	// a block. A circle is as wide as it is high.
	Width, Height float32
	// Radius rounds the corners of a line or a block. Zero selects a pill for
	// a line, and the corner of a control for a block.
	Radius float32

	// share, if set, makes the width that share of what the parent allows.
	share float32
}

// Build implements core.StatelessWidget.
func (s Skeleton) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	base := th.Color.FillStrong
	light := base.WithAlpha(base.A * 0.4)
	if th.Brightness == theme.BrightnessDark {
		light = base.WithAlpha(min(base.A*1.8, 1))
	}
	radius := s.Radius
	if radius <= 0 && s.Shape == SkeletonBlock {
		radius = th.Radius.Control
	}
	return skeletonView{s: s, radius: radius, base: base, light: light, still: basic.ReduceMotionOf(ctx)}
}

// SkeletonText stands for a paragraph that is still loading: Lines skeleton
// lines of varied width, the last one shorter.
type SkeletonText struct {
	core.StatelessBase
	// Lines is the number of lines. Zero selects 3.
	Lines int
	// Spacing is the room between lines. Zero selects 8.
	Spacing float32
}

// Build implements core.StatelessWidget.
func (t SkeletonText) Build(core.BuildContext) core.Widget {
	n := t.Lines
	if n <= 0 {
		n = skeletonLines
	}
	gap := t.Spacing
	if gap <= 0 {
		gap = skeletonLineGap
	}
	lines := make([]core.Widget, n)
	for i := range lines {
		share := skeletonShare(i)
		if n > 1 && i == n-1 {
			share = skeletonLastShare
		}
		lines[i] = Skeleton{share: share}
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: gap, Children: lines}
}

// SkeletonGroup holds the skeletons of one place, and tells a screen reader
// that the place is loading: it is a busy indicator named by Label, and hides
// what is inside. Show the real content in its place once it arrives.
type SkeletonGroup struct {
	core.StatelessBase
	// Label names what is loading. Empty says Loading in the language of the
	// window.
	Label string
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (g SkeletonGroup) Build(ctx core.BuildContext) core.Widget {
	label := g.Label
	if label == "" {
		label = basic.StringsOf(ctx).Loading
	}
	return skeletonGroupView{label: label, child: g.Child}
}

// skeletonGroupView is the render widget of a SkeletonGroup.
type skeletonGroupView struct {
	label string
	child core.Widget
}

// Key implements core.Widget.
func (skeletonGroupView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (g skeletonGroupView) CreateRenderObject() core.RenderObject {
	return &renderSkeletonGroup{label: g.label}
}

// UpdateRenderObject implements core.RenderWidget. A label is not painted, so
// nothing follows.
func (g skeletonGroupView) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderSkeletonGroup).label = g.label
}

// ChildWidget implements core.SingleChildWidget.
func (g skeletonGroupView) ChildWidget() core.Widget { return g.child }

// renderSkeletonGroup is the busy node of a SkeletonGroup.
type renderSkeletonGroup struct {
	layout.RenderProxy
	label string
}

// Semantics implements core.RenderObject. The skeletons stand for content
// that is not there yet, so nothing inside is visited.
func (r *renderSkeletonGroup) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleProgress, Label: r.label}, r.Size())
	sb.Close()
}

// skeletonView is the render widget of a Skeleton.
type skeletonView struct {
	s           Skeleton
	radius      float32
	base, light geom.Color
	still       bool
}

// Key implements core.Widget.
func (skeletonView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v skeletonView) CreateRenderObject() core.RenderObject {
	r := &renderSkeleton{}
	v.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v skeletonView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderSkeleton)
	if r.shape != v.s.Shape || r.width != v.s.Width || r.height != v.s.Height || r.share != v.s.share {
		r.MarkNeedsLayout()
	}
	if r.radius != v.radius || r.base != v.base || r.light != v.light || r.still != v.still {
		r.MarkNeedsPaint()
	}
	v.apply(r)
}

func (v skeletonView) apply(r *renderSkeleton) {
	r.shape, r.width, r.height, r.share = v.s.Shape, v.s.Width, v.s.Height, v.s.share
	r.radius, r.base, r.light = v.radius, v.base, v.light
	if v.still && !r.still && r.ticker != nil {
		r.ticker.Stop()
	}
	r.still = v.still
}

// renderSkeleton draws one shape of a Skeleton, and runs the light over it
// while it is seen.
type renderSkeleton struct {
	core.RenderBase
	shape                        SkeletonShape
	width, height, radius, share float32
	base, light                  geom.Color
	// still holds the light, for a window that asks for less motion.
	still bool

	ticker *core.Ticker
	// now is the time of the last tick, which places the light; hasNow says
	// there was one.
	now    time.Duration
	hasNow bool
	// painted says the shape was painted and seen since the last tick. A tick
	// that finds it unset stops the ticker: nobody looks.
	painted bool
	shine   paint.Gradient
}

// Layout implements core.RenderObject.
func (r *renderSkeleton) Layout(c geom.Constraints) geom.Size {
	if r.shape == SkeletonCircle {
		d := r.height
		if d <= 0 {
			d = r.width
		}
		if d <= 0 {
			d = skeletonCircle
		}
		return c.Constrain(geom.Size{W: d, H: d})
	}
	w := r.width
	if w <= 0 {
		w = c.MaxW
		if w >= geom.Inf {
			w = skeletonWidth
		}
		if r.share > 0 {
			w *= r.share
		}
	}
	h := r.height
	if h <= 0 {
		h = skeletonLineHeight
		if r.shape == SkeletonBlock {
			h = skeletonBlock
		}
	}
	return c.Constrain(geom.Size{W: w, H: h})
}

// shapeAt returns the shape with its corner at off.
func (r *renderSkeleton) shapeAt(off geom.Offset) geom.RRect {
	sz := r.Size()
	radius := r.radius
	if r.shape == SkeletonCircle || r.shape == SkeletonLine && radius <= 0 {
		radius = min(sz.W, sz.H) / 2
	}
	return geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, min(radius, min(sz.W, sz.H)/2))
}

// Paint implements core.RenderObject.
func (r *renderSkeleton) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	rr := r.shapeAt(off)
	before := len(cv.DisplayList().Ops)
	if r.still || !r.hasNow {
		cv.DrawRRect(rr, paint.Paint{Color: r.base})
	} else {
		r.placeShine(off)
		cv.DrawRRect(rr, paint.Paint{Gradient: &r.shine})
	}
	if r.still || len(cv.DisplayList().Ops) == before {
		// A canvas that leaves the shape out finds it outside the clip, or
		// transparent: nobody sees the light, so it does not run.
		return
	}
	r.painted = true
	if r.ticker == nil {
		r.ticker = r.NewTicker(r.tick)
	}
	if !r.ticker.Running() {
		r.ticker.Start()
	}
}

// placeShine aims the gradient of the light, in the coordinates of the
// canvas, at where the band is across the window now.
func (r *renderSkeleton) placeShine(off geom.Offset) {
	var root core.RenderObject = r
	for p := r.Parent(); p != nil; p = p.Parent() {
		root = p
	}
	travel := root.Size().W + 2*skeletonBand
	phase := float32(r.now%skeletonPeriod) / float32(skeletonPeriod)
	center := -skeletonBand + travel*phase
	// The band is placed across the window; the canvas draws the shape at off.
	x := center - core.LocalToGlobal(r, geom.Offset{}).X + off.X
	r.shine = paint.Gradient{
		Start: geom.Offset{X: x - skeletonBand/2, Y: off.Y},
		End:   geom.Offset{X: x + skeletonBand/2, Y: off.Y},
		Stops: [paint.MaxGradientStops]paint.GradientStop{
			{Offset: 0, Color: r.base}, {Offset: 0.5, Color: r.light}, {Offset: 1, Color: r.base},
		},
		StopCount: 3,
	}
}

// tick moves the light on, or stops the ticker if nobody saw the shape since
// the last tick.
func (r *renderSkeleton) tick(now time.Duration) {
	if !r.Attached() || r.still || !r.painted {
		r.ticker.Stop()
		return
	}
	r.painted = false
	r.now, r.hasNow = now, true
	r.MarkNeedsPaint()
}
