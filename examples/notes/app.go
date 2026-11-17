package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
)

// Defaults for the settings that the store leaves zero.
const (
	defaultFontSize = 18
	minFontSize     = 10
	maxFontSize     = 48
	defaultAutosave = 5 * time.Second
	// searchLimit is how many hits a search shows.
	searchLimit = 60
)

// persistence is the part of *store.Store the application uses, so that the
// tests run on a store in memory.
type persistence interface {
	Settings() store.Settings
	SetSettings(store.Settings) error
	FileState(path string) store.FileState
	SetFileState(path string, st store.FileState) error
	Recent() []string
	AddRecent(path string) error
	Session() (store.Session, bool)
	SaveSession(store.Session) error
	SaveRecovery(key, text string) error
	Recovery(key string) (string, bool)
	DropRecovery(key string) error
	NewRecoveryKey() string
}

// notebook is the part of store.Library the application uses.
type notebook interface {
	Notes() ([]store.Note, error)
	Search(query string, limit int) ([]store.Hit, error)
	NewNote() (string, error)
}

// disk reads and writes text files: store.ReadDoc and store.WriteDoc.
type disk struct {
	read  func(path string) (store.Doc, error)
	write func(store.Doc) error
}

// shell is what the application asks of gowaku outside its widgets, so that
// tests and screenshots run without Run.
type shell interface {
	// post runs fn on the UI goroutine. It is safe to call from any goroutine.
	post(fn func())
	// async runs work off the UI goroutine, then what it returns on it.
	async(work func() (done func()))
	// after runs fn on the UI goroutine once d has passed.
	after(d time.Duration, fn func())
	openWindow(title string, root core.Widget, onClose func()) error
	tearOff(e nav.TabTearOff, title string, root core.Widget, onClose func()) error
	fileDialog(d gowaku.FileDialog, done func(paths []string, err error))
	// window returns the window of ctx.
	window(ctx core.BuildContext) windowHandle
	quit()
}

// windowHandle is the part of *gowaku.WindowHandle the windows use.
type windowHandle interface {
	SetTitle(title string)
	Close()
	SetOnCloseRequest(fn func() bool)
	MoveTo(e nav.TabTearOff)
	Activate()
}

// app is what the windows share: the store, the settings and every open
// document. It belongs to the UI goroutine.
type app struct {
	store      persistence
	disk       disk
	shell      shell
	newLibrary func(dir string) notebook
	library    notebook

	settings store.Settings
	// fontSize is the size of the editor now, which zooming changes without
	// touching the setting.
	fontSize float32

	docs     map[int]*document
	nextDoc  int
	untitled int
	windows  []*windowModel
	group    nav.TabGroup

	// notes is the notebook as last read, and notesErr why it could not be.
	notes    []store.Note
	notesErr error
	notesGen int

	// quitting means the session is saved for good and closing windows
	// must not change it.
	quitting bool
	// pending are the windows of the session after the first, opened once
	// Run has started.
	pending []*windowModel
	// now tells the time, and lastAutosave when autosave last ran.
	now          func() time.Time
	lastAutosave time.Time
	// warned are the failures of the store already told.
	warned map[string]bool
}

// newApp makes the application over its store.
func newApp(st persistence, d disk, sh shell, newLibrary func(dir string) notebook) *app {
	a := &app{store: st, disk: d, shell: sh, newLibrary: newLibrary, docs: make(map[int]*document), now: time.Now, warned: make(map[string]bool)}
	a.settings = resolveSettings(st.Settings())
	a.fontSize = a.settings.FontSize
	a.library = newLibrary(a.settings.Library)
	return a
}

// resolveSettings fills in the defaults of what s leaves zero.
func resolveSettings(s store.Settings) store.Settings {
	if s.FontSize <= 0 {
		s.FontSize = defaultFontSize
	}
	if s.AutosaveEvery <= 0 {
		s.AutosaveEvery = defaultAutosave
	}
	if s.Library == "" {
		s.Library = defaultLibrary()
	}
	return s
}

