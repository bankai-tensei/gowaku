package basic

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// ShaderMask draws Child through a custom shader made by cmd/gowaku-shader: the
// child is drawn into a layer, which the shader reads as tex0 with sample0 at
// the place of each pixel, and what the shader returns is what shows. It can
// fade, tint, shimmer or bend the child, more than the masks of Flutter, which
// only multiply. Layout, the pointer and assistive technology see Child as it
// is.
//
// The shader reads Params, Texture, as tex1, and Time. Animate makes Time run
// on from where it starts, at the frame rate, in a repaint boundary of its
// own; the clock pauses while a fully
// transparent Opacity hides it (ADR 0027), and a window that asks for less
// motion holds it at Time. Under a scale transition, where the layer cannot
// follow, the child shows as it is, as it does inside the layer of another
// ShaderMask. See docs/ADR/0045-shaders-svg-animated-images.md.
type ShaderMask struct {
	core.StatelessBase
	Shader *paint.Shader
	// Params are the sixteen floats the shader reads as params[0] to params[3].
	Params paint.ShaderParams
	// Texture is the image the shader reads with sample1, stretched over the
	// box.
	Texture *paint.Image
	// Time is the time the shader sees, or where it starts with Animate.
	Time time.Duration
	// Animate runs the time of the shader.
	Animate bool
	Child   core.Widget
}

// Build implements core.StatelessWidget.
func (m ShaderMask) Build(ctx core.BuildContext) core.Widget {
	if m.Shader == nil || !m.Animate {
		return shaderMaskView{m: m}
	}
	// Each frame repaints the mask alone.
	return RepaintBoundary{Child: shaderMaskView{m: m, still: ReduceMotionOf(ctx)}}
}

// shaderMaskView is the render widget of ShaderMask.
type shaderMaskView struct {
	m     ShaderMask
	still bool
}

// Key implements core.Widget.
func (shaderMaskView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v shaderMaskView) CreateRenderObject() core.RenderObject {
	r := &layout.RenderShaderMask{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v shaderMaskView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderShaderMask)
	m := &v.m
	if r.Shader != m.Shader || r.Params != m.Params || r.Texture != m.Texture || r.Time != m.Time {
		r.Shader, r.Params, r.Texture, r.Time = m.Shader, m.Params, m.Texture, m.Time
		r.MarkNeedsPaint()
	}
	r.SetAnimate(m.Animate && m.Shader != nil, v.still)
}

// ChildWidget implements core.SingleChildWidget.
func (v shaderMaskView) ChildWidget() core.Widget { return v.m.Child }
