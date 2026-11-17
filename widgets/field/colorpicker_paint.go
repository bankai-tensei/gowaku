package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of the parts of a color picker.
const (
	// pickerStrip is the height of a strip, which is that of its thumb, and
	// pickerTrack the height of the colored pill inside it.
	pickerStrip = 18
	pickerTrack = 12
	// pickerPlaneThumb and pickerStripThumb are the radii of the thumbs.
	pickerPlaneThumb = 8
	pickerStripThumb = pickerStrip / 2
	// pickerThumbBorder is the white ring of a thumb around its color.
	pickerThumbBorder = 2.5
	// pickerRingGap is the room between a thumb and its focus ring.
	pickerRingGap = 2
	// pickerRingWidth is the width of the focus ring around a thumb.
	pickerRingWidth = 3
	// pickerChecker is the side of a square of the checkerboard behind
	// translucent colors.
	pickerChecker = 6
)

// thumbLook is how the thumbs of a part look: the shadow and edge of the
// knob, and the focus ring, transparent when there is none.
type thumbLook struct {
	shadows []paint.BoxShadow
	edge    paint.Stroke
	ring    paint.Stroke
}

// checker holds the two colors of the checkerboard behind translucent colors.
type checker struct {
	light, dark geom.Color
}

// checkerOf returns the checkerboard of a theme: quiet grays that read as
// "transparent" in light and dark alike.
func checkerOf(th *theme.Theme) checker {
	if th.Brightness == theme.BrightnessDark {
		return checker{light: geom.Hex(0x5a5a5e), dark: geom.Hex(0x3a3a3e)}
	}
	return checker{light: geom.Hex(0xffffff), dark: geom.Hex(0xd9d9de)}
}

// paint fills rr with the checkerboard. The squares line up with the corner
// of rr, so the pattern stays put under the color as it changes.
func (k *checker) paint(cv *paint.Canvas, rr geom.RRect) {
	r := rr.Rect
	cv.Save()
	cv.ClipRRect(rr)
	cv.DrawRRect(geom.RRect{Rect: r}, paint.Paint{Color: k.light})
	for row := float32(0); row*pickerChecker < r.H; row++ {
		odd := int(row)%2 == 1
		for col := float32(0); col*pickerChecker < r.W; col++ {
			if odd == (int(col)%2 == 0) {
				continue
			}
			cell := geom.Rect{X: r.X + col*pickerChecker, Y: r.Y + row*pickerChecker, W: pickerChecker, H: pickerChecker}
			cell.W = min(cell.W, r.X+r.W-cell.X)
			cell.H = min(cell.H, r.Y+r.H-cell.Y)
			cv.DrawRRect(geom.RRect{Rect: cell}, paint.Paint{Color: k.dark})
		}
	}
	cv.Restore()
}

// circleAt returns the circle around c.
func circleAt(c geom.Offset, radius float32) geom.RRect {
	return geom.RRectAll(geom.Rect{X: c.X - radius, Y: c.Y - radius, W: 2 * radius, H: 2 * radius}, radius)
}

