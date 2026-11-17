package markdown

import (
	"strings"
)

// elemKind says how the package treats an HTML element. See
// docs/ADR/0119-markdown-html-subset.md.
type elemKind uint8

// The kinds of element.
const (
	// elemUnknown is not interpreted: its tags show as the text they are.
	elemUnknown elemKind = iota
	// elemBlock holds blocks: p, div, h1 to h6, details, summary, the
	// elements of a table and pre.
	elemBlock
	// elemInline holds text: a, b, strong, i, em, code, kbd, sub, sup, s,
	// del, strike and picture. These are also read within Markdown text.
	elemInline
	// elemVoid has no content: br, img and source within text, and hr.
	elemVoid
)

// elementKind returns how the element name is treated.
func elementKind(name string) elemKind {
	switch name {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "details", "summary",
		"table", "thead", "tbody", "tfoot", "tr", "td", "th", "pre":
		return elemBlock
	case "a", "b", "strong", "i", "em", "code", "kbd", "sub", "sup", "s", "del", "strike", "picture":
		return elemInline
	case "br", "img", "source", "hr":
		return elemVoid
	}
	return elemUnknown
}

// inlineTagName reports whether raw HTML within Markdown text may use the
// element name. Block elements there show as text.
func inlineTagName(name string) bool {
	k := elementKind(name)
	return k == elemInline || (k == elemVoid && name != "hr")
}

// isRawText reports whether the content of the element name is text up to
// its closing tag, as for <script>. Such elements are not interpreted, and
// their content shows as written.
func isRawText(name string) bool {
	switch name {
	case "script", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes", "noscript", "plaintext":
		return true
	}
	return false
}

// htoken is a token of an HTML block: a tag that is interpreted, or text.
type htoken struct {
	tag *htmlTag
	// text has its character references decoded, unless it is a tag that
	// is not interpreted, which shows as written.
	text string
	// offset is where a tag starts in the content of the HTML block.
	offset int
}

// tokenizeHTML splits the content of an HTML block into tags and text.
// Comments, processing instructions, declarations and CDATA are dropped, as
// a browser does not show them. A tag of an element that is not interpreted
// becomes text, and so does a '<' that starts no tag.
func tokenizeHTML(s string) []htoken {
	var out []htoken
	text := func(t string) {
		if t != "" {
			out = append(out, htoken{text: t})
		}
	}
	i, last := 0, 0
	for i < len(s) {
		k := strings.IndexByte(s[i:], '<')
		if k < 0 {
			break
		}
		i += k
		if n := markupEnd(s[i:], true); n > 0 {
			text(decodeText(s[last:i]))
			i += n
			last = i
			continue
		}
		tag, n := scanTag(s[i:], false)
		if n == 0 {
			i++
			continue
		}
		text(decodeText(s[last:i]))
		start := i
		i += n
		last = i
		if elementKind(tag.name) != elemUnknown {
			out = append(out, htoken{tag: &tag, offset: start})
			continue
		}
		raw := tag.raw
		if !tag.closing && isRawText(tag.name) {
			end := indexClosing(s[i:], tag.name)
			raw += s[i : i+end]
			i += end
			last = i
		}
		out = append(out, htoken{text: raw})
	}
	text(decodeText(s[last:]))
	return out
}

// indexClosing returns where the closing tag of name starts in s, in any case,
// or len(s) if it does not.
func indexClosing(s, name string) int {
	for i := 0; i < len(s); {
		k := strings.Index(s[i:], "</")
		if k < 0 {
			return len(s)
		}
		i += k
		if rest := s[i+2:]; len(rest) >= len(name) && strings.EqualFold(rest[:len(name)], name) {
			return i
		}
		i += 2
	}
	return len(s)
}

// helem is an element of the tree that buildHTML builds from the HTML blocks
// and the Markdown blocks between them. Only block elements nest; the text and
// the inline tags inside stay a flat run, which pairTags pairs.
type helem struct {
	// tag is nil for the root.
	tag    *htmlTag
	name   string
	items  []hitem
	parent *helem
	depth  int
	// line is the line of the source, from 1, that the tag is on.
	line int
}

// hitem is an item of an element: an element, a Markdown block or a token.
type hitem struct {
	el  *helem
	md  *block
	tok htoken
}

