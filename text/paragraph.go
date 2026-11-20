package text

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

const (
	// fullWidthTolerance is how far an advance may be from the em, as a
	// fraction, to count as full width. Fonts round their advances to font units.
	fullWidthTolerance = 0.02

	// rubySize is the size of ruby relative to its base (JLReq 3.3.2).
	rubySize = 0.5
	// rubyLift is the height of the ruby baseline above the base baseline, in
	// base ems. It puts the ruby just above the em box.
	rubyLift = 0.95
	// rubyInk is how far ruby reaches above its baseline, in ruby ems.
	rubyInk = 0.9
)

// RubyLineHeight is the least line height, as a multiple of the font size,
// that leaves ruby room between lines. Text with ruby, or with Style.RubyRoom,
// has at least this line height.
const RubyLineHeight = 1.75

// Span is a stretch of text with its own look. Zero fields of Style take the
// value of the paragraph, and the composition rules are the paragraph's.
type Span struct {
	Text  string
	Style Style
	// Ruby is set above Text, or to its right in vertical text, which then
	// stays on one line as a group. For mono ruby, make one Span per base
	// character. Ruby is only drawn: the text of the paragraph, which a
	// screen reader reads, copies and searches, is the base text alone.
	Ruby string
	// TateChuYoko sets Text horizontally within one em of a vertical line,
	// as is done for two or three digits (JLReq 3.4.3). Longer text shrinks to
	// fit. Horizontal text ignores it.
	TateChuYoko bool
	// Upright stands Western text upright in a vertical line, one character
	// below the other, instead of turning it sideways.
	Upright bool
	// Emphasis sets an emphasis mark (圏点, 傍点) beside every character of
	// Text: one mark per character, at half its size, centered on it, above
	// it in horizontal text and to its right in vertical text, on the side of
	// ruby. Spaces, controls and punctuation get none, and tate-chu-yoko gets
	// one for the group. Where Text also has Ruby, the marks stand outside
	// the ruby, and the lines of the paragraph open up to make room for both.
	// Text with emphasis takes the line height of ruby (RubyLineHeight). See
	// docs/ADR/0135-emphasis-dots.md.
	Emphasis Emphasis
	// Background, when not transparent, fills the box of Text behind its
	// glyphs, as a highlighter does, for the matches of a search: along the
	// line the advances of its characters, across it the em box of its size,
	// reaching down to the descent of the font. Text that breaks across lines
	// gets a box on each line. Ruby and emphasis marks stand outside the box,
	// as the CSS mark element leaves ruby above its highlight, and the line
	// gap between boxes stays open. Layout.Backgrounds returns the boxes;
	// basic.Text and basic.RichText paint them. See
	// docs/ADR/0139-scroll-position-stick-to-end-span-background.md.
	Background geom.Color
}

// shapedGlyph is a glyph relative to the start of its cluster. inline runs
// along the line. cross runs across it: down from the baseline in horizontal
// text, right of the column center in vertical text.
type shapedGlyph struct {
	id            uint32
	inline, cross float32
}

// cluster is what the caret cannot enter: a grapheme cluster, grown to the
// extent of a ligature or a ruby group.
type cluster struct {
	// start and end are byte offsets into the text.
	start, end int
	// glyphStart and glyphEnd delimit its glyphs.
	glyphStart, glyphEnd int
	face                 paint.FaceID
	section              int
	// size is the font size to draw the glyphs at, smaller than the section's
	// for tate-chu-yoko that had to shrink.
	size float32
	// sideways turns the glyphs 90° clockwise, for Western text in vertical
	// lines.
	sideways bool
	// mixed marks a ruby group whose base is drawn in pieces of different
	// looks (rubyRun.pieces); face, size and sideways are then those of its
	// first piece.
	mixed bool
	// level is the bidi embedding level: odd runs right to left.
	level uint8
}

// block is a stretch of text between hard line breaks.
type block struct {
	// start and end are byte offsets, without the line break.
	start, end int
	// clusterStart and clusterEnd delimit its clusters.
	clusterStart, clusterEnd int
	// rtl is set for a block that runs right to left, and bidi for one whose
	// clusters may leave logical order.
	rtl, bidi bool
}

