package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

// rangeDecide is how far, in logical pixels, a press on two thumbs at one
// place has to move before its direction picks the thumb.
const rangeDecide = 1

// RangePart is what a press on a RenderRangeSlider took hold of.
type RangePart uint8

// The parts of a range slider.
const (
	// RangeLow is the thumb of the low end.
	RangeLow RangePart = iota
	// RangeHigh is the thumb of the high end.
	RangeHigh
	// RangeBar is the stretch of track between the thumbs, which moves both.
	RangeBar
)

// String returns the name of the part.
func (p RangePart) String() string {
	switch p {
	case RangeLow:
		return "low"
	case RangeHigh:
		return "high"
	case RangeBar:
		return "bar"
	}
	return "unknown"
}

// RenderRangeSlider draws a slider with two thumbs, the low and the high end of
// a range, with the track, thumbs, shadows and focus rings of RenderSlider, and
// turns the pointer into new places for them. A press on a thumb drags it, a
// press between the thumbs drags both, and a press elsewhere brings the nearer
// thumb there. A drag never lets the thumbs cross: each stops Gap short of the
// other.
//
// Its children, at most two, are laid out over the thumbs at the size of a
// thumb, the low one first: a widget puts the focus and the semantics of each
// thumb there. They paint after the thumbs.
type RenderRangeSlider struct {
	core.RenderBase
	// Low and High are the places of the thumbs, from 0 to 1, Low not above
	// High.
	Low, High float32
	// Mirrored puts 0 at the right end, for a right-to-left layout.
	Mirrored bool
	// Snap, above zero, is the grid a drag snaps the places to, as a share of
	// the track.
	Snap float32
	// Gap is the least distance a drag keeps between the places.
	Gap float32
	// Active colors the track between the thumbs, and Inactive the rest.
	Active, Inactive geom.Color
	Thumb            geom.Color
	// Edge is the fine line around the thumbs.
	Edge geom.Color
	// Shadows are those of a thumb.
	Shadows []paint.BoxShadow
	// Grow enlarges each thumb, the low one first, as RenderSlider.Grow does.
	Grow [2]float32
	// Ring is the color of the focus ring of each thumb. Transparent means that
	// there is none.
	Ring [2]geom.Color
	// OnDrag receives the places during a drag, each time one changes. Nil
	// makes the slider inert.
	OnDrag func(low, high float32)
	// OnDragStart runs when a press takes hold of a part. A press on two
	// thumbs at one place takes the low one, and the high one instead if it
	// then moves up the range.
	OnDragStart func(part RangePart)
	// OnDragEnd runs when the pointer lets go.
	OnDragEnd func()

	children []core.RenderObject
	// part is what the drag holds, and grab the distance from the pointer to
	// the place of the part when it was taken. undecided says that two
	// thumbs at one place await the direction of the drag.
	part      RangePart
	grab      float32
	downX     float32
	dragging  bool
	undecided bool
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderRangeSlider) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderRangeSlider) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderRangeSlider) ChildAt(i int) core.RenderObject { return r.children[i] }

// Dragging reports whether a press holds a part, and which.
func (r *RenderRangeSlider) Dragging() (RangePart, bool) { return r.part, r.dragging }

// Layout implements core.RenderObject.
func (r *RenderRangeSlider) Layout(c Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = sliderDefaultWidth
	}
	sz := c.Constrain(geom.Size{W: w, H: sliderHeight})
	r.placeChildren(sz)
	return sz
}

// placeChildren puts the children over the thumbs.
func (r *RenderRangeSlider) placeChildren(sz geom.Size) {
	start, length := sliderSpan(sz.W)
	for i, child := range r.children {
		if i > 1 {
			break
		}
		f := r.Low
		if i == 1 {
			f = r.High
		}
		core.LayoutChild(child, geom.Tight(geom.Size{W: sliderThumb, H: sliderThumb}))
		child.SetOffset(geom.Offset{X: r.xOf(start, length, f) - sliderThumb/2, Y: (sz.H - sliderThumb) / 2})
	}
}

// sliderSpan returns the left end and the length of the way of a thumb's
// center along a slider of width w.
func sliderSpan(w float32) (start, length float32) {
	return sliderThumb / 2, max(w-sliderThumb, 0)
}

// xOf returns the position of the center of a thumb at place f.
func (r *RenderRangeSlider) xOf(start, length, f float32) float32 {
	f = geom.Clamp32(f, 0, 1)
	if r.Mirrored {
		f = 1 - f
	}
	return start + length*f
}

// placeAt returns the place under the position x.
func (r *RenderRangeSlider) placeAt(x float32) float32 {
	start, length := sliderSpan(r.Size().W)
	if length <= 0 {
		return 0
	}
	f := geom.Clamp32((x-start)/length, 0, 1)
	if r.Mirrored {
		f = 1 - f
	}
	return f
}

