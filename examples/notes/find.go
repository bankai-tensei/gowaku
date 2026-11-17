package main

import (
	"fmt"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/theme"
)

// findWidth is the width of the find bar.
const findWidth = 460

// findState is the find and replace bar of an editor.
type findState struct {
	open, replace bool
	query, with   string
	// fold ignores case.
	fold bool
	// current is the match the selection is on, or -1.
	current int
	matches []match
	// queryField is the field of the query, which Ctrl+F focuses, and
	// withField that of the replacement.
	queryField, withField field.InputController
}

// openFind shows the bar, with the replace row if replace is set, and puts
// the keyboard in the query.
func (s *editorState) openFind(replace bool) {
	d := s.Widget().win.current()
	start, end := s.selection()
	s.SetState(func() {
		s.find.open, s.find.replace, s.find.current = true, replace, -1
		// A selection on one line is what to look for, as in most editors.
		// The notation of ruby is left out: 嵐山 finds ｜嵐山《あらしやま》 too.
		if d != nil && start < end && end <= len(d.text) {
			if sel := aozora.Plain(d.text[start:end]); !strings.Contains(sel, "\n") {
				s.find.query = sel
			}
		}
	})
	// The field takes the keyboard now, or once the bar is laid out, with its
	// text selected so that typing replaces it.
	s.find.queryField.SetSelection(0, len(s.find.query))
	s.find.queryField.Focus()
}

// hairStroke is the hairline around a floating card.
func hairStroke(th *theme.Theme) paint.Stroke { return paint.Stroke{Width: 1, Color: th.Color.Hair} }

// closeFind hides the bar and gives the keyboard back to the text.
func (s *editorState) closeFind() {
	s.SetState(func() { s.find.open = false })
	if c := s.field(); c != nil {
		c.Focus()
	}
}

// search finds the query in the current document again.
func (s *editorState) search() []match {
	d := s.Widget().win.current()
	if d == nil {
		return nil
	}
	s.find.matches = findAll(d.text, s.find.query, s.find.fold, s.find.matches)
	return s.find.matches
}

// findStep selects the next match after the selection, or the previous one
// before it when dir is negative, wrapping around.
func (s *editorState) findStep(dir int) {
	if s.find.query == "" {
		s.openFind(false)
		return
	}
	ms := s.search()
	start, end := s.selection()
	i := nextMatch(ms, end)
	if dir < 0 {
		i = prevMatch(ms, start)
	}
	s.selectMatch(i)
}

// selectMatch selects match i in the text.
func (s *editorState) selectMatch(i int) {
	s.SetState(func() { s.find.current = i })
	if i < 0 || i >= len(s.find.matches) {
		return
	}
	m := s.find.matches[i]
	if c := s.field(); c != nil {
		c.SetSelection(m.start, m.end)
	}
}

// queryChanged searches as the query is typed, from where the caret is.
func (s *editorState) queryChanged(q string) {
	s.find.query = q
	ms := s.search()
	start, _ := s.selection()
	s.selectMatch(nextMatch(ms, start))
}

// replaceOne replaces the match the selection is on and goes to the next.
func (s *editorState) replaceOne() {
	ms := s.search()
	start, end := s.selection()
	for _, m := range ms {
		if c := s.field(); c != nil && m.start == start && m.end == end {
			c.Replace(m.start, m.end, s.find.with)
			break
		}
	}
	s.findStep(1)
}

// replaceAll replaces every match, as one edit that undo takes back.
func (s *editorState) replaceAll() {
	w := s.Widget().win
	d := w.current()
	ms := s.search()
	if d == nil || len(ms) == 0 {
		return
	}
	// One replacement from the first match to the last is one step for undo.
	n := len(ms)
	first, last := ms[0].start, ms[n-1].end
	all := replaceAll(d.text, ms, s.find.with)
	s.fieldOf(d).Replace(first, last, all[first:len(all)-(len(d.text)-last)])
	s.SetState(func() { s.find.current = -1 })
	w.toast(fmt.Sprintf("%d 件を置換しました", n))
}

// findBar is the bar of find and replace.
func (s *editorState) findBar(ctx core.BuildContext, d *document) core.Widget {
	th := basic.ThemeOf(ctx)
	f := &s.find
	ms := findAll(d.text, f.query, f.fold, f.matches)
	f.matches = ms
	count := "検索語を入力"
	switch {
	case f.query == "":
	case len(ms) == 0:
		count = "見つかりません"
	case f.current >= 0 && f.current < len(ms):
		count = fmt.Sprintf("%d / %d 件", f.current+1, len(ms))
	default:
		count = fmt.Sprintf("%d 件", len(ms))
	}
	quiet := func(label string, on func()) core.Widget {
		return control.Button{Label: label, Style: control.ButtonQuiet, Size: control.ButtonSmall, OnPress: on}
	}
	rows := []core.Widget{
		basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space6, Children: []core.Widget{
			basic.Expanded{Child: field.Input{
				Value: f.query, Placeholder: "検索", Controller: &f.queryField,
				OnChange: s.queryChanged,
				OnSubmit: func(string) { s.findStep(1) },
			}},
			basic.SizedBox{Width: 104, Child: basic.Text{Content: count, Style: colored(th.Text.Caption, th.Color.Ink2)}},
			quiet("前へ", func() { s.findStep(-1) }),
			quiet("次へ", func() { s.findStep(1) }),
			quiet("閉じる", s.closeFind),
		}},
	}
	if f.replace {
		rows = append(rows, basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space6, Children: []core.Widget{
			basic.Expanded{Child: field.Input{
				Value: f.with, Placeholder: "置換後の文字列", Controller: &f.withField,
				OnChange: func(v string) { f.with = v },
				OnSubmit: func(string) { s.replaceOne() },
			}},
			control.Button{Label: "置換", Size: control.ButtonSmall, OnPress: s.replaceOne},
			control.Button{Label: "すべて置換", Size: control.ButtonSmall, OnPress: s.replaceAll},
		}})
	}
	rows = append(rows, control.Checkbox{Value: !f.fold, Label: "大文字と小文字を区別する", OnChange: func(v bool) {
		s.SetState(func() {
			f.fold = !v
			f.current = -1
		})
	}})
	return interact.Focus{
		OnKey: func(e *core.KeyEvent) bool {
			if e.Key == input.KeyEscape {
				s.closeFind()
				return true
			}
			return false
		},
		Child: basic.Container{
			Color: th.Color.Raised, Radius: th.Radius.Group, Padding: geom.All(theme.Space10),
			Border:  hairStroke(th),
			Shadows: th.Color.Shadows(theme.LiftFloat),
			Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: rows},
		},
	}
}
