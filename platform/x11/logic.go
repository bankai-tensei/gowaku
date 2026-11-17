package x11

import (
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/input"
)

// standardDPI is the resolution at which a logical pixel is a physical one.
const standardDPI = 96

// The modifier bits of the state of an event.
const (
	ShiftMask   = 1 << 0
	ControlMask = 1 << 2
	Mod1Mask    = 1 << 3
)

// The buttons of the core protocol that a wheel presses, a notch each.
const wheelUp, wheelDown, wheelLeft, wheelRight = 4, 5, 6, 7

// isWheel reports whether the button detail is a notch of a wheel.
func isWheel(detail uint32) bool { return detail >= wheelUp && detail <= wheelRight }

// buttonOf maps the detail of a ButtonPress. 8 and 9 are the thumb buttons, as
// the X server numbers them after the wheels. ok is false for the wheels and
// for buttons gowaku ignores.
func buttonOf(detail uint32) (b input.Button, ok bool) {
	switch detail {
	case 1:
		return input.ButtonPrimary, true
	case 2:
		return input.ButtonMiddle, true
	case 3:
		return input.ButtonSecondary, true
	case 8:
		return input.ButtonBack, true
	case 9:
		return input.ButtonForward, true
	}
	return 0, false
}

func modifiersOf(state uint32) input.Modifiers {
	var m input.Modifiers
	if state&ShiftMask != 0 {
		m |= input.ModShift
	}
	if state&ControlMask != 0 {
		m |= input.ModControl
	}
	if state&Mod1Mask != 0 {
		m |= input.ModAlt
	}
	return m
}

// scaleOf finds the Xft.dpi resource in the text of RESOURCE_MANAGER, which
// desktops set to say how large things should be. Without it the scale is 1.
func scaleOf(resources string) float32 {
	for line := range strings.SplitSeq(resources, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) != "Xft.dpi" {
			continue
		}
		dpi, err := strconv.ParseFloat(strings.TrimSpace(value), 32)
		if err == nil && dpi > 0 {
			return float32(dpi) / standardDPI
		}
	}
	return 1
}
