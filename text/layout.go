package text

import (
	"sort"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// line is one line of a Layout, a column in vertical text. Positions along the
// line are inline, positions across it cross: the cross axis runs downwards in
// horizontal text and leftwards from the right edge in vertical text, so the
// first line always starts at cross 0.
type line struct {
	// clusterStart and clusterEnd delimit the clusters of the line.
	clusterStart, clusterEnd int
	// start and end are byte offsets. end excludes a hard line break.
	start, end int
	// x is the offset of the alignment, width and hang the jlreq results.
	x, width, hang float32
	// top and height place the line on the cross axis.
	top, height float32
	// rtl is set for a line of a right-to-left block, and bidi for one whose
	// clusters are not in logical order: their xs are then visual.
	rtl, bidi bool
	// hyphen is set for a line that ends inside a word.
	hyphen bool
	// ellipsis is set for the last line of text cut short by MaxLines. The
	// ellipsis is in the style of section ellipsisSection, at ellipsisAt from
	// x.
	ellipsis        bool
	ellipsisSection int
	ellipsisAt      float32
	// block is the block of the line, and spanStart and spanEnd delimit its
	// spans.
	block              int
	spanStart, spanEnd int
}

// Layout is a Paragraph broken into lines at one width. It does not change, and
// painting it does not allocate.
type Layout struct {
	paragraph *Paragraph
	size      geom.Size
	// extent is the length of the longest line.
	extent float32
	lines  []line
	// xs holds the inline position of each cluster from the start of its line,
	// and adv its inline extent there: its advance, or for a tab the distance
	// to the next stop.
	xs, adv []float32
	glyphs  []paint.Glyph
	// spans are the runs before their glyphs are sliced: those of the lines
	// in order, then those of the ruby, then those of the emphasis marks.
	// rubySpans holds where the spans of each ruby start, and then where the
	// spans of the marks start.
	spans     []span
	rubySpans []int
	runs      []paint.GlyphRun
	// backgrounds are the boxes of Span.Background, line by line.
	backgrounds []Background
	// maxLength and gen are what the layout was made for: the line length
	// and the generation of the paragraph, for Relayout.
	maxLength float32
	gen       uint64
	// truncated is set when MaxLines left text out.
	truncated bool
}

// Text returns the text of the paragraph.
func (p *Paragraph) Text() string { return p.text }

// Orientation returns the orientation of the paragraph.
func (p *Paragraph) Orientation() Orientation { return p.style.Orientation }

// Layout breaks the paragraph into lines of at most maxLength: the width for
// horizontal text, or the height of the columns for vertical text. An infinite
// length (geom.Inf) keeps every block on one line.
//
// The length of the layout, Size().W or Size().H in vertical text, is that of
// its longest line, and Style.Align places the lines within it. Text centered
// or set to the end (AlignCenter, AlignEnd), or running right to left, takes
// the whole of a finite maxLength instead, so that its lines are aligned
// within maxLength.
func (p *Paragraph) Layout(maxLength float32) *Layout {
	n := len(p.clusters)
	l := &Layout{paragraph: p, xs: make([]float32, n), adv: make([]float32, n), maxLength: maxLength, gen: p.gen}
	top := p.topRoom
	limit := p.style.MaxLines
	for i := range p.blocks {
		if limit > 0 && len(l.lines) > limit {
			break
		}
		top = l.layoutBlock(i, top)
	}
	if limit > 0 && len(l.lines) > limit {
		top = l.truncate(limit)
	}
	l.measure(top)
	l.align()
	l.buildRuns()
	return l
}

// MinLength returns the shortest maxLength at which Layout breaks lines only
// where breaking is allowed, never inside a word: the length of the longest
// piece that cannot be broken, such as a Western word, a number or a Japanese
// character with the punctuation that must stay with it. It is the
// min-content width of horizontal text. It does not allocate.
func (p *Paragraph) MinLength() float32 {
	opts := p.style.options()
	var longest float32
	for i := range p.blocks {
		b := &p.blocks[i]
		if b.clusterStart == b.clusterEnd {
			continue
		}
		longest = max(longest, p.shaper.breaker.MinWidth(p.units[b.clusterStart:b.clusterEnd], opts))
	}
	return longest
}

// layoutBlock breaks block i into lines that start at top, and returns where
// the next block starts.
func (l *Layout) layoutBlock(i int, top float32) float32 {
	p := l.paragraph
	b := &p.blocks[i]
	opts := p.style.options()
	if b.clusterStart == b.clusterEnd {
		base := p.style.Size * p.lineHeight
		l.lines = append(l.lines, line{
			clusterStart: b.clusterStart, clusterEnd: b.clusterStart,
			start: b.start, end: b.end, width: p.style.Indent, top: top, height: base, rtl: b.rtl, block: i,
		})
		return top + base
	}
	breaker := &p.shaper.breaker
	breaker.Break(p.units[b.clusterStart:b.clusterEnd], l.maxLength, opts)
	copy(l.xs[b.clusterStart:b.clusterEnd], breaker.X)
	copy(l.adv[b.clusterStart:b.clusterEnd], breaker.Advance)
	for k := range breaker.Lines {
		bl := &breaker.Lines[k]
		first, last := b.clusterStart+bl.Start, b.clusterStart+bl.End
		height := l.heightOf(first, last)
		l.lines = append(l.lines, line{
			clusterStart: first, clusterEnd: last,
			start: p.clusters[first].start, end: p.clusters[last-1].end,
			width: bl.Width, hang: bl.Hang, top: top, height: height, rtl: b.rtl, hyphen: bl.Hyphen, block: i,
		})
		if b.bidi || b.rtl {
			indent := opts.HangingIndent
			if k == 0 {
				indent = opts.Indent
			}
			l.reorderLine(&l.lines[len(l.lines)-1], indent)
		}
		top += height
	}
	return top
}

// measure sets the extent and the size from the lines, which end at top.
func (l *Layout) measure(top float32) {
	p := l.paragraph
	var longest float32
	rtl := false
	for i := range l.lines {
		longest = max(longest, l.lines[i].width)
		rtl = rtl || l.lines[i].rtl
	}
	l.extent = min(longest, max(l.maxLength, 0))
	align := p.style.Align
	if (rtl || align == AlignCenter || align == AlignEnd) && l.maxLength < geom.Inf {
		// A right-to-left paragraph starts at the right edge of the room it
		// has, not of its longest line, and centered text is centered in
		// that room.
		l.extent = max(l.maxLength, 0)
	}
	l.size = geom.Size{W: l.extent, H: top}
	if p.vertical {
		l.size = geom.Size{W: top, H: l.extent}
	}
}

// heightOf returns the height of a line: that of its largest text.
func (l *Layout) heightOf(first, last int) float32 {
	p := l.paragraph
	em := p.style.Size
	for i := first; i < last; i++ {
		em = max(em, p.units[i].Em)
	}
	return em * p.lineHeight
}

func (l *Layout) align() {
	for i := range l.lines {
		ln := &l.lines[i]
		toEnd := l.extent - ln.width
		switch l.paragraph.style.Align {
		case AlignCenter:
			ln.x = toEnd / 2
		case AlignEnd:
			if !ln.rtl {
				ln.x = toEnd
			}
		default:
			// The start of a right-to-left line is on the right.
			if ln.rtl {
				ln.x = toEnd
			}
		}
	}
}

// runKey is what the glyphs of a run have in common.
type runKey struct {
	face     paint.FaceID
	size     float32
	color    geom.Color
	sideways bool
}

// span collects the glyphs of a run while lines are visited. Slices of l.glyphs
// can only be taken once it has stopped growing.
type span struct {
	key        runKey
	start, end int
	origin     geom.Offset
	// lo and hi are the inline extent of the glyphs.
	lo, hi float32
}

// lineOrigin returns where the glyphs of a line are measured from: the left
// end of the baseline in horizontal text, the top of the center line of the
// column in vertical text.
func (l *Layout) lineOrigin(ln *line) geom.Offset {
	p := l.paragraph
	if p.vertical {
		return geom.Offset{X: l.size.W - ln.top - ln.height/2}
	}
	return geom.Offset{Y: ln.top + p.baselineIn(ln.height)}
}

// place returns the position of a glyph relative to the origin of its line.
func (l *Layout) place(inline, cross float32) (x, y float32) {
	if l.paragraph.vertical {
		return cross, inline
	}
	return inline, cross
}

// buildRuns turns every line into runs of one look each.
func (l *Layout) buildRuns() {
	p := l.paragraph
	total := len(p.glyphs) + len(l.lines) + len(p.marks)
	for i := range p.rubies {
		total += len(p.rubies[i].shaped.glyphs)
	}
	l.glyphs = make([]paint.Glyph, 0, total)
	l.spans = l.spans[:0]
	for i := range l.lines {
		l.lineSpans(&l.lines[i])
	}
	l.spans, l.rubySpans = l.addRubySpans(l.spans, 0, len(p.rubies), l.rubySpans[:0])
	l.rubySpans = append(l.rubySpans, len(l.spans))
	l.addMarkSpans()
	l.makeRuns()
	l.addBackgrounds()
}

// lineSpans appends the glyphs and spans of ln to l.glyphs and l.spans.
func (l *Layout) lineSpans(ln *line) {
	p := l.paragraph
	spans := l.spans
	ln.spanStart = len(spans)
	origin := l.lineOrigin(ln)
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		cl := &p.clusters[c]
		sec := &p.sections[cl.section]
		at := ln.x + l.xs[c]
		if cl.mixed {
			spans = l.pieceSpans(spans, c, at, at+l.adv[c], origin, c > ln.clusterStart)
			continue
		}
		start := len(l.glyphs)
		for g := cl.glyphStart; g < cl.glyphEnd; g++ {
			sg := &p.glyphs[g]
			x, y := l.place(at+sg.inline, sg.cross)
			l.glyphs = append(l.glyphs, paint.Glyph{ID: sg.id, X: x, Y: y})
		}
		end := at + l.adv[c]
		key := runKey{cl.face, cl.size, sec.color, cl.sideways}
		if n := len(spans); n > 0 && c > ln.clusterStart && spans[n-1].key == key {
			spans[n-1].end = len(l.glyphs)
			// Right to left, clusters come in from the right.
			spans[n-1].lo = min(spans[n-1].lo, at)
			spans[n-1].hi = max(spans[n-1].hi, end)
			continue
		}
		spans = append(spans, span{key, start, len(l.glyphs), origin, at, end})
	}
	if ln.hyphen {
		spans = l.addHyphen(ln, origin, spans)
	}
	if ln.ellipsis {
		spans = l.addEllipsis(ln, origin, spans)
	}
	ln.spanEnd = len(spans)
	l.spans = spans
}

