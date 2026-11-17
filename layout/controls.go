package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

const (
	sliderHeight       = 22
	sliderDefaultWidth = 200
	sliderTrack        = 4
	sliderThumb        = 18

	segmentInset    = 2
	segmentPadding  = 12
	segmentVertical = 3

	scrollBarThickness = 12
	scrollBarThumb     = 6
	scrollBarWide      = 8
	scrollBarMinThumb  = 24
	scrollBarEdge      = 2
)

// RenderSlider draws a slider and turns the pointer into positions along it.
type RenderSlider struct {
	core.RenderBase
	// Fraction is where the thumb is, from 0 to 1.
	Fraction float32
	// Active and Inactive color the track before and after the thumb.
	Active, Inactive geom.Color
	Thumb            geom.Color
	// Edge is the fine line around the thumb, which keeps white on white visible.
	Edge geom.Color
	// Shadows are those of the thumb.
	Shadows []paint.BoxShadow
	// Grow enlarges the thumb, from 0 for none, under the pointer.
	Grow float32
	// Ring is the color of the focus ring. Transparent means that there is none.
	Ring geom.Color
	// OnDrag receives the fraction under the pointer. Nil makes the slider inert.
	OnDrag func(fraction float32)
	// OnDragEnd runs when the pointer lets go.
	OnDragEnd func()

	dragging bool
}

// Layout implements core.RenderObject.
func (r *RenderSlider) Layout(c Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = sliderDefaultWidth
	}
	return c.Constrain(geom.Size{W: w, H: sliderHeight})
}

// span returns the left end and the length of the way of the thumb's center.
func (r *RenderSlider) span() (start, length float32) {
	return sliderThumb / 2, max(r.Size().W-sliderThumb, 0)
}

// Paint implements core.RenderObject.
func (r *RenderSlider) Paint(cv *paint.Canvas, off geom.Offset) {
	start, length := r.span()
	mid := off.Y + r.Size().H/2
	x := off.X + start + length*geom.Clamp32(r.Fraction, 0, 1)
	track := geom.Rect{X: off.X, Y: mid - sliderTrack/2, W: r.Size().W, H: sliderTrack}
	cv.DrawRRect(geom.RRectAll(track, sliderTrack/2), paint.Paint{Color: r.Inactive})
	track.W = x - track.X
	cv.DrawRRect(geom.RRectAll(track, sliderTrack/2), paint.Paint{Color: r.Active})
	radius := sliderThumb / 2 * (1 + r.Grow)
	knob := circle(x, mid, radius)
	if r.Ring.A > 0 {
		cv.DrawRRect(circle(x, mid, radius+ringWidth), paint.Paint{Stroke: &paint.Stroke{Width: ringWidth, Color: r.Ring}})
	}
	for i := range r.Shadows {
		s := &r.Shadows[i]
		cv.DrawShadow(knob, s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(knob, paint.Paint{Color: r.Thumb, Stroke: &paint.Stroke{Width: 0.5, Color: r.Edge}})
}

func circle(x, y, radius float32) geom.RRect {
	return geom.RRectAll(geom.Rect{X: x - radius, Y: y - radius, W: 2 * radius, H: 2 * radius}, radius)
}

// HandlePointer implements core.PointerTarget.
func (r *RenderSlider) HandlePointer(e *core.PointerEvent) {
	if r.OnDrag == nil || e.Kind == core.PointerDown && e.Button != input.ButtonPrimary {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		r.dragging = true
		e.Handled = true
		r.drag(e.Pos.X)
	case core.PointerMove:
		if r.dragging {
			r.drag(e.Pos.X)
		}
	case core.PointerUp, core.PointerCancel:
		if r.dragging {
			r.dragging = false
			if r.OnDragEnd != nil {
				r.OnDragEnd()
			}
		}
	}
}

func (r *RenderSlider) drag(x float32) {
	start, length := r.span()
	if length > 0 {
		r.OnDrag(geom.Clamp32((x-start)/length, 0, 1))
	}
}

// RenderSegmented lays out the choices of a segmented control side by side at one
// width, with a plate that slides under the selected one.
type RenderSegmented struct {
	core.RenderBase
	// Plate is the place of the plate, in segments. It moves between whole numbers.
	Plate float32
	// Track and PlateColor fill the ground and the plate.
	Track, PlateColor geom.Color
	// Shadows are those of the plate.
	Shadows []paint.BoxShadow
	Radius  float32
	// Sink shrinks the plate, from 0 for none, under a press.
	Sink float32
	// OnSelect receives the segment under a press, and OnPress the start and end
	// of the press.
	OnSelect func(i int)
	OnPress  func(down bool)

	children []core.RenderObject
	segment  float32
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderSegmented) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderSegmented) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderSegmented) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *RenderSegmented) Layout(c Constraints) geom.Size {
	var widest, tallest float32
	free := Constraints{MaxW: geom.Inf, MaxH: geom.Inf}
	for _, child := range r.children {
		sz := core.LayoutChild(child, free)
		widest, tallest = max(widest, sz.W), max(tallest, sz.H)
	}
	r.segment = widest + 2*segmentPadding
	height := tallest + 2*segmentVertical + 2*segmentInset
	for i, child := range r.children {
		sz := child.Size()
		child.SetOffset(geom.Offset{
			X: segmentInset + float32(i)*r.segment + (r.segment-sz.W)/2,
			Y: (height - sz.H) / 2,
		})
	}
	return c.Constrain(geom.Size{W: r.segment*float32(len(r.children)) + 2*segmentInset, H: height})
}

