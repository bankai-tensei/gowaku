package text

import (
	"sort"

	"github.com/bankai-tensei/gowaku/geom"
)

// caretStop is a position the caret can take on a line: an edge of a cluster.
type caretStop struct {
	x      float32
	offset int
}

// edges returns the offsets at the left and right edges of cluster c. A
// right-to-left cluster starts on its right.
func (l *Layout) edges(c int) (left, right int) {
	cl := &l.paragraph.clusters[c]
	if l.isRTL(c) {
		return cl.end, cl.start
	}
	return cl.start, cl.end
}

// hitBidi is hitLine for a line in visual order.
func (l *Layout) hitBidi(ln *line, x float32) int {
	best, bestDist := ln.end, float32(-1)
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		left := ln.x + l.xs[c]
		right := left + l.adv[c]
		leftOff, rightOff := l.edges(c)
		for _, s := range [2]caretStop{{left, leftOff}, {right, rightOff}} {
			d := geom.Abs32(s.x - x)
			if bestDist < 0 || d < bestDist {
				best, bestDist = s.offset, d
			}
		}
	}
	if best == ln.end {
		for i := range l.lines {
			if &l.lines[i] == ln && i+1 < len(l.lines) && l.lines[i+1].start == ln.end {
				// The seam of a soft wrap belongs to the next line.
				return l.PrevCaret(ln.end)
			}
		}
	}
	return best
}

// bidiSelection appends the rectangles of the bytes from start to end on line
// i, which is in visual order: one per stretch of selected clusters that touch
// on screen.
func (l *Layout) bidiSelection(i, start, end int, dst []geom.Rect) []geom.Rect {
	ln := &l.lines[i]
	p := l.paragraph
	var spans [][2]float32
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		cl := &p.clusters[c]
		if cl.end <= start || cl.start >= end {
			continue
		}
		left := ln.x + l.xs[c]
		spans = append(spans, [2]float32{left, left + l.adv[c]})
	}
	sort.Slice(spans, func(a, b int) bool { return spans[a][0] < spans[b][0] })
	for k := 0; k < len(spans); {
		lo, hi := spans[k][0], spans[k][1]
		k++
		for k < len(spans) && spans[k][0] <= hi+0.5 {
			hi = max(hi, spans[k][1])
			k++
		}
		dst = append(dst, l.lineRect(i, lo, hi))
	}
	return dst
}

// visualStop is where the caret of an offset is drawn on a line in visual
// order. Two offsets can share an x where a run of the other direction ends:
// the one whose cluster lies left of x, as a right-to-left cluster does of the
// caret before it, comes first on screen.
type visualStop struct {
	x float32
	// side is 0 if the cluster of the offset lies left of x, and 1 if right.
	side   uint8
	offset int
}

// before reports whether a comes left of b on screen.
func (a visualStop) before(b visualStop) bool {
	switch {
	case a.x < b.x-0.01:
		return true
	case a.x > b.x+0.01:
		return false
	case a.side != b.side:
		return a.side < b.side
	}
	return a.offset < b.offset
}

// visualStop returns where caretX draws the caret at offset on ln, with offset
// moved to the start of its cluster.
func (l *Layout) visualStop(ln *line, offset int) visualStop {
	p := l.paragraph
	if c := p.clusterAt(offset); c >= ln.clusterStart && c < ln.clusterEnd {
		if l.isRTL(c) {
			return visualStop{x: ln.x + l.xs[c] + l.adv[c], side: 0, offset: p.clusters[c].start}
		}
		return visualStop{x: ln.x + l.xs[c], side: 1, offset: p.clusters[c].start}
	}
	// After the last cluster, which lies right of the caret if it runs right
	// to left.
	s := visualStop{x: l.caretX(ln, offset), offset: offset}
	if ln.clusterStart < ln.clusterEnd && l.isRTL(ln.clusterEnd-1) {
		s.side = 1
	}
	return s
}

// VisualMove returns the caret position one step from offset on screen: to the
// right if dir is positive, to the left otherwise. Within left-to-right text it
// is NextCaret or PrevCaret. A walk in one direction passes every caret
// position of a line once, in the order CaretRect draws them. Off the end of a
// line it goes to the line after or before in reading order, and at that end of
// the text it returns offset.
func (l *Layout) VisualMove(offset, dir int) int {
	i := l.lineAt(offset)
	ln := &l.lines[i]
	if !ln.bidi {
		if dir > 0 {
			return l.NextCaret(offset)
		}
		return l.PrevCaret(offset)
	}
	p := l.paragraph
	// The seam of a soft wrap belongs to the next line.
	seam := i+1 < len(l.lines) && l.lines[i+1].start == ln.end
	cur := l.visualStop(ln, offset)
	found := false
	var best visualStop
	for c := ln.clusterStart; c <= ln.clusterEnd; c++ {
		at := ln.end
		if c < ln.clusterEnd {
			at = p.clusters[c].start
		} else if seam {
			break
		}
		s := l.visualStop(ln, at)
		if dir > 0 && cur.before(s) && (!found || s.before(best)) ||
			dir <= 0 && s.before(cur) && (!found || best.before(s)) {
			best, found = s, true
		}
	}
	if found {
		return best.offset
	}
	// Off the edge of the line: right is forward unless the line runs right
	// to left. The edge on screen need not be the end of the line in the text,
	// so the step is taken from the line and not from offset.
	if (dir > 0) != ln.rtl {
		if i+1 == len(l.lines) {
			return offset
		}
		return l.lines[i+1].start
	}
	if i == 0 {
		return offset
	}
	return l.PrevCaret(ln.start)
}
