package text

import (
	"github.com/go-text/typesetting/di"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/width"
)

// defaultEmCenter is how far the center of the ideographic em box lies above
// the baseline, in ems, for a font that does not say. Japanese fonts put the
// bottom of the em box 0.12 em below the baseline.
const defaultEmCenter = 0.38

// directionFor returns the shaping direction of a section of a paragraph.
func directionFor(vertical, upright bool) di.Direction {
	if !vertical {
		return di.DirectionLTR
	}
	d := di.DirectionTTB
	if upright {
		d.SetSideways(false)
	}
	return d
}

// keepWideUpright splits the sideways runs of vertical text so that wide and
// full-width characters stand upright. The segmenter turns every character of
// UAX #50 class Tr sideways, but Japanese fonts have vertical forms for those
// that matter, such as ー, 〜 and （, and `vert` picks them up.
func keepWideUpright(runs []shaping.Input) []shaping.Input {
	out := make([]shaping.Input, 0, len(runs))
	for _, run := range runs {
		if !run.Direction.IsSideways() {
			out = append(out, run)
			continue
		}
		sub := run
		for i := run.RunStart; i < run.RunEnd; i++ {
			upright := isWide(run.Text[i])
			if i == run.RunStart {
				sub.Direction.SetSideways(!upright)
				continue
			}
			if upright == sub.Direction.IsSideways() {
				sub.RunEnd = i
				out = append(out, sub)
				sub.RunStart = i
				sub.Direction.SetSideways(!upright)
			}
		}
		sub.RunEnd = run.RunEnd
		out = append(out, sub)
	}
	return out
}

func isWide(r rune) bool {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return true
	}
	return false
}

// placeGlyph returns where a shaped glyph goes relative to the pen: along the
// line and across it. Across is down from the baseline in horizontal text, and
// right of the column center in vertical text.
//
// A sideways glyph was shaped horizontally and is drawn turned 90° clockwise,
// so its up points right. Its em box is centered on the column.
func placeGlyph(gl *shaping.Glyph, vertical, sideways bool, emCenter float32) (inline, cross, advance float32) {
	switch {
	case sideways:
		return toFloat(gl.XOffset), toFloat(gl.YOffset) - emCenter, toFloat(gl.Advance)
	case vertical:
		// HarfBuzz moves the glyph from its vertical origin, at the top
		// center of the em box, to its horizontal origin.
		return -toFloat(gl.YOffset), toFloat(gl.XOffset), -toFloat(gl.Advance)
	}
	return toFloat(gl.XOffset), -toFloat(gl.YOffset), toFloat(gl.Advance)
}

// tcyFeatures are the OpenType features that make two or three digits fit an
// em: half widths and third widths.
func tcyFeatures(n int) []shaping.FontFeature {
	switch n {
	case 2:
		return []shaping.FontFeature{{Tag: ot.NewTag('h', 'w', 'i', 'd'), Value: 1}}
	case 3:
		return []shaping.FontFeature{{Tag: ot.NewTag('t', 'w', 'i', 'd'), Value: 1}}
	}
	return nil
}

// shapeTateChuYoko sets the runes from first to last horizontally within one em
// of vertical text (JLReq 3.4.3). They become one cluster that follows the
// rules of a kanji. Text wider than the em shrinks to fit. It reports false,
// leaving the paragraph as it was, if the text needs more than one font.
func (p *Paragraph) shapeTateChuYoko(bt *blockText, i, first, last int) bool {
	s := p.shaper
	sec := &p.sections[i]
	em := sec.style.Size
	input := shaping.Input{
		Text:         bt.runes[first:last],
		RunEnd:       last - first,
		Direction:    di.DirectionLTR,
		Size:         fixed.Int26_6(em * 64),
		Language:     s.language,
		FontFeatures: tcyFeatures(last - first),
	}
	runs := s.split.Split(input, s.fonts)
	if len(runs) != 1 {
		return false
	}
	var hb *shaping.HarfbuzzShaper
	runs[0].Face, hb = s.varied(runs[0].Face, &sec.style)
	out := hb.Shape(runs[0])
	w := toFloat(out.Advance)
	scale := float32(1)
	if w > em {
		scale = em / w
	}
	// The em box of the scaled text is centered in the em of the column.
	baseline := em/2 + p.emCenter*em*scale
	glyphStart := len(p.glyphs)
	var pen float32
	for k := range out.Glyphs {
		gl := &out.Glyphs[k]
		p.glyphs = append(p.glyphs, shapedGlyph{
			id:     uint32(gl.GlyphID),
			inline: baseline - toFloat(gl.YOffset)*scale,
			cross:  (pen + toFloat(gl.XOffset) - w/2) * scale,
		})
		pen += toFloat(gl.Advance)
	}
	p.clusters = append(p.clusters, cluster{
		start:      bt.offsets[first],
		end:        bt.offsets[last],
		glyphStart: glyphStart,
		glyphEnd:   len(p.glyphs),
		face:       s.faceID(out.Face),
		section:    i,
		size:       em * scale,
	})
	p.addUnit(bt, first, last, em, em, true)
	return true
}
