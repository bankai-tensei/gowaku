package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// parseCloseBracket handles ]: it closes a link or an image if the brackets
// match and a destination follows, and is text otherwise.
func (p *inlineParser) parseCloseBracket() {
	start := p.pos
	p.pos++
	opener := p.brackets
	if opener == nil {
		p.text("]")
		return
	}
	if !opener.active {
		p.removeBracket()
		p.text("]")
		return
	}
	dest, title, ok := p.linkTail(opener, start)
	if !ok {
		p.removeBracket()
		p.pos = start + 1
		p.text("]")
		return
	}
	kind := inlineLink
	if opener.image {
		kind = inlineImage
	}
	node := &inode{kind: kind, dest: dest, title: title}
	for tmp := opener.node.next; tmp != nil; {
		next := tmp.next
		node.appendChild(tmp)
		tmp = next
	}
	p.root.appendChild(node)
	p.processEmphasis(opener.prevDelimiter)
	p.removeBracket()
	opener.node.unlink()
	if !opener.image {
		// Links do not nest: the brackets before this link cannot make one
		// around it.
		for b := p.brackets; b != nil; b = b.prev {
			if !b.image {
				b.active = false
			}
		}
	}
}

// linkTail reads what follows the ] at start: an inline destination, or a
// reference to a definition.
func (p *inlineParser) linkTail(opener *bracket, start int) (dest, title string, ok bool) {
	save := p.pos
	if p.peek() == '(' {
		p.pos++
		p.spnl()
		if d, ok := p.parseLinkDestination(); ok {
			p.spnl()
			t, hasTitle := "", false
			if isWhitespaceByte(peek(p.subject, p.pos-1)) {
				t, hasTitle = p.parseLinkTitle()
			}
			if hasTitle {
				p.spnl()
			}
			if p.peek() == ')' {
				p.pos++
				return d, t, true
			}
		}
		p.pos = save
	}
	var label string
	before := p.pos
	if n := p.parseLinkLabel(); n > 2 {
		label = p.subject[before : before+n]
	} else if !opener.bracketAfter {
		// A shortcut or collapsed reference: the link text is the label.
		label = p.subject[opener.index:start] + "]"
		if n == 0 {
			p.pos = save
		}
	}
	if label == "" {
		return "", "", false
	}
	ref, ok := p.refs[normalizeReference(label)]
	if !ok {
		p.pos = save
		return "", "", false
	}
	return ref.dest, ref.title, true
}

// spnl skips spaces and at most one line break.
func (p *inlineParser) spnl() {
	for p.peek() == ' ' || p.peek() == '\t' {
		p.pos++
	}
	if p.peek() == '\n' {
		p.pos++
	}
	for p.peek() == ' ' || p.peek() == '\t' {
		p.pos++
	}
}

// parseLinkLabel reads a link label such as [foo] at pos and returns its
// length, brackets included, or 0. It does not move pos past a label that
// is not there.
func (p *inlineParser) parseLinkLabel() int {
	s := p.subject
	if peek(s, p.pos) != '[' {
		return 0
	}
	chars := 0
	for i := p.pos + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				i++
			}
		case '[':
			return 0
		case ']':
			if chars > 999 {
				return 0
			}
			n := i + 1 - p.pos
			p.pos = i + 1
			return n
		}
		chars++
		if chars > 999 {
			return 0
		}
	}
	return 0
}

// parseLinkDestination reads a destination, <in brackets> or bare with
// balanced parentheses.
func (p *inlineParser) parseLinkDestination() (string, bool) {
	s := p.subject
	if p.peek() == '<' {
		for i := p.pos + 1; i < len(s); i++ {
			switch s[i] {
			case '\\':
				if i+1 < len(s) && isASCIIPunct(s[i+1]) {
					i++
				}
			case '\n', '<':
				return "", false
			case '>':
				dest := unescapeString(s[p.pos+1 : i])
				p.pos = i + 1
				return dest, true
			}
		}
		return "", false
	}
	start := p.pos
	parens := 0
	i := start
loop:
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]):
			i += 2
		case c == '(':
			parens++
			if parens > maxLinkParens {
				return "", false
			}
			i++
		case c == ')':
			if parens == 0 {
				break loop
			}
			parens--
			i++
		case c <= ' ' || c == 0x7f:
			break loop
		default:
			i++
		}
	}
	if (i == start && peek(s, i) != ')') || parens != 0 {
		return "", false
	}
	p.pos = i
	return unescapeString(s[start:i]), true
}

