package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// ScrollOffset is the position of a scrolling view. The widget's State owns it
// and moves it, and the render object repaints, without any rebuild.
//
// The offset is a float64: a float32 counts whole pixels only up to 2^24, about
// half a million rows of 32, and a list of ten million would move in steps of
// a row. Positions inside the view stay float32, as the rest of layout is.
type ScrollOffset struct {
	value float64
	view  scrollView
}

// scrollView is the render object that shows a ScrollOffset.
type scrollView interface {
	offsetChanged()
}

// Value returns how far the content has moved.
func (s *ScrollOffset) Value() float64 { return s.value }

// Set moves the content.
func (s *ScrollOffset) Set(v float64) {
	if s.value != v {
		s.value = v
		if s.view != nil {
			s.view.offsetChanged()
		}
	}
}

// RenderViewport shows part of a child that may be larger than itself, along
// one axis.
type RenderViewport struct {
	RenderProxy
	Axis Axis
	// Reverse puts the start of the scroll range at the right end, or at the
	// bottom: Position counts from there, the child lines up with that end,
	// and a child shorter than the view sits at it. The wheel still moves the
	// content the way it turns.
	Reverse bool
	// Position is the scroll offset, from the start of the scroll range.
	Position *ScrollOffset
	// OnScroll receives the wheel, in notches, positive toward the end of the
	// scroll range: right or down, or left or up with Reverse.
	OnScroll func(notches float32)
	// OnMeasured reports the size of the view and of the child after a layout.
	OnMeasured func(view, content float64)
	// OnJump moves the view to offset at once for code that brings a part of
	// the child into view, such as basic.ScrollIntoView, through the owner of
	// Position so that a motion under way stops. Nil sets Position.
	OnJump func(offset float64)

	// lead is where the child starts at offset 0 along the axis: 0, or with
	// Reverse the length of the view less that of the child.
	lead float32
}

// The panics of a viewport whose room is unbounded, by its Axis. They are
// constants so that Layout does not build them.
var (
	unboundedHeight = [...]string{
		Horizontal: "gowaku: horizontal Scroll has an unbounded height, as inside a vertical Scroll; give it a height with a SizedBox",
		Vertical:   "gowaku: Scroll has an unbounded height, as inside another vertical Scroll or a Column; give it a height with a SizedBox or an Expanded",
	}
	unboundedWidth = [...]string{
		Horizontal: "gowaku: horizontal Scroll has an unbounded width, as inside another horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded",
		Vertical:   "gowaku: Scroll has an unbounded width, as inside a horizontal Scroll; give it a width with a SizedBox",
	}
)

// Attach connects the object to its position. Call it after creation.
func (r *RenderViewport) Attach() { r.Position.view = r }

func (r *RenderViewport) offsetChanged() {
	r.place()
	r.MarkNeedsPaint()
}

func (r *RenderViewport) place() {
	if r.child == nil {
		return
	}
	at := float32(-r.Position.value)
	if r.Reverse {
		at = float32(float64(r.lead) + r.Position.value)
	}
	if r.Axis == Vertical {
		r.child.SetOffset(geom.Offset{Y: at})
	} else {
		r.child.SetOffset(geom.Offset{X: at})
	}
}

// Layout implements core.RenderObject. The child gets all the room it wants
// along the axis. The view itself takes all the room it is given, so both
// lengths must be bounded: a Scroll inside a Scroll of the other direction, or
// inside a Row or Column along its axis, panics unless something such as a
// SizedBox or an Expanded bounds it.
func (r *RenderViewport) Layout(c Constraints) geom.Size {
	if c.MaxH >= geom.Inf {
		panic(unboundedHeight[r.Axis])
	}
	if c.MaxW >= geom.Inf {
		panic(unboundedWidth[r.Axis])
	}
	sz := geom.Size{W: c.MaxW, H: c.MaxH}
	if r.child == nil {
		return c.Constrain(sz)
	}
	inner := Constraints{MaxW: c.MaxW, MaxH: geom.Inf}
	if r.Axis == Horizontal {
		inner = Constraints{MaxW: geom.Inf, MaxH: c.MaxH}
	}
	content := core.LayoutChild(r.child, inner)
	sz = c.Constrain(sz)
	view, length := sz.H, content.H
	if r.Axis == Horizontal {
		view, length = sz.W, content.W
	}
	r.lead = 0
	if r.Reverse {
		r.lead = view - length
	}
	r.place()
	if r.OnMeasured != nil {
		r.OnMeasured(float64(view), float64(length))
	}
	return sz
}

