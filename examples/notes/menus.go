package main

import (
	"path/filepath"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/input"
)

// shortcut is a key with modifiers.
func shortcut(k input.KeyCode, m input.Modifiers) interact.Shortcut {
	return interact.Shortcut{Key: k, Modifiers: m}
}

const (
	ctrl      = input.ModControl
	ctrlShift = input.ModControl | input.ModShift
)

// windowActions are the commands of a window, which the menus, the shortcuts
// and the command palette share.
type windowActions struct {
	newDoc, newWindow, open, save, saveAs, closeTab, closeWindow, quit         interact.Action
	undo, redo, cut, copyText, paste, selectAll, find, replace, next, previous interact.Action
	vertical, bigger, smaller, normal, mincho, gothic, notebook, palette       interact.Action
	settings                                                                   interact.Action
	// paletteK is Ctrl+K, the second way to the palette.
	paletteK interact.Action
	recent   []overlay.MenuItem
}

// actions makes the commands of the window for its current state.
func (s *windowState) actions() windowActions {
	w := s.Widget().model
	a := w.app
	d := w.current()
	onDoc := d != nil && s.onEditor
	var acts windowActions
	acts.newDoc = interact.Action{Label: "新規(&N)", Shortcut: shortcut(input.KeyN, ctrl), OnInvoke: s.newDoc}
	acts.newWindow = interact.Action{Label: "新しいウィンドウ(&W)", Shortcut: shortcut(input.KeyN, ctrlShift), OnInvoke: s.newWindow}
	acts.open = interact.Action{Label: "開く(&O)…", Shortcut: shortcut(input.KeyO, ctrl), OnInvoke: s.openDialog}
	acts.save = interact.Action{Label: "保存(&S)", Shortcut: shortcut(input.KeyS, ctrl)}
	acts.saveAs = interact.Action{Label: "名前を付けて保存(&A)…", Shortcut: shortcut(input.KeyS, ctrlShift)}
	acts.closeTab = interact.Action{Label: "タブを閉じる(&C)", Shortcut: shortcut(input.KeyW, ctrl)}
	acts.closeWindow = interact.Action{Label: "ウィンドウを閉じる(&D)", Shortcut: shortcut(input.KeyW, ctrlShift), OnInvoke: s.closeWindow}
	acts.quit = interact.Action{Label: "終了(&X)", OnInvoke: a.quit}

	acts.undo = interact.Action{Label: "元に戻す(&U)", Shortcut: shortcut(input.KeyZ, ctrl)}
	acts.redo = interact.Action{Label: "やり直し(&R)", Shortcut: shortcut(input.KeyY, ctrl)}
	acts.cut = interact.Action{Label: "切り取り(&T)", Shortcut: shortcut(input.KeyX, ctrl)}
	acts.copyText = interact.Action{Label: "コピー(&C)", Shortcut: shortcut(input.KeyC, ctrl)}
	acts.paste = interact.Action{Label: "貼り付け(&P)", Shortcut: shortcut(input.KeyV, ctrl)}
	acts.selectAll = interact.Action{Label: "すべて選択(&A)", Shortcut: shortcut(input.KeyA, ctrl)}
	acts.find = interact.Action{Label: "検索(&F)…", Shortcut: shortcut(input.KeyF, ctrl)}
	acts.replace = interact.Action{Label: "置換(&E)…", Shortcut: shortcut(input.KeyH, ctrl)}
	acts.next = interact.Action{Label: "次を検索(&N)", Shortcut: shortcut(input.KeyF3, 0)}
	acts.previous = interact.Action{Label: "前を検索(&V)", Shortcut: shortcut(input.KeyF3, input.ModShift)}

	acts.vertical = interact.Action{Label: "縦書き(&T)", Shortcut: shortcut(input.KeyT, ctrlShift), Checkable: true, Checked: d != nil && d.vertical}
	acts.bigger = interact.Action{Label: "文字を大きく(&I)", Shortcut: shortcut(input.KeyEqual, ctrl), OnInvoke: func() { a.zoom(1) }}
	acts.smaller = interact.Action{Label: "文字を小さく(&O)", Shortcut: shortcut(input.KeyMinus, ctrl), OnInvoke: func() { a.zoom(-1) }}
	acts.normal = interact.Action{Label: "標準の大きさ(&Z)", Shortcut: shortcut(input.Key0, ctrl), OnInvoke: func() { a.zoom(0) }}
	acts.mincho = interact.Action{Label: "明朝(&M)", Checkable: true, Checked: a.settings.Family == store.Mincho, OnInvoke: func() { s.setFamily(store.Mincho) }}
	acts.gothic = interact.Action{Label: "ゴシック(&G)", Checkable: true, Checked: a.settings.Family == store.Gothic, OnInvoke: func() { s.setFamily(store.Gothic) }}
	acts.notebook = interact.Action{Label: "ノート帳(&L)", Shortcut: shortcut(input.KeyL, ctrlShift), OnInvoke: s.toggleNotebook}
	acts.palette = interact.Action{Label: "コマンドパレット(&P)…", Shortcut: shortcut(input.KeyP, ctrlShift), OnInvoke: s.openPalette}
	acts.paletteK = interact.Action{Label: "コマンドパレット", Shortcut: shortcut(input.KeyK, ctrl), OnInvoke: s.openPalette}
	acts.settings = interact.Action{Label: "設定(&S)…", Shortcut: shortcut(input.KeyComma, ctrl), OnInvoke: s.openSettings}

	if onDoc {
		acts.save.OnInvoke = func() { s.save(d) }
		acts.saveAs.OnInvoke = func() { s.saveAs(d) }
		acts.closeTab.OnInvoke = func() { s.closeTab(w.selected) }
		acts.vertical.OnInvoke = func() { a.setVertical(d, !d.vertical) }
		acts.undo.OnInvoke = s.editText((*field.InputController).Undo)
		acts.redo.OnInvoke = s.editText((*field.InputController).Redo)
		acts.cut.OnInvoke = s.editText((*field.InputController).Cut)
		acts.copyText.OnInvoke = s.editText((*field.InputController).Copy)
		acts.paste.OnInvoke = s.editText((*field.InputController).Paste)
		acts.selectAll.OnInvoke = s.editText((*field.InputController).SelectAll)
		acts.find.OnInvoke = func() { s.withEditor(func(e *editorState) { e.openFind(false) }) }
		acts.replace.OnInvoke = func() { s.withEditor(func(e *editorState) { e.openFind(true) }) }
		acts.next.OnInvoke = func() { s.withEditor(func(e *editorState) { e.findStep(1) }) }
		acts.previous.OnInvoke = func() { s.withEditor(func(e *editorState) { e.findStep(-1) }) }
	}
	for _, path := range a.recent() {
		acts.recent = append(acts.recent, overlay.MenuItem{Action: interact.Action{
			Label: strings.ReplaceAll(filepath.Base(path), "&", "&&"), OnInvoke: func() { s.openPath(path) },
		}})
	}
	if len(acts.recent) == 0 {
		acts.recent = []overlay.MenuItem{{Action: interact.Action{Label: "（なし）"}}}
	}
	return acts
}

