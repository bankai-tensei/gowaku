package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Icon draws a vector shape in a square. Package icons has common shapes,
// such as icons.Folder() and icons.Search(), drawn on the grid of 24 that
// Icon expects. Make the path once, from there or with paint.ParseSVGPath,
// and keep it: the scaled copy and the atlas entry depend on it staying the
// same. A nil Path leaves the square empty.
type Icon struct {
	Path *paint.Path
	// ViewBox is the size of the square the path is drawn in. Zero selects 24,
	// the grid of most icon sets.
	ViewBox float32
	// Size is the size on screen. Zero selects 24.
	Size float32
	// Color fills the shape. The zero value is opaque black.
	Color geom.Color
	// Label describes the icon to assistive technology.
	Label string
}

func (i Icon) size() float32 {
	if i.Size > 0 {
		return i.Size
	}
	return kit.DefaultIconSize
}

func (i Icon) color() geom.Color {
	if i.Color == (geom.Color{}) {
		return geom.Color{A: 1}
	}
	return i.Color
}

func (i Icon) scaled() *paint.Path {
	if i.Path == nil {
		return nil
	}
	box := i.ViewBox
	if box <= 0 {
		box = kit.DefaultIconSize
	}
	if box == i.size() {
		return i.Path
	}
	return i.Path.Scaled(i.size() / box)
}

// Key implements core.Widget.
func (Icon) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (i Icon) CreateRenderObject() core.RenderObject {
	return &iconObject{RenderIcon: layout.RenderIcon{Extent: i.size(), Color: i.color(), Label: i.Label, Path: i.scaled()}, source: i}
}

// iconObject remembers what its scaled path came from.
type iconObject struct {
	layout.RenderIcon
	source Icon
}

// UpdateRenderObject implements core.RenderWidget.
func (i Icon) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*iconObject)
	if r.source.Path != i.Path || r.source.ViewBox != i.ViewBox || r.source.Size != i.Size {
		r.Path = i.scaled()
		if r.Extent != i.size() {
			r.Extent = i.size()
			r.MarkNeedsLayout()
		}
		r.MarkNeedsPaint()
	}
	if r.Color != i.color() {
		r.Color = i.color()
		r.MarkNeedsPaint()
	}
	r.source, r.Label = i, i.Label
}

// Image shows a picture. Without Width and Height it takes the size of the
// picture, and with one of them it keeps the proportions.
type Image struct {
	Source        *paint.Image
	Width, Height float32
	Fit           layout.ImageFit
	// Radius rounds the corners.
	Radius float32
	// Quality is how the picture is filtered when it is drawn at another
	// size. The zero value smooths shrunk pictures with mipmaps; use
	// paint.FilterNearest for pixel art.
	Quality paint.FilterQuality
	// Label describes the picture to assistive technology.
	Label string
}

// Key implements core.Widget.
func (Image) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (m Image) CreateRenderObject() core.RenderObject {
	return &layout.RenderImage{
		Source: m.Source, Width: m.Width, Height: m.Height, Fit: m.Fit, Radius: m.Radius, Quality: m.Quality, Label: m.Label,
	}
}

// UpdateRenderObject implements core.RenderWidget.
func (m Image) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderImage)
	if r.Source != m.Source || r.Width != m.Width || r.Height != m.Height {
		r.MarkNeedsLayout()
	}
	if r.Fit != m.Fit || r.Radius != m.Radius || r.Quality != m.Quality {
		r.MarkNeedsPaint()
	}
	r.Source, r.Width, r.Height, r.Fit, r.Radius, r.Quality, r.Label = m.Source, m.Width, m.Height, m.Fit, m.Radius, m.Quality, m.Label
}
