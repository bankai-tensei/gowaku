package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/text/aozora"
)

// base is the modification time of the first note a test writes.
var base = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// writeNote writes text to the file rel of dir, modified at base plus age
// minutes, and returns its path.
func writeNote(t testing.TB, dir, rel, text string, age int) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := base.Add(time.Duration(age) * time.Minute)
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	return path
}

// skipWithoutAozora skips a test of the notation until text/aozora reads it.
func skipWithoutAozora(t *testing.T) {
	t.Helper()
	const s = "｜漢字《かんじ》"
	if aozora.Plain(s) != "漢字" || len(aozora.Marks(s, nil)) != 1 {
		t.Skip("text/aozora does not read the notation yet")
	}
}

func TestNotesOrderAndFilter(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "old.txt", "古い", 1)
	writeNote(t, dir, "sub/deep/new.txt", "新しい", 9)
	writeNote(t, dir, "mid.TXT", "中", 5)
	writeNote(t, dir, "same-b.txt", "b", 3)
	writeNote(t, dir, "same-a.txt", "a", 3)
	writeNote(t, dir, "image.png", "x", 10)
	writeNote(t, dir, "readme.md", "x", 10)
	writeNote(t, dir, ".hidden.txt", "x", 10)
	writeNote(t, dir, ".git/x.txt", "x", 10)
	writeNote(t, dir, "sub/.cache/y.txt", "x", 10)
	writeNote(t, dir, ".a.txt.123.tmp", "x", 10)

	notes, err := Library{Dir: dir}.Notes()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range notes {
		rel, err := filepath.Rel(dir, n.Path)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	want := []string{"sub/deep/new.txt", "mid.TXT", "same-a.txt", "same-b.txt", "old.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Notes = %v, want %v", got, want)
	}
	if !notes[0].Modified.Equal(base.Add(9 * time.Minute)) {
		t.Fatalf("Modified = %v", notes[0].Modified)
	}
	if !filepath.IsAbs(notes[0].Path) {
		t.Fatalf("Path %s is not absolute", notes[0].Path)
	}
}

func TestNotesMissingFolder(t *testing.T) {
	notes, err := Library{Dir: filepath.Join(t.TempDir(), "none")}.Notes()
	if err != nil || len(notes) != 0 {
		t.Fatalf("Notes = %v, %v", notes, err)
	}
}

func TestNotesSeesChanges(t *testing.T) {
	dir := t.TempDir()
	l := Library{Dir: dir}
	a := writeNote(t, dir, "a.txt", "一行目", 1)
	b := writeNote(t, dir, "b.txt", "二", 2)
	if n, err := l.Notes(); err != nil || len(n) != 2 || n[1].Title != "一行目" {
		t.Fatalf("Notes = %+v", n)
	}
	writeNote(t, dir, "a.txt", "書き直した", 3)
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	n, err := l.Notes()
	if err != nil || len(n) != 1 || n[0].Path != a || n[0].Title != "書き直した" {
		t.Fatalf("Notes after changes = %+v", n)
	}
}

func TestTitleAndPreview(t *testing.T) {
	long := strings.Repeat("あ", 100)
	tests := []struct {
		name, text, title, preview string
	}{
		{"empty", "", "note", ""},
		{"blank lines", "\n  \n\t\n", "note", ""},
		{"first line", "題\n本文です。\n続き", "題", "本文です。 続き"},
		{"leading blanks", "\n\n  題名  \n\n本文", "題名", "本文"},
		{"whitespace collapsed", "t\n a \t b\n\n\n c　d", "t", "a b c d"},
		{"title capped", long, strings.Repeat("あ", 60), ""},
		{"preview capped", "t\n" + long + " " + long, "t", long + " " + strings.Repeat("あ", 19)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeNote(t, dir, "note.txt", tt.text, 0)
			n, err := Library{Dir: dir}.Notes()
			if err != nil || len(n) != 1 {
				t.Fatalf("Notes = %v, %v", n, err)
			}
			if n[0].Title != tt.title || n[0].Preview != tt.preview {
				t.Fatalf("Title, Preview = %q, %q\nwant %q, %q", n[0].Title, n[0].Preview, tt.title, tt.preview)
			}
			if utf8.RuneCountInString(n[0].Preview) > previewRunes {
				t.Fatalf("preview of %d runes", utf8.RuneCountInString(n[0].Preview))
			}
		})
	}
}

func TestTitleWithoutNotation(t *testing.T) {
	skipWithoutAozora(t)
	dir := t.TempDir()
	writeNote(t, dir, "a.txt", "｜吾輩《わがはい》は猫である\n名前は｜未《ま》だ無い。", 0)
	writeNote(t, dir, "b.txt", "\n｜《》\n本文", 1)
	n, err := Library{Dir: dir}.Notes()
	if err != nil {
		t.Fatal(err)
	}
	if n[1].Title != "吾輩は猫である" || n[1].Preview != "名前は未だ無い。" {
		t.Fatalf("a: %q, %q", n[1].Title, n[1].Preview)
	}
	_ = n[0] // b's first line may or may not be notation only; it must not panic.
}

func TestNotesUnreadable(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "sjis.txt", "\x82\xa0\x82\xa2", 0)
	n, err := Library{Dir: dir}.Notes()
	if err != nil || len(n) != 1 || n[0].Title != "sjis" || n[0].Preview != "" {
		t.Fatalf("Notes = %+v, %v", n, err)
	}
	hits, err := Library{Dir: dir}.Search("あ", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("Search = %+v, %v", hits, err)
	}
}

func TestNewNote(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notes")
	l := Library{Dir: dir}
	var got []string
	for range 3 {
		p, err := l.NewNote()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.Base(p))
		if b, err := os.ReadFile(p); err != nil || len(b) != 0 {
			t.Fatalf("new note %s: %q, %v", p, b, err)
		}
	}
	want := []string{"無題.txt", "無題 2.txt", "無題 3.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v", got)
	}
	// A freed name is taken again.
	if err := os.Remove(filepath.Join(dir, "無題 2.txt")); err != nil {
		t.Fatal(err)
	}
	p, err := l.NewNote()
	if err != nil || filepath.Base(p) != "無題 2.txt" {
		t.Fatalf("NewNote = %s, %v", p, err)
	}
}

func TestCutRunes(t *testing.T) {
	tests := []struct {
		s     string
		n     int
		first string
		last  string
	}{
		{"", 3, "", ""},
		{"abc", 0, "", ""},
		{"abc", 5, "abc", "abc"},
		{"あいうえお", 2, "あい", "えお"},
	}
	for _, tt := range tests {
		if got := cutRunes(tt.s, tt.n); got != tt.first {
			t.Errorf("cutRunes(%q, %d) = %q", tt.s, tt.n, got)
		}
		if got := lastRunes(tt.s, tt.n); got != tt.last {
			t.Errorf("lastRunes(%q, %d) = %q", tt.s, tt.n, got)
		}
	}
}
