package text

import (
	"slices"
	"sort"
	"strings"
)

// edit is what the last Reshape changed: the blocks from first to oldLast
// became the blocks from first to newLast. Blocks after them moved by
// clusters clusters and bytes bytes.
type edit struct {
	valid                bool
	first                int
	oldLast, newLast     int
	clusters, bytes      int
	oldCount, newCount   int // clusters of the changed blocks
	oldGlyphs, newGlyphs int
	// rubyFirst is the first ruby of the changed blocks, which had oldRubies
	// and have newRubies.
	rubyFirst, oldRubies, newRubies int
}

// Reshape is Shape for content that differs from the text of old in one
// place, as it does after a keystroke. It shapes only the blocks that changed
// and keeps the rest, so that the cost follows the size of the edit rather than
// of the text. It updates and returns old, which must not be used otherwise
// again, nor any Layout of it except as the old of Relayout.
//
// A paragraph in another style is shaped anew.
func (s *Shaper) Reshape(old *Paragraph, content string, st Style) *Paragraph {
	return s.ReshapeSpans(old, []Span{{Text: content}}, st)
}

// ReshapeSpans is Reshape for text of several spans, with ruby: ShapeSpans for
// spans that differ from those old was made of in one place. Spans before and
// after the edit keep their glyphs.
func (s *Shaper) ReshapeSpans(old *Paragraph, spans []Span, st Style) *Paragraph {
	if old == nil || old.shaper != s || st.resolved() != old.style || len(old.blocks) == 0 {
		return s.ShapeSpans(spans, st)
	}
	content := joinSpans(spans)
	secs := old.makeSections(spans, old.spare[:0])
	// The sections that start alike and those that end alike, counted from
	// the end, keep their clusters. The text outside them may not count as
	// unchanged.
	olds := old.sections
	front := 0
	for front < len(olds) && front < len(secs) && olds[front].start == secs[front].start && sameLook(&olds[front], &secs[front]) {
		secs[front].hyphen, secs[front].ellipsis, secs[front].mark = olds[front].hyphen, olds[front].ellipsis, olds[front].mark
		front++
	}
	back := 0
	for {
		oi, ni := len(olds)-1-back, len(secs)-1-back
		if oi < 0 || ni < 0 || len(old.text)-olds[oi].end != len(content)-secs[ni].end || !sameLook(&olds[oi], &secs[ni]) {
			break
		}
		secs[ni].hyphen, secs[ni].ellipsis, secs[ni].mark = olds[oi].hyphen, olds[oi].ellipsis, olds[oi].mark
		back++
	}
	pre := commonPrefix(old.text, content)
	if front < len(olds) {
		pre = min(pre, olds[front].start)
	}
	if front < len(secs) {
		pre = min(pre, secs[front].start)
	}
	suf := commonSuffix(old.text[pre:], content[pre:])
	if oi := len(olds) - 1 - back; oi >= 0 {
		suf = min(suf, len(old.text)-olds[oi].end)
	}
	if ni := len(secs) - 1 - back; ni >= 0 {
		suf = min(suf, len(content)-secs[ni].end)
	}
	delta := len(content) - len(old.text)
	blocks := old.blocks
	// The changed blocks: from the one holding the first changed byte to the
	// one holding the last. A block owns the line feed after it.
	first := sort.Search(len(blocks), func(i int) bool { return blocks[i].end >= pre })
	last := sort.Search(len(blocks), func(i int) bool { return blocks[i].end >= len(old.text)-suf })
	if first >= len(blocks) || last >= len(blocks) {
		return s.ShapeSpans(spans, st)
	}
	start, end := blocks[first].start, blocks[last].end+delta
	if end < start || end > len(content) || (end < len(content) && content[end] != '\n' && content[end] != '\r') {
		return s.ShapeSpans(spans, st)
	}

	// Shape the changed lines into a paragraph of their own, with the offsets
	// and sections of the new text, and splice it in.
	mid := &Paragraph{
		shaper: s, text: content, style: old.style, lineHeight: old.lineHeight,
		ascent: old.ascent, descent: old.descent, vertical: old.vertical, emCenter: old.emCenter,
		sections: secs,
	}
	s.selectFaces(&mid.style)
	mid.addLines(start, end)
	mid.fitRubies(0)

	cs, ce := blocks[first].clusterStart, blocks[last].clusterEnd
	rubies := old.rubies
	ra := sort.Search(len(rubies), func(i int) bool { return rubies[i].cluster >= cs })
	rb := sort.Search(len(rubies), func(i int) bool { return rubies[i].cluster >= ce })
	marks := old.marks
	ma := sort.Search(len(marks), func(i int) bool { return marks[i].cluster >= cs })
	mb := sort.Search(len(marks), func(i int) bool { return marks[i].cluster >= ce })
	annotated := len(rubies)-(rb-ra)+len(mid.rubies) > 0 || len(marks)-(mb-ma)+len(mid.marks) > 0
	stacked := old.stacked(marks[:ma]) || old.stacked(marks[mb:]) || mid.stacked(mid.marks)
	if lh, top := old.roomFor(annotated, stacked); lh != old.lineHeight || top != old.topRoom {
		// The first ruby or mark came or the last went, or marks came to
		// stand outside ruby or no longer do, and the lines change height.
		return s.ShapeSpans(spans, st)
	}
	gs, ge := old.glyphAt(cs), old.glyphAt(ce)
	dc, dg := len(mid.clusters)-(ce-cs), len(mid.glyphs)-(ge-gs)
	dsec := len(secs) - len(olds)
	for k := range mid.clusters {
		mid.clusters[k].glyphStart += gs
		mid.clusters[k].glyphEnd += gs
	}
	for k := range mid.blocks {
		mid.blocks[k].clusterStart += cs
		mid.blocks[k].clusterEnd += cs
	}
	for k := range mid.rubies {
		mid.rubies[k].cluster += cs
	}
	for k := range mid.marks {
		mid.marks[k].cluster += cs
	}
	old.clusters = slices.Replace(old.clusters, cs, ce, mid.clusters...)
	old.units = slices.Replace(old.units, cs, ce, mid.units...)
	old.glyphs = slices.Replace(old.glyphs, gs, ge, mid.glyphs...)
	for k := cs + len(mid.clusters); k < len(old.clusters); k++ {
		c := &old.clusters[k]
		c.start += delta
		c.end += delta
		c.glyphStart += dg
		c.glyphEnd += dg
		c.section += dsec
	}
	old.rubies = slices.Replace(old.rubies, ra, rb, mid.rubies...)
	for k := ra + len(mid.rubies); k < len(old.rubies); k++ {
		old.rubies[k].cluster += dc
	}
	old.marks = slices.Replace(old.marks, ma, mb, mid.marks...)
	for k := ma + len(mid.marks); k < len(old.marks); k++ {
		old.marks[k].cluster += dc
	}
	old.blocks = slices.Replace(old.blocks, first, last+1, mid.blocks...)
	for k := first + len(mid.blocks); k < len(old.blocks); k++ {
		b := &old.blocks[k]
		b.start += delta
		b.end += delta
		b.clusterStart += dc
		b.clusterEnd += dc
	}
	old.text = content
	old.sections, old.spare = secs, olds[:0]
	old.gen++
	old.edit = edit{
		valid: true, first: first, oldLast: last, newLast: first + len(mid.blocks) - 1,
		clusters: dc, bytes: delta,
		oldCount: ce - cs, newCount: len(mid.clusters),
		oldGlyphs: ge - gs, newGlyphs: len(mid.glyphs),
		rubyFirst: ra, oldRubies: rb - ra, newRubies: len(mid.rubies),
	}
	return old
}