// pieceSpans appends the glyphs and spans of the ruby group c, whose base is
// drawn in pieces of different looks, from at to end along the line. join
// lets the first piece continue the span before it.
func (l *Layout) pieceSpans(spans []span, c int, at, end float32, origin geom.Offset, join bool) []span {
	p := l.paragraph
	cl := &p.clusters[c]
	color := p.sections[cl.section].color
	r := sort.Search(len(p.rubies), func(i int) bool { return p.rubies[i].cluster >= c })
	pieces := p.rubies[r].pieces
	first := true
	for k := range pieces {
		pc := &pieces[k]
		from, to := cl.glyphStart+pc.glyph, cl.glyphEnd
		if k+1 < len(pieces) {
			to = cl.glyphStart + pieces[k+1].glyph
		}
		// A piece of characters without glyphs has nothing to draw, and no
		// glyph to measure from.
		if from >= to {
			continue
		}
		lo, hi := at+p.glyphs[from].inline, end
		if to < cl.glyphEnd {
			hi = at + p.glyphs[to].inline
		}
		lead := first
		if lead {
			lo, first = at, false
		}
		start := len(l.glyphs)
		for g := from; g < to; g++ {
			sg := &p.glyphs[g]
			x, y := l.place(at+sg.inline, sg.cross)
			l.glyphs = append(l.glyphs, paint.Glyph{ID: sg.id, X: x, Y: y})
		}
		key := runKey{pc.face, pc.size, color, pc.sideways}
		if n := len(spans); n > 0 && lead && join && spans[n-1].key == key {
			spans[n-1].end = len(l.glyphs)
			spans[n-1].lo = min(spans[n-1].lo, lo)
			spans[n-1].hi = max(spans[n-1].hi, hi)
			continue
		}
		spans = append(spans, span{key, start, len(l.glyphs), origin, lo, hi})
	}
	return spans
}

