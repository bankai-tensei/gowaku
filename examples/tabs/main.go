// Command tabs is a browser in miniature: documents in tabs that close, move,
// and tear off into windows of their own. Drag a tab down off the strip and let
// go to open it in a new window under the pointer; drag it onto the strip of
// another window to move it there. A window whose last tab leaves closes.
//
//	tabs                  opens one window with a few documents
//	tabs -light           in the light theme, whatever the OS uses
//	tabs -quit-after 5s   quits on its own
package main

import (
	"flag"
	"fmt"
	"log"
	"slices"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// doc is one document. Its ID is the key of its tab in every window.
type doc struct {
	id    int
	title string
	body  string
}

// browser holds every document and joins the tab bars of all windows.
type browser struct {
	group nav.TabGroup
	docs  map[int]*doc
	next  int
}

// add makes a new document.
func (b *browser) add(title, body string) int {
	b.next++
	b.docs[b.next] = &doc{id: b.next, title: title, body: body}
	return b.next
}

// window is the root of one window: a tab bar over the selected document.
type window struct {
	gowaku.StatefulBase
	app  *browser
	tabs []int
}

type windowState struct {
	gowaku.StateOf[window]
	tabs     []int
	selected int
	handle   *gowaku.WindowHandle
	// title is what the window was last called.
	title string
}

func (window) CreateState() gowaku.State { return &windowState{} }

// InitState implements gowaku.State.
func (s *windowState) InitState() { s.tabs = slices.Clone(s.Widget().tabs) }

// remove takes tab i out, and closes the window when none is left.
func (s *windowState) remove(i int) {
	s.SetState(func() {
		s.tabs = slices.Delete(s.tabs, i, i+1)
		if s.selected > i || s.selected >= len(s.tabs) {
			s.selected = max(s.selected-1, 0)
		}
	})
	if len(s.tabs) == 0 {
		s.handle.Close()
	}
}

// tearOff gives a tab dropped away from every bar a window of its own.
func (s *windowState) tearOff(e nav.TabTearOff) {
	b := s.Widget().app
	if e.Alone {
		// The only tab takes its window along, as in a browser.
		s.handle.MoveTo(e)
		return
	}
	s.remove(e.Index)
	opts := gowaku.WindowOptions{Title: b.docs[e.Key.(int)].title, Width: 640, Height: 420}
	if _, err := gowaku.TearOffWindow(e, opts, window{app: b, tabs: []int{e.Key.(int)}}); err != nil {
		log.Print(err)
	}
}

func (s *windowState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	b := s.Widget().app
	th := basic.ThemeOf(ctx)
	s.handle = gowaku.WindowOf(ctx)
	items := make([]nav.TabItem, len(s.tabs))
	for i, id := range s.tabs {
		items[i] = nav.TabItem{Key: id, Label: b.docs[id].title}
	}
	var page gowaku.Widget = basic.SizedBox{}
	if s.selected < len(s.tabs) {
		d := b.docs[s.tabs[s.selected]]
		if title := d.title + " - gowaku"; title != s.title {
			s.title = title
			h := s.handle
			gowaku.Post(func() { h.SetTitle(title) })
		}
		page = basic.Pad(theme.Space28, basic.Column{Spacing: theme.Space12, Cross: layout.CrossStart, Children: []gowaku.Widget{
			basic.Text{Content: d.title, Style: th.Text.Title},
			basic.Text{Content: d.body, Style: th.Text.Reading},
		}})
	}
	bar := nav.TabBar{
		Tabs: items, Selected: s.selected, Label: "文書", Group: &b.group,
		OnSelect: func(i int) { s.SetState(func() { s.selected = i }) },
		OnClose:  s.remove,
		OnMove: func(from, to int) {
			s.SetState(func() {
				id := s.tabs[from]
				s.tabs = slices.Insert(slices.Delete(s.tabs, from, from+1), to, id)
			})
		},
		OnNew: func() {
			id := b.add(fmt.Sprintf("無題 %d", b.next+1), "新しい文書です。")
			s.SetState(func() {
				s.tabs = append(s.tabs, id)
				s.selected = len(s.tabs) - 1
			})
		},
		OnReceive: func(k gowaku.Key, i int) {
			s.SetState(func() { s.tabs = slices.Insert(s.tabs, i, k.(int)) })
		},
		OnDetach:  s.remove,
		OnTearOff: s.tearOff,
	}
	return basic.Column{Cross: layout.CrossStretch, Children: []gowaku.Widget{
		basic.Padding{Insets: geom.Insets{Left: theme.Space8, Top: theme.Space4, Right: theme.Space8}, Child: bar},
		basic.Expanded{Child: basic.Container{Color: th.Color.Card, Child: page}},
	}}
}

func main() {
	light := flag.Bool("light", false, "use the light theme")
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the windows to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}
	b := &browser{docs: make(map[int]*doc)}
	first := []int{
		b.add("吾輩は猫である", "吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。"),
		b.add("坊っちゃん", "親譲りの無鉄砲で小供の時から損ばかりしている。"),
		b.add("草枕", "山路を登りながら、こう考えた。智に働けば角が立つ。情に棹させば流される。"),
		b.add("こころ", "私はその人を常に先生と呼んでいた。"),
		b.add("三四郎", "うとうととして眼が覚めると女はいつのまにか、隣のじいさんと話を始めている。"),
	}
	// The labels are Japanese, so the words of the library are too, whatever
	// the language of the OS.
	app := gowaku.App{Title: "gowaku: タブ", Width: 760, Height: 460, Locale: language.Japanese, Home: window{app: b, tabs: first}}
	if *light {
		app.ThemeMode = gowaku.ThemeLight
	}
	if err := gowaku.Run(app); err != nil {
		log.Fatal(err)
	}
}
