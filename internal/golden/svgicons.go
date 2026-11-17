package golden

import (
	"embed"
	"io/fs"
	"path"
	"slices"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

//go:embed testdata/svg/*/*.svg
var svgFiles embed.FS

// svgIcons reads the icons of testdata/svg, in the order of their paths.
func svgIcons(dir string) []*paint.SVG {
	names, err := fs.Glob(svgFiles, path.Join("testdata/svg", dir, "*.svg"))
	if err != nil {
		panic(err)
	}
	slices.Sort(names)
	icons := make([]*paint.SVG, 0, len(names))
	for _, name := range names {
		data, err := svgFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		s, err := paint.ParseSVG(data)
		if err != nil {
			panic(name + ": " + err.Error())
		}
		icons = append(icons, s)
	}
	return icons
}

// svgScene draws icons of Lucide (strokes) and Material Symbols (fills) and
// hand-written cases in a grid at 24 and 36 pixels, which go through the path
// atlas, and a few at 96 pixels, which go through stencil and cover.
func svgScene() core.Widget {
	lucide, material, hand := svgIcons("lucide"), svgIcons("material"), svgIcons("hand")
	accent := geom.Hex(0x2f5fd0)
	row := func(icons []*paint.SVG, size float32, color geom.Color) core.Widget {
		cells := make([]core.Widget, len(icons))
		for i, s := range icons {
			cells[i] = basic.SVG{Data: s, Size: size, Color: color}
		}
		return basic.Row{Spacing: 8, Children: cells}
	}
	return basic.Padding{Insets: geom.All(12), Child: basic.Column{Spacing: 10, Children: []core.Widget{
		row(lucide[:10], 24, geom.Color{}),
		row(lucide[10:], 24, accent),
		row(material[:7], 36, geom.Color{}),
		row(material[7:], 36, accent),
		row(hand, 36, geom.Color{}),
		basic.Row{Spacing: 12, Children: []core.Widget{
			basic.SVG{Data: lucide[6], Size: 96},
			basic.SVG{Data: material[6], Size: 96, Color: accent},
			basic.SVG{Data: hand[1], Size: 96},
			basic.SVG{Data: hand[5], Size: 96},
		}},
	}}}
}
