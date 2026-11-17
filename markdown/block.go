package markdown

import (
	"strings"
)

const (
	// maxNesting bounds how many block quotes and list items can nest. A
	// marker past it is text, so that adversarial input cannot build a tree,
	// and a widget tree, as deep as it is long.
	maxNesting = 32
	// codeIndent is the indentation of an indented code block.
	codeIndent = 4
	// tabStop is the distance between tab stops when a tab indents a line.
	tabStop = 4
)

// blockParser turns lines into the block tree, after the parsing strategy of
// the CommonMark specification (appendix A): each line first continues the
// open blocks it can, then may start new ones, and what remains is text.
type blockParser struct {
	doc, tip, oldtip *block
	line             string
	lineNumber       int

	offset, column     int
	nextNonspace       int
	nextNonspaceColumn int
	indent             int
	indented, blank    bool
	// partiallyConsumedTab means that the tab at offset was taken in part
	// as indentation, and the rest of it counts as spaces of content.
	partiallyConsumedTab bool
	// lineTabs says that the line has tabs. Without them, wsFrom..wsEnd is
	// a run of spaces that findNextNonspace measured.
	lineTabs      bool
	wsFrom, wsEnd int

	allClosed            bool
	lastMatchedContainer *block

	refs map[string]linkRef
}

// linkRef is the destination of a link reference definition.
type linkRef struct {
	dest, title string
}

// parse parses a Markdown document. It never fails: any input is some
// document.
func parse(src string) *block {
	src = normalizeSource(src)
	doc := &block{kind: blockDocument, open: true, startLine: 1}
	p := &blockParser{doc: doc, tip: doc, oldtip: doc, refs: map[string]linkRef{}}
	lines := strings.Split(src, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		// A final line break ends the last line; it does not start another.
		lines = lines[:n-1]
	}
	for _, ln := range lines {
		p.incorporateLine(ln)
	}
	for p.tip != nil {
		p.finalize(p.tip, len(lines))
	}
	p.processInlines(doc)
	return doc
}

// normalizeSource turns every line ending into \n and replaces what cannot be
// shown: NUL, as the specification asks, and invalid UTF-8.
func normalizeSource(src string) string {
	src = strings.ToValidUTF8(src, "�")
	if strings.IndexByte(src, '\r') >= 0 {
		src = strings.ReplaceAll(src, "\r\n", "\n")
		src = strings.ReplaceAll(src, "\r", "\n")
	}
	if strings.IndexByte(src, 0) >= 0 {
		src = strings.ReplaceAll(src, "\x00", "�")
	}
	return src
}

func (p *blockParser) incorporateLine(ln string) {
	container := p.doc
	p.oldtip = p.tip
	p.offset, p.column = 0, 0
	p.blank, p.partiallyConsumedTab = false, false
	p.lineNumber++
	p.line = ln
	p.lineTabs = strings.IndexByte(ln, '\t') >= 0
	p.wsFrom, p.wsEnd = -1, -2

	// Continue the open blocks the line belongs to.
	allMatched := true
	for {
		n := len(container.children)
		if n == 0 || !container.children[n-1].open {
			break
		}
		container = container.children[n-1]
		p.findNextNonspace()
		switch p.continues(container) {
		case continueNo:
			allMatched = false
		case continueDone:
			return
		}
		if !allMatched {
			container = container.parent
			break
		}
	}
	p.allClosed = container == p.oldtip
	p.lastMatchedContainer = container

	// Start new blocks.
	matchedLeaf := container.kind == blockCode || container.kind == blockHTML
	for !matchedLeaf {
		p.findNextNonspace()
		if !p.indented && !maybeSpecial(p.line, p.nextNonspace) {
			p.advanceNextNonspace()
			break
		}
		started := p.tryStarts(container)
		if started == startNone {
			p.advanceNextNonspace()
			break
		}
		container = p.tip
		if started == startLeaf {
			matchedLeaf = true
		}
	}

	// What remains of the line is text.
	if !p.allClosed && !p.blank && p.tip.kind == blockParagraph {
		// A lazy continuation line of a paragraph.
		p.addLine()
		return
	}
	p.closeUnmatchedBlocks()
	switch {
	case container.kind == blockHTML:
		p.addLine()
		if container.htmlType <= 5 && htmlBlockEnds(container.htmlType, p.line[min(p.offset, len(p.line)):]) {
			p.finalize(container, p.lineNumber)
		}
	case container.kind == blockParagraph || container.kind == blockCode || container.kind == blockTable:
		p.addLine()
	case p.offset < len(p.line) && !p.blank:
		p.addChild(blockParagraph)
		p.advanceNextNonspace()
		p.addLine()
	}
}