// treeBuilder nests the block elements of a stream of tokens and Markdown
// blocks, closing what HTML closes implicitly, as a browser does in a simple
// form.
type treeBuilder struct {
	cur *helem
	// line is the line of the source that the current token is on.
	line int
}

// buildHTML interprets the HTML blocks among children, the blocks of parent,
// and returns the blocks that take their place. An element can span several
// blocks: <details> on one HTML block, Markdown after a blank line and
// </details> on another put the Markdown in the details, as on GitHub.
func buildHTML(parent *block, children []*block) []*block {
	root := &helem{depth: nesting(parent)}
	t := treeBuilder{cur: root}
	for _, c := range children {
		if c.kind != blockHTML {
			t.markdown(c)
			continue
		}
		content := string(c.content)
		line, from := c.startLine, 0
		for _, tok := range tokenizeHTML(content) {
			if tok.tag != nil {
				line += strings.Count(content[from:tok.offset], "\n")
				from = tok.offset
			}
			t.line = line
			t.token(tok)
		}
	}
	out := flow(root, alignNone)
	adopt(parent, out)
	return out
}

// adopt makes parent the parent of blocks, and of what details and tables
// among them hold.
func adopt(parent *block, blocks []*block) {
	for _, b := range blocks {
		b.parent = parent
		switch b.kind {
		case blockDetails:
			adopt(b, b.children)
		case blockTable:
			for _, row := range b.cells {
				for _, cell := range row {
					adopt(b, cell)
				}
			}
		}
	}
}

func (t *treeBuilder) add(it hitem) { t.cur.items = append(t.cur.items, it) }

// markdown adds a Markdown block. It closes what cannot hold a block, such as
// a <p>, as the <p> that the block is in HTML would.
func (t *treeBuilder) markdown(b *block) {
	for t.cur.tag != nil && !holdsBlocks(t.cur.name) {
		t.cur = t.cur.parent
	}
	t.add(hitem{md: b})
}

// holdsBlocks reports whether a Markdown block can sit in the element name.
func holdsBlocks(name string) bool {
	switch name {
	case "div", "details", "td", "th":
		return true
	}
	return false
}

func (t *treeBuilder) token(tok htoken) {
	tag := tok.tag
	switch {
	case tag == nil:
		t.add(hitem{tok: tok})
	case elementKind(tag.name) == elemBlock && tag.closing:
		t.close(tag.name)
	case elementKind(tag.name) == elemBlock:
		t.open(tag)
	case tag.name == "hr":
		if !tag.closing {
			t.closeP()
			t.add(hitem{tok: tok})
		}
	default:
		t.add(hitem{tok: tok})
	}
}

// open opens the block element of tag.
func (t *treeBuilder) open(tag *htmlTag) {
	switch tag.name {
	case "tr":
		if !t.popTo("table", "thead", "tbody", "tfoot") {
			return
		}
	case "td", "th":
		if !t.popTo("tr") {
			if !t.popTo("table", "thead", "tbody", "tfoot") {
				return
			}
			t.push(&htmlTag{name: "tr"})
		}
	case "thead", "tbody", "tfoot":
		if !t.popTo("table") {
			return
		}
	case "h1", "h2", "h3", "h4", "h5", "h6":
		t.closeP()
		if isHeading(t.cur.name) {
			t.cur = t.cur.parent
		}
	default:
		t.closeP()
	}
	t.push(tag)
}

// push opens an element of tag in the current one. Past maxNesting the tag is
// dropped and its content goes to the current element, so that the tree is
// never as deep as the input is long.
func (t *treeBuilder) push(tag *htmlTag) {
	if t.cur.depth >= maxNesting {
		return
	}
	e := &helem{tag: tag, name: tag.name, parent: t.cur, depth: t.cur.depth + 1, line: t.line}
	t.add(hitem{el: e})
	t.cur = e
}

// close closes the open element name and those in it. A closing tag with no
// open element is dropped, and so is one that would reach out of a cell.
func (t *treeBuilder) close(name string) {
	for e := t.cur; e.tag != nil; e = e.parent {
		if e.name == name {
			t.cur = e.parent
			return
		}
		if (e.name == "td" || e.name == "th" || e.name == "table") && !isTablePart(name) {
			return
		}
	}
}