// defaultLibrary is the folder of the notebook when the settings name none.
func defaultLibrary() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "gowaku notes"
	}
	return filepath.Join(home, "Documents", "gowaku notes")
}

// family returns the font family of the editor.
func (a *app) family() string {
	if a.settings.Family == store.Gothic {
		return gothicFamily
	}
	return minchoFamily
}

// setSettings applies and saves new settings.
func (a *app) setSettings(s store.Settings) {
	s = resolveSettings(s)
	libraryMoved := !samePath(s.Library, a.settings.Library)
	if s.FontSize != a.settings.FontSize {
		a.fontSize = s.FontSize
	}
	a.settings = s
	a.warn(a.store.SetSettings(s))
	if libraryMoved {
		a.library = a.newLibrary(s.Library)
		a.reloadNotes()
	}
	a.refreshAll()
}

// warn tells the user, once for each kind, that the store failed. Such a
// failure costs a convenience, such as the recent files or the next session,
// but never the text, which saving writes or reports itself.
func (a *app) warn(err error) {
	if err == nil || a.warned[err.Error()] {
		return
	}
	a.warned[err.Error()] = true
	a.shell.post(func() {
		if len(a.windows) > 0 {
			a.windows[0].toast("設定や状態を保存できません: " + errText(err))
		}
	})
}

// zoom changes the size of the editor by steps; zero goes back to the setting.
func (a *app) zoom(step int) {
	switch {
	case step == 0:
		a.fontSize = a.settings.FontSize
	case step > 0:
		a.fontSize = min(a.fontSize+2, maxFontSize)
	default:
		a.fontSize = max(a.fontSize-2, minFontSize)
	}
	a.refreshAll()
}

// refreshAll builds every window again.
func (a *app) refreshAll() {
	for _, w := range a.windows {
		w.refresh()
	}
}

// --- documents

// addDoc registers d and returns it.
func (a *app) addDoc(d *document) *document {
	a.nextDoc++
	d.id = a.nextDoc
	a.docs[d.id] = d
	return d
}

// newUntitled makes an empty untitled document.
func (a *app) newUntitled() *document {
	a.untitled++
	return a.addDoc(&document{untitled: a.untitled})
}

// findPath returns the open document of the file at path.
func (a *app) findPath(path string) (*document, bool) {
	for _, d := range a.docs {
		if d.path != "" && samePath(d.path, path) {
			return d, true
		}
	}
	return nil, false
}

// windowOf returns the window that shows document id.
func (a *app) windowOf(id int) (*windowModel, int) {
	for _, w := range a.windows {
		if i := slices.Index(w.tabs, id); i >= 0 {
			return w, i
		}
	}
	return nil, -1
}

// readDoc reads the file at path into a new document.
func (a *app) readDoc(path string) (*document, error) {
	doc, err := a.disk.read(path)
	if err != nil {
		return nil, err
	}
	st := a.store.FileState(path)
	d := a.addDoc(&document{
		path: path, text: doc.Text, saved: doc.Text, crlf: doc.CRLF, bom: doc.BOM,
		vertical: st.Vertical, caret: min(max(st.Caret, 0), len(doc.Text)),
	})
	return d, nil
}

// open shows the file at path in w: in the tab that already shows it, in w or
// another window, or in a new tab. It returns the document.
func (a *app) open(w *windowModel, path string) (*document, error) {
	if d, ok := a.findPath(path); ok {
		if other, i := a.windowOf(d.id); other != nil {
			other.selectTab(i)
			if other != w {
				other.activate()
			}
			return d, nil
		}
	}
	d, err := a.readDoc(path)
	if err != nil {
		return nil, err
	}
	a.warn(a.store.AddRecent(path))
	w.addTab(d.id)
	a.saveSession()
	return d, nil
}

// save writes d to its file, or into the notebook if it has none: a new note
// belongs to the notebook, where search finds it, so Ctrl+S never stops to
// ask for a name; 名前を付けて保存 puts it anywhere else.
func (a *app) save(d *document) error {
	if d.path == "" {
		path, err := a.library.NewNote()
		if err != nil {
			return err
		}
		return a.saveTo(d, path)
	}
	return a.saveTo(d, d.path)
}

