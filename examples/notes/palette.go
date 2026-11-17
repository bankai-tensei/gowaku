package main

import (
	"path/filepath"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
)

// palette puts the command palette of the window around child: the commands
// that can run now, the recent files and the notes, found by fuzzy search.
// The palette is the library's field.CommandPalette (ADR 0097); the window
// only gives it the choices.
func (s *windowState) palette(acts windowActions, child core.Widget) core.Widget {
	return fieldw.CommandPalette{
		Actions:     enabledOnly(acts.commands()),
		Extra:       s.paletteItems(),
		Placeholder: "コマンド、最近のファイル、ノートを検索",
		// The window opens it with Ctrl+Shift+P and Ctrl+K itself, through
		// its menus, so the palette's own shortcut repeats the first.
		Shortcut: acts.palette.Shortcut,
		Child: basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
			s.paletteAt = ctx
			return child
		}},
	}
}

// enabledOnly returns the actions of list that can run now, which are all
// the palette lists: a command that cannot run is left out rather than
// grayed out.
func enabledOnly(list []interact.Action) []interact.Action {
	out := list[:0]
	for _, a := range list {
		if a.Enabled() {
			out = append(out, a)
		}
	}
	return out
}

// paletteItems are the choices of the palette besides the commands: the
// recent files and the notes.
func (s *windowState) paletteItems() []fieldw.PaletteItem {
	a := s.Widget().model.app
	recent := a.recent()
	items := make([]fieldw.PaletteItem, 0, len(recent)+len(a.notes))
	for _, path := range recent {
		items = append(items, fieldw.PaletteItem{Label: filepath.Base(path), Detail: "最近のファイル", OnInvoke: func() { s.openPath(path) }})
	}
	for _, n := range a.notes {
		items = append(items, fieldw.PaletteItem{Label: n.Title, Detail: "ノート", OnInvoke: func() { s.openPath(n.Path) }})
	}
	return items
}

// openPalette shows the command palette over the window.
func (s *windowState) openPalette() {
	if s.paletteAt != (core.BuildContext{}) {
		fieldw.OpenCommandPalette(s.paletteAt)
	}
}
