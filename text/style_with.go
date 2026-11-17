package text

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// The With methods return a copy of a Style with one field replaced, so that a
// theme style can be adjusted inside a widget literal:
//
//	widgets.Text{Text: "3:42", Style: th.Text.Caption.WithColor(th.Color.Ink2)}
//
// They are single steps on a value, like geom.Color.WithAlpha, not a builder:
// they cover only the fields that apps change most, and several can follow each
// other inside a literal (th.Text.Caption.WithSize(11).WithWeight(600)). For any
// other field, copy the style and assign it:
//
//	st := th.Text.Body
//	st.Align, st.Italic = text.AlignCenter, true

// WithColor returns s with Color replaced by c. The zero Color is still opaque
// black.
func (s Style) WithColor(c geom.Color) Style {
	s.Color = c
	return s
}

// WithSize returns s with Size replaced by size in logical pixels. Zero still
// selects 14.
//
// A style with a LineHeight keeps its line in pixels, so that smaller text on a
// theme role stays on the grid of the role: Section, 12px on 18px lines, gives
// 11px on 18px lines, not 16.5. Only when the line would be tighter than 1.2
// times the new size does it grow, in proportion, rounded up to an even pixel.
// A style without a LineHeight keeps the default ratio of 1.5.
func (s Style) WithSize(size float32) Style {
	if s.LineHeight > 0 {
		was, now := orDefault(s.Size), orDefault(size)
		line := s.LineHeight * was
		if line < minLineRatio*now {
			line = 2 * float32(math.Ceil(float64(s.LineHeight*now/2)))
		}
		s.LineHeight = line / now
	}
	s.Size = size
	return s
}

// LinePixels returns the distance between baselines in logical pixels, which
// is the height of each line of a paragraph in s: 20 for the theme's Body.
func (s Style) LinePixels() float32 {
	r := s.resolved()
	return r.Size * r.LineHeight
}

// minLineRatio is the tightest line WithSize keeps, as a multiple of the size.
const minLineRatio = 1.2

// orDefault returns size, or the default size for zero.
func orDefault(size float32) float32 {
	if size <= 0 {
		return defaultSize
	}
	return size
}

// WithAlign returns s with Align replaced by a.
func (s Style) WithAlign(a Align) Style {
	s.Align = a
	return s
}

// WithWeight returns s with Weight replaced by the CSS font weight weight.
// Zero still selects 400.
func (s Style) WithWeight(weight uint16) Style {
	s.Weight = weight
	return s
}

// WithMaxLines returns s with MaxLines replaced by n: one line for the preview
// in a list row, two for the description on a card. Zero is unlimited.
func (s Style) WithMaxLines(n int) Style {
	s.MaxLines = n
	return s
}