// parseLinkTitle reads a title in quotes or parentheses.
func (p *inlineParser) parseLinkTitle() (string, bool) {
	s := p.subject
	open := p.peek()
	closing := open
	switch open {
	case '"', '\'':
	case '(':
		closing = ')'
	default:
		return "", false
	}
	for i := p.pos + 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s):
			i++
		case c == closing:
			title := unescapeString(s[p.pos+1 : i])
			p.pos = i + 1
			return title, true
		case open == '(' && c == '(':
			return "", false
		}
	}
	return "", false
}

// parseAutolink reads <scheme:...> or <user@host> at pos.
func (p *inlineParser) parseAutolink() bool {
	s := p.subject[p.pos:]
	// Neither kind has spaces or <, so the search stops at them, and a run of
	// < costs no more than its length.
	end := 1
	for end < len(s) && s[end] != '>' {
		if s[end] <= ' ' || s[end] == '<' {
			return false
		}
		end++
	}
	if end >= len(s) {
		return false
	}
	body := s[1:end]
	switch {
	case isURIAutolink(body):
		p.addAutolink(body, body)
	case isEmailAutolink(body):
		p.addAutolink(body, "mailto:"+body)
	default:
		return false
	}
	p.pos += end + 1
	return true
}

func (p *inlineParser) addAutolink(label, dest string) {
	link := &inode{kind: inlineLink, dest: dest, autolink: true}
	link.appendChild(&inode{kind: inlineText, text: label})
	p.root.appendChild(link)
}

// isURIAutolink reports whether s is a scheme of 2 to 32 characters, a colon
// and no spaces or angle brackets.
func isURIAutolink(s string) bool {
	colon := strings.IndexByte(s, ':')
	if colon < 2 || colon > 32 || !isAlpha(s[0]) {
		return false
	}
	for i := 1; i < colon; i++ {
		c := s[i]
		if !isAlnum(c) && c != '+' && c != '.' && c != '-' {
			return false
		}
	}
	for i := colon + 1; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c == '<' || c == '>' {
			return false
		}
	}
	return true
}

