package text

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// ellipsisText is what ends text cut short by Style.MaxLines. Shaped in a
// vertical line it takes its vertical form, as the font's vert feature gives.
const ellipsisText = "…"

// truncate keeps the first limit lines and ends the last one with an
// ellipsis, if anything visible is left out. It returns where the last line
// ends. See docs/ADR/0125-text-truncation-decorative-stacks.md.
func (l *Layout) truncate(limit int) float32 {
	p := l.paragraph
	ln := &l.lines[limit-1]
	b := &p.blocks[ln.block]
	more := b.clusterEnd > ln.clusterEnd
	for i := limit; i < len(l.lines) && !more; i++ {
		more = l.lines[i].clusterEnd > l.lines[i].clusterStart
	}
	l.lines = l.lines[:limit]
	if !more {
		// Only empty lines are left out, which the eye cannot miss.
		return ln.top + ln.height
	}
	l.truncated = true

	// The last line takes the rest of its block, on one line, so that it is
	// filled up to the length and not only to its last break.
	first, end := ln.clusterStart, b.clusterEnd
	opts := p.style.options()
	opts.Justify = false
	if first > b.clusterStart {
		opts.Indent = opts.HangingIndent
	}
	if first < end {
		br := &p.shaper.breaker
		br.Break(p.units[first:end], geom.Inf, opts)
		copy(l.xs[first:end], br.X)
		copy(l.adv[first:end], br.Advance)
	}
	extent := func(cut int) float32 {
		if cut == first {
			return opts.Indent
		}
		return l.xs[cut-1] + l.adv[cut-1] + p.units[cut-1].EndExtra
	}

	// Clusters come off the logical end until the ellipsis fits. The ellipsis
	// takes the look of the cluster before it, which can change as the cut
	// moves back; the cut only ever moves back, so this ends.
	cut := end
	sec := l.sectionBefore(ln, cut)
	ell := p.ellipsisFor(sec)
	fits := true
	for {
		adv := ell.naturalWidth()
		for cut > first && extent(cut)+adv > l.maxLength {
			cut--
		}
		for cut > first && endsBadly(p.units[cut-1].Rune) {
			cut--
		}
		fits = extent(cut)+adv <= l.maxLength
		next := l.sectionBefore(ln, cut)
		if next == sec {
			break
		}
		sec, ell = next, p.ellipsisFor(next)
	}

	ln.clusterEnd = cut
	ln.end = ln.start
	if cut > first {
		ln.end = p.clusters[cut-1].end
	}
	ln.hyphen, ln.hang, ln.bidi = false, 0, false
	at := extent(cut)
	ln.width = at
	ln.height = l.heightOf(first, cut)
	if fits {
		// A box narrower than the ellipsis shows nothing rather than spill.
		adv := ell.naturalWidth()
		ln.ellipsis, ln.ellipsisSection, ln.ellipsisAt = true, sec, at
		ln.width += adv
		ln.height = max(ln.height, p.sections[sec].style.Size*p.lineHeight)
		if ln.rtl {
			// The logical end of a right-to-left line is on its left: the
			// clusters move right to make room.
			ln.ellipsisAt = 0
		}
		if (b.bidi || b.rtl) && cut > first {
			l.reorderLine(ln, opts.Indent)
		}
		if ln.rtl {
			for c := first; c < cut; c++ {
				l.xs[c] += adv
			}
		}
	} else if (b.bidi || b.rtl) && cut > first {
		l.reorderLine(ln, opts.Indent)
	}
	return ln.top + ln.height
}

// sectionBefore returns the section of the cluster before cut on ln, or the
// one where ln starts when no cluster is left.
func (l *Layout) sectionBefore(ln *line, cut int) int {
	p := l.paragraph
	if cut > ln.clusterStart {
		return p.clusters[cut-1].section
	}
	if ln.clusterStart < len(p.clusters) {
		return p.clusters[ln.clusterStart].section
	}
	for i := len(p.sections) - 1; i > 0; i-- {
		if p.sections[i].start <= ln.start {
			return i
		}
	}
	return 0
}

// endsBadly reports whether r should not stand before the ellipsis: a space,
// which would leave a gap, or an opening bracket, which must not end a line
// (JLReq 3.1.8).
func endsBadly(r rune) bool {
	c := jlreq.Classify(r)
	return c == jlreq.ClassSpace || c == jlreq.ClassOpening
}

// ellipsisFor returns the ellipsis in the style of section i, shaping it on
// first use.
func (p *Paragraph) ellipsisFor(i int) *Paragraph {
	sec := &p.sections[i]
	if sec.ellipsis == nil {
		st := sec.style
		st.Hyphenate, st.MaxLines, st.Indent, st.HangingIndent = false, 0, 0, 0
		sec.ellipsis = p.shaper.Shape(ellipsisText, st)
		p.shaper.selectFaces(&p.style)
	}
	return sec.ellipsis
}

// addEllipsis draws the ellipsis of ln.
func (l *Layout) addEllipsis(ln *line, origin geom.Offset, spans []span) []span {
	p := l.paragraph
	sec := &p.sections[ln.ellipsisSection]
	q := sec.ellipsis
	at := ln.x + ln.ellipsisAt
	for c := range q.clusters {
		cl := &q.clusters[c]
		start := len(l.glyphs)
		for g := cl.glyphStart; g < cl.glyphEnd; g++ {
			sg := &q.glyphs[g]
			x, y := l.place(at+sg.inline, sg.cross)
			l.glyphs = append(l.glyphs, paint.Glyph{ID: sg.id, X: x, Y: y})
		}
		next := at + q.units[c].Advance
		spans = append(spans, span{runKey{cl.face, cl.size, sec.color, cl.sideways}, start, len(l.glyphs), origin, at, next})
		at = next
	}
	return spans
}
