package text

import (
	"sort"
	"unicode"

	"github.com/go-text/typesetting/font"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// emphasisSize is the size of an emphasis mark relative to its base, as for
// ruby.
const emphasisSize = 0.5

// Emphasis is the emphasis mark (圏点, 傍点) set beside every character of a
// Span. The zero value sets none. See Span.Emphasis.
type Emphasis uint8

// The emphasis marks.
const (
	EmphasisNone Emphasis = iota
	// EmphasisSesame is the filled sesame ﹅, the default mark of Japanese
	// text (JIS X 4051) and of the 傍点 of Aozora Bunko.
	EmphasisSesame
	// EmphasisOpenSesame is the open sesame ﹆ (白ゴマ傍点).
	EmphasisOpenSesame
	// EmphasisDot is the small filled dot •.
	EmphasisDot
	// EmphasisCircle is the filled circle ● (丸傍点).
	EmphasisCircle
	// EmphasisOpenCircle is the open circle ○ (白丸傍点).
	EmphasisOpenCircle
	// EmphasisDoubleCircle is the double circle ◎ (二重丸傍点).
	EmphasisDoubleCircle
	// EmphasisBullseye is the bullseye ◉ (蛇の目傍点).
	EmphasisBullseye
	// EmphasisTriangle is the filled triangle ▲ (黒三角傍点).
	EmphasisTriangle
	// EmphasisOpenTriangle is the open triangle △ (白三角傍点).
	EmphasisOpenTriangle
	// EmphasisCross is the saltire × (ばつ傍点).
	EmphasisCross
)

// String returns the name of e.
func (e Emphasis) String() string {
	switch e {
	case EmphasisNone:
		return "None"
	case EmphasisSesame:
		return "Sesame"
	case EmphasisOpenSesame:
		return "OpenSesame"
	case EmphasisDot:
		return "Dot"
	case EmphasisCircle:
		return "Circle"
	case EmphasisOpenCircle:
		return "OpenCircle"
	case EmphasisDoubleCircle:
		return "DoubleCircle"
	case EmphasisBullseye:
		return "Bullseye"
	case EmphasisTriangle:
		return "Triangle"
	case EmphasisOpenTriangle:
		return "OpenTriangle"
	case EmphasisCross:
		return "Cross"
	}
	return "Emphasis(?)"
}

// mark returns the character drawn for e, or "" for none.
func (e Emphasis) mark() string {
	switch e {
	case EmphasisSesame:
		return "﹅" // ﹅
	case EmphasisOpenSesame:
		return "﹆" // ﹆
	case EmphasisDot:
		return "•"
	case EmphasisCircle:
		return "●"
	case EmphasisOpenCircle:
		return "○"
	case EmphasisDoubleCircle:
		return "◎"
	case EmphasisBullseye:
		return "◉"
	case EmphasisTriangle:
		return "▲"
	case EmphasisOpenTriangle:
		return "△"
	case EmphasisCross:
		return "×"
	}
	return ""
}

// emphasisMark is the emphasis mark of one character.
type emphasisMark struct {
	// cluster is the cluster of the character, and center the middle of the
	// character along the line, from the start of the cluster.
	cluster int
	center  float32
	// glyph is the first glyph of the character counted from the first glyph
	// of its cluster. Spreading the base of ruby moves the glyphs, and the
	// mark with them.
	glyph int
}

// takesMark reports whether a character starting with r gets an emphasis
// mark: spaces, controls and punctuation do not (JLReq, CSS Text Decoration 3
// text-emphasis).
func takesMark(r rune) bool {
	return !unicode.IsSpace(r) && !unicode.In(r, unicode.Z, unicode.P, unicode.Cc, unicode.Cf)
}

// addMarks gives the clusters from first on, of section sec, their emphasis
// marks, one per cluster, centered on it.
func (p *Paragraph) addMarks(first, sec int) {
	p.markFor(sec)
	for c := first; c < len(p.clusters); c++ {
		u := &p.units[c]
		if takesMark(u.Rune) {
			p.marks = append(p.marks, emphasisMark{cluster: c, center: u.Advance / 2})
		}
	}
}

// markGlyph is an emphasis mark shaped in the style of a section.
type markGlyph struct {
	shaped *Paragraph
	// inline and cross are the middle of the ink of the mark from its origin,
	// along the line and across it (down in horizontal text, right in
	// vertical text). Fonts put the ink of the marks in different places of
	// the em box, the sesame of a Japanese font near a corner for vertical
	// text, so the marks are placed by their ink.
	inline, cross float32
}

// markFor returns the emphasis mark of section i, shaping it on first use.
func (p *Paragraph) markFor(i int) *markGlyph {
	sec := &p.sections[i]
	if sec.mark == nil {
		st := sec.style
		st.Size *= emphasisSize
		st.Italic, st.Hanging, st.Hyphenate = false, false, false
		st.MaxLines, st.Indent, st.HangingIndent = 0, 0, 0
		// A mark stands upright in vertical text: a triangle points up.
		q := p.shaper.ShapeSpans([]Span{{Text: sec.emphasis.mark(), Upright: true}}, st)
		p.shaper.selectFaces(&p.style)
		sec.mark = &markGlyph{shaped: q}
		sec.mark.inline, sec.mark.cross = p.shaper.inkCenter(q)
	}
	return sec.mark
}

// inkCenter returns the middle of the ink of the first glyph of q, along and
// across the line from the origin of q. Without the extents of the glyph, it
// is the middle of the em box.
func (s *Shaper) inkCenter(q *Paragraph) (inline, cross float32) {
	if len(q.clusters) == 0 || q.clusters[0].glyphEnd == q.clusters[0].glyphStart {
		return 0, 0
	}
	cl := &q.clusters[0]
	sg := &q.glyphs[cl.glyphStart]
	em := cl.size
	inline, cross = q.naturalWidth()/2, -q.emCenter*em
	if q.vertical {
		cross = 0
	}
	face := s.faces[cl.face]
	if face == nil || cl.sideways {
		return inline, cross
	}
	ext, ok := face.GlyphExtents(font.GID(sg.id))
	if !ok || ext.Width == 0 || ext.Height == 0 {
		return inline, cross
	}
	unit := em / float32(face.Upem())
	// A glyph is drawn from its horizontal origin, y up in font units.
	x := (ext.XBearing + ext.Width/2) * unit
	y := -(ext.YBearing + ext.Height/2) * unit
	if q.vertical {
		return sg.inline + y, sg.cross + x
	}
	return sg.inline + x, sg.cross + y
}

// marksOf returns the marks of cluster c, from k to the end.
func (p *Paragraph) marksOf(c int) (k, end int) {
	m := p.marks
	k = sort.Search(len(m), func(i int) bool { return m[i].cluster >= c })
	end = k
	for end < len(m) && m[end].cluster == c {
		end++
	}
	return k, end
}

func (p *Paragraph) hasMark(c int) bool {
	k, end := p.marksOf(c)
	return end > k
}

// stacked reports whether a mark of marks stands outside ruby, which takes
// more room between lines.
func (p *Paragraph) stacked(marks []emphasisMark) bool {
	for i := range marks {
		if p.overRuby(&marks[i]) {
			return true
		}
	}
	return false
}

// overRuby reports whether m is on a character with ruby.
func (p *Paragraph) overRuby(m *emphasisMark) bool {
	return p.sections[p.clusters[m.cluster].section].ruby != ""
}

// addMarkSpans adds the glyphs of the emphasis marks beside their characters:
// above them, or right of them in vertical text, and outside the ruby of
// characters that have it.
func (l *Layout) addMarkSpans() {
	p := l.paragraph
	spans := l.spans
	first := len(spans)
	for i := range p.marks {
		m := &p.marks[i]
		ln := l.lineOfCluster(m.cluster)
		if ln == nil {
			continue
		}
		sec := &p.sections[p.clusters[m.cluster].section]
		if sec.mark == nil || len(sec.mark.shaped.clusters) == 0 {
			continue
		}
		q := sec.mark.shaped
		at := ln.x + l.xs[m.cluster] + m.center - sec.mark.inline
		origin := l.markOrigin(m, ln, sec.mark.cross)
		for c := range q.clusters {
			cl := &q.clusters[c]
			start := len(l.glyphs)
			for g := cl.glyphStart; g < cl.glyphEnd; g++ {
				sg := &q.glyphs[g]
				x, y := l.place(at+sg.inline, sg.cross)
				l.glyphs = append(l.glyphs, paint.Glyph{ID: sg.id, X: x, Y: y})
			}
			next := at + q.units[c].Advance
			key := runKey{cl.face, cl.size, sec.color, cl.sideways}
			if n := len(spans); n > first && spans[n-1].key == key && spans[n-1].origin == origin {
				spans[n-1].end = len(l.glyphs)
				spans[n-1].lo = min(spans[n-1].lo, at)
				spans[n-1].hi = max(spans[n-1].hi, next)
			} else {
				spans = append(spans, span{key, start, len(l.glyphs), origin, at, next})
			}
			at = next
		}
	}
	l.spans = spans
}

// markOrigin returns the origin of the glyphs of m, whose character is on ln.
// The middle of the ink of the mark, cross from the origin, goes to the middle
// of the row of the marks: half an em wide, beside the em box of the
// character, or beside its ruby.
func (l *Layout) markOrigin(m *emphasisMark, ln *line, cross float32) geom.Offset {
	p := l.paragraph
	cl := &p.clusters[m.cluster]
	size := p.sections[cl.section].style.Size
	row := (0.5 + emphasisSize/2) * size
	if p.overRuby(m) {
		row += rubySize * size
	}
	origin := l.lineOrigin(ln)
	if p.vertical {
		origin.X += row - cross
		return origin
	}
	lift := p.emCenter*size + row
	origin.Y -= lift + cross
	if p.shaper.slanted[cl.face] {
		origin.X += obliqueShear * lift
	}
	return origin
}
