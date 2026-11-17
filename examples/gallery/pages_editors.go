package main

import (
	"fmt"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/editor"
)

// The editor views page: one document in two editors side by side. The left
// one does not wrap and scrolls sideways; the right one wraps. They share the
// text, the history and the search, and each keeps its own selection and
// scroll position in an editor.View. With the switch on, scrolling either
// scrolls the other to the same line. Comments are italic and keywords bold,
// through Decoration.

// pageEditors returns the editor views page.
func pageEditors() core.Widget { return editorsPage{} }

// editorsSource is the document of the page: Go with long comments, so that
// the left editor has something to scroll sideways.
func editorsSource() string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("// この文書は左右の二つのエディターに同じ Controller で表示しています。左は折り返さずに横へスクロールし、右は折り返します。\n")
	sb.WriteString("import \"github.com/bankai-tensei/gowaku/widgets/editor\"\n\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&sb, "// step%d は %d 番目の手順です。片方で打った文字はもう片方にもすぐ出ますが、選択とスクロール位置はエディターごとに別々です。\n", i, i)
		fmt.Fprintf(&sb, "func step%d(doc *editor.Controller) int {\n\treturn doc.LineCount() + %d // 行の数に足す\n}\n\n", i, i)
	}
	return sb.String()
}

type editorsPage struct{ core.StatefulBase }

type editorsPageState struct {
	core.StateBase
	doc         editor.Controller
	left, right editor.View
	sync        bool
}

// CreateState implements core.StatefulWidget.
func (editorsPage) CreateState() core.State { return &editorsPageState{sync: true} }

// InitState implements core.State.
func (s *editorsPageState) InitState() {
	s.doc.SetText(editorsSource())
	s.left.OnScroll = func(p editor.ScrollPosition) { s.scrolled(&s.right, p) }
	s.right.OnScroll = func(p editor.ScrollPosition) { s.scrolled(&s.left, p) }
}

// scrolled follows a move of one view: the other goes to the same line when
// the switch is on, keeping its own sideways position. SetScroll does not run
// OnScroll, so the two do not chase each other.
func (s *editorsPageState) scrolled(other *editor.View, p editor.ScrollPosition) {
	if s.sync {
		p.X = other.Scroll().X
		other.SetScroll(p)
	}
	s.SetState(func() {})
}

// Build implements core.State.
func (s *editorsPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	note := th.Text.Caption
	note.Color = th.Color.Ink2
	where := func(name string, v *editor.View) string {
		p := v.Scroll()
		return fmt.Sprintf("%s: %d 行目 + %.0fpx、横 %.0fpx", name, p.Line+1, p.Offset, p.X)
	}
	decorate := codeDecorations(th)
	pane := func(label string, v *editor.View, wrap bool) core.Widget {
		return basic.Expanded{Child: editor.Editor{
			Controller: &s.doc, View: v, Label: label, Wrap: wrap, Border: true,
			LineNumbers: true, HighlightCurrentLine: true, Decorations: decorate,
		}}
	}
	return basic.Pad(theme.Space28, basic.Column{Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
		heading(th, "エディターのビュー", "一つの Controller を二つの Editor で表示します。文字と履歴と検索は共有、選択とスクロールは別々です。"),
		basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossCenter, Children: []core.Widget{
			control.Switch{Label: "スクロールを同期", Value: s.sync, OnChange: func(v bool) { s.SetState(func() { s.sync = v }) }},
			basic.Text{Content: where("左", &s.left) + "　／　" + where("右", &s.right), Style: note},
		}},
		basic.Expanded{Child: basic.Row{Spacing: theme.Space12, Cross: layout.CrossStretch, Children: []core.Widget{
			pane("左（折り返しなし）", &s.left, false),
			pane("右（折り返し）", &s.right, true),
		}}},
	}})
}

// codeDecorations sets the comments of a line in italic and a quieter ink, and
// the keywords in bold.
func codeDecorations(th *theme.Theme) func(int, string, []editor.Decoration) []editor.Decoration {
	comment, keyword := th.Color.Ink2, th.Color.AccentInk
	keywords := []string{"package", "import", "func", "return"}
	return func(_ int, line string, dst []editor.Decoration) []editor.Decoration {
		code := line
		if i := strings.Index(line, "//"); i >= 0 {
			dst = append(dst, editor.Decoration{Start: i, End: len(line), Color: comment, Italic: true})
			code = line[:i]
		}
		for _, k := range keywords {
			for at := 0; ; {
				i := strings.Index(code[at:], k)
				if i < 0 {
					break
				}
				i += at
				dst = append(dst, editor.Decoration{Start: i, End: i + len(k), Color: keyword, Bold: true})
				at = i + len(k)
			}
		}
		return dst
	}
}
