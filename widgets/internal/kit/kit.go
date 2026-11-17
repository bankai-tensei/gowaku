package kit

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

const (
	// HoverDuration is how fast a control follows the pointer.
	HoverDuration = 80 * time.Millisecond
	// ToggleDuration is how fast colors change between states.
	ToggleDuration = 150 * time.Millisecond
	// RingOpacity is the strength of the focus ring.
	RingOpacity = 0.28
	// KeyKick is the push that a key gives a control, in sinks per second.
	KeyKick = 9
)

// Activates reports whether e is a key that presses the focused control.
func Activates(e *core.KeyEvent) bool {
	return e.Key == input.KeySpace || e.Key == input.KeyEnter
}

// OnOff turns a state into the end of an animation.
func OnOff(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

// Colored returns s in color.
func Colored(s text.Style, color geom.Color) text.Style {
	s.Color = color
	return s
}

// LabelHeight returns the height of one line in style.
func LabelHeight(style *text.Style) float32 { return style.Size * style.LineHeight }

// Rebuild asks for a new Build, unless the state has left the tree.
func Rebuild(s *core.StateBase) {
	if s.Mounted() {
		s.SetState(func() {})
	}
}

// ChevronPath returns a chevron on the 24 unit grid, pointing left or right.
func ChevronPath(right bool) *paint.Path {
	pts := [...]geom.Offset{{X: 15.4, Y: 6.4}, {X: 9.8, Y: 12}, {X: 15.4, Y: 17.6}, {X: 14, Y: 19}, {X: 7, Y: 12}, {X: 14, Y: 5}}
	p := &paint.Path{}
	for i, pt := range pts {
		if right {
			pt.X = 24 - pt.X
		}
		if i == 0 {
			p.MoveTo(pt.X, pt.Y)
		} else {
			p.LineTo(pt.X, pt.Y)
		}
	}
	p.Close()
	return p
}
