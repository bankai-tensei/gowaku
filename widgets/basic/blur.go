package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// BackdropFilter blurs what is painted below its box, then paints Child on top,
// as backdrop-filter: blur() does in CSS. Sigma is the standard deviation of the
// Gaussian in logical pixels; zero paints Child alone. Radius rounds the corners
// of the blurred box, so that a glass card can be a translucent Container of the
// same Radius:
//
//	BackdropFilter{Sigma: 20, Radius: 16, Child: Container{
//		Radius: 16, Color: th.Color.Glass, Child: content,
//	}}
//
// The blur stays inside the clip in effect, and an Opacity above fades it in over
// the sharp backdrop. It reads what is already on the surface, so a
// RepaintBoundary around it is drawn as part of the frame each time instead of
// keeping its pixels. See docs/ADR/0044-drawing-extensions.md.
type BackdropFilter struct {
	Sigma  float32
	Radius float32
	Child  core.Widget
}

// Key implements core.Widget.
func (BackdropFilter) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b BackdropFilter) CreateRenderObject() core.RenderObject {
	return &layout.RenderBackdropFilter{Sigma: b.Sigma, Radius: b.Radius}
}

// UpdateRenderObject implements core.RenderWidget.
func (b BackdropFilter) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderBackdropFilter)
	if r.Sigma != b.Sigma || r.Radius != b.Radius {
		r.Sigma, r.Radius = b.Sigma, b.Radius
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (b BackdropFilter) ChildWidget() core.Widget { return b.Child }

// ImageFiltered draws Child blurred, as filter: blur() does in CSS. Sigma is the
// standard deviation of the Gaussian in logical pixels, and the blur spreads
// three of them past Child. Zero draws Child as usual.
//
// Child is recorded into a layer of its own, which also makes ImageFiltered a
// RepaintBoundary, and the blur works on its pixels as one picture, a group of
// the canvas with a filter (paint.Canvas.PushGroup): an Opacity above fades the
// blurred picture, and a scale transition scales the blur. It blurs wherever it
// is, inside other layers and groups such as that of a Blend too. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
type ImageFiltered struct {
	Sigma float32
	Child core.Widget
}

// Key implements core.Widget.
func (ImageFiltered) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f ImageFiltered) CreateRenderObject() core.RenderObject {
	r := core.NewRenderRepaintBoundary()
	r.SetBlur(f.Sigma)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f ImageFiltered) UpdateRenderObject(ro core.RenderObject) {
	ro.(*core.RenderRepaintBoundary).SetBlur(f.Sigma)
}

// ChildWidget implements core.SingleChildWidget.
func (f ImageFiltered) ChildWidget() core.Widget { return f.Child }
