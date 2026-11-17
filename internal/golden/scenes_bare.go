package golden

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// bareNote is the text of the page of the bare field scenes.
const bareNote = "A bare field is the page itself: no border, no ground and no ring.\n\n" +
	"The caret alone shows where the keyboard is, as in the editors of the platforms, " +
	"and the selection is the only tint."

// bareScenes returns the scenes of a Input without its box.
func bareScenes() []Scene {
	s := make([]Scene, 0, 2)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		var c field.InputController
		s = append(s, Scene{
			Name: "field-bare" + suffix, Width: 720, Height: 450, Scale: 1.5, Dark: dark,
			Root: func() core.Widget { return barePage(&c) },
			Act: func(*core.Tree, core.BuildContext) {
				at := strings.Index(bareNote, "caret alone")
				c.SetSelection(at+len("caret alone"), at)
				c.Focus()
			},
		})
	}
	return s
}

// barePage is a page of a notes app: a title field with its placeholder over
// the text, both bare on the card.
func barePage(c *field.InputController) core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		return basic.Container{Color: th.Color.Card, Padding: geom.Symmetric(8, 12), Child: basic.Column{
			Cross: layout.CrossStretch, Spacing: 4, Children: []core.Widget{
				field.Input{
					Bare: true, Padding: geom.Symmetric(16, 4), Placeholder: "Title",
					Style: th.Text.Title, OnChange: func(string) {},
				},
				basic.Expanded{Child: field.Input{
					Controller: c, Bare: true, Multiline: true, Padding: geom.Symmetric(16, 8),
					Value: bareNote, OnChange: func(string) {},
				}},
			},
		}}
	}}
}
