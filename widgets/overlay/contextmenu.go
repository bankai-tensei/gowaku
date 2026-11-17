package overlay

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// contextAnchorHeight is the most of a focused object that a context menu
// opened from the keyboard goes below.
const contextAnchorHeight = 2 * kit.MenuItemHeight

// ShowPopupMenu opens items as a menu at pos, in logical pixels from the top
// left of the widget of ctx, as GestureDetector reports places. The menu stays
// inside the window, opening up or to the left where there is no room. It
// closes when an item runs, on Escape, or on a press beside it, and gives the
// focus back. It must not be called during a Build.
func ShowPopupMenu(ctx core.BuildContext, pos geom.Offset, items []MenuItem) {
	showPopupMenu(ctx, pos, items, false)
}

// ShowMenuBelow opens items as a menu below anchor, as a toolbar shows the
// buttons that do not fit. keyboard puts the focus on the first item, for a
// menu opened from the keyboard. Like ShowPopupMenu, the menu stays inside the
// window, closes when an item runs, on Escape or on a press beside it, and
// gives the focus back. It must not be called during a Build.
func ShowMenuBelow(ctx core.BuildContext, anchor core.RenderObject, items []MenuItem, keyboard bool) {
	openMenu(ctx, items, kit.GlobalRect(anchor), layout.PopupBelow, nil, kit.FocusOf(ctx), keyboard)
}

func showPopupMenu(ctx core.BuildContext, pos geom.Offset, items []MenuItem, keyboard bool) {
	ro := ctx.RenderObject()
	if ro == nil || len(items) == 0 {
		return
	}
	at := core.LocalToGlobal(ro, pos)
	openMenu(ctx, items, geom.Rect{X: at.X, Y: at.Y}, layout.PopupBelow, nil, kit.FocusOf(ctx), keyboard)
}

// ContextMenu opens Items as a menu where the secondary button is clicked on
// Child, where a finger or a pen holds it, or below what has the focus inside Child when the menu key or
// Shift+F10 is pressed.
type ContextMenu struct {
	core.StatelessBase
	Items []MenuItem
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (m ContextMenu) Build(ctx core.BuildContext) core.Widget {
	return shortcuts{
		onShortcut: func(e *core.KeyEvent) bool { return m.key(ctx, e) },
		child: interact.GestureDetector{
			// The gesture sits at the place of the shortcuts, which is what
			// ctx finds.
			OnSecondaryTapAt: func(t layout.Tap) { showPopupMenu(ctx, t.Pos, m.Items, false) },
			// A finger or a pen has no secondary button: they hold.
			OnLongPress: func(pos geom.Offset, device input.PointerDevice) {
				if device != input.DeviceMouse {
					showPopupMenu(ctx, pos, m.Items, false)
				}
			},
			Child: m.Child,
		},
	}
}

// key opens the menu from the keyboard, if the focus is inside.
func (m ContextMenu) key(ctx core.BuildContext, e *core.KeyEvent) bool {
	if e.Key != input.KeyContextMenu && (e.Key != input.KeyF10 || e.Modifiers != input.ModShift) {
		return false
	}
	ro := ctx.RenderObject()
	focus := kit.FocusFrom(ro)
	if focus == nil || !isAncestor(ro, focus) {
		return false
	}
	// A tall object, such as a text area, gets the menu near its top rather
	// than below it.
	r := kit.GlobalRect(focus)
	openMenu(ctx, m.Items, geom.Rect{X: r.X, Y: r.Y, H: min(r.H, contextAnchorHeight)}, layout.PopupBelow, nil,
		focus.(core.FocusTarget), true)
	return true
}

// isAncestor reports whether a is ro or one of its ancestors.
func isAncestor(a, ro core.RenderObject) bool {
	for ; ro != nil; ro = ro.Parent() {
		if ro == a {
			return true
		}
	}
	return false
}
