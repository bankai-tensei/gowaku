package field

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// This file holds what Calendar, DatePicker and TimePicker share: the dates
// they count with, the icons they draw and face, the label of their parts.

// civil returns the day of t as midnight UTC, which the pickers count with so
// that no zone or summer time can shift a day.
func civil(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysIn returns the number of days of month m of year y.
func daysIn(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

// addMonths moves the civil day d by n months, keeping its day of the month
// where the new month has it and taking the last day otherwise: January 31
// plus a month is February 28 or 29.
func addMonths(d time.Time, n int) time.Time {
	m := int(d.Month()) - 1 + n
	y := d.Year() + floorDiv(m, 12)
	month := time.Month(m - floorDiv(m, 12)*12 + 1)
	return time.Date(y, month, min(d.Day(), daysIn(y, month)), 0, 0, 0, 0, time.UTC)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// in returns the civil day d as midnight in the zone of like, or UTC when like
// is zero, which is the time the pickers hand to OnChange.
func in(d, like time.Time) time.Time {
	loc := time.UTC
	if !like.IsZero() {
		loc = like.Location()
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
}

// calendarPath returns the calendar icon of a date field on the 24 unit grid:
// a page with a band at the top and two rings.
func calendarPath() *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	p.AddRRect(geom.RRectAll(geom.Rect{X: 3, Y: 5, W: 18, H: 16}, 3))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 5, Y: 10, W: 14, H: 9}, 1.5))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 7, Y: 2, W: 2, H: 3}, 0.8))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 15, Y: 2, W: 2, H: 3}, 0.8))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 7, Y: 12, W: 3, H: 3}, 0.8))
	return p
}

// focusHandle is Focus that hands its render object to the state that builds
// it, so that the arrows can move the keyboard between the parts of a picker,
// and that reports whether the focus shows.
type focusHandle struct {
	ref           **layout.RenderFocus
	enabled       bool
	autofocus     bool
	onKey         func(e *core.KeyEvent) bool
	onFocusChange func(focused bool)
	onShowFocus   func(show bool)
	child         core.Widget
}

// Key implements core.Widget.
func (focusHandle) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f focusHandle) CreateRenderObject() core.RenderObject {
	r := &layout.RenderFocus{Autofocus: f.autofocus}
	f.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f focusHandle) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFocus)
	r.Enabled, r.OnKey, r.OnFocusChange, r.OnShowFocus = f.enabled, f.onKey, f.onFocusChange, f.onShowFocus
	if f.ref != nil {
		*f.ref = r
	}
}

// ChildWidget implements core.SingleChildWidget.
func (f focusHandle) ChildWidget() core.Widget { return f.child }

// face is a short line of text on an optional ground, for the parts of the
// pickers, a day of a calendar or a part of a time field, and for the
// placeholders of the fields that are not text fields. Unlike Text it says
// nothing to assistive technology, because the Semantics around it speaks for
// it in the words a reader needs rather than the digits it shows: a day says
// the whole date, not "9".
type face struct {
	shaper  *text.Shaper
	content string
	style   text.Style
	// width and height fix the size; zero takes that of the text.
	width, height float32
	// start puts the text at the left edge instead of the middle.
	start  bool
	ground geom.Color
	radius float32
	// strike crosses the text out, as for a day that cannot be chosen.
	strike bool
	// dots are drawn in a centered row under the text, those of zero alpha
	// left out, as the marks of a day.
	dots [calendarMarks]geom.Color
}

// Key implements core.Widget.
func (face) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f face) CreateRenderObject() core.RenderObject {
	r := &renderFace{}
	f.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f face) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderFace)
	if r.shaper != f.shaper || r.content != f.content || !r.style.SameLayout(f.style) {
		r.shaper, r.content, r.style = f.shaper, f.content, f.style
		r.shaped, r.laidOut = nil, nil
		r.width, r.height, r.start = f.width, f.height, f.start
		r.MarkNeedsLayout()
	} else if r.width != f.width || r.height != f.height || r.start != f.start {
		r.width, r.height, r.start = f.width, f.height, f.start
		r.MarkNeedsLayout()
	}
	if r.style.PaintColor() != f.style.PaintColor() || r.ground != f.ground || r.radius != f.radius ||
		r.strike != f.strike || r.dots != f.dots {
		r.style = f.style
		r.ground, r.radius, r.strike, r.dots = f.ground, f.radius, f.strike, f.dots
		r.MarkNeedsPaint()
	}
}

// renderFace is the render object of face. It shapes its text once and paints
// without allocating.
type renderFace struct {
	core.RenderBase
	shaper        *text.Shaper
	content       string
	style         text.Style
	width, height float32
	start         bool
	ground        geom.Color
	radius        float32
	strike        bool
	dots          [calendarMarks]geom.Color

	shaped  *text.Paragraph
	laidOut *text.Layout
}

// Layout implements core.RenderObject.
func (r *renderFace) Layout(c geom.Constraints) geom.Size {
	if r.shaped == nil {
		r.shaped = r.shaper.Shape(r.content, r.style)
		r.laidOut = r.shaped.Layout(geom.Inf)
	}
	sz := r.laidOut.Size()
	if r.width > 0 {
		sz.W = r.width
	}
	if r.height > 0 {
		sz.H = r.height
	}
	return c.Constrain(sz)
}

// Paint implements core.RenderObject.
func (r *renderFace) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if r.ground.A > 0 {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, r.radius), paint.Paint{Color: r.ground})
	}
	ts := r.laidOut.Size()
	origin := geom.Offset{Y: geom.Floor32((sz.H-ts.H)/2 + 0.5)}
	if !r.start {
		origin.X = geom.Floor32((sz.W-ts.W)/2 + 0.5)
	}
	color := r.style.PaintColor()
	runs := r.laidOut.Runs()
	// A text wider than the face, as a long placeholder in a narrow field, is
	// cut at its edge.
	clip := ts.W > sz.W
	if clip {
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	}
	for i := range runs {
		cv.DrawGlyphs(&runs[i], off.Add(origin), color)
	}
	if clip {
		cv.Restore()
	}
	if r.strike {
		line := geom.Rect{X: off.X + origin.X - 2, Y: off.Y + geom.Floor32(sz.H/2), W: ts.W + 4, H: 1}
		cv.DrawRRect(geom.RRectAll(line, 0), paint.Paint{Color: color})
	}
	r.paintDots(cv, off, sz)
}

// paintDots draws the dots in a row centered under the text.
func (r *renderFace) paintDots(cv *paint.Canvas, off geom.Offset, sz geom.Size) {
	n := 0
	for _, d := range r.dots {
		if d.A > 0 {
			n++
		}
	}
	if n == 0 {
		return
	}
	row := float32(n)*calendarDot + float32(n-1)*calendarDotGap
	x := off.X + geom.Floor32((sz.W-row)/2+0.5)
	y := off.Y + sz.H - calendarDotBottom - calendarDot/2
	for _, d := range r.dots {
		if d.A <= 0 {
			continue
		}
		dot := geom.Rect{X: x, Y: y, W: calendarDot, H: calendarDot}
		cv.DrawRRect(geom.RRectAll(dot, calendarDot/2), paint.Paint{Color: d})
		x += calendarDot + calendarDotGap
	}
}
