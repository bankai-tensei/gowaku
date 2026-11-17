// Command menus is a small editor with a menu bar, shortcuts and a context
// menu. See docs/ADR/0046-menus-and-shortcuts.md.
//
//	menus                     opens the window, with its own title bar. Alt or F10 moves to the menu bar,
//	                          Alt+F opens ファイル, Ctrl+N, Ctrl+S and Ctrl+Z work
//	                          from anywhere, and a right click on the text opens
//	                          the context menu
//	menus -dark               the same in the dark theme
//	menus -png out.png        opens ファイル > 最近使ったファイル from the keyboard,
//	                          draws one frame and exits
//	menus -png out.png -context
//	                          opens the context menu instead
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/theme"
)

func ctrl(k input.KeyCode) interact.Shortcut {
	return interact.Shortcut{Key: k, Modifiers: input.ModControl}
}

type editor struct {
	core.StatefulBase
	// native keeps the title bar of the OS, with the menu bar under it.
	native bool
}

type editorState struct {
	core.StateBase
	text     string
	history  []string
	wrap     bool
	ruler    bool
	zoom     int
	status   string
	untitled int
}

func (editor) CreateState() core.State {
	return &editorState{
		text: "メニューは Alt か F10 で。\nAlt+F でファイルを開き、矢印と Enter で選ぶ。\n右クリックで文脈メニュー。",
		wrap: true, zoom: 100, status: "準備完了", untitled: 1,
	}
}

func (s *editorState) did(what string) {
	s.SetState(func() { s.status = what })
}

// editorActions are the commands of the editor, which the menus, the context
// menu and the shortcuts share.
type editorActions struct {
	newFile, save, undo, cut, copyText, paste, selectAll, wrap, ruler, zoomIn, zoomOut interact.Action
}

func (s *editorState) actions() editorActions {
	var newFile, save, undo, cut, copyText, paste, selectAll, wrap, ruler, zoomIn, zoomOut interact.Action
	newFile = interact.Action{Label: "新規(&N)", Shortcut: ctrl(input.KeyN), OnInvoke: func() {
		s.SetState(func() {
			s.history = append(s.history, s.text)
			s.text, s.untitled = "", s.untitled+1
			s.status = fmt.Sprintf("無題 %d を作った", s.untitled)
		})
	}}
	save = interact.Action{Label: "保存(&S)", Shortcut: ctrl(input.KeyS), OnInvoke: func() { s.did("保存した") }}
	undo = interact.Action{Label: "元に戻す(&U)", Shortcut: ctrl(input.KeyZ)}
	if len(s.history) > 0 {
		undo.OnInvoke = func() {
			s.SetState(func() {
				s.text = s.history[len(s.history)-1]
				s.history = s.history[:len(s.history)-1]
				s.status = "元に戻した"
			})
		}
	}
	cut = interact.Action{Label: "切り取り(&T)", Shortcut: ctrl(input.KeyX), OnInvoke: func() { s.did("切り取り") }}
	copyText = interact.Action{Label: "コピー(&C)", Shortcut: ctrl(input.KeyC), OnInvoke: func() { s.did("コピー") }}
	paste = interact.Action{Label: "貼り付け(&P)", Shortcut: ctrl(input.KeyV), OnInvoke: func() { s.did("貼り付け") }}
	selectAll = interact.Action{Label: "すべて選択(&A)", Shortcut: ctrl(input.KeyA), OnInvoke: func() { s.did("すべて選択") }}
	wrap = interact.Action{Label: "折り返す(&W)", Checkable: true, Checked: s.wrap, OnInvoke: func() {
		s.SetState(func() { s.wrap = !s.wrap })
	}}
	ruler = interact.Action{Label: "ルーラー(&R)", Checkable: true, Checked: s.ruler, OnInvoke: func() {
		s.SetState(func() { s.ruler = !s.ruler })
	}}
	zoomIn = interact.Action{Label: "拡大(&I)", Shortcut: ctrl(input.KeyEqual), OnInvoke: func() {
		s.SetState(func() { s.zoom = min(s.zoom+10, 200) })
	}}
	zoomOut = interact.Action{Label: "縮小(&O)", Shortcut: ctrl(input.KeyMinus), OnInvoke: func() {
		s.SetState(func() { s.zoom = max(s.zoom-10, 50) })
	}}
	return editorActions{newFile, save, undo, cut, copyText, paste, selectAll, wrap, ruler, zoomIn, zoomOut}
}

