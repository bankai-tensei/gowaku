package field

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

// FuzzySpans appends label to dst as the spans of a basic.RichText, with the
// ranges that Fuzzy.Ranges returned for it set as hit and the rest in style,
// and returns the extended slice. ranges are byte offsets into label, start
// and end in pairs, on rune boundaries, as Fuzzy.Ranges gives them; to mark
// what strings.Index or a regexp found, pass its byte offsets the same way.
// A matched span is hit with the matched bytes as its Text, so that it takes
// the Style, Background and Emphasis of hit. The spans share the bytes of
// label, so that with room in dst it allocates nothing. Ranges that do not fit
// in label, as from a match of another label, are left out. To highlight as CommandPalette does, take
// hit from FuzzyHighlight; to mark the matches of a search as a highlighter
// does, give hit a Background:
//
//	spans = field.FuzzySpans(spans[:0], name, ranges, body, field.FuzzyHighlight(body, th.Color.AccentInk))
//	spans = field.FuzzySpans(spans[:0], line, found, body, text.Span{Style: body, Background: marker})
//	basic.RichText{Spans: spans, Style: body}
func FuzzySpans(dst []text.Span, label string, ranges []int, style text.Style, hit text.Span) []text.Span {
	at := 0
	for k := 0; k+1 < len(ranges); k += 2 {
		a, b := ranges[k], ranges[k+1]
		if a < at || b <= a || b > len(label) {
			continue
		}
		if a > at {
			dst = append(dst, text.Span{Text: label[at:a], Style: style})
		}
		hit.Text = label[a:b]
		dst = append(dst, hit)
		at = b
	}
	if at < len(label) || label == "" {
		dst = append(dst, text.Span{Text: label[at:], Style: style})
	}
	return dst
}

// FuzzyHighlight returns the span of FuzzySpans that shows the runes that match
// as CommandPalette does: style in the color mark, usually the AccentInk of
// the theme, and in weight 600, or 700 when style is already as heavy.
func FuzzyHighlight(style text.Style, mark geom.Color) text.Span {
	hit := style
	hit.Color = mark
	hit.Weight = 600
	if style.Weight >= 600 {
		hit.Weight = 700
	}
	return text.Span{Style: hit}
}
