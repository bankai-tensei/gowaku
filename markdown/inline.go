package markdown

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxLinkParens bounds the nesting of parentheses in a link destination, as
// the specification allows.
const maxLinkParens = 32

// inode is a node of the inline tree while it is being built. The tree is
// linked both ways, as the emphasis and link algorithms of the specification
// move runs of siblings around.
type inode struct {
	kind                inlineKind
	text                string
	dest, title         string
	parent, first, last *inode
	prev, next          *inode
	autolink            bool
	tag                 *htmlTag
}

func (n *inode) appendChild(c *inode) {
	c.unlink()
	c.parent = n
	if n.last != nil {
		n.last.next = c
		c.prev = n.last
	} else {
		n.first = c
	}
	n.last = c
}

func (n *inode) insertAfter(s *inode) {
	s.unlink()
	s.next = n.next
	if s.next != nil {
		s.next.prev = s
	}
	s.prev = n
	n.next = s
	s.parent = n.parent
	if s.parent != nil && s.next == nil {
		s.parent.last = s
	}
}

func (n *inode) unlink() {
	if n.prev != nil {
		n.prev.next = n.next
	} else if n.parent != nil {
		n.parent.first = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else if n.parent != nil {
		n.parent.last = n.prev
	}
	n.parent, n.next, n.prev = nil, nil, nil
}

// delimiter is a run of *, _ or ~ that may open or close emphasis.
type delimiter struct {
	char              byte
	count, origCount  int
	node              *inode
	prev, next        *delimiter
	canOpen, canClose bool
}

// bracket is a [ or ![ that may open a link or an image.
type bracket struct {
	node          *inode
	prev          *bracket
	prevDelimiter *delimiter
	index         int
	image         bool
	active        bool
	// bracketAfter says that another bracket followed this one, so that
	// "[foo][]" cannot mean the shortcut reference "[foo]".
	bracketAfter bool
}

// inlineParser parses the inline content of one block.
type inlineParser struct {
	subject    string
	pos        int
	refs       map[string]linkRef
	root       *inode
	delimiters *delimiter
	brackets   *bracket
	// ticks lists, for each length of a run of backticks, where such runs
	// start, so that finding the end of a code span takes a search instead
	// of a scan.
	ticks map[int][]int
	// hasTags says that the content has tags of raw HTML to pair.
	hasTags bool
	// unclosed says, for each kind of markup, that its end is nowhere after
	// a place it was looked for, so that no later one is looked for: a run
	// of "<!--" without "-->" is not scanned to its end once for each.
	unclosed [markupKinds]bool
}

// parseInlines parses the inline content s of a block.
func parseInlines(s string, refs map[string]linkRef) []inline {
	p := &inlineParser{subject: s, refs: refs, root: &inode{}}
	for p.pos < len(p.subject) {
		p.parseInline()
	}
	p.processEmphasis(nil)
	ins := convert(p.root, false)
	if p.hasTags {
		ins = pairTags(ins, false)
	}
	return ins
}

// convert turns the children of n into inlines, merging neighboring texts.
// Inside a link, links lose their link: links do not nest.
func convert(n *inode, inLink bool) []inline {
	var out []inline
	for c := n.first; c != nil; c = c.next {
		kind := c.kind
		if kind == inlineLink && (inLink || !safeURL(c.dest)) {
			// A link to javascript: and the like keeps its text only.
			out = append(out, convert(c, inLink)...)
			continue
		}
		if kind == inlineText && c.text == "" {
			continue
		}
		in := inline{kind: kind, text: c.text, dest: c.dest, title: c.title, tag: c.tag}
		if c.first != nil {
			in.children = convert(c, inLink || kind == inlineLink)
		}
		out = append(out, in)
	}
	return mergeTexts(out)
}

// mergeTexts joins neighboring texts. It joins each run at once, as joining
// one text at a time would copy a long run over and over.
func mergeTexts(ins []inline) []inline {
	out := ins[:0]
	for i := 0; i < len(ins); {
		if ins[i].kind != inlineText {
			out = append(out, ins[i])
			i++
			continue
		}
		j := i + 1
		for j < len(ins) && ins[j].kind == inlineText {
			j++
		}
		if j == i+1 {
			out = append(out, ins[i])
		} else {
			var b strings.Builder
			for k := i; k < j; k++ {
				b.WriteString(ins[k].text)
			}
			out = append(out, inline{kind: inlineText, text: b.String()})
		}
		i = j
	}
	return out
}

func (p *inlineParser) text(s string) *inode {
	n := &inode{kind: inlineText, text: s}
	p.root.appendChild(n)
	return n
}

func (p *inlineParser) peek() byte { return peek(p.subject, p.pos) }

// parseInline parses what starts at pos and moves past it. It always moves
// at least one byte, so the loop over the subject ends.
func (p *inlineParser) parseInline() {
	switch c := p.subject[p.pos]; c {
	case '\n':
		p.parseNewline()
	case '\\':
		p.parseBackslash()
	case '`':
		p.parseBackticks()
	case '*', '_', '~':
		p.handleDelim(c)
	case '[':
		p.text("[")
		p.addBracket(p.root.last, p.pos, false)
		p.pos++
	case '!':
		p.parseBang()
	case ']':
		p.parseCloseBracket()
	case '<':
		if !p.parseAutolink() && !p.parseRawHTML() {
			p.text("<")
			p.pos++
		}
	case '&':
		if v, n := decodeEntity(p.subject[p.pos:]); n > 0 {
			p.text(v)
			p.pos += n
		} else {
			p.text("&")
			p.pos++
		}
	default:
		if !p.parseExtendedAutolink() {
			p.parseString()
		}
	}
}

// isSpecial reports whether c may start something other than plain text.
func isSpecial(c byte) bool {
	switch c {
	case '\n', '\\', '`', '*', '_', '~', '[', ']', '!', '<', '&':
		return true
	}
	return false
}

func (p *inlineParser) parseString() {
	start := p.pos
	p.pos++
	for p.pos < len(p.subject) {
		c := p.subject[p.pos]
		if isSpecial(c) || ((c == 'h' || c == 'w') && p.autolinkBoundary(p.pos) && matchExtendedAutolink(p.subject[p.pos:]) > 0) {
			break
		}
		p.pos++
	}
	p.text(p.subject[start:p.pos])
}

func (p *inlineParser) parseNewline() {
	p.pos++
	hard := false
	if last := p.root.last; last != nil && last.kind == inlineText && strings.HasSuffix(last.text, " ") {
		trimmed := strings.TrimRight(last.text, " ")
		hard = len(last.text)-len(trimmed) >= 2
		last.text = trimmed
	}
	if hard {
		p.root.appendChild(&inode{kind: inlineHardBreak})
	} else {
		p.root.appendChild(&inode{kind: inlineSoftBreak})
	}
	for p.pos < len(p.subject) && (p.subject[p.pos] == ' ' || p.subject[p.pos] == '\t') {
		p.pos++
	}
}

func (p *inlineParser) parseBackslash() {
	p.pos++
	switch c := p.peek(); {
	case c == '\n':
		p.pos++
		p.root.appendChild(&inode{kind: inlineHardBreak})
		for p.pos < len(p.subject) && (p.subject[p.pos] == ' ' || p.subject[p.pos] == '\t') {
			p.pos++
		}
	case isASCIIPunct(c):
		p.text(string(c))
		p.pos++
	default:
		p.text("\\")
	}
}

func (p *inlineParser) parseBackticks() {
	start := p.pos
	n := runLength(p.subject, start, '`')
	afterOpen := start + n
	if end, ok := p.closingTicks(n, afterOpen); ok {
		content := strings.ReplaceAll(p.subject[afterOpen:end], "\n", " ")
		if len(content) > 2 && content[0] == ' ' && content[len(content)-1] == ' ' && strings.Trim(content, " ") != "" {
			content = content[1 : len(content)-1]
		}
		p.root.appendChild(&inode{kind: inlineCode, text: content})
		p.pos = end + n
		return
	}
	p.text(p.subject[start:afterOpen])
	p.pos = afterOpen
}

// closingTicks finds the first run of exactly n backticks at or after from.
func (p *inlineParser) closingTicks(n, from int) (int, bool) {
	if p.ticks == nil {
		p.ticks = map[int][]int{}
		for i := 0; i < len(p.subject); {
			if p.subject[i] != '`' {
				i++
				continue
			}
			k := runLength(p.subject, i, '`')
			p.ticks[k] = append(p.ticks[k], i)
			i += k
		}
	}
	runs := p.ticks[n]
	i := sort.SearchInts(runs, from)
	if i < len(runs) {
		return runs[i], true
	}
	return 0, false
}

func (p *inlineParser) parseBang() {
	if peek(p.subject, p.pos+1) == '[' {
		p.text("![")
		p.addBracket(p.root.last, p.pos+1, true)
		p.pos += 2
		return
	}
	p.text("!")
	p.pos++
}

func (p *inlineParser) addBracket(n *inode, index int, image bool) {
	if p.brackets != nil {
		p.brackets.bracketAfter = true
	}
	p.brackets = &bracket{node: n, prev: p.brackets, prevDelimiter: p.delimiters, index: index, image: image, active: true}
}

func (p *inlineParser) removeBracket() { p.brackets = p.brackets.prev }

// handleDelim reads a run of c and remembers it as a delimiter when it can
// open or close.
func (p *inlineParser) handleDelim(c byte) {
	start := p.pos
	n := runLength(p.subject, start, c)
	canOpen, canClose := p.flanking(c, start, n)
	p.pos += n
	node := p.text(p.subject[start:p.pos])
	if !canOpen && !canClose {
		return
	}
	if c == '~' && n > 2 {
		// Strikethrough takes one or two tildes; more are text.
		return
	}
	d := &delimiter{char: c, count: n, origCount: n, node: node, prev: p.delimiters, canOpen: canOpen, canClose: canClose}
	if d.prev != nil {
		d.prev.next = d
	}
	p.delimiters = d
}

// flanking decides whether the run of n c at start can open and close
// emphasis, by the rules of left- and right-flanking delimiter runs. Wide
// characters next to the run count as punctuation, so that **「強調」**の
// works in Japanese, where words have no spaces between them. See
// docs/ADR/0099-markdown-module.md.
func (p *inlineParser) flanking(c byte, start, n int) (canOpen, canClose bool) {
	before, after := '\n', '\n'
	if start > 0 {
		before, _ = utf8.DecodeLastRuneInString(p.subject[:start])
	}
	if start+n < len(p.subject) {
		after, _ = utf8.DecodeRuneInString(p.subject[start+n:])
	}
	afterSpace, afterPunct := unicode.IsSpace(after), isPunct(after) || isWide(after)
	beforeSpace, beforePunct := unicode.IsSpace(before), isPunct(before) || isWide(before)
	left := !afterSpace && (!afterPunct || beforeSpace || beforePunct)
	right := !beforeSpace && (!beforePunct || afterSpace || afterPunct)
	if c == '_' {
		return left && (!right || beforePunct), right && (!left || afterPunct)
	}
	return left, right
}

func (p *inlineParser) removeDelimiter(d *delimiter) {
	if d.prev != nil {
		d.prev.next = d.next
	}
	if d.next == nil {
		p.delimiters = d.prev
	} else {
		d.next.prev = d.prev
	}
}

// openersBottomIndex sorts the lower bounds of the search for openers, as
// the specification keeps one for each kind of closer.
func openersBottomIndex(d *delimiter) int {
	switch d.char {
	case '*':
		return boolIndex(d.canOpen)*3 + d.origCount%3
	case '_':
		return 6 + boolIndex(d.canOpen)*3 + d.origCount%3
	}
	return 12 + d.origCount%2
}

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

// processEmphasis matches the delimiters above stackBottom into emphasis,
// strong emphasis and strikethrough, and removes them.
func (p *inlineParser) processEmphasis(stackBottom *delimiter) {
	var openersBottom [14]*delimiter
	for i := range openersBottom {
		openersBottom[i] = stackBottom
	}
	closer := p.delimiters
	for closer != nil && closer.prev != stackBottom {
		closer = closer.prev
	}
	for closer != nil {
		if !closer.canClose {
			closer = closer.next
			continue
		}
		idx := openersBottomIndex(closer)
		opener := closer.prev
		found := false
		for opener != nil && opener != stackBottom && opener != openersBottom[idx] {
			if opener.char == closer.char && opener.canOpen && p.compatible(opener, closer) {
				found = true
				break
			}
			opener = opener.prev
		}
		old := closer
		if found {
			closer = p.matchDelimiters(opener, closer)
		} else {
			closer = closer.next
			openersBottom[idx] = old.prev
			if !old.canOpen {
				p.removeDelimiter(old)
			}
		}
	}
	for p.delimiters != nil && p.delimiters != stackBottom {
		p.removeDelimiter(p.delimiters)
	}
}

// compatible applies the rule of three to * and _, and asks tildes to come
// in runs of the same length.
func (p *inlineParser) compatible(opener, closer *delimiter) bool {
	if closer.char == '~' {
		return opener.count == closer.count
	}
	odd := (opener.canClose || closer.canOpen) && closer.origCount%3 != 0 && (opener.origCount+closer.origCount)%3 == 0
	return !odd
}

// matchDelimiters wraps what lies between opener and closer in emphasis and
// returns the closer to look at next.
func (p *inlineParser) matchDelimiters(opener, closer *delimiter) *delimiter {
	use := 1
	kind := inlineEmph
	switch {
	case closer.char == '~':
		use, kind = closer.count, inlineStrike
	case closer.count >= 2 && opener.count >= 2:
		use, kind = 2, inlineStrong
	}
	opener.count -= use
	closer.count -= use
	openNode, closeNode := opener.node, closer.node
	openNode.text = openNode.text[:len(openNode.text)-use]
	closeNode.text = closeNode.text[:len(closeNode.text)-use]

	em := &inode{kind: kind}
	for tmp := openNode.next; tmp != nil && tmp != closeNode; {
		next := tmp.next
		em.appendChild(tmp)
		tmp = next
	}
	openNode.insertAfter(em)

	// The delimiters between the two are inside the emphasis now, and can
	// no longer match anything outside it.
	for d := closer.prev; d != nil && d != opener; {
		prev := d.prev
		p.removeDelimiter(d)
		d = prev
	}
	if opener.count == 0 {
		openNode.unlink()
		p.removeDelimiter(opener)
	}
	if closer.count == 0 {
		closeNode.unlink()
		next := closer.next
		p.removeDelimiter(closer)
		return next
	}
	return closer
}

// isPunct reports whether r is Unicode punctuation as the specification
// counts it: the P and S categories.
func isPunct(r rune) bool {
	if r < utf8.RuneSelf {
		return isASCIIPunct(byte(r))
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func isWhitespaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

// parseRawHTML reads raw HTML at pos, as CommonMark finds it. A comment, a
// processing instruction, a declaration or CDATA shows nothing, as in a
// browser. The tags of the elements that text can hold (inlineTagName) are
// kept for pairTags; any other tag stays the text it is. See
// docs/ADR/0119-markdown-html-subset.md.
func (p *inlineParser) parseRawHTML() bool {
	s := p.subject[p.pos:]
	if k := markupKind(s); k >= 0 && !p.unclosed[k] {
		if n := markupEnd(s, false); n > 0 {
			p.pos += n
			return true
		}
		p.unclosed[k] = true
	}
	tag, n := scanTag(s, true)
	if n == 0 {
		return false
	}
	p.pos += n
	if !inlineTagName(tag.name) {
		p.text(tag.raw)
		return true
	}
	p.root.appendChild(&inode{kind: inlineTag, tag: &tag})
	p.hasTags = true
	return true
}