// section is the part of the text that one Span covers.
type section struct {
	start, end int
	style      Style
	color      geom.Color
	ruby       string
	tcy        bool
	upright    bool
	emphasis   Emphasis
	background geom.Color
	// mark is the emphasis mark in the style of the section, shaped when its
	// first character is.
	mark   *markGlyph
	hyphen hyphenGlyph
	// ellipsis is "…" in the style of the section, shaped on first use.
	ellipsis *Paragraph
}

// rubyRun is the ruby of one cluster.
type rubyRun struct {
	cluster int
	shaped  *Paragraph
	// x moves the ruby relative to its base.
	x float32
	// gap is the space between ruby characters.
	gap float32
	// pieces are the stretches of the glyphs of the base that differ in
	// face, size or orientation, as Latin and kanji in 「AI時代」 or a kanji
	// from a fallback font do. It is nil when the whole base looks alike,
	// and its cluster is marked mixed when not.
	pieces []basePiece
}

// basePiece is a stretch of the glyphs of a ruby base drawn alike, from glyph
// on, counted from the first glyph of the base, to the next piece.
type basePiece struct {
	glyph    int
	face     paint.FaceID
	size     float32
	sideways bool
}

// Paragraph is shaped text, ready to be broken into lines at any width. It
// keeps everything that does not depend on the width, so that a resized window
// does not shape again.
type Paragraph struct {
	shaper *Shaper
	text   string
	style  Style

	ascent, descent float32
	// vertical is set for vertical writing. emCenter is how far the center of
	// the em box lies above the baseline, in ems.
	vertical bool
	emCenter float32
	// lineHeight is the line height factor in effect, and topRoom the space
	// above the first line that its ruby needs.
	lineHeight float32
	topRoom    float32

	sections []section
	blocks   []block
	clusters []cluster
	units    []jlreq.Unit
	glyphs   []shapedGlyph
	rubies   []rubyRun
	// marks are the emphasis marks, in the order of their clusters.
	marks []emphasisMark
	// spare is the room of the sections before the last ReshapeSpans, for the
	// next one to fill.
	spare []section

	// gen counts the edits Reshape made, and edit says what the last one
	// changed, for Relayout.
	gen  uint64
	edit edit
}

// Shape prepares content for layout. Line feeds end a line, with or without a
// carriage return before them, and tabs count as spaces.
func (s *Shaper) Shape(content string, st Style) *Paragraph {
	return s.ShapeSpans([]Span{{Text: content}}, st)
}

// ShapeSpans prepares text of several styles, with ruby, for layout. st is the
// style of the paragraph.
func (s *Shaper) ShapeSpans(spans []Span, st Style) *Paragraph {
	st = st.resolved()
	s.selectFaces(&st)
	p := &Paragraph{shaper: s, style: st, lineHeight: st.LineHeight, vertical: st.Orientation == Vertical}
	p.ascent, p.descent, p.emCenter = s.lineMetrics(&st)
	p.addSections(spans)
	content := p.text
	for start := 0; ; {
		end := len(content)
		next := -1
		if i := strings.IndexByte(content[start:], '\n'); i >= 0 {
			end = start + i
			next = end + 1
		}
		textEnd := end
		if textEnd > start && content[textEnd-1] == '\r' {
			textEnd--
		}
		p.addBlock(start, textEnd)
		if next < 0 {
			break
		}
		start = next
	}
	p.fitRubies(0)
	p.lineHeight, p.topRoom = p.rubyRoom()
	return p
}

func (p *Paragraph) addSections(spans []Span) {
	p.sections = p.makeSections(spans, nil)
	p.text = joinSpans(spans)
}

// makeSections appends the sections of spans to dst.
func (p *Paragraph) makeSections(spans []Span, dst []section) []section {
	at := 0
	for i := range spans {
		sp := &spans[i]
		dst = append(dst, section{
			start:      at,
			end:        at + len(sp.Text),
			style:      sp.Style.over(&p.style),
			color:      sp.Style.Color,
			ruby:       sp.Ruby,
			tcy:        sp.TateChuYoko,
			upright:    sp.Upright,
			emphasis:   sp.Emphasis,
			background: sp.Background,
		})
		at += len(sp.Text)
	}
	return dst
}

