// Command dragdrop takes files and text dropped from other programs, drags
// text and a file out to them, and shows a tray icon and notifications. See
// docs/ADR/0050-drag-and-drop.md and docs/ADR/0051-tray-and-notifications.md.
//
//	dragdrop          opens the window: drop files from Explorer or Nautilus on
//	                  the left, drag the cards on the right to a text editor,
//	                  a browser, a folder or the desktop
//	dragdrop -dark    the same in the dark theme
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

type page struct {
	core.StatefulBase
	// file is what the second card drags out.
	file string
}

type pageState struct {
	core.StateBase
	over    bool
	dropped []string
	status  string
	tray    *gowaku.Tray
}

func (page) CreateState() core.State {
	return &pageState{status: "ファイルかテキストを左に落とす"}
}

// InitState shows the tray icon, once the window is up.
func (s *pageState) InitState() {
	gowaku.Post(func() {
		tray, err := gowaku.ShowTray(s.Context(), gowaku.TrayOptions{
			Icon:    trayIcon(),
			Tooltip: "gowaku: dragdrop",
			Menu: []overlay.MenuItem{
				{Action: interact.Action{Label: "通知を出す(&N)", OnInvoke: s.notify}},
				{Separator: true},
				{Action: interact.Action{Label: "終了(&X)", OnInvoke: gowaku.Quit}},
			},
			OnActivate:          func() { s.say("トレイのアイコンがクリックされた") },
			OnNotificationClick: func() { s.say("通知がクリックされた") },
		})
		if err != nil {
			s.say("トレイが無い: " + err.Error())
			return
		}
		s.tray = tray
	})
}

func (s *pageState) say(what string) { s.SetState(func() { s.status = what }) }

func (s *pageState) notify() {
	if err := gowaku.Notify(s.Context(), "gowaku", fmt.Sprintf("%d 件を受け取っている", len(s.dropped))); err != nil {
		s.say("通知を出せない: " + err.Error())
	}
}

// trayIcon is a blue rounded square with a white bar, 32 pixels wide.
func trayIcon() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			dx, dy := min(x, 31-x), min(y, 31-y)
			if dx+dy < 6 && dx < 4 && dy < 4 && (4-dx)*(4-dx)+(4-dy)*(4-dy) > 16 {
				continue
			}
			c := color.NRGBA{0x2f, 0x5f, 0xd0, 255}
			if y >= 14 && y < 18 && x >= 8 && x < 24 {
				c = color.NRGBA{255, 255, 255, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func (s *pageState) drop(d input.DragData, pos geom.Offset) {
	s.SetState(func() {
		s.dropped = s.dropped[:0]
		for _, f := range d.Files {
			s.dropped = append(s.dropped, "📄 "+f)
		}
		if d.Text != "" {
			s.dropped = append(s.dropped, "✎ "+strings.TrimSpace(d.Text))
		}
		s.status = fmt.Sprintf("%d 件を (%.0f, %.0f) に受け取った", len(s.dropped), pos.X, pos.Y)
	})
}

func effectName(e input.DropEffect) string {
	switch e {
	case input.DropCopy:
		return "コピー"
	case input.DropMove:
		return "移動"
	case input.DropLink:
		return "リンク"
	}
	return "取り消し"
}

func (s *pageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	file := s.Widget().(page).file
	border := th.Color.Hair
	ground := th.Color.Card
	if s.over {
		border, ground = th.Color.AccentInk, th.Color.AccentTint
	}
	lines := make([]core.Widget, 0, 1+len(s.dropped))
	lines = append(lines, basic.Text{Content: "ここにドロップ", Style: th.Text.Headline})
	for _, d := range s.dropped {
		lines = append(lines, basic.Text{Content: d, Style: th.Text.Body.WithColor(th.Color.Ink2)})
	}
	zone := interact.DragTarget{
		OnHover: func(over bool) { s.SetState(func() { s.over = over }) },
		OnDrop:  s.drop,
		Child: basic.Container{
			Color: ground, Radius: th.Radius.Group, Border: paint.Stroke{Width: 1.5, Color: border},
			Padding: geom.All(theme.Space16),
			Child:   basic.Column{Spacing: theme.Space8, Children: lines},
		},
	}
	card := func(title, detail string, data input.DragData) core.Widget {
		return interact.Draggable{
			Data: data, Allowed: input.DropCopy,
			OnDragStart: func() { s.SetState(func() { s.status = title + " をドラッグ中" }) },
			OnDragEnd: func(e input.DropEffect) {
				s.SetState(func() { s.status = title + ": " + effectName(e) })
			},
			Child: basic.Container{
				Color: th.Color.Raised, Radius: th.Radius.Control, Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
				Padding: geom.All(theme.Space12),
				Child: basic.Column{Spacing: theme.Space4, Children: []core.Widget{
					basic.Text{Content: title, Style: th.Text.Control},
					basic.Text{Content: detail, Style: th.Text.Caption.WithColor(th.Color.Ink2)},
				}},
			},
		}
	}
	cards := basic.Column{Spacing: theme.Space12, Children: []core.Widget{
		card("テキスト", "エディタやブラウザの入力欄へ", input.DragData{Text: "gowaku からドラッグした日本語のテキスト"}),
		card("ファイル", filepath.Base(file)+" をフォルダやデスクトップへ", input.DragData{Files: []string{file}}),
	}}
	return basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
		Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
			basic.Expanded{Child: basic.Row{Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
				basic.Expanded{Child: zone},
				basic.SizedBox{Width: 260, Child: cards},
			}}},
			basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space12, Children: []core.Widget{
				basic.Expanded{Child: basic.Text{Content: s.status, Style: th.Text.Caption.WithColor(th.Color.Ink2)}},
				control.Button{Label: "通知を出す", Size: control.ButtonSmall, OnPress: s.notify},
			}},
		},
	}}
}

func main() {
	dark := flag.Bool("dark", false, "use the dark theme")
	flag.Parse()
	file := filepath.Join(os.TempDir(), "gowaku-drag.txt")
	if err := os.WriteFile(file, []byte("gowaku からドラッグしたファイル\n"), 0o600); err != nil {
		log.Fatal(err)
	}
	mode := gowaku.ThemeLight
	if *dark {
		mode = gowaku.ThemeDark
	}
	err := gowaku.Run(gowaku.App{Title: "gowaku: dragdrop", Width: 720, Height: 420, ThemeMode: mode, Home: page{file: file}})
	if err != nil {
		log.Fatal(err)
	}
}