// closeP closes an open <p>, as the start of a block element does.
func (t *treeBuilder) closeP() {
	for e := t.cur; e.tag != nil; e = e.parent {
		switch e.name {
		case "p":
			t.cur = e.parent
			return
		case "td", "th", "table":
			return
		}
	}
}

// popTo makes the nearest open element with one of names current, and
// reports whether there is one, within the nearest cell.
func (t *treeBuilder) popTo(names ...string) bool {
	for e := t.cur; e.tag != nil; e = e.parent {
		for _, n := range names {
			if e.name == n {
				t.cur = e
				return true
			}
		}
		if e.name == "td" || e.name == "th" {
			return false
		}
	}
	return false
}

func isTablePart(name string) bool {
	switch name {
	case "table", "thead", "tbody", "tfoot", "tr", "td", "th":
		return true
	}
	return false
}

func isHeading(name string) bool {
	return len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
}

// flow turns the items of e into blocks. Runs of text and inline tags become
// paragraphs aligned as al says, unless e has an align of its own.
func flow(e *helem, al align) []*block {
	var out []*block
	var run []htoken
	flush := func() {
		if ins := inlineRun(run); hasText(ins) {
			out = append(out, &block{kind: blockParagraph, html: true, inlines: ins, textAlign: al})
		}
		run = run[:0]
	}
	for _, it := range e.items {
		switch {
		case it.md != nil:
			flush()
			if k := it.md.kind; al != alignNone && it.md.textAlign == alignNone && (k == blockParagraph || k == blockHeading) {
				it.md.textAlign = al
			}
			out = append(out, it.md)
		case it.el != nil:
			flush()
			out = append(out, elementBlocks(it.el, al)...)
		case it.tok.tag != nil && it.tok.tag.name == "hr":
			flush()
			out = append(out, &block{kind: blockRule})
		default:
			run = append(run, it.tok)
		}
	}
	flush()
	return out
}

// elementBlocks turns the block element e into blocks.
func elementBlocks(e *helem, al align) []*block {
	al = alignOf(e.tag, al)
	switch {
	case isHeading(e.name):
		return []*block{{kind: blockHeading, html: true, level: int(e.name[1] - '0'), inlines: inlineRun(leafTokens(e, nil)), textAlign: al, startLine: e.line}}
	case e.name == "details":
		return []*block{details(e, al)}
	case e.name == "table":
		return tableBlocks(e)
	case e.name == "pre":
		var b strings.Builder
		for _, tok := range leafTokens(e, nil) {
			if tok.tag == nil {
				b.WriteString(tok.text)
			}
		}
		return []*block{{kind: blockCode, fenced: true, content: []byte(strings.TrimPrefix(b.String(), "\n"))}}
	}
	// p, div, and a summary or a part of a table out of place.
	return flow(e, al)
}

// details builds a details block: its first <summary> is the title, and the
// rest the content.
func details(e *helem, al align) *block {
	_, open := e.tag.attr("open")
	d := &block{kind: blockDetails, startsOpen: open}
	rest := &helem{name: "div"}
	found := false
	for _, it := range e.items {
		if it.el != nil && it.el.name == "summary" && !found {
			d.summary = inlineRun(leafTokens(it.el, nil))
			found = true
			continue
		}
		rest.items = append(rest.items, it)
	}
	if !found || !hasText(d.summary) {
		// What a browser shows for a details without a summary.
		d.summary = []inline{{kind: inlineText, text: "Details"}}
	}
	d.children = flow(rest, al)
	return d
}

