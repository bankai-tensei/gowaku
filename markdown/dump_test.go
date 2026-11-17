package markdown

import (
	"strconv"
	"strings"
)

// dump writes the tree in a short form for table tests:
//
//	h1(Foo) p(bar *baz*) ul{li{p(a)} li{p(b)}} code[go](x := 1↵)
//
// Inline: *em*, **strong**, ~~strike~~, `code`, [text](dest "title"),
// ![alt](src), ⏎ for a soft break and <br> for a hard one.
//
// From HTML: p[c](…) and h1[r](…) for an align, details[open](summary){…},
// htable[h]({blocks} | {blocks} / …) for a table in HTML, <kbd>k</kbd>,
// <sub>…</sub>, <sup>…</sup>, and ![alt](src 280x40% dark=d.svg light=l.svg)
// for the size and the sources of an image.
func dump(doc *block) string {
	var b strings.Builder
	dumpBlocks(&b, doc.children)
	return b.String()
}

func dumpBlocks(b *strings.Builder, blocks []*block) {
	for i, c := range blocks {
		if i > 0 {
			b.WriteByte(' ')
		}
		dumpBlock(b, c)
	}
}

func dumpBlock(b *strings.Builder, c *block) {
	switch c.kind {
	case blockParagraph:
		b.WriteString("p" + alignMark(c.textAlign) + "(")
		dumpInlines(b, c.inlines)
		b.WriteString(")")
	case blockHeading:
		b.WriteString("h" + strconv.Itoa(c.level) + alignMark(c.textAlign) + "(")
		dumpInlines(b, c.inlines)
		b.WriteString(")")
	case blockDetails:
		b.WriteString("details")
		if c.startsOpen {
			b.WriteString("[open]")
		}
		b.WriteString("(")
		dumpInlines(b, c.summary)
		b.WriteString("){")
		dumpBlocks(b, c.children)
		b.WriteString("}")
	case blockHTML:
		b.WriteString("html(" + strings.ReplaceAll(string(c.content), "\n", "↵") + ")")
	case blockTable:
		if c.cells == nil {
			dumpTable(b, c)
			break
		}
		b.WriteString("htable")
		if c.header {
			b.WriteString("[h]")
		}
		b.WriteString("(")
		for i, row := range c.cells {
			if i > 0 {
				b.WriteString(" / ")
			}
			for j, cell := range row {
				if j > 0 {
					b.WriteString(" | ")
				}
				b.WriteString("{")
				dumpBlocks(b, cell)
				b.WriteString("}")
			}
		}
		b.WriteString(")")
	case blockCode:
		b.WriteString("code")
		if c.info != "" {
			b.WriteString("[" + c.info + "]")
		}
		b.WriteString("(" + strings.ReplaceAll(string(c.content), "\n", "↵") + ")")
	case blockQuote:
		b.WriteString("quote{")
		dumpBlocks(b, c.children)
		b.WriteString("}")
	case blockList:
		if c.ordered {
			b.WriteString("ol")
			if c.start != 1 {
				b.WriteString(strconv.Itoa(c.start))
			}
		} else {
			b.WriteString("ul")
		}
		if !c.tight {
			b.WriteString(".loose")
		}
		b.WriteString("{")
		dumpBlocks(b, c.children)
		b.WriteString("}")
	case blockItem:
		switch {
		case c.task && c.checked:
			b.WriteString("li[x]{")
		case c.task:
			b.WriteString("li[ ]{")
		default:
			b.WriteString("li{")
		}
		dumpBlocks(b, c.children)
		b.WriteString("}")
	case blockRule:
		b.WriteString("hr")
	}
}

func dumpTable(b *strings.Builder, c *block) {
	b.WriteString("table[")
	for i, a := range c.aligns {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString([...]string{"-", "l", "c", "r"}[a])
	}
	b.WriteString("](")
	for i, row := range c.rows {
		if i > 0 {
			b.WriteString(" / ")
		}
		for j, cell := range row {
			if j > 0 {
				b.WriteString(" | ")
			}
			dumpInlines(b, cell)
		}
	}
	b.WriteString(")")
}

func dumpInlines(b *strings.Builder, ins []inline) {
	for _, in := range ins {
		switch in.kind {
		case inlineText:
			b.WriteString(in.text)
		case inlineCode:
			b.WriteString("`" + in.text + "`")
		case inlineEmph:
			b.WriteString("*")
			dumpInlines(b, in.children)
			b.WriteString("*")
		case inlineStrong:
			b.WriteString("**")
			dumpInlines(b, in.children)
			b.WriteString("**")
		case inlineStrike:
			b.WriteString("~~")
			dumpInlines(b, in.children)
			b.WriteString("~~")
		case inlineLink, inlineImage:
			if in.kind == inlineImage {
				b.WriteString("!")
			}
			b.WriteString("[")
			dumpInlines(b, in.children)
			b.WriteString("](" + in.dest)
			if in.title != "" {
				b.WriteString(" \"" + in.title + "\"")
			}
			if in.width > 0 || in.height > 0 {
				b.WriteString(" ")
				if in.width > 0 {
					b.WriteString(strconv.FormatFloat(float64(in.width), 'g', -1, 32))
				}
				if in.widthPercent {
					b.WriteString("%")
				}
				b.WriteString("x")
				if in.height > 0 {
					b.WriteString(strconv.FormatFloat(float64(in.height), 'g', -1, 32))
				}
			}
			for _, s := range in.sources {
				b.WriteString(" " + [...]string{"any", "light", "dark"}[s.scheme] + "=" + s.src)
			}
			b.WriteString(")")
		case inlineKbd:
			b.WriteString("<kbd>" + in.text + "</kbd>")
		case inlineSub:
			b.WriteString("<sub>")
			dumpInlines(b, in.children)
			b.WriteString("</sub>")
		case inlineSup:
			b.WriteString("<sup>")
			dumpInlines(b, in.children)
			b.WriteString("</sup>")
		case inlineTag:
			b.WriteString("TAG(" + in.tag.raw + ")")
		case inlineSoftBreak:
			b.WriteString("⏎")
		case inlineHardBreak:
			b.WriteString("<br>")
		}
	}
}

// alignMark writes an align as a mark after the name of a block.
func alignMark(a align) string {
	return [...]string{"", "[l]", "[c]", "[r]"}[a]
}
