package main

import (
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	navw "github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// Names of the pages of a window. The editor comes in two routes because a
// route is instant or not for good: the one restored at the start has no
// transition.
const (
	editorRoute        = "/editor"
	editorRouteInstant = "/editor/instant"
)

// heroKey tags the title of a note as it flies from its card to the editor:
// a note is one card in the list, offset -1, but may be several cards in the
// results of a search, one per hit.
type heroKey struct {
	path   string
	offset int
}

// windowModel is one window: its tabs, which are documents of the app, and
// the page it shows. The app owns it; the window's State shows it.
type windowModel struct {
	app      *app
	tabs     []int
	selected int
	// startInEditor shows the editor at once when the window opens.
	startInEditor bool
	// hero is the tag of the card the editor was last opened from.
	hero heroKey
	// pick is a stretch of text for the editor to select once it shows the
	// current document, as a hit of a search opened.
	pick *match
	// closed means the window closed on purpose, and its documents went with
	// it.
	closed bool

	state  *windowState
	handle windowHandle
}

// current returns the document of the selected tab, or nil.
func (w *windowModel) current() *document {
	if w.selected < 0 || w.selected >= len(w.tabs) {
		return nil
	}
	return w.app.docs[w.tabs[w.selected]]
}

// windowTitle is what the title bar of the OS says.
func (w *windowModel) windowTitle() string {
	if d := w.current(); d != nil && w.onEditor() {
		return d.tabTitle() + " — ノート"
	}
	return "ノート帳 — ノート"
}

// onEditor reports whether the window shows the editor.
func (w *windowModel) onEditor() bool {
	return w.state != nil && w.state.onEditor || w.state == nil && w.startInEditor
}

// refresh builds the window again.
func (w *windowModel) refresh() {
	if w.state != nil && !w.state.disposed {
		w.state.SetState(func() {})
	}
}

// addTab shows document id in a new tab after the selected one, and the
// editor.
func (w *windowModel) addTab(id int) {
	at := min(w.selected+1, len(w.tabs))
	w.tabs = slices.Insert(w.tabs, at, id)
	w.selected = at
	w.showEditor()
	w.refresh()
}

// insertTab puts document id at index i, as a tab dropped from another
// window.
func (w *windowModel) insertTab(i, id int) {
	i = min(max(i, 0), len(w.tabs))
	w.tabs = slices.Insert(w.tabs, i, id)
	w.selected = i
	w.showEditor()
	w.refresh()
	w.app.saveSession()
}

// selectTab shows tab i.
func (w *windowModel) selectTab(i int) {
	if i < 0 || i >= len(w.tabs) {
		return
	}
	w.remember()
	w.selected = i
	w.showEditor()
	w.refresh()
}

// remember notes where the caret of the current document is.
func (w *windowModel) remember() {
	if s := w.state; s != nil && s.editor != nil {
		s.editor.noteCaret()
	}
}

// removeTab takes tab i out without closing its document, selecting the tab
// on its right, and goes back to the notebook when none is left.
func (w *windowModel) removeTab(i int) {
	if i < 0 || i >= len(w.tabs) {
		return
	}
	w.tabs = slices.Delete(w.tabs, i, i+1)
	if w.selected > i || w.selected >= len(w.tabs) {
		w.selected = max(w.selected-1, 0)
	}
	if len(w.tabs) == 0 {
		w.showNotebook()
	}
	w.refresh()
}

// moveTab moves the tab at from to to.
func (w *windowModel) moveTab(from, to int) {
	if from < 0 || from >= len(w.tabs) || to < 0 || to >= len(w.tabs) {
		return
	}
	id := w.tabs[from]
	w.tabs = slices.Insert(slices.Delete(w.tabs, from, from+1), to, id)
	w.refresh()
	w.app.saveSession()
}

// showEditor brings the editor to the front, with the Hero of w.hero.
func (w *windowModel) showEditor() {
	if s := w.state; s != nil && s.attached && !s.onEditor {
		s.onEditor = true
		s.nav.PushNamed(editorRoute)
	}
}

// showNotebook goes back to the notebook.
func (w *windowModel) showNotebook() {
	if s := w.state; s != nil && s.attached && s.onEditor {
		w.remember()
		s.onEditor = false
		s.nav.PopUntil("/")
		w.app.reloadNotes()
	}
}

// activate brings the window to the front.
func (w *windowModel) activate() {
	if w.handle != nil {
		w.handle.Activate()
	}
}

// toast says msg at the bottom of the window.
func (w *windowModel) toast(msg string) {
	if s := w.state; s != nil && !s.disposed {
		overlay.ShowToast(s.Context(), overlay.Toast{Message: msg})
	}
}

// heroFor is the tag of the title of d in the editor: that of the card it was
// opened from, so that the title flies back to it.
func (w *windowModel) heroFor(d *document) heroKey {
	if d.path != "" && w.hero.path != "" && samePath(w.hero.path, d.path) {
		return w.hero
	}
	if d.path == "" {
		return heroKey{path: "\x00untitled", offset: d.id}
	}
	return heroKey{path: d.path, offset: -1}
}

// appLocale is the locale of the notebook. Its menus and labels are fixed
// Japanese text, so the words the library supplies (the count line of the
// palette, the dialogs, the date pickers, the labels for screen readers) are
// Japanese too, whatever language the OS is in.
var appLocale = language.Japanese

// windowView is the root of a window. It gives the window, and the dialogs,
// menus and toasts opened from it, appLocale; Run gives the title bar the
// same through App.Locale.
type windowView struct {
	core.StatelessBase
	model *windowModel
}

// Build implements core.StatelessWidget.
func (v windowView) Build(ctx core.BuildContext) core.Widget {
	md := basic.MediaQueryOf(ctx)
	md.Locale = appLocale
	return basic.MediaQuery{Data: md, Child: windowContent{model: v.model}}
}

// windowContent is the window under the MediaQuery of windowView.
type windowContent struct {
	core.StatefulBase
	model *windowModel
}

type windowState struct {
	core.StateOf[windowContent]
	nav navw.NavigatorController
	// attached means the Navigator has been built, so that nav works.
	attached bool
	onEditor bool
	disposed bool
	// editor and notebook are the States of the pages while they are in the
	// tree.
	editor   *editorState
	notebook *notebookState
	// title is what the window was last called, and dropping that files are
	// dragged over it.
	title    string
	dropping bool
	// asking means that the window asks whether to save before it closes.
	asking bool
	// paletteAt is a place inside the command palette, which opens it.
	paletteAt core.BuildContext
}

// CreateState implements core.StatefulWidget.
func (windowContent) CreateState() core.State { return &windowState{} }

// InitState implements core.State.
func (s *windowState) InitState() {
	w := s.Widget().model
	w.state = s
	a := w.app
	// The first window starts the others of the session, now that Run runs.
	a.shell.post(func() {
		s.attached = true
		if w.startInEditor && len(w.tabs) > 0 {
			s.onEditor = true
			s.nav.PushNamed(editorRouteInstant)
			s.SetState(func() {})
		}
		a.start()
	})
	if len(a.notes) == 0 && a.notesErr == nil {
		a.reloadNotes()
	}
}

// Dispose implements core.State.
func (s *windowState) Dispose() {
	s.disposed = true
	if w := s.Widget().model; w.state == s {
		w.state = nil
	}
}

// Build implements core.State.
func (s *windowState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().model
	a := w.app
	th := basic.ThemeOf(ctx)
	w.handle = a.shell.window(ctx)
	w.handle.SetOnCloseRequest(s.closeRequested)
	if title := w.windowTitle(); title != s.title {
		s.title = title
		h := w.handle
		a.shell.post(func() { h.SetTitle(title) })
	}
	editor := func(navw.RouteParams) core.Widget { return editorPage{win: w} }
	nav := navw.Navigator{
		Home:       notebookPage{win: w},
		Controller: &s.nav,
		Routes: []navw.RoutePattern{
			{Pattern: editorRouteInstant, Build: editor, Instant: true},
			{Pattern: editorRoute, Build: editor},
		},
		OnChange: func(name string) {
			on := strings.HasPrefix(name, editorRoute)
			if on != s.onEditor {
				s.onEditor = on
			}
			s.SetState(func() {})
		},
	}
	acts := s.actions()
	body := basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Container{Color: th.Color.Sidebar, Child: s.menuBar(acts)},
		basic.Container{Height: 1, Color: th.Color.Hair},
		basic.Expanded{Child: nav},
	}}
	layers := []core.Widget{body}
	if s.dropping {
		layers = append(layers, basic.IgnorePointer{Child: dropHint(th)})
	}
	return s.palette(acts, interact.Actions{
		Actions: acts.extra(),
		Child: interact.DragTarget{
			Kinds:   input.DataFiles,
			OnHover: func(over bool) { s.SetState(func() { s.dropping = over }) },
			OnDrop:  func(d input.DragData, _ geom.Offset) { s.drop(d.Files) },
			Child:   basic.Stack{Fit: layout.StackExpand, Children: layers},
		},
	})
}

