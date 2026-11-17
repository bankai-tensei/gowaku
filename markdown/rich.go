package markdown

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
)

const (
	// defaultTextSize is the size text.Style takes for zero.
	defaultTextSize = 14
	// codeRoundness rounds the ground of a code span.
	codeRoundness = 3
	// ringWidth is the width of the focus ring around a link, and ringRadius
	// its corners.
	ringWidth, ringRadius = 3, 4
)

// richColors are the colors a paragraph draws besides its glyphs.
type richColors struct {
	link, code, focus, rule geom.Color
}

// richText is a paragraph of spans with the decorations of Markdown: the
// ground of code spans, strikethrough lines, and links that underline on
// hover. It is RichText with what RichText cannot do: the links are child
// objects over their part of the text, which take the pointer, the focus and
// the place of the link in the accessibility tree.
type richText struct {
	shaper *text.Shaper
	spans  []text.Span
	style  text.Style
	links  []linkSpan
	decos  []deco
	colors richColors
	// silent keeps the text out of the accessibility tree, for text that a
	// node around it already says.
	silent bool
	// noSelect keeps the text out of a field.SelectionArea, for the numbers
	// of list items and the language of a code block. brk is what a copy
	// puts before the text (see SelectionBreak), and anchor the slug of a
	// heading.
	noSelect    bool
	brk, anchor string
	// children are the targets of the links that can be followed.
	children []core.Widget
}

// Key implements core.Widget.
func (richText) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t richText) CreateRenderObject() core.RenderObject {
	return &renderRich{
		shaper: t.shaper, spans: t.spans, style: t.style, links: t.links, decos: t.decos,
		colors: t.colors, silent: t.silent, noSelect: t.noSelect, brk: t.brk, anchor: t.anchor,
	}
}

// UpdateRenderObject implements core.RenderWidget.
func (t richText) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderRich)
	if !slices.Equal(r.spans, t.spans) || !r.style.SameLayout(t.style) || r.style.Align != t.style.Align ||
		!slices.Equal(r.links, t.links) || !slices.Equal(r.decos, t.decos) || r.shaper != t.shaper {
		r.shaper, r.spans, r.style, r.links, r.decos = t.shaper, t.spans, t.style, t.links, t.decos
		r.shaped, r.laidOut, r.spare.laidOut = nil, nil, nil
		r.MarkNeedsLayout()
	}
	if r.style.Color != t.style.Color || r.colors != t.colors {
		r.style.Color, r.colors = t.style.Color, t.colors
		r.MarkNeedsPaint()
	}
	r.silent, r.noSelect, r.brk, r.anchor = t.silent, t.noSelect, t.brk, t.anchor
}

// ChildWidgets implements core.MultiChildWidget.
func (t richText) ChildWidgets() []core.Widget { return t.children }

// fragment is the part of a link on one line: the box that takes the
// pointer, and the underline below it.
type fragment struct {
	box, line geom.Rect
}

// renderRich lays out and paints a richText. The geometry of the
// decorations is worked out at layout, so that painting does not allocate.
type renderRich struct {
	core.RenderBase
	shaper *text.Shaper
	spans  []text.Span
	style  text.Style
	links  []linkSpan
	decos  []deco
	colors richColors
	silent bool

	noSelect    bool
	brk, anchor string
	// selection is set only under a field.SelectionArea (selection.go).
	selection *richSelection

	children []core.RenderObject
	shaped   *text.Paragraph
	// minW is the least width of shaped, or negative until it is asked for.
	minW    float32
	laidOut *text.Layout
	// width is the line length laidOut was broken at.
	width float32
	// shift moves the text right within a box wider than the text, as Align
	// asks.
	shift float32
	// grounds are the grounds of code spans, keys those of <kbd> and strikes
	// the strikethrough lines; frags holds the fragments of each link. All
	// are relative to the text. shifts moves each run of glyphs down, for
	// <sub>, or up, for <sup>; it is empty while there are none.
	grounds, keys, strikes []geom.Rect
	frags                  [][]fragment
	shifts                 []float32
	// spare is the layout at the width before, with its decorations.
	spare         richGeometry
	scratch       []geom.Rect
	ring, keyLine paint.Stroke
}

