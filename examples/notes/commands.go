package main

import (
	"fmt"
	"path/filepath"

	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
)

// textFilters returns the choices of file type in the file dialogs.
func textFilters() []gowaku.FileFilter {
	return []gowaku.FileFilter{{Name: "テキスト", Patterns: []string{"*.txt"}}, {Name: "すべてのファイル", Patterns: []string{"*"}}}
}

// answer is what the user said to closing a text with unsaved changes.
type answer uint8

const (
	answerCancel answer = iota
	answerSave
	answerDiscard
)

// newDoc opens an empty untitled text in a new tab.
func (s *windowState) newDoc() {
	w := s.Widget().model
	d := w.app.newUntitled()
	w.hero = heroKey{}
	w.addTab(d.id)
	w.app.saveSession()
}

// newWindow opens another window with the notebook.
func (s *windowState) newWindow() {
	a := s.Widget().model.app
	a.openWindow(a.newWindow())
	a.saveSession()
}

// openDialog asks for files to open, each in a tab.
func (s *windowState) openDialog() {
	w := s.Widget().model
	d := gowaku.FileDialog{Title: "開く", Multiple: true, Folder: w.app.settings.Library, Filters: textFilters()}
	w.app.shell.fileDialog(s.Context(), d, func(paths []string, err error) {
		if err != nil {
			w.toast("ファイルのダイアログを開けません: " + errText(err))
			return
		}
		for _, p := range paths {
			s.openPath(p)
		}
	})
}

// openPath opens the file at path in a tab.
func (s *windowState) openPath(path string) {
	w := s.Widget().model
	w.hero = heroKey{}
	if _, err := w.app.open(w, path); err != nil {
		w.toast(filepath.Base(path) + " を開けません: " + errText(err))
	}
}

// save saves d, saying so or why not.
func (s *windowState) save(d *document) bool {
	w := s.Widget().model
	if err := w.app.save(d); err != nil {
		w.toast("保存できません: " + errText(err))
		return false
	}
	w.toast(d.title() + " を保存しました")
	return true
}

// saveAs asks where to save d, then saves it there.
func (s *windowState) saveAs(d *document) {
	w := s.Widget().model
	a := w.app
	folder := a.settings.Library
	if d.path != "" {
		folder = filepath.Dir(d.path)
	}
	dialog := gowaku.FileDialog{Title: "名前を付けて保存", Save: true, Name: d.title() + ".txt", Folder: folder, Filters: textFilters()}
	a.shell.fileDialog(s.Context(), dialog, func(paths []string, err error) {
		if err != nil {
			w.toast("ファイルのダイアログを開けません: " + errText(err))
			return
		}
		if len(paths) == 0 {
			return
		}
		path := paths[0]
		if filepath.Ext(path) == "" {
			path += ".txt"
		}
		if err := a.saveTo(d, path); err != nil {
			w.toast("保存できません: " + errText(err))
			return
		}
		w.toast(d.title() + " を保存しました")
	})
}

// closeTab closes tab i, asking first if its text has unsaved changes.
func (s *windowState) closeTab(i int) {
	w := s.Widget().model
	if i < 0 || i >= len(w.tabs) {
		return
	}
	a := w.app
	d := a.docs[w.tabs[i]]
	done := func() {
		if j, ok := indexOf(w.tabs, d.id); ok {
			w.removeTab(j)
		}
		a.forget(d)
		a.saveSession()
	}
	if !d.dirty() {
		done()
		return
	}
	s.confirm(fmt.Sprintf("「%s」への変更を保存しますか？", d.title()), func(ans answer) {
		switch ans {
		case answerSave:
			if s.save(d) {
				done()
			}
		case answerDiscard:
			done()
		}
	})
}

// closeWindow closes the window, asking first if any of its texts has
// unsaved changes. Closing the last window quits.
func (s *windowState) closeWindow() {
	w := s.Widget().model
	a := w.app
	var dirty []*document
	for _, id := range w.tabs {
		if d := a.docs[id]; d.dirty() {
			dirty = append(dirty, d)
		}
	}
	done := func() {
		if len(a.windows) == 1 {
			a.quit()
			return
		}
		for _, id := range w.tabs {
			a.forget(a.docs[id])
		}
		w.closed = true
		a.dropWindow(w)
		a.saveSession()
		if w.handle != nil {
			w.handle.Close()
		}
	}
	if len(dirty) == 0 {
		done()
		return
	}
	message := fmt.Sprintf("「%s」への変更を保存しますか？", dirty[0].title())
	if len(dirty) > 1 {
		message = fmt.Sprintf("%d 個のテキストに保存していない変更があります。保存しますか？", len(dirty))
	}
	if s.asking {
		return
	}
	s.asking = true
	s.confirm(message, func(ans answer) {
		s.asking = false
		switch ans {
		case answerSave:
			for _, d := range dirty {
				if !s.save(d) {
					return
				}
			}
			done()
		case answerDiscard:
			for _, d := range dirty {
				d.text = d.saved
			}
			done()
		}
	})
}

// setFamily sets the editor in mincho or gothic.
func (s *windowState) setFamily(f store.Family) {
	a := s.Widget().model.app
	st := a.settings
	st.Family = f
	a.setSettings(st)
}

// toggleNotebook goes to the notebook, or back to the editor if there are
// tabs.
func (s *windowState) toggleNotebook() {
	w := s.Widget().model
	if s.onEditor {
		w.showNotebook()
		return
	}
	if len(w.tabs) > 0 {
		w.showEditor()
		w.refresh()
	}
}

// editText returns a command that runs fn on the field with the keyboard.
func (s *windowState) editText(fn func(*field.InputController)) func() {
	return func() { s.withEditor(func(e *editorState) { e.editText(fn) }) }
}

// withEditor runs fn on the editor page, if it shows.
func (s *windowState) withEditor(fn func(*editorState)) {
	if s.editor != nil && s.onEditor {
		fn(s.editor)
	}
}

// confirm asks whether to save before closing, and gives the answer to done.
// Escape and a click beside the dialog cancel.
func (s *windowState) confirm(message string, done func(answer)) {
	overlay.ShowDialog(s.Context(), func(dismiss func()) core.Widget {
		choose := func(ans answer) func() {
			return func() {
				dismiss()
				done(ans)
			}
		}
		return overlay.Dialog{
			Title:   message,
			Message: "保存しないと、変更は失われます。",
			Width:   400,
			Actions: []core.Widget{
				control.Button{Label: "保存しない", OnPress: choose(answerDiscard)},
				control.Button{Label: "キャンセル", OnPress: choose(answerCancel)},
				control.Button{Label: "保存", Style: control.ButtonPrimary, OnPress: choose(answerSave)},
			},
		}
	})
}

// indexOf returns the index of v in s.
func indexOf(s []int, v int) (int, bool) {
	for i, x := range s {
		if x == v {
			return i, true
		}
	}
	return 0, false
}
