package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// maxRecent is how many recent files the store keeps.
const maxRecent = 10

// The files of the store, in its folder.
const (
	settingsFile = "settings.json"
	filesFile    = "files.json"
	recentFile   = "recent.json"
	sessionFile  = "session.json"
	recoveryDir  = "recovery"
)

// Store is the settings and the state kept in a folder of the user's
// configuration. Its methods are for the UI goroutine.
//
// Each part is a JSON file of the folder, read once by Open and written
// whole, through a temporary file, by each change. A file that does not
// parse is renamed to name.bad and the part starts empty, so that a damaged
// file never stops the application from starting.
type Store struct {
	dir      string
	settings Settings
	files    map[string]FileState
	recent   []string
	session  Session
}

// settingsJSON is the settings as settings.json writes them.
type settingsJSON struct {
	FontSize      float32 `json:"fontSize,omitempty"`
	Family        string  `json:"family,omitempty"`
	Library       string  `json:"library,omitempty"`
	AutosaveEvery string  `json:"autosaveEvery,omitempty"`
}

// fileStateJSON is a FileState as files.json writes it.
type fileStateJSON struct {
	Vertical bool `json:"vertical,omitempty"`
	Caret    int  `json:"caret,omitempty"`
}

// sessionJSON is a Session as session.json writes it.
type sessionJSON struct {
	Windows []windowJSON `json:"windows"`
}

type windowJSON struct {
	Tabs   []tabJSON `json:"tabs"`
	Active int       `json:"active,omitempty"`
}

type tabJSON struct {
	Path     string `json:"path,omitempty"`
	Recovery string `json:"recovery,omitempty"`
}

// Open opens the store in dir, creating it if needed. Use DefaultDir for the
// usual place.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("store: Open with an empty folder")
	}
	if err := os.MkdirAll(filepath.Join(dir, recoveryDir), 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, files: map[string]FileState{}}

	var set settingsJSON
	if err := s.load(settingsFile, &set); err != nil {
		return nil, err
	}
	s.settings = set.settings()

	var files map[string]fileStateJSON
	if err := s.load(filesFile, &files); err != nil {
		return nil, err
	}
	for k, v := range files {
		s.files[k] = FileState(v)
	}

	if err := s.load(recentFile, &s.recent); err != nil {
		return nil, err
	}
	if len(s.recent) > maxRecent {
		s.recent = s.recent[:maxRecent]
	}

	var ses sessionJSON
	if err := s.load(sessionFile, &ses); err != nil {
		return nil, err
	}
	s.session = ses.session()
	return s, nil
}

// load reads the JSON file name of the store into v. A missing file leaves v
// as it is; a file that does not parse is renamed to name.bad and leaves v
// zero.
func (s *Store) load(name string, v any) error {
	path := filepath.Join(s.dir, name)
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		if err := os.Rename(path, path+".bad"); err != nil {
			return fmt.Errorf("store: %s does not parse and cannot be put aside: %w", path, err)
		}
		// Unmarshal may have filled part of v before it failed.
		clearJSON(v)
	}
	return nil
}

// clearJSON sets what v points to back to zero.
func clearJSON(v any) {
	switch v := v.(type) {
	case *settingsJSON:
		*v = settingsJSON{}
	case *map[string]fileStateJSON:
		*v = nil
	case *[]string:
		*v = nil
	case *sessionJSON:
		*v = sessionJSON{}
	default:
		panic(fmt.Sprintf("gowaku: store cannot clear %T", v))
	}
}

// save writes v as the JSON file name of the store.
func (s *Store) save(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, name), append(b, '\n'))
}

// DefaultDir returns the folder of the store in the user's configuration
// directory.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gowaku-notes"), nil
}

// DefaultLibraryDir returns the folder of the notebook when the settings name
// none: Documents/gowaku notes in the home directory.
func DefaultLibraryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents", "gowaku notes"), nil
}

// Settings returns the settings, with zero fields left zero.
func (s *Store) Settings() Settings { return s.settings }

// SetSettings saves the settings.
func (s *Store) SetSettings(set Settings) error {
	s.settings = set
	j := settingsJSON{FontSize: set.FontSize, Library: set.Library}
	if set.Family != Mincho {
		j.Family = set.Family.String()
	}
	if set.AutosaveEvery != 0 {
		j.AutosaveEvery = set.AutosaveEvery.String()
	}
	return s.save(settingsFile, j)
}

// settings returns the settings j writes, leaving out what does not parse.
func (j settingsJSON) settings() Settings {
	set := Settings{FontSize: j.FontSize, Library: j.Library}
	if j.FontSize < 0 {
		set.FontSize = 0
	}
	if j.Family == Gothic.String() {
		set.Family = Gothic
	}
	if d, err := time.ParseDuration(j.AutosaveEvery); err == nil && d > 0 {
		set.AutosaveEvery = d
	}
	return set
}

