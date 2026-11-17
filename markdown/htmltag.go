package markdown

import (
	"strings"
)

// maxAttrs bounds the attributes kept of one tag; the rest are read and
// dropped.
const maxAttrs = 32

// htmlAttr is an attribute of a tag: its name in lower case and its value
// with character references decoded.
type htmlAttr struct {
	name, value string
}

// htmlTag is an opening or closing HTML tag of the source. See
// docs/ADR/0119-markdown-html-subset.md.
type htmlTag struct {
	// name is the name of the element in lower case.
	name        string
	closing     bool
	selfClosing bool
	attrs       []htmlAttr
	// raw is the tag as written, shown when the tag is not interpreted.
	raw string
}

// attr returns the value of the attribute name, and whether the tag has it.
func (t *htmlTag) attr(name string) (string, bool) {
	for _, a := range t.attrs {
		if a.name == name {
			return a.value, true
		}
	}
	return "", false
}

// scanTag reads the opening or closing tag at the start of s, which starts
// with '<', and returns it with its length, or a length of 0 if s does not
// start with one. strict follows the grammar of raw HTML in CommonMark;
// otherwise names and unquoted values take what browsers take. Neither lets
// a name or an unquoted value hold '<', so a scan ends at the next '<' outside
// quotes, and scanning every '<' of a text stays linear.
func scanTag(s string, strict bool) (htmlTag, int) {
	var t htmlTag
	if len(s) < 3 || s[0] != '<' {
		return t, 0
	}
	i := 1
	if s[i] == '/' {
		t.closing = true
		i++
	}
	start := i
	if i >= len(s) || !isAlpha(s[i]) {
		return t, 0
	}
	for i < len(s) && (isAlnum(s[i]) || s[i] == '-') {
		i++
	}
	t.name = strings.ToLower(s[start:i])
	if t.closing {
		i = skipHTMLSpace(s, i)
		if i < len(s) && s[i] == '>' {
			t.raw = s[:i+1]
			return t, i + 1
		}
		return htmlTag{}, 0
	}
	for {
		ws := skipHTMLSpace(s, i)
		switch {
		case ws >= len(s):
			return htmlTag{}, 0
		case s[ws] == '>':
			t.raw = s[:ws+1]
			return t, ws + 1
		case s[ws] == '/' && ws+1 < len(s) && s[ws+1] == '>':
			t.selfClosing = true
			t.raw = s[:ws+2]
			return t, ws + 2
		case ws == i:
			// An attribute must follow white space.
			return htmlTag{}, 0
		}
		i = ws
		nameEnd := scanAttrName(s, i, strict)
		if nameEnd == i {
			return htmlTag{}, 0
		}
		a := htmlAttr{name: strings.ToLower(s[i:nameEnd])}
		i = nameEnd
		if j := skipHTMLSpace(s, i); j < len(s) && s[j] == '=' {
			v, end, ok := scanAttrValue(s, skipHTMLSpace(s, j+1), strict)
			if !ok {
				return htmlTag{}, 0
			}
			a.value, i = decodeText(v), end
		}
		if len(t.attrs) < maxAttrs {
			t.attrs = append(t.attrs, a)
		}
	}
}

// scanAttrName returns the end of the attribute name at s[i:], or i if there
// is none.
func scanAttrName(s string, i int, strict bool) int {
	if strict {
		if i >= len(s) || (!isAlpha(s[i]) && s[i] != '_' && s[i] != ':') {
			return i
		}
		j := i + 1
		for j < len(s) && (isAlnum(s[j]) || strings.IndexByte("_.:-", s[j]) >= 0) {
			j++
		}
		return j
	}
	j := i
	for j < len(s) && !isHTMLSpace(s[j]) && strings.IndexByte("/>=\"'<", s[j]) < 0 {
		j++
	}
	return j
}

// scanAttrValue reads the attribute value at s[i:], quoted or not, and returns
// it without quotes and where it ends.
func scanAttrValue(s string, i int, strict bool) (v string, end int, ok bool) {
	if i >= len(s) {
		return "", i, false
	}
	if q := s[i]; q == '"' || q == '\'' {
		k := strings.IndexByte(s[i+1:], q)
		if k < 0 {
			return "", i, false
		}
		return s[i+1 : i+1+k], i + 2 + k, true
	}
	j := i
	stop := " \t\n\r\f\"'=<>`"
	if !strict {
		stop = " \t\n\r\f<>"
	}
	for j < len(s) && strings.IndexByte(stop, s[j]) < 0 {
		j++
	}
	if j == i {
		return "", i, false
	}
	return s[i:j], j, true
}

// The kinds of markup that markupEnd reads.
const (
	markupComment = iota
	markupCDATA
	markupInstruction
	markupDeclaration
	markupKinds
)

// markupKind returns the kind of markup at the start of s, or -1.
func markupKind(s string) int {
	switch {
	case strings.HasPrefix(s, "<!--"):
		return markupComment
	case strings.HasPrefix(s, "<![CDATA["):
		return markupCDATA
	case strings.HasPrefix(s, "<?"):
		return markupInstruction
	case len(s) > 2 && s[1] == '!' && isAlpha(s[2]):
		return markupDeclaration
	}
	return -1
}

// markupEnd returns the length of the comment, processing instruction,
// declaration or CDATA section at the start of s, or 0 if s starts with none
// of them. With open set, one that does not end takes the rest of s, as a
// browser reads it; otherwise it is no markup at all, as in CommonMark.
func markupEnd(s string, open bool) int {
	var from int
	var end string
	switch markupKind(s) {
	case markupComment:
		if strings.HasPrefix(s, "<!-->") {
			return len("<!-->")
		}
		if strings.HasPrefix(s, "<!--->") {
			return len("<!--->")
		}
		from, end = len("<!--"), "-->"
	case markupCDATA:
		from, end = len("<![CDATA["), "]]>"
	case markupInstruction:
		from, end = len("<?"), "?>"
	case markupDeclaration:
		from, end = 2, ">"
	default:
		return 0
	}
	k := strings.Index(s[from:], end)
	if k < 0 {
		if open {
			return len(s)
		}
		return 0
	}
	return from + k + len(end)
}

// decodeText decodes the character references of s, such as &amp; and
// &#12354;. A reference the package does not know stays as written.
func decodeText(s string) string {
	if strings.IndexByte(s, '&') < 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '&' {
			if v, n := decodeEntity(s[i:]); n > 0 {
				b.WriteString(v)
				i += n
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func skipHTMLSpace(s string, i int) int {
	for i < len(s) && isHTMLSpace(s[i]) {
		i++
	}
	return i
}

func isHTMLSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
