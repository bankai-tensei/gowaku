package markdown

import (
	"io/fs"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

const (
	// strongWeight is the weight of **strong** text.
	strongWeight = 700
	// codePadding is the space inside a code block.
	codePadding = theme.Space12
	// scrollRoom keeps the last line of a code block clear of the scroll bar
	// that appears over its bottom edge.
	scrollRoom = theme.Space8
	// codeRadius rounds the corners of a code block.
	codeRadius = 6
	// cellPadding is the space inside a cell of a table.
	cellPaddingX, cellPaddingY = theme.Space12, theme.Space6
)

// builder turns the block tree into widgets.
type builder struct {
	shaper *text.Shaper
	looks  looks
	// tap follows a link. external says whether the app takes links, through
	// OnLinkTap; without it, only links to the headings in slugs work.
	tap      func(url string)
	external bool
	slugs    map[string]*block
	// brk is what a copy puts before each text (SelectionBreak). lead, when
	// it points at a string that is not empty, is the break of the next text
	// in its place, which then clears it: a list takes the break of a block
	// before its first item, and that of a line before the others.
	brk       string
	lead      *string
	images    func(src, alt string) core.Widget
	pictures  *imageCache
	files     fs.FS
	highlight func(lang, code string) []text.Span
	// onCode is Markdown.CodeBlock. copy, when set, writes code to the
	// clipboard from the Copy button of each code block, labeled copyLabel.
	onCode    func(c CodeBlock) core.Widget
	copy      func(code string)
	copyLabel string
	// copiedLabel is what the Copy button says for a moment after a copy.
	copiedLabel string
	// listDepth counts the lists around what is being built, for the shape
	// of bullets.
	listDepth int
	// dark says that the theme is dark, for the pictures of <picture>.
	dark bool
	// isOpen reports whether a details block is open, and toggle opens or
	// closes it.
	isOpen func(c *block) bool
	toggle func(c *block)
}

func (b *builder) document(doc *block) core.Widget {
	return column(b.blocks(doc.children, b.looks.blockSpacing), b.looks.blockSpacing)
}

func column(children []core.Widget, spacing float32) core.Widget {
	return basic.Column{Size: layout.MainMin, Spacing: spacing, Children: children}
}

// blocks builds a run of blocks. spacing is the space the column puts
// between them, which headings add to.
func (b *builder) blocks(bs []*block, spacing float32) []core.Widget {
	out := make([]core.Widget, 0, len(bs))
	for i, c := range bs {
		ws := b.block(c)
		if c.kind == blockHeading && i > 0 && c.level <= 3 {
			// A heading starts a section: more room above than below.
			ws[0] = basic.Padding{Insets: geom.Insets{Top: spacing / float32(c.level)}, Child: ws[0]}
		}
		out = append(out, ws...)
	}
	return out
}

// block builds one block. A paragraph with images builds several widgets.
func (b *builder) block(c *block) []core.Widget {
	switch c.kind {
	case blockParagraph:
		return b.inlineBlocks(c.inlines, b.looks.body, c.textAlign)
	case blockHeading:
		return []core.Widget{b.heading(c)}
	case blockDetails:
		return []core.Widget{b.details(c)}
	case blockCode:
		return []core.Widget{b.codeBlock(c)}
	case blockQuote:
		inner := *b
		inner.looks.body.Color = b.looks.muted
		return []core.Widget{quoteBox{
			indent: b.looks.indent * 0.75,
			color:  b.looks.rule,
			child:  column(inner.blocks(c.children, b.looks.blockSpacing), b.looks.blockSpacing),
		}}
	case blockList:
		return []core.Widget{b.list(c)}
	case blockRule:
		return []core.Widget{basic.Padding{
			Insets: geom.Symmetric(0, theme.Space4),
			Child:  basic.Divider{Color: b.looks.rule},
		}}
	case blockTable:
		if c.cells != nil {
			return []core.Widget{b.htmlTable(c)}
		}
		return []core.Widget{b.table(c)}
	}
	return nil
}

// trimEdges returns a copy of ins without the space at its two ends, which
// an image that stood there leaves.
func trimEdges(ins []inline) []inline {
	out := append([]inline(nil), ins...)
	for len(out) > 0 && (out[0].kind == inlineSoftBreak || out[0].kind == inlineHardBreak) {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1].kind == inlineSoftBreak || out[len(out)-1].kind == inlineHardBreak) {
		out = out[:len(out)-1]
	}
	if len(out) > 0 && out[0].kind == inlineText {
		out[0].text = strings.TrimLeft(out[0].text, " \t")
	}
	if n := len(out); n > 0 && out[n-1].kind == inlineText {
		out[n-1].text = strings.TrimRight(out[n-1].text, " \t")
	}
	return out
}

