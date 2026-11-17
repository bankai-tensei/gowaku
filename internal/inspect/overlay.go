//go:build gowakudebug

package inspect

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// The colors of the overlay are fixed, not themed: they must read over any
// application, light or dark. The thick outlines have a faint dark halo, so
// that they stand out from a background of their own hue.
var (
	boundsColor = geom.RGBA(0.05, 0.65, 0.75, 0.55)
	hoverFill   = geom.RGBA(0.20, 0.50, 1.00, 0.18)
	hoverEdge   = geom.RGBA(0.20, 0.50, 1.00, 0.95)
	selectEdge  = geom.RGBA(1.00, 0.55, 0.10, 0.95)
	insetFill   = geom.RGBA(0.30, 0.75, 0.35, 0.28)
	// flashColor tints the layers recorded again: one quiet violet, apart
	// from the colors of hover, selection and insets.
	flashColor  = geom.RGBA(0.60, 0.45, 1.00, 1)
	halo        = geom.RGBA(0, 0, 0, 0.35)
	labelGround = geom.RGBA(0.08, 0.09, 0.11, 0.92)
	labelInk    = geom.RGBA(1, 1, 1, 1)
)

// The places of the labels of the overlay.
const (
	labelSelection = iota
	labelHover
	labelCount
)

// overlayLabel keeps the shaped label of an object from frame to frame:
// shaping allocates, and the overlay draws inside the frames it measures.
type overlayLabel struct {
	ro     core.RenderObject
	size   geom.Size
	layout *text.Layout
}

// Overlay draws what the session shows over t onto cv, after the picture of
// the window: outlines, tints of recorded layers, and the hovered and selected
// objects. Once the labels are shaped, it does not allocate.
func (s *Session) Overlay(t Target, cv *paint.Canvas) {
	tree := t.Tree()
	root := tree.RootRenderObject()
	if root == nil {
		return
	}
	win := geom.Rect{W: t.Size().W, H: t.Size().H}
	if s.ShowBounds {
		outlineAll(cv, root, geom.Translation(root.Offset()), win)
	}
	if s.ShowRepaints {
		s.drawFlashes(t, cv)
	}
	hovering := s.Picking && s.hoverTarget == t && s.hovered != nil && tree.HostedBy(s.hovered)
	if s.selTarget == t && s.selRO != nil && tree.HostedBy(s.selRO) {
		m := globalTransform(s.selRO)
		drawInsets(cv, s.selRO, m)
		drawBox(cv, m, s.selRO.Size(), 2, selectEdge)
		if !hovering {
			s.label(cv, labelSelection, win, s.selRO, m, selectEdge)
		}
	}
	if hovering {
		m := globalTransform(s.hovered)
		fillBox(cv, m, s.hovered.Size(), hoverFill)
		drawBox(cv, m, s.hovered.Size(), 1.5, hoverEdge)
		s.label(cv, labelHover, win, s.hovered, m, hoverEdge)
	}
}

// globalRect returns the upright bounds of ro in the window.
func globalRect(ro core.RenderObject) geom.Rect {
	sz := ro.Size()
	return core.LocalRectToGlobal(ro, geom.Rect{W: sz.W, H: sz.H})
}

// globalTransform returns the transform from the coordinates of ro to those of
// the window, through the Transformers above it.
func globalTransform(ro core.RenderObject) geom.Affine {
	m := geom.Identity()
	for cur := ro; cur != nil; cur = cur.Parent() {
		m = geom.Translation(cur.Offset()).Mul(m)
		if tr, ok := cur.Parent().(core.Transformer); ok {
			if c := tr.ChildTransform(); !c.IsIdentity() {
				m = c.Mul(m)
			}
		}
	}
	return m
}

// childTransform returns the transform of child, below ro whose transform is
// m.
func childTransform(ro, child core.RenderObject, m geom.Affine) geom.Affine {
	if tr, ok := ro.(core.Transformer); ok {
		if c := tr.ChildTransform(); !c.IsIdentity() {
			m = m.Mul(c)
		}
	}
	return m.Mul(geom.Translation(child.Offset()))
}

// hides reports whether ro hides its subtree, which core then leaves out of
// hit testing.
func hides(ro core.RenderObject) bool {
	h, ok := ro.(core.Hider)
	return ok && h.Hidden()
}

