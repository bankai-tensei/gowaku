package markdown

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// Heading is a heading of a document.
type Heading struct {
	// Level is from 1 for # to 6 for ######.
	Level int
	// Text is the text of the heading as it shows, without the marks of
	// emphasis, code or links.
	Text string
	// Slug is the anchor of the heading: a link to "#" followed by it goes to
	// the heading. It is made as GitHub makes it (see Headings), so that the
	// links of a README work.
	Slug string
	// Line is the line of the source, from 1, that the heading starts on:
	// the line of the # marks, or the first line of the text of a setext
	// heading (the one above its === or ---). Lines end at "\n", "\r\n" or
	// "\r". A heading written in HTML has the line of its <h1> to <h6> tag.
	// editor.Editor counts its lines from 0: its caret goes to the heading
	// at editor.TextPosition{Line: h.Line - 1}.
	Line int
	// Offset is the byte offset in the source of the start of Line, for an
	// editor that places its caret by offset. On that line the heading may
	// come after the marks of a block quote or a list item.
	Offset int
}

// Headings returns the headings of the document source in order, those in
// block quotes and list items included, for an outline. It parses source;
// Controller.Headings returns the same list for the document a Markdown
// shows, without parsing it again. Line and Offset say where each heading is
// in source, so that an outline can also move the caret of a source editor.
//
// The slug of a heading is made as GitHub makes it: the text is lowercased,
// each space becomes "-", and every character other than a letter, a digit, a
// combining mark, "-" and "_" is dropped, so CJK text stays as it is while
// punctuation, symbols and emoji go. A slug that an earlier heading already
// has gets "-1", "-2" and so on. For example:
//
//	# Getting Started         getting-started
//	## API: v2.0 (beta)       api-v20-beta
//	## 見出し                  見出し
//	## はじめに「概要」         はじめに概要
//	## Go 言語の UI           go-言語の-ui
//	## 見出し                  見出し-1
//	## FAQ!                   faq
//	## FAQ?                   faq-1
func Headings(source string) []Heading {
	doc := parse(source)
	assignSlugs(doc)
	return headingsOf(doc, source)
}

// headingsOf lists the headings of doc, parsed from source, whose slugs
// assignSlugs has set.
func headingsOf(doc *block, source string) []Heading {
	var out []Heading
	// line and off walk the source forward to the start of each heading's
	// line. Document order is source order, so this is one pass; a heading
	// before the last one starts the walk over all the same.
	line, off := 1, 0
	walkHeadings(doc, func(b *block) {
		if b.startLine < line {
			line, off = 1, 0
		}
		for line < b.startLine && off < len(source) {
			off = nextLine(source, off)
			line++
		}
		out = append(out, Heading{Level: b.level, Text: headingText(b), Slug: b.slug, Line: b.startLine, Offset: off})
	})
	return out
}

// nextLine returns the offset of the line after the one off is on, or
// len(source) on the last line. A line ends at "\n", "\r\n" or "\r", as
// normalizeSource counts them.
func nextLine(source string, off int) int {
	i := strings.IndexAny(source[off:], "\r\n")
	if i < 0 {
		return len(source)
	}
	i += off
	if source[i] == '\r' && i+1 < len(source) && source[i+1] == '\n' {
		i++
	}
	return i + 1
}

// walkHeadings calls fn for each heading below b, in document order: those in
// details and in the cells of a table written in HTML too.
func walkHeadings(b *block, fn func(*block)) {
	for _, c := range b.children {
		if c.kind == blockHeading {
			fn(c)
			continue
		}
		walkHeadings(c, fn)
		for _, row := range c.cells {
			for _, cell := range row {
				walkHeadings(&block{children: cell}, fn)
			}
		}
	}
}

// headingText returns the text of a heading as it shows.
func headingText(b *block) string {
	return strings.TrimSpace(string(plainText(nil, b.inlines)))
}

// assignSlugs gives each heading of doc its slug, in document order, and
// returns the headings by slug.
func assignSlugs(doc *block) map[string]*block {
	taken := map[string]*block{}
	// counts holds how many times each base slug came, for the suffixes.
	counts := map[string]int{}
	walkHeadings(doc, func(b *block) {
		base := slugify(headingText(b))
		s := base
		for {
			if _, dup := taken[s]; !dup {
				break
			}
			counts[base]++
			s = base + "-" + strconv.Itoa(counts[base])
		}
		b.slug = s
		taken[s] = b
	})
	return taken
}

// slugify turns the text of a heading into its slug, before duplicates get
// their suffix, as github-slugger does: lowercase, " " to "-", and only
// letters, digits, marks, "-" and "_" kept.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// anchorOf returns the heading that the link destination dest goes to, or
// nil: dest is "#" and the slug of a heading, or failing that the slug of a
// heading in other case, as GitHub also accepts.
func anchorOf(slugs map[string]*block, dest string) *block {
	f, ok := linkFragment(dest)
	if !ok || f == "" {
		return nil
	}
	if b := slugs[f]; b != nil {
		return b
	}
	return slugs[strings.ToLower(f)]
}

// linkFragment returns the anchor a link destination names within the document:
// the part after a leading "#", with escapes such as %E8 decoded. ok is
// false for a destination that does not start with "#".
func linkFragment(dest string) (string, bool) {
	f, ok := strings.CutPrefix(dest, "#")
	if !ok {
		return "", false
	}
	if strings.IndexByte(f, '%') >= 0 {
		if d, err := url.PathUnescape(f); err == nil {
			f = d
		}
	}
	return f, true
}
