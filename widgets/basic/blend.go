package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/paint"
)

// Blend draws Child as one picture and combines it with what is below by Mode,
// as mix-blend-mode does in CSS:
//
//	Stack{Children: []core.Widget{
//		photo,
//		Blend{Mode: paint.BlendMultiply, Child: tint},
//	}}
//
// Child is recorded into a layer of its own, which also makes Blend a
// RepaintBoundary, and draws as a group of the canvas (paint.Canvas.PushGroup):
// one picture wherever it is, under a scale transition and inside other layers
// and groups too. An Opacity above fades the picture as a whole before it
// blends. The zero Mode, paint.BlendSrcOver, draws Child as usual. The shape that
// the bounded modes such as BlendSrcIn act on is what Child draws, pixel by
// pixel. See docs/ADR/0044-drawing-extensions.md and
// docs/ADR/0114-gpu-layers-masks-filters.md.
type Blend struct {
	Mode  paint.BlendMode
	Child core.Widget
}

// Key implements core.Widget.
func (Blend) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b Blend) CreateRenderObject() core.RenderObject {
	r := core.NewRenderRepaintBoundary()
	r.SetBlend(b.Mode)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (b Blend) UpdateRenderObject(ro core.RenderObject) {
	ro.(*core.RenderRepaintBoundary).SetBlend(b.Mode)
}

// ChildWidget implements core.SingleChildWidget.
func (b Blend) ChildWidget() core.Widget { return b.Child }
