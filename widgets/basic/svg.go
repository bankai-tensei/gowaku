package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// SVG draws an SVG drawing, read once with paint.ParseSVG and kept, fitted into
// a square. Like Icon it is decorative unless Label is set. A nil Data leaves
// the square empty. The text of the drawing is set with the text.Shaper above,
// through paint.SVG.SetText, if there is one.
type SVG struct {
	core.StatelessBase
	Data *paint.SVG
	// Size is the size of the square on screen. Zero selects 24.
	Size float32
	// Color is currentColor, which icon sets draw with. The zero value
	// selects the Ink of the theme, or opaque black without a theme.
	Color geom.Color
	// Label describes the drawing to assistive technology.
	Label string
}

// Build implements core.StatelessWidget.
func (s SVG) Build(ctx core.BuildContext) core.Widget {
	c := s.Color
	if c == (geom.Color{}) {
		c = geom.Color{A: 1}
		if th, ok := core.DependOn[*theme.Theme](ctx); ok {
			c = th.Color.Ink
		}
	}
	if s.Data != nil && s.Data.HasText() {
		if shaper, ok := core.DependOn[*text.Shaper](ctx); ok {
			// The document parsed once already, so it parses again; and
			// the same shaper again does nothing.
			err := s.Data.SetText(shaper)
			_ = err
		}
	}
	size := s.Size
	if size <= 0 {
		size = kit.DefaultIconSize
	}
	return svgBox{data: s.Data, size: size, color: c, label: s.Label}
}

// svgBox is SVG with its color found.
type svgBox struct {
	data  *paint.SVG
	size  float32
	color geom.Color
	label string
}

// Key implements core.Widget.
func (svgBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b svgBox) CreateRenderObject() core.RenderObject {
	return &layout.RenderSVG{Extent: b.size, Color: b.color, Label: b.label, Data: b.data}
}

// UpdateRenderObject implements core.RenderWidget.
func (b svgBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderSVG)
	if r.Extent != b.size {
		r.Extent = b.size
		r.MarkNeedsLayout()
	}
	if r.Data != b.data || r.Color != b.color {
		r.Data, r.Color = b.data, b.color
		r.MarkNeedsPaint()
	}
	r.Label = b.label
}
