package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func reopen(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func sessionEqual(a, b Session) bool {
	return slices.EqualFunc(a.Windows, b.Windows, func(x, y WindowState) bool {
		return x.Active == y.Active && slices.Equal(x.Tabs, y.Tabs)
	})
}

func TestOpenCreates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	s := reopen(t, dir)
	if fi, err := os.Stat(filepath.Join(dir, "recovery")); err != nil || !fi.IsDir() {
		t.Fatalf("recovery folder: %v", err)
	}
	if s.Settings() != (Settings{}) {
		t.Fatalf("fresh settings = %+v", s.Settings())
	}
	if r := s.Recent(); len(r) != 0 {
		t.Fatalf("fresh recent = %v", r)
	}
	if _, ok := s.Session(); ok {
		t.Fatal("fresh store has a session")
	}
	if _, err := Open(""); err == nil {
		t.Fatal("Open of an empty folder succeeded")
	}
}

func TestDefaultDirs(t *testing.T) {
	d, err := DefaultDir()
	if err != nil {
		t.Skip(err)
	}
	if filepath.Base(d) != "gowaku-notes" {
		t.Fatalf("DefaultDir = %s", d)
	}
	l, err := DefaultLibraryDir()
	if err != nil {
		t.Skip(err)
	}
	if filepath.Base(l) != "gowaku notes" || filepath.Base(filepath.Dir(l)) != "Documents" {
		t.Fatalf("DefaultLibraryDir = %s", l)
	}
}

func TestSettingsPersist(t *testing.T) {
	tests := []Settings{
		{},
		{FontSize: 22, Family: Gothic, Library: `C:\notes`, AutosaveEvery: 3 * time.Second},
		{FontSize: 14.5, AutosaveEvery: 1500 * time.Millisecond},
		{Family: Mincho, Library: "/home/a/ノート"},
	}
	dir := t.TempDir()
	for _, want := range tests {
		if err := reopen(t, dir).SetSettings(want); err != nil {
			t.Fatal(err)
		}
		if got := reopen(t, dir).Settings(); got != want {
			t.Fatalf("Settings = %+v, want %+v", got, want)
		}
	}
}