// Paint implements core.RenderObject.
func (r *RenderRangeSlider) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	start, length := sliderSpan(sz.W)
	mid := off.Y + sz.H/2
	lx := off.X + r.xOf(start, length, r.Low)
	hx := off.X + r.xOf(start, length, r.High)
	track := geom.Rect{X: off.X, Y: mid - sliderTrack/2, W: sz.W, H: sliderTrack}
	cv.DrawRRect(geom.RRectAll(track, sliderTrack/2), paint.Paint{Color: r.Inactive})
	track.X, track.W = min(lx, hx), max(lx, hx)-min(lx, hx)
	cv.DrawRRect(geom.RRectAll(track, sliderTrack/2), paint.Paint{Color: r.Active})
	// The thumb a drag holds comes last, on top of the other.
	if r.dragging && r.part == RangeLow {
		r.paintThumb(cv, hx, mid, 1)
		r.paintThumb(cv, lx, mid, 0)
	} else {
		r.paintThumb(cv, lx, mid, 0)
		r.paintThumb(cv, hx, mid, 1)
	}
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// paintThumb draws thumb i as RenderSlider draws its one.
func (r *RenderRangeSlider) paintThumb(cv *paint.Canvas, x, y float32, i int) {
	radius := sliderThumb / 2 * (1 + r.Grow[i])
	knob := circle(x, y, radius)
	if ring := r.Ring[i]; ring.A > 0 {
		cv.DrawRRect(circle(x, y, radius+ringWidth), paint.Paint{Stroke: &paint.Stroke{Width: ringWidth, Color: ring}})
	}
	for j := range r.Shadows {
		s := &r.Shadows[j]
		cv.DrawShadow(knob, s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(knob, paint.Paint{Color: r.Thumb, Stroke: &paint.Stroke{Width: 0.5, Color: r.Edge}})
}

// HandlePointer implements core.PointerTarget.
func (r *RenderRangeSlider) HandlePointer(e *core.PointerEvent) {
	if r.OnDrag == nil || e.Kind == core.PointerDown && e.Button != input.ButtonPrimary {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		e.Handled = true
		r.press(e.Pos.X)
	case core.PointerMove:
		if !r.dragging {
			return
		}
		if r.undecided {
			if d := e.Pos.X - r.downX; d > rangeDecide || d < -rangeDecide {
				r.undecided = false
				if up := (d > 0) != r.Mirrored; up {
					r.take(RangeHigh, r.placeAt(r.downX)-r.High)
				}
			} else {
				return
			}
		}
		r.drag(e.Pos.X)
	case core.PointerUp, core.PointerCancel:
		if r.dragging {
			r.dragging, r.undecided = false, false
			r.MarkNeedsPaint()
			if r.OnDragEnd != nil {
				r.OnDragEnd()
			}
		}
	}
}

// press takes hold of the part under x.
func (r *RenderRangeSlider) press(x float32) {
	start, length := sliderSpan(r.Size().W)
	lx, hx := r.xOf(start, length, r.Low), r.xOf(start, length, r.High)
	dl, dh := geom.Abs32(x-lx), geom.Abs32(x-hx)
	f := r.placeAt(x)
	r.dragging, r.undecided, r.downX = true, false, x
	const reach = sliderThumb / 2
	switch {
	case dl <= reach || dh <= reach:
		switch {
		case geom.Abs32(lx-hx) < 0.5:
			// Two thumbs at one place: the drag's direction decides.
			r.undecided = true
			r.take(RangeLow, f-r.Low)
		case dl <= dh:
			r.take(RangeLow, f-r.Low)
		default:
			r.take(RangeHigh, f-r.High)
		}
	case x > min(lx, hx) && x < max(lx, hx):
		r.take(RangeBar, f-r.Low)
	default:
		// Outside the range: the nearer thumb comes to the pointer.
		if dl <= dh {
			r.take(RangeLow, 0)
		} else {
			r.take(RangeHigh, 0)
		}
		r.drag(x)
	}
	r.MarkNeedsPaint()
}

// take makes part the one the drag holds.
func (r *RenderRangeSlider) take(part RangePart, grab float32) {
	r.part, r.grab = part, grab
	if r.OnDragStart != nil {
		r.OnDragStart(part)
	}
}

// drag moves the part held to the pointer at x.
func (r *RenderRangeSlider) drag(x float32) {
	f := r.placeAt(x) - r.grab
	low, high := r.Low, r.High
	switch r.part {
	case RangeLow:
		low = geom.Clamp32(r.snap(f), 0, max(high-r.Gap, 0))
	case RangeHigh:
		high = geom.Clamp32(r.snap(f), min(low+r.Gap, 1), 1)
	case RangeBar:
		width := high - low
		low = geom.Clamp32(r.snap(f), 0, max(1-width, 0))
		high = low + width
	}
	if low == r.Low && high == r.High {
		return
	}
	r.Low, r.High = low, high
	r.MarkNeedsLayout()
	r.MarkNeedsPaint()
	r.OnDrag(low, high)
}

// snap puts f on the grid of Snap.
func (r *RenderRangeSlider) snap(f float32) float32 {
	if r.Snap > 0 {
		f = geom.Floor32(f/r.Snap+0.5) * r.Snap
	}
	return geom.Clamp32(f, 0, 1)
}
