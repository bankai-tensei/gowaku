package semantics

import (
	"unicode"
	"unicode/utf16"
)

// UTF16 returns s in UTF-16 code units, the offsets of TextInfo.
func UTF16(s string) []uint16 { return utf16.Encode([]rune(s)) }

// Line returns the index of the line that holds offset off.
func (t *TextInfo) Line(off int) int {
	i := 0
	for i < len(t.LineStarts) && t.LineStarts[i] <= off {
		i++
	}
	return i
}

// LineBounds returns where line i starts and ends, in a text of n code units.
// The end of a line is the start of the next one.
func (t *TextInfo) LineBounds(i, n int) (start, end int) {
	if i > 0 && i-1 < len(t.LineStarts) {
		start = t.LineStarts[i-1]
	}
	end = n
	if i < len(t.LineStarts) {
		end = t.LineStarts[i]
	}
	return start, end
}

// Lines returns the number of lines.
func (t *TextInfo) Lines() int { return len(t.LineStarts) + 1 }

// IsWordStart reports whether a word starts at off in s. A word is a run of
// letters and digits of one script, or a single punctuation mark, together with
// the spaces after it. Kanji, hiragana and katakana are scripts of their own,
// so 日本語のテキスト has the words 日本語, の and テキスト. That is coarser than
// the dictionary segmentation of a screen reader, and enough to move by.
func IsWordStart(s []uint16, off int) bool {
	if off <= 0 {
		return true
	}
	if off >= len(s) {
		return true
	}
	// The second half of a surrogate pair never starts anything.
	if utf16.IsSurrogate(rune(s[off])) && s[off] >= 0xDC00 {
		return false
	}
	prev := classAt(s, off-1)
	cur := classAt(s, off)
	switch {
	case cur == classSpace:
		return false
	case cur == classPunct:
		return true
	case prev == classSpace:
		return true
	}
	return cur != prev
}

type charClass uint8

const (
	classSpace charClass = iota
	classPunct
	classWord
	classHan
	classHiragana
	classKatakana
)

// classAt returns the class of the character that holds code unit i.
func classAt(s []uint16, i int) charClass {
	r := rune(s[i])
	if utf16.IsSurrogate(r) {
		switch {
		case r < 0xDC00 && i+1 < len(s):
			r = utf16.DecodeRune(r, rune(s[i+1]))
		case r >= 0xDC00 && i > 0:
			r = utf16.DecodeRune(rune(s[i-1]), r)
		}
	}
	switch {
	case unicode.IsSpace(r):
		return classSpace
	case unicode.Is(unicode.Han, r):
		return classHan
	case unicode.Is(unicode.Hiragana, r):
		return classHiragana
	case unicode.Is(unicode.Katakana, r) || r == 'ー':
		return classKatakana
	case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
		return classWord
	}
	return classPunct
}

// IsCharStart reports whether a character starts at off: it is not the second
// half of a surrogate pair.
func IsCharStart(s []uint16, off int) bool {
	return off <= 0 || off >= len(s) || s[off] < 0xDC00 || s[off] > 0xDFFF
}