// pathKey returns the key of path in files.json and recent.json: the path
// made absolute and clean, in lower case on Windows, whose file names ignore
// case.
func pathKey(path string) string {
	p := cleanPath(path)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// cleanPath returns path absolute and clean, or only clean if it cannot be
// made absolute.
func cleanPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// FileState returns what is remembered of the file at path.
func (s *Store) FileState(path string) FileState { return s.files[pathKey(path)] }

// SetFileState remembers st for the file at path.
func (s *Store) SetFileState(path string, st FileState) error {
	k := pathKey(path)
	if old, ok := s.files[k]; ok && old == st {
		return nil
	}
	if st == (FileState{}) {
		if _, ok := s.files[k]; !ok {
			return nil
		}
		delete(s.files, k)
	} else {
		s.files[k] = st
	}
	j := make(map[string]fileStateJSON, len(s.files))
	for k, v := range s.files {
		j[k] = fileStateJSON(v)
	}
	return s.save(filesFile, j)
}

// Recent returns the files opened last, the newest first, at most ten.
// Files that no longer exist are left out.
func (s *Store) Recent() []string {
	var out []string
	for _, p := range s.recent {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// AddRecent puts path at the head of the recent files.
func (s *Store) AddRecent(path string) error {
	p, k := cleanPath(path), pathKey(path)
	rec := make([]string, 0, maxRecent)
	rec = append(rec, p)
	for _, q := range s.recent {
		if len(rec) == maxRecent {
			break
		}
		if pathKey(q) != k {
			rec = append(rec, q)
		}
	}
	if slices.Equal(rec, s.recent) {
		return nil
	}
	s.recent = rec
	return s.save(recentFile, rec)
}

// Session returns the windows of the last session, and whether there was one.
// A session without windows counts as none.
func (s *Store) Session() (Session, bool) {
	if len(s.session.Windows) == 0 {
		return Session{}, false
	}
	return cloneSession(s.session), true
}

// SaveSession saves the windows open now.
func (s *Store) SaveSession(ses Session) error {
	s.session = cloneSession(ses)
	j := sessionJSON{Windows: make([]windowJSON, 0, len(ses.Windows))}
	for _, w := range ses.Windows {
		wj := windowJSON{Tabs: make([]tabJSON, 0, len(w.Tabs)), Active: w.Active}
		for _, t := range w.Tabs {
			wj.Tabs = append(wj.Tabs, tabJSON(t))
		}
		j.Windows = append(j.Windows, wj)
	}
	return s.save(sessionFile, j)
}

// session returns the session j writes, with each Active in the range of
// its tabs.
func (j sessionJSON) session() Session {
	var ses Session
	for _, wj := range j.Windows {
		w := WindowState{Active: wj.Active}
		for _, t := range wj.Tabs {
			w.Tabs = append(w.Tabs, TabState(t))
		}
		if w.Active >= len(w.Tabs) {
			w.Active = len(w.Tabs) - 1
		}
		if w.Active < 0 {
			w.Active = 0
		}
		ses.Windows = append(ses.Windows, w)
	}
	return ses
}

// cloneSession copies ses, so that the caller's slices and the store's do
// not alias.
func cloneSession(ses Session) Session {
	out := Session{Windows: make([]WindowState, len(ses.Windows))}
	for i, w := range ses.Windows {
		out.Windows[i] = WindowState{Tabs: slices.Clone(w.Tabs), Active: w.Active}
	}
	return out
}

// validKey reports whether key can name a file of the recovery folder: it
// is what NewRecoveryKey returns, letters, digits, '-' and '_'.
func validKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, c := range key {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// recoveryPath returns the file of the text autosaved under key.
func (s *Store) recoveryPath(key string) string {
	return filepath.Join(s.dir, recoveryDir, key+".txt")
}

// SaveRecovery autosaves text under key, a name NewRecoveryKey returned.
func (s *Store) SaveRecovery(key, text string) error {
	if !validKey(key) {
		return fmt.Errorf("store: %q is not a recovery key", key)
	}
	return writeAtomic(s.recoveryPath(key), []byte(text))
}

// Recovery returns the text autosaved under key.
func (s *Store) Recovery(key string) (string, bool) {
	if !validKey(key) {
		return "", false
	}
	b, err := os.ReadFile(s.recoveryPath(key))
	if err != nil {
		return "", false
	}
	return string(b), true
}

// DropRecovery forgets the text autosaved under key, once it is saved or
// thrown away.
func (s *Store) DropRecovery(key string) error {
	if !validKey(key) {
		return fmt.Errorf("store: %q is not a recovery key", key)
	}
	err := os.Remove(s.recoveryPath(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// RecoveryKeys returns the keys of all the autosaved texts, sorted, so that
// the application can drop those no tab of the session refers to.
func (s *Store) RecoveryKeys() []string {
	ents, err := os.ReadDir(filepath.Join(s.dir, recoveryDir))
	if err != nil {
		return nil
	}
	var keys []string
	for _, e := range ents {
		k, ok := strings.CutSuffix(e.Name(), ".txt")
		if ok && !e.IsDir() && validKey(k) {
			keys = append(keys, k)
		}
	}
	return keys
}

// NewRecoveryKey returns a key no other autosaved text has.
//
// The key is the time in UTC and six random bytes, such as
// 20261109T153000-1a2b3c4d5e6f: it sorts by age and is safe as a file name.
func (s *Store) NewRecoveryKey() string {
	for {
		var r [6]byte
		_, _ = rand.Read(r[:]) // crypto/rand.Read never fails (Go 1.24)
		key := time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(r[:])
		if _, err := os.Lstat(s.recoveryPath(key)); errors.Is(err, fs.ErrNotExist) {
			return key
		}
	}
}