// The results of continues.
const (
	continueYes = iota
	continueNo
	// continueDone means that the line closed the block and nothing is left.
	continueDone
)

// continues reports whether the line continues the open block b, and takes
// the marker that says so.
func (p *blockParser) continues(b *block) int {
	switch b.kind {
	case blockQuote:
		if !p.indented && peek(p.line, p.nextNonspace) == '>' {
			p.advanceNextNonspace()
			p.advanceOffset(1, false)
			if isSpaceOrTab(peek(p.line, p.offset)) {
				p.advanceOffset(1, true)
			}
			return continueYes
		}
		return continueNo
	case blockItem:
		switch {
		case p.blank:
			if len(b.children) == 0 {
				// A blank line after an empty item ends it.
				return continueNo
			}
			p.advanceNextNonspace()
		case p.indent >= b.markerOffset+b.padding:
			p.advanceOffset(b.markerOffset+b.padding, true)
		default:
			return continueNo
		}
		return continueYes
	case blockHeading, blockRule:
		return continueNo
	case blockCode:
		return p.continuesCode(b)
	case blockParagraph, blockTable:
		if p.blank {
			return continueNo
		}
		return continueYes
	case blockHTML:
		if p.blank && b.htmlType >= 6 {
			return continueNo
		}
		return continueYes
	}
	// The document and lists.
	return continueYes
}

func (p *blockParser) continuesCode(b *block) int {
	if b.fenced {
		if p.indent <= 3 && peek(p.line, p.nextNonspace) == b.fenceChar {
			n := runLength(p.line, p.nextNonspace, b.fenceChar)
			if n >= b.fenceLen && isBlankFrom(p.line, p.nextNonspace+n) {
				b.closed = true
				p.finalize(b, p.lineNumber)
				return continueDone
			}
		}
		// Skip the indentation of the opening fence.
		for i := b.fenceOffset; i > 0 && isSpaceOrTab(peek(p.line, p.offset)); i-- {
			p.advanceOffset(1, true)
		}
		return continueYes
	}
	switch {
	case p.indent >= codeIndent:
		p.advanceOffset(codeIndent, true)
	case p.blank:
		p.advanceNextNonspace()
	default:
		return continueNo
	}
	return continueYes
}

func (p *blockParser) findNextNonspace() {
	i, cols := p.offset, p.column
	if !p.lineTabs && p.offset >= p.wsFrom && p.offset <= p.wsEnd {
		// The run of spaces was measured for an outer container already;
		// a line indented by thousands of spaces is not scanned once per
		// container.
		i = p.wsEnd
		cols += i - p.offset
	} else {
		for ; i < len(p.line) && isSpaceOrTab(p.line[i]); i++ {
			if p.line[i] == '\t' {
				cols += tabStop - cols%tabStop
			} else {
				cols++
			}
		}
		p.wsFrom, p.wsEnd = p.offset, i
	}
	p.blank = i >= len(p.line)
	p.nextNonspace, p.nextNonspaceColumn = i, cols
	p.indent = cols - p.column
	p.indented = p.indent >= codeIndent
}

