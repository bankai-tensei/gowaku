package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// searchDelay is how long typing must pause before the search runs.
	searchDelay = 180 * time.Millisecond
	cardWidth   = 232
	cardHeight  = 140
)

// notebookPage is the first page of a window: the notes of the notebook as
// cards, and a search over them.
type notebookPage struct {
	core.StatefulBase
	win *windowModel
}

type notebookState struct {
	core.StateOf[notebookPage]
	query string
	// hits are the results for searched, and gen counts the searches so that
	// a late answer to an old one is dropped.
	hits     []store.Hit
	searched string
	err      error
	gen      int
	disposed bool
}

// CreateState implements core.StatefulWidget.
func (notebookPage) CreateState() core.State { return &notebookState{} }

// Dispose implements core.State.
func (s *notebookState) Dispose() { s.disposed = true }

// typed takes a new query, and searches once typing pauses.
func (s *notebookState) typed(q string) {
	a := s.Widget().win.app
	s.gen++
	gen := s.gen
	s.SetState(func() { s.query = q })
	if strings.TrimSpace(q) == "" {
		s.SetState(func() { s.hits, s.searched, s.err = nil, "", nil })
		return
	}
	a.shell.after(searchDelay, func() {
		if gen != s.gen || s.disposed {
			return
		}
		a.search(q, func(hits []store.Hit, err error) {
			if gen != s.gen || s.disposed {
				return
			}
			s.SetState(func() { s.hits, s.searched, s.err = hits, q, err })
		})
	})
}

// Build implements core.State.
func (s *notebookState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().win
	a := w.app
	if w.state != nil {
		w.state.notebook = s
	}
	th := basic.ThemeOf(ctx)
	caption := colored(th.Text.Caption, th.Color.Ink2)
	var cards []core.Widget
	var summary string
	var empty string
	searching := strings.TrimSpace(s.query) != ""
	switch {
	case searching && s.err != nil:
		empty = "検索できません: " + errText(s.err)
	case searching && s.searched != s.query:
		summary = "検索しています…"
	case searching:
		summary = fmt.Sprintf("「%s」が %d か所に見つかりました", s.searched, len(s.hits))
		if len(s.hits) == 0 {
			summary, empty = "", fmt.Sprintf("「%s」は見つかりませんでした", s.searched)
		}
		for i := range s.hits {
			cards = append(cards, s.hitCard(th, &s.hits[i]))
		}
	case a.notesErr != nil:
		empty = "ノート帳を読めません: " + errText(a.notesErr)
	default:
		summary = fmt.Sprintf("%d 冊のノート　%s", len(a.notes), a.settings.Library)
		if len(a.notes) == 0 {
			summary, empty = "", "ノートはまだありません。「新しいノート」で書き始めましょう。"
		}
		for _, n := range a.notes {
			cards = append(cards, s.noteCard(th, n))
		}
	}
	var content core.Widget
	if empty != "" {
		content = basic.Padding{Insets: geom.Insets{Top: theme.Space28}, Child: basic.Align{
			Alignment: layout.AlignTopCenter, Child: basic.Text{Content: empty, Style: colored(th.Text.Body, th.Color.Ink2)},
		}}
	} else {
		content = basic.Scroll{Child: basic.Padding{
			Insets: geom.Insets{Left: theme.Space28, Right: theme.Space28, Bottom: theme.Space28},
			Child: basic.Grid{
				MinColumnWidth: cardWidth, Spacing: theme.Space12, RunSpacing: theme.Space12,
				TileHeight: cardHeight, Children: cards,
			},
		}}
	}
	return basic.Container{Color: th.Color.Canvas, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Padding{
			Insets: geom.Insets{Left: theme.Space28, Top: theme.Space24, Right: theme.Space28, Bottom: theme.Space16},
			Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space12, Children: []core.Widget{
				basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
					basic.Expanded{Child: basic.Text{Content: "ノート帳", Style: colored(th.Text.LargeTitle, th.Color.Ink)}},
					control.Button{Label: "開く…", OnPress: w.state.openDialog},
					control.Button{Label: "新しいノート", Style: control.ButtonPrimary, OnPress: w.state.newDoc},
				}},
				field.Search{Value: s.query, Placeholder: "ノートを検索", OnChange: s.typed},
				basic.Text{Content: summary, Style: caption},
			}},
		},
		basic.Expanded{Child: content},
	}}}
}

