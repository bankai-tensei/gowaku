package cocoa

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/input"
)

// markedRun is a run of the marked text of setMarkedText, in UTF-16 code units
// as NSString counts: a clause when the attributes carry
// NSMarkedClauseSegment, and thick when its NSUnderline is thick, which is how
// the input methods of macOS mark the clause being converted.
type markedRun struct {
	start, end int
	thick      bool
}

// byteOffset returns the byte offset in s of the UTF-16 offset u. An offset
// inside a surrogate pair, or past the end, rounds up to the next rune.
func byteOffset(s string, u int) int {
	units := 0
	for i, r := range s {
		if units >= u {
			return i
		}
		units += utf16.RuneLen(r)
	}
	return len(s)
}

// utf16Len returns how many UTF-16 code units s has.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// utf16Offset returns the UTF-16 offset of the byte offset b of s.
func utf16Offset(s string, b int) int {
	n := 0
	for i, r := range s {
		if i >= b {
			break
		}
		n += utf16.RuneLen(r)
	}
	return n
}

// composition converts marked text, its runs and the end of its selection to
// what gowaku shows. With a thick run, the IME is converting: the thick run is
// the target and the others are converted. Without, everything is still input.
func composition(text string, runs []markedRun, caret int) *input.Composition {
	c := &input.Composition{Text: text, Cursor: byteOffset(text, caret)}
	converting := false
	for _, r := range runs {
		if r.thick {
			converting = true
		}
	}
	for _, r := range runs {
		start, end := byteOffset(text, r.start), byteOffset(text, r.end)
		if start >= end {
			continue
		}
		kind := input.ClauseInput
		if converting {
			kind = input.ClauseConverted
			if r.thick {
				kind = input.ClauseTarget
			}
		}
		// Runs of the same kind next to each other are one clause when the IME
		// does not segment, as attributes split runs for other reasons too.
		if n := len(c.Clauses); n > 0 && c.Clauses[n-1].End == start && c.Clauses[n-1].Kind == kind && kind == input.ClauseInput {
			c.Clauses[n-1].End = end
			continue
		}
		c.Clauses = append(c.Clauses, input.Clause{Start: start, End: end, Kind: kind})
	}
	if len(c.Clauses) == 0 && text != "" {
		c.Clauses = []input.Clause{{Start: 0, End: len(text), Kind: input.ClauseInput}}
	}
	return c
}

// validUTF8 replaces what is not UTF-8, which NSString never hands over but
// a lone surrogate would make.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return string([]rune(s))
}