// standaloneImage reports whether in is an image, or a link whose only
// content is an image, and returns the image and the destination of the link.
func standaloneImage(in *inline) (img *inline, dest string, ok bool) {
	switch {
	case in.kind == inlineImage:
		return in, "", true
	case in.kind == inlineLink && len(in.children) == 1 && in.children[0].kind == inlineImage:
		return &in.children[0], in.dest, true
	}
	return nil, "", false
}

// hasText reports whether ins shows anything but space and line breaks.
func hasText(ins []inline) bool {
	for i := range ins {
		switch ins[i].kind {
		case inlineSoftBreak, inlineHardBreak:
		case inlineText:
			if strings.TrimSpace(ins[i].text) != "" {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func (b *builder) codeBlock(c *block) core.Widget {
	raw := strings.TrimSuffix(string(c.content), "\n")
	code := expandTabs(raw)
	lang, _, _ := strings.Cut(c.info, " ")
	style := b.looks.code
	var spans []text.Span
	if b.highlight != nil && code != "" {
		spans = b.highlight(lang, code)
		if spans != nil && joinedText(spans) != code {
			panic("gowaku: Markdown.Highlight returned spans whose texts do not spell out the code")
		}
	}
	if spans == nil {
		spans = []text.Span{{Text: code}}
	}
	lines := float32(strings.Count(code, "\n") + 1)
	body := basic.SizedBox{
		Height: lines*style.LinePixels() + scrollRoom,
		Child:  basic.Scroll{Horizontal: true, Child: b.richSpans(spans, style, nil, nil, false)},
	}
	children := []core.Widget{body}
	var head core.Widget
	if lang != "" {
		caption := style
		caption.Size = evenPixels(style.Size * 0.85)
		caption.Color = b.looks.muted
		head = b.label(lang, caption)
	}
	if b.copy != nil {
		copyCode := b.copy
		button := copyButton{label: b.copyLabel, done: b.copiedLabel, onPress: func() { copyCode(raw) }}
		if head == nil {
			head = basic.SizedBox{}
		}
		head = basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{basic.Expanded{Child: head}, button}}
	}
	if head != nil {
		children = []core.Widget{head, body}
	}
	def := interact.Semantics{
		Role:  semantics.RoleGroup,
		Label: lang,
		Child: basic.Container{
			Color:   b.looks.codeBackground,
			Radius:  codeRadius,
			Padding: geom.Insets{Left: codePadding, Top: codePadding * 0.75, Right: codePadding, Bottom: codePadding*0.75 - scrollRoom/2},
			Child:   column(children, theme.Space4),
		},
	}
	if b.onCode != nil {
		if w := b.onCode(CodeBlock{Lang: lang, Info: c.info, Code: raw, Open: c.fenced && !c.closed, Default: def}); w != nil {
			return w
		}
	}
	return def
}

// expandTabs turns tabs into spaces up to the next stop of four columns, so
// that code keeps its columns in any font.
func expandTabs(s string) string {
	if strings.IndexByte(s, '\t') < 0 {
		return s
	}
	var out strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			n := tabStop - col%tabStop
			out.WriteString("    "[:n])
			col += n
		case '\n':
			out.WriteRune(r)
			col = 0
		default:
			out.WriteRune(r)
			col++
		}
	}
	return out.String()
}

func joinedText(spans []text.Span) string {
	var b strings.Builder
	for i := range spans {
		b.WriteString(spans[i].Text)
	}
	return b.String()
}

func (b *builder) list(l *block) core.Widget {
	inner := *b
	inner.listDepth++
	spacing := float32(theme.Space4)
	if !l.tight {
		spacing = b.looks.blockSpacing
	} else {
		lead := b.brk
		inner.brk, inner.lead = lineBreak, &lead
	}
	width := b.looks.indent
	if l.ordered {
		last := strconv.Itoa(l.start+len(l.children)-1) + string(l.delimiter)
		width = max(width, b.shaper.Shape(last, b.looks.body).Layout(geom.Inf).Size().W+theme.Space8)
	}
	items := make([]core.Widget, len(l.children))
	for i, it := range l.children {
		items[i] = inner.item(l, it, i, width, spacing)
	}
	return interact.Semantics{Role: semantics.RoleList, Child: column(items, spacing)}
}

func (b *builder) item(l, it *block, i int, width, spacing float32) core.Widget {
	line := b.looks.body.LinePixels()
	var marker core.Widget
	switch {
	case it.task:
		label := ""
		if len(it.children) > 0 && it.children[0].kind == blockParagraph {
			label = string(plainText(nil, it.children[0].inlines))
		}
		marker = checkMark{
			checked: it.checked, label: label, width: width, line: line,
			size: evenPixels(b.looks.body.Size), box: b.looks.muted, fill: b.looks.accent, mark: b.looks.onAccent,
			radius: b.looks.radius,
		}
	case l.ordered:
		style := b.looks.body
		style.Align = text.AlignEnd
		number := b.label(strconv.Itoa(l.start+i)+string(l.delimiter), style)
		marker = basic.Padding{Insets: geom.Insets{Right: theme.Space8}, Child: number}
	default:
		marker = bullet{depth: b.listDepth - 1, width: width, line: line, size: b.looks.body.Size, color: b.looks.body.Color}
	}
	content := column(b.blocks(it.children, spacing), spacing)
	return interact.Semantics{
		Role: semantics.RoleListItem,
		Child: basic.Row{
			Cross: layout.CrossStart,
			Children: []core.Widget{
				basic.SizedBox{Width: width, Child: marker},
				basic.Expanded{Child: content},
			},
		},
	}
}

func (b *builder) table(t *block) core.Widget {
	cols := len(t.aligns)
	rows := make([]core.Widget, len(t.rows))
	for i, row := range t.rows {
		cells := make([]core.Widget, cols)
		for j := range cells {
			style := b.looks.body
			role := semantics.RoleGridCell
			if i == 0 {
				style.Weight = 600
				role = semantics.RoleColumnHeader
			}
			switch t.aligns[j] {
			case alignCenter:
				style.Align = text.AlignCenter
			case alignRight:
				style.Align = text.AlignEnd
			}
			var content []inline
			if j < len(row) {
				content = row[j]
			}
			cell := b.rich(content, style, false)
			switch {
			case j > 0:
				cell.brk = cellBreak
			case i > 0:
				cell.brk = lineBreak
			}
			cells[j] = interact.Semantics{Role: role, Child: basic.Padding{
				Insets: geom.Symmetric(cellPaddingX, cellPaddingY),
				Child:  cell,
			}}
		}
		rows[i] = tableRow{cells: cells}
	}
	return tableView{cols: cols, rule: b.looks.rule, header: b.looks.headerFill, fill: true, rows: rows}
}

// rich builds a paragraph of inline content.
func (b *builder) rich(ins []inline, style text.Style, silent bool) richText {
	f := flattener{looks: &b.looks, base: style, dark: b.dark}
	f.walk(ins, inlineStyle{})
	f.flush()
	return b.richSpans(f.spans, style, f.links, f.decos, silent)
}

// richSpans builds a paragraph of spans, with a target for each link that
// can be followed.
func (b *builder) richSpans(spans []text.Span, style text.Style, links []linkSpan, decos []deco, silent bool) richText {
	rt := richText{
		shaper: b.shaper,
		spans:  spans,
		style:  style,
		links:  links,
		decos:  decos,
		colors: richColors{link: b.looks.link, code: b.looks.codeBackground, focus: b.looks.focus, rule: b.looks.rule},
		silent: silent,
		brk:    b.brk,
	}
	if b.lead != nil && *b.lead != "" {
		rt.brk, *b.lead = *b.lead, ""
	}
	var all string
	for i, l := range links {
		if !b.active(l.url) {
			continue
		}
		if all == "" {
			all = joinedText(spans)
		}
		rt.children = append(rt.children, linkTarget{url: l.url, label: all[l.start:l.end], tap: b.tap, index: i})
	}
	return rt
}

// label builds a text that a screen reader and a selection pass over, such as
// the number of a list item: the node around it says it, and a copy leaves it
// out.
func (b *builder) label(s string, style text.Style) richText {
	return richText{
		shaper:   b.shaper,
		spans:    []text.Span{{Text: s}},
		style:    style,
		colors:   richColors{link: b.looks.link, code: b.looks.codeBackground, focus: b.looks.focus, rule: b.looks.rule},
		silent:   true,
		noSelect: true,
	}
}

// active reports whether a link to dest can be followed: any link when the
// app takes links, and else a link to a heading of the document; never one to
// javascript: and the like (safeURL).
func (b *builder) active(dest string) bool {
	return b.tap != nil && safeURL(dest) && (b.external || anchorOf(b.slugs, dest) != nil)
}

// inlineStyle is what the inlines around a text make of it.
type inlineStyle struct {
	italic, strike, code, link, muted bool
	kbd, sub, sup                     bool
	weight                            uint16
}

// linkSpan is the byte range of a link in the text of a paragraph.
type linkSpan struct {
	start, end int
	url        string
}

// decoKind says what is drawn with the text of a range.
type decoKind uint8

// The decorations.
const (
	decoCode decoKind = iota
	decoStrike
	// decoKey draws a key cap around the text of <kbd>.
	decoKey
	// decoSub and decoSup move the text below and above the baseline.
	decoSub
	decoSup
)

// deco is a range of the text of a paragraph that is drawn with more than
// glyphs.
type deco struct {
	start, end int
	kind       decoKind
}

// flattener turns an inline tree into spans of one look each, with the
// ranges of the links and decorations.
type flattener struct {
	looks *looks
	base  text.Style
	// dark says that the theme is dark, which hides the images meant for
	// the light theme.
	dark bool

	spans []text.Span
	links []linkSpan
	decos []deco
	// cur collects the text of the span being built, in style curStyle.
	cur      strings.Builder
	curStyle text.Style
	// n is the length of the text so far, and last its last piece.
	n    int
	last string
	// soft is a soft line break that waits for the text after it.
	soft bool
}

func (f *flattener) walk(ins []inline, st inlineStyle) {
	for i := range ins {
		in := &ins[i]
		s := st
		switch in.kind {
		case inlineText:
			f.add(in.text, st)
		case inlineCode:
			s.code = true
			f.add(in.text, s)
		case inlineEmph:
			s.italic = true
			f.walk(in.children, s)
		case inlineStrong:
			s.weight = strongWeight
			f.walk(in.children, s)
		case inlineStrike:
			s.strike = true
			f.walk(in.children, s)
		case inlineLink:
			if st.link {
				f.walk(in.children, st)
				continue
			}
			// A soft break before the link is settled first, so that its
			// space does not become part of the link.
			if f.soft {
				f.settleSoft(string(plainText(nil, in.children)))
			}
			s.link = true
			start := f.n
			f.walk(in.children, s)
			if f.n > start {
				f.links = append(f.links, linkSpan{start: start, end: f.n, url: in.dest})
			}
		case inlineImage:
			// An image within text shows its alternative text.
			if !imageShown(in, f.dark) {
				continue
			}
			s.muted = true
			f.add(string(plainText(nil, in.children)), s)
		case inlineKbd:
			s.kbd = true
			f.add(in.text, s)
		case inlineSub:
			s.sub, s.sup = true, false
			f.walk(in.children, s)
		case inlineSup:
			s.sup, s.sub = true, false
			f.walk(in.children, s)
		case inlineSoftBreak:
			f.soft = true
		case inlineHardBreak:
			f.soft = false
			f.append("\n", f.spanStyle(st))
		}
	}
}

// add appends text in the look st.
func (f *flattener) add(s string, st inlineStyle) {
	if s == "" {
		return
	}
	if f.soft {
		f.settleSoft(s)
	}
	start := f.n
	f.append(s, f.spanStyle(st))
	switch {
	case st.code:
		f.decos = append(f.decos, deco{start: start, end: f.n, kind: decoCode})
	case st.kbd:
		f.decos = append(f.decos, deco{start: start, end: f.n, kind: decoKey})
	}
	switch {
	case st.sub:
		f.extend(start, decoSub)
	case st.sup:
		f.extend(start, decoSup)
	}
	if st.strike {
		f.extend(start, decoStrike)
	}
}

// extend adds a decoration of kind from start to the end of the text, joined
// to the last one of the same kind if it ends at start.
func (f *flattener) extend(start int, kind decoKind) {
	for k := len(f.decos) - 1; k >= 0; k-- {
		if d := &f.decos[k]; d.kind == kind {
			if d.end == start {
				d.end = f.n
				return
			}
			break
		}
	}
	f.decos = append(f.decos, deco{start: start, end: f.n, kind: kind})
}

// settleSoft turns the waiting soft break into a space, or nothing between
// wide characters, now that the text after it is known.
func (f *flattener) settleSoft(next string) {
	f.soft = false
	if sep := softBreak(f.last, next); sep != "" {
		f.cur.WriteString(sep)
		f.n += len(sep)
	}
}

func (f *flattener) append(s string, style text.Style) {
	if f.cur.Len() > 0 && style != f.curStyle {
		f.flush()
	}
	f.curStyle = style
	f.cur.WriteString(s)
	f.n += len(s)
	f.last = s
}

// flush ends the span being built.
func (f *flattener) flush() {
	if f.cur.Len() > 0 {
		f.spans = append(f.spans, text.Span{Text: f.cur.String(), Style: f.curStyle})
		f.cur.Reset()
	}
}

// spanStyle returns the style of a span in the look st. Its zero fields take
// those of the paragraph.
func (f *flattener) spanStyle(st inlineStyle) text.Style {
	var s text.Style
	s.Italic = st.italic
	s.Weight = st.weight
	if st.code || st.kbd {
		s.Family = f.looks.code.Family
		// Code keeps its size relative to the text around it, in a heading
		// as in the body.
		base := f.base.Size
		if base <= 0 {
			base = f.looks.body.Size
		}
		if body := f.looks.body.Size; body > 0 {
			s.Size = evenPixels(base * f.looks.code.Size / body)
		}
		if c := f.looks.code.Color; c != f.looks.ink {
			s.Color = c
		}
	}
	if st.sub || st.sup {
		base := s.Size
		if base <= 0 {
			base = f.base.Size
		}
		if base <= 0 {
			base = f.looks.body.Size
		}
		if base <= 0 {
			base = defaultTextSize
		}
		s.Size = evenPixels(base * scriptScale)
	}
	switch {
	case st.link:
		s.Color = f.looks.link
	case st.muted:
		s.Color = f.looks.muted
	}
	return s
}

// scriptScale is the size of <sub> and <sup> text relative to the text
// around it, as browsers set it.
const scriptScale = 0.75
