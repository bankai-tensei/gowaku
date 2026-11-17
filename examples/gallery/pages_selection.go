package main

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

// The page of SelectionArea: the texts under it select with the mouse and the
// keyboard, and copy. See docs/ADR/0089-selection-area.md.

// selectionLog is the error a log might show, the text people want to copy.
const selectionLog = `2026-11-13 10:42:07 ERROR sync: upload "報告書.pdf" failed
  cause: dial tcp 192.0.2.10:443: i/o timeout
  retry 3/3 in 30s`

// pageSelection shows a log, ruby, vertical writing and a bidi line in one
// SelectionArea, with a button that keeps working inside it.
func pageSelection() core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		th := basic.ThemeOf(ctx)
		ink2 := th.Text.Caption.WithColor(th.Color.Ink2)
		body := th.Text.Body.WithColor(th.Color.Ink)
		mono := th.Text.Code.WithColor(th.Color.Ink)
		vertical := th.Text.Reading.WithColor(th.Color.Ink)
		vertical.Orientation = text.Vertical
		return basic.Padding{Insets: geom.All(theme.Space24), Child: field.SelectionArea{Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space16,
			Children: []core.Widget{
				basic.Text{Content: "選択できるテキスト", Style: th.Text.Title.WithColor(th.Color.Ink)},
				basic.Text{
					Content: "ドラッグで範囲選択、ダブルクリックで単語、トリプルクリックで行。Shift+クリックと Shift+矢印で広げ、Ctrl+C でコピー、Ctrl+A ですべて選択。右クリックでメニュー。",
					Style:   ink2,
				},
				basic.Container{
					Color: th.Color.Fill, Radius: th.Radius.Control, Padding: geom.All(theme.Space16),
					Child: basic.Text{Content: selectionLog, Style: mono},
				},
				basic.RichText{Style: th.Text.Reading.WithColor(th.Color.Ink), Spans: []text.Span{
					{Text: "コピーするとルビは落ち、"},
					{Text: "親文字", Ruby: "おやもじ"},
					{Text: "だけが残ります。"},
					{Text: "東京", Ruby: "とうきょう"},
					{Text: "の空は"},
					{Text: "高", Ruby: "たか"},
					{Text: "い。"},
				}},
				basic.Row{Spacing: theme.Space24, Cross: layout.CrossStart, Children: []core.Widget{
					basic.SizedBox{Height: 180, Child: basic.Text{
						Content: "縦書きの文章も、列をまたいで選べます。",
						Style:   vertical,
					}},
					basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
						basic.Text{Content: "日本語と עברית と العربية が混ざった行", Style: body},
						basic.Text{Content: "ボタンの上を押しても選択は始まりません。", Style: ink2},
						control.Button{Label: "押せるボタン", OnPress: func() {}},
					}},
				}},
			},
		}}}
	}}
}