// addLines shapes the text from start to end as blocks, one per line.
func (p *Paragraph) addLines(start, end int) {
	for {
		stop := end
		next := -1
		if i := strings.IndexByte(p.text[start:end], '\n'); i >= 0 {
			stop = start + i
			next = stop + 1
		}
		textEnd := stop
		if textEnd > start && p.text[textEnd-1] == '\r' {
			textEnd--
		}
		p.addBlock(start, textEnd)
		if next < 0 {
			return
		}
		start = next
	}
}

// glyphAt returns the index of the first glyph of cluster c, or the number of
// glyphs past the last cluster.
func (p *Paragraph) glyphAt(c int) int {
	if c < len(p.clusters) {
		return p.clusters[c].glyphStart
	}
	return len(p.glyphs)
}

// compareChunk is how many bytes commonPrefix and commonSuffix compare at a
// time before they go byte by byte: string comparison is much faster than a
// loop over bytes.
const compareChunk = 256

func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i+compareChunk <= n && a[i:i+compareChunk] == b[i:i+compareChunk] {
		i += compareChunk
	}
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func commonSuffix(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i+compareChunk <= n && a[len(a)-i-compareChunk:len(a)-i] == b[len(b)-i-compareChunk:len(b)-i] {
		i += compareChunk
	}
	for i < n && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}
	return i
}

