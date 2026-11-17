package golden

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/editor"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// editorSource is the text of the editor scenes: Latin only, for the one font
// of the golden renderer.
const editorSource = `// Package notes keeps short notes.
package notes

import "strings"

// Note is one entry of the list.
type Note struct {
	Title string
	Body  string
}

// Summary returns the first line of the body.
func (n Note) Summary() string {
	line, _, _ := strings.Cut(n.Body, "\n")
	return strings.TrimSpace(line) // trimed
}`

// editorScenes returns the scenes of Editor.
func editorScenes() []Scene {
	s := make([]Scene, 0, 4)
	for _, dark := range []bool{false, true} {
		suffix := ""
		if dark {
			suffix = "-dark"
		}
		var a, b editor.Controller
		s = append(s,
			Scene{
				Name: "editor" + suffix, Width: 720, Height: 570, Scale: 1.5, Dark: dark,
				Root: func() core.Widget { return editorPage(&a) },
				Act: func(*core.Tree, core.BuildContext) {
					a.SetSelection(editor.TextRange{
						Start: editor.TextPosition{Line: 5, Column: 3},
						End:   editor.TextPosition{Line: 6, Column: 9},
					})
				},
			},
			Scene{
				Name: "editor-find" + suffix, Width: 720, Height: 570, Scale: 1.5, Dark: dark,
				Root: func() core.Widget { return editorPage(&b) },
				Act:  func(tree *core.Tree, _ core.BuildContext) { openEditorFind(tree, &b) },
			},
		)
	}
	return s
}

// editorPage shows source with line numbers, colors and a squiggle under a
// misspelled word.
func editorPage(ctl *editor.Controller) core.Widget {
	if ctl.Text() == "" {
		ctl.SetText(editorSource)
	}
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		return basic.Padding{Insets: geom.All(16), Child: editor.Editor{
			Controller:  ctl,
			Label:       "Source",
			LineNumbers: true,
			Border:      true,
			Decorations: func(_ int, s string, dst []editor.Decoration) []editor.Decoration {
				return decorateGo(s, dst, th.Color.AccentInk, th.Color.Success, th.Color.Ink2, th.Color.Danger)
			},
		}}
	}}
}

// decorateGo colors a little Go: keywords, strings and comments, and puts a
// squiggle under "trimed".
func decorateGo(s string, dst []editor.Decoration, keyword, str, comment, wrong geom.Color) []editor.Decoration {
	if i := strings.Index(s, "//"); i >= 0 {
		dst = append(dst, editor.Decoration{Start: i, End: len(s), Color: comment})
		if j := strings.Index(s, "trimed"); j > i {
			dst = append(dst, editor.Decoration{
				Start: j, End: j + len("trimed"), Underline: editor.UnderlineSquiggle, UnderlineColor: wrong,
			})
		}
		s = s[:i]
	}
	for _, kw := range []string{"package", "import", "type", "struct", "func", "return", "string"} {
		for at := 0; ; {
			k := strings.Index(s[at:], kw)
			if k < 0 {
				break
			}
			a, z := at+k, at+k+len(kw)
			if (a == 0 || !isIdent(s[a-1])) && (z == len(s) || !isIdent(s[z])) {
				dst = append(dst, editor.Decoration{Start: a, End: z, Color: keyword, Bold: true})
			}
			at = z
		}
	}
	if a := strings.IndexByte(s, '"'); a >= 0 {
		if z := strings.IndexByte(s[a+1:], '"'); z >= 0 {
			dst = append(dst, editor.Decoration{Start: a, End: a + z + 2, Color: str})
		}
	}
	return dst
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// openEditorFind opens the find bar on "strings", which the editor then
// highlights three times.
func openEditorFind(tree *core.Tree, ctl *editor.Controller) {
	tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleTextField, "Source"), semantics.ActionFocus, core.SemanticsArg{})
	ctl.SetSelection(editor.TextRange{
		Start: editor.TextPosition{Line: 3, Column: 8},
		End:   editor.TextPosition{Line: 3, Column: 15},
	})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyF, Modifiers: input.ModControl})
}
