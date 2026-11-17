package markdown

import (
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

const (
	// imageRadius rounds the corners of a picture of the document.
	imageRadius = 6
	// maxSVGFile bounds the size of an SVG file that is read.
	maxSVGFile = 8 << 20
	// defaultSVGSize is the size of a drawing that gives none, as a browser
	// makes it.
	defaultSVGSize = 150
)

// pictureFile is a picture read from Markdown.Images: a bitmap or an SVG
// drawing.
type pictureFile struct {
	image *paint.Image
	svg   *paint.SVG
}

// size returns the size the picture has of itself, in pixels.
func (p *pictureFile) size() geom.Size {
	if p.image != nil {
		return p.image.Size()
	}
	sz := geom.Size{W: p.svg.Width, H: p.svg.Height}
	if sz.W <= 0 || sz.H <= 0 {
		sz = geom.Size{W: defaultSVGSize, H: defaultSVGSize}
	}
	return sz
}

// imageCache holds the pictures of a document that were read from
// Markdown.Images, by the source as written. A source that could not be read
// is kept as nil, so that it is tried once and not on every build.
//
// The pictures go when the document changes, not when Images does: an fs.FS
// need not be comparable (fstest.MapFS is a map, and comparing two panics),
// so there is no telling whether the parent passed another one.
type imageCache struct {
	byRef map[string]*pictureFile
}

// reset forgets every picture.
func (c *imageCache) reset() { clear(c.byRef) }

// load returns the picture of src from the file system from, or nil if there is
// no file system, src is not a file in it, or the file is no picture.
func (c *imageCache) load(from fs.FS, src string) *pictureFile {
	if from == nil {
		return nil
	}
	if m, ok := c.byRef[src]; ok {
		return m
	}
	if c.byRef == nil {
		c.byRef = map[string]*pictureFile{}
	}
	m := readPicture(from, src)
	c.byRef[src] = m
	return m
}

// readPicture reads the picture at src, a path relative to the root of from
// as a document writes it: "logo.png", "./img/logo.png". A file named .svg is
// read as an SVG drawing. An address with a scheme, a path that leaves the
// root, and a file that is no PNG, JPEG, WebP or SVG give nil.
func readPicture(from fs.FS, src string) *pictureFile {
	name, ok := imagePath(src)
	if !ok {
		return nil
	}
	f, err := from.Open(name)
	if err != nil {
		return nil
	}
	// Nothing was written, so there is nothing to lose by a failing Close.
	defer f.Close() //nolint:errcheck // read only
	if strings.EqualFold(path.Ext(name), ".svg") {
		data, err := io.ReadAll(io.LimitReader(f, maxSVGFile+1))
		if err != nil || len(data) > maxSVGFile {
			return nil
		}
		d, err := paint.ParseSVG(data)
		if err != nil {
			return nil
		}
		return &pictureFile{svg: d}
	}
	m, err := paint.DecodeImage(f)
	if err != nil {
		return nil
	}
	return &pictureFile{image: m}
}

// imagePath returns src as a name fs.FS takes, and whether it is one: no
// scheme, no query, and nothing above the root.
func imagePath(src string) (string, bool) {
	if src == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "//") ||
		strings.HasPrefix(src, "data:") || strings.ContainsAny(src, "?#\\") {
		return "", false
	}
	name := path.Clean(strings.TrimPrefix(src, "/"))
	if !fs.ValidPath(name) || name == "." {
		return "", false
	}
	return name, true
}

// picture shows p at size, or smaller where the document is narrower, keeping
// its proportions; with a fraction, at that fraction of the width there is.
// ink is the currentColor of a drawing.
func picture(p *pictureFile, size geom.Size, fraction float32, alt string, ink geom.Color) core.Widget {
	var child core.Widget
	if p.image != nil {
		child = basic.Image{Source: p.image, Radius: imageRadius, Label: alt}
	} else {
		child = svgPicture{data: p.svg, color: ink, label: alt}
	}
	return shrinkBox{natural: size, fraction: fraction, child: child}
}

// shrinkBox gives its child the size natural, scaled down to the width there
// is. A fraction makes the width that fraction of the width there is, and
// the height follow.
type shrinkBox struct {
	natural  geom.Size
	fraction float32
	child    core.Widget
}

// Key implements core.Widget.
func (shrinkBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (s shrinkBox) CreateRenderObject() core.RenderObject {
	return &renderShrink{natural: s.natural, fraction: s.fraction}
}

// UpdateRenderObject implements core.RenderWidget.
func (s shrinkBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderShrink)
	if r.natural != s.natural || r.fraction != s.fraction {
		r.natural, r.fraction = s.natural, s.fraction
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (s shrinkBox) ChildWidget() core.Widget { return s.child }

type renderShrink struct {
	core.RenderBase
	natural  geom.Size
	fraction float32
	child    core.RenderObject
}

// SetChild implements core.SingleChildRenderObject.
func (r *renderShrink) SetChild(child core.RenderObject) { r.child = child }

// ChildCount implements core.RenderObject.
func (r *renderShrink) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

// ChildAt implements core.RenderObject.
func (r *renderShrink) ChildAt(int) core.RenderObject { return r.child }

// Layout implements core.RenderObject.
func (r *renderShrink) Layout(c geom.Constraints) geom.Size {
	sz := r.natural
	if r.fraction > 0 && c.MaxW < geom.Inf && sz.W > 0 {
		w := c.MaxW * r.fraction
		sz = geom.Size{W: w, H: sz.H * w / sz.W}
	}
	if sz.W > c.MaxW && sz.W > 0 {
		sz = geom.Size{W: c.MaxW, H: sz.H * c.MaxW / sz.W}
	}
	if r.child != nil {
		core.LayoutChild(r.child, geom.Tight(sz))
		r.child.SetOffset(geom.Offset{})
	}
	return c.Constrain(sz)
}

// Paint implements core.RenderObject.
func (r *renderShrink) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.child != nil {
		r.child.Paint(cv, off.Add(r.child.Offset()))
	}
}

// svgPicture draws an SVG drawing filling its box, which shrinkBox sizes.
type svgPicture struct {
	data  *paint.SVG
	color geom.Color
	label string
}

// Key implements core.Widget.
func (svgPicture) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p svgPicture) CreateRenderObject() core.RenderObject {
	return &layout.RenderSVG{Data: p.data, Color: p.color, Label: p.label}
}

// UpdateRenderObject implements core.RenderWidget.
func (p svgPicture) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderSVG)
	if r.Data != p.data || r.Color != p.color {
		r.Data, r.Color = p.data, p.color
		r.MarkNeedsPaint()
	}
	r.Label = p.label
}
