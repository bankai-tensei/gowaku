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

// surfaced puts every style of button on one surface of the theme.
type surfaced struct {
	gowaku.StatelessBase
	ground func(*theme.Colors) geom.Color
}

func (s surfaced) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	buttons := make([]gowaku.Widget, 0, 4)
	for _, style := range []control.ButtonStyle{control.ButtonNormal, control.ButtonPrimary, control.ButtonQuiet, control.ButtonDanger} {
		buttons = append(buttons, control.Button{Label: "Delete all", Style: style, OnPress: func() {}})
	}
	return basic.Container{Color: s.ground(&th.Color), Padding: geom.All(16), Child: basic.Column{Spacing: 8, Children: []gowaku.Widget{
		basic.Text{Content: "Body text", Style: th.Text.Body},
		basic.Text{Content: "Supporting text", Style: th.Text.Caption.WithColor(th.Color.Ink2)},
		basic.Text{Content: "Accent text", Style: th.Text.Body.WithColor(th.Color.AccentInk)},
		basic.Text{Content: "Danger text", Style: th.Text.Body.WithColor(th.Color.Danger)},
		basic.Text{Content: "Success text", Style: th.Text.Body.WithColor(th.Color.Success)},
		basic.Row{Spacing: 8, Children: buttons},
	}}}
}

// readmeCounter is the counter of the README: a primary button on a tint
// that deepens with every press.
type readmeCounter struct{ gowaku.StatefulBase }

type readmeCounterState struct {
	gowaku.StateBase
	n int
}

func (readmeCounter) CreateState() gowaku.State { return &readmeCounterState{} }

func (s *readmeCounterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("count: %d", s.n), Style: th.Text.LargeTitle},
		basic.AnimatedContainer{
			Color:   th.Color.AccentTint.WithAlpha(float32(s.n%10) / 10),
			Padding: geom.All(16),
			Radius:  th.Radius.Group,
			Child: control.Button{Label: "Add one", Style: control.ButtonPrimary, OnPress: func() {
				s.SetState(func() { s.n++ })
			}},
		},
	)}
}

// TestWarnings_ReadmeCounterPasses checks the README counter at rest and with
// the pointer resting on its button after a few presses.
func TestWarnings_ReadmeCounterPasses(t *testing.T) {
	for _, accent := range []uint32{0x2f5fd0, 0x0f766e, 0xc2410c, 0x7c3aed} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%06x dark=%v", accent, dark), func(t *testing.T) {
				tt := gowakutest.New(t, readmeCounter{}, gowakutest.Options{
					Width: 400, Height: 300, Theme: theme.FromAccent(geom.Hex(accent)), Dark: dark, Fonts: goFonts.Fonts,
				})
				for _, w := range tt.Warnings() {
					t.Error("at rest:", w)
				}
				for range 9 {
					tt.Tap("Add one")
				}
				tt.Settle()
				for _, w := range tt.Warnings() {
					t.Error("hovered:", w)
				}
			})
		}
	}
}

// TestWarnings_DefaultsPassTheirOwnCheck keeps the library's defaults from
// failing its own contrast check: every text ink and every button style on
// every surface, in both themes, for a few accents, as the README counter
// draws them.
func TestWarnings_DefaultsPassTheirOwnCheck(t *testing.T) {
	grounds := map[string]func(*theme.Colors) geom.Color{
		"Canvas":  func(c *theme.Colors) geom.Color { return c.Canvas },
		"Sidebar": func(c *theme.Colors) geom.Color { return c.Sidebar },
		"Card":    func(c *theme.Colors) geom.Color { return c.Card },
		"Raised":  func(c *theme.Colors) geom.Color { return c.Raised },
		"Field":   func(c *theme.Colors) geom.Color { return c.Field },
	}
	for _, accent := range []uint32{0x2f5fd0, 0x0f766e, 0xc2410c, 0x7c3aed} {
		for _, dark := range []bool{false, true} {
			for name, ground := range grounds {
				t.Run(fmt.Sprintf("%06x dark=%v %s", accent, dark, name), func(t *testing.T) {
					tt := gowakutest.New(t, surfaced{ground: ground}, gowakutest.Options{
						Width: 600, Height: 300, Theme: theme.FromAccent(geom.Hex(accent)), Dark: dark, Fonts: goFonts.Fonts,
					})
					for _, w := range tt.Warnings() {
						t.Error(w)
					}
				})
			}
		}
	}
}
