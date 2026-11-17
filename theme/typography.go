package theme

import "github.com/bankai-tensei/gowaku/text"

// MonoFamilies is the Family of TextTheme.Code: the monospaced fonts that come
// with Windows (Cascadia Mono, Consolas), macOS (SF Mono, Menlo) and Linux
// (DejaVu Sans Mono, Noto Sans Mono, Liberation Mono), Go Mono, which the
// pictures of the catalog and many tests set text in, then any monospaced
// family. A system takes the first it has, so one list serves every OS;
// Japanese in code falls through to the Japanese families of text.Style.
const MonoFamilies = "Cascadia Mono, Consolas, SF Mono, Menlo, DejaVu Sans Mono, Noto Sans Mono, Liberation Mono, Go Mono, monospace"

// TextTheme is the type scale. Japanese sets the sizes and leading, so there is
// no separate rule for CJK fonts. The weights lean heavy, because Yu Gothic is
// thin at 400. Each role sets Size, Weight and LineHeight and leaves Color zero;
// only Code sets a Family.
// basic.Text and basic.RichText draw a zero color in the theme's Ink; set
// another with text.Style.WithColor.
type TextTheme struct {
	// LargeTitle heads a page, and Title a dialog or section.
	LargeTitle, Title text.Style
	// Headline stands out in a list. Body is running text in the interface.
	Headline, Body text.Style
	// Control labels buttons and the like.
	Control text.Style
	// Caption is supporting text, and Section heads a group.
	Caption, Section text.Style
	// Reading is for text meant to be read at length.
	Reading text.Style
	// Code is monospaced, for source code, logs and values to copy: the
	// default of editor.Editor and of the code in markdown. Its Family is
	// MonoFamilies.
	Code text.Style
}

// NewTextTheme returns the type scale of docs/DESIGN.md §3.
func NewTextTheme() TextTheme {
	// The line is given in pixels, and even, so that whatever stacks below text
	// stays on the grid of §4.
	role := func(size float32, weight uint16, line float32) text.Style {
		return text.Style{Size: size, Weight: weight, LineHeight: line / size}
	}
	return TextTheme{
		LargeTitle: role(24, 600, 32),
		Title:      role(16, 600, 24),
		Headline:   role(13, 600, 20),
		Body:       role(13, 400, 20),
		Control:    role(13, 500, 20),
		Caption:    role(12, 400, 18),
		Section:    role(12, 500, 18),
		Reading:    role(15, 400, 26),
		Code:       text.Style{Family: MonoFamilies, Size: 13, Weight: 400, LineHeight: 20.0 / 13},
	}
}
