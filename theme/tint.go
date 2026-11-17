package theme

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// tintTarget is the contrast that OnTint asks of text on AccentTint: the
// 4.5:1 of WCAG 2 with a little room, so that the 8-bit pixels of a screen,
// which round the composited ground, still reach it.
const tintTarget = 4.6

// tintSteps is how many halvings OnTint takes to find the least mix that
// reads, which leaves it within 1/4096 of the least.
const tintSteps = 12

// OnTint returns the colors for what sits on AccentTint, such as the content
// of a chosen row, a selected chip or a checked toggle. The tint deepens the
// ground, so that Ink2, AccentInk, Danger and Success, which read at 4.5:1 on
// every surface, fall below it there; OnTint mixes each toward Ink, by the
// least that makes it read at 4.5:1 on AccentTint over every surface, also
// under Fill and under the hover wash of a quiet control. What already reads
// keeps its color, and the rest stay as they are: Ink reads on the tint, and
// Ink3 is faint on purpose.
//
// A row that paints AccentTint itself, as a chosen row of a ListView or a
// ReorderableList does, sets its content in these colors with
// basic.Theme{Data: th.OnTint()}, so that the library's text inside it, such
// as a quiet Badge or a hovered quiet Button, reads too. The chosen rows of
// the library, such as SidebarItem and the lit row of a menu, do it
// themselves.
func (c *Colors) OnTint() Colors {
	var grounds [10]float64
	n := 0
	for _, s := range [...]geom.Color{c.Canvas, c.Sidebar, c.Card, c.Raised, c.Field} {
		tint := Over(s, c.AccentTint, 1)
		grounds[n], grounds[n+1] = luminance(tint), luminance(Over(tint, c.Fill, 1))
		n += 2
	}
	out := *c
	out.Ink2 = readableOn(c.Ink2, c.Ink, &grounds)
	out.AccentInk = readableOn(c.AccentInk, c.Ink, &grounds)
	out.Danger = readableOn(c.Danger, c.Ink, &grounds)
	out.Success = readableOn(c.Success, c.Ink, &grounds)
	return out
}

// OnTint returns a copy of t whose colors are t.Color.OnTint(), to set what
// sits on AccentTint in with basic.Theme. The copy shares the extensions of t.
func (t *Theme) OnTint() *Theme {
	out := *t
	out.Color = t.Color.OnTint()
	return &out
}

// readableOn returns ink mixed toward toward by the least that reads at
// tintTarget on every ground, given by luminance. It returns toward when no
// mix reaches it.
func readableOn(ink, toward geom.Color, grounds *[10]float64) geom.Color {
	if worstContrast(luminance(ink), grounds) >= tintTarget {
		return ink
	}
	lo, hi := float32(0), float32(1)
	for range tintSteps {
		mid := (lo + hi) / 2
		if worstContrast(luminance(Over(ink, toward, mid)), grounds) >= tintTarget {
			hi = mid
		} else {
			lo = mid
		}
	}
	return Over(ink, toward, hi)
}

// worstContrast returns the lowest contrast of a color of luminance l on the
// grounds.
func worstContrast(l float64, grounds *[10]float64) float64 {
	worst := math.Inf(1)
	for _, g := range grounds {
		worst = min(worst, contrastOf(l, g))
	}
	return worst
}

// luminance is the WCAG 2 relative luminance of c, ignoring its alpha.
func luminance(c geom.Color) float64 {
	lin := func(v float32) float64 {
		x := float64(v)
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrast is the WCAG 2 contrast ratio of two opaque colors.
func contrast(a, b geom.Color) float64 { return contrastOf(luminance(a), luminance(b)) }

// contrastOf is the WCAG 2 contrast ratio of two luminances.
func contrastOf(x, y float64) float64 {
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}