// makeRuns turns l.spans into l.runs.
func (l *Layout) makeRuns() {
	p := l.paragraph
	l.runs = l.runs[:0]
	for _, s := range l.spans {
		em := s.key.size
		// Glyphs can reach past their advance and the em box, so the bounds
		// keep room. They only decide what gets culled.
		bounds := geom.Rect{X: s.lo - em, Y: -2 * em, W: s.hi - s.lo + 2*em, H: 3 * em}
		if p.vertical {
			bounds = geom.Rect{X: -1.5 * em, Y: s.lo - em, W: 3 * em, H: s.hi - s.lo + 2*em}
		}
		l.runs = append(l.runs, paint.GlyphRun{
			Face:     s.key.face,
			Size:     em,
			Color:    s.key.color,
			Origin:   s.origin,
			Glyphs:   l.glyphs[s.start:s.end],
			Bounds:   bounds,
			Sideways: s.key.sideways,
		})
	}
}

// addHyphen draws the hyphen after the last cluster of ln.
func (l *Layout) addHyphen(ln *line, origin geom.Offset, spans []span) []span {
	p := l.paragraph
	c := ln.clusterEnd - 1
	cl := &p.clusters[c]
	h := p.sections[cl.section].hyphen
	if !h.ok {
		return spans
	}
	at := ln.x + l.xs[c] + l.adv[c]
	x, y := l.place(at, 0)
	start := len(l.glyphs)
	l.glyphs = append(l.glyphs, paint.Glyph{ID: h.id, X: x, Y: y})
	key := runKey{h.face, cl.size, p.sections[cl.section].color, false}
	return append(spans, span{key, start, len(l.glyphs), origin, at, at + h.advance})
}

