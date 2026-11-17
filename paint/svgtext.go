package paint

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/geom"
)

// The text of an SVG is set by the caller: paint knows no fonts, and the text
// package, which does, imports paint. An SVG parsed without a TextOutliner
// leaves its text out and counts it in Skipped; SetText parses it again with
// one, and text then becomes the outlines of its glyphs, painted like any
// other shape: gradients, strokes, clip paths, masks and filters apply. See
// docs/ADR/0113-svg-filters-text.md.

// SVGFont is the font of a run of SVG text.
type SVGFont struct {
	// Family is the first family that font-family names, without quotes.
	// Generic families such as sans-serif are passed on as they are.
	Family string
	// Size is the font size in the units of the drawing.
	Size   float32
	Weight uint16
	Italic bool
	// LetterSpacing is added after every glyph.
	LetterSpacing float32
}

// TextMetrics measures a run of text set by a TextOutliner.
type TextMetrics struct {
	// Advance is the width of the run, letter spacing included.
	Advance float32
	// Ascent and Descent are the distances of the font above and below the
	// baseline, both positive.
	Ascent, Descent float32
}

// TextOutliner sets text for SVG. *text.Shaper is one.
type TextOutliner interface {
	// OutlineText sets content on one line in font and calls glyph with the
	// outline of each glyph, a path to fill with the non-zero rule that the
	// callee keeps, with the origin of the line at the origin, on the
	// baseline, and y pointing down. It returns the metrics of the run.
	OutlineText(content string, font SVGFont, glyph func(outline *Path)) TextMetrics
}

// svgTextState is the text style in effect at an element.
type svgTextState struct {
	family        string
	size          float32
	weight        uint16
	italic        bool
	letterSpacing float32
	anchor        uint8 // 0 start, 1 middle, 2 end
	baseline      string
}

// svgTextPiece is a run of characters set together: the first one may have a
// position of its own, and the rest follow it.
type svgTextPiece struct {
	text   string
	st     svgState
	x, y   float32
	hasX   bool
	hasY   bool
	dx, dy float32
	// glyphs are the outlines, and adv the advance, once set.
	glyphs []*Path
	m      TextMetrics
}

// svgTextLists are the per-character positions of a text or tspan element.
type svgTextLists struct {
	x, y, dx, dy []float32
	at           int
}

// textElement draws text element n with state st.
func (p *svgDocParser) textElement(n *svgNode, st *svgState) error {
	p.svg.hasText = true
	// The text is read even without an outliner, so that what does not
	// parse is an error of ParseSVG and not of SetText.
	pieces, err := p.textPieces(n, st)
	if err != nil {
		return err
	}
	if p.outliner == nil {
		p.svg.Skipped++
		return nil
	}
	p.layoutText(pieces)
	// The bounding box of the whole text, in its own coordinates, is what
	// gradients in its units refer to.
	first := true
	var box geom.Rect
	for i := range pieces {
		for _, g := range pieces[i].glyphs {
			b := g.Bounds()
			if first {
				box, first = b, false
				continue
			}
			box = unionRect(box, b)
		}
	}
	for i := range pieces {
		pc := &pieces[i]
		for _, g := range pc.glyphs {
			p.emitGlyph(&pc.st, g, box)
		}
	}
	return nil
}

func unionRect(a, b geom.Rect) geom.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// emitGlyph adds the fill and the stroke of one glyph, with box as the
// bounding box of its text.
func (p *svgDocParser) emitGlyph(st *svgState, g *Path, box geom.Rect) {
	if !st.visible || st.ctm.Det() == 0 {
		return
	}
	p.geoms = append(p.geoms, svgGeom{path: g, ctm: st.ctm})
	for _, step := range st.order {
		switch step {
		case svgPaintFill:
			if st.fill.kind != svgPaintNone {
				p.add(st, st.fill, st.fillOpacity, g.transformed(st.ctm), box)
			}
		case svgPaintStroke:
			if st.stroke.kind == svgPaintNone || st.strokeWidth <= 0 {
				continue
			}
			tol := p.tol / maxScale(st.ctm)
			stroked := g
			if st.dashes != nil {
				stroked = dash(g, st.dashes, st.dashOffset, 1, tol)
			}
			outline := stroked.Stroke(StrokeStyle{Width: st.strokeWidth, Cap: st.cap, Join: st.join, MiterLimit: st.miterLimit, Tolerance: tol})
			if !outline.Empty() {
				p.add(st, st.stroke, st.strokeOpacity, outline.transformed(st.ctm), box)
			}
		}
	}
}

// clipText adds the outlines of text element n to clip.
func (p *svgDocParser) clipText(clip *svgClip, n *svgNode, st *svgState) error {
	p.svg.hasText = true
	pieces, err := p.textPieces(n, st)
	if err != nil {
		return err
	}
	if p.outliner == nil {
		p.svg.Skipped++
		return nil
	}
	p.layoutText(pieces)
	for i := range pieces {
		for _, g := range pieces[i].glyphs {
			q := g.transformed(pieces[i].st.ctm)
			q.Rule = FillNonZero
			clip.paths = append(clip.paths, q)
		}
	}
	return nil
}

