package markdown

import (
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// pictureGap is the space between pictures that the source separates with
// white space, as the badges of a README.
const pictureGap = theme.Space4

// inlineBlocks builds inline content aligned as al says. Pictures that stand
// on their own, with nothing but white space between them, go in rows as
// wide as the document; the text before, between and after them makes
// paragraphs. An image that shows no picture shows its alternative text in
// the text around it, or on its own where it stands alone. See
// docs/ADR/0119-markdown-html-subset.md.
func (b *builder) inlineBlocks(ins []inline, style text.Style, al align) []core.Widget {
	style.Align = textAlign(al, style.Align)
	var pics []core.Widget
	for i := range ins {
		img, dest, ok := standaloneImage(&ins[i])
		if !ok || !b.shown(img) {
			continue
		}
		if w := b.pictureItem(img, dest); w != nil {
			if pics == nil {
				pics = make([]core.Widget, len(ins))
			}
			pics[i] = w
		}
	}
	if pics == nil {
		return b.textBlocks(ins, style, al, true)
	}
	var out []core.Widget
	start := 0
	for i := 0; i < len(ins); {
		if pics[i] == nil {
			i++
			continue
		}
		row := []core.Widget{pics[i]}
		spaced := false
		last := i
		for k := i + 1; k < len(ins); k++ {
			if pics[k] != nil {
				row = append(row, pics[k])
				last = k
				continue
			}
			if !b.isGap(&ins[k]) {
				break
			}
			spaced = true
		}
		out = append(out, b.textBlocks(ins[start:i], style, al, false)...)
		out = append(out, pictureRow(row, spaced, al))
		start, i = last+1, last+1
	}
	return append(out, b.textBlocks(ins[start:], style, al, false)...)
}

// isGap reports whether in shows only white space within a row of pictures:
// a space, a soft line break, or an image the theme hides.
func (b *builder) isGap(in *inline) bool {
	switch in.kind {
	case inlineSoftBreak:
		return true
	case inlineText:
		return strings.TrimSpace(in.text) == ""
	}
	img, _, ok := standaloneImage(in)
	return ok && !b.shown(img)
}

// textBlocks builds a run of inline content as a paragraph. A run of a part
// that shows nothing builds nothing, unless it is the whole of its block.
func (b *builder) textBlocks(seg []inline, style text.Style, al align, whole bool) []core.Widget {
	if !whole {
		if !b.visible(seg) {
			return nil
		}
		seg = trimEdges(seg)
	}
	var w core.Widget
	if img, dest, ok := b.loneImage(seg); ok {
		w = b.altImage(img, dest, style)
	} else {
		w = b.rich(seg, style, false)
	}
	return []core.Widget{aligned(w, al)}
}

// loneImage returns the image of seg, if seg is nothing but white space and
// one image, alone or in a link.
func (b *builder) loneImage(seg []inline) (img *inline, dest string, ok bool) {
	for i := range seg {
		if b.isGap(&seg[i]) {
			continue
		}
		if ok {
			return nil, "", false
		}
		if img, dest, ok = standaloneImage(&seg[i]); !ok {
			return nil, "", false
		}
	}
	return img, dest, ok
}

// visible reports whether seg shows anything.
func (b *builder) visible(seg []inline) bool {
	for i := range seg {
		if img, _, ok := standaloneImage(&seg[i]); ok && !b.shown(img) {
			continue
		}
		if hasText(seg[i : i+1]) {
			return true
		}
	}
	return false
}

// textAlign returns the alignment of text for al, or def for none.
func textAlign(al align, def text.Align) text.Align {
	switch al {
	case alignLeft:
		return text.AlignStart
	case alignCenter:
		return text.AlignCenter
	case alignRight:
		return text.AlignEnd
	}
	return def
}

// aligned makes w as wide as the document when al moves it from the start,
// so that its text aligns within that width.
func aligned(w core.Widget, al align) core.Widget {
	if al != alignCenter && al != alignRight {
		return w
	}
	return fullWidth{child: w}
}

// pictureRow lays out pictures in a row that wraps, aligned as al says.
func pictureRow(row []core.Widget, spaced bool, al align) core.Widget {
	if len(row) == 1 && (al == alignNone || al == alignLeft) {
		return row[0]
	}
	main := layout.MainStart
	switch al {
	case alignCenter:
		main = layout.MainCenter
	case alignRight:
		main = layout.MainEnd
	}
	var gap float32
	if spaced {
		gap = pictureGap
	}
	return basic.Wrap{Main: main, Cross: layout.CrossEnd, Size: layout.MainMax, Spacing: gap, RunSpacing: pictureGap, Children: row}
}

// scheme returns the color scheme of the theme.
func (b *builder) scheme() colorScheme {
	if b.dark {
		return schemeDark
	}
	return schemeLight
}

// source returns the address an image shows in the current theme: the first
// <source> of its <picture> for the theme, or else its own.
func (b *builder) source(img *inline) string {
	scheme := b.scheme()
	for _, s := range img.sources {
		if s.scheme == schemeAny || s.scheme == scheme {
			return s.src
		}
	}
	return stripModeOnly(img.dest)
}

// shown reports whether the image shows in the current theme: an address
// that ends in #gh-dark-mode-only or #gh-light-mode-only shows only in that
// theme, as on GitHub.
func (b *builder) shown(img *inline) bool {
	return imageShown(img, b.dark)
}

func imageShown(img *inline, dark bool) bool {
	switch {
	case strings.HasSuffix(img.dest, "#gh-dark-mode-only"):
		return dark
	case strings.HasSuffix(img.dest, "#gh-light-mode-only"):
		return !dark
	}
	return true
}

func stripModeOnly(dest string) string {
	dest = strings.TrimSuffix(dest, "#gh-dark-mode-only")
	return strings.TrimSuffix(dest, "#gh-light-mode-only")
}

// pictureItem returns the picture of an image, in a link to dest if dest is
// set, or nil if the image shows no picture: neither ImageProvider nor
// Images has one for it.
func (b *builder) pictureItem(img *inline, dest string) core.Widget {
	src := b.source(img)
	alt := string(plainText(nil, img.children))
	var w core.Widget
	if b.images != nil {
		if w = b.images(src, alt); w != nil && img.width > 0 && img.height > 0 && !img.widthPercent {
			w = shrinkBox{natural: geom.Size{W: img.width, H: img.height}, child: w}
		}
	}
	if w == nil && b.pictures != nil {
		if f := b.pictures.load(b.files, src); f != nil {
			if f.svg != nil {
				// The text of a drawing needs the fonts; the same shaper again
				// does nothing.
				err := f.svg.SetText(b.shaper)
				_ = err // the drawing shows without its text
			}
			size, fraction := pictureSize(f.size(), img)
			w = picture(f, size, fraction, alt, b.looks.ink)
		}
	}
	if w == nil {
		return nil
	}
	return b.linked(w, alt, dest)
}

// pictureSize returns the size an image shows its picture at, of natural
// size, by its width and height attributes: both as they are, one with the
// other in proportion, a percentage of the width there is as a fraction.
func pictureSize(natural geom.Size, img *inline) (geom.Size, float32) {
	w, h := img.width, img.height
	switch {
	case img.widthPercent:
		return natural, w / 100
	case w > 0 && h > 0:
		return geom.Size{W: w, H: h}, 0
	case w > 0 && natural.W > 0:
		return geom.Size{W: w, H: natural.H * w / natural.W}, 0
	case h > 0 && natural.H > 0:
		return geom.Size{W: natural.W * h / natural.H, H: h}, 0
	}
	return natural, 0
}

// altImage builds an image that shows no picture as its alternative text, or
// its address without one, in a link to dest if dest is set.
func (b *builder) altImage(img *inline, dest string, style text.Style) core.Widget {
	alt := string(plainText(nil, img.children))
	style.Color = b.looks.muted
	if dest != "" && b.active(dest) {
		style.Color = b.looks.link
	}
	style.Italic = true
	label := alt
	if label == "" {
		label = b.source(img)
	}
	w := interact.Semantics{
		Role:  semantics.RoleImage,
		Label: alt,
		Child: b.richSpans([]text.Span{{Text: label}}, style, nil, nil, true),
	}
	return b.linked(w, alt, dest)
}

// linked puts w in a link to dest, if dest is set and can be followed.
func (b *builder) linked(w core.Widget, alt, dest string) core.Widget {
	if dest == "" || !b.active(dest) {
		return w
	}
	tap := b.tap
	label := alt
	if label == "" {
		label = dest
	}
	return interact.Pressable{
		Role:    semantics.RoleLink,
		Label:   label,
		Value:   dest,
		OnPress: func() { tap(dest) },
		Build: func(s interact.PressState) core.Widget {
			return interact.FocusRing{Visible: s.FocusVisible, Radius: theme.Space4, Child: w}
		},
	}
}

// heading builds a heading. Its text carries the anchor; a heading of
// pictures alone carries it on a box around them.
func (b *builder) heading(c *block) core.Widget {
	style := b.looks.headings[c.level-1]
	ws := b.inlineBlocks(c.inlines, style, c.textAlign)
	var child core.Widget
	if rt, ok := unaligned(ws); ok {
		rt.anchor = c.slug
		child = aligned(rt, c.textAlign)
	} else {
		child = anchorBox{slug: c.slug, child: column(ws, theme.Space4)}
	}
	return interact.Semantics{
		Role:  semantics.RoleHeading,
		Level: c.level,
		Label: headingText(c),
		Child: child,
	}
}

// unaligned returns the paragraph of ws, if ws is one paragraph and nothing
// else, with or without a box that aligns it.
func unaligned(ws []core.Widget) (richText, bool) {
	if len(ws) != 1 {
		return richText{}, false
	}
	w := ws[0]
	if f, ok := w.(fullWidth); ok {
		w = f.child
	}
	rt, ok := w.(richText)
	return rt, ok
}

// details builds a <details>: a title that opens and closes it, closed unless
// the source opens it, and its blocks below while it is open.
func (b *builder) details(c *block) core.Widget {
	open := b.isOpen != nil && b.isOpen(c)
	style := b.looks.body
	label := strings.TrimSpace(string(plainText(nil, c.summary)))
	line := style.LinePixels()
	title := basic.Row{
		Cross: layout.CrossStart,
		Children: []core.Widget{
			disclosure{open: open, size: evenPixels(style.Size), line: line, color: b.looks.muted},
			basic.Expanded{Child: b.rich(c.summary, style, true)},
		},
	}
	state := semantics.Collapsed
	if open {
		state = semantics.Expanded
	}
	toggle := b.toggle
	head := interact.Pressable{
		Label: label,
		State: state,
		OnPress: func() {
			if toggle != nil {
				toggle(c)
			}
		},
		Build: func(s interact.PressState) core.Widget {
			return interact.FocusRing{Visible: s.FocusVisible, Radius: theme.Space4, Child: title}
		},
	}
	if !open || len(c.children) == 0 {
		return head
	}
	body := column(b.blocks(c.children, b.looks.blockSpacing), b.looks.blockSpacing)
	return column([]core.Widget{head, body}, b.looks.blockSpacing)
}

// htmlTable builds a table written in HTML: its cells hold blocks, pictures
// as well as text. Only a first row of <th> is a header.
func (b *builder) htmlTable(t *block) core.Widget {
	cols := len(t.aligns)
	rows := make([]core.Widget, len(t.cells))
	for i, row := range t.cells {
		cells := make([]core.Widget, cols)
		for j := range cells {
			inner := *b
			lead := ""
			switch {
			case j > 0:
				lead = cellBreak
			case i > 0:
				lead = lineBreak
			}
			if lead != "" {
				inner.lead = &lead
			}
			role := semantics.RoleGridCell
			if t.header && i == 0 {
				role = semantics.RoleColumnHeader
				inner.looks.body.Weight = 600
			}
			var content []*block
			if j < len(row) {
				content = row[j]
			}
			cells[j] = interact.Semantics{Role: role, Child: basic.Padding{
				Insets: geom.Symmetric(cellPaddingX, cellPaddingY),
				Child:  column(inner.blocks(content, theme.Space8), theme.Space8),
			}}
		}
		rows[i] = tableRow{cells: cells}
	}
	return tableView{cols: cols, rule: b.looks.rule, header: b.looks.headerFill, fill: t.header, rows: rows}
}

// fullWidth lays out its child as wide as it is allowed, where that is
// bounded, so that aligned text has the width to align within.
type fullWidth struct {
	child core.Widget
}

// Key implements core.Widget.
func (fullWidth) Key() core.Key { return nil }

// ChildWidget implements core.SingleChildWidget.
func (f fullWidth) ChildWidget() core.Widget { return f.child }

// CreateRenderObject implements core.RenderWidget.
func (fullWidth) CreateRenderObject() core.RenderObject { return &renderFullWidth{} }

// UpdateRenderObject implements core.RenderWidget.
func (fullWidth) UpdateRenderObject(core.RenderObject) {}

type renderFullWidth struct {
	layout.RenderProxy
}

// Layout implements core.RenderObject.
func (r *renderFullWidth) Layout(c geom.Constraints) geom.Size {
	if c.MaxW < geom.Inf {
		c.MinW = c.MaxW
	}
	return r.RenderProxy.Layout(c)
}

// anchorBox carries the anchor of a heading that has no text of its own to
// carry it, as one of a picture alone.
type anchorBox struct {
	slug  string
	child core.Widget
}

// Key implements core.Widget.
func (anchorBox) Key() core.Key { return nil }

// ChildWidget implements core.SingleChildWidget.
func (a anchorBox) ChildWidget() core.Widget { return a.child }

// CreateRenderObject implements core.RenderWidget.
func (a anchorBox) CreateRenderObject() core.RenderObject { return &renderAnchor{slug: a.slug} }

// UpdateRenderObject implements core.RenderWidget.
func (a anchorBox) UpdateRenderObject(ro core.RenderObject) { ro.(*renderAnchor).slug = a.slug }

type renderAnchor struct {
	layout.RenderProxy
	slug string
	laid bool
}

// Layout implements core.RenderObject.
func (r *renderAnchor) Layout(c geom.Constraints) geom.Size {
	r.laid = true
	return r.RenderProxy.Layout(c)
}

func (r *renderAnchor) headingSlug() string { return r.slug }

func (r *renderAnchor) ready() bool { return r.laid }

// disclosure is the triangle of a details: pointing right while closed and
// down while open, in the middle of the first line.
type disclosure struct {
	open       bool
	size, line float32
	color      geom.Color
}

// Key implements core.Widget.
func (disclosure) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (d disclosure) CreateRenderObject() core.RenderObject { return &renderDisclosure{d: d} }

// UpdateRenderObject implements core.RenderWidget.
func (d disclosure) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDisclosure)
	if r.d != d {
		r.d = d
		r.mark.Reset()
		r.MarkNeedsLayout()
	}
}