// addRubySpans adds the glyphs of the ruby from index from to index to above
// its base, or right of it in vertical text. It appends where the spans of
// each ruby start to starts.
func (l *Layout) addRubySpans(spans []span, from, to int, starts []int) ([]span, []int) {
	p := l.paragraph
	for i := from; i < to; i++ {
		starts = append(starts, len(spans))
		r := &p.rubies[i]
		ln := l.lineOfCluster(r.cluster)
		if ln == nil {
			continue
		}
		base := ln.x + l.xs[r.cluster] + r.x
		if r.cluster == ln.clusterStart && r.x < 0 {
			// At the head of a line the ruby starts with its base, which has
			// taken the room of the overhang (jlreq.Unit.StartExtra).
			base -= r.x
		}
		sec := &p.sections[p.clusters[r.cluster].section]
		q := r.shaped
		origin := l.rubyOrigin(r, ln)
		var pen float32
		for c := range q.clusters {
			cl := &q.clusters[c]
			start := len(l.glyphs)
			at := base + pen
			for g := cl.glyphStart; g < cl.glyphEnd; g++ {
				sg := &q.glyphs[g]
				x, y := l.place(at+sg.inline, sg.cross)
				l.glyphs = append(l.glyphs, paint.Glyph{ID: sg.id, X: x, Y: y})
			}
			pen += q.units[c].Advance + r.gap
			key := runKey{cl.face, cl.size, sec.color, cl.sideways}
			if n := len(spans); n > 0 && c > 0 && spans[n-1].key == key {
				spans[n-1].end = len(l.glyphs)
				spans[n-1].hi = base + pen
				continue
			}
			spans = append(spans, span{key, start, len(l.glyphs), origin, at, base + pen})
		}
	}
	return spans, starts
}

// rubyOrigin returns the origin of the glyphs of r, whose base is on ln.
func (l *Layout) rubyOrigin(r *rubyRun, ln *line) geom.Offset {
	p := l.paragraph
	size := p.sections[p.clusters[r.cluster].section].style.Size
	origin := l.lineOrigin(ln)
	if p.vertical {
		origin.X += (size + r.shaped.style.Size) / 2
	} else {
		origin.Y -= rubyLift * size
		if p.shaper.slanted[p.clusters[r.cluster].face] {
			// Ruby over a synthetic oblique moves along the lean of its
			// base, so that it stays over the top of the slanted glyphs.
			origin.X += obliqueShear * rubyLift * size
		}
	}
	return origin
}

