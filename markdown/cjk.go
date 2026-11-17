package markdown

import (
	"unicode/utf8"

	"golang.org/x/text/width"
)

// isWide reports whether r is East Asian wide or full width: kana, kanji,
// hangul and the full-width punctuation that goes with them.
func isWide(r rune) bool {
	if r < 0x1100 {
		return false
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return true
	}
	return false
}

// softBreak returns what a soft line break between before and after turns
// into: nothing between two wide characters, as Japanese and Chinese put no
// space between words, and a space otherwise. A break at either end of the
// text is nothing.
func softBreak(before, after string) string {
	if before == "" || after == "" {
		return ""
	}
	b, _ := utf8.DecodeLastRuneInString(before)
	a, _ := utf8.DecodeRuneInString(after)
	if isWide(b) && isWide(a) {
		return ""
	}
	return " "
}
