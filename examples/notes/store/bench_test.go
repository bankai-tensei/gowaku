package store

import (
	"fmt"
	"strings"
	"testing"
)

// benchLibrary writes a notebook of 2000 notes of about 5 KB each, in 20
// folders, and returns its folder.
func benchLibrary(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	para := strings.Repeat("吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。Some ASCII text, too. ", 12)
	for i := range 2000 {
		text := fmt.Sprintf("ノート %d の｜題名《だいめい》\n%s\n%s番目の段落。\n%s", i, para, fmt.Sprint(i), para)
		writeNote(b, dir, fmt.Sprintf("f%02d/n%04d.txt", i%20, i), text, i)
	}
	return dir
}

// clearCache forgets every file read, for a cold run.
func clearCache() {
	cache.Lock()
	clear(cache.m)
	cache.Unlock()
}

func BenchmarkNotesCold(b *testing.B) {
	l := Library{Dir: benchLibrary(b)}
	for b.Loop() {
		clearCache()
		if n, err := l.Notes(); err != nil || len(n) != 2000 {
			b.Fatal(len(n), err)
		}
	}
}

func BenchmarkNotesWarm(b *testing.B) {
	l := Library{Dir: benchLibrary(b)}
	if _, err := l.Notes(); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if n, err := l.Notes(); err != nil || len(n) != 2000 {
			b.Fatal(len(n), err)
		}
	}
}

func BenchmarkSearchWarm(b *testing.B) {
	l := Library{Dir: benchLibrary(b)}
	if _, err := l.Notes(); err != nil {
		b.Fatal(err)
	}
	for _, q := range []string{"1999番目", "ascii", "見当", "存在しない言葉"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				if _, err := l.Search(q, 200); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSearchCold(b *testing.B) {
	l := Library{Dir: benchLibrary(b)}
	for b.Loop() {
		clearCache()
		if _, err := l.Search("存在しない言葉", 200); err != nil {
			b.Fatal(err)
		}
	}
}
