package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Wrap lays out its children in runs along Axis, like a Row that starts a new
// row when the next child does not fit, or a Column that starts a new column.
// It suits tags, chips and buttons of different lengths.
//
// Main places the children of each run along the axis, Cross places each child
// within its run, and RunAlignment places the runs across the axis. Spacing is
// the gap between children in a run and RunSpacing the gap between runs. Like
// Row, a Wrap takes all the length it is allowed along its axis unless Size is
// MainMin, and is as long across it as its runs. Where the length along the
// axis is unbounded, as for a horizontal Wrap inside a Row, nothing wraps.
//
// The zero value lays out its children in rows from the top left with no gaps.
type Wrap struct {
	Axis         layout.Axis
	Main         layout.MainAxisAlignment
	Cross        layout.CrossAxisAlignment
	RunAlignment layout.MainAxisAlignment
	Size         layout.MainAxisSize
	Spacing      float32
	RunSpacing   float32
	Children     []core.Widget
}

// Key implements core.Widget.
func (Wrap) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (w Wrap) CreateRenderObject() core.RenderObject {
	r := &layout.RenderWrap{}
	w.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (w Wrap) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderWrap)
	if r.Axis != w.Axis || r.Main != w.Main || r.Cross != w.Cross || r.RunAlignment != w.RunAlignment ||
		r.MainSize != w.Size || r.Spacing != w.Spacing || r.RunSpacing != w.RunSpacing {
		w.apply(r)
		r.MarkNeedsLayout()
	}
}

func (w Wrap) apply(r *layout.RenderWrap) {
	r.Axis, r.Main, r.Cross, r.RunAlignment = w.Axis, w.Main, w.Cross, w.RunAlignment
	r.MainSize, r.Spacing, r.RunSpacing = w.Size, w.Spacing, w.RunSpacing
}

// ChildWidgets implements core.MultiChildWidget.
func (w Wrap) ChildWidgets() []core.Widget { return w.Children }