// joinSpans returns the text of spans, without a copy for a single span.
func joinSpans(spans []Span) string {
	if len(spans) == 1 {
		return spans[0].Text
	}
	n := 0
	for i := range spans {
		n += len(spans[i].Text)
	}
	var text strings.Builder
	text.Grow(n)
	for i := range spans {
		text.WriteString(spans[i].Text)
	}
	return text.String()
}

// sameLook reports whether two sections set their text alike, wherever they
// are.
func sameLook(a, b *section) bool {
	return a.style == b.style && a.color == b.color && a.ruby == b.ruby && a.tcy == b.tcy && a.upright == b.upright &&
		a.emphasis == b.emphasis && a.background == b.background
}

// addBlock shapes the text between two hard line breaks.
func (p *Paragraph) addBlock(start, end int) {
	b := block{start: start, end: end, clusterStart: len(p.clusters)}
	defer func() {
		b.clusterEnd = len(p.clusters)
		p.blocks = append(p.blocks, b)
	}()
	if start == end {
		return
	}
	runes, offsets := decode(p.text[start:end], start)
	s := p.shaper

	graphemes := make([]bool, len(runes)+1)
	lineBreaks := make([]bool, len(runes)+1)
	s.breaks.Init(runes)
	for it := s.breaks.GraphemeIterator(); it.Next(); {
		g := it.Grapheme()
		graphemes[g.Offset+len(g.Text)] = true
	}
	for it := s.breaks.LineIterator(); it.Next(); {
		l := it.Line()
		lineBreaks[l.Offset+len(l.Text)] = true
	}
	shaped := blockText{runes: runes, offsets: offsets, graphemes: graphemes, lineBreaks: lineBreaks}
	if !p.vertical {
		// Vertical lines keep logical order: the scripts that run right to
		// left are not set vertically.
		shaped.levels, shaped.rtl = s.blockLevels(runes, p.style.Direction)
		b.rtl, b.bidi = shaped.rtl, shaped.levels != nil
	}
	// The sections are in order: start from the first that reaches the block.
	from := sort.Search(len(p.sections), func(i int) bool { return p.sections[i].end > start })
	for i := from; i < len(p.sections) && p.sections[i].start < end; i++ {
		sec := &p.sections[i]
		a, z := max(sec.start, start), min(sec.end, end)
		if a >= z {
			continue
		}
		first := sort.SearchInts(offsets, a)
		last := sort.SearchInts(offsets, z)
		from := len(p.clusters)
		p.shapeSection(&shaped, i, first, last)
		if sec.emphasis.mark() != "" {
			p.addMarks(from, i)
		}
		if sec.ruby != "" && from < len(p.clusters) {
			p.addRuby(from, i)
		}
	}
	if p.style.Hyphenate {
		p.markHyphens(b.clusterStart, len(p.clusters))
	}
}

// blockText is what the sections of a block share.
type blockText struct {
	runes                 []rune
	offsets               []int
	graphemes, lineBreaks []bool
	// levels are the embedding levels of the runes, nil if all are zero.
	levels []uint8
	rtl    bool
}

// shapeSection shapes the runes from first to last in the style of section i.
func (p *Paragraph) shapeSection(bt *blockText, i, first, last int) {
	s := p.shaper
	sec := &p.sections[i]
	st := &sec.style
	s.selectFaces(st)
	if p.vertical && sec.tcy && p.shapeTateChuYoko(bt, i, first, last) {
		return
	}
	// Emoji take fonts of their own, so they are shaped apart, a stretch of
	// whole grapheme clusters at a time.
	start, emoji := first, false
	for a := first; a < last; {
		b := a + 1
		for b < last && !bt.graphemes[b] {
			b++
		}
		e := isEmojiCluster(bt.runes[a:b])
		if e != emoji && a > start {
			p.shapeRange(bt, i, start, a, emoji)
			start = a
		}
		emoji = e
		a = b
	}
	p.shapeRange(bt, i, start, last, emoji)
}

