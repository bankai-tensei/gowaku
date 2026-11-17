package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/editor"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// The page of a code reader (ADR 0117): a quick open that matches and marks
// file names as the command palette does, through field.Fuzzy and FuzzySpans;
// a tree of files whose rows select on a right click and open a context menu
// through Tree.OnContextMenu; and a diff in an editor whose added and removed
// lines are colored across the whole width by Editor.LineStyle.

// linesFiles is the file system of the page: "" holds the roots.
var linesFiles = map[string][]string{
	"":                  {"cmd", "internal", "docs", "go.mod", "README.md"},
	"cmd":               {"cmd/reader"},
	"cmd/reader":        {"cmd/reader/main.go", "cmd/reader/flags.go"},
	"internal":          {"internal/markdown", "internal/viewer"},
	"internal/markdown": {"internal/markdown/parse.go", "internal/markdown/render.go", "internal/markdown/render_test.go"},
	"internal/viewer":   {"internal/viewer/diff.go", "internal/viewer/tree.go"},
	"docs":              {"docs/design.md", "docs/keys.md"},
}

// linesAllFiles are the files of linesFiles, for the quick open.
func linesAllFiles() []string {
	var out []string
	var walk func(k string)
	walk = func(k string) {
		for _, c := range linesFiles[k] {
			if len(linesFiles[c]) == 0 {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk("")
	return out
}

// linesDiff is the diff the editor shows.
const linesDiff = `diff --git a/internal/viewer/diff.go b/internal/viewer/diff.go
@@ -12,14 +12,20 @@ func (v *Viewer) lineStyle(i int, s string) editor.LineStyle {
 	// lineStyle colors the lines of a diff as a whole, across the editor.
 	switch {
-	case strings.HasPrefix(s, "+"):
-		return editor.LineStyle{Gutter: v.added}
-	case strings.HasPrefix(s, "-"):
-		return editor.LineStyle{Gutter: v.removed}
+	case strings.HasPrefix(s, "+"):
+		// The whole line, empty or not, and every row of a wrapped one.
+		return editor.LineStyle{Background: v.addedTint, Gutter: v.addedTint, Bar: v.added}
+
+	case strings.HasPrefix(s, "-"):
+		return editor.LineStyle{Background: v.removedTint, Gutter: v.removedTint, Bar: v.removed}
+	case strings.HasPrefix(s, "@@"):
+		return editor.LineStyle{Background: v.hunk}
 	}
 	return editor.LineStyle{}
 }
@@ -40,6 +46,9 @@ func (v *Viewer) open(path string) {
 	v.doc.SetText(source)
+	// The diff of a file changes the colors of its lines, not their text: ask
+	// for the styles again.
+	v.doc.Redecorate()
 	v.path = path
 }`

type linesPage struct{ core.StatefulBase }

type linesPageState struct {
	core.StateBase
	doc      editor.Controller
	tree     collection.TreeController[string]
	query    string
	fuzzy    field.Fuzzy
	files    []string
	shown    []linesHit
	lastMenu string
}

// linesHit is a file that matches the query, with where.
type linesHit struct {
	name   string
	score  int
	ranges []int
}

// pageLines returns the page.
func pageLines() core.Widget { return linesPage{} }

// CreateState implements core.StatefulWidget.
func (linesPage) CreateState() core.State { return &linesPageState{} }

// InitState implements core.State.
func (s *linesPageState) InitState() {
	s.doc.SetText(linesDiff)
	s.files = linesAllFiles()
	s.tree.Expand("internal")
	s.tree.Expand("internal/markdown")
	s.filter()
}

// filter fills shown with the files that match the query, best first.
func (s *linesPageState) filter() {
	s.fuzzy.SetQuery(s.query)
	s.shown = s.shown[:0]
	for _, f := range s.files {
		if score, ok := s.fuzzy.Match(f); ok {
			s.shown = append(s.shown, linesHit{name: f, score: score, ranges: slices.Clone(s.fuzzy.Ranges())})
		}
	}
	slices.SortStableFunc(s.shown, func(a, b linesHit) int { return b.score - a.score })
}

// lineStyle colors the lines of the diff.
func linesStyle(th *theme.Theme) func(int, string) editor.LineStyle {
	added, removed := th.Color.Success, th.Color.Danger
	addedTint, removedTint, hunk := added.WithAlpha(0.14), removed.WithAlpha(0.14), th.Color.AccentTint
	return func(_ int, s string) editor.LineStyle {
		switch {
		case strings.HasPrefix(s, "+"):
			return editor.LineStyle{Background: addedTint, Gutter: addedTint, Bar: added}
		case strings.HasPrefix(s, "-"):
			return editor.LineStyle{Background: removedTint, Gutter: removedTint, Bar: removed}
		case strings.HasPrefix(s, "@@"):
			return editor.LineStyle{Background: hunk, Gutter: hunk}
		}
		return editor.LineStyle{}
	}
}

// Build implements core.State.
func (s *linesPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	note := th.Text.Caption
	note.Color = th.Color.Ink2
	body := th.Text.Body
	body.Color = th.Color.Ink
	hit := field.FuzzyHighlight(body, th.Color.AccentInk)
	matches := make([]core.Widget, 0, 4)
	for _, h := range s.shown[:min(len(s.shown), 4)] {
		matches = append(matches, basic.RichText{Spans: field.FuzzySpans(nil, h.name, h.ranges, body, hit), Style: body})
	}
	if len(matches) == 0 {
		matches = append(matches, basic.Text{Content: "一致なし", Style: note})
	}
	label := func(k string) string { return k[strings.LastIndexByte(k, '/')+1:] }
	tree := basic.Builder{Content: func(tctx core.BuildContext) core.Widget {
		return collection.Tree[string]{
			Roots: linesFiles[""], ChildrenOf: func(k string) []string { return linesFiles[k] },
			LabelOf: label, Label: "ファイル", Controller: &s.tree, Multiselect: true,
			OnContextMenu: func(node string, pos geom.Offset) {
				s.SetState(func() { s.lastMenu = node })
				n := len(s.tree.Selection())
				what := label(node)
				if n > 1 {
					what = strconv.Itoa(n) + " 個のファイル"
				}
				overlay.ShowPopupMenu(tctx, pos, []overlay.MenuItem{
					{Action: interact.Action{Label: "開く", OnInvoke: func() {}}},
					{Action: interact.Action{Label: "名前を変更", OnInvoke: func() {}}},
					{Separator: true},
					{Action: interact.Action{Label: what + " を削除", OnInvoke: func() {}}},
				})
			},
		}
	}}
	menuNote := "行を右クリック（またはメニューキー）"
	if s.lastMenu != "" {
		menuNote = "メニュー: " + label(s.lastMenu)
	}
	left := basic.SizedBox{Width: 300, Child: basic.Column{Spacing: theme.Space8, Cross: layout.CrossStretch, Children: []core.Widget{
		field.Input{Label: "クイックオープン", Placeholder: "ファイル名の一部", Value: s.query, OnChange: func(q string) {
			s.SetState(func() { s.query = q; s.filter() })
		}},
		basic.Column{Size: layout.MainMin, Spacing: 2, Cross: layout.CrossStart, Children: matches},
		basic.Text{Content: menuNote, Style: note},
		basic.Expanded{Child: tree},
	}}}
	return basic.Pad(theme.Space28, basic.Column{Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
		heading(th, "差分とメニュー", "あいまい一致の強調、右クリックで選ぶツリー、行の幅いっぱいの背景を持つ差分。"),
		basic.Expanded{Child: basic.Row{Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []core.Widget{
			left,
			basic.Expanded{Child: editor.Editor{
				Controller: &s.doc, Label: "差分", ReadOnly: true, Border: true, LineNumbers: true, Wrap: true,
				LineStyle: linesStyle(th),
			}},
		}}},
	}})
}
