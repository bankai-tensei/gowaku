package markdown

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// linkTarget is a link within a richText: it takes the pointer over the
// link's part of the text, the focus, and the link's place in the
// accessibility tree. The richText around it draws the underline and the
// focus ring, since the text it covers may run over several lines.
type linkTarget struct {
	url, label string
	tap        func(url string)
	// index is the place of the link among the links of the paragraph.
	index int
}

// Key implements core.Widget.
func (linkTarget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t linkTarget) CreateRenderObject() core.RenderObject {
	return &renderLink{url: t.url, label: t.label, tap: t.tap, index: t.index}
}

// UpdateRenderObject implements core.RenderWidget. The boxes come from the
// parent at its layout.
func (t linkTarget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderLink)
	r.url, r.label, r.tap = t.url, t.label, t.tap
	if r.index != t.index {
		r.index = t.index
		if p := r.Parent(); p != nil {
			p.MarkNeedsLayout()
		}
	}
}

// renderLink is the object of a linkTarget. Its own box is the first
// fragment of the link; boxes, in the coordinates of the parent, are all of
// them.
type renderLink struct {
	core.RenderBase
	url, label string
	tap        func(url string)
	index      int
	boxes      []geom.Rect

	hovered, pressed, shown bool
}

var (
	_ core.PointerTarget        = (*renderLink)(nil)
	_ core.HoverTarget          = (*renderLink)(nil)
	_ core.CursorSource         = (*renderLink)(nil)
	_ core.FocusTarget          = (*renderLink)(nil)
	_ core.FocusVisibleListener = (*renderLink)(nil)
	_ core.SemanticsActor       = (*renderLink)(nil)
)

// Layout implements core.RenderObject. The parent gives the size of the
// first fragment.
func (r *renderLink) Layout(c geom.Constraints) geom.Size { return c.Constrain(geom.Size{}) }

// Paint implements core.RenderObject. The parent draws the link.
func (r *renderLink) Paint(*paint.Canvas, geom.Offset) {}

// HitTest implements core.RenderObject. Only the fragments of the link take
// the pointer, not the box around them.
func (r *renderLink) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	if !r.over(pos) {
		return false
	}
	res.Path = append(res.Path, r)
	return true
}

// over reports whether pos, relative to the object, is over a fragment.
func (r *renderLink) over(pos geom.Offset) bool {
	p := pos.Add(r.Offset())
	for _, b := range r.boxes {
		if b.Contains(p) {
			return true
		}
	}
	return false
}

// HandlePointer implements core.PointerTarget. A click is a press and a
// release over the link.
func (r *renderLink) HandlePointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		if e.Button == input.ButtonPrimary {
			r.pressed = true
			e.Handled = true
		}
	case core.PointerUp:
		if r.pressed {
			r.pressed = false
			if r.over(e.Pos) {
				r.activate()
			}
		}
	case core.PointerCancel:
		r.pressed = false
	}
}

// HandleHover implements core.HoverTarget.
func (r *renderLink) HandleHover(inside bool) {
	if r.hovered != inside {
		r.hovered = inside
		r.repaintParent()
	}
}

// MouseCursor implements core.CursorSource.
func (r *renderLink) MouseCursor() input.Cursor { return input.CursorHand }

// CanFocus implements core.FocusTarget.
func (r *renderLink) CanFocus() bool { return r.tap != nil }

// HandleKey implements core.FocusTarget. Enter follows the link, as in a
// browser; Space is left to scroll the page.
func (r *renderLink) HandleKey(e *core.KeyEvent) bool {
	if e.Key != input.KeyEnter || e.Modifiers != 0 {
		return false
	}
	if !e.Repeat {
		r.activate()
	}
	return true
}

// HandleText implements core.FocusTarget.
func (r *renderLink) HandleText(string) {}

// HandleComposition implements core.FocusTarget.
func (r *renderLink) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderLink) FocusChanged(focused bool) { r.show(focused && r.FocusVisible()) }

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderLink) FocusVisibleChanged(visible bool) { r.show(visible) }

func (r *renderLink) show(v bool) {
	if r.shown != v {
		r.shown = v
		r.repaintParent()
	}
}

func (r *renderLink) repaintParent() {
	if p := r.Parent(); p != nil {
		p.MarkNeedsPaint()
	}
}

// Semantics implements core.RenderObject.
func (r *renderLink) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleLink, Label: r.label, Value: r.url, Actions: semantics.ActionTap}, r.Size())
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderLink) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	if a != semantics.ActionTap {
		return false
	}
	r.activate()
	return true
}

func (r *renderLink) activate() {
	if r.tap != nil {
		r.tap(r.url)
	}
}