// textPieces walks text element n and returns its characters in pieces, each
// with the state of the element that holds it.
func (p *svgDocParser) textPieces(n *svgNode, st *svgState) ([]svgTextPiece, error) {
	var pieces []svgTextPiece
	var lists []*svgTextLists
	space := true // the last character was a space, or there was none
	var walk func(n *svgNode, st *svgState) error
	walk = func(n *svgNode, st *svgState) error {
		l, err := textLists(n, st.viewport)
		if err != nil {
			return fmt.Errorf("<%s>: %w", n.name, err)
		}
		lists = append(lists, l)
		defer func() { lists = lists[:len(lists)-1] }()
		for _, c := range n.children {
			switch {
			case c.name == "":
				for _, r := range collapseSpace(c.text, &space) {
					pc := svgTextPiece{text: string(r), st: *st}
					explicit := false
					// The innermost element with a value for this
					// character supplies it; every element counts it.
					var hasDX, hasDY bool
					for i := len(lists) - 1; i >= 0; i-- {
						ls := lists[i]
						if !pc.hasX && ls.at < len(ls.x) {
							pc.x, pc.hasX = ls.x[ls.at], true
						}
						if !pc.hasY && ls.at < len(ls.y) {
							pc.y, pc.hasY = ls.y[ls.at], true
						}
						if !hasDX && ls.at < len(ls.dx) {
							pc.dx, hasDX = ls.dx[ls.at], true
						}
						if !hasDY && ls.at < len(ls.dy) {
							pc.dy, hasDY = ls.dy[ls.at], true
						}
					}
					for _, ls := range lists {
						ls.at++
					}
					explicit = pc.hasX || pc.hasY || hasDX || hasDY
					if k := len(pieces) - 1; !explicit && k >= 0 && sameTextStyle(&pieces[k].st, st) {
						pieces[k].text += pc.text
						continue
					}
					pieces = append(pieces, pc)
				}
			case c.foreign:
			case c.name == "tspan" || c.name == "a":
				if !p.visit() {
					continue
				}
				cst, err := p.style(st, c)
				if err != nil {
					return fmt.Errorf("<%s>: %w", c.name, err)
				}
				if v, _ := c.prop("display"); v == "none" {
					continue
				}
				if err := walk(c, &cst); err != nil {
					return err
				}
			case c.name == "textPath":
				// Text on a path is not drawn.
				p.svg.Skipped++
			}
		}
		return nil
	}
	if err := walk(n, st); err != nil {
		return nil, err
	}
	// Trailing space at the end of the text is dropped.
	if k := len(pieces) - 1; k >= 0 {
		pieces[k].text = strings.TrimRight(pieces[k].text, " ")
	}
	return pieces, nil
}

// sameTextStyle reports whether text in b can join a run in a.
func sameTextStyle(a, b *svgState) bool {
	return a.text == b.text && a.fill == b.fill && a.stroke == b.stroke && a.fillOpacity == b.fillOpacity &&
		a.strokeOpacity == b.strokeOpacity && a.strokeWidth == b.strokeWidth && a.ctm == b.ctm && a.visible == b.visible &&
		a.color == b.color
}

// collapseSpace returns s with line breaks removed, tabs as spaces, and runs
// of spaces as one, as xml:space="default" asks. space carries whether the
// character before was a space, so that a leading space is dropped too.
func collapseSpace(s string, space *bool) []rune {
	out := make([]rune, 0, utf8.RuneCountInString(s))
	for _, r := range s {
		switch r {
		case '\n', '\r':
			continue
		case '\t':
			r = ' '
		}
		if r == ' ' {
			if *space {
				continue
			}
			*space = true
		} else {
			*space = false
		}
		out = append(out, r)
	}
	return out
}

// textLists reads x, y, dx and dy of a text or tspan element.
func textLists(n *svgNode, vp geom.Size) (*svgTextLists, error) {
	l := &svgTextLists{}
	read := func(name string, ref float32) ([]float32, error) {
		v := strings.TrimSpace(n.attr(name))
		if v == "" {
			return nil, nil
		}
		var out []float32
		for f := range strings.FieldsFuncSeq(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
			x, err := parseLength(f, ref)
			if err != nil {
				return nil, err
			}
			out = append(out, x)
		}
		return out, nil
	}
	var err error
	if l.x, err = read("x", vp.W); err != nil {
		return nil, err
	}
	if l.y, err = read("y", vp.H); err != nil {
		return nil, err
	}
	if l.dx, err = read("dx", vp.W); err != nil {
		return nil, err
	}
	if l.dy, err = read("dy", vp.H); err != nil {
		return nil, err
	}
	return l, nil
}

