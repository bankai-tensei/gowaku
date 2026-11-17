package theme

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// TestColors_InkReadsOnEverySurface pins the promise of Colors: every ink
// meant for text (Ink, Ink2, AccentInk, Danger and Success) reads at
// 4.5:1 on every surface, also under the hover Fill, for accents of every
// kind; OnAccent does on Accent, also under its hover wash; Ink3 is the faint
// one; white reads on Success in the light theme. Warning is not meant for
// text (ADR 0124) and is left out.
func TestColors_InkReadsOnEverySurface(t *testing.T) {
	white := geom.Hex(0xffffff)
	for _, accent := range []geom.Color{
		geom.Hex(defaultAccent), geom.Hex(0x6a4cf0), geom.Hex(0x0a84ff), geom.Hex(0xe0457b), geom.Hex(0x2f7d5b),
		geom.Hex(0xb58900), geom.Hex(0x0f766e), geom.Hex(0xc2410c), geom.Hex(0x7c3aed),
	} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(accent, b)
			surfaces := map[string]geom.Color{}
			for name, s := range map[string]geom.Color{"Canvas": c.Canvas, "Sidebar": c.Sidebar, "Card": c.Card, "Raised": c.Raised, "Field": c.Field} {
				surfaces[name] = s
				surfaces["Fill over "+name] = Over(s, c.Fill, 1)
			}
			inks := map[string]geom.Color{"Ink": c.Ink, "Ink2": c.Ink2, "AccentInk": c.AccentInk, "Danger": c.Danger, "Success": c.Success}
			for name, s := range surfaces {
				for ink, col := range inks {
					if r := contrast(col, s); r < 4.5 {
						t.Errorf("accent %v, brightness %v: %s on %s is %.3f:1, want 4.5:1", accent, b, ink, name, r)
					}
				}
				if r := contrast(c.Ink3, s); r >= 4.5 {
					t.Errorf("accent %v, brightness %v: Ink3 on %s is %.2f:1; it is meant to be faint", accent, b, name, r)
				}
			}
			// A primary button, a chosen day or a badge sets OnAccent on Accent,
			// and hover washes OnAccent into the ground (kit.HoverStrength).
			for name, ground := range map[string]geom.Color{"Accent": c.Accent, "Accent under hover": Over(c.Accent, c.OnAccent, 0.05)} {
				if r := contrast(c.OnAccent, ground); r < 4.5 {
					t.Errorf("accent %v, brightness %v: OnAccent on %s is %.3f:1, want 4.5:1", accent, b, name, r)
				}
			}
			for name, fill := range map[string]geom.Color{"Success": c.Success} {
				if r := contrast(white, fill); b == BrightnessLight && r < 4.5 {
					t.Errorf("accent %v: white on %s is %.3f:1, want 4.5:1", accent, name, r)
				}
			}
		}
	}
}

// TestColors_InkReadsOnGlass pins the promise of Glass: over any surface of the
// theme under the scrim, as behind a dialog, Ink and Ink2 read at 4.5:1; over
// black or white, Ink still does.
func TestColors_InkReadsOnGlass(t *testing.T) {
	for _, accent := range []geom.Color{geom.Hex(0x6a4cf0), geom.Hex(0x0a84ff), geom.Hex(0xe0457b), geom.Hex(0x2f7d5b), geom.Hex(0xb58900)} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(accent, b)
			if c.Glass.A <= 0 || c.Glass.A >= 1 {
				t.Fatalf("brightness %v: Glass %v is not translucent", b, c.Glass)
			}
			backdrops := map[string]geom.Color{
				"Canvas": c.Canvas, "Sidebar": c.Sidebar, "Card": c.Card, "Raised": c.Raised,
				"Accent": c.Accent, "AccentInk": c.AccentInk, "Danger": c.Danger, "Success": c.Success, "Warning": c.Warning,
			}
			for name, s := range backdrops {
				ground := Over(Over(s, c.Scrim, 1), c.Glass, 1)
				for ink, col := range map[string]geom.Color{"Ink": c.Ink, "Ink2": c.Ink2} {
					if r := contrast(col, ground); r < 4.5 {
						t.Errorf("accent %v, brightness %v: %s on Glass over %s is %.2f:1, want 4.5:1", accent, b, ink, name, r)
					}
				}
			}
			for _, s := range []geom.Color{geom.Hex(0), geom.Hex(0xffffff)} {
				if r := contrast(c.Ink, Over(Over(s, c.Scrim, 1), c.Glass, 1)); r < 4.5 {
					t.Errorf("accent %v, brightness %v: Ink on Glass over %v is %.2f:1, want 4.5:1", accent, b, s, r)
				}
			}
		}
	}
}

