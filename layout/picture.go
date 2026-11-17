package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// RenderIcon draws a path as an icon, Extent wide and high.
type RenderIcon struct {
	core.RenderBase
	// Extent is the width and height.
	Extent float32
	Color  geom.Color
	// Label describes the icon to assistive technology.
	Label string
	// Path is the shape, scaled to Extent.
	Path *paint.Path
}

// Layout implements core.RenderObject.
func (r *RenderIcon) Layout(c Constraints) geom.Size {
	return c.Constrain(geom.Size{W: r.Extent, H: r.Extent})
}

// Paint implements core.RenderObject.
func (r *RenderIcon) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Path == nil {
		return
	}
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.DrawPath(r.Path, paint.Paint{Color: r.Color})
	cv.Restore()
}

// Semantics implements core.RenderObject.
func (r *RenderIcon) Semantics(sb *core.SemanticsBuilder) {
	if r.Label != "" {
		sb.Open(semantics.Node{Role: semantics.RoleImage, Label: r.Label}, r.Size())
		sb.Close()
	}
}

// RenderSVG draws an SVG drawing fitted into a square, Extent wide and high. A
// drawing with groups keeps its composited picture on the GPU while its size,
// color and drawing stay the same (paint.SVGCache).
type RenderSVG struct {
	core.RenderBase
	Extent float32
	// Color is the value of currentColor.
	Color geom.Color
	// Label describes the drawing to assistive technology.
	Label string
	Data  *paint.SVG
	// cache keeps the composited picture of a drawing with groups.
	cache paint.SVGCache
}

// Layout implements core.RenderObject.
func (r *RenderSVG) Layout(c Constraints) geom.Size {
	return c.Constrain(geom.Size{W: r.Extent, H: r.Extent})
}

// Paint implements core.RenderObject.
func (r *RenderSVG) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Data == nil {
		return
	}
	sz := r.Size()
	r.Data.DrawCached(cv, geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}, r.Color, &r.cache)
}

// Semantics implements core.RenderObject.
func (r *RenderSVG) Semantics(sb *core.SemanticsBuilder) {
	if r.Label != "" {
		sb.Open(semantics.Node{Role: semantics.RoleImage, Label: r.Label}, r.Size())
		sb.Close()
	}
}

// ImageFit says how a picture fills its box.
type ImageFit uint8

// The image fits.
const (
	// FitContain shows the whole picture, centered, with room around it.
	FitContain ImageFit = iota
	// FitCover fills the box and cuts off what sticks out.
	FitCover
	// FitFill stretches the picture to the box.
	FitFill
)

// RenderImage draws a picture.
type RenderImage struct {
	core.RenderBase
	Source *paint.Image
	// Width and Height force a size. Zero takes the size of the picture.
	Width, Height float32
	Fit           ImageFit
	// Radius rounds the corners of the box.
	Radius float32
	// Quality is how the picture is filtered when it is drawn at another
	// size. The zero value is paint.FilterMipmap.
	Quality paint.FilterQuality
	// Label describes the picture to assistive technology.
	Label string
}

// Layout implements core.RenderObject.
func (r *RenderImage) Layout(c Constraints) geom.Size {
	var natural geom.Size
	if r.Source != nil {
		natural = r.Source.Size()
	}
	sz := geom.Size{W: r.Width, H: r.Height}
	switch {
	case sz.W == 0 && sz.H == 0:
		sz = natural
	case sz.W == 0 && natural.H > 0:
		sz.W = sz.H * natural.W / natural.H
	case sz.H == 0 && natural.W > 0:
		sz.H = sz.W * natural.H / natural.W
	}
	return c.Constrain(sz)
}

// destination returns where the picture goes inside a box of size sz.
func (r *RenderImage) destination(sz geom.Size) geom.Rect {
	natural := r.Source.Size()
	if r.Fit == FitFill || natural.W == 0 || natural.H == 0 {
		return geom.RectFromSize(sz)
	}
	sx, sy := sz.W/natural.W, sz.H/natural.H
	s := min(sx, sy)
	if r.Fit == FitCover {
		s = max(sx, sy)
	}
	w, h := natural.W*s, natural.H*s
	return geom.Rect{X: (sz.W - w) / 2, Y: (sz.H - h) / 2, W: w, H: h}
}

// Paint implements core.RenderObject.
func (r *RenderImage) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.Source == nil {
		return
	}
	sz := r.Size()
	cv.Save()
	cv.Translate(off.X, off.Y)
	cv.ClipRRect(geom.RRectAll(geom.RectFromSize(sz), r.Radius))
	cv.DrawImageQuality(r.Source, r.destination(sz), 1, r.Quality)
	cv.Restore()
}

// Semantics implements core.RenderObject.
func (r *RenderImage) Semantics(sb *core.SemanticsBuilder) {
	if r.Label != "" {
		sb.Open(semantics.Node{Role: semantics.RoleImage, Label: r.Label}, r.Size())
		sb.Close()
	}
}