// saveTo writes d to path and makes path its file.
func (a *app) saveTo(d *document, path string) error {
	text := d.text
	if err := a.disk.write(store.Doc{Path: path, Text: text, CRLF: d.crlf, BOM: d.bom}); err != nil {
		return err
	}
	d.path, d.saved = path, text
	if d.recovery != "" {
		a.warn(a.store.DropRecovery(d.recovery))
		d.recovery, d.autosaved = "", ""
	}
	a.warn(a.store.AddRecent(path))
	a.rememberFile(d)
	a.reloadNotes()
	a.saveSession()
	a.refreshAll()
	return nil
}

// rememberFile keeps the state of d's file for the next time it opens.
func (a *app) rememberFile(d *document) {
	if d.path != "" {
		a.warn(a.store.SetFileState(d.path, store.FileState{Vertical: d.vertical, Caret: d.caret}))
	}
}

// forget closes d for good, throwing away unsaved changes.
func (a *app) forget(d *document) {
	a.rememberFile(d)
	if d.recovery != "" {
		a.warn(a.store.DropRecovery(d.recovery))
	}
	delete(a.docs, d.id)
}

// setVertical writes d top to bottom or left to right, and remembers it.
func (a *app) setVertical(d *document, vertical bool) {
	d.vertical = vertical
	a.rememberFile(d)
	a.refreshAll()
}

// edit takes new text for d from an editor.
func (a *app) edit(d *document, text string) {
	wasDirty := d.dirty()
	d.text = text
	if d.dirty() != wasDirty {
		a.refreshAll()
	}
}

// --- autosave and the session

// autosave keeps the text of every document with unsaved changes in the
// recovery folder, so that a crash or a closed window loses nothing.
func (a *app) autosave() {
	keys := false
	for _, id := range a.docIDs() {
		d := a.docs[id]
		switch {
		case d.dirty() && d.text != d.autosaved:
			if d.recovery == "" {
				d.recovery = a.store.NewRecoveryKey()
				keys = true
			}
			if err := a.store.SaveRecovery(d.recovery, d.text); err != nil {
				a.warn(err)
			} else {
				d.autosaved = d.text
			}
		case !d.dirty() && d.recovery != "":
			a.warn(a.store.DropRecovery(d.recovery))
			d.recovery, d.autosaved = "", ""
			keys = true
		}
	}
	if keys {
		a.saveSession()
	}
}

// tick autosaves when the interval of the settings has passed since the last
// time. The application calls it every second.
func (a *app) tick() {
	if now := a.now(); now.Sub(a.lastAutosave) >= a.settings.AutosaveEvery {
		a.lastAutosave = now
		a.autosave()
	}
}