// Relayout is Layout that reuses old, a layout of p before the last Reshape:
// only the lines of the blocks that changed are broken again. It updates and
// returns old. Anything else, such as a new length or a MaxLines, takes a
// full Layout.
func (p *Paragraph) Relayout(old *Layout, maxLength float32) *Layout {
	e := &p.edit
	if old == nil || old.paragraph != p || old.gen+1 != p.gen || !e.valid || old.maxLength != maxLength || p.style.MaxLines > 0 {
		return p.Layout(maxLength)
	}
	if len(old.rubySpans) != len(p.rubies)-e.newRubies+e.oldRubies+1 {
		return p.Layout(maxLength)
	}
	l := old
	// The marks come last, and are all placed again at the end.
	marksFrom := l.rubySpans[len(l.rubySpans)-1]
	l.glyphs = l.glyphs[:l.glyphOfSpan(marksFrom)]
	l.spans = l.spans[:marksFrom]
	// The old lines of the changed blocks run from a to b.
	a := sort.Search(len(l.lines), func(i int) bool { return l.lines[i].block >= e.first })
	b := sort.Search(len(l.lines), func(i int) bool { return l.lines[i].block > e.oldLast })
	topA, topB := l.endTop(a), l.endTop(b)
	spanA, spanB := l.spanAt(a), l.spanAt(b)
	glyphA, glyphB := l.glyphOfSpan(spanA), l.glyphOfSpan(spanB)

	cs := p.blocks[e.first].clusterStart
	l.xs = slices.Replace(l.xs, cs, cs+e.oldCount, make([]float32, e.newCount)...)
	l.adv = slices.Replace(l.adv, cs, cs+e.oldCount, make([]float32, e.newCount)...)

	tail := append([]line(nil), l.lines[b:]...)
	l.lines = l.lines[:a]
	top := topA
	for i := e.first; i <= e.newLast; i++ {
		top = l.layoutBlock(i, top)
	}
	newB := len(l.lines)
	dTop := top - topB
	dBlocks := e.newLast - e.oldLast
	for k := range tail {
		ln := &tail[k]
		ln.clusterStart += e.clusters
		ln.clusterEnd += e.clusters
		ln.start += e.bytes
		ln.end += e.bytes
		ln.top += dTop
		ln.block += dBlocks
	}
	l.lines = append(l.lines, tail...)

	extent := l.extent
	l.measure(l.endTop(len(l.lines)))
	if l.extent != extent && (p.style.Align != AlignStart || l.hasRTL()) {
		// The alignment of every line moved with the longest one.
		return p.Layout(maxLength)
	}
	l.align()

	// The glyphs and spans of the new lines, made apart and spliced in.
	glyphs, spans := l.glyphs, l.spans
	l.glyphs, l.spans = nil, nil
	for i := a; i < newB; i++ {
		l.lineSpans(&l.lines[i])
	}
	midGlyphs, midSpans := l.glyphs, l.spans
	for k := range midSpans {
		midSpans[k].start += glyphA
		midSpans[k].end += glyphA
	}
	for i := a; i < newB; i++ {
		l.lines[i].spanStart += spanA
		l.lines[i].spanEnd += spanA
	}
	l.glyphs = slices.Replace(glyphs, glyphA, glyphB, midGlyphs...)
	l.spans = slices.Replace(spans, spanA, spanB, midSpans...)
	dg := len(midGlyphs) - (glyphB - glyphA)
	ds := len(midSpans) - (spanB - spanA)
	for k := spanA + len(midSpans); k < len(l.spans); k++ {
		l.spans[k].start += dg
		l.spans[k].end += dg
	}
	for i := newB; i < len(l.lines); i++ {
		l.lines[i].spanStart += ds
		l.lines[i].spanEnd += ds
	}
	// Lines below moved down, and vertical columns all move with the width.
	for i := range l.lines {
		ln := &l.lines[i]
		origin := l.lineOrigin(ln)
		for k := ln.spanStart; k < ln.spanEnd; k++ {
			l.spans[k].origin = origin
		}
	}
	l.relayoutRubies(e, ds)
	l.addMarkSpans()
	l.makeRuns()
	l.addBackgrounds()
	l.gen = p.gen
	return l
}

