package main

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// The page of Disclosure and Link (docs/ADR/0087-disclosure-link.md).

// pageDisclosure shows Disclosure on its own and as an accordion, and Link
// enabled, disabled, in a smaller style and wrapped.
func pageDisclosure() core.Widget { return disclosurePage{} }

// disclosurePage keeps the state the page shows: which section of the
// accordion is open, and what the links did.
type disclosurePage struct{ core.StatefulBase }

type disclosurePageState struct {
	core.StateBase
	// faq is the open question of the accordion, or -1.
	faq int
	// all opens or closes every section of the uncontrolled group.
	all  bool
	said string
}

func (disclosurePage) CreateState() core.State { return &disclosurePageState{faq: 0} }

func (s *disclosurePageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	body := th.Text.Body
	body.Color = th.Color.Ink2
	text := func(s string) core.Widget { return basic.Text{Content: s, Style: body} }
	pressed := func(what string) func() { return func() { s.SetState(func() { s.said = what }) } }

	questions := []struct{ q, a string }{
		{"gowaku は cgo を使いますか？", "使いません。Windows でも Linux でも macOS でも、gcc 無しでクロスコンパイルできます。"},
		{"日本語の入力はできますか？", "できます。IME の文節の下線、候補窓の位置、ルビと縦書きまで扱います。"},
		{"ライセンスは？", "MIT です。"},
	}
	faq := make([]core.Widget, len(questions))
	for i, item := range questions {
		faq[i] = control.Disclosure{Title: item.q, Child: text(item.a), Open: s.faq == i, OnChange: func(open bool) {
			s.SetState(func() {
				s.faq = -1
				if open {
					s.faq = i
				}
			})
		}}
	}

	said := s.said
	if said == "" {
		said = "リンクを押すと、ここに出ます。"
	}
	caption := th.Text.Caption
	return basic.Column{Size: layout.MainMin, Spacing: theme.Space24, Cross: layout.CrossStretch, Children: []core.Widget{
		heading(th, "ディスクロージャーとリンク", "見出しで開閉する Disclosure と、本文の中に置く Link。"),
		collection.Group{Title: "ひとつずつ（自分で開閉を持つ）", Children: []core.Widget{
			basic.Pad(theme.Space8, basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{
				control.Disclosure{Title: "詳細設定", Child: text("閉じている間、中身はレイアウトされず、Tab もスクリーンリーダーも素通りします。")},
				control.Disclosure{Title: "最初から開いている", Open: true, Child: text("Open を true にすると開いた状態で始まります。")},
				control.Disclosure{Title: "通知", Header: basic.Row{Size: layout.MainMin, Spacing: theme.Space6, Children: []core.Widget{
					basic.Text{Content: "通知", Style: th.Text.Headline}, control.Badge{Count: 3},
				}}, Child: text("見出しにはバッジなどのウィジェットも置けます。")},
				control.Disclosure{Title: "すべて開く・閉じるボタンに従う", Open: s.all, Child: control.Button{Label: "中のボタン", OnPress: pressed("中のボタン")}},
				basic.Pad(theme.Space8, basic.Row{Size: layout.MainMin, Children: []core.Widget{
					control.Button{Label: "上の項目を親から開閉", OnPress: func() { s.SetState(func() { s.all = !s.all }) }},
				}}),
			}}),
		}},
		collection.Group{Title: "アコーディオン（ひとつだけ開く）", Children: []core.Widget{
			basic.Pad(theme.Space8, basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: faq}),
		}},
		collection.Group{Title: "リンク", Children: []core.Widget{
			basic.Pad(theme.Space16, basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space12, Children: []core.Widget{
				basic.Row{Size: layout.MainMin, Spacing: theme.Space4, Children: []core.Widget{
					text("同期はオフです。"),
					control.Link{Label: "オンにする", OnPress: pressed("オンにする")},
				}},
				basic.Row{Size: layout.MainMin, Spacing: theme.Space4, Children: []core.Widget{
					text("この機能は使えません。"),
					control.Link{Label: "詳しくは（無効）"},
				}},
				control.Link{Label: "小さな字のリンク", Style: caption, OnPress: pressed("小さな字のリンク")},
				basic.SizedBox{Width: 240, Child: control.Link{
					Label: "長いリンクは幅で折り返し、ポインタを載せるとすべての行に下線が引かれます", OnPress: pressed("長いリンク"),
				}},
				// In a Row the link takes the width of its text; a right-to-left
				// text alone in a column takes the whole width, as Text does.
				basic.Row{Size: layout.MainMin, Children: []core.Widget{
					control.Link{Label: "اقرأ المزيد", OnPress: pressed("اقرأ المزيد")},
				}},
				text(said),
			}}),
		}},
	}}
}
