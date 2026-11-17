package text

import (
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text/hyphen"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// hyphenGlyph is the hyphen that ends a line broken inside a word.
type hyphenGlyph struct {
	id      uint32
	face    paint.FaceID
	advance float32
	ok      bool
}

// hyphenFor returns the hyphen in the style of section i, shaping it on first
// use.
func (p *Paragraph) hyphenFor(i int) hyphenGlyph {
	sec := &p.sections[i]
	if !sec.hyphen.ok {
		st := sec.style
		st.Hyphenate = false
		h := p.shaper.Shape("-", st)
		p.shaper.selectFaces(&p.style)
		if len(h.glyphs) == 1 {
			sec.hyphen = hyphenGlyph{id: h.glyphs[0].id, face: h.clusters[0].face, advance: h.units[0].Advance, ok: true}
		}
	}
	return sec.hyphen
}

// markHyphens lets the Western words among clusters first to last break where
// the English patterns allow, by giving those clusters a hyphen width.
func (p *Paragraph) markHyphens(first, last int) {
	en := hyphen.English()
	var points []int
	for c := first; c < last; {
		if !p.isWordCluster(c) {
			c++
			continue
		}
		end := c + 1
		for end < last && p.isWordCluster(end) && p.clusters[end].section == p.clusters[c].section {
			end++
		}
		word := p.text[p.clusters[c].start:p.clusters[end-1].end]
		points = en.Points(word, points[:0])
		if len(points) > 0 {
			h := p.hyphenFor(p.clusters[c].section)
			// Walk the clusters, counting runes, to find where each point falls.
			runes, k := 0, 0
			for w := c; w < end && k < len(points); w++ {
				cl := &p.clusters[w]
				runes += utf8.RuneCountInString(p.text[cl.start:cl.end])
				for k < len(points) && points[k] < runes {
					k++
				}
				if k < len(points) && points[k] == runes && h.ok {
					p.units[w].Hyphen = h.advance
				}
			}
		}
		c = end
	}
}

// isWordCluster reports whether cluster c is a letter of a Western word.
func (p *Paragraph) isWordCluster(c int) bool {
	r := p.units[c].Rune
	return jlreq.Classify(r) == jlreq.ClassWestern && unicode.IsLetter(r) && !p.clusters[c].sideways
}