// hiddenAt reports whether ro or an object above it hides it.
func hiddenAt(ro core.RenderObject) bool {
	for cur := ro; cur != nil; cur = cur.Parent() {
		if hides(cur) {
			return true
		}
	}
	return false
}

// outlineAll outlines ro, whose transform is m, and the objects below it,
// but neither what has no size nor what lies outside win, and nothing that a
// Hider hides. The children of an object outside win may still reach into
// it, so they are visited all the same.
func outlineAll(cv *paint.Canvas, ro core.RenderObject, m geom.Affine, win geom.Rect) {
	if hides(ro) {
		return
	}
	if sz := ro.Size(); sz.W > 0 && sz.H > 0 {
		if b := m.ApplyRect(geom.Rect{W: sz.W, H: sz.H}); b.Intersect(win).W > 0 {
			drawBox(cv, m, sz, 1, boundsColor)
		}
	}
	for i := range ro.ChildCount() {
		if c := ro.ChildAt(i); c != nil {
			outlineAll(cv, c, childTransform(ro, c, m), win)
		}
	}
}

// drawBox outlines the box of size sz, whose transform is m, width thick in
// the pixels of the window and inside its edge. A turned box is outlined
// turned.
func drawBox(cv *paint.Canvas, m geom.Affine, sz geom.Size, width float32, c geom.Color) {
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	if m.IsScaleTranslate() {
		stroke(cv, m.ApplyRect(geom.Rect{W: sz.W, H: sz.H}), width, c)
		return
	}
	scale := geom.Sqrt32(geom.Abs32(m.Det()))
	if scale == 0 {
		return
	}
	cv.Save()
	cv.Transform(m)
	stroke(cv, geom.Rect{W: sz.W, H: sz.H}, width/scale, c)
	cv.Restore()
}

// fillBox fills the box of size sz, whose transform is m.
func fillBox(cv *paint.Canvas, m geom.Affine, sz geom.Size, c geom.Color) {
	if sz.W <= 0 || sz.H <= 0 {
		return
	}
	cv.Save()
	cv.Transform(m)
	cv.DrawRRect(geom.RRectAll(geom.Rect{W: sz.W, H: sz.H}, 0), paint.Paint{Color: c})
	cv.Restore()
}

// drawInsets shades the room between ro, whose transform is m, and its only
// child, such as the insets of a padding, the way the inspectors of browsers
// show them.
func drawInsets(cv *paint.Canvas, ro core.RenderObject, m geom.Affine) {
	if ro.ChildCount() != 1 || hides(ro) {
		return
	}
	c := ro.ChildAt(0)
	if c == nil {
		return
	}
	if tr, ok := ro.(core.Transformer); ok && !tr.ChildTransform().IsIdentity() {
		return
	}
	outer := geom.Rect{W: ro.Size().W, H: ro.Size().H}
	inner := geom.Rect{X: c.Offset().X, Y: c.Offset().Y, W: c.Size().W, H: c.Size().H}.Intersect(outer)
	if inner == outer || inner.W <= 0 || inner.H <= 0 {
		return
	}
	cv.Save()
	cv.Transform(m)
	insetStrip(cv, geom.Rect{W: outer.W, H: inner.Y})
	insetStrip(cv, geom.Rect{Y: inner.Y + inner.H, W: outer.W, H: outer.H - inner.Y - inner.H})
	insetStrip(cv, geom.Rect{Y: inner.Y, W: inner.X, H: inner.H})
	insetStrip(cv, geom.Rect{X: inner.X + inner.W, Y: inner.Y, W: outer.W - inner.X - inner.W, H: inner.H})
	cv.Restore()
}

func insetStrip(cv *paint.Canvas, r geom.Rect) {
	if r.W > 0 && r.H > 0 {
		cv.DrawRRect(geom.RRectAll(r, 0), paint.Paint{Color: insetFill})
	}
}

// stroke outlines r inside its edge. A thick line gets a faint dark halo
// just outside it, so that it reads on any background.
func stroke(cv *paint.Canvas, r geom.Rect, width float32, c geom.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if width > 1 {
		h := geom.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}
		cv.DrawRRect(geom.RRectAll(h, 0), paint.Paint{Stroke: &paint.Stroke{Width: 1, Color: halo}})
	}
	cv.DrawRRect(geom.RRectAll(r, 0), paint.Paint{Stroke: &paint.Stroke{Width: width, Color: c}})
}

