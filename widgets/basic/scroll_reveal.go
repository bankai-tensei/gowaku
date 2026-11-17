package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// ScrollIntoView moves the Scrolls around ro at once so that rect, in the
// coordinates of ro, is in view: each puts rect at align along its axis, where
// 0 puts the start of rect at the start of the view, 1 its end at the end, and
// 0.5 its middle in the middle. Each Scroll stays within its content, so a
// rect near the end cannot come to the start. The nearest Scroll moves first,
// and then the ones around it, as for a heading in a document inside a
// scrolled page.
//
// Call it at event time, or from a ticker, after ro has been laid out; never
// during a build or a layout. It moves only Scrolls (and views made of
// Viewport); List, Tree, Table and the editor scroll through their own
// controllers.
func ScrollIntoView(ro core.RenderObject, rect geom.Rect, align float32) {
	for p := ro.Parent(); p != nil; p = p.Parent() {
		v, ok := p.(*layout.RenderViewport)
		if !ok || v.ChildCount() == 0 || v.Position == nil {
			continue
		}
		at, ok := core.GlobalToLocal(v, core.LocalToGlobal(ro, rect.Origin()))
		if !ok {
			continue
		}
		view, content := v.Size(), v.ChildAt(0).Size()
		start, length, viewLen, contentLen := at.Y, rect.H, view.H, content.H
		if v.Axis == layout.Horizontal {
			start, length, viewLen, contentLen = at.X, rect.W, view.W, content.W
		}
		pos := v.Position.Value()
		// The content moves against the offset, or with it in reverse.
		move := start - align*(viewLen-length)
		if v.Reverse {
			move = -move
		}
		target := pos + float64(move)
		target = min(max(target, 0), float64(max(contentLen-viewLen, 0)))
		if target == pos {
			continue
		}
		if v.OnJump != nil {
			v.OnJump(target)
		} else {
			v.Position.Set(target)
		}
	}
}
