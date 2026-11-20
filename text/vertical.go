package text

import (
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
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

// keepWideUpright settles the orientation of each character of vertical text.
// The segmenter decides by script (UAX #24), not by character: a character of
// the Common script, such as a dash, stands upright between kana and turns
// sideways next to Latin, whatever UAX #50 says of it.
//
//   - Wide and full-width characters stand upright. The segmenter turns those
//     of UAX #50 class Tr, such as ー, 〜 and （, sideways, but Japanese fonts
//     have vertical forms for them and `vert` picks them up.
//   - Dashes, leaders and quotes (see [byVerticalForm]) stand upright where the
//     font has a vertical form for them and turn sideways where it does not,
//     wherever they stand.
//   - Everything else keeps the segmenter's choice.
func keepWideUpright(runs []shaping.Input) []shaping.Input {
	out := make([]shaping.Input, 0, len(runs))
	for _, run := range runs {
		sub := run
		for i := run.RunStart; i < run.RunEnd; i++ {
			sideways := run.Direction.IsSideways()
			if r := run.Text[i]; isWide(r) {
				sideways = false
			} else if byVerticalForm(r) {
				sideways = !hasVerticalForm(run.Face, r)
			}
			if i == run.RunStart {
				sub.Direction.SetSideways(sideways)
				continue
			}
			if sideways != sub.Direction.IsSideways() {
				sub.RunEnd = i
				out = append(out, sub)
				sub.RunStart = i
				sub.Direction.SetSideways(sideways)
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

// byVerticalForm reports whether r stands upright in vertical text only if the
// font has a vertical form for it. These are narrow or ambiguous width, so
// [isWide] misses them, and Japanese text uses them all the time.
//
// UAX #50 turns the leaders … ‥ and the dashes ― — – ‐ sideways (class R), but
// Japanese typesetting (JLReq) sets leaders and dashes upright in their
// vertical forms. Japanese fonts have those forms for … ‥ ―, and some fonts
// for — – ‐ too (Yu Gothic has none for — and –). Turned sideways where the
// font has no vertical form, a dash still runs along the column; upright in
// its horizontal form it would cross it. This holds in an Upright span too.
//
// The quotes “ ” ‘ ’ are UAX #50 class Tr: the vertical form where the font has
// one, else turned. Japanese vertical text usually writes 〝 〟 instead, which
// are wide and always stand upright; the curly quotes follow UAX #50.
func byVerticalForm(r rune) bool {
	switch r {
	case '…', '‥', '―', '—', '–', '‐', '“', '”', '‘', '’':
		return true
	}
	return false
}

// vertTag is the OpenType feature that holds vertical forms, the one HarfBuzz
// applies to upright vertical text. `vrt2` is left out: its glyphs are the
// horizontal ones turned, as a sideways run draws them anyway, and Yu Gothic
// has them for “ and — though it has no vertical forms for them.
var vertTag = ot.NewTag('v', 'e', 'r', 't')

// hasVerticalForm reports whether face substitutes a vertical form for r under
// `vert`.
func hasVerticalForm(face *font.Face, r rune) bool {
	if face == nil {
		return false
	}
	gid, ok := face.NominalGlyph(r)
	if !ok {
		return false
	}
	gsub := &face.GSUB
	for i := range gsub.Features {
		f := &gsub.Features[i]
		if f.Tag != vertTag {
			continue
		}
		for _, l := range f.LookupListIndices {
			if int(l) >= len(gsub.Lookups) {
				continue
			}
			for _, st := range gsub.Lookups[l].Subtables {
				if _, ok := st.(tables.SingleSubs); !ok {
					continue
				}
				if _, ok := st.Cov().Index(tables.GlyphID(gid)); ok {
					return true
				}
			}
		}
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
