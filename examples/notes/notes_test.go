package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
)

// harness is a window of the sample notebook under test.
type harness struct {
	t  *testing.T
	tt *gowakutest.Tester
	a  *app
	sh *localShell
	fs *memFS
	st *memStore
	w  *windowModel
}

// newHarness opens the first window of the sample notebook, after prepare
// has had its way with the store and the files.
func newHarness(t *testing.T, dark bool, prepare func(fs *memFS, st *memStore)) *harness {
	t.Helper()
	fs, st := newDemo(demoNow())
	if prepare != nil {
		prepare(fs, st)
	}
	sh := &localShell{}
	a := newApp(st, fs.disk(), sh, func(dir string) notebook { return memLibrary{fs: fs, dir: dir} })
	a.now = demoNow
	w := a.restore()
	h := &harness{t: t, a: a, sh: sh, fs: fs, st: st, w: w}
	h.tt = gowakutest.New(t, windowView{model: w}, gowakutest.Options{Width: windowWidth, Height: windowHeight, Dark: dark})
	h.settle()
	return h
}

// settle runs what was posted and lets the window come to rest.
func (h *harness) settle() {
	h.t.Helper()
	for range 20 {
		ran := h.sh.flush()
		h.tt.Settle()
		if !ran {
			return
		}
	}
}

func (h *harness) key(k input.KeyCode, m input.Modifiers) {
	h.t.Helper()
	h.tt.Key(k, m)
	h.settle()
}

func (h *harness) tap(label string) {
	h.t.Helper()
	h.tt.Tap(label)
	h.settle()
}

func (h *harness) typeText(s string) {
	h.t.Helper()
	h.tt.Type(s)
	h.settle()
}

// editor returns the State of the editor page.
func (h *harness) editor() *editorState {
	h.t.Helper()
	if h.w.state == nil || h.w.state.editor == nil || !h.w.state.onEditor {
		h.t.Fatalf("the window does not show the editor:\n%s", h.tt.Dump())
	}
	return h.w.state.editor
}

// selection returns the selection of the editor.
func (h *harness) selection() (int, int) {
	h.t.Helper()
	if h.editor().field() == nil {
		h.t.Fatal("the editor has no field")
	}
	return h.editor().selection()
}

// open opens a note of the sample notebook in a tab.
func (h *harness) open(name string) *document {
	h.t.Helper()
	h.w.state.openPath(demoPath(name))
	h.settle()
	d := h.w.current()
	if d == nil || !samePath(d.path, demoPath(name)) {
		h.t.Fatalf("%s did not open", name)
	}
	return d
}

func (h *harness) shot(name string) {
	if dir := os.Getenv("NOTES_SHOTS"); dir != "" {
		h.tt.SaveScreenshot(filepath.Join(dir, name+".png"))
	}
}

func TestNotebook_SearchOpensTheNoteAtTheHit(t *testing.T) {
	h := newHarness(t, false, nil)
	if _, ok := h.tt.Find("京都の旅"); !ok {
		t.Fatalf("the notebook shows no card for 京都の旅:\n%s", h.tt.Dump())
	}
	h.tap("ノートを検索")
	h.typeText("豆腐")
	if _, ok := h.tt.Find("「豆腐」が 2 か所に見つかりました"); !ok {
		t.Fatalf("no summary of the hits:\n%s", h.tt.Dump())
	}
	h.tap("京都の旅 昼は湯豆腐。抹茶は宇治まで足を延ばす 🍵")
	d := h.w.current()
	if d == nil || filepath.Base(d.path) != "京都の旅.txt" {
		t.Fatalf("opened %v, want 京都の旅", d)
	}
	want := strings.Index(d.text, "豆腐")
	start, end := h.selection()
	if start != want || end != want+len("豆腐") {
		t.Errorf("selection %d..%d, want the hit at %d..%d", start, end, want, want+len("豆腐"))
	}
	if !h.editor().field().Focused() {
		t.Error("the editor does not have the keyboard")
	}
	// Back to the notebook, with the results still there.
	h.key(input.KeyL, ctrlShift)
	if h.w.state.onEditor {
		t.Fatal("Ctrl+Shift+L did not go back to the notebook")
	}
	if _, ok := h.tt.Find("「豆腐」が 2 か所に見つかりました"); !ok {
		t.Errorf("the results are gone:\n%s", h.tt.Dump())
	}
}

