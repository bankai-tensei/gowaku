package editor

import (
	"sort"
	"strings"
)

// replaceLines puts with[k], or s when with is nil, in place of each of
// matches, which are in order and each in one line, as ReplaceAll does. It
// goes once over the lines from the first match to the last, making each
// edited line anew, and changes the text in one step, which the search does
// not follow; replacing the matches one at a time cost a move of the lines or
// pieces after each, many seconds for 100,000 matches. It returns the text
// from the start of the first match to the end of the last, before and after,
// and where each replacement ends in the new text.
func (c *Controller) replaceLines(matches []TextRange, with []string, s string) (before, after string, ends []TextPosition) {
	s = normalizeBreaks(s)
	n := len(matches)
	firstLine, lastLine := matches[0].Start.Line, matches[n-1].Start.Line
	size := c.buf.offset(matches[n-1].End) - c.buf.offset(matches[0].Start) + matches[0].Start.Column
	var old, neu strings.Builder
	old.Grow(size)
	neu.Grow(size)
	ends = make([]TextPosition, n)
	// spans are where each edited line lies in neu, to be cut into lines once
	// it is a string.
	type span struct{ line, from, to int }
	var spans []span
	shift := 0
	lastLen := 0
	k := 0
	for i := firstLine; i <= lastLine; i++ {
		l := c.buf.line(i)
		lastLen = len(l)
		if i > firstLine {
			old.WriteByte('\n')
			neu.WriteByte('\n')
		}
		old.WriteString(l)
		if k == n || matches[k].Start.Line != i {
			neu.WriteString(l)
			continue
		}
		from := neu.Len()
		row, rowStart := i+shift, from
		at := 0
		for ; k < n && matches[k].Start.Line == i; k++ {
			m := matches[k]
			neu.WriteString(l[at:m.Start.Column])
			r := s
			if with != nil {
				r = normalizeBreaks(with[k])
			}
			if nl := strings.Count(r, "\n"); nl > 0 {
				row += nl
				rowStart = neu.Len() + strings.LastIndexByte(r, '\n') + 1
			}
			neu.WriteString(r)
			ends[k] = TextPosition{Line: row, Column: neu.Len() - rowStart}
			at = m.End.Column
		}
		neu.WriteString(l[at:])
		spans = append(spans, span{line: i, from: from, to: neu.Len()})
		shift = row - i
	}
	whole, wholeNew := old.String(), neu.String()
	head, tail := matches[0].Start.Column, lastLen-matches[n-1].End.Column
	before = whole[head : len(whole)-tail]
	after = wholeNew[head : len(wholeNew)-tail]
	edits := make([]lineEdit, len(spans))
	for e, sp := range spans {
		// The lines are cut out of wholeNew, which the history holds anyway.
		edits[e] = lineEdit{line: sp.line, lines: strings.Split(wholeNew[sp.from:sp.to], "\n")}
	}
	c.buf.rewrite(edits)
	c.current = -1
	// The views learn of each edited line, from the last, as they would of
	// each replace.
	chs := make([]lineChange, len(edits))
	for e, ed := range edits {
		chs[len(edits)-1-e] = lineChange{first: ed.line, removed: 1, added: len(ed.lines)}
	}
	for _, v := range c.views {
		if v != c.primary {
			v.sel.anchor = moveReplaced(v.sel.anchor, matches, ends)
			v.sel.caret = moveReplaced(v.sel.caret, matches, ends)
		}
		if v.r != nil {
			v.r.linesRewritten(chs)
		}
	}
	return before, after, ends
}

// moveReplaced moves p through the replace of each of matches, which ended
// at ends, as shiftEdited moves it through each: a position inside a match
// goes to the end of its replacement, one after it moves with its text.
func moveReplaced(p TextPosition, matches []TextRange, ends []TextPosition) TextPosition {
	k := sort.Search(len(matches), func(k int) bool { return !matches[k].Start.Before(p) }) - 1
	switch {
	case k < 0:
		return p
	case p.Before(matches[k].End):
		return ends[k]
	}
	return shiftPos(p, matches[k].End, ends[k])
}