// tableBlocks builds a table of the rows and cells of e. Text out of the
// cells goes before the table, as a browser puts it.
func tableBlocks(e *helem) []*block {
	t := &block{kind: blockTable}
	var stray []htoken
	allHeads := true
	var rows func(items []hitem)
	rows = func(items []hitem) {
		for _, it := range items {
			switch {
			case it.el == nil:
				stray = append(stray, it.tok)
			case it.el.name == "tr":
				var row [][]*block
				for _, c := range it.el.items {
					if c.el == nil || (c.el.name != "td" && c.el.name != "th") {
						stray = append(stray, leafOf(c)...)
						continue
					}
					if len(t.cells) == 0 && c.el.name != "th" {
						allHeads = false
					}
					row = append(row, flow(c.el, alignOf(c.el.tag, alignNone)))
				}
				if len(row) > 0 {
					t.cells = append(t.cells, row)
				}
			case it.el.name == "thead" || it.el.name == "tbody" || it.el.name == "tfoot":
				rows(it.el.items)
			default:
				stray = append(stray, leafOf(it)...)
			}
		}
	}
	rows(e.items)
	var out []*block
	if ins := inlineRun(stray); hasText(ins) {
		out = append(out, &block{kind: blockParagraph, html: true, inlines: ins})
	}
	cols := 0
	for _, row := range t.cells {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return out
	}
	for i, row := range t.cells {
		for len(row) < cols {
			row = append(row, nil)
		}
		t.cells[i] = row
	}
	t.aligns = make([]align, cols)
	t.header = allHeads
	return append(out, t)
}

// leafOf returns the tokens of an item.
func leafOf(it hitem) []htoken {
	if it.el != nil {
		return leafTokens(it.el, nil)
	}
	return []htoken{it.tok}
}

// leafTokens appends the tokens of e and of the elements in it to dst, in
// order: what is left of its text when its blocks are flattened.
func leafTokens(e *helem, dst []htoken) []htoken {
	for _, it := range e.items {
		switch {
		case it.el != nil:
			dst = leafTokens(it.el, dst)
		case it.md == nil:
			dst = append(dst, it.tok)
		}
	}
	return dst
}

// alignOf returns the align attribute of tag, or al if it has none.
func alignOf(tag *htmlTag, al align) align {
	if tag == nil {
		return al
	}
	v, _ := tag.attr("align")
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "left":
		return alignLeft
	case "center", "middle":
		return alignCenter
	case "right":
		return alignRight
	}
	return al
}

// inlineRun turns a run of text and inline tags into inline content, with
// white space collapsed as HTML collapses it.
func inlineRun(toks []htoken) []inline {
	var ins []inline
	for _, tok := range toks {
		if tok.tag == nil {
			ins = appendWords(ins, tok.text)
			continue
		}
		ins = append(ins, inline{kind: inlineTag, tag: tok.tag})
	}
	ins = pairTags(ins, false)
	return trimEnd(collapseSpace(ins))
}

// appendWords appends the text s with each run of white space made one: a
// soft line break if it holds a line break, so that Japanese text keeps no
// space there, and a space otherwise.
func appendWords(ins []inline, s string) []inline {
	var b strings.Builder
	for i := 0; i < len(s); {
		if !isHTMLSpace(s[i]) {
			j := i + 1
			for j < len(s) && !isHTMLSpace(s[j]) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}
		j := i
		newline := false
		for j < len(s) && isHTMLSpace(s[j]) {
			newline = newline || s[j] == '\n'
			j++
		}
		if newline {
			if b.Len() > 0 {
				ins = append(ins, inline{kind: inlineText, text: b.String()})
				b.Reset()
			}
			ins = append(ins, inline{kind: inlineSoftBreak})
		} else {
			b.WriteByte(' ')
		}
		i = j
	}
	if b.Len() > 0 {
		ins = append(ins, inline{kind: inlineText, text: b.String()})
	}
	return ins
}

// collapseSpace drops the white space that HTML does not show: at the start,
// after other white space, and after a line break.
func collapseSpace(ins []inline) []inline {
	space := true
	var walk func(ins []inline) []inline
	walk = func(ins []inline) []inline {
		out := ins[:0]
		for i := range ins {
			// out never passes i, so in can be changed in place.
			in := &ins[i]
			switch in.kind {
			case inlineText:
				if space {
					in.text = strings.TrimLeft(in.text, " ")
				}
				if in.text == "" {
					continue
				}
				space = in.text[len(in.text)-1] == ' '
			case inlineSoftBreak:
				if space {
					continue
				}
				space = true
			case inlineHardBreak:
				space = true
			case inlineCode, inlineKbd, inlineImage:
				space = false
			default:
				in.children = walk(in.children)
			}
			out = append(out, *in)
		}
		return out
	}
	return walk(ins)
}

