package text

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text/jlreq"
)

const (
	defaultSize       = 14
	defaultLineHeight = 1.5
	defaultWeight     = 400
)

// Align places lines within the length of a Layout: the longest line, or for
// AlignCenter and AlignEnd the whole maxLength given to Paragraph.Layout when
// it is finite.
type Align uint8

// The alignments.
const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
	// AlignJustify stretches every line of a block but the last to the full
	// width, by the rules of JLReq 3.8.
	AlignJustify
)

// Orientation is the direction in which the characters of a line follow each
// other.
type Orientation uint8

// The orientations.
const (
	// Horizontal sets lines from left to right, stacked downwards.
	Horizontal Orientation = iota
	// Vertical sets lines from top to bottom, stacked from right to left
	// (JLReq 1.2.2). Japanese stands upright, Western text turns 90° clockwise,
	// and punctuation takes its vertical form.
	Vertical
)

// String returns the name of o.
func (o Orientation) String() string {
	if o == Vertical {
		return "Vertical"
	}
	return "Horizontal"
}

// Style describes how to set a text. The zero value is regular black text of 14
// logical pixels in the default family, composed by the rules of JLReq.
type Style struct {
	// Family is tried before the default Japanese families. It may name
	// several, separated by commas and tried in order, as CSS font-family
	// does: "Cascadia Mono, Menlo, monospace". The families a system lacks
	// are skipped, so one list serves every OS. The generic names of fontscan
	// ("monospace", "serif", "sans-serif") stand for a family of that kind.
	Family string
	// Size is the font size in logical pixels. Zero selects 14.
	Size float32
	// Weight is the CSS font weight. Zero selects 400.
	Weight uint16
	// Italic selects the italic face of the font. A face without one, as
	// Japanese fonts are, is slanted by about 12° instead, except in vertical
	// lines, for color emoji and for right-to-left scripts. Advances, carets and
	// selections stay those of upright text. See
	// docs/ADR/0101-synthetic-italic.md.
	Italic bool
	// Variations set the axes of a variable font. Weight already sets "wght"
	// and Size "opsz" on fonts that have them.
	Variations [maxVariations]Variation
	// LineHeight is the distance between baselines as a multiple of Size. It
	// is fixed so that a fallback font cannot make lines uneven. Zero selects
	// 1.5.
	LineHeight float32
	// Color fills the glyphs. The zero value is opaque black, because
	// invisible text is never the intent. The text widgets of package
	// widgets/basic replace a zero color with the theme's Ink first.
	Color geom.Color
	Align Align
	// Direction is the base direction of every block of horizontal text. The
	// zero value follows the first strong character, so Arabic and Hebrew
	// paragraphs run right to left by themselves.
	Direction Direction
	// Orientation selects horizontal or vertical writing. In vertical writing
	// a "line" is a column, its width is the height of the column, and
	// LineHeight is the distance between the centers of columns.
	Orientation Orientation
	// MaxLines limits the text to that many lines, columns in vertical text.
	// Zero leaves it unlimited. Text cut short ends its last kept line with an
	// ellipsis "…", vertical in vertical text, at the logical end of the text:
	// on the right of a left-to-right line and on the left of a right-to-left
	// one. That line is filled up to the length, as one line of the rest of
	// its block would be, and whole grapheme clusters come off its end until
	// the ellipsis fits; spaces and opening brackets before the ellipsis go
	// too. With MaxLines 1, text that does not fit is cut at the width. The
	// text itself, and what a screen reader reads, stay whole. Spans ignore
	// it. See docs/ADR/0125-text-truncation-decorative-stacks.md.
	MaxLines int

	// Indent is the space before the first line of every block, in logical
	// pixels. A Japanese paragraph usually starts one em in.
	Indent float32
	// HangingIndent is the space before the other lines of a block.
	HangingIndent float32
	// TabStop is the distance between tab stops in logical pixels. Zero sets
	// tabs as spaces.
	TabStop float32

	// LineBreak sets how strict the line start prohibition is.
	LineBreak jlreq.Strictness
	// Kinsoku selects how a prohibited line start is resolved.
	Kinsoku jlreq.Kinsoku
	// Punctuation selects whether adjacent punctuation is squeezed.
	Punctuation jlreq.Punctuation
	// WesternSpacing selects the space between Japanese and Western text.
	WesternSpacing jlreq.WesternSpacing
	// Hanging lets commas and full stops stick out past the line end.
	Hanging bool
	// Hyphenate lets Western words break across lines with a hyphen, by the
	// patterns of American English.
	Hyphenate bool
	// RubyRoom gives the text the line height and the space above its first
	// line that ruby needs, even where no Span has ruby. Text whose ruby comes
	// and goes, as in an editor that shows the ruby of a notation only away
	// from the caret, then keeps its lines in place.
	RubyRoom bool
}

// resolved fills in the defaults. Color does not affect shaping and is left
// out, so that it does not take part in comparisons of layouts.
func (s Style) resolved() Style {
	if s.Size <= 0 {
		s.Size = defaultSize
	}
	if s.Weight == 0 {
		s.Weight = defaultWeight
	}
	if s.LineHeight <= 0 {
		s.LineHeight = defaultLineHeight
	}
	s.Color = geom.Color{}
	return s
}

// over returns s with its zero fields taken from base, which is resolved.
func (s Style) over(base *Style) Style {
	out := *base
	if s.Family != "" {
		out.Family = s.Family
	}
	if s.Size > 0 {
		out.Size = s.Size
	}
	if s.Weight != 0 {
		out.Weight = s.Weight
	}
	out.Italic = out.Italic || s.Italic
	if s.Variations != ([maxVariations]Variation{}) {
		out.Variations = s.Variations
	}
	return out
}

// PaintColor returns the color to fill the glyphs with.
func (s *Style) PaintColor() geom.Color {
	if s.Color == (geom.Color{}) {
		return geom.Color{A: 1}
	}
	return s.Color
}

func (s *Style) options() jlreq.Options {
	return jlreq.Options{
		Strictness:     s.LineBreak,
		Kinsoku:        s.Kinsoku,
		Punctuation:    s.Punctuation,
		WesternSpacing: s.WesternSpacing,
		Hanging:        s.Hanging,
		Justify:        s.Align == AlignJustify,
		Indent:         s.Indent,
		HangingIndent:  s.HangingIndent,
		TabStop:        s.TabStop,
	}
}

// SameLayout reports whether s and o produce the same glyph positions.
func (s Style) SameLayout(o Style) bool { return s.resolved() == o.resolved() }