// noteCard is the card of a note: its title, the start of its text and when
// it changed.
func (s *notebookState) noteCard(th *theme.Theme, n store.Note) core.Widget {
	w := s.Widget().win
	tag := heroKey{path: n.Path, offset: -1}
	preview := aozora.Plain(n.Preview)
	if preview == "" {
		preview = "（本文なし）"
	}
	return control.Card{
		Label: n.Title, Padding: geom.All(theme.Space16),
		OnPress: func() { s.open(n.Path, tag, nil) },
		Child: basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space6, Children: []core.Widget{
			nav.Hero{Tag: tag, Child: basic.Text{Content: truncate(n.Title, 16), Style: colored(th.Text.Headline, th.Color.Ink)}},
			basic.Expanded{Child: basic.Text{Content: truncate(preview, 38), Style: colored(th.Text.Body, th.Color.Ink2)}},
			basic.Text{Content: modified(n.Modified, w.app.now()), Style: colored(th.Text.Caption, th.Color.Ink2)},
		}},
	}
}

// hitCard is the card of a hit: the title of the note, and the line of the
// match with the match marked.
func (s *notebookState) hitCard(th *theme.Theme, h *store.Hit) core.Widget {
	tag := heroKey{path: h.Note.Path, offset: h.Offset}
	body := colored(th.Text.Body, th.Color.Ink2)
	mark := colored(th.Text.Body, th.Color.AccentInk)
	mark.Weight = 700
	before := []rune(h.Before)
	if len(before) > 14 {
		before = append([]rune("…"), before[len(before)-14:]...)
	}
	m := match{start: h.Offset, end: h.Offset + len(h.Match)}
	return control.Card{
		Label: h.Note.Title + " " + h.Before + h.Match + h.After, Padding: geom.All(theme.Space16),
		OnPress: func() { s.open(h.Note.Path, tag, &m) },
		Child: basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space6, Children: []core.Widget{
			nav.Hero{Tag: tag, Child: basic.Text{Content: truncate(h.Note.Title, 16), Style: colored(th.Text.Headline, th.Color.Ink)}},
			basic.Expanded{Child: basic.RichText{Spans: []text.Span{
				{Text: string(before), Style: body},
				{Text: h.Match, Style: mark},
				{Text: truncate(h.After, 30), Style: body},
			}}},
			basic.Text{Content: fmt.Sprintf("%d 行目", h.Line+1), Style: colored(th.Text.Caption, th.Color.Ink2)},
		}},
	}
}

// open opens a note from its card, with the title flying to the editor, and
// selects pick if a hit gave one.
func (s *notebookState) open(path string, tag heroKey, pick *match) {
	w := s.Widget().win
	w.hero = tag
	d, err := w.app.open(w, path)
	if err != nil {
		w.toast("開けません: " + errText(err))
		return
	}
	if other, _ := w.app.windowOf(d.id); other != nil && other != w {
		// The note is open in another window, which came to the front.
		other.hero, w = tag, other
	}
	if pick != nil {
		p := *pick
		p.end = min(p.end, len(d.text))
		p.start = min(p.start, p.end)
		w.pick = &p
	}
	w.showEditor()
	w.refresh()
}

// modified says when a note changed, briefly.
func modified(t, now time.Time) string {
	switch {
	case t.IsZero():
		return ""
	case t.YearDay() == now.YearDay() && t.Year() == now.Year():
		return t.Format("今日 15:04")
	case t.Year() == now.Year():
		return fmt.Sprintf("%d月%d日", t.Month(), t.Day())
	}
	return fmt.Sprintf("%d年%d月%d日", t.Year(), t.Month(), t.Day())
}
