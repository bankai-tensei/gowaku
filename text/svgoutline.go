package text

import (
	ot "github.com/go-text/typesetting/font/opentype"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// OutlineText implements paint.TextOutliner, so that the text of an SVG can be
// set with a Shaper: it shapes content on one line in the style of font and
// hands each glyph to glyph as an outline, with the origin of the line on the
// baseline and y down. Glyphs without an outline, such as color emoji, are
// left out. letterSpacing in font is added after every glyph. See
// docs/ADR/0113-svg-filters-text.md.
func (s *Shaper) OutlineText(content string, font paint.SVGFont, glyph func(outline *paint.Path)) paint.TextMetrics {
	st := Style{Family: font.Family, Size: font.Size, Weight: font.Weight, Italic: font.Italic, LineHeight: 1}
	if st.Size <= 0 {
		return paint.TextMetrics{}
	}
	layout := s.Shape(content, st).Layout(1 << 24)
	runs := layout.Runs()
	var m paint.TextMetrics
	if len(runs) == 0 {
		return m
	}
	baseline := runs[0].Origin.Y
	if f := s.faces[runs[0].Face]; f != nil {
		if extents, ok := f.FontHExtents(); ok {
			unit := font.Size / float32(f.Upem())
			m.Ascent, m.Descent = extents.Ascender*unit, -extents.Descender*unit
		}
	}
	if m.Ascent == 0 && m.Descent == 0 {
		m.Ascent, m.Descent = font.Size*0.88, font.Size*0.12
	}
	count := 0
	// The advance runs to the end of the last glyph: the width of the line
	// leaves out trailing spaces, which matter between runs of SVG text.
	end := float32(0)
	for i := range runs {
		run := &runs[i]
		face := s.faces[run.Face]
		unit := run.Size / float32(face.Upem())
		slanted := s.slanted[run.Face]
		for _, g := range run.Glyphs {
			shift := float32(count) * font.LetterSpacing
			count++
			end = max(end, run.Origin.X+g.X+face.HorizontalAdvance(ot.GID(g.ID))*unit)
			outline, ok := face.GlyphDataOutline(ot.GID(g.ID))
			if !ok || len(outline.Segments) == 0 {
				continue
			}
			origin := geom.Offset{X: run.Origin.X + g.X + shift, Y: run.Origin.Y - baseline + g.Y}
			at := func(p ot.SegmentPoint) (float32, float32) {
				x := p.X
				if slanted {
					// The conversion keeps arm64 from fusing this into
					// FMA, as in the atlas; see ADR 0086.
					x = p.X + float32(obliqueShear*p.Y)
				}
				return origin.X + x*unit, origin.Y - p.Y*unit
			}
			path := &paint.Path{}
			open := false
			for k := range outline.Segments {
				seg := &outline.Segments[k]
				switch seg.Op {
				case ot.SegmentOpMoveTo:
					if open {
						path.Close()
					}
					x, y := at(seg.Args[0])
					path.MoveTo(x, y)
					open = true
				case ot.SegmentOpLineTo:
					x, y := at(seg.Args[0])
					path.LineTo(x, y)
				case ot.SegmentOpQuadTo:
					cx, cy := at(seg.Args[0])
					x, y := at(seg.Args[1])
					path.QuadTo(cx, cy, x, y)
				case ot.SegmentOpCubeTo:
					ax, ay := at(seg.Args[0])
					bx, by := at(seg.Args[1])
					x, y := at(seg.Args[2])
					path.CubeTo(ax, ay, bx, by, x, y)
				}
			}
			if open {
				path.Close()
			}
			glyph(path)
		}
	}
	m.Advance = max(end, layout.Size().W) + float32(count)*font.LetterSpacing
	return m
}
