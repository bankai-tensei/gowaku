package theme

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestBrightness_ZeroValueIsLight(t *testing.T) {
	var b Brightness
	if b != BrightnessLight || b.String() != "light" {
		t.Errorf("got %v", b)
	}
	if got := Brightness(9).String(); got != "Brightness(9)" {
		t.Errorf("got %q", got)
	}
}

func TestColors_SurfacesAreNeverWarm(t *testing.T) {
	// Beige is the one thing the design must not look like (docs/DESIGN.md §1):
	// blue must never fall below red on a surface.
	for _, accent := range []uint32{0x2f5fd0, 0xc2410c, 0x0f766e, 0x808080} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(geom.Hex(accent), b)
			for name, s := range map[string]geom.Color{"canvas": c.Canvas, "sidebar": c.Sidebar, "card": c.Card, "raised": c.Raised} {
				if s.B < s.R {
					t.Errorf("%06x %v %s: %+v is warm", accent, b, name, s.Std())
				}
			}
		}
	}
}

func TestColors_LightCanvas(t *testing.T) {
	got := Default().Light.Color.Canvas
	if got != geom.Hex(0xf5f5f7) {
		t.Errorf("got %+v", got.Std())
	}
}

func TestColors_SurfacesIgnoreTheAccent(t *testing.T) {
	a, b := NewColors(geom.Hex(0x2f5fd0), BrightnessLight), NewColors(geom.Hex(0xc2410c), BrightnessLight)
	if a.Canvas != b.Canvas || a.Ink != b.Ink {
		t.Error("the surfaces must not depend on the accent")
	}
	if a.Accent == b.Accent {
		t.Error("the accent must")
	}
}

func TestColors_TextIsLegible(t *testing.T) {
	for _, accent := range []uint32{0x2f5fd0, 0xc2410c, 0x0f766e, 0x7c3aed} {
		for _, b := range []Brightness{BrightnessLight, BrightnessDark} {
			c := NewColors(geom.Hex(accent), b)
			if got := contrast(c.OnAccent, c.Accent); got < 4.5 {
				t.Errorf("%06x %v: text on the accent has contrast %.2f", accent, b, got)
			}
			if got := contrast(c.AccentInk, c.Card); got < 4.5 {
				t.Errorf("%06x %v: accent text on a card has contrast %.2f", accent, b, got)
			}
			if got := contrast(c.Ink2, c.Canvas); got < 4.5 {
				t.Errorf("%06x %v: supporting text has contrast %.2f", accent, b, got)
			}
			if got := contrast(c.Success, c.Card); got < 4.5 {
				t.Errorf("%06x %v: success text on a card has contrast %.2f", accent, b, got)
			}
			if got := contrast(geom.Hex(0xffffff), c.Success); b == BrightnessLight && got < 4.5 {
				t.Errorf("%06x: white on success has contrast %.2f", accent, got)
			}
		}
	}
}

func TestShadows_FloatIsWiderThanAControl(t *testing.T) {
	c := NewColors(geom.Hex(defaultAccent), BrightnessLight)
	if len(c.Shadows(LiftNone)) != 0 {
		t.Error("no lift casts no shadow")
	}
	if c.Shadows(LiftFloat)[1].Blur <= c.Shadows(LiftKnob)[1].Blur {
		t.Error("a higher lift must spread further")
	}
}

func TestSet_For(t *testing.T) {
	light, dark := New(geom.Hex(1), BrightnessLight), New(geom.Hex(1), BrightnessDark)
	if got := (Set{Light: light, Dark: dark}).For(BrightnessDark); got != dark {
		t.Errorf("got %v", got.Brightness)
	}
	if got := (Set{Light: light}).For(BrightnessDark); got != light {
		t.Error("a missing half falls back to the other")
	}
	if got := (Set{}).For(BrightnessDark); got != Default().Dark {
		t.Error("an empty set is the default")
	}
}

func TestDefault_ComputedOnce(t *testing.T) {
	first, second := Default(), Default()
	if first.Light != second.Light {
		t.Error("got two themes")
	}
}

func TestOver(t *testing.T) {
	got := Over(geom.Hex(0xffffff), geom.Hex(0), 0.5)
	if got.A != 1 || got.R < 0.49 || got.R > 0.51 {
		t.Errorf("got %+v", got)
	}
}

func TestTextTheme_LinesStayOnTheGrid(t *testing.T) {
	th := NewTextTheme()
	for name, s := range map[string]float32{
		"LargeTitle": th.LargeTitle.Size * th.LargeTitle.LineHeight,
		"Title":      th.Title.Size * th.Title.LineHeight,
		"Headline":   th.Headline.Size * th.Headline.LineHeight,
		"Body":       th.Body.Size * th.Body.LineHeight,
		"Control":    th.Control.Size * th.Control.LineHeight,
		"Caption":    th.Caption.Size * th.Caption.LineHeight,
		"Section":    th.Section.Size * th.Section.LineHeight,
		"Reading":    th.Reading.Size * th.Reading.LineHeight,
		"Code":       th.Code.Size * th.Code.LineHeight,
	} {
		whole := math.Round(float64(s))
		if math.Abs(float64(s)-whole) > 1e-3 || int(whole)%2 != 0 {
			t.Errorf("%s: a line of %v is not an even number of pixels", name, s)
		}
	}
}
