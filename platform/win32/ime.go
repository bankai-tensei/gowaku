package win32

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/input"
)

// The values of GCS_COMPATTR. They keep their header names (CODING_STYLE §8).
const (
	ATTR_INPUT               = 0
	ATTR_TARGET_CONVERTED    = 1
	ATTR_CONVERTED           = 2
	ATTR_TARGET_NOTCONVERTED = 3
	ATTR_INPUT_ERROR         = 4
	ATTR_FIXEDCONVERTED      = 5
)

// buildComposition turns what ImmGetCompositionStringW reports, in UTF-16 units,
// into an input.Composition in UTF-8 byte offsets. attrs has one entry per
// unit, and clauses holds the boundaries, starting with 0. This file has no OS
// calls, so that it can be tested alone.
func buildComposition(units []uint16, attrs []byte, clauses []uint32, cursor int) *input.Composition {
	offsets := utf8Offsets(units)
	text := string(utf16.Decode(units))
	c := &input.Composition{Text: text}
	if cursor >= 0 && cursor < len(offsets) {
		c.Cursor = offsets[cursor]
	} else {
		c.Cursor = len(text)
	}
	if len(clauses) < 2 {
		// No boundaries: the IME has not converted anything yet.
		if len(units) > 0 {
			c.Clauses = []input.Clause{{End: len(text), Kind: clauseKind(attrs, 0)}}
		}
		return c
	}
	c.Clauses = make([]input.Clause, 0, len(clauses)-1)
	for i := 0; i+1 < len(clauses); i++ {
		a, b := int(clauses[i]), int(clauses[i+1])
		if a >= b || b >= len(offsets) {
			continue
		}
		c.Clauses = append(c.Clauses, input.Clause{Start: offsets[a], End: offsets[b], Kind: clauseKind(attrs, a)})
	}
	return c
}

func clauseKind(attrs []byte, i int) input.ClauseKind {
	if i >= len(attrs) {
		return input.ClauseInput
	}
	switch attrs[i] {
	case ATTR_TARGET_CONVERTED, ATTR_TARGET_NOTCONVERTED:
		return input.ClauseTarget
	case ATTR_CONVERTED, ATTR_FIXEDCONVERTED:
		return input.ClauseConverted
	}
	return input.ClauseInput
}

// utf8Offsets returns the UTF-8 byte offset of every UTF-16 position, plus the
// end. The second half of a surrogate pair maps to the end of the rune.
func utf8Offsets(units []uint16) []int {
	offsets := make([]int, len(units)+1)
	at := 0
	for i := 0; i < len(units); i++ {
		offsets[i] = at
		r := rune(units[i])
		if utf16.IsSurrogate(r) && i+1 < len(units) {
			if joined := utf16.DecodeRune(r, rune(units[i+1])); joined != utf8.RuneError {
				at += utf8.RuneLen(joined)
				i++
				offsets[i] = at
				continue
			}
		}
		if n := utf8.RuneLen(r); n > 0 {
			at += n
		} else {
			at += utf8.RuneLen(utf8.RuneError)
		}
	}
	offsets[len(units)] = at
	return offsets
}