// extra are the shortcuts that no menu shows.
func (acts windowActions) extra() []interact.Action {
	return []interact.Action{acts.paletteK}
}

// commands are the actions the palette lists, in the order of the menus.
func (acts windowActions) commands() []interact.Action {
	return []interact.Action{
		acts.newDoc, acts.newWindow, acts.open, acts.save, acts.saveAs, acts.closeTab, acts.closeWindow,
		acts.undo, acts.redo, acts.cut, acts.copyText, acts.paste, acts.selectAll,
		acts.find, acts.replace, acts.next, acts.previous,
		acts.vertical, acts.bigger, acts.smaller, acts.normal, acts.mincho, acts.gothic, acts.notebook,
		acts.settings, acts.quit,
	}
}

// menuBar is the menu bar of the window.
func (s *windowState) menuBar(acts windowActions) core.Widget {
	item := func(a interact.Action) overlay.MenuItem { return overlay.MenuItem{Action: a} }
	sep := overlay.MenuItem{Separator: true}
	return overlay.MenuBar{Items: []overlay.MenuItem{
		{Action: interact.Action{Label: "ファイル(&F)"}, Items: []overlay.MenuItem{
			item(acts.newDoc), item(acts.newWindow), item(acts.open),
			{Action: interact.Action{Label: "最近のファイル(&R)"}, Items: acts.recent},
			sep, item(acts.save), item(acts.saveAs),
			sep, item(acts.closeTab), item(acts.closeWindow),
			sep, item(acts.quit),
		}},
		{Action: interact.Action{Label: "編集(&E)"}, Items: []overlay.MenuItem{
			item(acts.undo), item(acts.redo),
			sep, item(acts.cut), item(acts.copyText), item(acts.paste),
			sep, item(acts.selectAll),
			sep, item(acts.find), item(acts.replace), item(acts.next), item(acts.previous),
		}},
		{Action: interact.Action{Label: "表示(&V)"}, Items: []overlay.MenuItem{
			item(acts.vertical),
			sep, item(acts.bigger), item(acts.smaller), item(acts.normal),
			sep, item(acts.mincho), item(acts.gothic),
			sep, item(acts.notebook), item(acts.palette),
		}},
		{Action: interact.Action{Label: "ツール(&T)"}, Items: []overlay.MenuItem{
			item(acts.settings),
		}},
	}}
}
