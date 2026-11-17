// Command todo is a small to-do application that runs on every backend: the
// same code runs on Windows with Direct3D 11 and on Linux with Vulkan. Enter adds what the
// field holds.
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

const (
	showAll = iota
	showOpen
	showDone
)

type item struct {
	id    int
	title string
	done  bool
}

type todo struct{ gowaku.StatefulBase }

type todoState struct {
	gowaku.StateBase
	draft  string
	items  []item
	lastID int
	filter int
}

func (todo) CreateState() gowaku.State {
	s := &todoState{}
	for _, title := range []string{"牛乳を買う", "gowaku の README を書く", "Reply to the review"} {
		s.lastID++
		s.items = append(s.items, item{id: s.lastID, title: title})
	}
	s.items[0].done = true
	return s
}

func (s *todoState) add(title string) {
	title = strings.TrimSpace(title)
	s.SetState(func() {
		s.draft = ""
		if title != "" {
			s.lastID++
			s.items = append(s.items, item{id: s.lastID, title: title})
		}
	})
}

func (s *todoState) toggle(id int, done bool) {
	s.SetState(func() {
		for i := range s.items {
			if s.items[i].id == id {
				s.items[i].done = done
			}
		}
	})
}

func (s *todoState) remove(id int) {
	s.SetState(func() {
		for i := range s.items {
			if s.items[i].id == id {
				s.items = append(s.items[:i], s.items[i+1:]...)
				return
			}
		}
	})
}

func (s *todoState) visible(it *item) bool {
	switch s.filter {
	case showOpen:
		return !it.done
	case showDone:
		return it.done
	}
	return true
}

func (s *todoState) row(it item) gowaku.Widget {
	return gowaku.Keyed(it.id, basic.Padding{
		Insets: geom.Insets{Left: theme.Space16, Right: theme.Space8, Top: theme.Space6, Bottom: theme.Space6},
		Child: basic.Row{Spacing: theme.Space8, Cross: layout.CrossCenter, Children: []gowaku.Widget{
			basic.Expanded{Child: control.Checkbox{
				Value: it.done, Label: it.title,
				OnChange: func(v bool) { s.toggle(it.id, v) },
			}},
			control.Button{Label: "削除", Hint: it.title + " を削除", Style: control.ButtonQuiet, OnPress: func() { s.remove(it.id) }},
		}},
	})
}

func (s *todoState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	open := 0
	rows := make([]gowaku.Widget, 0, len(s.items))
	for i := range s.items {
		if !s.items[i].done {
			open++
		}
		if s.visible(&s.items[i]) {
			rows = append(rows, s.row(s.items[i]))
		}
	}
	remaining := fmt.Sprintf("残り %d 件", open)
	list := gowaku.Widget(collection.Group{Title: "やること", Children: rows})
	if len(rows) == 0 {
		list = basic.Center{Child: basic.Text{Content: "ここには何もありません", Style: text.Style{Size: 13, Color: th.Color.Ink2}}}
	}
	return basic.Pad(theme.Space24, basic.Column{Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []gowaku.Widget{
		interact.Semantics{Role: semantics.RoleHeading, Level: 1, Label: "Todo", Child: basic.Text{
			Content: "Todo", Style: text.Style{Size: 24, Weight: 700, Color: th.Color.Ink},
		}},
		basic.Row{Spacing: theme.Space8, Children: []gowaku.Widget{
			basic.Expanded{Child: field.Input{
				Value:       s.draft,
				Placeholder: "新しいやること",
				OnChange:    func(v string) { s.SetState(func() { s.draft = v }) },
				OnSubmit:    s.add,
			}},
			control.Button{Label: "追加", Style: control.ButtonPrimary, OnPress: func() { s.add(s.draft) }},
		}},
		control.SegmentedControl{
			Options: []string{"すべて", "未完了", "完了"}, Selected: s.filter, Label: "表示",
			OnChange: func(i int) { s.SetState(func() { s.filter = i }) },
		},
		basic.Expanded{Child: basic.Scroll{Child: list}},
		// A live region: screen readers say the count when it changes.
		interact.Semantics{Role: semantics.RoleText, Label: remaining, Live: semantics.LivePolite, Child: basic.Text{
			Content: remaining, Style: text.Style{Size: 12, Color: th.Color.Ink2},
		}},
	}})
}

func main() {
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the window to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}
	err := gowaku.Run(gowaku.App{Title: "gowaku: todo", Width: 520, Height: 560, Home: todo{}})
	if err != nil {
		log.Fatal(err)
	}
}
