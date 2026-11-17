package main

import (
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// display shows the toolbar, tabs, chips, badges, tooltips and cards of Phase
// 15a, with buttons that raise a toast and a sheet.
func (s *galleryState) display(th *theme.Theme) []gowaku.Widget {
	ctx := s.Context()
	chip := func(label string) gowaku.Widget {
		return control.Chip{Label: label, Selected: s.tags[label], OnChange: func(v bool) { s.set(func() { s.tags[label] = v }) }}
	}
	ink := th.Color.Ink2
	return []gowaku.Widget{
		heading(th, "表示", "ツールバー、タブ、チップ、バッジ、ツールチップ、カード、通知。"),
		control.Card{Child: nav.Toolbar{Label: "書式", Items: []nav.ToolbarItem{
			{Action: interact.Action{Label: "太字(&B)", Checkable: true, Checked: s.bold, OnInvoke: func() { s.set(func() { s.bold = !s.bold }) }}, ShowLabel: true},
			{Action: interact.Action{Label: "固定", Checkable: true, Checked: s.pinned, OnInvoke: func() { s.set(func() { s.pinned = !s.pinned }) }}, Icon: s.star},
			{Separator: true},
			{Action: interact.Action{Label: "追加", OnInvoke: func() {}}, Icon: s.plus},
			{Action: interact.Action{Label: "クラウドへ"}, Icon: s.cloud},
			{Spacer: true},
			{Action: interact.Action{Label: "通知", OnInvoke: func() {
				overlay.ShowToast(ctx, overlay.Toast{Message: "保存しました", Action: "元に戻す", OnAction: func() {}})
			}}, ShowLabel: true},
			{Action: interact.Action{Label: "シート", OnInvoke: s.sheet}, ShowLabel: true},
		}}},
		basic.SizedBox{Height: 132, Child: control.Tabs{Label: "表示のタブ", Selected: s.tab, OnChange: func(i int) { s.set(func() { s.tab = i }) }, Pages: []control.TabPage{
			{Label: "チップ", Child: basic.Pad(theme.Space12, basic.Wrap{Spacing: 8, RunSpacing: 8, Children: []gowaku.Widget{
				chip("仕事"), chip("家族"), chip("趣味"),
				control.Chip{Label: "削除できる", OnRemove: func() {}},
				control.Chip{Label: "タグ"},
			}})},
			{Label: "バッジ", Child: basic.Pad(theme.Space12, basic.Row{Size: layout.MainMin, Spacing: 28, Cross: layout.CrossCenter, Children: []gowaku.Widget{
				control.Badge{Count: 3, Child: basic.Icon{Path: s.home, Size: 24, Color: ink}},
				control.Badge{Count: 120, Child: basic.Icon{Path: s.cloud, Size: 24, Color: ink}},
				control.Badge{Dot: true, Child: basic.Icon{Path: s.star, Size: 24, Color: ink}},
				control.Badge{Label: "新着", Quiet: true},
			}})},
			{Label: "ツールチップ", Child: basic.Pad(theme.Space12, basic.Row{Size: layout.MainMin, Spacing: 8, Children: []gowaku.Widget{
				overlay.Tooltip{Message: "ポインタを止めると出ます", Child: control.Button{Label: "ここに載せる", OnPress: func() {}}},
				overlay.Tooltip{Message: "キーボードでも出ます", Child: control.Button{Label: "Tab で来る", OnPress: func() {}}},
			}})},
		}}},
		basic.Row{Spacing: theme.Space12, Cross: layout.CrossStart, Children: []gowaku.Widget{
			basic.Expanded{Child: control.Card{Label: "ただのカード", Child: basic.Column{Size: layout.MainMin, Spacing: 6, Cross: layout.CrossStretch, Children: []gowaku.Widget{
				basic.Text{Content: "カード", Style: th.Text.Headline},
				basic.Divider{},
				basic.Text{Content: "面と線と影だけ。", Style: th.Text.Body},
			}}}},
			basic.Expanded{Child: control.Card{OnPress: func() {}, Child: basic.Column{Size: layout.MainMin, Spacing: 6, Cross: layout.CrossStart, Children: []gowaku.Widget{
				basic.Text{Content: "押せるカード", Style: th.Text.Headline},
				basic.Text{Content: "押すと少し沈みます。", Style: th.Text.Body},
			}}}},
		}},
	}
}

// forms shows the fields, pickers and validation.
func (s *galleryState) forms(th *theme.Theme) []gowaku.Widget {
	fruits := []field.DropdownOption[string]{
		{Value: "りんご", Label: "りんご"}, {Value: "みかん", Label: "みかん"},
		{Value: "ぶどう", Label: "ぶどう"}, {Value: "もも", Label: "もも", Disabled: true},
	}
	cities := []string{"東京", "大阪", "名古屋", "札幌", "福岡", "京都", "神戸", "横浜"}
	return []gowaku.Widget{
		heading(th, "フォーム", "検索、選択、候補、日付と時刻、検証。"),
		collection.Group{Title: "入力", Children: []gowaku.Widget{
			collection.GroupRow{Label: "検索", Child: basic.SizedBox{Width: 240, Child: field.Search{
				Value: s.query, OnChange: func(v string) { s.set(func() { s.query = v }) },
			}}},
			collection.GroupRow{Label: "果物", Child: field.DropdownMenu[string]{
				Value: s.fruit, Options: fruits, OnChange: func(v string) { s.set(func() { s.fruit = v }) },
			}},
			collection.GroupRow{Label: "都市", Child: basic.SizedBox{Width: 240, Child: field.Autocomplete[string]{
				Value: s.city, Options: cities, OnChange: func(v string) { s.set(func() { s.city = v }) },
			}}},
			collection.GroupRow{Label: "日付", Child: basic.SizedBox{Width: 200, Child: field.DatePicker{
				Value: s.date, Today: today, ShowToday: true,
				OnChange: func(v time.Time) { s.set(func() { s.date = v }) },
			}}},
			collection.GroupRow{Label: "時刻", Child: field.TimePicker{
				Value: s.clock, MinuteStep: 5, OnChange: func(v time.Duration) { s.set(func() { s.clock = v }) },
			}},
			collection.GroupRow{Label: "プラン", Child: control.RadioGroup[int]{
				Label: "プラン", Horizontal: true, Value: s.plan, OnChange: func(v int) { s.set(func() { s.plan = v }) },
				Options: []control.RadioOption[int]{{Value: 0, Label: "無料"}, {Value: 1, Label: "個人"}, {Value: 2, Label: "法人", Disabled: true}},
			}},
		}},
		field.Form{Controller: &s.form, OnSubmit: func() { overlay.ShowToast(s.Context(), overlay.Toast{Message: "送信しました"}) }, Child: collection.Group{Title: "登録", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, basic.Column{Size: layout.MainMin, Spacing: theme.Space12, Cross: layout.CrossStretch, Children: []gowaku.Widget{
				field.FormItem[string]{Value: s.nick, Required: true, Child: field.Input{
					Label: "ニックネーム", Value: s.nick, OnChange: func(v string) { s.set(func() { s.nick = v }) },
				}},
				field.FormItem[string]{Value: s.code, Validators: []func(string) string{field.MinLength(7, "郵便番号は 7 桁です")}, Child: field.Input{
					Label: "郵便番号", Value: s.code, Placeholder: "1000001", OnChange: func(v string) { s.set(func() { s.code = v }) },
				}},
				field.FormItem[bool]{Value: s.agree, Required: true, Child: control.Checkbox{
					Label: "規約に同意する", Value: s.agree, OnChange: func(v bool) { s.set(func() { s.agree = v }) },
				}},
				basic.Row{Size: layout.MainMin, Children: []gowaku.Widget{
					control.Button{Label: "送信", Style: control.ButtonPrimary, OnPress: func() { s.form.Submit() }},
				}},
			}}),
		}}},
	}
}

// sheet slides in a sheet from the trailing edge.
func (s *galleryState) sheet() {
	ctx := s.Context()
	overlay.ShowSheet(ctx, overlay.Sheet{Edge: overlay.SheetTrailing, Label: "詳細"}, func(dismiss func()) gowaku.Widget {
		th := basic.ThemeOf(ctx)
		return basic.Column{Size: layout.MainMin, Spacing: theme.Space12, Cross: layout.CrossStart, Children: []gowaku.Widget{
			basic.Text{Content: "詳細", Style: th.Text.Title},
			basic.Text{Content: "横から出るシートです。Escape か外側のクリックで閉じます。", Style: th.Text.Body},
			control.Button{Label: "閉じる", OnPress: dismiss},
		}}
	})
}