func (p *blockParser) advanceNextNonspace() {
	p.offset, p.column = p.nextNonspace, p.nextNonspaceColumn
	p.partiallyConsumedTab = false
}

// advanceOffset moves past count bytes, or count columns if columns is set,
// in which case a tab can be taken in part.
func (p *blockParser) advanceOffset(count int, columns bool) {
	for count > 0 && p.offset < len(p.line) {
		if p.line[p.offset] != '\t' {
			p.partiallyConsumedTab = false
			p.offset++
			p.column++
			count--
			continue
		}
		toTab := tabStop - p.column%tabStop
		if !columns {
			p.partiallyConsumedTab = false
			p.column += toTab
			p.offset++
			count--
			continue
		}
		p.partiallyConsumedTab = toTab > count
		step := min(toTab, count)
		p.column += step
		if !p.partiallyConsumedTab {
			p.offset++
		}
		count -= step
	}
}

// addLine adds what remains of the line to the tip.
func (p *blockParser) addLine() {
	t := p.tip
	if p.partiallyConsumedTab {
		// The rest of the tab is spaces of content.
		p.offset++
		for range tabStop - p.column%tabStop {
			t.content = append(t.content, ' ')
		}
	}
	rest := ""
	if p.offset < len(p.line) {
		rest = p.line[p.offset:]
	}
	if t.kind == blockTable {
		// The delimiter row that started the table leaves nothing.
		if rest != "" {
			t.rawRows = append(t.rawRows, splitRow(rest))
		}
		return
	}
	t.content = append(t.content, rest...)
	t.content = append(t.content, '\n')
}

// addChild opens a block of kind at the tip, closing the blocks that cannot
// hold it.
func (p *blockParser) addChild(kind blockKind) *block {
	for !canContain(p.tip.kind, kind) {
		p.finalize(p.tip, p.lineNumber-1)
	}
	b := &block{kind: kind, parent: p.tip, startLine: p.lineNumber, open: true}
	p.tip.children = append(p.tip.children, b)
	p.tip = b
	return b
}

func canContain(parent, child blockKind) bool {
	switch parent {
	case blockDocument, blockQuote, blockItem:
		return child != blockItem
	case blockList:
		return child == blockItem
	}
	return false
}

// closeUnmatchedBlocks closes the blocks that the line did not continue.
func (p *blockParser) closeUnmatchedBlocks() {
	if p.allClosed {
		return
	}
	for p.oldtip != p.lastMatchedContainer {
		parent := p.oldtip.parent
		p.finalize(p.oldtip, p.lineNumber-1)
		p.oldtip = parent
	}
	p.allClosed = true
}

// finalize closes b, which ends on line, and makes its parent the tip.
func (p *blockParser) finalize(b *block, line int) {
	b.open = false
	b.endLine = line
	switch b.kind {
	case blockParagraph:
		rest := p.consumeReferences(string(b.content))
		b.content = []byte(rest)
		if strings.TrimSpace(rest) == "" {
			removeChild(b.parent, b)
		}
	case blockCode:
		finalizeCode(b)
	case blockItem:
		if n := len(b.children); n > 0 {
			b.endLine = b.children[n-1].endLine
		} else {
			b.endLine = b.startLine
		}
	case blockList:
		finalizeList(b)
	}
	p.tip = b.parent
}

func finalizeCode(b *block) {
	content := string(b.content)
	if b.fenced {
		first, rest, _ := strings.Cut(content, "\n")
		b.info = unescapeString(strings.TrimSpace(first))
		b.content = []byte(rest)
		return
	}
	// Blank lines at the end of an indented block are not part of it.
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	end := len(lines)
	for end > 0 && strings.Trim(lines[end-1], " \t") == "" {
		end--
	}
	b.endLine -= len(lines) - end
	if end == 0 {
		b.content = b.content[:0]
		return
	}
	b.content = []byte(strings.Join(lines[:end], "\n") + "\n")
}

