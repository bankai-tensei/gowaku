package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// AnimatedImage plays an animation, such as an animated GIF, APNG or WebP
// file read by paint.DecodeAnimation. It lays out as Image does with its
// first frame, and plays Source.Loops times, then holds the last frame.
//
// A frame change repaints the picture alone, in a repaint boundary of its
// own, without a build or a layout, and the window sleeps between frames. The
// animation pauses while a fully transparent Opacity hides it (ADR 0027) and
// stops when it leaves the tree. A window that asks for less motion shows the
// first frame and does not play it. Changing Source starts the new animation
// from its first frame. A nil Source shows nothing.
type AnimatedImage struct {
	core.StatelessBase
	Source        *paint.Animation
	Width, Height float32
	Fit           layout.ImageFit
	// Radius rounds the corners.
	Radius float32
	// Quality is how the frames are filtered when they are drawn at another
	// size, as in Image.
	Quality paint.FilterQuality
	// Label describes the picture to assistive technology.
	Label string
}

// Build implements core.StatelessWidget.
func (a AnimatedImage) Build(ctx core.BuildContext) core.Widget {
	return RepaintBoundary{Child: animatedImageView{a: a, still: ReduceMotionOf(ctx)}}
}

// animatedImageView is the render widget of AnimatedImage. It holds the
// widget in a field: embedding it would bring its Build along.
type animatedImageView struct {
	a     AnimatedImage
	still bool
}

// Key implements core.Widget.
func (animatedImageView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v animatedImageView) CreateRenderObject() core.RenderObject {
	r := &layout.RenderAnimatedImage{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v animatedImageView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderAnimatedImage)
	a := v.a
	r.SetAnimation(a.Source)
	r.SetStill(v.still)
	if r.Width != a.Width || r.Height != a.Height {
		r.MarkNeedsLayout()
	}
	if r.Fit != a.Fit || r.Radius != a.Radius || r.Quality != a.Quality {
		r.MarkNeedsPaint()
	}
	r.Width, r.Height, r.Fit, r.Radius, r.Quality, r.Label = a.Width, a.Height, a.Fit, a.Radius, a.Quality, a.Label
}