// ChildClip implements core.Clipper.
func (r *RenderViewport) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *RenderViewport) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// HandlePointer implements core.PointerTarget.
func (r *RenderViewport) HandlePointer(e *core.PointerEvent) {
	if e.Kind != core.PointerScroll || r.OnScroll == nil {
		return
	}
	if n := wheelNotches(r.Axis, r.Reverse, e.Scroll); n != 0 {
		r.OnScroll(n)
		e.Handled = true
	}
}

// wheelNotches returns the notches of scroll along axis, positive toward the
// end of the scroll range, or 0 if scroll does not move along it. The wheel
// turning away from the user moves the content down, whichever end the range
// starts at.
func wheelNotches(axis Axis, reverse bool, scroll geom.Offset) float32 {
	n := scroll.X
	if axis == Vertical {
		n = -scroll.Y
	}
	if reverse {
		return -n
	}
	return n
}

// RenderList shows a column of rows, of which only those near the view exist.
// Child k has the index First+k. The rows have the same height, Extent, unless
// Extents is set, which gives each row its own, or Measured is set, which lets
// each row take the height it lays itself out at.
type RenderList struct {
	core.RenderBase
	// Position is the scroll offset.
	Position *ScrollOffset
	// Extent is the height of a child when Extents is nil.
	Extent float32
	// Extents, when not nil, holds the height of each row. Set it with
	// SetExtents, which notices when the table has been reset.
	Extents *ExtentTable
	// Measured, when not nil and Extents is nil, lets each row be as tall as
	// it lays itself out at the width of the list, and keeps the heights. The
	// rows not laid out yet count as its estimate. While rows take their
	// heights, the first row in view stays where it was on screen: Position
	// moves instead, and OnShift reports by how much.
	Measured *MeasuredExtents
	// First is the index of the first child.
	First int
	// OnScroll receives the wheel, in notches, positive toward the end.
	OnScroll func(notches float32)
	// OnMeasured reports the height of the view after a layout.
	OnMeasured func(view float32)
	// OnShift reports that a layout moved Position by delta, so that the rows
	// in view stay where they are while rows take their measured heights. A
	// motion under way should move by delta too.
	OnShift func(delta float64)

	children []core.RenderObject
	// version is the Version of Extents at the last SetExtents.
	version uint64

	// The rest serves Measured. base is the top of the first child, and width
	// and view the size the heights were measured at.
	base        float64
	width, view float32
	// anchor is one more than the first row in view when the rows were last
	// placed, or 0 for none, and anchorTop its top relative to the view.
	anchor    int
	anchorTop float32
	// revealing keeps row reveal at revealAlign in the view, from Reveal until
	// StopRevealing.
	revealing   bool
	reveal      int
	revealAlign float32
}

// Reveal keeps row i at align in the view while Measured learns the heights of
// the rows around it: align 0 puts its top at the top of the view, 1 its
// bottom at the bottom, and values between in proportion. It holds over the
// layouts that follow, so that a jump to a row whose neighbors were never
// measured lands on it, until StopRevealing. It does nothing without Measured.
func (r *RenderList) Reveal(i int, align float32) {
	r.revealing, r.reveal, r.revealAlign = true, i, geom.Clamp32(align, 0, 1)
	r.MarkNeedsLayout()
}

// StopRevealing ends a Reveal, as when the user scrolls; from then on the rows
// in view stay where they are.
func (r *RenderList) StopRevealing() { r.revealing = false }

// SetExtents sets Extents, and marks the list for layout if the table is
// another one or has been reset since the last call.
func (r *RenderList) SetExtents(t *ExtentTable) {
	var v uint64
	if t != nil {
		v = t.Version()
	}
	if r.Extents != t || r.version != v {
		r.Extents, r.version = t, v
		r.MarkNeedsLayout()
	}
}

// Attach connects the object to its position. Call it after creation.
func (r *RenderList) Attach() { r.Position.view = r }

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderList) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderList) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderList) ChildAt(i int) core.RenderObject { return r.children[i] }

func (r *RenderList) offsetChanged() {
	r.place()
	r.MarkNeedsPaint()
}