func TestVertical_IsRememberedPerFile(t *testing.T) {
	h := newHarness(t, false, nil)
	d := h.open("京都の旅")
	h.key(input.KeyT, ctrlShift)
	if !d.vertical {
		t.Fatal("Ctrl+Shift+T did not turn the text vertical")
	}
	if _, ok := h.tt.Find("縦書き"); !ok {
		t.Errorf("the status bar does not say 縦書き:\n%s", h.tt.Dump())
	}
	h.key(input.KeyW, ctrl)
	if !h.st.FileState(demoPath("京都の旅")).Vertical {
		t.Fatal("the store does not remember the vertical file")
	}
	d = h.open("京都の旅")
	if !d.vertical {
		t.Error("the file opened again horizontal")
	}
	if d2 := h.open("買い物"); d2.vertical {
		t.Error("another file opened vertical")
	}
}

func TestFind_ReplacesAndSelectsMatches(t *testing.T) {
	h := newHarness(t, false, func(fs *memFS, _ *memStore) {
		fs.files[demoPath("英語")] = memFile{text: "Gowaku gowaku GOWAKU go", modified: fs.now}
	})
	d := h.open("英語")
	h.key(input.KeyH, ctrl)
	h.typeText("gowaku")
	if got := len(h.editor().find.matches); got != 3 {
		t.Fatalf("%d matches ignoring case, want 3", got)
	}
	if start, end := h.selection(); start != 0 || end != 6 {
		t.Errorf("selection %d..%d, want the first match 0..6", start, end)
	}
	h.key(input.KeyF3, 0)
	if start, _ := h.selection(); start != 7 {
		t.Errorf("F3 selected from %d, want 7", start)
	}
	h.key(input.KeyF3, input.ModShift)
	if start, _ := h.selection(); start != 0 {
		t.Errorf("Shift+F3 selected from %d, want 0", start)
	}
	h.tap("大文字と小文字を区別する")
	if got := len(h.editor().search()); got != 1 {
		t.Errorf("%d matches with case, want 1", got)
	}
	h.tap("大文字と小文字を区別する")
	h.tap("置換後の文字列")
	h.typeText("和")
	h.tap("置換")
	if d.text != "和 gowaku GOWAKU go" {
		t.Errorf("after 置換: %q", d.text)
	}
	h.tap("すべて置換")
	if d.text != "和 和 和 go" {
		t.Errorf("after すべて置換: %q", d.text)
	}
	if !d.dirty() {
		t.Error("the replaced text is not marked unsaved")
	}
	h.key(input.KeyEscape, 0)
	if h.editor().find.open {
		t.Error("Escape did not close the bar")
	}
	// The text has the keyboard back, and one undo takes back all of
	// すべて置換.
	h.key(input.KeyZ, ctrl)
	if d.text != "和 gowaku GOWAKU go" {
		t.Errorf("after undo: %q", d.text)
	}
}

func TestEditor_StatusFollowsTheCaretAcrossTabs(t *testing.T) {
	h := newHarness(t, false, nil)
	d := h.open("買い物")
	h.key(input.KeyDown, 0)
	h.key(input.KeyDown, 0)
	h.key(input.KeyEnd, 0)
	const where = "3 行、18 列"
	if _, ok := h.tt.Find(where); !ok {
		t.Fatalf("the status bar does not say %s:\n%s", where, h.tt.Dump())
	}
	caret := strings.Index(d.text, "十枚。") + len("十枚。")
	if d.caret != caret {
		t.Errorf("the document keeps the caret at %d, want %d", d.caret, caret)
	}
	// Shift+Home selects backwards; the caret is the start of the line.
	h.key(input.KeyHome, input.ModShift)
	if _, ok := h.tt.Find("3 行、1 列"); !ok {
		t.Errorf("the status bar does not follow a backwards selection:\n%s", h.tt.Dump())
	}
	h.key(input.KeyEnd, 0)
	h.open("京都の旅")
	if _, ok := h.tt.Find("1 行、1 列"); !ok {
		t.Errorf("a new tab does not start at the top:\n%s", h.tt.Dump())
	}
	h.key(input.KeyTab, ctrl)
	if h.w.current() != d {
		t.Fatal("Ctrl+Tab did not go back to 買い物")
	}
	if _, ok := h.tt.Find(where); !ok {
		t.Errorf("the caret did not come back with the tab:\n%s", h.tt.Dump())
	}
	if start, end := h.selection(); start != caret || end != caret {
		t.Errorf("selection %d..%d after the tab came back, want %d", start, end, caret)
	}
	h.typeText("！")
	if !strings.Contains(d.text, "十枚。！") {
		t.Errorf("typing after the tab came back: %q", d.text)
	}
}

