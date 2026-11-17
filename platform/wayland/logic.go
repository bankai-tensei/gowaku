package wayland

import (
	"math"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform/internal/frame"
)

// The buttons of linux/input-event-codes.h.
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
	// btnSide and btnExtra are the thumb buttons of most mice, and btnForward
	// and btnBack those of the mice whose driver names them so.
	btnSide    = 0x113
	btnExtra   = 0x114
	btnForward = 0x115
	btnBack    = 0x116
)

// buttonOf maps an evdev button. ok is false for buttons gowaku ignores.
func buttonOf(code uint32) (b input.Button, ok bool) {
	switch code {
	case btnLeft:
		return input.ButtonPrimary, true
	case btnRight:
		return input.ButtonSecondary, true
	case btnMiddle:
		return input.ButtonMiddle, true
	case btnSide, btnBack:
		return input.ButtonBack, true
	case btnExtra, btnForward:
		return input.ButtonForward, true
	}
	return 0, false
}

// The axes of wl_pointer.
const (
	axisVertical   = 0
	axisHorizontal = 1
)

// continuousPerNotch is how many surface units of a smooth scroll make one
// notch of a wheel. It is what libinput reports for a wheel click.
const continuousPerNotch = 15

// scrollAccumulator gathers the axis events of one wl_pointer frame.
type scrollAccumulator struct {
	// value is the continuous motion of each axis, and v120 the wheel motion in
	// 120ths of a notch. Wheels send both, and v120 wins.
	value [2]float64
	v120  [2]int32
	has   [2]bool
	wheel [2]bool
}

func (s *scrollAccumulator) axis(axis uint32, value float64) {
	if axis > axisHorizontal {
		return
	}
	s.value[axis] += value
	s.has[axis] = true
}

// discrete takes axis_discrete (before version 8) as 120ths.
func (s *scrollAccumulator) discrete(axis uint32, steps int32) { s.value120(axis, steps*120) }

func (s *scrollAccumulator) value120(axis uint32, v int32) {
	if axis > axisHorizontal {
		return
	}
	s.v120[axis] += v
	s.has[axis], s.wheel[axis] = true, true
}

// take returns the motion in notches, with y positive away from the user as
// platform.Event wants, and resets. ok is false if nothing moved.
func (s *scrollAccumulator) take() (x, y float32, ok bool) {
	notches := func(axis int) float64 {
		if s.wheel[axis] {
			return float64(s.v120[axis]) / 120
		}
		return s.value[axis] / continuousPerNotch
	}
	ok = s.has[0] || s.has[1]
	x, y = float32(notches(axisHorizontal)), float32(-notches(axisVertical))
	*s = scrollAccumulator{}
	return x, y, ok
}

// physicalSize returns the buffer size of a logical size at scale, as
// fractional-scale-v1 rounds it.
func physicalSize(logical int32, scale float64) int {
	return int(math.Round(float64(logical) * scale))
}

// fractionalScale converts the preferred scale of wp_fractional_scale_v1.
func fractionalScale(v120 uint32) float64 {
	if v120 == 0 {
		return 1
	}
	return float64(v120) / 120
}

// edge is a resize_edge of xdg_toplevel.
type edge uint32

// The edges.
const (
	edgeNone        edge = 0
	edgeTop         edge = 1
	edgeBottom      edge = 2
	edgeLeft        edge = 4
	edgeTopLeft     edge = 5
	edgeBottomLeft  edge = 6
	edgeRight       edge = 8
	edgeTopRight    edge = 9
	edgeBottomRight edge = 10
)

// edgeAt returns the edge of a w×h window that resizes at (x, y), all logical.
func edgeAt(x, y, w, h float64) edge { return edge(frame.At(x, y, w, h)) }

// cursorShape is a shape of wp_cursor_shape_device_v1.
type cursorShape uint32

// The shapes gowaku uses, from cursor-shape-v1.xml. This file builds on every
// system for its tests, and the generated constants only on Linux.
const (
	shapeDefault   cursorShape = 1
	shapePointer   cursorShape = 4
	shapeText      cursorShape = 9
	shapeEResize   cursorShape = 18
	shapeNResize   cursorShape = 19
	shapeNEResize  cursorShape = 20
	shapeNWResize  cursorShape = 21
	shapeSResize   cursorShape = 22
	shapeSEResize  cursorShape = 23
	shapeSWResize  cursorShape = 24
	shapeWResize   cursorShape = 25
	shapeColResize cursorShape = 30
	shapeRowResize cursorShape = 31
)

func shapeOf(c input.Cursor) cursorShape {
	switch c {
	case input.CursorIBeam:
		return shapeText
	case input.CursorHand:
		return shapePointer
	case input.CursorColResize:
		return shapeColResize
	case input.CursorRowResize:
		return shapeRowResize
	}
	return shapeDefault
}

func edgeShape(e edge) cursorShape {
	switch e {
	case edgeTop:
		return shapeNResize
	case edgeBottom:
		return shapeSResize
	case edgeLeft:
		return shapeWResize
	case edgeRight:
		return shapeEResize
	case edgeTopLeft:
		return shapeNWResize
	case edgeTopRight:
		return shapeNEResize
	case edgeBottomLeft:
		return shapeSWResize
	case edgeBottomRight:
		return shapeSEResize
	}
	return shapeDefault
}

// cursorName returns the name of a shape in cursor themes, for the compositors
// without cursor-shape-v1.
func cursorName(s cursorShape) string {
	switch s {
	case shapePointer:
		return "pointer"
	case shapeText:
		return "text"
	case shapeNResize:
		return "n-resize"
	case shapeSResize:
		return "s-resize"
	case shapeWResize:
		return "w-resize"
	case shapeEResize:
		return "e-resize"
	case shapeNWResize:
		return "nw-resize"
	case shapeNEResize:
		return "ne-resize"
	case shapeSWResize:
		return "sw-resize"
	case shapeSEResize:
		return "se-resize"
	case shapeColResize:
		return "col-resize"
	case shapeRowResize:
		return "row-resize"
	}
	return "default"
}

// legacyCursorName is the older name of the same cursor, which themes from
// before the CSS names have.
func legacyCursorName(s cursorShape) string {
	switch s {
	case shapePointer:
		return "hand2"
	case shapeText:
		return "xterm"
	case shapeNResize:
		return "top_side"
	case shapeSResize:
		return "bottom_side"
	case shapeWResize:
		return "left_side"
	case shapeEResize:
		return "right_side"
	case shapeNWResize:
		return "top_left_corner"
	case shapeNEResize:
		return "top_right_corner"
	case shapeSWResize:
		return "bottom_left_corner"
	case shapeSEResize:
		return "bottom_right_corner"
	case shapeColResize:
		return "sb_h_double_arrow"
	case shapeRowResize:
		return "sb_v_double_arrow"
	}
	return "left_ptr"
}