// trimEnd drops the white space and line breaks at the end of ins.
func trimEnd(ins []inline) []inline {
	for len(ins) > 0 {
		last := &ins[len(ins)-1]
		switch last.kind {
		case inlineSoftBreak, inlineHardBreak:
			ins = ins[:len(ins)-1]
			continue
		case inlineText:
			last.text = strings.TrimRight(last.text, " ")
			if last.text == "" {
				ins = ins[:len(ins)-1]
				continue
			}
		case inlineEmph, inlineStrong, inlineStrike, inlineLink, inlineSub, inlineSup:
			last.children = trimEnd(last.children)
		}
		return ins
	}
	return ins
}

// pairTags pairs each opening tag among ins with its closing tag and turns
// the two and what lies between them into the element they stand for; the
// children of ins first, as the tags of raw HTML in Markdown text can sit
// within emphasis or a link. An element that is not closed reaches to the end
// of ins, as in a browser, and a closing tag with no opening one is dropped.
// Within a link (inLink), <a> adds no link: links do not nest.
func pairTags(ins []inline, inLink bool) []inline {
	for i := range ins {
		if len(ins[i].children) > 0 {
			ins[i].children = pairTags(ins[i].children, inLink || ins[i].kind == inlineLink)
		}
	}
	out := make([]inline, 0, len(ins))
	var open []int
	for i := range ins {
		in := &ins[i]
		if in.kind != inlineTag {
			out = append(out, *in)
			continue
		}
		t := in.tag
		switch {
		case t.closing:
			k := len(open) - 1
			for k >= 0 && out[open[k]].tag.name != t.name {
				k--
			}
			for k >= 0 && len(open) > k {
				out = wrap(out, open[len(open)-1], inLink)
				open = open[:len(open)-1]
			}
		case t.name == "br":
			out = append(out, inline{kind: inlineHardBreak})
		case t.name == "img":
			out = append(out, imageOf(t))
		case t.name == "source":
			// Kept for the <picture> around it.
			out = append(out, *in)
		case elementKind(t.name) == elemInline && len(open) < maxNesting:
			// Deeper tags are dropped, so that the inline tree is never as
			// deep as the input is long.
			open = append(open, len(out))
			out = append(out, *in)
		}
	}
	for len(open) > 0 {
		out = wrap(out, open[len(open)-1], inLink)
		open = open[:len(open)-1]
	}
	return dropTags(out)
}

// wrap turns the opening tag at out[j] and what follows it into its element.
func wrap(out []inline, j int, inLink bool) []inline {
	t := out[j].tag
	children := append([]inline(nil), out[j+1:]...)
	out = out[:j]
	if t.name != "picture" {
		children = dropTags(children)
	}
	return append(out, element(t, children, inLink)...)
}

// dropTags removes the tags left in ins: <source> out of a <picture>.
func dropTags(ins []inline) []inline {
	out := ins[:0]
	for i := range ins {
		if ins[i].kind != inlineTag {
			out = append(out, ins[i])
		}
	}
	return out
}

// element returns what the inline element of t with children shows as.
func element(t *htmlTag, children []inline, inLink bool) []inline {
	var kind inlineKind
	switch t.name {
	case "b", "strong":
		kind = inlineStrong
	case "i", "em":
		kind = inlineEmph
	case "s", "del", "strike":
		kind = inlineStrike
	case "sub":
		kind = inlineSub
	case "sup":
		kind = inlineSup
	case "code":
		return []inline{{kind: inlineCode, text: string(plainText(nil, children))}}
	case "kbd":
		return []inline{{kind: inlineKbd, text: string(plainText(nil, children))}}
	case "a":
		href, ok := t.attr("href")
		href = strings.TrimSpace(href)
		if !ok || inLink || !safeURL(href) {
			return children
		}
		return []inline{{kind: inlineLink, dest: href, children: unlink(children)}}
	case "picture":
		return pictureOf(children)
	default:
		return children
	}
	return []inline{{kind: kind, children: children}}
}

// unlink replaces the links within ins with their content.
func unlink(ins []inline) []inline {
	out := make([]inline, 0, len(ins))
	for i := range ins {
		in := &ins[i]
		in.children = unlink(in.children)
		if in.kind == inlineLink {
			out = append(out, in.children...)
			continue
		}
		out = append(out, *in)
	}
	return out
}

