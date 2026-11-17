package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/text/aozora"
)

// The lengths, in runes, of a note's title and preview.
const (
	titleRunes   = 60
	previewRunes = 120
)

// Notes returns the notes, the most recently modified first.
//
// A missing folder is an empty notebook. Files and folders whose names start
// with a dot are left out. A file that cannot be read, or is not UTF-8, is
// listed under its file name with no preview.
func (l Library) Notes() ([]Note, error) {
	ents, err := l.scan()
	if err != nil {
		return nil, err
	}
	notes := make([]Note, len(ents))
	for i, e := range ents {
		notes[i] = e.note
	}
	return notes, nil
}

// dir returns the folder of the notebook, DefaultLibraryDir when Dir is empty.
func (l Library) dir() (string, error) {
	if l.Dir != "" {
		return cleanPath(l.Dir), nil
	}
	d, err := DefaultLibraryDir()
	if err != nil {
		return "", err
	}
	return cleanPath(d), nil
}

// NewNote creates an empty note with a free name in the folder of the
// notebook and returns its path. The names are 無題.txt, 無題 2.txt, 無題 3.txt
// and so on; the folder is created if needed.
func (l Library) NewNote() (string, error) {
	dir, err := l.dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		name := "無題.txt"
		if n > 1 {
			name = "無題 " + strconv.Itoa(n) + ".txt"
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, f.Close()
	}
}

// entry is what the library knows of one file: the note, and the text the
// search reads. Entries are not changed once made, so that they are shared
// between goroutines without a lock.
type entry struct {
	note Note
	size int64
	// ok says the file was read and is UTF-8; idx is empty otherwise.
	ok  bool
	idx index
}

// cache keeps the entries of the files read so far, by clean absolute path,
// so that listing and searching the notebook again reads only the files that
// changed. An entry is good while the file keeps its modification time and
// size.
//
// It is a package variable, against CODING_STYLE §3.2, because Library is a
// value type whose zero value must work: the cache cannot live in it. The
// mutex makes Library safe for concurrent use.
var cache = struct {
	sync.Mutex
	m map[string]*entry
}{m: map[string]*entry{}}

// found is a .txt file the walk of the notebook found.
type found struct {
	path string
	mod  time.Time
	size int64
}

// scan walks the notebook and returns the entries of its notes, the most
// recently modified first, reading the files that changed since the last
// scan.
func (l Library) scan() ([]*entry, error) {
	dir, err := l.dir()
	if err != nil {
		return nil, err
	}
	files, err := walk(dir)
	if err != nil {
		return nil, err
	}

	ents := make([]*entry, len(files))
	var stale []int
	cache.Lock()
	for i, f := range files {
		if e := cache.m[f.path]; e != nil && e.note.Modified.Equal(f.mod) && e.size == f.size {
			ents[i] = e
		} else {
			stale = append(stale, i)
		}
	}
	cache.Unlock()

	readAll(files, stale, ents)

	cache.Lock()
	for _, i := range stale {
		cache.m[files[i].path] = ents[i]
	}
	// Forget the files of this folder that are gone.
	prefix := dir + string(filepath.Separator)
	for p := range cache.m {
		if strings.HasPrefix(p, prefix) {
			if _, ok := slices.BinarySearchFunc(files, p, func(f found, p string) int { return strings.Compare(f.path, p) }); !ok {
				delete(cache.m, p)
			}
		}
	}
	cache.Unlock()

	slices.SortStableFunc(ents, func(a, b *entry) int {
		if c := b.note.Modified.Compare(a.note.Modified); c != 0 {
			return c
		}
		return strings.Compare(a.note.Path, b.note.Path)
	})
	return ents, nil
}

// walk returns the .txt files under dir, sorted by path, leaving out the
// names that start with a dot. A missing dir has no files.
func walk(dir string) ([]found, error) {
	var files []found
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			// A folder that cannot be read inside the notebook is left out.
			return nil
		}
		name := d.Name()
		if path != dir && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() || !strings.EqualFold(filepath.Ext(name), ".txt") {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil // removed since the folder was read
		}
		files = append(files, found{path: path, mod: fi.ModTime(), size: fi.Size()})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	slices.SortFunc(files, func(a, b found) int { return strings.Compare(a.path, b.path) })
	return files, err
}

// readAll reads the files at the indices todo into ents, on as many
// goroutines as there are processors, since a cold notebook is thousands of
// small files.
func readAll(files []found, todo []int, ents []*entry) {
	workers := min(runtime.GOMAXPROCS(0), (len(todo)+31)/32)
	if workers <= 1 {
		for _, i := range todo {
			ents[i] = readEntry(files[i])
		}
		return
	}
	var wg sync.WaitGroup
	next := make(chan int)
	for range workers {
		wg.Go(func() {
			for i := range next {
				ents[i] = readEntry(files[i])
			}
		})
	}
	for _, i := range todo {
		next <- i
	}
	close(next)
	wg.Wait()
}

// readEntry reads the file f into an entry.
func readEntry(f found) *entry {
	e := &entry{size: f.size, note: Note{Path: f.path, Modified: f.mod}}
	b, err := os.ReadFile(f.path)
	var d Doc
	if err == nil {
		d, err = decodeDoc(f.path, b)
	}
	if err != nil {
		e.note.Title = fileTitle(f.path)
		return e
	}
	e.ok = true
	e.note.Title, e.note.Preview = titleAndPreview(d.Text)
	if e.note.Title == "" {
		e.note.Title = fileTitle(f.path)
	}
	e.idx = makeIndex(d.Text)
	return e
}

// fileTitle returns the name of the file at path without its extension.
func fileTitle(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// titleAndPreview returns the first line of s that is not empty, with its
// notation taken out, and the text after it on one line.
func titleAndPreview(s string) (title, preview string) {
	rest := s
	for rest != "" {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		if t := strings.TrimSpace(aozora.Plain(line)); t != "" {
			title = cutRunes(t, titleRunes)
			break
		}
	}
	if title == "" {
		return "", ""
	}
	var b strings.Builder
	n := 0
	for rest != "" && n < previewRunes {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		for w := range strings.FieldsSeq(aozora.Plain(line)) {
			if n >= previewRunes {
				break
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
				n++
			}
			w = cutRunes(w, previewRunes-n)
			b.WriteString(w)
			n += utf8.RuneCountInString(w)
		}
	}
	return title, strings.TrimRightFunc(b.String(), unicode.IsSpace)
}

// cutRunes returns the first n runes of s.
func cutRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
