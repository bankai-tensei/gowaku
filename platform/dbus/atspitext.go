package dbus

import (
	"slices"
	"unicode"
	"unicode/utf16"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file answers the Text interface of AT-SPI over the text of a field,
// without calling the bus (ADR 0043 §4). AT-SPI counts offsets in characters,
// that is code points, while semantics.TextInfo counts UTF-16 code units: the
// conversion happens here and nowhere else.

// textUnit is what the text is divided into.
type textUnit uint8

const (
	unitChar textUnit = iota
	// unitWord divides at the start of each word, so that a piece is a word
	// with the spaces after it.
	unitWord
	// unitWordEnd divides after the last letter of each word, so that a piece
	// is the spaces before a word with the word.
	unitWordEnd
	unitLine
	// unitLineEnd divides before the line break that ends each line.
	unitLineEnd
	unitParagraph
)

// atspiText is the text of a field as AT-SPI sees it.
type atspiText struct {
	runes []rune
	u16   []uint16
	// starts holds the UTF-16 offset at which each character starts, and the
	// length of the text after the last.
	starts []int
	info   *semantics.TextInfo
}

func newAtspiText(ti *semantics.TextInfo) atspiText {
	t := atspiText{runes: []rune(ti.Text), info: ti}
	t.u16 = utf16.Encode(t.runes)
	t.starts = make([]int, len(t.runes)+1)
	off := 0
	for i, r := range t.runes {
		t.starts[i] = off
		// A rune from a string is always valid: bad bytes became U+FFFD.
		off += utf16.RuneLen(r)
	}
	t.starts[len(t.runes)] = off
	return t
}

// count returns the number of characters.
func (t atspiText) count() int { return len(t.runes) }

// clamp keeps a character offset inside the text.
func (t atspiText) clamp(off int) int { return min(max(off, 0), len(t.runes)) }

// toRune turns a UTF-16 offset into a character offset. An offset inside a
// surrogate pair counts as the character it is in.
func (t atspiText) toRune(off int) int {
	i, found := slices.BinarySearch(t.starts, off)
	if !found && i > 0 {
		i--
	}
	return t.clamp(i)
}

// toU16 turns a character offset into a UTF-16 offset.
func (t atspiText) toU16(off int) int { return t.starts[t.clamp(off)] }

// slice returns the characters from start to end. An end of -1, or past the
// text, is its end.
func (t atspiText) slice(start, end int) string {
	if end < 0 || end > len(t.runes) {
		end = len(t.runes)
	}
	start = t.clamp(start)
	if start >= end {
		return ""
	}
	return string(t.runes[start:end])
}

// caret returns the offset of the caret: the moving end of the selection.
func (t atspiText) caret() int { return t.toRune(t.info.SelectionEnd) }

// selection returns the selection in order, and false if there is none, only
// a caret.
func (t atspiText) selection() (start, end int, ok bool) {
	a, b := t.toRune(t.info.SelectionStart), t.toRune(t.info.SelectionEnd)
	if a == b {
		return a, a, false
	}
	return min(a, b), max(a, b), true
}

func isSpace(r rune) bool { return unicode.IsSpace(r) }

// bounds returns the offsets where unit divides the text, in order, with both
// ends.
func (t atspiText) bounds(unit textUnit) []int {
	n := len(t.runes)
	out := []int{0}
	add := func(p int) {
		if p > out[len(out)-1] && p < n {
			out = append(out, p)
		}
	}
	switch unit {
	case unitChar:
		for p := 1; p < n; p++ {
			add(p)
		}
	case unitWord:
		for p := 1; p < n; p++ {
			if semantics.IsWordStart(t.u16, t.starts[p]) {
				add(p)
			}
		}
	case unitWordEnd:
		words := t.bounds(unitWord)
		for i := 0; i+1 < len(words); i++ {
			e := words[i+1]
			for e > words[i] && isSpace(t.runes[e-1]) {
				e--
			}
			if e > words[i] {
				add(e)
			}
		}
	case unitLine, unitLineEnd:
		for _, s := range t.info.LineStarts {
			p := t.toRune(s)
			if unit == unitLineEnd && p > 0 && t.runes[p-1] == '\n' {
				p--
			}
			add(p)
		}
	case unitParagraph:
		for p := 1; p < n; p++ {
			if t.runes[p-1] == '\n' {
				add(p)
			}
		}
	}
	if n > 0 {
		out = append(out, n)
	}
	return out
}

// pieceAt returns the piece of bounds that holds off. The end of the text
// belongs to the last piece.
func pieceAt(bounds []int, off int) (start, end int) {
	if len(bounds) < 2 {
		return 0, 0
	}
	i, found := slices.BinarySearch(bounds, off)
	if !found {
		i--
	}
	if i >= len(bounds)-1 {
		i = len(bounds) - 2
	}
	i = max(i, 0)
	return bounds[i], bounds[i+1]
}

// unitOfGranularity returns the unit of an AtspiTextGranularity. Sentences
// are lines: gowaku does not find sentences.
func unitOfGranularity(g uint32) (textUnit, bool) {
	switch g {
	case atspiGranularityChar:
		return unitChar, true
	case atspiGranularityWord:
		return unitWord, true
	case atspiGranularitySentence, atspiGranularityLine:
		return unitLine, true
	case atspiGranularityParagraph:
		return unitParagraph, true
	}
	return 0, false
}

// unitOfBoundary returns the unit of an AtspiTextBoundaryType. Sentences are
// lines.
func unitOfBoundary(b uint32) (textUnit, bool) {
	switch b {
	case atspiBoundaryChar:
		return unitChar, true
	case atspiBoundaryWordStart:
		return unitWord, true
	case atspiBoundaryWordEnd:
		return unitWordEnd, true
	case atspiBoundarySentenceStart, atspiBoundaryLineStart:
		return unitLine, true
	case atspiBoundarySentenceEnd, atspiBoundaryLineEnd:
		return unitLineEnd, true
	}
	return 0, false
}

// stringAt answers GetStringAtOffset. A word is without the spaces after it:
// in them, it is the word before.
func (t atspiText) stringAt(off int, g uint32) (s string, start, end int) {
	unit, ok := unitOfGranularity(g)
	if !ok || off < 0 || off > len(t.runes) {
		return "", 0, 0
	}
	start, end = pieceAt(t.bounds(unit), off)
	if unit == unitWord {
		e := end
		for e > start && isSpace(t.runes[e-1]) {
			e--
		}
		if e > start {
			end = e
		}
	}
	return t.slice(start, end), start, end
}

// Which piece textAround returns, relative to the one at the offset.
const (
	pieceBefore = -1
	pieceAtOff  = 0
	pieceAfter  = 1
)

// textAround answers GetTextBeforeOffset, GetTextAtOffset and
// GetTextAfterOffset, the older interface of boundary types.
func (t atspiText) textAround(off int, boundary uint32, which int) (s string, start, end int) {
	unit, ok := unitOfBoundary(boundary)
	n := len(t.runes)
	if !ok || off < 0 || off > n {
		return "", 0, 0
	}
	b := t.bounds(unit)
	start, end = pieceAt(b, off)
	switch which {
	case pieceBefore:
		if start == 0 {
			return "", 0, 0
		}
		start, end = pieceAt(b, start-1)
	case pieceAfter:
		if end >= n {
			return "", n, n
		}
		start, end = pieceAt(b, end)
	}
	return t.slice(start, end), start, end
}

// line returns the line that holds character off.
func (t atspiText) line(off int) int { return t.info.Line(t.toU16(off)) }

// lineRect returns the box of line i, in logical pixels of the window.
func (t atspiText) lineRect(i int) (geom.Rect, bool) {
	if i < 0 || i >= len(t.info.LineRects) {
		return geom.Rect{}, false
	}
	return t.info.LineRects[i], true
}

// rangeRect returns the box of the lines from start to end, characters.
func (t atspiText) rangeRect(start, end int) (geom.Rect, bool) {
	start, end = t.clamp(start), t.clamp(end)
	if end < start {
		start, end = end, start
	}
	var r geom.Rect
	found := false
	for i := t.line(start); i <= t.line(max(end-1, start)); i++ {
		if lr, ok := t.lineRect(i); ok {
			r = union(r, lr)
			found = true
		}
	}
	return r, found
}

// offsetAtPoint returns the start of the line under p, in logical pixels of
// the window, or -1.
func (t atspiText) offsetAtPoint(p geom.Offset) int {
	for i, r := range t.info.LineRects {
		if p.Y >= r.Y && p.Y < r.Y+r.H && p.X >= r.X && p.X < r.X+r.W {
			start, _ := t.info.LineBounds(i, len(t.u16))
			return t.toRune(start)
		}
	}
	return -1
}

// textChange returns how before became after, as one replacement found by
// the common start and end: removed at start, then inserted there, in
// characters.
func textChange(before, after string) (start int, removed, inserted []rune) {
	a, b := []rune(before), []rune(after)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	return p, a[p : len(a)-s], b[p : len(b)-s]
}
