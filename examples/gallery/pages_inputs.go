package main

import (
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of NumberField, RangeSlider and TokenField (ADR 0093).

// pageInputs returns the page of the number, range and token inputs.
func pageInputs() core.Widget { return inputsPage{} }

// inputsPage keeps the values its inputs show, as an app would.
type inputsPage struct{ core.StatefulBase }

type inputsPageState struct {
	core.StateOf[inputsPage]
	count, price, ratio float64
	low, high           float32
	rtlLow, rtlHigh     float32
	tags, mail          []string
}

// CreateState implements core.StatefulWidget.
func (inputsPage) CreateState() core.State {
	return &inputsPageState{
		count: 3, price: 128000, ratio: 0.25,
		low: 20000, high: 60000, rtlLow: 0.2, rtlHigh: 0.7,
		tags: []string{"デザイン", "Go", "日本語"},
		mail: []string{"hanako@example.jp"},
	}
}

// Build implements core.State.
func (s *inputsPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	title, section, caption := th.Text.LargeTitle, th.Text.Section, th.Text.Caption
	title.Color, section.Color, caption.Color = th.Color.Ink, th.Color.Ink2, th.Color.Ink2
	set := func(fn func()) { s.SetState(fn) }
	box := func(w float32, child core.Widget) core.Widget { return basic.SizedBox{Width: w, Child: child} }
	row := func(children ...core.Widget) core.Widget {
		return basic.Row{Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossEnd, Children: children}
	}
	yen := func(v float32) string { return "¥" + basic.FormatInt(ctx, int(v)) }

	return basic.Scroll{Child: basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space16,
		Children: []core.Widget{
			basic.Text{Content: "数値・範囲・タグの入力", Style: title},

			basic.Text{Content: "NumberField — 直接入力、↑↓（Shift で 10 倍、Ctrl で 1/10）、ホイール（フォーカス中のみ）、右端のつまみをドラッグ", Style: section},
			row(
				box(120, field.NumberField{Label: "数量", Value: s.count, Min: 0, Max: 99, OnChange: func(v float64) { set(func() { s.count = v }) }}),
				box(160, field.NumberField{Label: "価格（円）", Value: s.price, Step: 100, Grouping: true, OnChange: func(v float64) { set(func() { s.price = v }) }}),
				box(120, field.NumberField{Label: "割合", Value: s.ratio, Min: 0, Max: 1, Step: 0.05, OnChange: func(v float64) { set(func() { s.ratio = v }) }}),
				box(120, field.NumberField{Label: "無効", Value: 42}),
			),
			basic.Text{Content: "全角の「１２．５」も受け付けます。数でない文字は赤枠になり、確定すると元の値に戻ります。", Style: caption},

			basic.Text{Content: "RangeSlider — つまみ 2 つ、間のバーで範囲ごと移動、Tab でつまみを移る", Style: section},
			box(360, control.RangeSlider{
				Label: "価格帯", Low: s.low, High: s.high, Min: 0, Max: 100000, Step: 1000, Snap: true, MinGap: 5000,
				OnChange: func(l, h float32) { set(func() { s.low, s.high = l, h }) },
			}),
			basic.Text{Content: "価格帯: " + yen(s.low) + " – " + yen(s.high) + "（最小の差 ¥5,000）", Style: caption},
			row(
				box(220, control.RangeSlider{
					Label: "右から左", Low: s.rtlLow, High: s.rtlHigh, RTL: true,
					OnChange: func(l, h float32) { set(func() { s.rtlLow, s.rtlHigh = l, h }) },
				}),
				box(220, control.RangeSlider{Label: "無効", Low: 0.3, High: 0.6}),
			),
			basic.Text{Content: "右から左: " + strconv.FormatFloat(float64(s.rtlLow), 'f', 2, 32) + " – " + strconv.FormatFloat(float64(s.rtlHigh), 'f', 2, 32), Style: caption},

			basic.Text{Content: "TokenField — Enter か「、」「,」で確定。変換中の Enter はトークンにしない。先頭で Backspace を 2 回で前のトークンを消す", Style: section},
			box(420, field.TokenField{
				Label: "タグ", Tokens: s.tags, Placeholder: "タグを追加", MaxTokens: 8,
				OnChange: func(t []string) { set(func() { s.tags = t }) },
			}),
			box(420, field.TokenField{
				Label: "宛先", Tokens: s.mail, Placeholder: "メールアドレス",
				Validate: func(t string) string {
					if !strings.Contains(t, "@") {
						return "メールアドレスの形ではありません"
					}
					return ""
				},
				OnChange: func(t []string) { set(func() { s.mail = t }) },
			}),
			box(420, field.TokenField{Label: "無効", Tokens: []string{"読み取り専用", "表示のみ"}}),
		},
	}}}
}
