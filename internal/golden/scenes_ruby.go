package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
)

// rubyNotes is text with the ruby notation of Aozora Bunko. The scenes have
// only a Latin font, so base and ruby are Latin.
const rubyNotes = "Ruby sits over ｜words《above》 as the\nfield ｜shows《hides》 the notation."

// emphasisNotes is rubyNotes with notes of emphasis marks, which the Latin
// font has as circles: beside a word, and outside the ruby of a word.
const emphasisNotes = "Ruby sits［＃「sits」に丸傍点］ over ｜words《above》［＃「words」に丸傍点］ as the\nfield ｜shows《hides》 the notation."

// rubyScenes returns the scenes of the ruby notation in a Input.
func rubyScenes() []Scene {
	return []Scene{
		{Name: "ruby", Width: 720, Height: 330, Scale: 1.5, Root: rubyPage(text.Horizontal), Act: openRuby(input.KeyLeft)},
		{Name: "ruby-vertical", Width: 720, Height: 540, Scale: 1.5, Root: rubyPage(text.Vertical), Act: openRuby(input.KeyUp)},
	}
}

// rubyPage shows the same text twice: with the ruby over the bases, and with
// the caret inside the second mark, whose notation then shows. The vertical
// page has emphasis marks too.
func rubyPage(o text.Orientation) func() core.Widget {
	return func() core.Widget {
		value := rubyNotes
		if o == text.Vertical {
			value = emphasisNotes
		}
		field := func(label string) core.Widget {
			return fieldw.Input{
				Label: label, Value: value, Multiline: true, Ruby: aozora.Marks,
				Style: text.Style{Orientation: o}, OnChange: func(string) {},
			}
		}
		if o == text.Vertical {
			return basic.Padding{Insets: geom.All(16), Child: basic.SizedBox{Height: 300, Child: basic.Row{
				Cross: layout.CrossStart, Spacing: 24, Children: []core.Widget{
					basic.SizedBox{Width: 140, Child: field("Shown")},
					basic.SizedBox{Width: 140, Child: field("Editing")},
				},
			}}}
		}
		return basic.Padding{Insets: geom.All(16), Child: basic.SizedBox{Width: 420, Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 16, Children: []core.Widget{field("Shown"), field("Editing")},
		}}}
	}
}

// openRuby puts the caret of the second field into its second mark: to the
// end of the text, then back over " the notation." and into the 》, with the
// key that goes back along a line.
func openRuby(back input.KeyCode) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleTextField, "Editing"), semantics.ActionFocus, core.SemanticsArg{})
		tree.DispatchKey(core.KeyEvent{Key: input.KeyEnd, Modifiers: input.ModControl})
		for range len(" the notation.") + 1 {
			tree.DispatchKey(core.KeyEvent{Key: back})
		}
	}
}
