package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// RenderRepaintBoundary keeps the picture of its child in a layer of its own.
// A change below it records that layer again and nothing else, and a change
// above it reuses the layer as it is. See docs/ADR/0019-layers.md.
type RenderRepaintBoundary struct {
	RenderBase
	child RenderObject
	layer paint.Layer
	// stale means that the layer does not show the child as it is.
	stale bool
	// blend combines the layer with what is below it, as one picture.
	blend paint.BlendMode
	// blur is the standard deviation of the blur of the layer as one picture.
	blur float32
	// transparency is one minus the opacity of the layer as one picture, so
	// that the zero value is opaque.
	transparency float32
	// filter and blurs are the blur of the group, which the canvas holds by
	// pointer, and region its region relative to the child.
	filter paint.Filter
	blurs  [1]paint.FilterPrimitive
	region geom.Rect
	// cache keeps the picture of the group on the GPU while the layer and
	// the filter stay the same.
	cache paint.GroupCache
}

var _ SingleChildRenderObject = (*RenderRepaintBoundary)(nil)

// HitTestSelf implements SelfHitTester. The boundary is hit only over its child.
func (*RenderRepaintBoundary) HitTestSelf(geom.Offset) bool { return false }

// NewRenderRepaintBoundary returns a boundary without a child.
func NewRenderRepaintBoundary() *RenderRepaintBoundary {
	r := &RenderRepaintBoundary{stale: true}
	r.repaint = r
	return r
}

// SetChild implements SingleChildRenderObject.
func (r *RenderRepaintBoundary) SetChild(child RenderObject) {
	r.child = child
	r.MarkNeedsLayout()
}

// ChildCount implements RenderObject.
func (r *RenderRepaintBoundary) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements RenderObject.
func (r *RenderRepaintBoundary) ChildAt(int) RenderObject { return r.child }

// Layout implements RenderObject.
func (r *RenderRepaintBoundary) Layout(c geom.Constraints) geom.Size {
	if r.child == nil {
		return c.Constrain(geom.Size{})
	}
	return LayoutChild(r.child, c)
}

// Blend returns the mode that SetBlend set.
func (r *RenderRepaintBoundary) Blend() paint.BlendMode { return r.blend }

// SetBlend makes the boundary draw its child as one picture, faded by the
// opacity in effect and combined with what is below by mode, as a group of the
// canvas. BlendSrcOver, the default, draws it as the child would draw itself.
// See docs/ADR/0044-drawing-extensions.md and
// docs/ADR/0114-gpu-layers-masks-filters.md.
func (r *RenderRepaintBoundary) SetBlend(mode paint.BlendMode) {
	if r.blend == mode {
		return
	}
	r.blend = mode
	// The mode belongs to the picture of the parent, which draws the layer.
	switch {
	case r.parent != nil:
		r.parent.MarkNeedsPaint()
	case r.tree != nil:
		r.tree.needsPaint = true
	}
}

// Blur returns what SetBlur set.
func (r *RenderRepaintBoundary) Blur() float32 { return r.blur }

// SetBlur makes the boundary draw its child as one picture blurred with the
// standard deviation sigma, in logical pixels, as filter: blur() does in CSS, in
// sRGB. Zero, the default, draws it sharp. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
func (r *RenderRepaintBoundary) SetBlur(sigma float32) {
	sigma = max(sigma, 0)
	if r.blur == sigma {
		return
	}
	r.blur = sigma
	// The blur belongs to the picture of the parent, which draws the layer.
	switch {
	case r.parent != nil:
		r.parent.MarkNeedsPaint()
	case r.tree != nil:
		r.tree.needsPaint = true
	}
}

// Opacity returns what SetOpacity set, 1 by default.
func (r *RenderRepaintBoundary) Opacity() float32 { return 1 - r.transparency }

// SetOpacity makes the boundary draw its child as one picture faded to a, from
// 0 to 1, as opacity does in CSS: where shapes of the child overlap, the lower
// one does not show through. The layer of the child stays as it is, so a fade
// records nothing again from frame to frame, which is what a page transition
// wants. 1, the default, draws the layer as usual.
func (r *RenderRepaintBoundary) SetOpacity(a float32) {
	t := 1 - min(max(a, 0), 1)
	if r.transparency == t {
		return
	}
	r.transparency = t
	// The opacity belongs to the picture of the parent, which draws the layer.
	switch {
	case r.parent != nil:
		r.parent.MarkNeedsPaint()
	case r.tree != nil:
		r.tree.needsPaint = true
	}
}

