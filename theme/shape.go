package theme

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// The spaces of the design, named for their size in logical pixels: Space2,
// Space4, Space6, Space8, Space10, Space12, Space16, Space20, Space24 and
// Space28. All of them are even, which lands them on whole physical pixels at
// 100, 150 and 200 percent.
const (
	Space2  = 2
	Space4  = 4
	Space6  = 6
	Space8  = 8
	Space10 = 10
	Space12 = 12
	Space16 = 16
	Space20 = 20
	Space24 = 24
	Space28 = 28
)

// ControlHeight is the height of a regular control.
const ControlHeight = 30

// Radii are the corners of the design.
type Radii struct {
	// Check rounds a checkbox, Control a button, an input or a row, Group a card,
	// and Dialog what floats.
	Check, Control, Group, Dialog float32
}

// NewRadii returns the corners of docs/DESIGN.md §4.
func NewRadii() Radii {
	return Radii{Check: 4, Control: 8, Group: 12, Dialog: 16}
}

// Lift is a height above the page, which Colors.Shadows turns into a shadow:
// LiftNone, LiftControl, LiftKnob or LiftFloat, from low to high.
type Lift uint8

// The heights, from low to high: LiftNone, LiftControl, LiftKnob and
// LiftFloat.
const (
	// LiftNone casts nothing.
	LiftNone Lift = iota
	// LiftControl is a button on the page.
	LiftControl
	// LiftKnob is a small part that moves, such as the thumb of a switch.
	LiftKnob
	// LiftFloat is what floats above the page, such as a dialog.
	LiftFloat
)

// Shadows returns the layers of a height: light comes from straight above, and a
// thin contact shadow sits under a wide, faint one, never one dark blur. The
// slice is new on each call, and is what basic.Container and the Decoration of
// package layout take as Shadows.
func (c *Colors) Shadows(l Lift) []paint.BoxShadow {
	layer := func(y, blur, strength float32) paint.BoxShadow {
		return paint.BoxShadow{Offset: geom.Offset{Y: y}, Blur: blur, Color: c.Shadow.WithAlpha(c.Shadow.A * strength)}
	}
	switch l {
	case LiftControl:
		return []paint.BoxShadow{layer(0.5, 1, 0.3)}
	case LiftKnob:
		return []paint.BoxShadow{layer(0.5, 1, 0.5), layer(1, 3, 1)}
	case LiftFloat:
		return []paint.BoxShadow{layer(2, 4, 0.5), layer(24, 48, 1)}
	}
	return nil
}
