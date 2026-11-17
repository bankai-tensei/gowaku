package markdown

import (
	"bytes"
	"strings"
)

// The results of a block start.
const (
	startNone = iota
	// startContainer opened a container; more blocks may start inside it.
	startContainer
	// startLeaf opened a leaf that takes the rest of the line.
	startLeaf
)

// tryStarts tries the block starts, in the order of the specification, on
// what remains of the line, and reports what the first that matched opened.
func (p *blockParser) tryStarts(container *block) int {
	if r := startQuote(p, container); r != startNone {
		return r
	}
	if r := startATXHeading(p, container); r != startNone {
		return r
	}
	if r := startFence(p, container); r != startNone {
		return r
	}
	if r := startHTMLBlock(p, container); r != startNone {
		return r
	}
	// A table comes before a setext heading: "a | b" over "--- | ---" is a
	// table, as on GitHub.
	if r := startTable(p, container); r != startNone {
		return r
	}
	if r := startSetextHeading(p, container); r != startNone {
		return r
	}
	if r := startRule(p, container); r != startNone {
		return r
	}
	if r := startListItem(p, container); r != startNone {
		return r
	}
	return startIndentedCode(p, container)
}

func startQuote(p *blockParser, container *block) int {
	if p.indented || peek(p.line, p.nextNonspace) != '>' || nesting(container) >= maxNesting {
		return startNone
	}
	p.advanceNextNonspace()
	p.advanceOffset(1, false)
	if isSpaceOrTab(peek(p.line, p.offset)) {
		p.advanceOffset(1, true)
	}
	p.closeUnmatchedBlocks()
	p.addChild(blockQuote)
	return startContainer
}

func startATXHeading(p *blockParser, _ *block) int {
	if p.indented {
		return startNone
	}
	rest := p.line[p.nextNonspace:]
	level := runLength(rest, 0, '#')
	if level < 1 || level > 6 || (level < len(rest) && !isSpaceOrTab(rest[level])) {
		return startNone
	}
	p.advanceNextNonspace()
	p.advanceOffset(level, false)
	p.closeUnmatchedBlocks()
	h := p.addChild(blockHeading)
	h.level = level
	h.content = append(h.content, trimClosingHashes(p.line[p.offset:])...)
	p.advanceOffset(len(p.line)-p.offset, false)
	return startLeaf
}

// trimClosingHashes removes the optional closing sequence of an ATX heading.
func trimClosingHashes(s string) string {
	s = strings.TrimRight(s, " \t")
	end := len(s)
	for end > 0 && s[end-1] == '#' {
		end--
	}
	switch {
	case end == 0:
		return ""
	case end < len(s) && isSpaceOrTab(s[end-1]):
		return strings.TrimRight(s[:end], " \t")
	}
	return s
}

func startFence(p *blockParser, _ *block) int {
	if p.indented {
		return startNone
	}
	c := peek(p.line, p.nextNonspace)
	if c != '`' && c != '~' {
		return startNone
	}
	n := runLength(p.line, p.nextNonspace, c)
	if n < 3 || (c == '`' && strings.IndexByte(p.line[p.nextNonspace+n:], '`') >= 0) {
		return startNone
	}
	p.closeUnmatchedBlocks()
	b := p.addChild(blockCode)
	b.fenced, b.fenceChar, b.fenceLen, b.fenceOffset = true, c, n, p.indent
	p.advanceNextNonspace()
	p.advanceOffset(n, false)
	return startLeaf
}

func startTable(p *blockParser, container *block) int {
	if p.indented || container.kind != blockParagraph {
		return startNone
	}
	row := p.line[p.nextNonspace:]
	aligns, ok := parseDelimiterRow(row)
	if !ok {
		return startNone
	}
	// Only the last line of the paragraph can be the header; the lines
	// before it stay a paragraph.
	content := bytes.TrimSuffix(container.content, []byte("\n"))
	i := bytes.LastIndexByte(content, '\n')
	header, before := string(content[i+1:]), content[:i+1]
	cells := splitRow(header)
	if len(cells) != len(aligns) || (!hasPipe(header) && !hasPipe(row)) {
		return startNone
	}
	p.closeUnmatchedBlocks()
	if len(bytes.TrimSpace(before)) == 0 {
		// The table takes the place of the paragraph.
		parent := container.parent
		parent.children = parent.children[:len(parent.children)-1]
		p.tip = parent
	} else {
		container.content = before
		p.finalize(container, p.lineNumber-2)
	}
	t := p.addChild(blockTable)
	t.startLine = p.lineNumber - 1
	t.aligns = aligns
	t.rawRows = append(t.rawRows, cells)
	p.advanceOffset(len(p.line)-p.offset, false)
	return startLeaf
}

