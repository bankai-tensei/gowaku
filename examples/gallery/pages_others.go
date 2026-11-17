package main

import (
	"log"
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of the widgets the other pages leave out, and a small
// app with its own Navigator.

// leafSVG is a small drawing that SVG renders in the ink of the theme.
const leafSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path d="M5 19c0-8 6-14 15-14 0 9-6 15-14 15l-1-1z" fill="currentColor" opacity=".35"/>
<path d="M5 19 15 9" stroke="currentColor" stroke-width="1.8" fill="none" stroke-linecap="round"/>
</svg>`

// brand is a token of the gallery itself, kept on the theme as an extension.
type brand struct{ Ribbon geom.Color }

// ribbon shows the brand token of the theme above it, or Fill without one.
type ribbon struct {
	gowaku.StatelessBase
	label string
}

func (r ribbon) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	b := theme.ExtensionOr(th, brand{Ribbon: th.Color.Fill})
	style := th.Text.Caption
	style.Color = th.Color.Ink
	return basic.Container{Color: b.Ribbon, Radius: 6, Padding: geom.Insets{Left: 10, Right: 10, Top: 4, Bottom: 4},
		Child: basic.Text{Content: r.label, Style: style}}
}

// otherState is what the page of the other widgets keeps.
type otherState struct {
	steps, choice int
	day           time.Time
	shown         bool
	leaf          *paint.SVG
	log           string
}

func (s *galleryState) others(th *theme.Theme) []gowaku.Widget {
	o := &s.other
	if o.leaf == nil {
		leaf, err := paint.ParseSVG([]byte(leafSVG))
		if err != nil {
			log.Fatal(err)
		}
		o.leaf, o.steps, o.day = leaf, 3, today
	}
	did := func(what string) func() { return func() { s.set(func() { o.log = what }) } }
	edit := []overlay.MenuItem{
		{Action: interact.Action{Label: "元に戻す", Shortcut: interact.Shortcut{Key: input.KeyZ, Modifiers: input.ModControl}, OnInvoke: did("元に戻す")}},
		{Separator: true},
		{Action: interact.Action{Label: "切り取り", Shortcut: interact.Shortcut{Key: input.KeyX, Modifiers: input.ModControl}, OnInvoke: did("切り取り")}},
		{Action: interact.Action{Label: "コピー", Shortcut: interact.Shortcut{Key: input.KeyC, Modifiers: input.ModControl}, OnInvoke: did("コピー")}},
		{Action: interact.Action{Label: "貼り付け", OnInvoke: did("貼り付け")}},
	}
	view := []overlay.MenuItem{
		{Action: interact.Action{Label: "サイドバー", Checkable: true, Checked: true, OnInvoke: did("サイドバー")}},
		{Action: interact.Action{Label: "拡大"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "100%", OnInvoke: did("100%")}},
			{Action: interact.Action{Label: "150%", OnInvoke: did("150%")}},
		}},
	}
	body := th.Text.Body
	body.Color = th.Color.Ink
	strong := body
	strong.Weight = 600
	accent := body
	accent.Color = th.Color.AccentInk
	rich := basic.RichText{Spans: []text.Span{
		{Text: "RichText は ", Style: body},
		{Text: "太字", Style: strong},
		{Text: " や ", Style: body},
		{Text: "色", Style: accent},
		{Text: " を混ぜ、", Style: body},
		{Text: "漢字", Style: body, Ruby: "かんじ"},
		{Text: "にルビも振れます。", Style: body},
	}}
	tiles := make([]gowaku.Widget, 6)
	for i := range tiles {
		tiles[i] = basic.DecoratedBox{
			Decoration: layout.Decoration{Color: th.Color.Fill, Radius: 10},
			Child:      basic.Center{Child: basic.Text{Content: "タイル " + strconv.Itoa(i+1)}},
		}
	}
	status := o.log
	if status == "" {
		status = "メニューか右クリックで選んだものがここに出ます。"
	}
	return []gowaku.Widget{
		heading(th, "そのほか", "ほかのページに出てこない部品をまとめました。"),
		collection.Group{Title: "メニュー", Children: []gowaku.Widget{
			basic.Pad(theme.Space8, overlay.MenuBar{Items: []overlay.MenuItem{
				{Action: interact.Action{Label: "編集(&E)"}, Items: edit},
				{Action: interact.Action{Label: "表示(&V)"}, Items: view},
			}}),
			overlay.ContextMenu{Items: edit, Child: basic.Pad(theme.Space16, basic.Text{Content: "ここを右クリックすると、同じ編集のメニューが出ます。"})},
			collection.GroupRow{Label: "選んだもの", Child: basic.Text{Content: status}},
		}},
		collection.Group{Title: "進み具合と数", Children: []gowaku.Widget{
			collection.GroupRow{Label: "書き出し", Child: basic.SizedBox{Width: 220, Child: control.ProgressBar{Value: float32(o.steps) / 10, Label: "書き出し"}}},
			collection.GroupRow{Label: "同期", Child: basic.Row{Size: layout.MainMin, Spacing: 12, Cross: layout.CrossCenter, Children: []gowaku.Widget{
				control.ProgressRing{Value: float32(o.steps) / 10, Label: "同期"},
				control.ProgressRing{Value: 1, Label: "完了"},
			}}},
			collection.GroupRow{Label: "段階", Child: control.NumberStepper{Value: o.steps, Min: 0, Max: 10,
				OnChange: func(v int) { s.set(func() { o.steps = v }) }}},
			collection.GroupRow{Label: "ひとつだけの Radio", Child: control.Radio{Label: "毎朝知らせる", Selected: o.choice == 1,
				OnSelect: func() { s.set(func() { o.choice = 1 - o.choice }) }}},
		}},
		collection.Group{Title: "文字と絵", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, rich),
			collection.GroupRow{Label: "SVG", Child: basic.SVG{Data: o.leaf, Size: 32, Label: "葉"}},
			collection.GroupRow{Label: "テーマの拡張", Detail: "ThemeWithExtension で持たせた色",
				Child: basic.Row{Size: layout.MainMin, Spacing: 8, Children: []gowaku.Widget{
					ribbon{label: "拡張なし"},
					basic.Theme{Data: theme.WithExtension(th, brand{Ribbon: th.Color.AccentTint}), Child: ribbon{label: "ブランドの色"}},
				}}},
			collection.GroupRow{Label: "フェード", Child: basic.Row{Size: layout.MainMin, Spacing: 12, Cross: layout.CrossCenter, Children: []gowaku.Widget{
				control.Button{Label: "切り替える", OnPress: func() { s.set(func() { o.shown = !o.shown }) }},
				basic.AnimatedOpacity{Opacity: map[bool]float32{true: 1, false: 0.2}[o.shown], Child: basic.Text{Content: "現れたり消えたり"}},
			}}},
		}},
		collection.Group{Title: "並べる", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, basic.Grid{MinColumnWidth: 120, Spacing: 8, RunSpacing: 8, TileHeight: 56, Children: tiles}),
			basic.Pad(theme.Space8, basic.SizedBox{Height: 160, Child: collection.List{ItemCount: 10_000, ItemExtent: 32,
				ItemBuilder: func(i int) gowaku.Widget {
					return basic.Pad(theme.Space8, basic.Text{Content: "ListView の " + strconv.Itoa(i+1) + " 行目（全 10,000 行）"})
				}}}),
		}},
		collection.Group{Title: "カレンダー", Children: []gowaku.Widget{
			basic.Pad(theme.Space16, field.Calendar{Value: o.day, Today: today, Marks: scheduleMarks(th),
				OnChange: func(d time.Time) { s.set(func() { o.day = d }) }}),
		}},
	}
}

// scheduleMarks marks the days of a made-up schedule around today: the
// accent ink for meetings, Success for days off and Danger for deadlines. The
// 20th has four, of which the calendar shows the first three.
func scheduleMarks(th *theme.Theme) func(day time.Time) []geom.Color {
	c := &th.Color
	marks := map[int][]geom.Color{
		-6: {c.Success},
		-2: {c.AccentInk},
		0:  {c.AccentInk, c.Danger},
		2:  {c.AccentInk},
		4:  {c.Danger},
		8:  {c.AccentInk, c.Success},
		11: {c.AccentInk, c.Success, c.Danger, c.Ink2},
		24: {c.AccentInk},
	}
	return func(day time.Time) []geom.Color {
		d := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		t := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		return marks[int(d.Sub(t).Hours()/24)]
	}
}

// The navigation page: a list of notes whose rows open a page each.

type galleryNote struct {
	title, body string
	hue         uint32
}

var galleryNotes = []galleryNote{
	{"買い物", "米、味噌、ねぎ、豆腐。帰りに郵便局へ寄る。", 0x4f9e6b},
	{"京都旅行", "金曜の九時に出る。祇園の近くの宿を予約する。", 0xdb7348},
	{"読みたい本", "枕草子、こころ、雪国。図書館の返却は月曜まで。", 0x5c7ad1},
	{"会議メモ", "デザインのレビュー。Navigator と Hero を見せる。", 0xcc5c7a},
	{"レシピ", "肉じゃが: じゃがいも 3 個、玉ねぎ 1 個、牛肉 200g。", 0xc79933},
}

// noteIcon is the square of a note with its first character.
func noteIcon(th *theme.Theme, n galleryNote, side float32) gowaku.Widget {
	style := th.Text.Headline
	style.Size, style.Color = side*0.45, geom.RGBA(1, 1, 1, 1)
	return basic.Container{Width: side, Height: side, Radius: side / 4, Color: geom.Hex(n.hue),
		Child: basic.Center{Child: basic.Text{Content: string([]rune(n.title)[:1]), Style: style}}}
}

type noteList struct{ gowaku.StatelessBase }

func (noteList) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	navigator := nav.NavigatorOf(ctx)
	rows := make([]gowaku.Widget, len(galleryNotes))
	for i, n := range galleryNotes {
		rows[i] = interact.Pressable{Label: n.title, OnPress: func() { navigator.PushNamed("/notes/" + strconv.Itoa(i)) },
			Build: func(p interact.PressState) gowaku.Widget {
				bg := geom.Color{}
				if p.Hovered {
					bg = th.Color.Fill
				}
				return basic.Container{Color: bg, Radius: 8, Padding: geom.Symmetric(theme.Space16, theme.Space8), Child: basic.Row{
					Spacing: 12, Cross: layout.CrossCenter, Children: []gowaku.Widget{
						nav.Hero{Tag: n.title, Child: noteIcon(th, n, 36)},
						basic.Expanded{Child: basic.Text{Content: n.title}},
					}}}
			}}
	}
	return basic.Container{Color: th.Color.Canvas, Child: basic.Pad(theme.Space28, basic.Column{
		Size: layout.MainMin, Spacing: theme.Space16, Cross: layout.CrossStretch, Children: []gowaku.Widget{
			heading(th, "ナビゲーション", "行を押すとページが重なり、アイコンが飛んでいきます。Alt+← とマウスの戻るボタンで戻ります。"),
			collection.Group{Children: rows},
		}})}
}

type noteDetail struct {
	gowaku.StatelessBase
	note galleryNote
}

func (d noteDetail) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	title := th.Text.LargeTitle
	title.Color = th.Color.Ink
	return basic.Container{Color: th.Color.Canvas, Child: basic.Pad(theme.Space28, basic.Column{
		Size: layout.MainMin, Spacing: theme.Space20, Cross: layout.CrossStart, Children: []gowaku.Widget{
			nav.BackButton{Label: "メモ"},
			basic.Row{Size: layout.MainMin, Spacing: 16, Cross: layout.CrossCenter, Children: []gowaku.Widget{
				nav.Hero{Tag: d.note.title, Child: noteIcon(th, d.note, 72)},
				basic.Text{Content: d.note.title, Style: title},
			}},
			basic.Text{Content: d.note.body},
		}})}
}

func (s *galleryState) navigation() gowaku.Widget {
	return nav.Navigator{
		Home: noteList{},
		Routes: []nav.RoutePattern{{Pattern: "/notes/:id", Build: func(p nav.RouteParams) gowaku.Widget {
			i, ok := p.Int("id")
			if !ok || i < 0 || i >= len(galleryNotes) {
				return basic.Center{Child: basic.Text{Content: "そのメモはありません"}}
			}
			return noteDetail{note: galleryNotes[i]}
		}}},
	}
}
