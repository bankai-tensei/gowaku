package win32

import (
	"unicode/utf16"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file moves the ranges of the UIA Text pattern over the text of a field,
// without calling the OS (ADR 0042 §4: whole text, selection, caret, and
// movement by character, word and line). Offsets are UTF-16 code units.

// fieldText is the text of a field as UI Automation sees it.
type fieldText struct {
	s    []uint16
	info *semantics.TextInfo
}

func newFieldText(ti *semantics.TextInfo) fieldText {
	return fieldText{s: semantics.UTF16(ti.Text), info: ti}
}

// normalizeUnit returns the unit that stands in for u. There is one format for the
// whole text, so Format is the document, as is Page.
func normalizeUnit(u int32) int32 {
	switch u {
	case TextUnit_Character, TextUnit_Word, TextUnit_Line, TextUnit_Paragraph:
		return u
	}
	return TextUnit_Document
}

// boundary reports whether a unit starts at off. Both ends of the text are
// boundaries of every unit.
func (t fieldText) boundary(unit int32, off int) bool {
	n := len(t.s)
	if off <= 0 || off >= n {
		return true
	}
	switch normalizeUnit(unit) {
	case TextUnit_Character:
		return semantics.IsCharStart(t.s, off)
	case TextUnit_Word:
		return semantics.IsWordStart(t.s, off)
	case TextUnit_Line:
		for _, start := range t.info.LineStarts {
			if start == off {
				return true
			}
		}
		return false
	case TextUnit_Paragraph:
		return t.s[off-1] == '\n'
	}
	return false
}

// next returns the first boundary after off, or the end.
func (t fieldText) next(unit int32, off int) int {
	n := len(t.s)
	for p := off + 1; p < n; p++ {
		if t.boundary(unit, p) {
			return p
		}
	}
	return n
}

// prev returns the last boundary before off, or the start.
func (t fieldText) prev(unit int32, off int) int {
	for p := min(off, len(t.s)) - 1; p > 0; p-- {
		if t.boundary(unit, p) {
			return p
		}
	}
	return 0
}

func (t fieldText) clamp(start, end int) (int, int) {
	n := len(t.s)
	start, end = min(max(start, 0), n), min(max(end, 0), n)
	if start > end {
		start = end
	}
	return start, end
}

// expand is ExpandToEnclosingUnit: the unit that holds start. A range at the
// end of the text takes the last unit.
func (t fieldText) expand(unit int32, start int) (int, int) {
	n := len(t.s)
	if n == 0 {
		return 0, 0
	}
	if start >= n {
		start = t.prev(unit, n)
	}
	if !t.boundary(unit, start) {
		start = t.prev(unit, start)
	}
	return start, t.next(unit, start)
}

// move is Move. An empty range moves as a caret, by count boundaries. Any
// other range collapses to the start of its first unit, moves by count
// starts of units, and grows to span the unit it lands on; the end of the
// text is not the start of a unit then.
func (t fieldText) move(unit int32, count, start, end int) (newStart, newEnd, moved int) {
	if count == 0 {
		return start, end, 0
	}
	n := len(t.s)
	if start == end {
		pos := start
		for moved < count {
			p := t.next(unit, pos)
			if p == pos {
				break
			}
			pos = p
			moved++
		}
		for moved > count {
			p := t.prev(unit, pos)
			if p == pos {
				break
			}
			pos = p
			moved--
		}
		return pos, pos, moved
	}
	pos := start
	if !t.boundary(unit, pos) {
		pos = t.prev(unit, pos)
	}
	for moved < count {
		p := t.next(unit, pos)
		if p >= n {
			break
		}
		pos = p
		moved++
	}
	for moved > count && pos > 0 {
		pos = t.prev(unit, pos)
		moved--
	}
	return pos, t.next(unit, pos), moved
}

// moveEndpoint is MoveEndpointByUnit. When the endpoint passes the other one,
// the range collapses where it stopped.
func (t fieldText) moveEndpoint(endpoint, unit int32, count, start, end int) (newStart, newEnd, moved int) {
	pos := start
	if endpoint == TextPatternRangeEndpoint_End {
		pos = end
	}
	for moved < count {
		p := t.next(unit, pos)
		if p == pos {
			break
		}
		pos = p
		moved++
	}
	for moved > count {
		p := t.prev(unit, pos)
		if p == pos {
			break
		}
		pos = p
		moved--
	}
	newStart, newEnd = setEndpoint(endpoint, pos, start, end)
	return newStart, newEnd, moved
}

// setEndpoint moves one end of a range to pos, collapsing the range if it
// passes the other end.
func setEndpoint(endpoint int32, pos, start, end int) (newStart, newEnd int) {
	if endpoint == TextPatternRangeEndpoint_End {
		end = pos
		if start > end {
			start = end
		}
	} else {
		start = pos
		if end < start {
			end = start
		}
	}
	return start, end
}

// text returns the range as a string of at most maxLength code units, or all
// of it for a negative maxLength. A character is not cut in half.
func (t fieldText) text(start, end, maxLength int) string {
	start, end = t.clamp(start, end)
	if maxLength >= 0 && end-start > maxLength {
		end = start + maxLength
		if end > start && !semantics.IsCharStart(t.s, end) {
			end--
		}
	}
	return string(utf16.Decode(t.s[start:end]))
}

// lineRects returns the boxes of the lines that the range touches, as far as
// the field knows them. An empty range has none: the field does not tell
// where in a line an offset is.
func (t fieldText) lineRects(start, end int) []geom.Rect {
	start, end = t.clamp(start, end)
	if start == end {
		return nil
	}
	var rects []geom.Rect
	for l := t.info.Line(start); l <= t.info.Line(end-1); l++ {
		if l < len(t.info.LineRects) {
			rects = append(rects, t.info.LineRects[l])
		}
	}
	return rects
}

// lineAt returns the start of the line whose box holds the height y, or of
// the nearest one, in logical pixels.
func (t fieldText) lineAt(y float32) int {
	best, bestDist := 0, float32(-1)
	for l, r := range t.info.LineRects {
		var d float32
		switch {
		case y < r.Y:
			d = r.Y - y
		case y >= r.Y+r.H:
			d = y - (r.Y + r.H)
		}
		if bestDist < 0 || d < bestDist {
			best, bestDist = l, d
		}
	}
	start, _ := t.info.LineBounds(best, len(t.s))
	return start
}