func TestCloseTab_AsksAboutUnsavedChanges(t *testing.T) {
	h := newHarness(t, false, nil)
	d := h.open("買い物")
	h.typeText("牛乳、")
	if !d.dirty() || !strings.HasPrefix(d.text, "牛乳、") {
		t.Fatalf("typing did not reach the document: %q", d.text)
	}
	const ask = "「買い物」への変更を保存しますか？"
	h.key(input.KeyW, ctrl)
	if _, ok := h.tt.Find(ask); !ok {
		t.Fatalf("Ctrl+W asked nothing:\n%s", h.tt.Dump())
	}
	h.tap("キャンセル")
	if len(h.w.tabs) != 1 {
		t.Fatal("キャンセル closed the tab")
	}
	h.key(input.KeyW, ctrl)
	h.tap("保存")
	if len(h.w.tabs) != 0 {
		t.Fatal("保存 did not close the tab")
	}
	if got := h.fs.files[demoPath("買い物")].text; !strings.HasPrefix(got, "牛乳、") {
		t.Errorf("the file holds %q", got)
	}
	h.open("買い物")
	h.typeText("パン、")
	h.key(input.KeyW, ctrl)
	h.tap("保存しない")
	if len(h.w.tabs) != 0 {
		t.Fatal("保存しない did not close the tab")
	}
	if got := h.fs.files[demoPath("買い物")].text; strings.Contains(got, "パン") {
		t.Errorf("保存しない saved: %q", got)
	}
	if h.w.state.onEditor {
		t.Error("the window still shows the editor without tabs")
	}
}

func TestPalette_FiltersAndRuns(t *testing.T) {
	h := newHarness(t, false, nil)
	d := h.open("京都の旅")
	h.key(input.KeyP, ctrlShift)
	if _, ok := h.tt.Find("新規"); !ok {
		t.Fatalf("the palette shows no commands:\n%s", h.tt.Dump())
	}
	h.typeText("縦")
	if _, ok := h.tt.Find("新規"); ok {
		t.Error("the palette did not filter")
	}
	if _, ok := h.tt.Find("縦書き"); !ok {
		t.Fatalf("縦書き is not offered:\n%s", h.tt.Dump())
	}
	h.key(input.KeyEnter, 0)
	if !d.vertical {
		t.Error("Enter did not run 縦書き")
	}
	// Ctrl+K opens it too, and finds notes by title.
	h.key(input.KeyK, ctrl)
	h.typeText("俳句")
	h.key(input.KeyDown, 0)
	h.key(input.KeyEnter, 0)
	if cur := h.w.current(); cur == nil || filepath.Base(cur.path) != "俳句.txt" {
		t.Errorf("the palette opened %v, want 俳句", cur)
	}
}

func TestSession_RestoresRecoveredText(t *testing.T) {
	h := newHarness(t, false, func(_ *memFS, st *memStore) {
		st.recovery["r1"] = "京都の旅\n書きかけの続き。\n"
		st.recovery["r2"] = "無題のまま閉じた文。"
		st.next = 2
		st.session = &store.Session{Windows: []store.WindowState{{
			Tabs: []store.TabState{
				{Path: demoPath("京都の旅"), Recovery: "r1"},
				{Path: demoPath("俳句")},
				{Recovery: "r2"},
			},
			Active: 2,
		}}}
	})
	if len(h.w.tabs) != 3 {
		t.Fatalf("%d tabs, want 3", len(h.w.tabs))
	}
	kyoto, haiku, untitled := h.a.docs[h.w.tabs[0]], h.a.docs[h.w.tabs[1]], h.a.docs[h.w.tabs[2]]
	if kyoto.text != "京都の旅\n書きかけの続き。\n" || !kyoto.dirty() {
		t.Errorf("京都の旅: %q, dirty %v", kyoto.text, kyoto.dirty())
	}
	if haiku.dirty() {
		t.Error("the saved tab came back unsaved")
	}
	if untitled.path != "" || untitled.text != "無題のまま閉じた文。" || !untitled.dirty() {
		t.Errorf("untitled: %q %q", untitled.path, untitled.text)
	}
	if h.w.current() != untitled || h.editor() == nil {
		t.Error("the active tab is not the third, on the editor")
	}
	if tab, ok := h.tt.FindRole(semantics.RoleTab, "京都の旅"); !ok || tab.Value == "" {
		t.Errorf("the tab of the recovered text is not marked:\n%s", h.tt.Dump())
	}
	// Edits autosave, and the session keeps their key.
	h.typeText("追記")
	h.a.autosave()
	key := untitled.recovery
	if got, ok := h.st.Recovery(key); !ok || !strings.Contains(got, "追記") {
		t.Errorf("autosave kept %q", got)
	}
	if s, _ := h.st.Session(); s.Windows[0].Tabs[2].Recovery != key {
		t.Errorf("the session keeps %+v", s.Windows[0].Tabs[2])
	}
	// Saving drops the autosaved text.
	h.key(input.KeyS, ctrl)
	if _, ok := h.st.Recovery(key); ok {
		t.Error("the recovery outlived the save")
	}
	if untitled.path == "" || !strings.HasPrefix(untitled.path, demoDir()) {
		t.Errorf("the untitled text went to %q, want the notebook", untitled.path)
	}
}