// imageOf returns the image of an <img> tag.
func imageOf(t *htmlTag) inline {
	src, _ := t.attr("src")
	src = strings.TrimSpace(src)
	if !safeURL(src) {
		src = ""
	}
	img := inline{kind: inlineImage, dest: src}
	if alt, _ := t.attr("alt"); alt != "" {
		img.children = []inline{{kind: inlineText, text: alt}}
	}
	if w, ok := t.attr("width"); ok {
		img.width, img.widthPercent = parseLength(w)
	}
	if h, ok := t.attr("height"); ok {
		if v, percent := parseLength(h); !percent {
			img.height = v
		}
	}
	return img
}

// pictureOf returns the image of a <picture> with the <source> elements
// before it, or nothing if it has no <img>.
func pictureOf(children []inline) []inline {
	var sources []pictureSource
	for i := range children {
		c := &children[i]
		switch {
		case c.kind == inlineTag && c.tag.name == "source":
			if s, ok := sourceOf(c.tag); ok {
				sources = append(sources, s)
			}
		case c.kind == inlineImage:
			c.sources = sources
			return []inline{*c}
		}
	}
	return nil
}

// sourceOf reads a <source>: the theme its media query asks for, and the
// first address of its srcset. A source with a query other than
// prefers-color-scheme, or of a type the package does not read, is left out.
func sourceOf(t *htmlTag) (pictureSource, bool) {
	var s pictureSource
	if ty, ok := t.attr("type"); ok {
		switch strings.ToLower(strings.TrimSpace(ty)) {
		case "image/png", "image/jpeg", "image/webp", "image/svg+xml":
		default:
			return s, false
		}
	}
	media, _ := t.attr("media")
	switch strings.ReplaceAll(strings.ToLower(media), " ", "") {
	case "", "all":
	case "(prefers-color-scheme:dark)":
		s.scheme = schemeDark
	case "(prefers-color-scheme:light)":
		s.scheme = schemeLight
	default:
		return s, false
	}
	srcset, _ := t.attr("srcset")
	s.src = pickSrcset(srcset)
	return s, s.src != "" && safeURL(s.src)
}

// pickSrcset returns the address of a srcset for a density of 1: the
// candidate marked 1x or with no descriptor, or else the first.
func pickSrcset(srcset string) string {
	first := ""
	for _, c := range strings.Split(srcset, ",") {
		f := strings.Fields(c)
		if len(f) == 0 {
			continue
		}
		if first == "" {
			first = f[0]
		}
		if len(f) == 1 || f[1] == "1x" {
			return f[0]
		}
	}
	return first
}

// maxLength bounds the width and the height an image can ask for.
const maxLength = 16384

// parseLength reads the value of a width or a height attribute: pixels, as
// "280" or "280px", or a percentage, as "50%". Anything else is 0.
func parseLength(v string) (float32, bool) {
	v = strings.TrimSpace(strings.ToLower(v))
	percent := strings.HasSuffix(v, "%")
	v = strings.TrimSuffix(strings.TrimSuffix(v, "%"), "px")
	var n float32
	digits, frac, scale := 0, false, float32(1)
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case isDigit(c):
			digits++
			if frac {
				scale /= 10
				n += float32(c-'0') * scale
			} else {
				n = n*10 + float32(c-'0')
			}
			if n > maxLength {
				n = maxLength
			}
		case c == '.' && !frac:
			frac = true
		default:
			return 0, false
		}
	}
	if digits == 0 || n <= 0 {
		return 0, false
	}
	if percent {
		return min(n, 100), true
	}
	return n, false
}

// safeURL reports whether a link or an image may go to u: not a javascript:,
// vbscript: or data: address, however it is spelled, as these run code or
// carry content in place of an address.
func safeURL(u string) bool {
	var b [len("javascript:")]byte
	n := 0
	for i := 0; i < len(u) && n < len(b); i++ {
		c := u[i]
		if c <= ' ' {
			// Browsers drop tabs, line breaks and leading spaces in an
			// address, so "java\tscript:" is javascript: too.
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[n] = c
		n++
	}
	s := string(b[:n])
	return !strings.HasPrefix(s, "javascript:") && !strings.HasPrefix(s, "vbscript:") && !strings.HasPrefix(s, "data:")
}
