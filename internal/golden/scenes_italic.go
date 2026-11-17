package golden

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
)

// italicNote is the text of the field of the italic scenes.
const italicNote = "The caret and the selection stay upright over slanted text."

// italicScenes returns the scenes of synthetic italic: the scenes have only Go
// Regular, which has no italic, so every italic glyph here is slanted by the
// atlas. See docs/ADR/0101-synthetic-italic.md.
func italicScenes() []Scene {
	s := make([]Scene, 0, 2)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		var c field.InputController
		s = append(s, Scene{
			Name: "italic" + suffix, Width: 720, Height: 540, Scale: 1.5, Dark: dark,
			Root: func() core.Widget { return italicPage(&c) },
			Act: func(*core.Tree, core.BuildContext) {
				at := strings.Index(italicNote, "selection")
				c.SetSelection(at, at+len("selection stay"))
				c.Focus()
			},
		})
	}
	return s
}

// italicPage sets upright and italic text side by side: a sentence with an
// italic word, a wrapped italic paragraph, italic under ruby, a large line
// whose lean reaches past its last advance, and a field of italic text with a
// selection.
func italicPage(c *field.InputController) core.Widget {
	italic := text.Style{Italic: true}
	const wrapped = "An italic paragraph wraps like any other: the advances do not change, so the lines break where the upright ones do."
	return basic.Padding{Insets: geom.All(16), Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 12, Children: []core.Widget{
			basic.RichText{Spans: []text.Span{
				{Text: "Upright text, then "}, {Text: "an italic phrase", Style: italic}, {Text: ", then upright again."},
			}},
			basic.Row{Spacing: 16, Children: []core.Widget{
				basic.SizedBox{Width: 200, Child: basic.Text{Content: wrapped}},
				basic.SizedBox{Width: 200, Child: basic.Text{Content: wrapped, Style: italic}},
			}},
			basic.RichText{Spans: []text.Span{
				{Text: "Ruby over "}, {Text: "upright", Ruby: "plain"}, {Text: " and "},
				{Text: "italic", Ruby: "leaning", Style: italic}, {Text: " words."},
			}},
			basic.Row{Spacing: 16, Children: []core.Widget{
				basic.Text{Content: "Hill", Style: text.Style{Size: 40}},
				basic.Text{Content: "Hill", Style: text.Style{Size: 40, Italic: true}},
			}},
			basic.SizedBox{Width: 440, Child: field.Input{
				Label: "Italic", Value: italicNote, Controller: c, Style: italic, OnChange: func(string) {},
			}},
		},
	}}
}