// finalizeList decides whether the list is tight: no item, and no block
// inside an item, is followed by a blank line before the next one.
func finalizeList(l *block) {
	l.tight = true
	for i, it := range l.children {
		if i+1 < len(l.children) && endsWithBlankLine(it, l.children[i+1]) {
			l.tight = false
			break
		}
		for j, sub := range it.children {
			if j+1 < len(it.children) && endsWithBlankLine(sub, it.children[j+1]) {
				l.tight = false
				break
			}
		}
		if !l.tight {
			break
		}
	}
	if n := len(l.children); n > 0 {
		l.endLine = l.children[n-1].endLine
	}
}

func endsWithBlankLine(b, next *block) bool { return b.endLine != next.startLine-1 }

func removeChild(parent, child *block) {
	if parent == nil {
		return
	}
	for i, c := range parent.children {
		if c == child {
			parent.children = append(parent.children[:i], parent.children[i+1:]...)
			return
		}
	}
}

// consumeReferences takes the link reference definitions at the start of the
// content of a paragraph, records them and returns the rest.
func (p *blockParser) consumeReferences(content string) string {
	for strings.HasPrefix(content, "[") {
		n := parseReference(content, p.refs)
		if n == 0 {
			break
		}
		content = content[n:]
	}
	return content
}

// processInlines parses the inline content of every block below b.
func (p *blockParser) processInlines(b *block) {
	// The HTML blocks go first, from the top down, so that the details they
	// open count toward the nesting of what is inside them.
	for _, c := range b.children {
		if c.kind == blockHTML {
			b.children = buildHTML(b, b.children)
			break
		}
	}
	// The tree is at most maxNesting containers deep, so recursion is bounded.
	for _, c := range b.children {
		switch c.kind {
		case blockParagraph, blockHeading:
			if !c.html {
				c.inlines = parseInlines(strings.TrimSpace(string(c.content)), p.refs)
				c.content = nil
			}
		case blockTable:
			for _, row := range c.cells {
				for _, cell := range row {
					p.processInlines(&block{kind: blockDocument, children: cell})
				}
			}
			c.rows = make([][][]inline, len(c.rawRows))
			for i, raw := range c.rawRows {
				row := make([][]inline, len(c.aligns))
				for j := range row {
					if j < len(raw) {
						row[j] = parseInlines(raw[j], p.refs)
					}
				}
				c.rows[i] = row
			}
			c.rawRows = nil
		case blockItem:
			markTask(c)
			p.processInlines(c)
		default:
			p.processInlines(c)
		}
	}
}

// markTask turns an item whose text starts with [ ] or [x] into a task.
func markTask(it *block) {
	if len(it.children) == 0 || it.children[0].kind != blockParagraph {
		return
	}
	para := it.children[0]
	s := string(para.content)
	if len(s) < 3 || s[0] != '[' || s[2] != ']' || (len(s) > 3 && !isSpaceOrTab(s[3]) && s[3] != '\n') {
		return
	}
	switch s[1] {
	case ' ':
	case 'x', 'X':
		it.checked = true
	default:
		return
	}
	it.task = true
	para.content = []byte(s[3:])
}

// isThematicBreak reports whether s is three or more of the same of *, - or _,
// with only spaces and tabs between them.
func isThematicBreak(s string) bool {
	c := peek(s, 0)
	if c != '*' && c != '-' && c != '_' {
		return false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case c:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
}

// maybeSpecial reports whether a block other than a paragraph can start at
// s[i].
func maybeSpecial(s string, i int) bool {
	switch peek(s, i) {
	case '#', '`', '~', '*', '+', '_', '=', '>', '-', '|', ':', '<',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

// peek returns s[i], or 0 past the end.
func peek(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func runLength(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

func isBlankFrom(s string, i int) bool {
	for ; i < len(s); i++ {
		if !isSpaceOrTab(s[i]) {
			return false
		}
	}
	return true
}

func isSpaceOrTab(c byte) bool { return c == ' ' || c == '\t' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