// shapeRange shapes the runes from first to last, whole grapheme clusters, in
// the style of section i. emoji prefers the color emoji fonts.
func (p *Paragraph) shapeRange(bt *blockText, i, first, last int, emoji bool) {
	s := p.shaper
	sec := &p.sections[i]
	st := &sec.style
	s.selectFacesFor(st, emoji)
	defer s.selectFaces(st)
	input := shaping.Input{
		Text:      bt.runes[first:last],
		RunEnd:    last - first,
		Direction: directionFor(p.vertical, sec.upright),
		Size:      fixed.Int26_6(st.Size * 64),
		Language:  s.language,
	}
	if bt.rtl {
		input.Direction = di.DirectionRTL
	}
	runs := s.split.Split(input, s.fonts)
	if p.vertical {
		runs = keepWideUpright(runs)
	}
	emCenter := p.emCenter * st.Size
	pending := pendingCluster{first: -1}
	for _, run := range runs {
		sideways := run.Direction.IsSideways()
		if sideways {
			// Shaped as horizontal text, and turned when drawn.
			run.Direction = di.DirectionLTR
		}
		vertical := run.Direction.IsVertical()
		if isEastAsian(run.Script) {
			run.FontFeatures = s.noKern
		}
		var hb *shaping.HarfbuzzShaper
		run.Face, hb = s.varied(run.Face, st)
		out := hb.Shape(run)
		// HarfBuzz returns right-to-left glyphs in visual order. The
		// clusters are taken in logical order, so such a run is read from
		// its end.
		rtl := run.Direction == di.DirectionRTL
		var face paint.FaceID
		if st.Italic && !emoji && !rtl && !p.vertical {
			// Italic of a face without one is slanted, except for color
			// emoji, right-to-left scripts and vertical lines. See
			// docs/ADR/0101-synthetic-italic.md.
			face = s.italicFaceID(out.Face)
		} else {
			face = s.faceID(out.Face)
		}
		groups := clusterStarts(out.Glyphs)
		for n := range groups {
			if rtl {
				n = len(groups) - 1 - n
			}
			g := groups[n]
			head := &out.Glyphs[g]
			count := head.GlyphsCount()
			var advance float32
			pen := pending.advance
			if rtl && pending.first >= 0 {
				// A logically later piece of a right-to-left cluster sits
				// left of the earlier ones.
				var w float32
				for k := range count {
					_, _, adv := placeGlyph(&out.Glyphs[g+k], vertical, sideways, emCenter)
					w += adv
				}
				for k := len(p.glyphs) - pending.glyphs; k < len(p.glyphs); k++ {
					p.glyphs[k].inline += w
				}
				pen = 0
			}
			for k := range count {
				inline, cross, adv := placeGlyph(&out.Glyphs[g+k], vertical, sideways, emCenter)
				p.glyphs = append(p.glyphs, shapedGlyph{
					id:     uint32(out.Glyphs[g+k].GlyphID),
					inline: pen + advance + inline,
					cross:  cross,
				})
				advance += adv
			}
			at := first + head.TextIndex()
			pending.add(at, at+head.RunesCount(), advance, count, face, sideways)
			if bt.graphemes[pending.last] {
				p.addCluster(&pending, bt, i)
			}
		}
	}
	if pending.first >= 0 {
		p.addCluster(&pending, bt, i)
	}
}

// pendingCluster collects shaped clusters until a grapheme boundary.
type pendingCluster struct {
	first, last int // runes
	advance     float32
	glyphs      int
	face        paint.FaceID
	sideways    bool
}

func (c *pendingCluster) add(first, last int, advance float32, glyphs int, face paint.FaceID, sideways bool) {
	if c.first < 0 {
		c.first, c.face, c.sideways = first, face, sideways
	}
	c.last = max(c.last, last)
	c.advance += advance
	c.glyphs += glyphs
}

func (p *Paragraph) addCluster(c *pendingCluster, bt *blockText, section int) {
	em := p.sections[section].style.Size
	p.clusters = append(p.clusters, cluster{
		start:      bt.offsets[c.first],
		end:        bt.offsets[c.last],
		glyphStart: len(p.glyphs) - c.glyphs,
		glyphEnd:   len(p.glyphs),
		face:       c.face,
		section:    section,
		size:       em,
		sideways:   c.sideways,
		level:      levelAt(bt.levels, c.first),
	})
	p.addUnit(bt, c.first, c.last, c.advance, em, false)
	*c = pendingCluster{first: -1}
}