func (r *RenderList) place() {
	if r.Extents == nil && r.Measured != nil {
		r.placeMeasured()
		return
	}
	if t := r.Extents; t != nil {
		for k, c := range r.children {
			c.SetOffset(geom.Offset{Y: float32(t.Offset(r.First+k) - r.Position.value)})
		}
		return
	}
	for k, c := range r.children {
		c.SetOffset(geom.Offset{Y: float32(float64(r.First+k)*float64(r.Extent) - r.Position.value)})
	}
}

// Layout implements core.RenderObject. Like RenderViewport, it takes all the
// room it is given, and panics if a length is unbounded.
func (r *RenderList) Layout(c Constraints) geom.Size {
	if c.MaxH >= geom.Inf {
		panic("gowaku: collection.List has an unbounded height, as inside a vertical Scroll or a Column; give it a height with a SizedBox or an Expanded")
	}
	if c.MaxW >= geom.Inf {
		panic("gowaku: collection.List has an unbounded width, as inside a horizontal Scroll or a Row; give it a width with a SizedBox or an Expanded")
	}
	sz := c.Constrain(geom.Size{W: c.MaxW, H: c.MaxH})
	if r.Extents == nil && r.Measured != nil {
		r.layoutMeasured(sz)
	} else if t := r.Extents; t != nil {
		for k, child := range r.children {
			core.LayoutChild(child, geom.Tight(geom.Size{W: sz.W, H: t.Extent(r.First + k)}))
		}
	} else {
		item := geom.Tight(geom.Size{W: sz.W, H: r.Extent})
		for _, child := range r.children {
			core.LayoutChild(child, item)
		}
	}
	r.place()
	if r.OnMeasured != nil {
		r.OnMeasured(sz.H)
	}
	return sz
}

// ChildClip implements core.Clipper.
func (r *RenderList) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *RenderList) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	for _, c := range r.children {
		c.Paint(cv, off.Add(c.Offset()))
	}
	cv.Restore()
}

// HandlePointer implements core.PointerTarget.
func (r *RenderList) HandlePointer(e *core.PointerEvent) {
	if e.Kind == core.PointerScroll && e.Scroll.Y != 0 && r.OnScroll != nil {
		r.OnScroll(-e.Scroll.Y)
		e.Handled = true
	}
}

// layoutMeasured lays the rows out at the width of the list and any height,
// records their heights, and moves Position so that the row in view stays
// where it was on screen, or the revealed row is where it was asked to be.
func (r *RenderList) layoutMeasured(sz geom.Size) {
	m := r.Measured
	n := m.Len()
	pos := r.Position.value
	// The row to keep in place is chosen before the heights change: the
	// first row in view when the rows were last placed, or, when none of the
	// rows there is in view any more, as after a jump, the row at the offset.
	anchor, top := -1, float64(0)
	switch {
	case r.anchor > 0 && r.anchor <= n:
		anchor, top = r.anchor-1, float64(r.anchorTop)
	case n > 0:
		anchor = m.IndexAt(pos)
		top = m.Offset(anchor) - pos
	}
	if sz.W != r.width {
		// Wrapped text takes other heights at another width.
		m.ForgetAll()
		r.width = sz.W
	}
	r.view = sz.H
	row := geom.Constraints{MinW: sz.W, MaxW: sz.W, MaxH: geom.Inf}
	for k, child := range r.children {
		if i := r.First + k; i < n {
			m.Set(i, core.LayoutChild(child, row).H)
		} else {
			core.LayoutChild(child, row)
		}
	}
	if r.revealing && r.reveal >= n {
		r.revealing = false
	}
	want := pos
	switch {
	case r.revealing:
		i := r.reveal
		want = m.Offset(i) - float64(r.revealAlign*(sz.H-m.Extent(i)))
		want = min(max(want, 0), max(m.Total()-float64(sz.H), 0))
	case anchor >= 0:
		want = m.Offset(anchor) - top
	}
	r.base = m.Offset(min(r.First, n))
	if want != pos {
		r.Position.Set(want)
		if r.OnShift != nil {
			r.OnShift(want - pos)
		}
	}
}

// placeMeasured places the rows one under the other from the top of the
// first, and notes the first row in view.
func (r *RenderList) placeMeasured() {
	pos := r.Position.value
	y := r.base
	r.anchor = 0
	for k, c := range r.children {
		top := float32(y - pos)
		c.SetOffset(geom.Offset{Y: top})
		y += float64(c.Size().H)
		if r.anchor == 0 && y-pos > 0 && top < r.view {
			r.anchor, r.anchorTop = r.First+k+1, top
		}
	}
}