// richGeometry is a layout of a renderRich at one width with the geometry of
// its decorations.
type richGeometry struct {
	laidOut                *text.Layout
	width                  float32
	grounds, keys, strikes []geom.Rect
	frags                  [][]fragment
	shifts                 []float32
}

// swap trades the current layout for the spare one.
func (r *renderRich) swap() {
	s := &r.spare
	r.laidOut, s.laidOut = s.laidOut, r.laidOut
	r.width, s.width = s.width, r.width
	r.grounds, s.grounds = s.grounds, r.grounds
	r.keys, s.keys = s.keys, r.keys
	r.strikes, s.strikes = s.strikes, r.strikes
	r.frags, s.frags = s.frags, r.frags
	r.shifts, s.shifts = s.shifts, r.shifts
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderRich) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderRich) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderRich) ChildAt(i int) core.RenderObject { return r.children[i] }

// minWidth returns the width below which the text would have to break inside
// a word. It is valid after a layout.
func (r *renderRich) minWidth() float32 {
	if r.minW < 0 {
		r.minW = r.shaped.MinLength()
	}
	return r.minW
}

// Layout implements core.RenderObject.
func (r *renderRich) Layout(c geom.Constraints) geom.Size {
	if r.shaped == nil {
		if len(r.spans) == 0 {
			r.shaped = r.shaper.Shape("", r.style)
		} else {
			r.shaped = r.shaper.ShapeSpans(r.spans, r.style)
		}
		r.minW = -1
		r.laidOut, r.spare.laidOut = nil, nil
	}
	if r.laidOut == nil || r.width != c.MaxW {
		// A table lays its cells out twice, unbounded and then at the width
		// of the column: the layout of the other width is kept, so that
		// laying the table out again allocates nothing.
		hit := r.spare.laidOut != nil && r.spare.width == c.MaxW
		r.swap()
		if !hit {
			r.laidOut, r.width = r.shaped.Layout(c.MaxW), c.MaxW
			r.measureDecorations()
		}
	}
	natural := r.laidOut.Size()
	sz := c.Constrain(natural)
	switch r.style.Align {
	case text.AlignCenter:
		r.shift = (sz.W - natural.W) / 2
	case text.AlignEnd:
		r.shift = sz.W - natural.W
	default:
		r.shift = 0
	}
	shift := geom.Offset{X: r.shift}
	for _, ch := range r.children {
		l := ch.(*renderLink)
		l.boxes = l.boxes[:0]
		if i := l.index; i < len(r.frags) {
			for _, f := range r.frags[i] {
				l.boxes = append(l.boxes, f.box.Translate(shift))
			}
		}
		var first geom.Rect
		if len(l.boxes) > 0 {
			first = l.boxes[0]
		}
		core.LayoutChild(l, geom.Tight(first.Size()))
		l.SetOffset(first.Origin())
	}
	return sz
}

// measureDecorations works out where the decorations of the current layout
// go.
func (r *renderRich) measureDecorations() {
	em := r.style.Size
	if em <= 0 {
		em = defaultTextSize
	}
	thick := max(1, evenPixels(em/14))
	r.grounds, r.keys, r.strikes, r.shifts = r.grounds[:0], r.keys[:0], r.strikes[:0], r.shifts[:0]
	for _, d := range r.decos {
		r.scratch = r.laidOut.SelectionRects(d.start, d.end, r.scratch[:0])
		for _, box := range r.scratch {
			base := r.baseline(box, em)
			switch d.kind {
			case decoCode:
				r.grounds = append(r.grounds, geom.Rect{X: box.X - 2, Y: base - 0.92*em, W: box.W + 4, H: 1.2 * em})
			case decoKey:
				r.keys = append(r.keys, geom.Rect{X: box.X - 2, Y: base - 0.95*em, W: box.W + 4, H: 1.25 * em})
			case decoStrike:
				r.strikes = append(r.strikes, geom.Rect{X: box.X, Y: base - 0.3*em - thick/2, W: box.W, H: thick})
			case decoSub:
				r.shiftRuns(box, 0.2*em)
			case decoSup:
				r.shiftRuns(box, -0.4*em)
			}
		}
	}
	r.frags = r.frags[:0]
	for _, l := range r.links {
		r.scratch = r.laidOut.SelectionRects(l.start, l.end, r.scratch[:0])
		frags := make([]fragment, 0, len(r.scratch))
		for _, box := range r.scratch {
			base := r.baseline(box, em)
			line := geom.Rect{X: box.X, Y: base + max(1, em*0.1), W: box.W, H: thick}
			frags = append(frags, fragment{box: box, line: line})
		}
		r.frags = append(r.frags, frags)
	}
}