// hasFlash reports whether a tint shows over t.
func (s *Session) hasFlash(t Target) bool {
	for i := range s.flashes {
		if s.flashes[i].target == t {
			return true
		}
	}
	return false
}

func (s *Session) drawFlashes(t Target, cv *paint.Canvas) {
	now := s.Now()
	for _, f := range s.flashes {
		if f.target != t {
			continue
		}
		fade := 1 - float32(now-f.at)/float32(flashLife)
		if fade <= 0 {
			continue
		}
		c := flashColor
		if !t.Tree().HostedBy(f.layer) || hiddenAt(f.layer) {
			continue
		}
		m := globalTransform(f.layer)
		sz := f.layer.Size()
		fill := c
		fill.A = 0.16 * fade
		fillBox(cv, m, sz, fill)
		c.A = 0.7 * fade
		drawBox(cv, m, sz, 1.5, c)
	}
}

// labelGap is the room between a label and the box it names.
const labelGap = 2

// label draws a tag that names ro, whose transform is m, next to its box:
// above it if there is room, else below, else inside its top; always inside
// win.
func (s *Session) label(cv *paint.Canvas, slot int, win geom.Rect, ro core.RenderObject, m geom.Affine, edge geom.Color) {
	l := s.labelLayout(slot, ro)
	if l == nil {
		return
	}
	sz := ro.Size()
	box := labelBox(m.ApplyRect(geom.Rect{W: sz.W, H: sz.H}), l.Size(), win)
	cv.DrawRRect(geom.RRectAll(box, 3), paint.Paint{Color: labelGround, Stroke: &paint.Stroke{Width: 1, Color: edge}})
	runs := l.Runs()
	at := geom.Offset{X: box.X + 5, Y: box.Y + 2}
	for i := range runs {
		cv.DrawGlyphs(&runs[i], at, labelInk)
	}
}

// labelBox places a label whose text is ts large next to r, inside win.
func labelBox(r geom.Rect, ts geom.Size, win geom.Rect) geom.Rect {
	box := geom.Rect{X: r.X, W: ts.W + 10, H: ts.H + 4}
	switch {
	case r.Y-box.H-labelGap >= win.Y:
		box.Y = r.Y - box.H - labelGap
	case r.Y+r.H+labelGap+box.H <= win.Y+win.H:
		box.Y = r.Y + r.H + labelGap
	default:
		box.Y = r.Y + labelGap
	}
	box.X = max(win.X, min(box.X, win.X+win.W-box.W))
	box.Y = max(win.Y, min(box.Y, win.Y+win.H-box.H))
	return box
}

// labelLayout returns the shaped label of ro in slot, shaping it again only
// when ro or its size changed.
func (s *Session) labelLayout(slot int, ro core.RenderObject) *text.Layout {
	if s.shaper == nil {
		return nil
	}
	c := &s.labels[slot]
	if c.layout == nil || c.ro != ro || c.size != ro.Size() {
		c.ro, c.size = ro, ro.Size()
		c.layout = s.shaper.Shape(describe(ro), text.Style{Size: 11, LineHeight: 1.3, Color: labelInk}).Layout(geom.Inf)
	}
	return c.layout
}

// describe returns the type of ro with its size, such as "RenderPadding 120×32".
func describe(ro core.RenderObject) string {
	sz := ro.Size()
	return fmt.Sprintf("%s  %s×%s", TypeName(ro), trim(sz.W), trim(sz.H))
}

func trim(v float32) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}

// TypeName returns the name of the type of v without its package path, such
// as "control.Button" for a control.Button, or "Tree[string]" for a
// generic one.
func TypeName(v any) string {
	if v == nil {
		return "<nil>"
	}
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := t.String()
	// gowaku.Keyed wraps its child in an unexported widget of core.
	if name == "core.keyed" {
		return "Keyed"
	}
	// A generic type names its arguments with their full paths.
	if i := strings.IndexByte(name, '['); i >= 0 {
		name = name[:i] + shortArgs(name[i:])
	}
	return name
}

// shortArgs drops the package paths inside the brackets of a generic type.
func shortArgs(args string) string {
	var b strings.Builder
	word := 0
	for i := 0; i < len(args); i++ {
		switch c := args[i]; c {
		case '/':
			b.WriteString("")
			// Forget the path so far: keep what follows the last slash.
			s := b.String()
			b.Reset()
			b.WriteString(s[:word])
		case '[', ']', ',', ' ', '*':
			b.WriteByte(c)
			word = b.Len()
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