// docIDs returns the open documents in a fixed order.
func (a *app) docIDs() []int {
	ids := make([]int, 0, len(a.docs))
	for id := range a.docs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// session returns the windows open now.
func (a *app) session() store.Session {
	var s store.Session
	for _, w := range a.windows {
		ws := store.WindowState{Active: w.selected}
		for _, id := range w.tabs {
			d := a.docs[id]
			ts := store.TabState{Path: d.path}
			if d.dirty() {
				ts.Recovery = d.recovery
			}
			ws.Tabs = append(ws.Tabs, ts)
		}
		s.Windows = append(s.Windows, ws)
	}
	return s
}

// saveSession saves the windows open now, unless the application is ending.
func (a *app) saveSession() {
	if !a.quitting {
		a.warn(a.store.SaveSession(a.session()))
	}
}

// restore opens the windows of the last session, with the unsaved text
// autosaved for them, and returns the first. Without a session it returns a
// window with the notebook.
func (a *app) restore() *windowModel {
	s, ok := a.store.Session()
	if !ok || len(s.Windows) == 0 {
		return a.newWindow()
	}
	var first *windowModel
	for _, ws := range s.Windows {
		w := a.newWindow()
		for _, ts := range ws.Tabs {
			if d := a.restoreTab(ts); d != nil {
				w.tabs = append(w.tabs, d.id)
			}
		}
		w.selected = min(max(ws.Active, 0), max(len(w.tabs)-1, 0))
		w.startInEditor = len(w.tabs) > 0
		if first == nil {
			first = w
		} else {
			a.pending = append(a.pending, w)
		}
	}
	return first
}

// restoreTab makes the document of one tab of the session, or nil if there
// is nothing left of it.
func (a *app) restoreTab(ts store.TabState) *document {
	var recovered string
	hasRecovery := false
	if ts.Recovery != "" {
		recovered, hasRecovery = a.store.Recovery(ts.Recovery)
	}
	var d *document
	if ts.Path != "" {
		if open, ok := a.findPath(ts.Path); ok {
			return open
		}
		var err error
		d, err = a.readDoc(ts.Path)
		if err != nil {
			if !hasRecovery {
				return nil
			}
			// The file went away; its text lives on as an untitled one.
			d = a.newUntitled()
		}
	} else {
		if !hasRecovery {
			return a.newUntitled()
		}
		d = a.newUntitled()
	}
	if hasRecovery {
		d.text, d.recovery, d.autosaved = recovered, ts.Recovery, recovered
	}
	return d
}

// start opens the windows of the session after the first, once Run runs.
func (a *app) start() {
	pending := a.pending
	a.pending = nil
	for _, w := range pending {
		a.openWindow(w)
	}
}

// newWindow registers an empty window.
func (a *app) newWindow() *windowModel {
	w := &windowModel{app: a}
	a.windows = append(a.windows, w)
	return w
}

// openWindow opens a window of the OS for w.
func (a *app) openWindow(w *windowModel) {
	if err := a.shell.openWindow(w.windowTitle(), windowView{model: w}, func() { a.windowClosed(w) }); err != nil {
		a.dropWindow(w)
	}
}

// dropWindow forgets w.
func (a *app) dropWindow(w *windowModel) {
	if i := slices.Index(a.windows, w); i >= 0 {
		a.windows = slices.Delete(a.windows, i, i+1)
	}
}

// windowClosed runs when the window of w has closed, by whatever means. A
// window the user closes, with its close button or the menu, asks about
// unsaved text first and leaves a.windows before it goes (see
// windowState.closeRequested). Any other close is the application ending
// without asking, by gowaku.Quit or the window system, so unsaved text stays
// in the session, autosaved, as quit leaves it.
func (a *app) windowClosed(w *windowModel) {
	if !slices.Contains(a.windows, w) || a.quitting {
		return
	}
	a.autosave()
	a.saveSession()
	a.quitting = true
}

// quit ends the application, as the menu does. Unsaved text stays in the
// session, autosaved, and comes back at the next start, so quitting asks
// nothing. Closing the last window asks first, as closing any window does.
func (a *app) quit() {
	a.autosave()
	a.saveSession()
	a.quitting = true
	a.shell.quit()
}

// --- the notebook

// reloadNotes reads the notebook again, off the UI goroutine.
func (a *app) reloadNotes() {
	a.notesGen++
	gen, lib := a.notesGen, a.library
	a.shell.async(func() func() {
		notes, err := lib.Notes()
		return func() {
			if gen != a.notesGen {
				return
			}
			a.notes, a.notesErr = notes, err
			a.refreshAll()
		}
	})
}

// search looks for query in the notebook off the UI goroutine, and gives the
// hits to done on it.
func (a *app) search(query string, done func([]store.Hit, error)) {
	lib := a.library
	a.shell.async(func() func() {
		hits, err := lib.Search(query, searchLimit)
		return func() { done(hits, err) }
	})
}

// recent returns the recent files.
func (a *app) recent() []string { return a.store.Recent() }

// errText is the Japanese for an error, for a toast.
func errText(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "ファイルが見つかりません"
	case errors.Is(err, os.ErrPermission):
		return "ファイルにアクセスできません"
	}
	return strings.TrimPrefix(err.Error(), "store: ")
}