// shiftRuns moves the runs of glyphs that start within box, the part of a
// <sub> or a <sup> on one line, by dy. The text has no baseline shift of its
// own, so the runs move when they are painted. See
// docs/ADR/0119-markdown-html-subset.md.
func (r *renderRich) shiftRuns(box geom.Rect, dy float32) {
	runs := r.laidOut.Runs()
	if len(r.shifts) < len(runs) {
		r.shifts = append(r.shifts[:0], make([]float32, len(runs))...)
	}
	for i := range runs {
		run := &runs[i]
		x := run.Origin.X
		if len(run.Glyphs) > 0 {
			x += run.Glyphs[0].X
		}
		if x+0.5 >= box.X && x+0.5 <= box.X+box.W && run.Origin.Y >= box.Y && run.Origin.Y <= box.Y+box.H {
			r.shifts[i] = dy
		}
	}
}

// baseline returns the baseline of the line that box lies on: the origin of
// a run of glyphs on it, or an estimate for a line without glyphs.
func (r *renderRich) baseline(box geom.Rect, em float32) float32 {
	runs := r.laidOut.Runs()
	for i := range runs {
		if y := runs[i].Origin.Y; y >= box.Y && y <= box.Y+box.H {
			return y
		}
	}
	return box.Y + box.H/2 + 0.38*em
}

// Paint implements core.RenderObject.
func (r *renderRich) Paint(cv *paint.Canvas, off geom.Offset) {
	off.X += r.shift
	for _, g := range r.grounds {
		cv.DrawRRect(geom.RRectAll(g.Translate(off), codeRoundness), paint.Paint{Color: r.colors.code})
	}
	if len(r.keys) > 0 {
		r.keyLine = paint.Stroke{Width: 1, Color: r.colors.rule}
		for _, k := range r.keys {
			rr := geom.RRectAll(k.Translate(off), codeRoundness)
			cv.DrawRRect(rr, paint.Paint{Color: r.colors.code})
			cv.DrawRRect(rr, paint.Paint{Stroke: &r.keyLine})
		}
	}
	if r.selection != nil {
		r.paintSelection(cv, off)
	}
	color := r.style.PaintColor()
	runs := r.laidOut.Runs()
	for i := range runs {
		o := off
		if i < len(r.shifts) {
			o.Y += r.shifts[i]
		}
		cv.DrawGlyphs(&runs[i], o, color)
	}
	for _, s := range r.strikes {
		cv.DrawRRect(geom.RRect{Rect: s.Translate(off)}, paint.Paint{Color: color})
	}
	r.ring = paint.Stroke{Width: ringWidth, Color: r.colors.focus}
	for _, ch := range r.children {
		l := ch.(*renderLink)
		if (!l.hovered && !l.shown) || l.index >= len(r.frags) {
			continue
		}
		for _, f := range r.frags[l.index] {
			cv.DrawRRect(geom.RRect{Rect: f.line.Translate(off)}, paint.Paint{Color: r.colors.link})
			if l.shown {
				ring := geom.Rect{X: f.box.X - ringWidth, Y: f.box.Y - ringWidth/2, W: f.box.W + 2*ringWidth, H: f.box.H + ringWidth}
				cv.DrawRRect(geom.RRectAll(ring.Translate(off), ringRadius), paint.Paint{Stroke: &r.ring})
			}
		}
	}
}

// Semantics implements core.RenderObject. The text is one node, and each link
// one after it.
func (r *renderRich) Semantics(sb *core.SemanticsBuilder) {
	if !r.silent && r.shaped != nil && r.shaped.Text() != "" {
		sb.Open(semantics.Node{Role: semantics.RoleText, Label: r.shaped.Text()}, r.Size())
		sb.Close()
	}
	for _, ch := range r.children {
		sb.Visit(ch)
	}
}

func (r *renderRich) headingSlug() string { return r.anchor }

func (r *renderRich) ready() bool { return r.laidOut != nil }

// Text returns the text, with the spans joined.
func (r *renderRich) Text() string {
	if r.shaped == nil {
		return ""
	}
	return r.shaped.Text()
}