// layoutText sets the pieces and places them: each one starts where the one
// before ended, unless it has a position of its own, and a piece with an x or
// a y starts a chunk, which text-anchor aligns as a whole.
func (p *svgDocParser) layoutText(pieces []svgTextPiece) {
	var pen geom.Offset
	chunk := 0
	finish := func(end int) {
		if chunk >= end {
			return
		}
		anchor := pieces[chunk].st.text.anchor
		if anchor == 0 {
			return
		}
		lo, hi := pieces[chunk].x, pieces[chunk].x
		for i := chunk; i < end; i++ {
			lo = min(lo, pieces[i].x)
			hi = max(hi, pieces[i].x+pieces[i].m.Advance)
		}
		shift := -(hi - lo)
		if anchor == 1 {
			shift /= 2
		}
		for i := chunk; i < end; i++ {
			pieces[i].x += shift
		}
	}
	for i := range pieces {
		pc := &pieces[i]
		if pc.hasX || pc.hasY {
			finish(i)
			chunk = i
		}
		if pc.hasX {
			pen.X = pc.x
		}
		if pc.hasY {
			pen.Y = pc.y
		}
		pen.X += pc.dx
		pen.Y += pc.dy
		ts := &pc.st.text
		font := SVGFont{Family: ts.family, Size: ts.size, Weight: ts.weight, Italic: ts.italic, LetterSpacing: ts.letterSpacing}
		pc.m = p.outliner.OutlineText(pc.text, font, func(g *Path) { pc.glyphs = append(pc.glyphs, g) })
		pc.x, pc.y = pen.X, pen.Y
		pen.X += pc.m.Advance
	}
	finish(len(pieces))
	for i := range pieces {
		pc := &pieces[i]
		m := geom.Translation(geom.Offset{X: pc.x, Y: pc.y + baselineShift(pc.st.text.baseline, pc.st.text.size, pc.m)})
		for k, g := range pc.glyphs {
			pc.glyphs[k] = g.transformed(m)
			pc.glyphs[k].Rule = FillNonZero
		}
	}
}

// baselineShift is how far dominant-baseline moves glyphs down from the
// alphabetic baseline.
func baselineShift(baseline string, size float32, m TextMetrics) float32 {
	switch baseline {
	case "middle":
		// Half the x-height, which the outliner does not report: that of
		// common Latin fonts is about half an em.
		return 0.26 * size
	case "central":
		return (m.Ascent - m.Descent) / 2
	case "hanging":
		return 0.8 * m.Ascent
	case "text-before-edge", "text-top":
		return m.Ascent
	case "text-after-edge", "text-bottom", "ideographic":
		return -m.Descent
	case "mathematical":
		return 0.5 * m.Ascent
	}
	return 0
}

// textProperty applies a property of text, and reports whether name is one.
func textProperty(st, parent *svgState, name, value string) bool {
	ts := &st.text
	switch name {
	case "font-family":
		f := value
		if i := strings.IndexByte(f, ','); i >= 0 {
			f = f[:i]
		}
		ts.family = strings.Trim(strings.TrimSpace(f), `"'`)
	case "font-size":
		ref := parent.text.size
		switch value {
		case "xx-small":
			ts.size = 9
		case "x-small":
			ts.size = 10
		case "small":
			ts.size = 13
		case "medium":
			ts.size = 16
		case "large":
			ts.size = 18
		case "x-large":
			ts.size = 24
		case "xx-large":
			ts.size = 32
		case "larger":
			ts.size = ref * 1.2
		case "smaller":
			ts.size = ref / 1.2
		default:
			if v, ok := strings.CutSuffix(value, "em"); ok && !strings.HasSuffix(v, "r") {
				if x, err := parseNumber(v); err == nil {
					ts.size = x * ref
				}
			} else if x, err := parseLength(value, ref); err == nil && x >= 0 {
				ts.size = x
			}
		}
	case "font-weight":
		switch value {
		case "normal":
			ts.weight = 400
		case "bold":
			ts.weight = 700
		case "bolder":
			ts.weight = min(parent.text.weight+300, 900)
		case "lighter":
			ts.weight = max(parent.text.weight, 400) - 300
		default:
			if x, err := parseNumber(value); err == nil && x >= 1 && x <= 1000 {
				ts.weight = uint16(x)
			}
		}
	case "font-style":
		ts.italic = value == "italic" || value == "oblique"
	case "letter-spacing":
		if value == "normal" {
			ts.letterSpacing = 0
		} else if x, err := parseLength(value, ts.size); err == nil {
			ts.letterSpacing = x
		}
	case "text-anchor":
		switch value {
		case "start":
			ts.anchor = 0
		case "middle":
			ts.anchor = 1
		case "end":
			ts.anchor = 2
		}
	case "dominant-baseline", "alignment-baseline":
		ts.baseline = value
	default:
		return false
	}
	return true
}
