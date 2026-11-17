package main

import (
	"fmt"
	"path/filepath"
	"time"
)

// scenes are the states of a window that -shot draws and the tests check.
func scenes() []string {
	return []string{"notebook", "search", "editor", "vertical", "find", "palette", "settings", "prompt"}
}

// demoNow is the time of the sample notebook, so that dates do not move.
func demoNow() time.Time { return time.Date(2026, 11, 9, 15, 30, 0, 0, time.Local) }

// newDemoApp makes an application over the sample notebook in memory.
func newDemoApp() (*app, *localShell, *memFS) {
	fs, st := newDemo(demoNow())
	sh := &localShell{}
	a := newApp(st, fs.disk(), sh, func(dir string) notebook { return memLibrary{fs: fs, dir: dir} })
	a.now = demoNow
	return a, sh, fs
}

// stage drives a window to a scene. settle runs what is posted and draws
// frames until the window comes to rest.
type stage struct {
	app    *app
	win    *windowModel
	settle func()
}

// demoPath is the file of a note of the sample notebook.
func demoPath(name string) string { return filepath.Join(demoDir(), name+".txt") }

// play puts the window in scene name.
func (st stage) play(name string) error {
	a, w := st.app, st.win
	st.settle()
	ws := w.state
	if ws == nil {
		return fmt.Errorf("the window was not built")
	}
	openKyoto := func() {
		ws.openPath(demoPath("京都の旅"))
		st.settle()
	}
	switch name {
	case "notebook":
	case "search":
		if ws.notebook == nil {
			return fmt.Errorf("no notebook page")
		}
		ws.notebook.typed("豆腐")
	case "editor":
		openKyoto()
	case "vertical":
		openKyoto()
		a.setVertical(w.current(), true)
	case "find":
		openKyoto()
		st.settle()
		ws.withEditor(func(e *editorState) {
			e.openFind(true)
			st.settle()
			e.queryChanged("を")
		})
	case "palette":
		openKyoto()
		ws.openPalette()
	case "settings":
		ws.openSettings()
	case "prompt":
		openKyoto()
		a.edit(w.current(), w.current().text+"追記。\n")
		ws.closeTab(w.selected)
	default:
		return fmt.Errorf("no scene %q; the scenes are %v", name, scenes())
	}
	st.settle()
	return nil
}