// addUnit adds what jlreq needs to know of the cluster of the runes from first
// to last.
func (p *Paragraph) addUnit(bt *blockText, first, last int, advance, em float32, ideographic bool) {
	diff := advance - em
	p.units = append(p.units, jlreq.Unit{
		Rune:        bt.runes[first],
		Advance:     advance,
		Em:          em,
		FullWidth:   diff > -em*fullWidthTolerance && diff < em*fullWidthTolerance,
		BreakAfter:  bt.lineBreaks[last],
		Tab:         p.text[bt.offsets[first]] == '\t' && p.style.TabStop > 0,
		Ideographic: ideographic,
	})
}

// addRuby turns the clusters from first on into one group that carries ruby.
func (p *Paragraph) addRuby(first, sec int) {
	last := len(p.clusters) - 1
	merged := p.clusters[first]
	// The clusters keep how they are drawn, piece by piece.
	var pieces []basePiece
	for i := first + 1; i <= last; i++ {
		c, prev := &p.clusters[i], &p.clusters[i-1]
		if c.face == prev.face && c.size == prev.size && c.sideways == prev.sideways {
			continue
		}
		if pieces == nil {
			pieces = append(pieces, basePiece{0, merged.face, merged.size, merged.sideways})
		}
		pieces = append(pieces, basePiece{c.glyphStart - merged.glyphStart, c.face, c.size, c.sideways})
	}
	merged.mixed = pieces != nil
	offset := float32(0)
	// The marks of the clusters move into the group.
	mk := len(p.marks)
	for mk > 0 && p.marks[mk-1].cluster >= first {
		mk--
	}
	for i := first; i <= last; i++ {
		c := &p.clusters[i]
		for ; mk < len(p.marks) && p.marks[mk].cluster == i; mk++ {
			m := &p.marks[mk]
			m.cluster, m.center, m.glyph = first, m.center+offset, c.glyphStart-merged.glyphStart
		}
		for g := c.glyphStart; g < c.glyphEnd; g++ {
			p.glyphs[g].inline += offset
		}
		offset += p.units[i].Advance
	}
	merged.end, merged.glyphEnd = p.clusters[last].end, p.clusters[last].glyphEnd
	// The group meets the text before it by its first character and the text
	// after it by its last, for the line breaking rules and the space
	// between Japanese and Western text.
	unit := p.units[first]
	end := &p.units[last]
	unit.Last = end.Last
	if unit.Last == 0 {
		unit.Last = end.Rune
	}
	unit.Advance = offset
	unit.FullWidth = false
	unit.BreakAfter = end.BreakAfter
	p.clusters = append(p.clusters[:first], merged)
	p.units = append(p.units[:first], unit)

	st := p.sections[sec].style
	st.Size *= rubySize
	st.Hanging = false
	shaped := p.shaper.Shape(p.sections[sec].ruby, st)
	p.shaper.selectFaces(&p.style)
	p.rubies = append(p.rubies, rubyRun{cluster: first, shaped: shaped, pieces: pieces})
}

// fitRubies settles the width of the ruby groups from the one at index from on
// (JLReq 3.3.6). Ruby shorter than its base spreads out 1:2:1. Ruby longer than
// its base may overhang a neighboring kana of its block by up to one ruby
// character on each side, and the base opens up for the rest. The overhang
// is kept in the unit, so that the base takes that room itself where it
// starts or ends a line, and the ruby stays within the line.
func (p *Paragraph) fitRubies(from int) {
	for i := from; i < len(p.rubies); i++ {
		r := &p.rubies[i]
		u := &p.units[r.cluster]
		width := r.shaped.naturalWidth()
		if count := len(r.shaped.clusters); width < u.Advance && count > 0 {
			r.gap = (u.Advance - width) / float32(count)
			r.x = r.gap / 2
			continue
		}
		over := width - u.Advance
		before, after := p.allowedOverhang(r.cluster, r.shaped.style.Size)
		// Overhang as evenly as the neighbors allow.
		lead := min(over/2, before)
		trail := min(over-lead, after)
		lead = min(over-trail, before)
		grow := over - lead - trail
		p.spreadBase(r.cluster, grow)
		r.x = -lead
		u.Advance += grow
		u.StartExtra, u.EndExtra = lead, trail
	}
}

// rubyRoom returns the line height and the space above the first line that
// the ruby and the emphasis marks of the paragraph need, or that
// Style.RubyRoom asks for.
func (p *Paragraph) rubyRoom() (lineHeight, topRoom float32) {
	return p.roomFor(len(p.rubies) > 0 || len(p.marks) > 0, p.stacked(p.marks))
}