func TestSettingsFileIsReadable(t *testing.T) {
	dir := t.TempDir()
	s := reopen(t, dir)
	if err := s.SetSettings(Settings{Family: Gothic, AutosaveEvery: 10 * time.Second}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"family": "gothic"`, `"autosaveEvery": "10s"`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("settings.json %s lacks %s", b, want)
		}
	}
	// Values a hand edit got wrong are left zero.
	bad := `{"fontSize": -3, "family": "comic", "autosaveEvery": "soon"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir).Settings(); got != (Settings{}) {
		t.Fatalf("Settings = %+v", got)
	}
}

func TestFileStatePersist(t *testing.T) {
	dir, docs := t.TempDir(), t.TempDir()
	a, b := filepath.Join(docs, "a.txt"), filepath.Join(docs, "b.txt")
	s := reopen(t, dir)
	if err := s.SetFileState(a, FileState{Vertical: true, Caret: 12}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFileState(b, FileState{Caret: 3}); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, dir)
	if got := s.FileState(a); got != (FileState{Vertical: true, Caret: 12}) {
		t.Fatalf("a = %+v", got)
	}
	// The same file named another way.
	if got := s.FileState(filepath.Join(docs, "sub", "..", "b.txt")); got != (FileState{Caret: 3}) {
		t.Fatalf("b = %+v", got)
	}
	if got := s.FileState(filepath.Join(docs, "c.txt")); got != (FileState{}) {
		t.Fatalf("c = %+v", got)
	}
	if err := s.SetFileState(a, FileState{}); err != nil {
		t.Fatal(err)
	}
	if got := reopen(t, dir).FileState(a); got != (FileState{}) {
		t.Fatalf("cleared a = %+v", got)
	}
}

func TestRecent(t *testing.T) {
	dir, docs := t.TempDir(), t.TempDir()
	var paths []string
	for i := range 12 {
		p := filepath.Join(docs, string(rune('a'+i))+".txt")
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	s := reopen(t, dir)
	for _, p := range paths {
		if err := s.AddRecent(p); err != nil {
			t.Fatal(err)
		}
	}
	// Again, written another way: it moves to the head, not twice.
	if err := s.AddRecent(filepath.Join(docs, ".", "e.txt")); err != nil {
		t.Fatal(err)
	}
	want := []string{paths[4], paths[11], paths[10], paths[9], paths[8], paths[7], paths[6], paths[5], paths[3], paths[2]}
	if got := reopen(t, dir).Recent(); !slices.Equal(got, want) {
		t.Fatalf("Recent = %v\nwant %v", got, want)
	}
	if err := os.Remove(paths[11]); err != nil {
		t.Fatal(err)
	}
	want = append(want[:1:1], want[2:]...)
	if got := reopen(t, dir).Recent(); !slices.Equal(got, want) {
		t.Fatalf("Recent after a removal = %v\nwant %v", got, want)
	}
}

func TestSessionPersist(t *testing.T) {
	dir := t.TempDir()
	want := Session{Windows: []WindowState{
		{Tabs: []TabState{{Path: `C:\a.txt`}, {Recovery: "k1"}}, Active: 1},
		{Tabs: []TabState{{Path: "/b.txt", Recovery: "k2"}}},
	}}
	s := reopen(t, dir)
	if err := s.SaveSession(want); err != nil {
		t.Fatal(err)
	}
	// The store keeps its own copy.
	want.Windows[0].Tabs[0].Path = "changed"
	got, ok := s.Session()
	if !ok || got.Windows[0].Tabs[0].Path != `C:\a.txt` {
		t.Fatalf("Session aliases the saved one: %+v", got)
	}
	want.Windows[0].Tabs[0].Path = `C:\a.txt`
	got, ok = reopen(t, dir).Session()
	if !ok || !sessionEqual(got, want) {
		t.Fatalf("Session = %+v, %v\nwant %+v", got, ok, want)
	}
	if err := s.SaveSession(Session{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := reopen(t, dir).Session(); ok {
		t.Fatal("an empty session counts as one")
	}
}

func TestSessionClampsActive(t *testing.T) {
	dir := t.TempDir()
	data := `{"windows":[{"tabs":[{"path":"a"}],"active":5},{"tabs":[],"active":-2},{"tabs":[{},{}],"active":-1}]}`
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := reopen(t, dir).Session()
	if !ok || len(got.Windows) != 3 {
		t.Fatalf("Session = %+v, %v", got, ok)
	}
	for i, w := range got.Windows {
		if w.Active != 0 {
			t.Fatalf("window %d Active = %d", i, w.Active)
		}
	}
}

func TestCorruptJSON(t *testing.T) {
	for _, name := range []string{"settings.json", "files.json", "recent.json", "session.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			bad := []byte(`{"windows": [ {"tabs": 3`)
			if name == "recent.json" {
				bad = []byte(`["a", 7]`) // parses as JSON but not as a list of paths
			}
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, bad, 0o644); err != nil {
				t.Fatal(err)
			}
			s, err := Open(dir)
			if err != nil {
				t.Fatalf("Open failed on a corrupt %s: %v", name, err)
			}
			if s.Settings() != (Settings{}) || len(s.files) != 0 || len(s.recent) != 0 {
				t.Fatalf("store not empty: %+v %v %v", s.Settings(), s.files, s.recent)
			}
			if _, ok := s.Session(); ok {
				t.Fatal("corrupt session counts")
			}
			b, err := os.ReadFile(path + ".bad")
			if err != nil || string(b) != string(bad) {
				t.Fatalf("backup: %q, %v", b, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("corrupt file left in place: %v", err)
			}
			// The store works on.
			if err := s.SetSettings(Settings{FontSize: 20}); err != nil {
				t.Fatal(err)
			}
			if reopen(t, dir).Settings().FontSize != 20 {
				t.Fatal("settings lost after recovery")
			}
		})
	}
}

func TestRecovery(t *testing.T) {
	dir := t.TempDir()
	s := reopen(t, dir)
	seen := map[string]bool{}
	for range 100 {
		k := s.NewRecoveryKey()
		if seen[k] || !validKey(k) {
			t.Fatalf("key %q repeated or invalid", k)
		}
		seen[k] = true
	}
	k1, k2 := s.NewRecoveryKey(), s.NewRecoveryKey()
	if err := s.SaveRecovery(k1, "書きかけ\nの文章"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery(k2, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery(k1, "書きかけ\nの文章、続き"); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, dir)
	if got, ok := s.Recovery(k1); !ok || got != "書きかけ\nの文章、続き" {
		t.Fatalf("Recovery(k1) = %q, %v", got, ok)
	}
	if got, ok := s.Recovery(k2); !ok || got != "" {
		t.Fatalf("Recovery(k2) = %q, %v", got, ok)
	}
	keys := s.RecoveryKeys()
	if len(keys) != 2 {
		t.Fatalf("RecoveryKeys = %v", keys)
	}
	if err := s.DropRecovery(k1); err != nil {
		t.Fatal(err)
	}
	if err := s.DropRecovery(k1); err != nil {
		t.Fatalf("dropping twice: %v", err)
	}
	if _, ok := s.Recovery(k1); ok {
		t.Fatal("dropped text still there")
	}
	for _, bad := range []string{"", "../x", `a\b`, "a b", "a.txt"} {
		if err := s.SaveRecovery(bad, "x"); err == nil {
			t.Fatalf("SaveRecovery(%q) succeeded", bad)
		}
		if _, ok := s.Recovery(bad); ok {
			t.Fatalf("Recovery(%q) found text", bad)
		}
	}
	ents, err := os.ReadDir(filepath.Join(dir, "recovery"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temporary file left: %s", e.Name())
		}
	}
}
