package main

import (
	"slices"
	"strconv"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

// The page of GridView and ReorderableList: a GridView of many
// thumbnails, and a ReorderableList of things to do. It fills the window, as
// the desktop page does.

// gridPhotos is the number of thumbnails of the grid.
const gridPhotos = 10000

// pageGrid returns the page of GridView and ReorderableList.
func pageGrid() core.Widget { return gridPage{} }

type gridPage struct{ core.StatefulBase }

// todo is one thing to do.
type todo struct {
	title string
	done  bool
}

type gridPageState struct {
	core.StateBase
	photos collection.TableController
	opened int
	todos  []todo
	// colors holds the color of each thumbnail, made once.
	colors []geom.Color
	names  []string
}

// CreateState implements core.StatefulWidget.
func (gridPage) CreateState() core.State { return &gridPageState{} }

// InitState implements core.State.
func (s *gridPageState) InitState() {
	s.opened = -1
	s.colors = make([]geom.Color, gridPhotos)
	s.names = make([]string, gridPhotos)
	for i := range gridPhotos {
		p := theme.TonalPalette{Hue: float64(i*47%360) + 0.5, Chroma: 36 + float64(i%3)*8}
		s.colors[i] = p.Tone(62 + float64(i%4)*6)
		s.names[i] = "写真 " + strconv.Itoa(i+1)
	}
	s.todos = []todo{
		{title: "牛乳を買う"}, {title: "請求書を払う", done: true}, {title: "歯医者を予約する"},
		{title: "部屋の掃除"}, {title: "本を返す"}, {title: "旅行の宿を取る"},
		{title: "植木に水をやる"}, {title: "メールに返信する"},
	}
}

// Build implements core.State.
func (s *gridPageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	small := th.Text.Caption.WithColor(th.Color.Ink2)

	status := strconv.Itoa(s.photos.SelectedCount()) + " 枚を選択中"
	if s.opened >= 0 {
		status += "・最後に開いた写真: " + s.names[s.opened]
	}
	grid := collection.GridView{
		ItemCount: gridPhotos, MinColumnWidth: 112, AspectRatio: 4.0 / 3, ExtraHeight: 24,
		Spacing: theme.Space8, RunSpacing: theme.Space8, Padding: geom.All(theme.Space8),
		Label: "写真", Controller: &s.photos,
		LabelOf:           func(i int) string { return s.names[i] },
		OnSelectionChange: func() { s.SetState(func() {}) },
		OnActivate:        func(i int) { s.SetState(func() { s.opened = i }) },
		ItemBuilder: func(i int) core.Widget {
			return basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space4, Children: []core.Widget{
				basic.Expanded{Child: basic.Container{Color: s.colors[i], Radius: th.Radius.Control}},
				basic.Text{Content: s.names[i], Style: small},
			}}
		},
	}

	todos := collection.ReorderableList{
		ItemCount: len(s.todos), ItemExtent: 44, Label: "やること",
		LabelOf: func(i int) string { return s.todos[i].title },
		OnReorder: func(from, to int) {
			s.SetState(func() {
				t := s.todos[from]
				s.todos = slices.Insert(slices.Delete(s.todos, from, from+1), to, t)
			})
		},
		ItemBuilder: func(i int) core.Widget {
			t := s.todos[i]
			return basic.Padding{Insets: geom.Symmetric(theme.Space12, 0), Child: basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
				control.Checkbox{Value: t.done, Label: t.title, OnChange: func(v bool) {
					s.SetState(func() { s.todos[i].done = v })
				}},
				basic.Expanded{Child: basic.SizedBox{}},
				basic.Text{Content: "≡", Style: small},
			}}}
		},
	}

	head := th.Text.Headline.WithColor(th.Color.Ink)
	panel := func(title, hint, status string, child core.Widget) core.Widget {
		return basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space8, Children: []core.Widget{
			basic.Text{Content: title, Style: head},
			basic.Text{Content: hint, Style: small},
			basic.Text{Content: status, Style: small},
			basic.Expanded{Child: basic.Container{Color: th.Color.Card, Radius: th.Radius.Group, Child: child}},
		}}
	}
	done := 0
	for _, t := range s.todos {
		if t.done {
			done++
		}
	}
	return basic.Pad(theme.Space28, basic.Column{Cross: layout.CrossStretch, Spacing: theme.Space24, Children: []core.Widget{
		heading(th, "グリッドと並べ替え", "見えている行だけを作るので、1 万枚でも軽く動きます。"),
		basic.Expanded{Child: basic.Row{Cross: layout.CrossStretch, Spacing: theme.Space24, Children: []core.Widget{
			basic.Expanded{Child: panel("サムネイル",
				"クリックで選択、Ctrl と Shift で複数。矢印キーで移動し、Enter かダブルクリックで開きます。", status, grid)},
			basic.SizedBox{Width: 300, Child: panel("やること",
				"ドラッグか Ctrl+Shift+↑↓ で並べ替えます。", strconv.Itoa(done)+" / "+strconv.Itoa(len(s.todos))+" 件済み", todos)},
		}}},
	}})
}