func TestDrop_OpensTextFilesAsTabs(t *testing.T) {
	h := newHarness(t, false, func(fs *memFS, _ *memStore) {
		fs.files[filepath.Clean("/desk/a.txt")] = memFile{text: "A"}
		fs.files[filepath.Clean("/desk/b.txt")] = memFile{text: "B", crlf: true}
	})
	files := []string{filepath.Clean("/desk/a.txt"), filepath.Clean("/desk/b.txt"), filepath.Clean("/desk/c.png")}
	e := core.DragEvent{Pos: geom.Offset{X: 500, Y: 400}, Kinds: input.DataFiles, Data: input.DragData{Files: files}, Allowed: input.DropCopy}
	h.tt.Tree().DispatchDrag(core.DragOver, e)
	h.settle()
	if !h.w.state.dropping {
		t.Error("the window does not light up under the drag")
	}
	h.tt.Tree().DispatchDrag(core.DragDrop, e)
	h.settle()
	if len(h.w.tabs) != 2 {
		t.Fatalf("%d tabs, want a.txt and b.txt", len(h.w.tabs))
	}
	if d := h.w.current(); d.text != "B" || !d.crlf {
		t.Errorf("the selected tab is %q, crlf %v", d.text, d.crlf)
	}
	if _, ok := h.tt.Find("CRLF"); !ok {
		t.Errorf("the status bar does not say CRLF:\n%s", h.tt.Dump())
	}
}

func TestOpen_SameFileSharesOneTab(t *testing.T) {
	h := newHarness(t, false, nil)
	d1 := h.open("俳句")
	h.open("買い物")
	d2 := h.open("俳句")
	if d1 != d2 || len(h.w.tabs) != 2 || h.w.current() != d1 {
		t.Errorf("opening 俳句 twice made %d tabs", len(h.w.tabs))
	}
	// Ctrl+Tab goes to the next tab, from the text too.
	h.key(input.KeyTab, ctrl)
	if cur := h.w.current(); cur == d1 {
		t.Error("Ctrl+Tab stayed on 俳句")
	}
}

// requestClose presses the close button of the OS on the window, and reports
// whether the window said it may go at once.
func (h *harness) requestClose() bool {
	h.t.Helper()
	if h.sh.closeRequest == nil {
		h.t.Fatal("the window put nothing on its close button")
	}
	ok := h.sh.closeRequest()
	h.settle()
	return ok
}

func TestWindows_CloseButtonAsksAboutUnsavedChanges(t *testing.T) {
	h := newHarness(t, false, nil)
	haiku := h.open("俳句")
	shopping := h.open("買い物")
	h.w.state.tearOff(nav.TabTearOff{Index: 0, Key: haiku.id})
	h.settle()
	if len(h.sh.opened) != 1 || len(h.a.windows) != 2 || len(h.w.tabs) != 1 {
		t.Fatalf("tear-off: %d windows opened, %d known, %d tabs left", len(h.sh.opened), len(h.a.windows), len(h.w.tabs))
	}
	torn := h.a.windows[1]
	if len(torn.tabs) != 1 || torn.tabs[0] != haiku.id {
		t.Fatalf("the new window has %v", torn.tabs)
	}
	h.a.edit(shopping, "変えた買い物")
	const ask = "「買い物」への変更を保存しますか？"
	if h.requestClose() {
		t.Fatal("a window with unsaved text must not close at once")
	}
	if _, ok := h.tt.Find(ask); !ok {
		t.Fatalf("the close button asked nothing:\n%s", h.tt.Dump())
	}
	// A second press while it asks asks no more.
	h.requestClose()
	h.tap("キャンセル")
	if _, ok := h.tt.Find(ask); ok {
		t.Fatal("the close button asked twice")
	}
	if !slices.Contains(h.a.windows, h.w) || h.w.closed {
		t.Fatal("キャンセル closed the window")
	}
	h.requestClose()
	h.tap("保存")
	if slices.Contains(h.a.windows, h.w) || !h.w.closed {
		t.Fatal("保存 did not close the window")
	}
	if got := h.fs.files[demoPath("買い物")].text; got != "変えた買い物" {
		t.Errorf("the file holds %q", got)
	}
	// The window goes; what gowaku tells afterward changes nothing.
	h.a.windowClosed(h.w)
	if h.a.quitting || len(h.a.windows) != 1 || h.a.docs[shopping.id] != nil {
		t.Errorf("after the close: quitting %v, %d windows, document kept %v", h.a.quitting, len(h.a.windows), h.a.docs[shopping.id] != nil)
	}
	s, _ := h.st.Session()
	if len(s.Windows) != 1 || len(s.Windows[0].Tabs) != 1 {
		t.Errorf("the session after the close is %+v", s)
	}
}