// relayoutRubies follows Relayout with the ruby: the ruby of the changed
// blocks is placed again and spliced in, and the rest only moves with its
// line. The spans of the lines before it changed in number by ds.
func (l *Layout) relayoutRubies(e *edit, ds int) {
	p := l.paragraph
	rs := l.rubySpans
	for k := range rs {
		rs[k] += ds
	}
	ra, rb := e.rubyFirst, e.rubyFirst+e.oldRubies
	spanA, spanB := rs[ra], rs[rb]
	glyphA, glyphB := l.glyphOfSpan(spanA), l.glyphOfSpan(spanB)
	glyphs, spans := l.glyphs, l.spans
	l.glyphs = nil
	midSpans, midStarts := l.addRubySpans(nil, ra, ra+e.newRubies, nil)
	midGlyphs := l.glyphs
	for k := range midSpans {
		midSpans[k].start += glyphA
		midSpans[k].end += glyphA
	}
	for k := range midStarts {
		midStarts[k] += spanA
	}
	l.glyphs = slices.Replace(glyphs, glyphA, glyphB, midGlyphs...)
	l.spans = slices.Replace(spans, spanA, spanB, midSpans...)
	dg := len(midGlyphs) - (glyphB - glyphA)
	dsp := len(midSpans) - (spanB - spanA)
	for k := spanA + len(midSpans); k < len(l.spans); k++ {
		l.spans[k].start += dg
		l.spans[k].end += dg
	}
	for k := rb; k < len(rs); k++ {
		rs[k] += dsp
	}
	l.rubySpans = slices.Replace(rs, ra, rb, midStarts...)
	// Every line may have moved, and its ruby with it.
	for i := range p.rubies {
		r := &p.rubies[i]
		ln := l.lineOfCluster(r.cluster)
		if ln == nil {
			continue
		}
		origin := l.rubyOrigin(r, ln)
		for k := l.rubySpans[i]; k < l.rubySpans[i+1]; k++ {
			l.spans[k].origin = origin
		}
	}
}

// endTop returns the top of line i, or the bottom of the last line.
func (l *Layout) endTop(i int) float32 {
	if i < len(l.lines) {
		return l.lines[i].top
	}
	if n := len(l.lines); n > 0 {
		return l.lines[n-1].top + l.lines[n-1].height
	}
	return l.paragraph.topRoom
}

// spanAt returns the first span of line i, or the number of line spans.
func (l *Layout) spanAt(i int) int {
	if i < len(l.lines) {
		return l.lines[i].spanStart
	}
	if n := len(l.lines); n > 0 {
		return l.lines[n-1].spanEnd
	}
	return 0
}

// glyphOfSpan returns the first glyph of span k, or the number of glyphs.
func (l *Layout) glyphOfSpan(k int) int {
	if k < len(l.spans) {
		return l.spans[k].start
	}
	return len(l.glyphs)
}

func (l *Layout) hasRTL() bool {
	for i := range l.lines {
		if l.lines[i].rtl {
			return true
		}
	}
	return false
}
