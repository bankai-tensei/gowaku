package theme

import (
	"strings"

	"github.com/bankai-tensei/gowaku/geom"
)

// ParseHex reads a color written in hex, as CSS writes it: #RGB, #RGBA,
// #RRGGBB or #RRGGBBAA, in either case. It forgives what a paste brings along:
// space around the code, quotes, a trailing semicolon or comma, a missing #,
// and a 0x in its place. It reports false for anything else.
func ParseHex(s string) (geom.Color, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";,")
	s = strings.Trim(s, "\"'` \t")
	switch {
	case strings.HasPrefix(s, "#"):
		s = s[1:]
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		s = s[2:]
	}
	var n [8]uint8
	if len(s) != 3 && len(s) != 4 && len(s) != 6 && len(s) != 8 {
		return geom.Color{}, false
	}
	for i := range len(s) {
		d, ok := hexDigit(s[i])
		if !ok {
			return geom.Color{}, false
		}
		n[i] = d
	}
	var ch [4]uint8
	ch[3] = 0xff
	switch len(s) {
	case 3, 4:
		for i := range len(s) {
			ch[i] = n[i]<<4 | n[i]
		}
	default:
		for i := range len(s) / 2 {
			ch[i] = n[2*i]<<4 | n[2*i+1]
		}
	}
	return geom.Color{R: float32(ch[0]) / 255, G: float32(ch[1]) / 255, B: float32(ch[2]) / 255, A: float32(ch[3]) / 255}, true
}

func hexDigit(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// FormatHex spells c as #RRGGBB in capitals, with the alpha after it as
// #RRGGBBAA when c is not opaque. Channels are rounded to 8 bits.
func FormatHex(c geom.Color) string {
	const digits = "0123456789ABCDEF"
	var b [9]byte
	b[0] = '#'
	n := 1
	put := func(v float32) {
		x := int(geom.Clamp32(v, 0, 1)*255 + 0.5)
		b[n], b[n+1] = digits[x>>4], digits[x&15]
		n += 2
	}
	put(c.R)
	put(c.G)
	put(c.B)
	if int(geom.Clamp32(c.A, 0, 1)*255+0.5) < 255 {
		put(c.A)
	}
	return string(b[:n])
}