func (s *editorState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	a := s.actions()
	newFile, save, undo, cut, copyText, paste, selectAll := a.newFile, a.save, a.undo, a.cut, a.copyText, a.paste, a.selectAll
	wrap, ruler, zoomIn, zoomOut := a.wrap, a.ruler, a.zoomIn, a.zoomOut
	recent := make([]overlay.MenuItem, 0, 3)
	for _, name := range []string{"議事録 11 月.txt", "買い物.txt", "gowaku メモ.md"} {
		recent = append(recent, overlay.MenuItem{Action: interact.Action{Label: name, OnInvoke: func() { s.did(name + " を開いた") }}})
	}
	quit := interact.Action{Label: "終了(&X)", OnInvoke: gowaku.Quit}
	bar := overlay.MenuBar{Items: []overlay.MenuItem{
		{Action: interact.Action{Label: "ファイル(&F)"}, Items: []overlay.MenuItem{
			{Action: newFile},
			{Action: interact.Action{Label: "開く(&O)…", Shortcut: ctrl(input.KeyO), OnInvoke: func() { s.did("開く") }}},
			{Action: interact.Action{Label: "最近使ったファイル(&R)"}, Items: recent},
			{Separator: true},
			{Action: save},
			{Action: interact.Action{Label: "名前を付けて保存(&A)…", Shortcut: interact.Shortcut{Key: input.KeyS, Modifiers: input.ModControl | input.ModShift}}},
			{Separator: true},
			{Action: quit},
		}},
		{Action: interact.Action{Label: "編集(&E)"}, Items: []overlay.MenuItem{
			{Action: undo},
			{Separator: true},
			{Action: cut}, {Action: copyText}, {Action: paste},
			{Separator: true},
			{Action: selectAll},
		}},
		{Action: interact.Action{Label: "表示(&V)"}, Items: []overlay.MenuItem{
			{Action: wrap}, {Action: ruler},
			{Separator: true},
			{Action: interact.Action{Label: "ズーム(&Z)"}, Items: []overlay.MenuItem{{Action: zoomIn}, {Action: zoomOut}}},
		}},
		{Action: interact.Action{Label: "ツール(&T)"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "設定(&P)…", Shortcut: ctrl(input.KeyComma), OnInvoke: s.openSettings}},
		}},
		{Action: interact.Action{Label: "ヘルプ(&H)"}, Items: []overlay.MenuItem{
			{Action: interact.Action{Label: "gowaku について(&A)", OnInvoke: func() { s.did("gowaku 0.14") }}},
		}},
	}}
	context := []overlay.MenuItem{
		{Action: cut}, {Action: copyText}, {Action: paste},
		{Separator: true},
		{Action: selectAll},
		{Separator: true},
		{Action: wrap},
	}
	editorField := overlay.ContextMenu{Items: context, Child: field.Input{
		Value: s.text, Multiline: true,
		OnChange: func(v string) { s.SetState(func() { s.text = v }) },
	}}
	statusText := fmt.Sprintf("%s　　ズーム %d%%　　折り返し %s", s.status, s.zoom, onOff(s.wrap))
	return interact.Actions{
		Actions: []interact.Action{undo},
		Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
			s.top(th, bar),
			basic.Expanded{Child: basic.Padding{Insets: geom.All(theme.Space16), Child: editorField}},
			basic.Container{
				Height: 28, Padding: geom.Symmetric(theme.Space12, 0), Color: th.Color.Sidebar,
				Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{
					Content: statusText, Style: th.Text.Caption.WithColor(th.Color.Ink2),
				}},
			},
		}},
	}
}

// openSettings shows the settings in a modal window, which keeps the editor
// from the user until it closes.
func (s *editorState) openSettings() {
	var h *gowaku.WindowHandle
	choice := s.wrap
	ok := false
	opts := gowaku.WindowOptions{Title: "設定", Width: 360, Height: 180, OnClose: func() {
		if ok {
			s.SetState(func() { s.wrap = choice; s.status = "設定を変えた" })
		}
	}}
	var err error
	h, err = gowaku.ShowModal(s.Context(), opts, settings{
		wrap:     choice,
		onChange: func(v bool) { choice = v },
		onDone: func(accept bool) {
			ok = accept
			h.Close()
		},
	})
	if err != nil {
		s.did("設定を開けない: " + err.Error())
	}
}

// settings is the content of the modal window.
type settings struct {
	core.StatefulBase
	wrap     bool
	onChange func(bool)
	onDone   func(accept bool)
}

