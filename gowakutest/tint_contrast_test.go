package gowakutest_test

import (
	"fmt"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// quietRow is a row on a surface of the theme with chips, a quiet count and
// quiet buttons. When chosen, it paints the AccentTint, as an application
// does for the chosen row of a ListView, and sets its content in the colors
// of OnTint.
type quietRow struct {
	gowaku.StatelessBase
	surface func(*theme.Colors) geom.Color
	chosen  bool
}

func (r quietRow) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	chips := []gowaku.Widget{
		control.Chip{Label: "Tag"},
		control.Chip{Label: "Filter", OnChange: func(bool) {}},
		control.Chip{Label: "Press", OnPress: func() {}},
	}
	if !r.chosen {
		chips = append(chips, control.Chip{Label: "On", Selected: true, OnChange: func(bool) {}})
	}
	var content gowaku.Widget = basic.Column{Spacing: 8, Children: []gowaku.Widget{
		basic.Row{Spacing: 8, Children: chips},
		basic.Row{Spacing: 8, Children: []gowaku.Widget{
			control.Badge{Count: 12, Quiet: true},
			control.Button{Label: "Quiet", Style: control.ButtonQuiet, OnPress: func() {}},
			control.Button{Label: "Danger", Style: control.ButtonDanger, OnPress: func() {}},
		}},
	}}
	ground := r.surface(&th.Color)
	if r.chosen {
		ground = theme.Over(ground, th.Color.AccentTint, 1)
		content = basic.Theme{Data: th.OnTint(), Child: content}
	}
	return basic.Container{Color: r.surface(&th.Color), Padding: geom.All(8), Child: basic.Container{
		Color: ground, Padding: geom.All(8), Child: content,
	}}
}

// TestWarnings_QuietTextReadsOnTheTint checks that the text of chips, of a
// quiet badge and of quiet buttons, at rest and under the pointer, reads at
// 4.5:1 on every surface and on a chosen row's AccentTint over it, for
// accents of every kind in both themes.
func TestWarnings_QuietTextReadsOnTheTint(t *testing.T) {
	surfaces := map[string]func(*theme.Colors) geom.Color{
		"Canvas":  func(c *theme.Colors) geom.Color { return c.Canvas },
		"Sidebar": func(c *theme.Colors) geom.Color { return c.Sidebar },
		"Card":    func(c *theme.Colors) geom.Color { return c.Card },
		"Raised":  func(c *theme.Colors) geom.Color { return c.Raised },
	}
	for _, accent := range []uint32{0x2f5fd0, 0x0a84ff, 0xe0457b, 0x2f7d5b, 0xb58900, 0x0f766e, 0xc2410c, 0x7c3aed} {
		for _, dark := range []bool{false, true} {
			for name, surface := range surfaces {
				for _, chosen := range []bool{false, true} {
					t.Run(fmt.Sprintf("%06x dark=%v %s chosen=%v", accent, dark, name, chosen), func(t *testing.T) {
						tt := gowakutest.New(t, quietRow{surface: surface, chosen: chosen}, gowakutest.Options{
							Width: 400, Height: 120, Theme: theme.FromAccent(geom.Hex(accent)), Dark: dark, Fonts: goFonts.Fonts,
						})
						for _, w := range tt.Warnings() {
							t.Error("at rest:", w)
						}
						hover := []string{"Quiet", "Danger", "Filter"}
						if !chosen {
							hover = append(hover, "On")
						}
						for _, label := range hover {
							tt.Hover(label)
							tt.Settle()
							for _, w := range tt.Warnings() {
								t.Errorf("%s under the pointer: %v", label, w)
							}
						}
					})
				}
			}
		}
	}
}