// Paint implements RenderObject. Where a layer cannot stand in for the child, as
// under a scale transition, the child draws directly, and its changes reach the
// parent until a layer can take over again. A blended, blurred or faded boundary
// draws as a group of the canvas (paint.Canvas.PushGroup), which composites it as
// one picture wherever it is.
func (r *RenderRepaintBoundary) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child == nil || r.transparency >= 1 {
		return
	}
	if r.transparency > 0 || r.blend != paint.BlendSrcOver || r.blur > 0 {
		r.paintGroup(cv, off)
		return
	}
	r.paintChild(cv, off)
}

// paintChild draws the layer, or the child where the layer cannot stand in.
func (r *RenderRepaintBoundary) paintChild(cv *paint.Canvas, off geom.Offset) {
	if !cv.CanDrawLayer() {
		r.repaint, r.stale = nil, true
		r.child.Paint(cv, off)
		return
	}
	r.repaint = r
	r.flush()
	cv.DrawLayer(&r.layer, off)
}

// flush records the layer if it is out of date.
func (r *RenderRepaintBoundary) flush() {
	if !r.stale || r.repaint == nil || r.child == nil {
		return
	}
	r.stale = false
	r.child.Paint(r.layer.Begin(), geom.Offset{})
	r.layer.End()
	// The picture of a group around the layer changes with it, also when the
	// list that holds the group is kept.
	r.cache.Invalidate()
	if probing && r.tree != nil && r.tree.probe != nil {
		r.tree.probe.Recorded(r)
	}
}

// paintGroup draws the child as one picture: blurred, faded and blended as a
// group. The group starts at full opacity and source-over, so that inside it
// the layer can still stand in for the child. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
func (r *RenderRepaintBoundary) paintGroup(cv *paint.Canvas, off geom.Offset) {
	cv.Save()
	// The kept picture of the group moves with the translation at PushGroup.
	cv.Translate(off.X, off.Y)
	off = geom.Offset{}
	cv.PushOpacity(1 - r.transparency)
	style := paint.GroupStyle{Blend: r.blend, Cache: &r.cache}
	if r.blur > 0 {
		style.Filter = &r.filter
	}
	cv.PushGroup(style)
	r.paintChild(cv, off)
	if r.repaint == nil {
		// The child drew itself.
		r.cache.Invalidate()
	}
	if r.blur > 0 {
		r.setFilter(off)
	}
	cv.PopGroup()
	cv.Restore()
}

// setFilter sets the blur of the group for a child at off: its region is what
// the child draws, grown by three standard deviations. It runs after the child
// has painted, which brings the bounds of the layer up to date, and before the
// canvas hands the group on.
func (r *RenderRepaintBoundary) setFilter(off geom.Offset) {
	sz := r.Size()
	local := geom.Rect{W: sz.W, H: sz.H}
	if !r.layer.Empty() {
		local = union(local, r.layer.Bounds())
	}
	m := geom.Ceil32(3*r.blur) + 1
	local = geom.Rect{X: local.X - m, Y: local.Y - m, W: local.W + 2*m, H: local.H + 2*m}
	if local != r.region || r.blurs[0].StdDev.X != r.blur {
		// Moving the child moves the region with it; anything else changes
		// the picture.
		r.region = local
		r.cache.Invalidate()
	}
	r.blurs[0] = paint.FilterPrimitive{Kind: paint.FilterBlur, SRGB: true, StdDev: geom.Offset{X: r.blur, Y: r.blur}}
	r.filter = paint.Filter{Region: local.Translate(off), Primitives: r.blurs[:]}
}

// union returns the bounds of a and b.
func union(a, b geom.Rect) geom.Rect {
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return geom.Rect{X: x, Y: y, W: max(a.X+a.W, b.X+b.W) - x, H: max(a.Y+a.H, b.Y+b.H) - y}
}
