package markdown

import (
	"strings"
	"unicode/utf8"
)

// namedEntity returns the character of a named character reference. HTML
// has over two thousand; these are the ones that show up in prose. Any other
// stays as it was written. See docs/ADR/0099-markdown-module.md.
func namedEntity(name string) (string, bool) {
	switch name {
	case "amp":
		return "&", true
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "quot":
		return "\"", true
	case "apos":
		return "'", true
	case "nbsp":
		return " ", true
	case "copy":
		return "©", true
	case "reg":
		return "®", true
	case "trade":
		return "™", true
	case "hellip":
		return "…", true
	case "mdash":
		return "—", true
	case "ndash":
		return "–", true
	case "lsquo":
		return "‘", true
	case "rsquo":
		return "’", true
	case "ldquo":
		return "“", true
	case "rdquo":
		return "”", true
	case "laquo":
		return "«", true
	case "raquo":
		return "»", true
	case "middot":
		return "·", true
	case "bull":
		return "•", true
	case "times":
		return "×", true
	case "divide":
		return "÷", true
	case "deg":
		return "°", true
	case "plusmn":
		return "±", true
	case "sect":
		return "§", true
	case "larr":
		return "←", true
	case "rarr":
		return "→", true
	case "uarr":
		return "↑", true
	case "darr":
		return "↓", true
	case "harr":
		return "↔", true
	case "euro":
		return "€", true
	case "yen":
		return "¥", true
	case "pound":
		return "£", true
	case "ensp":
		return " ", true
	case "emsp":
		return " ", true
	case "thinsp":
		return " ", true
	case "le":
		return "≤", true
	case "ge":
		return "≥", true
	case "ne":
		return "≠", true
	case "minus":
		return "−", true
	}
	return "", false
}

// decodeEntity reads a character reference at the start of s, such as &amp;
// or &#x3042;, and returns what it stands for and its length. n is 0 when s
// does not start with one.
func decodeEntity(s string) (decoded string, n int) {
	if len(s) < 3 || s[0] != '&' {
		return "", 0
	}
	if s[1] == '#' {
		return decodeNumeric(s)
	}
	end := 1
	for end < len(s) && end <= 32 && isAlnum(s[end]) {
		end++
	}
	if end == 1 || end >= len(s) || s[end] != ';' || !isAlpha(s[1]) {
		return "", 0
	}
	v, ok := namedEntity(s[1:end])
	if !ok {
		return "", 0
	}
	return v, end + 1
}

func decodeNumeric(s string) (string, int) {
	i, base, limit := 2, 10, 7
	if i < len(s) && (s[i] == 'x' || s[i] == 'X') {
		i, base, limit = 3, 16, 6
	}
	start := i
	r := 0
	for i < len(s) && i-start < limit {
		d := digitValue(s[i], base)
		if d < 0 {
			break
		}
		r = r*base + d
		i++
	}
	if i == start || i >= len(s) || s[i] != ';' {
		return "", 0
	}
	if r == 0 || r > utf8.MaxRune || (r >= 0xD800 && r <= 0xDFFF) {
		r = utf8.RuneError
	}
	return string(rune(r)), i + 1
}

func digitValue(c byte, base int) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case base == 16 && c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case base == 16 && c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// unescapeString resolves the backslash escapes and the character references
// of a link destination, a title or an info string.
func unescapeString(s string) string {
	if strings.IndexByte(s, '\\') < 0 && strings.IndexByte(s, '&') < 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]):
			b.WriteByte(s[i+1])
			i++
		case c == '&':
			if v, n := decodeEntity(s[i:]); n > 0 {
				b.WriteString(v)
				i += n - 1
				continue
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isASCIIPunct(c byte) bool {
	return (c >= '!' && c <= '/') || (c >= ':' && c <= '@') || (c >= '[' && c <= '`') || (c >= '{' && c <= '~')
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func isAlnum(c byte) bool { return isAlpha(c) || isDigit(c) }