// paintThumb draws a knob at c: a white disc with a shadow and a hairline,
// showing fill inside its ring, and the focus ring around it.
func paintThumb(cv *paint.Canvas, c geom.Offset, radius float32, fill geom.Color, look *thumbLook) {
	if look.ring.Color.A > 0 {
		cv.DrawRRect(circleAt(c, radius+pickerRingGap+pickerRingWidth/2), paint.Paint{Stroke: &look.ring})
	}
	knob := circleAt(c, radius)
	for i := range look.shadows {
		s := &look.shadows[i]
		cv.DrawShadow(knob, s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(knob, paint.Paint{Color: geom.Hex(0xffffff), Stroke: &look.edge})
	cv.DrawRRect(circleAt(c, radius-pickerThumbBorder), paint.Paint{Color: fill})
}

// pickerDrag turns the pointer into positions on a part of a color picker,
// from the press on until the release.
type pickerDrag struct {
	dragging bool
}

// handle follows e and calls move with the local position while the pointer
// drags, and end when it lets go. It reports whether e started a drag.
func (d *pickerDrag) handle(e *core.PointerEvent, move func(pos geom.Offset), end func()) {
	if move == nil || e.Kind == core.PointerDown && e.Button != input.ButtonPrimary {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		d.dragging = true
		e.Handled = true
		move(e.Pos)
	case core.PointerMove:
		if d.dragging {
			move(e.Pos)
		}
	case core.PointerUp, core.PointerCancel:
		if d.dragging {
			d.dragging = false
			if end != nil {
				end()
			}
		}
	}
}

// renderColorPlane draws the square of saturation, across, and brightness,
// up, of one hue, with a thumb on the current color.
type renderColorPlane struct {
	core.RenderBase
	hue, sat, val float32
	// height is the height of the plane; the width is what it is allowed.
	height, radius float32
	border         paint.Stroke
	look           thumbLook
	// onDrag gets the saturation and brightness under the pointer.
	onDrag func(sat, val float32)
	onEnd  func()

	drag         pickerDrag
	white, black paint.Gradient
	move         func(pos geom.Offset)
}

// Layout implements core.RenderObject.
func (r *renderColorPlane) Layout(c geom.Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = colorPickerWidth
	}
	return c.Constrain(geom.Size{W: w, H: r.height})
}

// Paint implements core.RenderObject.
func (r *renderColorPlane) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	rect := geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}
	rr := geom.RRectAll(rect, r.radius)
	// Across, white to the pure hue: the saturation at full brightness. Down,
	// clear to black over it: the brightness. Both blends are exact in sRGB.
	r.white = paint.Gradient{
		Start: geom.Offset{X: rect.X, Y: rect.Y}, End: geom.Offset{X: rect.X + rect.W, Y: rect.Y},
		StartColor: geom.Hex(0xffffff), EndColor: theme.HSV{H: r.hue, S: 1, V: 1, A: 1}.Color(),
	}
	r.black = paint.Gradient{
		Start: geom.Offset{X: rect.X, Y: rect.Y}, End: geom.Offset{X: rect.X, Y: rect.Y + rect.H},
		StartColor: geom.Color{}, EndColor: geom.Hex(0),
	}
	cv.DrawRRect(rr, paint.Paint{Gradient: &r.white})
	cv.DrawRRect(rr, paint.Paint{Gradient: &r.black, Stroke: &r.border})
	c := geom.Offset{X: rect.X + geom.Clamp32(r.sat, 0, 1)*rect.W, Y: rect.Y + (1-geom.Clamp32(r.val, 0, 1))*rect.H}
	paintThumb(cv, c, pickerPlaneThumb, theme.HSV{H: r.hue, S: r.sat, V: r.val, A: 1}.Color(), &r.look)
}

// HandlePointer implements core.PointerTarget.
func (r *renderColorPlane) HandlePointer(e *core.PointerEvent) {
	if r.onDrag == nil {
		return
	}
	if r.move == nil {
		r.move = func(pos geom.Offset) {
			sz := r.Size()
			if sz.W <= 0 || sz.H <= 0 {
				return
			}
			r.onDrag(geom.Clamp32(pos.X/sz.W, 0, 1), 1-geom.Clamp32(pos.Y/sz.H, 0, 1))
		}
	}
	r.drag.handle(e, r.move, r.onEnd)
}

// renderColorStrip draws a strip of hue, or of opacity over a checkerboard,
// with a thumb at value.
type renderColorStrip struct {
	core.RenderBase
	// alpha makes it the strip of opacity of color; otherwise it runs around
	// the hues.
	alpha bool
	// value is the place of the thumb, from 0 to 1.
	value float32
	// color is the current color: the end of the strip of opacity, and what
	// the thumb shows.
	color   geom.Color
	checker checker
	border  paint.Stroke
	look    thumbLook
	onDrag  func(value float32)
	onEnd   func()

	drag pickerDrag
	move func(pos geom.Offset)
	fill paint.Gradient
	// hues are the stops around the circle of hues, made once.
	hues []paint.GradientStop
}