func (l *Layout) lineOfCluster(c int) *line {
	i := sort.Search(len(l.lines), func(i int) bool { return l.lines[i].clusterEnd > c })
	if i < len(l.lines) && c >= l.lines[i].clusterStart {
		return &l.lines[i]
	}
	return nil
}

// Size returns the extent of the text: the longest line, or the whole length
// of Layout for aligned text (see Paragraph.Layout), by the height of the
// lines. Hanging punctuation stays outside it.
func (l *Layout) Size() geom.Size { return l.size }

// Baseline returns the distance from the top of the layout to the baseline of
// its first line, which paint.Canvas.DrawText aligns to. Vertical text has no
// such baseline; there it returns 0, the top.
func (l *Layout) Baseline() float32 {
	p := l.paragraph
	if p.vertical {
		return 0
	}
	if len(l.lines) == 0 {
		return p.topRoom + p.baselineIn(p.style.Size*p.lineHeight)
	}
	ln := &l.lines[0]
	return ln.top + p.baselineIn(ln.height)
}

// Direction returns the resolved direction of the first block: DirectionLTR or
// DirectionRTL.
func (l *Layout) Direction() Direction {
	if len(l.lines) > 0 && l.lines[0].rtl {
		return DirectionRTL
	}
	return DirectionLTR
}

// Orientation returns the orientation of the text.
func (l *Layout) Orientation() Orientation { return l.paragraph.style.Orientation }

// Runs returns the glyphs to draw, with positions relative to the top left of
// the layout. The slice is owned by l.
func (l *Layout) Runs() []paint.GlyphRun { return l.runs }

// Truncated reports whether Style.MaxLines left part of the text out, so that
// the last line ends with an ellipsis. An app can then offer the whole text,
// in a tooltip for instance.
func (l *Layout) Truncated() bool { return l.truncated }

// LineCount returns the number of lines.
func (l *Layout) LineCount() int { return len(l.lines) }

// LineRange returns the byte offsets of line i of the text, without its hard
// line break.
func (l *Layout) LineRange(i int) (start, end int) { return l.lines[i].start, l.lines[i].end }

// toLine turns a position relative to the top left of the layout into inline
// and cross coordinates.
func (l *Layout) toLine(pos geom.Offset) (inline, cross float32) {
	if l.paragraph.vertical {
		return pos.Y, l.size.W - pos.X
	}
	return pos.X, pos.Y
}

// lineRect returns the rectangle that covers line i from inline lo to hi.
func (l *Layout) lineRect(i int, lo, hi float32) geom.Rect {
	ln := &l.lines[i]
	if l.paragraph.vertical {
		return geom.Rect{X: l.size.W - ln.top - ln.height, Y: lo, W: ln.height, H: hi - lo}
	}
	return geom.Rect{X: lo, Y: ln.top, W: hi - lo, H: ln.height}
}

// HitTest returns the byte offset of the caret position closest to pos, which
// is relative to the top left of the layout.
func (l *Layout) HitTest(pos geom.Offset) int {
	inline, cross := l.toLine(pos)
	i := 0
	for i+1 < len(l.lines) && cross >= l.lines[i+1].top {
		i++
	}
	return l.hitLine(i, inline)
}

// hitLine returns the caret position on line i closest to inline position x.
func (l *Layout) hitLine(i int, x float32) int {
	ln := &l.lines[i]
	p := l.paragraph
	if ln.bidi {
		return l.hitBidi(ln, x)
	}
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		left := ln.x + l.xs[c]
		if x < left+l.adv[c]/2 {
			return p.clusters[c].start
		}
	}
	if i+1 < len(l.lines) && l.lines[i+1].start == ln.end {
		// The seam of a soft wrap belongs to the next line, so the last
		// position on this one is before its last cluster.
		return l.PrevCaret(ln.end)
	}
	return ln.end
}

// CaretRect returns the caret at byte offset: a rectangle across the line,
// with no extent along it. It is zero wide in horizontal text and zero tall in
// vertical text. An offset inside a cluster moves to the start of the cluster.
func (l *Layout) CaretRect(offset int) geom.Rect {
	i := l.lineAt(offset)
	x := l.caretX(&l.lines[i], offset)
	return l.lineRect(i, x, x)
}

