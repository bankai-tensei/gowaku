package theme_test

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
)

// BrandColors are tokens of an application that the standard ones lack.
type BrandColors struct {
	Highlight geom.Color
}

// An application keeps its own tokens in the theme, with a value for each
// brightness, and reads them in Build with Extension.
func ExampleExtension() {
	set := theme.WithExtensionPair(theme.FromAccent(geom.Hex(0x2f5fd0)),
		BrandColors{Highlight: geom.Hex(0xffd60a)},
		BrandColors{Highlight: geom.Hex(0x806b05)},
	)

	// In Build: th := widgets.ThemeOf(ctx)
	th := set.For(theme.BrightnessDark)
	brand, ok := theme.Extension[BrandColors](th)
	fmt.Println(ok, brand.Highlight == geom.Hex(0x806b05))

	// A theme without the tokens falls back.
	fallback := theme.ExtensionOr(theme.Default().Light, BrandColors{Highlight: geom.Hex(0xffffff)})
	fmt.Println(fallback.Highlight == geom.Hex(0xffffff))
	// Output:
	// true true
	// true
}