type settingsState struct {
	core.StateBase
	wrap bool
}

func (w settings) CreateState() core.State { return &settingsState{wrap: w.wrap} }

func (s *settingsState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(settings)
	th := basic.ThemeOf(ctx)
	return interact.Actions{
		Actions: []interact.Action{{Shortcut: interact.Shortcut{Key: input.KeyEscape}, OnInvoke: func() { w.onDone(false) }}},
		Child: basic.Padding{Insets: geom.All(theme.Space24), Child: basic.Column{
			Cross: layout.CrossStretch, Spacing: theme.Space16, Children: []core.Widget{
				basic.Text{Content: "表示", Style: th.Text.Headline},
				basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
					basic.Expanded{Child: basic.Text{Content: "長い行を折り返す"}},
					control.Switch{Value: s.wrap, Label: "長い行を折り返す", OnChange: func(v bool) {
						s.SetState(func() { s.wrap = v })
						w.onChange(v)
					}},
				}},
				basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: []core.Widget{
					control.Button{Label: "キャンセル", OnPress: func() { w.onDone(false) }},
					control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: func() { w.onDone(true) }},
				}},
			},
		}},
	}
}

// top is the menu bar: in a title bar of gowaku, or under the one of the OS.
func (s *editorState) top(th *theme.Theme, bar overlay.MenuBar) core.Widget {
	if s.Widget().(editor).native {
		return basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
			basic.Container{Color: th.Color.Sidebar, Child: bar},
			basic.Container{Height: 1, Color: th.Color.Hair},
		}}
	}
	return gowaku.TitleBar{Child: basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
		bar,
		basic.Expanded{Child: basic.Center{Child: basic.Text{
			Content: fmt.Sprintf("無題 %d — menus", s.untitled), Style: th.Text.Control.WithColor(th.Color.Ink2),
		}}},
	}}}
}

func onOff(v bool) string {
	if v {
		return "オン"
	}
	return "オフ"
}

// screenshot opens a menu from the keyboard and writes one frame.
func screenshot(opts shot.Options, path string, context bool) error {
	// The harness has no window frame of gowaku, so the menu bar goes under
	// the title bar of the OS.
	h, err := shot.Open(editor{native: true}, opts)
	if err != nil {
		return err
	}
	defer h.Close()
	settle := func() error {
		for start := time.Now(); time.Since(start) < 300*time.Millisecond; {
			if _, err := h.Frame(); err != nil {
				return err
			}
			h.Pump()
		}
		return nil
	}
	if err := settle(); err != nil {
		return err
	}
	key := func(k input.KeyCode, m input.Modifiers) error {
		h.Tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
		return settle()
	}
	if context {
		// Tab to the field, then the menu key.
		if err := key(input.KeyTab, 0); err != nil {
			return err
		}
		if err := key(input.KeyContextMenu, 0); err != nil {
			return err
		}
		if err := key(input.KeyDown, 0); err != nil {
			return err
		}
	} else {
		for _, k := range []struct {
			k input.KeyCode
			m input.Modifiers
		}{{input.KeyF, input.ModAlt}, {input.KeyDown, 0}, {input.KeyDown, 0}, {input.KeyRight, 0}, {input.KeyDown, 0}} {
			if err := key(k.k, k.m); err != nil {
				return err
			}
		}
	}
	return h.Screenshot(path)
}

func main() {
	pngPath := flag.String("png", "", "open a menu, write one frame to this file and exit")
	context := flag.Bool("context", false, "with -png, open the context menu")
	dark := flag.Bool("dark", false, "use the dark theme")
	native := flag.Bool("native", false, "keep the title bar of the OS")
	vk := flag.Bool("vulkan", false, "use Vulkan instead of D3D11 (Windows)")
	flag.Parse()

	brightness := theme.BrightnessLight
	mode := gowaku.ThemeLight
	if *dark {
		brightness, mode = theme.BrightnessDark, gowaku.ThemeDark
	}
	var err error
	if *pngPath != "" {
		err = screenshot(shot.Options{Width: 720, Height: 460, Theme: theme.Set{}.For(brightness), Vulkan: *vk}, *pngPath, *context)
	} else {
		err = gowaku.Run(gowaku.App{
			Title: "gowaku: menus", Width: 860, Height: 500, ThemeMode: mode,
			CustomFrame: !*native, Home: editor{native: *native},
		})
	}
	if err != nil {
		log.Fatal(err)
	}
}