func startSetextHeading(p *blockParser, container *block) int {
	if p.indented || container.kind != blockParagraph {
		return startNone
	}
	rest := p.line[p.nextNonspace:]
	c := peek(rest, 0)
	if c != '=' && c != '-' {
		return startNone
	}
	if !isBlankFrom(rest, runLength(rest, 0, c)) {
		return startNone
	}
	p.closeUnmatchedBlocks()
	content := p.consumeReferences(string(container.content))
	if strings.TrimSpace(content) == "" {
		container.content = container.content[:0]
		return startNone
	}
	// The link reference definitions taken from the top of the paragraph
	// are not part of the heading, which starts on the line after them.
	refLines := strings.Count(string(container.content[:len(container.content)-len(content)]), "\n")
	h := &block{kind: blockHeading, parent: container.parent, startLine: container.startLine + refLines, open: true, level: 2}
	if c == '=' {
		h.level = 1
	}
	h.content = []byte(content)
	siblings := container.parent.children
	siblings[len(siblings)-1] = h
	p.tip = h
	p.advanceOffset(len(p.line)-p.offset, false)
	return startLeaf
}

func startRule(p *blockParser, _ *block) int {
	if p.indented || !isThematicBreak(p.line[p.nextNonspace:]) {
		return startNone
	}
	p.closeUnmatchedBlocks()
	p.addChild(blockRule)
	p.advanceOffset(len(p.line)-p.offset, false)
	return startLeaf
}

func startListItem(p *blockParser, container *block) int {
	if p.indented && container.kind != blockList {
		return startNone
	}
	if nesting(container) >= maxNesting {
		return startNone
	}
	data, ok := p.parseListMarker(container)
	if !ok {
		return startNone
	}
	p.closeUnmatchedBlocks()
	if p.tip.kind != blockList || !listsMatch(container, &data) {
		l := p.addChild(blockList)
		l.ordered, l.bulletChar, l.delimiter, l.start = data.ordered, data.bulletChar, data.delimiter, data.start
		l.markerOffset, l.padding = data.markerOffset, data.padding
	}
	it := p.addChild(blockItem)
	it.ordered, it.bulletChar, it.delimiter, it.start = data.ordered, data.bulletChar, data.delimiter, data.start
	it.markerOffset, it.padding = data.markerOffset, data.padding
	return startContainer
}

func startIndentedCode(p *blockParser, _ *block) int {
	if !p.indented || p.blank || p.tip.kind == blockParagraph || p.tip.kind == blockTable {
		return startNone
	}
	p.advanceOffset(codeIndent, true)
	p.closeUnmatchedBlocks()
	p.addChild(blockCode)
	return startLeaf
}

// parseListMarker reads a list marker at nextNonspace, and takes it and the
// spaces after it.
func (p *blockParser) parseListMarker(container *block) (block, bool) {
	var data block
	if p.indent >= codeIndent {
		return data, false
	}
	rest := p.line[p.nextNonspace:]
	n := 0
	switch c := peek(rest, 0); {
	case c == '*' || c == '+' || c == '-':
		data.bulletChar = c
		n = 1
	case isDigit(c):
		for n < len(rest) && n < 9 && isDigit(rest[n]) {
			data.start = data.start*10 + int(rest[n]-'0')
			n++
		}
		d := peek(rest, n)
		if (d != '.' && d != ')') || (container.kind == blockParagraph && data.start != 1) {
			return data, false
		}
		data.ordered, data.delimiter = true, d
		n++
	default:
		return data, false
	}
	if n < len(rest) && !isSpaceOrTab(rest[n]) {
		return data, false
	}
	if container.kind == blockParagraph && isBlankFrom(rest, n) {
		// An empty item cannot interrupt a paragraph.
		return data, false
	}
	data.markerOffset = p.indent
	p.advanceNextNonspace()
	p.advanceOffset(n, true)
	spacesStartCol, spacesStartOffset := p.column, p.offset
	for {
		p.advanceOffset(1, true)
		if p.column-spacesStartCol >= 5 || !isSpaceOrTab(peek(p.line, p.offset)) {
			break
		}
	}
	blankItem := p.offset >= len(p.line)
	spaces := p.column - spacesStartCol
	if spaces >= 5 || spaces < 1 || blankItem {
		// The content starts one space after the marker; the other spaces
		// belong to it, as for an indented code block in the item.
		data.padding = n + 1
		p.column, p.offset = spacesStartCol, spacesStartOffset
		p.partiallyConsumedTab = false
		if isSpaceOrTab(peek(p.line, p.offset)) {
			p.advanceOffset(1, true)
		}
	} else {
		data.padding = n + spaces
	}
	return data, true
}

func listsMatch(l, item *block) bool {
	return l.kind == blockList && l.ordered == item.ordered && l.delimiter == item.delimiter && l.bulletChar == item.bulletChar
}

// nesting counts the block quotes, list items and details around b and b
// itself.
func nesting(b *block) int {
	n := 0
	for ; b != nil; b = b.parent {
		if b.kind == blockQuote || b.kind == blockItem || b.kind == blockDetails {
			n++
		}
	}
	return n
}
