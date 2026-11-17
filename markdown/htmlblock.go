package markdown

import (
	"strings"
)

// htmlBlockNames are the names that start an HTML block of type 6 in
// CommonMark: the block-level elements of HTML.
var htmlBlockNames = map[string]bool{
	"address": true, "article": true, "aside": true, "base": true, "basefont": true, "blockquote": true,
	"body": true, "caption": true, "center": true, "col": true, "colgroup": true, "dd": true,
	"details": true, "dialog": true, "dir": true, "div": true, "dl": true, "dt": true,
	"fieldset": true, "figcaption": true, "figure": true, "footer": true, "form": true, "frame": true,
	"frameset": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"head": true, "header": true, "hr": true, "html": true, "iframe": true, "legend": true,
	"li": true, "link": true, "main": true, "menu": true, "menuitem": true, "nav": true,
	"noframes": true, "ol": true, "optgroup": true, "option": true, "p": true, "param": true,
	"search": true, "section": true, "summary": true, "table": true, "tbody": true, "td": true,
	"tfoot": true, "th": true, "thead": true, "title": true, "tr": true, "track": true, "ul": true,
}

// rawTextNames are the elements whose content is text up to their closing
// tag (HTML blocks of type 1).
var rawTextNames = [...]string{"pre", "script", "style", "textarea"}

// startHTMLBlock opens an HTML block, by the start conditions of CommonMark
// (4.6), so that the region of HTML is the one GitHub finds. The block is a
// leaf that keeps its lines as they are; buildHTML interprets it after the
// blocks are parsed.
func startHTMLBlock(p *blockParser, container *block) int {
	if p.indented || peek(p.line, p.nextNonspace) != '<' {
		return startNone
	}
	t := htmlBlockType(p.line[p.nextNonspace:])
	if t == 0 {
		return startNone
	}
	if t == 7 && (container.kind == blockParagraph || (!p.allClosed && !p.blank && p.tip.kind == blockParagraph)) {
		// Only the types 1 to 6 can interrupt a paragraph.
		return startNone
	}
	p.closeUnmatchedBlocks()
	// The offset stays: the spaces before the tag belong to the block.
	b := p.addChild(blockHTML)
	b.htmlType = t
	return startLeaf
}

// htmlBlockType returns the type, from 1 to 7, of the HTML block that the
// line s starts, or 0 if it starts none.
func htmlBlockType(s string) int {
	lower := strings.ToLower(s[:min(len(s), 16)])
	for _, n := range rawTextNames {
		if strings.HasPrefix(lower, "<"+n) && endsName(lower, len(n)+1) {
			return 1
		}
	}
	switch {
	case strings.HasPrefix(s, "<!--"):
		return 2
	case strings.HasPrefix(s, "<?"):
		return 3
	case strings.HasPrefix(s, "<![CDATA["):
		return 5
	case len(s) > 2 && s[1] == '!' && isAlpha(s[2]):
		return 4
	}
	i := 1
	if peek(s, i) == '/' {
		i++
	}
	j := i
	for j < len(s) && isAlnum(s[j]) {
		j++
	}
	if j > i && htmlBlockNames[strings.ToLower(s[i:j])] {
		if j == len(s) || isSpaceOrTab(s[j]) || s[j] == '>' || strings.HasPrefix(s[j:], "/>") {
			return 6
		}
	}
	tag, n := scanTag(s, true)
	if n > 0 && isBlankFrom(s, n) {
		for _, raw := range rawTextNames {
			if tag.name == raw {
				return 0
			}
		}
		return 7
	}
	return 0
}

// endsName reports whether the name in s ends at i: at the end of the line,
// a space, a tab or '>'.
func endsName(s string, i int) bool {
	return i >= len(s) || isSpaceOrTab(s[i]) || s[i] == '>'
}

// htmlBlockEnds reports whether the line s meets the end condition of an
// HTML block of type t, from 1 to 5. The types 6 and 7 end at a blank line.
func htmlBlockEnds(t int, s string) bool {
	switch t {
	case 1:
		lower := strings.ToLower(s)
		for _, n := range rawTextNames {
			if strings.Contains(lower, "</"+n+">") {
				return true
			}
		}
		return false
	case 2:
		return strings.Contains(s, "-->")
	case 3:
		return strings.Contains(s, "?>")
	case 4:
		return strings.IndexByte(s, '>') >= 0
	case 5:
		return strings.Contains(s, "]]>")
	}
	return false
}
