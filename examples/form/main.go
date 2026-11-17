// Command form is a sample of text input: text fields with Japanese input, a
// checkbox and buttons. Tab moves the focus.
package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

type form struct{ gowaku.StatefulBase }

type formState struct {
	gowaku.StateBase
	name, memo string
	agreed     bool
	sent       []string
}

func (form) CreateState() gowaku.State { return &formState{} }

func heading(th *theme.Theme, s string) gowaku.Widget {
	return basic.Text{Content: s, Style: text.Style{Size: 12, Color: th.Color.Ink2}}
}

func (s *formState) send() {
	s.SetState(func() {
		s.sent = append(s.sent, s.name+"："+strings.ReplaceAll(s.memo, "\n", " / "))
		s.name, s.memo, s.agreed = "", "", false
	})
}

func (s *formState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	var press func()
	if s.agreed && s.name != "" {
		press = s.send
	}
	rows := make([]gowaku.Widget, 0, 7+len(s.sent))
	rows = append(rows,
		basic.Text{Content: "入力の見本", Style: th.Text.LargeTitle},
		heading(th, "名前"),
		field.Input{
			Value:       s.name,
			Placeholder: "山田 太郎",
			OnChange:    func(v string) { s.SetState(func() { s.name = v }) },
		},
		heading(th, "メモ（Enter で改行）"),
		field.Input{
			Value:     s.memo,
			Multiline: true,
			OnChange:  func(v string) { s.SetState(func() { s.memo = v }) },
		},
		control.Checkbox{
			Value:    s.agreed,
			Label:    "内容を確認しました",
			OnChange: func(v bool) { s.SetState(func() { s.agreed = v }) },
		},
		control.Button{Label: "送信", OnPress: press},
	)
	for _, line := range s.sent {
		rows = append(rows, basic.Text{Content: line, Style: th.Text.Body})
	}
	return basic.Pad(32, basic.Column{Spacing: 12, Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Align{Child: basic.SizedBox{Width: 480, Child: basic.Column{
			Spacing: 12, Cross: layout.CrossStretch, Children: rows,
		}}},
	}})
}

func main() {
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the window to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}
	err := gowaku.Run(gowaku.App{
		Title:  "gowaku: form",
		Width:  560,
		Height: 480,
		Home:   form{},
	})
	if err != nil {
		log.Fatal(err)
	}
}
