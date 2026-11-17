package layout

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// clockOwner is a render object whose shader a shaderClock drives.
type clockOwner interface {
	core.RenderObject
	NewTicker(fn func(now time.Duration)) *core.Ticker
	Attached() bool
}

// shaderClock is the time of a shader that animates. It counts while the
// object shows: an ancestor that hides it (core.Hider) pauses it, as it pauses
// an AnimatedImage, and it stops when the object leaves the tree.
type shaderClock struct {
	animate, still bool
	ticker         *core.Ticker
	// elapsed is how long the shader has played, counted while it runs.
	elapsed time.Duration
	// anchor is the ticker time at which it would have started had it never
	// paused, valid while anchored.
	anchor   time.Duration
	anchored bool
}

// set makes the clock run if animate is set and still is not, as for a window
// that asks for less motion; it reports whether that changed anything. A clock
// that stops goes back to zero.
func (c *shaderClock) set(animate, still bool) bool {
	if c.animate == animate && c.still == still {
		return false
	}
	c.animate, c.still = animate, still
	if !animate || still {
		c.elapsed, c.anchored = 0, false
		if c.ticker != nil {
			c.ticker.Stop()
		}
	}
	return true
}

// now returns the time the shader sees: start, plus how long it has played.
func (c *shaderClock) now(start time.Duration) time.Duration { return start + c.elapsed }

// paint starts the ticker of a clock that runs, from the first paint of r on.
func (c *shaderClock) paint(r clockOwner) {
	if !c.animate || c.still {
		return
	}
	if c.ticker == nil {
		c.ticker = r.NewTicker(func(now time.Duration) { c.tick(r, now) })
	}
	if !c.ticker.Running() {
		c.anchored = false
		c.ticker.Start()
	}
}

func (c *shaderClock) tick(r clockOwner, now time.Duration) {
	if !r.Attached() || !c.animate || c.still {
		c.ticker.Stop()
		return
	}
	if hiddenAbove(r) {
		// Keep the time, and look again later, without frames.
		if c.anchored {
			c.elapsed = now - c.anchor
		}
		c.anchored = false
		c.ticker.SleepUntil(now + hiddenPoll)
		return
	}
	if !c.anchored {
		c.anchor, c.anchored = now-c.elapsed, true
	}
	c.elapsed = now - c.anchor
	r.MarkNeedsPaint()
}

// hiddenAbove reports whether r or an ancestor hides r.
func hiddenAbove(r core.RenderObject) bool {
	for p := r; p != nil; p = p.Parent() {
		if h, ok := p.(core.Hider); ok && h.Hidden() {
			return true
		}
	}
	return false
}

// ShaderPaint is what RenderCustomPaint fills its box with before its Painter
// draws: a custom shader with its parameters and up to two images.
type ShaderPaint struct {
	Shader *paint.Shader
	Params paint.ShaderParams
	// Textures are what sample0 and sample1 read, stretched over the box.
	Textures [2]*paint.Image
	// Time is the time the shader sees, or where it starts if it animates.
	Time time.Duration
}

// SetShader makes the object fill its box with s before its Painter draws,
// and animate it if animate is set and still is not. It repaints the object
// if anything changed.
func (r *RenderCustomPaint) SetShader(s ShaderPaint, animate, still bool) {
	changed := r.clock.set(animate, still)
	if changed || r.shader != s {
		r.shader = s
		r.MarkNeedsPaint()
	}
}

// Shader returns what SetShader set.
func (r *RenderCustomPaint) Shader() ShaderPaint { return r.shader }

// ShaderTime returns the time the shader sees now.
func (r *RenderCustomPaint) ShaderTime() time.Duration { return r.clock.now(r.shader.Time) }

// paintShader fills the box at off with the shader, if there is one.
func (r *RenderCustomPaint) paintShader(cv *paint.Canvas, off geom.Offset) {
	s := &r.shader
	if s.Shader == nil {
		return
	}
	sz := r.Size()
	cv.DrawShader(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, paint.ShaderCall{
		Shader: s.Shader,
		Params: s.Params,
		Time:   r.clock.now(s.Time),
		Tex0:   paint.ShaderTexture{Image: s.Textures[0]},
		Tex1:   paint.ShaderTexture{Image: s.Textures[1]},
	})
	r.clock.paint(r)
}

// RenderShaderMask draws its child into a layer and fills its box with a custom
// shader that reads the layer as tex0, at the place of the child: what the
// shader returns is what shows, instead of the child. Under a scale transition,
// where a layer cannot follow, it draws the child as it is.
type RenderShaderMask struct {
	RenderProxy
	Shader *paint.Shader
	Params paint.ShaderParams
	// Texture is what sample1 reads, stretched over the box.
	Texture *paint.Image
	// Time is the time the shader sees, or where it starts if it animates.
	Time  time.Duration
	clock shaderClock
	layer paint.Layer
}

// SetAnimate makes the shader animate if animate is set and still is not.
func (r *RenderShaderMask) SetAnimate(animate, still bool) {
	if r.clock.set(animate, still) {
		r.MarkNeedsPaint()
	}
}

// ShaderTime returns the time the shader sees now.
func (r *RenderShaderMask) ShaderTime() time.Duration { return r.clock.now(r.Time) }

// Paint implements core.RenderObject.
func (r *RenderShaderMask) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child == nil {
		return
	}
	if r.Shader == nil || !cv.CanShadeLayer() {
		r.RenderProxy.Paint(cv, off)
		return
	}
	r.child.Paint(r.layer.Begin(), r.child.Offset())
	r.layer.End()
	sz := r.Size()
	cv.DrawShader(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, paint.ShaderCall{
		Shader: r.Shader,
		Params: r.Params,
		Time:   r.clock.now(r.Time),
		Tex0:   paint.ShaderTexture{Layer: &r.layer, Origin: off},
		Tex1:   paint.ShaderTexture{Image: r.Texture},
	})
	r.clock.paint(r)
}
