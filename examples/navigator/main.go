// Command navigator is a list of notes that opens each note on a page of its
// own: the icon of the note flies from its row to the page (Hero), and the page
// slides in over the list. Go back with the button at the top left, Alt+Left or
// the back button of a mouse; Alt+Right and the forward button open the note
// again.
//
//	navigator                  opens the list
//	navigator -open /notes/3   opens on the page of a note
//	navigator -light           in the light theme, whatever the OS uses
//	navigator -quit-after 5s   quits on its own
package main

import (
	"flag"
	"log"
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// note is one note of the list.
type note struct {
	title, body string
	hue         geom.Color
}

var notes = []note{
	{"買い物", "米、味噌、ねぎ、豆腐。帰りに郵便局へ寄る。", geom.RGBA(0.30, 0.62, 0.42, 1)},
	{"京都旅行", "金曜の九時に出る。祇園の近くの宿を予約する。", geom.RGBA(0.86, 0.45, 0.28, 1)},
	{"読みたい本", "枕草子、こころ、雪国。図書館の返却は月曜まで。", geom.RGBA(0.36, 0.48, 0.82, 1)},
	{"アイデア", "画面を静かに暗くするタイマー。", geom.RGBA(0.64, 0.42, 0.74, 1)},
	{"会議メモ", "デザインのレビュー。Navigator と Hero を見せる。", geom.RGBA(0.80, 0.36, 0.48, 1)},
	{"レシピ", "肉じゃが: じゃがいも 3 個、玉ねぎ 1 個、牛肉 200g。", geom.RGBA(0.78, 0.60, 0.20, 1)},
	{"練習", "毎朝 20 分、ピアノの音階。", geom.RGBA(0.22, 0.58, 0.66, 1)},
	{"引っ越し", "電気とガスの手続き。段ボールを 20 箱。", geom.RGBA(0.50, 0.50, 0.56, 1)},
}

// icon is the square of a note with its first character.
func icon(ctx gowaku.BuildContext, n note, side float32) gowaku.Widget {
	style := basic.ThemeOf(ctx).Text.Headline
	style.Size = side * 0.45
	style.Color = geom.RGBA(1, 1, 1, 1)
	first := []rune(n.title)[:1]
	return basic.Container{
		Width: side, Height: side, Radius: side / 4, Color: n.hue,
		Child: basic.Center{Child: basic.Text{Content: string(first), Style: style}},
	}
}

// list is the first page: the notes, each a row that opens its page.
type list struct{ gowaku.StatelessBase }

func (list) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	navigator := nav.NavigatorOf(ctx)
	caption := th.Text.Caption
	caption.Color = th.Color.Ink2
	return basic.Container{Color: th.Color.Canvas, Child: basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Padding{Insets: geom.Insets{Left: theme.Space16, Top: theme.Space16, Bottom: theme.Space8}, Child: basic.Text{
			Content: "メモ", Style: th.Text.LargeTitle,
		}},
		basic.Expanded{Child: collection.List{ItemCount: len(notes), ItemExtent: 64, ItemBuilder: func(i int) gowaku.Widget {
			n := notes[i]
			return interact.Pressable{Label: n.title, OnPress: func() { navigator.PushNamed("/notes/" + strconv.Itoa(i)) }, Build: func(ps interact.PressState) gowaku.Widget {
				var ground geom.Color
				switch {
				case ps.Pressed:
					ground = th.Color.FillStrong
				case ps.Hovered:
					ground = th.Color.Fill
				}
				return interact.FocusRing{Visible: ps.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
					Color: ground, Padding: geom.Symmetric(theme.Space16, theme.Space8),
					Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space12, Children: []gowaku.Widget{
						nav.Hero{Tag: i, Child: icon(ctx, n, 40)},
						basic.Expanded{Child: basic.Column{Spacing: theme.Space2, Children: []gowaku.Widget{
							basic.Text{Content: n.title, Style: th.Text.Headline},
							basic.Text{Content: n.body, Style: caption},
						}}},
					}},
				}}
			}}
		}}},
	}}}
}

// detail is the page of one note.
type detail struct {
	gowaku.StatelessBase
	index int
}

func (d detail) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	if d.index < 0 || d.index >= len(notes) {
		return basic.Center{Child: basic.Column{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []gowaku.Widget{
			basic.Text{Content: "そのメモはありません"},
			nav.BackButton{Label: "メモ"},
		}}}
	}
	n := notes[d.index]
	body := th.Text.Reading
	body.Color = th.Color.Ink2
	return basic.Container{Color: th.Color.Card, Child: basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Padding{Insets: geom.All(theme.Space8), Child: basic.Row{Children: []gowaku.Widget{
			nav.BackButton{Label: "メモ"},
		}}},
		basic.Padding{Insets: geom.Symmetric(theme.Space24, theme.Space8), Child: basic.Row{
			Cross: layout.CrossCenter, Spacing: theme.Space16, Children: []gowaku.Widget{
				nav.Hero{Tag: d.index, Child: icon(ctx, n, 88)},
				basic.Text{Content: n.title, Style: th.Text.LargeTitle},
			},
		}},
		basic.Padding{Insets: geom.Symmetric(theme.Space24, theme.Space8), Child: basic.Text{Content: n.body, Style: body}},
	}}}
}

// home holds the Navigator, and names the window after the page on top.
type home struct {
	gowaku.StatefulBase
	open string
}

type homeState struct {
	gowaku.StateOf[home]
	handle *gowaku.WindowHandle
}

func (home) CreateState() gowaku.State { return &homeState{} }

func (s *homeState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	s.handle = gowaku.WindowOf(ctx)
	navigator := nav.Navigator{
		Home: list{},
		Routes: []nav.RoutePattern{{Pattern: "/notes/:id", Build: func(p nav.RouteParams) gowaku.Widget {
			i, ok := p.Int("id")
			if !ok {
				i = -1
			}
			return detail{index: i}
		}}},
		OnChange: func(name string) {
			title := "gowaku: Navigator"
			if name != "/" {
				title += " " + name
			}
			s.handle.SetTitle(title)
		},
	}
	if open := s.Widget().open; open != "" {
		navigator.Home, navigator.Initial = nil, open
		navigator.Routes = append(navigator.Routes, nav.RoutePattern{Pattern: "/", Build: func(nav.RouteParams) gowaku.Widget { return list{} }})
	}
	return navigator
}

func main() {
	light := flag.Bool("light", false, "use the light theme")
	open := flag.String("open", "", "the name of the first page, as /notes/3")
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the window to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}
	app := gowaku.App{Title: "gowaku: Navigator", Width: 520, Height: 560, Home: home{open: *open}}
	if *light {
		app.ThemeMode = gowaku.ThemeLight
	}
	if err := gowaku.Run(app); err != nil {
		log.Fatal(err)
	}
}