// TestColors_InkReadsOnTheChosenTint pins what a chosen row of a sidebar
// relies on: Ink reads at 4.5:1 on the AccentTint over every surface, also
// under Fill, where the quiet pill of a count sets it, for accents of every
// kind. Ink2 does not reach it there, so a chosen row sets its supporting text
// in Ink.
func TestColors_InkReadsOnTheChosenTint(t *testing.T) {
	for _, accent := range []geom.Color{
		geom.Hex(defaultAccent), geom.Hex(0x6a4cf0), geom.Hex(0x0a84ff), geom.Hex(0xe0457b), geom.Hex(0x2f7d5b),
		geom.Hex(0xb58900), geom.Hex(0x0f766e), geom.Hex(0xc2410c), geom.Hex(0x7c3aed),
	} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(accent, b)
			for name, s := range map[string]geom.Color{"Canvas": c.Canvas, "Sidebar": c.Sidebar, "Card": c.Card, "Raised": c.Raised} {
				tint := Over(s, c.AccentTint, 1)
				for ground, g := range map[string]geom.Color{"AccentTint": tint, "Fill over AccentTint": Over(tint, c.Fill, 1)} {
					if r := contrast(c.Ink, g); r < 4.5 {
						t.Errorf("accent %v, brightness %v: Ink on %s over %s is %.3f:1, want 4.5:1", accent, b, ground, name, r)
					}
				}
			}
		}
	}
}

// TestColors_OnTintReads pins the promise of OnTint: every ink meant for text
// reads at 4.5:1 on the AccentTint over every surface, also under Fill and
// under the hover wash of a quiet control, for accents of every kind; what
// OnTint should keep, it keeps.
func TestColors_OnTintReads(t *testing.T) {
	for _, accent := range []geom.Color{
		geom.Hex(defaultAccent), geom.Hex(0x6a4cf0), geom.Hex(0x0a84ff), geom.Hex(0xe0457b), geom.Hex(0x2f7d5b),
		geom.Hex(0xb58900), geom.Hex(0x0f766e), geom.Hex(0xc2410c), geom.Hex(0x7c3aed), geom.Hex(0xffcc00), geom.Hex(0x00ffff),
		geom.Hex(0x808080),
	} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(accent, b)
			on := c.OnTint()
			inks := map[string]geom.Color{"Ink": on.Ink, "Ink2": on.Ink2, "AccentInk": on.AccentInk, "Danger": on.Danger, "Success": on.Success}
			for name, s := range map[string]geom.Color{"Canvas": c.Canvas, "Sidebar": c.Sidebar, "Card": c.Card, "Raised": c.Raised, "Field": c.Field} {
				tint := Over(s, c.AccentTint, 1)
				grounds := map[string]geom.Color{
					"AccentTint":            tint,
					"Fill over AccentTint":  Over(tint, c.Fill, 1),
					"hover over AccentTint": Over(tint, c.Ink, 0.05),
				}
				for ground, g := range grounds {
					for ink, col := range inks {
						if r := contrast(col, g); r < 4.5 {
							t.Errorf("accent %v, brightness %v: OnTint's %s on %s over %s is %.3f:1, want 4.5:1", accent, b, ink, ground, name, r)
						}
					}
				}
			}
			if on.Ink3 != c.Ink3 || on.Ink != c.Ink || on.AccentTint != c.AccentTint || on.Fill != c.Fill || on.Accent != c.Accent {
				t.Errorf("accent %v, brightness %v: OnTint changed a color it should keep", accent, b)
			}
		}
	}
	// An ink that reads already keeps its color.
	c := NewColors(geom.Hex(defaultAccent), BrightnessLight)
	c.AccentInk = c.Ink
	if got := c.OnTint().AccentInk; got != c.Ink {
		t.Errorf("an ink that reads moved: %v, want %v", got, c.Ink)
	}
}