type renderDisclosure struct {
	core.RenderBase
	d      disclosure
	mark   paint.Path
	stroke paint.Stroke
}

// Layout implements core.RenderObject.
func (r *renderDisclosure) Layout(c geom.Constraints) geom.Size {
	if r.mark.Empty() {
		// The mark is made once, relative to the box, and moved with the
		// canvas.
		s := r.d.size
		y := (r.d.line - s) / 2
		if r.d.open {
			r.mark.MoveTo(s*0.25, y+s*0.38)
			r.mark.LineTo(s*0.5, y+s*0.63)
			r.mark.LineTo(s*0.75, y+s*0.38)
		} else {
			r.mark.MoveTo(s*0.38, y+s*0.25)
			r.mark.LineTo(s*0.63, y+s*0.5)
			r.mark.LineTo(s*0.38, y+s*0.75)
		}
	}
	return c.Constrain(geom.Size{W: r.d.size * 1.25, H: r.d.line})
}

// Paint implements core.RenderObject.
func (r *renderDisclosure) Paint(cv *paint.Canvas, off geom.Offset) {
	r.stroke = paint.Stroke{Width: max(1.5, r.d.size/9), Color: r.d.color}
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.DrawPath(&r.mark, paint.Paint{Stroke: &r.stroke})
	cv.Restore()
}