// CaretPosition returns the position of the caret at offset along its line: x
// in horizontal text, y in vertical text. It is what VerticalMove takes.
func (l *Layout) CaretPosition(offset int) float32 {
	return l.caretX(&l.lines[l.lineAt(offset)], offset)
}

// SelectionRects appends the rectangles that cover the bytes from start to end,
// one per line, to dst. Offsets inside a cluster move to its start.
func (l *Layout) SelectionRects(start, end int, dst []geom.Rect) []geom.Rect {
	if start >= end {
		return dst
	}
	first, last := l.lineAt(start), l.lineAt(end)
	for i := first; i <= last; i++ {
		ln := &l.lines[i]
		if ln.bidi {
			dst = l.bidiSelection(i, start, end, dst)
			continue
		}
		left := ln.x
		if i == first {
			left = l.caretX(ln, start)
		} else if ln.clusterStart < ln.clusterEnd {
			left = ln.x + l.xs[ln.clusterStart]
		}
		right := l.caretX(ln, ln.end)
		if i == last {
			right = l.caretX(ln, end)
		}
		if right > left {
			dst = append(dst, l.lineRect(i, left, right))
		}
	}
	return dst
}

// lineAt returns the line holding the caret at offset. At the seam between two
// lines of one block, the caret belongs to the start of the second.
func (l *Layout) lineAt(offset int) int {
	for i := range l.lines {
		if offset < l.lines[i].end || i == len(l.lines)-1 {
			return i
		}
		if offset == l.lines[i].end && (i+1 == len(l.lines) || l.lines[i+1].start > offset) {
			return i
		}
	}
	return len(l.lines) - 1
}

// caretX returns the inline position of the caret at offset on ln.
func (l *Layout) caretX(ln *line, offset int) float32 {
	p := l.paragraph
	for c := ln.clusterStart; c < ln.clusterEnd; c++ {
		if offset < p.clusters[c].end {
			if ln.bidi && l.isRTL(c) {
				// Right to left, the caret before a cluster is on its right.
				return ln.x + l.xs[c] + l.adv[c]
			}
			return ln.x + l.xs[c]
		}
	}
	if ln.bidi && ln.clusterStart < ln.clusterEnd && l.isRTL(ln.clusterEnd-1) {
		return ln.x + l.xs[ln.clusterEnd-1]
	}
	if ln.clusterStart == ln.clusterEnd {
		return ln.x + ln.width // an empty block: its indent
	}
	last := ln.clusterEnd - 1
	return ln.x + l.xs[last] + l.adv[last]
}

// reorderLine puts the clusters of ln in visual order (UAX #9 L1 and L2) and
// rewrites their xs. Each cluster keeps the room jlreq gave it, the space after
// it included. A right-to-left line has its indent on the right, which align
// leaves free.
func (l *Layout) reorderLine(ln *line, indent float32) {
	p := l.paragraph
	first, n := ln.clusterStart, ln.clusterEnd-ln.clusterStart
	levels := make([]uint8, n)
	var base uint8
	if ln.rtl {
		base = 1
	}
	for k := range n {
		levels[k] = p.clusters[first+k].level
	}
	// L1: spaces at the end of the line take the level of the paragraph.
	trailing := n
	for k := n - 1; k >= 0 && jlreq.Classify(p.units[first+k].Rune) == jlreq.ClassSpace; k-- {
		levels[k] = base
		trailing = k
	}
	order := make([]int, n)
	reorder(levels, order)
	room := make([]float32, n)
	for k := range n {
		if k+1 < n {
			room[k] = l.xs[first+k+1] - l.xs[first+k]
		} else {
			room[k] = l.adv[first+k]
		}
	}
	x := l.xs[first]
	if ln.rtl {
		x -= indent
		// The spaces at the end of a right-to-left line go to its left,
		// outside its width.
		for k := trailing; k < n; k++ {
			x -= room[k]
		}
	}
	for _, k := range order {
		l.xs[first+k] = x
		x += room[k]
	}
	ln.bidi = true
}

// isRTL reports whether cluster c runs right to left.
func (l *Layout) isRTL(c int) bool { return l.paragraph.clusters[c].level%2 == 1 }
