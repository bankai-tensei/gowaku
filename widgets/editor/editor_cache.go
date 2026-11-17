package editor

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// lineEntry is one line of a Editor laid out: its paragraph at the width
// of the editor, the decorations it was shaped with and its number.
type lineEntry struct {
	// src is the text the line was laid out from, with any composition, and
	// wrap the width. valid is cleared to lay the line out again anyway.
	src   string
	wrap  float32
	valid bool
	// decoGen is the Redecorate generation of decos, and styleGen that of
	// the style of the editor.
	decoGen, styleGen uint64
	para              *text.Paragraph
	layout            *text.Layout
	decos             []Decoration
	// line is what Editor.LineStyle returned for the line.
	line      LineStyle
	squiggles []squiggle
	// spans and paths are kept for their memory.
	spans []text.Span
	paths []*paint.Path
	// number is the line number that numbered shows, or 0.
	number    int
	numberGen uint64
	numbered  *text.Layout
}

// squiggle is the wavy line under a stretch of a line, relative to the top
// left of the line.
type squiggle struct {
	path  *paint.Path
	color geom.Color
}

// lineCache holds the entries of a stretch of lines around the view, so that
// the lines in view are shaped once and painting them allocates nothing.
type lineCache struct {
	// first is the line of entries[0].
	first   int
	entries []*lineEntry
	// spare holds entries for reuse, and scratch is the second buffer of
	// entries while the stretch moves.
	spare, scratch []*lineEntry
}

func (lc *lineCache) reset() {
	for _, e := range lc.entries {
		lc.recycle(e)
	}
	lc.entries = lc.entries[:0]
	lc.first = 0
}

func (lc *lineCache) recycle(e *lineEntry) {
	e.valid = false
	lc.spare = append(lc.spare, e)
}

func (lc *lineCache) take() *lineEntry {
	if n := len(lc.spare); n > 0 {
		e := lc.spare[n-1]
		lc.spare = lc.spare[:n-1]
		return e
	}
	return &lineEntry{}
}

// last returns the line after the last entry.
func (lc *lineCache) end() int { return lc.first + len(lc.entries) }

// get returns the entry of line i, or nil.
func (lc *lineCache) get(i int) *lineEntry {
	if i < lc.first || i >= lc.end() {
		return nil
	}
	return lc.entries[i-lc.first]
}

// cover makes the cache hold lines lo to hi, keeping the entries it has from
// keepLo to keepHi and recycling the others.
func (lc *lineCache) cover(lo, hi, keepLo, keepHi int) {
	newFirst := max(min(lc.first, lo), keepLo)
	newEnd := min(max(lc.end(), hi+1), keepHi+1)
	if len(lc.entries) == 0 {
		newFirst, newEnd = lo, hi+1
	}
	if newFirst == lc.first && newEnd == lc.end() {
		return
	}
	next := lc.scratch[:0]
	for i := newFirst; i < newEnd; i++ {
		if e := lc.get(i); e != nil {
			next = append(next, e)
		} else {
			next = append(next, nil)
		}
	}
	for k, e := range lc.entries {
		if i := lc.first + k; i < newFirst || i >= newEnd {
			lc.recycle(e)
		}
	}
	for k := range next {
		if next[k] == nil {
			next[k] = lc.take()
		}
	}
	lc.scratch, lc.entries = lc.entries[:0], next
	lc.first = newFirst
}

// maxSpliced is how many added lines splice makes entries for.
const maxSpliced = 1024

// splice follows an edit: removed lines from ch.first on gave way to added
// lines. The entries of the lines after it move with them, those of the lines
// it touched are laid out again.
func (lc *lineCache) splice(ch lineChange) {
	start := ch.first - lc.first
	switch {
	case len(lc.entries) == 0 || ch.first >= lc.end():
		return
	case ch.first+ch.removed <= lc.first:
		lc.first += ch.added - ch.removed
		return
	}
	if start < 0 || ch.first+ch.removed > lc.end() {
		// The edit reaches out of the stretch: begin again.
		lc.reset()
		return
	}
	if ch.added > maxSpliced {
		// So many lines, as a paste of a whole log or a source that found its
		// lines, would make an entry each: the stretch ends at the edit, and
		// cover makes the entries of the lines that come into view.
		for _, e := range lc.entries[start:] {
			lc.recycle(e)
		}
		lc.entries = lc.entries[:start]
		return
	}
	next := lc.scratch[:0]
	next = append(next, lc.entries[:start]...)
	for j := range ch.added {
		if j < ch.removed {
			e := lc.entries[start+j]
			e.valid = false
			next = append(next, e)
		} else {
			e := lc.take()
			e.valid = false
			next = append(next, e)
		}
	}
	for j := ch.added; j < ch.removed; j++ {
		lc.recycle(lc.entries[start+j])
	}
	next = append(next, lc.entries[start+ch.removed:]...)
	lc.scratch, lc.entries = lc.entries[:0], next
}

// invalidate makes the entries of lines first to last lay out again.
func (lc *lineCache) invalidate(first, last int) {
	for i := max(first, lc.first); i <= last && i < lc.end(); i++ {
		lc.entries[i-lc.first].valid = false
	}
}