// closeRequested answers the close button of the window, Alt+F4 and the like:
// the window asks about unsaved text as ウィンドウを閉じる does, and closes
// itself once answered, so the request itself is refused. The last window
// quits, also after asking.
func (s *windowState) closeRequested() bool {
	w := s.Widget().model
	a := w.app
	if w.closed || a.quitting || !slices.Contains(a.windows, w) {
		return true
	}
	if !s.asking {
		s.closeWindow()
	}
	return false
}

// drop opens the text files dropped on the window, each in a tab.
func (s *windowState) drop(files []string) {
	s.SetState(func() { s.dropping = false })
	w := s.Widget().model
	skipped := 0
	for _, f := range files {
		if !isText(f) {
			skipped++
			continue
		}
		if _, err := w.app.open(w, f); err != nil {
			w.toast(filepath.Base(f) + " を開けません: " + errText(err))
		}
	}
	if skipped > 0 {
		w.toast("テキストファイル（.txt）だけを開けます")
	}
}

// isText reports whether the file at path is one the application opens.
func isText(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".text", ".md":
		return true
	}
	return false
}

// dropHint covers the window while files are dragged over it.
func dropHint(th *theme.Theme) core.Widget {
	return basic.Padding{Insets: geom.All(theme.Space8), Child: basic.Container{
		Color: th.Color.AccentTint, Radius: th.Radius.Group,
		Border: paint.Stroke{Width: 2, Color: th.Color.AccentInk},
		Child: basic.Center{Child: basic.Text{
			Content: "ドロップしてタブで開く", Style: colored(th.Text.Title, th.Color.AccentInk),
		}},
	}}
}

// colored returns s in color c.
func colored(s textStyle, c geom.Color) textStyle {
	s.Color = c
	return s
}