// newHueStops returns the stops of a strip of hue: the six primaries and
// secondaries of sRGB and red again.
func newHueStops() []paint.GradientStop {
	stops := make([]paint.GradientStop, 7)
	for i := range stops {
		stops[i] = paint.GradientStop{Offset: float32(i) / 6, Color: theme.HSV{H: float32(i) * 60, S: 1, V: 1, A: 1}.Color()}
	}
	return stops
}

// Layout implements core.RenderObject.
func (r *renderColorStrip) Layout(c geom.Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = colorPickerWidth
	}
	return c.Constrain(geom.Size{W: w, H: pickerStrip})
}

// span returns where the thumb's center starts and how far it travels: from
// the middle of one round end of the track to that of the other.
func (r *renderColorStrip) span() (start, length float32) {
	return pickerTrack / 2, max(r.Size().W-pickerTrack, 0)
}

// Paint implements core.RenderObject.
func (r *renderColorStrip) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	track := geom.Rect{X: off.X, Y: off.Y + (sz.H-pickerTrack)/2, W: sz.W, H: pickerTrack}
	rr := geom.RRectAll(track, pickerTrack/2)
	start, length := r.span()
	opaque := r.color.WithAlpha(1)
	if r.alpha {
		r.checker.paint(cv, rr)
		r.fill = paint.Gradient{
			Start: geom.Offset{X: track.X + start, Y: track.Y}, End: geom.Offset{X: track.X + start + length, Y: track.Y},
			StartColor: opaque.WithAlpha(0), EndColor: opaque,
		}
	} else {
		if r.hues == nil {
			r.hues = newHueStops()
		}
		r.fill = paint.Gradient{
			Start: geom.Offset{X: track.X + start, Y: track.Y}, End: geom.Offset{X: track.X + start + length, Y: track.Y},
			MoreStops: &r.hues,
		}
	}
	cv.DrawRRect(rr, paint.Paint{Gradient: &r.fill, Stroke: &r.border})
	thumb := r.color
	if !r.alpha {
		thumb = theme.HSV{H: r.value * 360, S: 1, V: 1, A: 1}.Color()
	}
	c := geom.Offset{X: off.X + start + geom.Clamp32(r.value, 0, 1)*length, Y: off.Y + sz.H/2}
	paintThumb(cv, c, pickerStripThumb, thumb, &r.look)
}

// HandlePointer implements core.PointerTarget.
func (r *renderColorStrip) HandlePointer(e *core.PointerEvent) {
	if r.onDrag == nil {
		return
	}
	if r.move == nil {
		r.move = func(pos geom.Offset) {
			start, length := r.span()
			if length > 0 {
				r.onDrag(geom.Clamp32((pos.X-start)/length, 0, 1))
			}
		}
	}
	r.drag.handle(e, r.move, r.onEnd)
}

// renderColorSwatch draws a color over a checkerboard, in a box with the
// corners of rr.
type renderColorSwatch struct {
	core.RenderBase
	color         geom.Color
	width, height float32
	// corners are the radii of the corners, clockwise from the top left.
	corners [4]float32
	checker checker
	border  paint.Stroke
}

// Layout implements core.RenderObject.
func (r *renderColorSwatch) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: r.width, H: r.height})
}

// Paint implements core.RenderObject.
func (r *renderColorSwatch) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	rr := geom.RRect{
		Rect:    geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H},
		TopLeft: r.corners[0], TopRight: r.corners[1], BottomRight: r.corners[2], BottomLeft: r.corners[3],
	}
	if r.color.A < 1 {
		r.checker.paint(cv, rr)
	}
	cv.DrawRRect(rr, paint.Paint{Color: r.color, Stroke: &r.border})
}
