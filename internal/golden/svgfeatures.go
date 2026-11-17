package golden

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// svgFeaturesScale is the scale of the svg-features scene.
const svgFeaturesScale = 1.5

// svgFeatures draws the hand-written SVGs of testdata/svg/features, one per
// feature of ADR 0112 and 0113: structure, gradients, dashes and markers drawn
// shape by shape, and clip paths, masks, group opacity, filters and text,
// which draw as groups of the canvas that the GPU composites (ADR 0114). Text
// is set in the Go font of the Renderer.
func svgFeatures() core.Widget {
	drawings := svgIcons("features")
	const cols = 5
	var rows []core.Widget
	for i := 0; i < len(drawings); i += cols {
		var cells []core.Widget
		for _, s := range drawings[i:min(i+cols, len(drawings))] {
			cells = append(cells, basic.SVG{Data: s, Size: 150})
		}
		rows = append(rows, basic.Row{Spacing: 10, Children: cells})
	}
	return basic.Padding{Insets: geom.All(10), Child: basic.Column{Spacing: 10, Children: rows}}
}