// Paint implements core.RenderObject.
func (r *RenderSegmented) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, r.Radius), paint.Paint{Color: r.Track})
	height := sz.H - 2*segmentInset
	plate := geom.RRectAll(geom.Rect{
		X: off.X + segmentInset + r.Plate*r.segment + r.segment*r.Sink/2,
		Y: off.Y + segmentInset + height*r.Sink/2,
		W: r.segment * (1 - r.Sink),
		H: height * (1 - r.Sink),
	}, r.Radius-segmentInset)
	for i := range r.Shadows {
		s := &r.Shadows[i]
		cv.DrawShadow(plate, s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(plate, paint.Paint{Color: r.PlateColor})
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}

// HandlePointer implements core.PointerTarget.
func (r *RenderSegmented) HandlePointer(e *core.PointerEvent) {
	if r.OnSelect == nil || r.segment <= 0 {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		i := int((e.Pos.X - segmentInset) / r.segment)
		r.OnSelect(min(max(i, 0), len(r.children)-1))
		r.press(true)
		e.Handled = true
	case core.PointerUp, core.PointerCancel:
		r.press(false)
	}
}

func (r *RenderSegmented) press(down bool) {
	if r.OnPress != nil {
		r.OnPress(down)
	}
}

// ScrollMetrics is what a scroll bar knows of its view, filled in by layout.
type ScrollMetrics struct {
	// View and Content are the lengths of the view and of what it shows.
	// They are float64, as ScrollOffset is, for content of millions of rows.
	View, Content float64
}

// scrollable reports whether there is anything to scroll.
func (m *ScrollMetrics) scrollable() bool { return m.Content > m.View+0.5 }

// RenderScrollBar shows where a view is, and lets the pointer move it.
type RenderScrollBar struct {
	core.RenderBase
	Axis Axis
	// Reverse is that of the view: offset 0 puts the thumb at the right end,
	// or at the bottom.
	Reverse  bool
	Position *ScrollOffset
	Metrics  *ScrollMetrics
	// Color is the thumb, and Opacity the fade of the whole bar.
	Color   geom.Color
	Opacity float32
	// Wide draws the thumb thicker, for when the pointer is over the bar.
	Wide bool
	// OnDrag receives the scroll offset that the pointer asks for.
	OnDrag func(offset float64)
	// OnScroll receives the wheel, which would miss the view under the bar.
	OnScroll func(notches float32)
	// OnHover reports the pointer entering and leaving the bar.
	OnHover func(inside bool)

	dragging bool
	// grab is where the pointer took hold of the thumb, from its start.
	grab float32
}

// Layout implements core.RenderObject.
func (r *RenderScrollBar) Layout(c Constraints) geom.Size { return c.Biggest() }

// along returns the length of the bar.
func (r *RenderScrollBar) along() float32 {
	if r.Axis == Vertical {
		return r.Size().H
	}
	return r.Size().W
}

// thumb returns the start and length of the thumb along the bar.
func (r *RenderScrollBar) thumb() (start, length float32) {
	m := r.Metrics
	bar := r.along()
	length = min(max(bar*float32(m.View/m.Content), scrollBarMinThumb), bar)
	limit := m.Content - m.View
	f := geom.Clamp32(float32(r.Position.Value()/limit), 0, 1)
	if r.Reverse {
		f = 1 - f
	}
	return (bar - length) * f, length
}

// HitTest implements core.RenderObject. Without anything to scroll the bar is
// not there.
func (r *RenderScrollBar) HitTest(pos geom.Offset, result *core.HitTestResult) bool {
	return r.Metrics.scrollable() && r.RenderBase.HitTest(pos, result)
}

// HandleHover implements core.HoverTarget.
func (r *RenderScrollBar) HandleHover(inside bool) {
	if r.OnHover != nil {
		r.OnHover(inside)
	}
}

// Paint implements core.RenderObject.
func (r *RenderScrollBar) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Opacity <= 0 || !r.Metrics.scrollable() {
		return
	}
	start, length := r.thumb()
	thick := float32(scrollBarThumb)
	if r.Wide {
		thick = scrollBarWide
	}
	rect := geom.Rect{X: off.X + r.Size().W - thick - scrollBarEdge, Y: off.Y + start, W: thick, H: length}
	if r.Axis == Horizontal {
		rect = geom.Rect{X: off.X + start, Y: off.Y + r.Size().H - thick - scrollBarEdge, W: length, H: thick}
	}
	cv.DrawRRect(geom.RRectAll(rect, thick/2), paint.Paint{Color: r.Color.WithAlpha(r.Color.A * r.Opacity)})
}

