package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/examples/notes/store"
	"github.com/bankai-tensei/gowaku/text/aozora"
)

// The notebook of the screenshots and the tests lives in memory, so that
// both are the same on every machine. memFS, memStore and memLibrary stand for
// the files, *store.Store and store.Library.

// memFile is a file of a memFS.
type memFile struct {
	text      string
	crlf, bom bool
	modified  time.Time
}

// memFS is a folder of text files in memory.
type memFS struct {
	files map[string]memFile
	now   time.Time
	// fail makes writes fail, for tests.
	fail bool
}

func newMemFS(now time.Time) *memFS { return &memFS{files: make(map[string]memFile), now: now} }

func (m *memFS) read(path string) (store.Doc, error) {
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return store.Doc{}, fmt.Errorf("read %s: %w", path, os.ErrNotExist)
	}
	return store.Doc{Path: path, Text: f.text, CRLF: f.crlf, BOM: f.bom}, nil
}

func (m *memFS) write(d store.Doc) error {
	if m.fail {
		return errors.New("ディスクがいっぱいです")
	}
	m.now = m.now.Add(time.Minute)
	m.files[filepath.Clean(d.Path)] = memFile{text: d.Text, crlf: d.CRLF, bom: d.BOM, modified: m.now}
	return nil
}

func (m *memFS) disk() disk { return disk{read: m.read, write: m.write} }

// memLibrary is a notebook over a memFS.
type memLibrary struct {
	fs  *memFS
	dir string
}

