package theme

import "github.com/bankai-tensei/gowaku/geom"

// The palettes behind the colors. See docs/DESIGN.md §2.
const (
	// neutralHue and neutralChroma read as no color at all, a hair on the cool
	// side. True gray sits near hue 209 and chroma 2.8 in HCT: a lower chroma turns
	// pink, not grayer, and any warmth turns beige.
	neutralHue    = 235
	neutralChroma = 3.5
	// minAccentChroma keeps a dull seed from looking like a mistake.
	minAccentChroma = 40
	dangerHue       = 25
	dangerChroma    = 70
	successHue      = 145
	successChroma   = 48
	// warningHue is a bright amber, between the red of Danger and the green of
	// Success, so that the three tell apart also by hue.
	warningHue    = 72
	warningChroma = 80
	// lightGlass and darkGlass are the opacities of Glass: the least that
	// keeps Ink2 at 4.5:1 over the darkest surface of the theme under the
	// Scrim, an accent fill, rounded up.
	lightGlass = 0.85
	darkGlass  = 0.78
)

// Colors holds the colors of a theme, named for what they do. Everything comes
// from one accent color, and the surfaces do not depend on it.
type Colors struct {
	// Canvas is the ground of a window, Sidebar that of a sidebar, Card that of a
	// group, and Raised that of what floats above the page, such as a dialog.
	Canvas, Sidebar, Card, Raised geom.Color
	// Glass is Raised made translucent, for a surface over a blurred backdrop
	// such as Dialog{Glass: true}. It is opaque enough that Ink and Ink2 read
	// at 4.5:1 over any surface of the theme under the Scrim, and Ink over
	// anything at all (ADR 0044).
	Glass geom.Color
	// Field is the ground of an input. It is opaque.
	Field geom.Color
	// Control is the ground of an ordinary button.
	Control geom.Color
	// Ink is text, and Ink2 supporting text. Both read at 4.5:1 or more, as
	// WCAG 2 asks, on every surface, also under Fill.
	//
	// Ink3 is faint on purpose, for placeholders, disabled controls and
	// decoration such as the thumb of a scroll bar. It stays below 4.5:1, so
	// keep it from text that tells something, such as a time or a count: use
	// Ink2. A third level lighter than Ink2 that still reads at 4.5:1 does
	// not exist, since Ink2 is near the limit already (ADR 0041).
	Ink, Ink2, Ink3 geom.Color
	// Hair is a line.
	Hair geom.Color
	// Fill and FillStrong are quiet grounds that work on any surface.
	Fill, FillStrong geom.Color
	// Accent fills controls, under OnAccent, which reads on it at 4.5:1 also
	// under a hover. AccentInk is the accent as text or a thin line: in the
	// dark, what reads on the page is too light to carry white. Like Ink2, it
	// reads at 4.5:1 on every surface, also under Fill (ADR 0123).
	Accent, OnAccent, AccentInk geom.Color
	// AccentTint marks what is selected. Ink reads on it, also under Fill;
	// the other inks for text take their colors from OnTint there.
	AccentTint geom.Color
	// Danger is the color of destructive actions and of errors. It reads as
	// text at 4.5:1 on every surface, also under Fill (ADR 0123).
	Danger geom.Color
	// Success marks what went well or is under way, such as a finished step or
	// a running break. It reads as text at 4.5:1 on every surface, also under
	// Fill, and as a fill such as a ring; in the light theme it also carries
	// white text.
	Success geom.Color
	// Warning asks for attention before something goes wrong, such as a disk
	// nearly full or a setting that takes effect after a restart. It is a
	// bright amber for icons, dots, borders and tints, and unlike Danger and
	// Success it is not meant for text: on a light surface it reads at less
	// than 4.5:1 (ADR 0124). Set the words of a warning in Ink beside it.
	Warning geom.Color
	// Shadow is the color of shadows, and Scrim what covers a page behind a dialog.
	Shadow, Scrim geom.Color
}

// NewColors returns the colors of an accent.
func NewColors(accent geom.Color, b Brightness) Colors {
	h := HCTFromColor(accent)
	a := TonalPalette{Hue: h.Hue, Chroma: max(h.Chroma, minAccentChroma)}
	n := TonalPalette{Hue: neutralHue, Chroma: neutralChroma}
	red := TonalPalette{Hue: dangerHue, Chroma: dangerChroma}
	green := TonalPalette{Hue: successHue, Chroma: successChroma}
	amber := TonalPalette{Hue: warningHue, Chroma: warningChroma}
	white, black := geom.Hex(0xffffff), geom.Hex(0)
	if b == BrightnessDark {
		return Colors{
			Canvas: n.Tone(10), Sidebar: n.Tone(14), Card: n.Tone(15), Raised: n.Tone(20),
			Glass: n.Tone(20).WithAlpha(darkGlass),
			Field: n.Tone(19), Control: white.WithAlpha(0.14),
			Ink: n.Tone(95), Ink2: n.Tone(72), Ink3: n.Tone(50),
			Hair: white.WithAlpha(0.10), Fill: white.WithAlpha(0.08), FillStrong: white.WithAlpha(0.14),
			Accent: a.Tone(45), OnAccent: white, AccentInk: a.Tone(74), AccentTint: a.Tone(62).WithAlpha(0.22),
			Danger: red.Tone(72), Success: green.Tone(72), Warning: amber.Tone(76), Shadow: black.WithAlpha(0.5), Scrim: black.WithAlpha(0.4),
		}
	}
	return Colors{
		Canvas: n.Tone(96.5), Sidebar: n.Tone(93), Card: white, Raised: white, Glass: white.WithAlpha(lightGlass),
		Field: white, Control: white,
		Ink: n.Tone(13), Ink2: n.Tone(42), Ink3: n.Tone(62),
		Hair: black.WithAlpha(0.09), Fill: black.WithAlpha(0.05), FillStrong: black.WithAlpha(0.09),
		Accent: a.Tone(45), OnAccent: white, AccentInk: a.Tone(41), AccentTint: a.Tone(45).WithAlpha(0.12),
		Danger: red.Tone(41), Success: green.Tone(41), Warning: amber.Tone(70), Shadow: black.WithAlpha(0.16), Scrim: black.WithAlpha(0.25),
	}
}

// Over returns layer composited at opacity over base.
func Over(base, layer geom.Color, opacity float32) geom.Color {
	a := layer.A * opacity
	out := a + base.A*(1-a)
	if out <= 0 {
		return geom.Color{}
	}
	mix := func(l, b float32) float32 { return (l*a + b*base.A*(1-a)) / out }
	return geom.Color{R: mix(layer.R, base.R), G: mix(layer.G, base.G), B: mix(layer.B, base.B), A: out}
}