// HandlePointer implements core.PointerTarget.
func (r *RenderScrollBar) HandlePointer(e *core.PointerEvent) {
	pos := e.Pos.Y
	if r.Axis == Horizontal {
		pos = e.Pos.X
	}
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		r.dragging = true
		start, length := r.thumb()
		r.grab = pos - start
		if r.grab < 0 || r.grab > length {
			// A press beside the thumb brings its middle there.
			r.grab = length / 2
			r.drag(pos)
		}
	case core.PointerMove:
		if r.dragging {
			r.drag(pos)
		}
	case core.PointerUp, core.PointerCancel:
		r.dragging = false
	case core.PointerScroll:
		if r.OnScroll == nil {
			return
		}
		if n := wheelNotches(r.Axis, r.Reverse, e.Scroll); n != 0 {
			r.OnScroll(n)
			e.Handled = true
		}
	}
}

func (r *RenderScrollBar) drag(pos float32) {
	_, length := r.thumb()
	room := r.along() - length
	if room > 0 && r.OnDrag != nil {
		m := r.Metrics
		f := geom.Clamp32((pos-r.grab)/room, 0, 1)
		if r.Reverse {
			f = 1 - f
		}
		r.OnDrag(float64(f) * (m.Content - m.View))
	}
}

// RenderScrollFrame lays a scroll bar over the edge of a view. Its children are
// the view and the bar.
type RenderScrollFrame struct {
	core.RenderBase
	Axis     Axis
	children []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderScrollFrame) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderScrollFrame) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderScrollFrame) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *RenderScrollFrame) Layout(c Constraints) geom.Size {
	sz := core.LayoutChild(r.children[0], c)
	bar := geom.Size{W: scrollBarThickness, H: sz.H}
	off := geom.Offset{X: sz.W - scrollBarThickness}
	if r.Axis == Horizontal {
		bar = geom.Size{W: sz.W, H: scrollBarThickness}
		off = geom.Offset{Y: sz.H - scrollBarThickness}
	}
	core.LayoutChild(r.children[1], geom.Tight(bar))
	r.children[1].SetOffset(off)
	return sz
}

// Paint implements core.RenderObject.
func (r *RenderScrollFrame) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, c := range r.children {
		c.Paint(cv, off.Add(c.Offset()))
	}
}

// RenderCheckmark draws the tick of a checkbox, from the start of its stroke up
// to Progress.
type RenderCheckmark struct {
	core.RenderBase
	Color geom.Color
	// Progress runs from 0, which draws nothing, to 1.
	Progress float32

	mark paint.Path
}

// Layout implements core.RenderObject.
func (r *RenderCheckmark) Layout(c Constraints) geom.Size { return c.Biggest() }

// Paint implements core.RenderObject. The tick has a short and a long arm, of a
// third and two thirds of the stroke.
func (r *RenderCheckmark) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Progress <= 0 {
		return
	}
	s := r.Size().W
	ax, ay := 0.24*s, 0.52*s
	bx, by := 0.42*s, 0.70*s
	cx, cy := 0.76*s, 0.32*s
	p := &r.mark
	p.Reset()
	p.MoveTo(ax, ay)
	const elbow = 1.0 / 3
	if t := r.Progress / elbow; t < 1 {
		p.LineTo(ax+(bx-ax)*t, ay+(by-ay)*t)
	} else {
		t := (r.Progress - elbow) / (1 - elbow)
		p.LineTo(bx, by)
		p.LineTo(bx+(cx-bx)*t, by+(cy-by)*t)
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.DrawPath(p, paint.Paint{Stroke: &paint.Stroke{Width: 1.75, Color: r.Color}})
	cv.Restore()
}
