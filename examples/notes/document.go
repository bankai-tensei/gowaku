package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
)

// document is one text open in a tab: a file, or an untitled text that has
// not been saved yet. A document shows in one tab of one window at a time,
// because a tab's key must be unique among the bars of a TabGroup.
type document struct {
	id int
	// path is the file, or empty for an untitled text, which untitled numbers.
	path     string
	untitled int
	// text is what the editor holds, and saved what the file holds, so that
	// the two differ while there are unsaved changes.
	text, saved string
	// crlf and bom say how the file was written, so that saving keeps it.
	crlf, bom bool
	vertical  bool
	// recovery is the key of the autosaved text, or empty, and autosaved the
	// text saved under it.
	recovery, autosaved string
	// caret is the byte offset of the caret, as last seen by the editor.
	caret int
}

// dirty reports whether the text has changes that are not saved.
func (d *document) dirty() bool { return d.text != d.saved }

// title is the name of the tab.
func (d *document) title() string {
	if d.path == "" {
		if d.untitled <= 1 {
			return "無題"
		}
		return fmt.Sprintf("無題 %d", d.untitled)
	}
	return strings.TrimSuffix(filepath.Base(d.path), filepath.Ext(d.path))
}

// tabTitle is the title with a mark for unsaved changes, for the title bar
// of the window; the tab shows the mark as TabItem.Modified.
func (d *document) tabTitle() string {
	if d.dirty() {
		return "• " + d.title()
	}
	return d.title()
}

// rubyMarks finds the ruby of the editor's text in the notation of Aozora
// Bunko, so that the editor draws ｜漢字《かんじ》 as 漢字 with its reading.
func rubyMarks(value string, dst []text.RubyMark) []text.RubyMark {
	return aozora.Marks(value, dst)
}

// wholeMarks widens m to the whole ruby notation of each mark it falls
// inside, so that a search hit on 嵐山 selects ｜嵐山《あらしやま》: a selection
// that only holds whole marks leaves them drawn as ruby (ADR 0074), while one
// that ends inside a mark shows its notation raw.
func wholeMarks(s string, m match) match {
	var buf [16]text.RubyMark
	for _, mk := range aozora.Marks(lineAround(s, m.start, m.end), buf[:0]) {
		off := lineStart(s, m.start)
		start, end := mk.Start+off, mk.End+off
		if m.start > start && m.start < end {
			m.start = start
		}
		if m.end > start && m.end < end {
			m.end = end
		}
	}
	return m
}

// lineStart returns where the line holding i begins.
func lineStart(s string, i int) int { return strings.LastIndexByte(s[:i], '\n') + 1 }

// lineAround returns the lines that hold start..end; marks never cross a line.
func lineAround(s string, start, end int) string {
	from := lineStart(s, start)
	to := len(s)
	if i := strings.IndexByte(s[end:], '\n'); i >= 0 {
		to = end + i
	}
	return s[from:to]
}

// charCount counts the characters a reader sees: the text without its ruby
// notation, and without line breaks.
func charCount(s string) int {
	plain := aozora.Plain(s)
	return utf8.RuneCountInString(plain) - strings.Count(plain, "\n")
}

// lineColumn returns the line and column of the byte offset off, both from 1,
// the column in characters.
func lineColumn(s string, off int) (line, col int) {
	off = min(max(off, 0), len(s))
	before := s[:off]
	line = strings.Count(before, "\n") + 1
	start := strings.LastIndexByte(before, '\n') + 1
	return line, utf8.RuneCountInString(before[start:]) + 1
}

// match is a stretch of text that a search found, in bytes.
type match struct{ start, end int }

// findAll returns where query appears in s, in order and without overlaps.
// With fold, case does not matter.
func findAll(s, query string, fold bool, dst []match) []match {
	dst = dst[:0]
	if query == "" {
		return dst
	}
	for i := 0; i < len(s); {
		if end, ok := matchAt(s, i, query, fold); ok {
			dst = append(dst, match{i, end})
			i = end
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
	return dst
}

// matchAt reports whether query appears at byte i of s, and where it ends.
func matchAt(s string, i int, query string, fold bool) (int, bool) {
	if !fold {
		if strings.HasPrefix(s[i:], query) {
			return i + len(query), true
		}
		return 0, false
	}
	j := i
	for _, q := range query {
		if j >= len(s) {
			return 0, false
		}
		r, n := utf8.DecodeRuneInString(s[j:])
		if r != q && !sameFold(r, q) {
			return 0, false
		}
		j += n
	}
	return j, true
}

// sameFold reports whether a and b are the same letter in another case.
func sameFold(a, b rune) bool {
	for f := unicode.SimpleFold(a); f != a; f = unicode.SimpleFold(f) {
		if f == b {
			return true
		}
	}
	return false
}

// nextMatch returns the index of the first match that starts at or after
// from, wrapping to the first, or -1 if there are none.
func nextMatch(ms []match, from int) int {
	if len(ms) == 0 {
		return -1
	}
	for i, m := range ms {
		if m.start >= from {
			return i
		}
	}
	return 0
}

// prevMatch returns the index of the last match that ends at or before
// before, wrapping to the last, or -1 if there are none.
func prevMatch(ms []match, before int) int {
	if len(ms) == 0 {
		return -1
	}
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].end <= before {
			return i
		}
	}
	return len(ms) - 1
}

// replaceAll replaces every match in s with repl.
func replaceAll(s string, ms []match, repl string) string {
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, m := range ms {
		b.WriteString(s[last:m.start])
		b.WriteString(repl)
		last = m.end
	}
	b.WriteString(s[last:])
	return b.String()
}

// truncate cuts s to n characters on one line, with an ellipsis if it was
// longer.
func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// samePath reports whether a and b name the same file, as the file system of
// the OS compares names.
func samePath(a, b string) bool {
	return pathKey(a) == pathKey(b)
}
