package aozora

import (
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/text"
)

const (
	// openNote and closeNote enclose a note (U+FF3B U+FF03 and U+FF3D).
	openNote  = "［＃"
	closeNote = "］"
	// openName and closeName enclose the text that a note of emphasis names.
	openName  = "「"
	closeName = "」に"
)

// emphasisOf returns the mark that a note of emphasis, such as 傍点, sets.
func emphasisOf(note string) (text.Emphasis, bool) {
	switch note {
	case "傍点":
		return text.EmphasisSesame, true
	case "白ゴマ傍点":
		return text.EmphasisOpenSesame, true
	case "丸傍点":
		return text.EmphasisCircle, true
	case "白丸傍点":
		return text.EmphasisOpenCircle, true
	case "二重丸傍点":
		return text.EmphasisDoubleCircle, true
	case "蛇の目傍点":
		return text.EmphasisBullseye, true
	case "黒三角傍点":
		return text.EmphasisTriangle, true
	case "白三角傍点":
		return text.EmphasisOpenTriangle, true
	case "ばつ傍点":
		return text.EmphasisCross, true
	}
	return text.EmphasisNone, false
}

// maxMarks returns how many marks s can have at most: one per ruby, and for a
// note one more than the bases with ruby that its text runs over.
func maxMarks(s string) int {
	return 2*strings.Count(s, openMark) + strings.Count(s, openNote)
}

// index returns the position of sub in s at from or later, or -1.
func index(s string, from int, sub string) int {
	if i := strings.Index(s[from:], sub); i >= 0 {
		return from + i
	}
	return -1
}

// firstAfter returns the index of the first mark of dst[own:] that starts at
// limit or later, or len(dst).
func firstAfter(dst []text.RubyMark, own, limit int) int {
	k := len(dst)
	for k > own && dst[k-1].Start >= limit {
		k--
	}
	return k
}

// emphasize reads the note at n. If it is a note of emphasis that names the
// text just before it, it turns that text into marks, taking over the marks of
// ruby in dst[first:] that the text holds, and returns them with where the
// note ends. The text cannot reach back past limit.
func emphasize(s string, dst []text.RubyMark, first, limit, n int) ([]text.RubyMark, int, bool) {
	name, mark, end, ok := readNote(s, n)
	if !ok {
		return dst, 0, false
	}
	start, plain, ok := named(s, dst, first, limit, n, name)
	if !ok {
		return dst, 0, false
	}
	// The stretches of text outside ruby become marks of their own. dst
	// makes room for them and fills from the back, so that nothing is
	// overwritten that is still to be read.
	k := len(dst)
	for range plain {
		dst = append(dst, text.RubyMark{})
	}
	w, pos := len(dst), n
	for pos > start {
		var m text.RubyMark
		if k > first && dst[k-1].End == pos {
			k--
			m = dst[k]
		} else {
			stop := pos
			for pos > start && (k <= first || dst[k-1].End != pos) {
				_, size := utf8.DecodeLastRuneInString(s[:pos])
				pos -= size
			}
			m = text.RubyMark{Start: pos, BaseStart: pos, BaseEnd: stop, RubyStart: stop, RubyEnd: stop, End: stop}
		}
		m.Emphasis = mark
		if w == len(dst) {
			m.End = end // the last stretch holds the note
		}
		w--
		dst[w] = m
		pos = m.Start
	}
	return dst, end, true
}

// readNote reads the note at n: the text it names, the mark it sets and where
// it ends. Only a note of emphasis on one line counts.
func readNote(s string, n int) (name string, mark text.Emphasis, end int, ok bool) {
	at := n + len(openNote)
	c := at
	for c < len(s) && !strings.HasPrefix(s[c:], closeNote) {
		if s[c] == '\n' || s[c] == '\r' {
			return "", 0, 0, false
		}
		c++
	}
	body := s[at:min(c, len(s))]
	i := strings.LastIndex(body, closeName)
	if c >= len(s) || !strings.HasPrefix(body, openName) || i < len(openName) {
		return "", 0, 0, false
	}
	mark, ok = emphasisOf(body[i+len(closeName):])
	name = body[len(openName):i]
	return name, mark, c + len(closeNote), ok && name != ""
}

// named finds name in s just before n, read as plain text: the marks of ruby
// in dst[first:] read as their base. It returns where the text starts and how
// many stretches of it lie outside those marks. The text cannot reach back
// past limit or over a line break, nor start or end inside a base.
func named(s string, dst []text.RubyMark, first, limit, n int, name string) (start, plain int, ok bool) {
	pos, j, k := n, len(name), len(dst)
	outside := false
	for j > 0 {
		if k > first && dst[k-1].End == pos {
			m := &dst[k-1]
			if !strings.HasSuffix(name[:j], s[m.BaseStart:m.BaseEnd]) {
				return 0, 0, false
			}
			j -= m.BaseEnd - m.BaseStart
			pos = m.Start
			k--
			outside = false
			continue
		}
		r, size := utf8.DecodeLastRuneInString(s[:pos])
		if pos <= limit || pos-size < limit || r == '\n' || r == '\r' || !strings.HasSuffix(name[:j], s[pos-size:pos]) {
			return 0, 0, false
		}
		if !outside {
			plain++
			outside = true
		}
		j -= size
		pos -= size
	}
	return pos, plain, true
}