// roomFor returns the line height and the space above the first line for a
// paragraph that is annotated, with ruby or emphasis marks, or not, and whose
// marks are stacked outside ruby or not.
func (p *Paragraph) roomFor(annotated, stacked bool) (lineHeight, topRoom float32) {
	lineHeight = p.style.LineHeight
	if !annotated && !p.style.RubyRoom {
		return lineHeight, 0
	}
	// Ruby and marks take one row of half an em beside the base; marks
	// outside ruby take two.
	var more float32
	if stacked {
		more = rubySize
	}
	lineHeight = max(lineHeight, RubyLineHeight+more)
	em := p.style.Size
	if p.vertical {
		// Ruby stands right of the em box of its column.
		return lineHeight, max(em/2+(rubySize+more)*em-em*lineHeight/2, 0)
	}
	baseline := p.baselineIn(em * lineHeight)
	return lineHeight, max((rubyLift+more)*em+rubyInk*rubySize*em-baseline, 0)
}

// allowedOverhang returns how far ruby may reach over the neighbors before
// and after group i. Only kana without ruby or emphasis marks of their own in
// the same block count, so that a block can be shaped apart and the ruby does
// not run into the marks.
func (p *Paragraph) allowedOverhang(i int, rubyEm float32) (before, after float32) {
	b := sort.Search(len(p.blocks), func(k int) bool { return p.blocks[k].clusterEnd > i })
	lo, hi := 0, len(p.units)
	if b < len(p.blocks) {
		lo, hi = p.blocks[b].clusterStart, p.blocks[b].clusterEnd
	}
	free := func(n int) float32 {
		if n >= lo && n < hi && jlreq.Classify(p.units[n].Rune) == jlreq.ClassKana && !p.hasRuby(n) && !p.hasMark(n) {
			return rubyEm
		}
		return 0
	}
	return free(i - 1), free(i + 1)
}

func (p *Paragraph) hasRuby(i int) bool {
	return slices.ContainsFunc(p.rubies, func(r rubyRun) bool { return r.cluster == i })
}

// spreadBase opens up the glyphs of a group by grow in all, 1:2:1.
func (p *Paragraph) spreadBase(i int, grow float32) {
	c := &p.clusters[i]
	n := c.glyphEnd - c.glyphStart
	if n == 0 || grow <= 0 {
		return
	}
	gap := grow / float32(n)
	for k := range n {
		p.glyphs[c.glyphStart+k].inline += gap * (float32(k) + 0.5)
	}
	mk, end := p.marksOf(i)
	for ; mk < end; mk++ {
		m := &p.marks[mk]
		m.center += gap * (float32(m.glyph) + 0.5)
	}
}

// naturalWidth returns the width of the text on one line.
func (p *Paragraph) naturalWidth() float32 {
	var w float32
	for i := range p.units {
		w += p.units[i].Advance
	}
	return w
}

// baselineIn returns the baseline from the top of a line of the given height.
func (p *Paragraph) baselineIn(height float32) float32 {
	return (height-(p.ascent+p.descent))/2 + p.ascent
}

// decode returns the runes of s and the byte offset of each, plus the end. It
// replaces tabs with spaces, which the fonts can draw.
func decode(s string, base int) (runes []rune, offsets []int) {
	n := utf8.RuneCountInString(s)
	runes = make([]rune, 0, n)
	offsets = make([]int, 0, n+1)
	for i, r := range s {
		if r == '\t' {
			r = ' '
		}
		runes = append(runes, r)
		offsets = append(offsets, base+i)
	}
	return runes, append(offsets, base+len(s))
}

func isEastAsian(s language.Script) bool {
	return s == language.Han || s == language.Hiragana || s == language.Katakana
}

func toFloat(v fixed.Int26_6) float32 { return float32(v) / 64 }

// clusterStarts returns the index of the first glyph of every HarfBuzz cluster.
func clusterStarts(glyphs []shaping.Glyph) []int {
	starts := make([]int, 0, len(glyphs))
	for g := 0; g < len(glyphs); g += glyphs[g].GlyphsCount() {
		starts = append(starts, g)
	}
	return starts
}

func levelAt(levels []uint8, i int) uint8 {
	if levels == nil {
		return 0
	}
	return levels[i]
}
