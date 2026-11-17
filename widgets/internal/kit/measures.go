package kit

import (
	"time"

	"github.com/bankai-tensei/gowaku/theme"
)

// The measures that controls of one package draw again in another: a table
// paints the checkboxes and switches of its cells itself, a tab bar the focus
// ring of a tab, and so on.
const (
	// DividerThickness is the width of a divider line: one logical pixel,
	// which the renderer keeps to one physical pixel at 150% (DESIGN §4).
	DividerThickness = 1
	// DefaultIconSize is the side of an Icon without a Size.
	DefaultIconSize = 24
	// NotchDistance is how far one notch of the wheel scrolls.
	NotchDistance = 100

	// ButtonSink is how far a press shrinks a button: to 0.97.
	ButtonSink = 0.03
	// HoverStrength and PressStrength darken or lighten a ground.
	HoverStrength = 0.05
	PressStrength = 0.11

	// CheckboxBox is the side of the box of a checkbox.
	CheckboxBox = 16
	// SwitchWidth is the width of the track of a switch, SwitchEdge the room
	// between the track and the thumb, and SwitchThumb the side of the thumb.
	SwitchWidth = 38
	SwitchEdge  = theme.Space2
	SwitchThumb = 18
	// TabRingWidth is the width of the focus ring of a tab, as
	// layout.RenderRing draws it.
	TabRingWidth = 3

	// CaretWidth is the width of the caret of a text field.
	CaretWidth = 1.5
	// CaretBlink is how long the caret stays on or off.
	CaretBlink = 500 * time.Millisecond
	// TargetOpacity is the strength of the accent behind the clause the IME
	// is converting.
	TargetOpacity = 0.18

	// DialogEnter is how long a dialog takes to come in.
	DialogEnter = 280 * time.Millisecond

	// MenuItemHeight is the height of a row of a menu.
	MenuItemHeight = 28
	// MenuInset is the room around the items, inside the card.
	MenuInset = theme.Space4
	// MenuEdge is the room a menu keeps from the edges of the window, and
	// MenuGap the room between a menu and what opened it.
	MenuEdge = theme.Space4
	MenuGap  = theme.Space2
	// PopupMaxRows is how many rows a list of choices shows before it scrolls.
	PopupMaxRows = 8
)