func TestWindows_CloseButtonOfTheLastWindowAsksThenQuits(t *testing.T) {
	h := newHarness(t, false, nil)
	d := h.open("買い物")
	h.a.edit(d, "変えた買い物")
	if h.requestClose() {
		t.Fatal("the last window with unsaved text must not close at once")
	}
	h.tap("保存しない")
	if !h.sh.quitted || !h.a.quitting {
		t.Fatal("保存しない on the last window did not quit")
	}
	if got := h.fs.files[demoPath("買い物")].text; got == "変えた買い物" {
		t.Error("保存しない saved")
	}
	s, _ := h.st.Session()
	if len(s.Windows) != 1 || len(s.Windows[0].Tabs) != 1 || s.Windows[0].Tabs[0].Recovery != "" {
		t.Errorf("the session at the end is %+v", s)
	}
	// Quit closes the window, which then goes without asking.
	if !h.requestClose() {
		t.Error("a window of an application that quits must go")
	}
}

func TestWindows_CloseButtonWithoutChangesClosesAtOnce(t *testing.T) {
	h := newHarness(t, false, nil)
	h.open("買い物")
	h.requestClose()
	if !h.sh.quitted {
		t.Error("the last window without changes did not quit")
	}
}

// gowaku.Quit and the window system close windows without asking: unsaved
// text stays in the session.
func TestWindows_QuitKeepsUnsavedText(t *testing.T) {
	h := newHarness(t, false, nil)
	haiku := h.open("俳句")
	h.open("買い物")
	h.w.state.tearOff(nav.TabTearOff{Index: 0, Key: haiku.id})
	h.settle()
	torn := h.a.windows[1]
	h.a.edit(haiku, "変えた俳句")
	h.a.windowClosed(torn)
	h.a.windowClosed(h.w)
	s, _ := h.st.Session()
	if len(s.Windows) != 2 || len(s.Windows[1].Tabs) != 1 || s.Windows[1].Tabs[0].Recovery == "" {
		t.Errorf("the session at the end is %+v", s)
	}
}

func TestZoom_ChangesTheSizeOfTheEditor(t *testing.T) {
	h := newHarness(t, false, nil)
	h.open("俳句")
	h.key(input.KeyEqual, ctrl)
	h.key(input.KeyEqual, ctrl)
	if h.a.fontSize != defaultFontSize+4 {
		t.Errorf("size %v after zooming in twice", h.a.fontSize)
	}
	h.key(input.Key0, ctrl)
	if h.a.fontSize != defaultFontSize {
		t.Errorf("size %v after Ctrl+0", h.a.fontSize)
	}
}

func TestScenes_Play(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, name := range scenes() {
			suffix := ""
			if dark {
				suffix = "-dark"
			}
			t.Run(name+suffix, func(t *testing.T) {
				h := newHarness(t, dark, nil)
				if err := (stage{app: h.a, win: h.w, settle: h.settle}).play(name); err != nil {
					t.Fatal(err)
				}
				h.shot(name + suffix)
			})
		}
	}
}

func TestDocument_Helpers(t *testing.T) {
	if got := charCount("あい\nう"); got != 3 {
		t.Errorf("charCount = %d, want 3", got)
	}
	if l, c := lineColumn("ab\nかきく", len("ab\nかき")); l != 2 || c != 3 {
		t.Errorf("lineColumn = %d:%d, want 2:3", l, c)
	}
	ms := findAll("Straße STRASSE ǅ ǆ", "ǆ", true, nil)
	if len(ms) != 2 {
		t.Errorf("folded matches of ǆ: %v", ms)
	}
	if got := formatCount(1234567); got != "1,234,567" {
		t.Errorf("formatCount = %q", got)
	}
	// rubyMarks is the hook for Input.Ruby; with the stub of aozora it
	// finds nothing, and it must not fail.
	_ = rubyMarks("｜漢字《かんじ》", []text.RubyMark(nil))
}