// isEmailAutolink reports whether s is an e-mail address as the
// specification describes it.
func isEmailAutolink(s string) bool {
	at := strings.IndexByte(s, '@')
	if at < 1 || at == len(s)-1 {
		return false
	}
	for i := 0; i < at; i++ {
		c := s[i]
		if !isAlnum(c) && !strings.ContainsRune(".!#$%&'*+/=?^_`{|}~-", rune(c)) {
			return false
		}
	}
	for _, label := range strings.Split(s[at+1:], ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			if !isAlnum(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	return true
}

// autolinkBoundary reports whether an extended autolink may start at i: at
// the start, or after a space or one of *_~(.
func (p *inlineParser) autolinkBoundary(i int) bool {
	if i == 0 {
		return true
	}
	switch c := p.subject[i-1]; c {
	case ' ', '\t', '\n', '*', '_', '~', '(':
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(p.subject[:i])
	return unicode.IsSpace(r)
}

// parseExtendedAutolink reads a bare www. or http(s):// link, the GFM
// extension, at pos.
func (p *inlineParser) parseExtendedAutolink() bool {
	c := p.subject[p.pos]
	if (c != 'h' && c != 'w') || !p.autolinkBoundary(p.pos) {
		return false
	}
	n := matchExtendedAutolink(p.subject[p.pos:])
	if n == 0 {
		return false
	}
	label := p.subject[p.pos : p.pos+n]
	dest := label
	if strings.HasPrefix(label, "www.") {
		dest = "http://" + label
	}
	p.addAutolink(label, dest)
	p.pos += n
	return true
}

// matchExtendedAutolink returns the length of the www. or http(s):// link at
// the start of s, or 0. The link stops at a space, at < and at the first
// character beyond ASCII, so that a URL followed by Japanese without a space
// ends where the Japanese starts. Trailing punctuation is not part of it.
func matchExtendedAutolink(s string) int {
	var start int
	switch {
	case strings.HasPrefix(s, "www."):
		start = 4
	case strings.HasPrefix(s, "http://"):
		start = 7
	case strings.HasPrefix(s, "https://"):
		start = 8
	default:
		return 0
	}
	// The domain: segments of letters, digits, - and _ between periods.
	end := start
	for end < len(s) && (isAlnum(s[end]) || s[end] == '-' || s[end] == '_' || s[end] == '.') {
		end++
	}
	domain := strings.TrimRight(s[start:end], ".")
	if start == 4 {
		domain = "www." + domain
	}
	if !validDomain(domain) {
		return 0
	}
	for end < len(s) {
		c := s[end]
		// GitHub finds these links in text after the links and code spans,
		// so brackets and backticks end them.
		if c <= ' ' || c == '<' || c == '[' || c == ']' || c == '`' || c >= utf8.RuneSelf {
			break
		}
		end++
	}
	return trimAutolinkTail(s[:end])
}

// validDomain asks for at least one period and no underscore in the last two
// segments.
func validDomain(d string) bool {
	segs := strings.Split(d, ".")
	if len(segs) < 2 {
		return false
	}
	for i, seg := range segs {
		if seg == "" {
			return false
		}
		if i >= len(segs)-2 && strings.IndexByte(seg, '_') >= 0 {
			return false
		}
	}
	return true
}

// trimAutolinkTail drops the punctuation that ends a sentence rather than the
// link, and returns the length that remains.
func trimAutolinkTail(s string) int {
	for s != "" {
		c := s[len(s)-1]
		switch {
		case strings.IndexByte("?!.,:*_~'\"", c) >= 0:
			s = s[:len(s)-1]
		case c == ')' && strings.Count(s, ")") > strings.Count(s, "("):
			s = s[:len(s)-1]
		case c == ';':
			i := strings.LastIndexByte(s, '&')
			if i < 0 || !allAlnum(s[i+1:len(s)-1]) || i+1 == len(s)-1 {
				return len(s)
			}
			s = s[:i]
		default:
			return len(s)
		}
	}
	return 0
}

func allAlnum(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isAlnum(s[i]) {
			return false
		}
	}
	return true
}

// parseReference reads a link reference definition at the start of s,
// records it in refs unless the label is taken, and returns its length, or 0.
func parseReference(s string, refs map[string]linkRef) int {
	p := &inlineParser{subject: s}
	n := p.parseLinkLabel()
	if n == 0 {
		return 0
	}
	label := s[:n]
	if p.peek() != ':' {
		return 0
	}
	p.pos++
	p.spnl()
	dest, ok := p.parseLinkDestination()
	if !ok {
		return 0
	}
	beforeTitle := p.pos
	p.spnl()
	title, hasTitle := "", false
	if p.pos != beforeTitle {
		title, hasTitle = p.parseLinkTitle()
	}
	if !hasTitle {
		p.pos = beforeTitle
	}
	if !p.atLineEnd() {
		if !hasTitle {
			return 0
		}
		// The title must end its line; without it, the definition may.
		title, p.pos = "", beforeTitle
		if !p.atLineEnd() {
			return 0
		}
	}
	key := normalizeReference(label)
	if key == "" {
		return 0
	}
	if _, taken := refs[key]; !taken {
		refs[key] = linkRef{dest: dest, title: title}
	}
	return p.pos
}

// atLineEnd skips spaces and a line break, and reports whether nothing else
// stood before the end of the line.
func (p *inlineParser) atLineEnd() bool {
	i := p.pos
	for i < len(p.subject) && (p.subject[i] == ' ' || p.subject[i] == '\t') {
		i++
	}
	switch {
	case i == len(p.subject):
	case p.subject[i] == '\n':
		i++
	default:
		return false
	}
	p.pos = i
	return true
}

// normalizeReference turns a label into the key under which it is found:
// without brackets, case folded, with runs of space as one space.
func normalizeReference(label string) string {
	label = strings.TrimSuffix(strings.TrimPrefix(label, "["), "]")
	label = strings.Join(strings.Fields(label), " ")
	return strings.ToLower(strings.ToUpper(label))
}