func (l memLibrary) paths() []string {
	var out []string
	for p := range l.fs.files {
		if strings.HasPrefix(p, filepath.Clean(l.dir)+string(filepath.Separator)) && strings.EqualFold(filepath.Ext(p), ".txt") {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if c := l.fs.files[b].modified.Compare(l.fs.files[a].modified); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return out
}

func (l memLibrary) note(p string) store.Note {
	f := l.fs.files[p]
	title, rest := "", ""
	lines := strings.Split(f.text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			title = strings.TrimSpace(line)
			rest = strings.Join(lines[i+1:], " ")
			break
		}
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return store.Note{Path: p, Title: title, Modified: f.modified, Preview: truncate(rest, 80)}
}

func (l memLibrary) Notes() ([]store.Note, error) {
	paths := l.paths()
	out := make([]store.Note, 0, len(paths))
	for _, p := range paths {
		out = append(out, l.note(p))
	}
	return out, nil
}

func (l memLibrary) Search(query string, limit int) ([]store.Hit, error) {
	var hits []store.Hit
	var ms []match
	for _, p := range l.paths() {
		t := l.fs.files[p].text
		ms = findAll(t, query, true, ms)
		for _, m := range ms {
			if len(hits) >= limit {
				return hits, nil
			}
			start := strings.LastIndexByte(t[:m.start], '\n') + 1
			end := len(t)
			if i := strings.IndexByte(t[m.end:], '\n'); i >= 0 {
				end = m.end + i
			}
			// The card shows the line as a reader sees it, without the
			// ruby notation, as store.Library.Search does.
			before, match, after := aozora.Plain(t[start:m.start]), t[m.start:m.end], aozora.Plain(t[m.end:end])
			if pl := aozora.Plain(t[start:end]); strings.Count(pl, match) == 1 {
				i := strings.Index(pl, match)
				before, after = pl[:i], pl[i+len(match):]
			}
			hits = append(hits, store.Hit{
				Note: l.note(p), Offset: m.start, Line: strings.Count(t[:m.start], "\n"),
				Before: before, Match: match, After: after,
			})
		}
	}
	return hits, nil
}

func (l memLibrary) NewNote() (string, error) {
	for i := 1; ; i++ {
		p := filepath.Join(l.dir, fmt.Sprintf("ノート %d.txt", i))
		if _, ok := l.fs.files[p]; !ok {
			l.fs.files[p] = memFile{modified: l.fs.now}
			return p, nil
		}
	}
}

// memStore is a store in memory.
type memStore struct {
	settings store.Settings
	files    map[string]store.FileState
	recent   []string
	session  *store.Session
	recovery map[string]string
	next     int
}

func newMemStore() *memStore {
	return &memStore{files: make(map[string]store.FileState), recovery: make(map[string]string)}
}

func (m *memStore) Settings() store.Settings           { return m.settings }
func (m *memStore) SetSettings(s store.Settings) error { m.settings = s; return nil }
func (m *memStore) FileState(path string) store.FileState {
	return m.files[pathKey(path)]
}

func (m *memStore) SetFileState(path string, st store.FileState) error {
	m.files[pathKey(path)] = st
	return nil
}

func (m *memStore) Recent() []string { return slices.Clone(m.recent) }

func (m *memStore) AddRecent(path string) error {
	m.recent = slices.DeleteFunc(m.recent, func(p string) bool { return samePath(p, path) })
	m.recent = slices.Insert(m.recent, 0, path)
	m.recent = m.recent[:min(len(m.recent), 10)]
	return nil
}

func (m *memStore) Session() (store.Session, bool) {
	if m.session == nil {
		return store.Session{}, false
	}
	return *m.session, true
}

func (m *memStore) SaveSession(s store.Session) error { m.session = &s; return nil }

func (m *memStore) SaveRecovery(key, text string) error { m.recovery[key] = text; return nil }

func (m *memStore) Recovery(key string) (string, bool) {
	t, ok := m.recovery[key]
	return t, ok
}

func (m *memStore) DropRecovery(key string) error { delete(m.recovery, key); return nil }

func (m *memStore) NewRecoveryKey() string {
	m.next++
	return fmt.Sprintf("r%d", m.next)
}

// demoDir is the folder of the sample notebook.
func demoDir() string { return filepath.Join(string(filepath.Separator)+"demo", "ノート帳") }

// demoNotes are the notes of the sample notebook, oldest first.
func demoNotes() []struct{ name, text string } {
	return []struct{ name, text string }{
		{"俳句", "俳句\n古池や｜蛙《かわず》飛び込む水の音\n閑さや岩にしみ入る蝉の声\n五月雨をあつめて早し最上川\n"},
		{"買い物", "買い物\n米、味噌、ねぎ、豆腐、卵 🥚\n帰りに郵便局へ寄って、切手を十枚。\n"},
		{"読書メモ こころ", "読書メモ　こころ\n私《わたくし》はその人を常に先生と呼んでいた。だから此所でもただ先生と書くだけで本名は打ち明けない。\n\n下巻の遺書は、手紙の形で長く続く。\n"},
		{"gowaku の設計", "gowaku の設計\n縦書きと横書きを切り替えられること。ルビは青空文庫の記法で書く：｜漢字《かんじ》。\n絵文字 🎌 もカラーで出る。\n"},
		{"京都の旅", "京都の旅\n金曜の朝、｜嵐山《あらしやま》へ向かう。竹林の道を歩いて、渡月橋で川を眺める。\n昼は湯豆腐。抹茶は宇治まで足を延ばす 🍵\n夜は先斗町を歩く。\n"},
	}
}

// newDemo fills a memFS with the sample notebook and returns it with a store
// whose notebook is there.
func newDemo(now time.Time) (*memFS, *memStore) {
	fs := newMemFS(now.Add(-time.Duration(len(demoNotes())) * 26 * time.Hour))
	for _, n := range demoNotes() {
		fs.now = fs.now.Add(26 * time.Hour)
		if fs.now.After(now) {
			fs.now = now.Add(-time.Hour)
		}
		fs.files[filepath.Join(demoDir(), n.name+".txt")] = memFile{text: n.text, crlf: utf8.RuneCountInString(n.text)%2 == 0, modified: fs.now}
	}
	st := newMemStore()
	st.settings.Library = demoDir()
	return fs, st
}
