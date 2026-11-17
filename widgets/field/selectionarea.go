package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// SelectionArea makes the Text and RichText below it selectable with the mouse
// and the keyboard, and copyable, as error messages and logs need to be. So is
// any other render object that implements basic.SelectableText, such as the
// paragraphs of the markdown module.
//
// A drag selects from where it started to the pointer, across the texts in the
// order they are painted, and the copy joins their parts with the
// SelectionBreak of each: "\n" before a Text or a RichText. A double
// click selects a word, a triple click the line between hard line breaks, and
// a Shift+click extends the selection. Ctrl+C (Cmd+C on macOS) copies and
// Ctrl+A selects every text of the area. Once a click has put an anchor down,
// Shift with the arrow keys, Home and End moves the free end; plain arrows are
// left to the widgets above, so there is no caret to browse with. Escape
// clears the selection. With an overlay.Overlay above, the secondary button and
// the menu key open a menu with Copy and Select all.
//
// Ruby is not copied: the copy holds the base text, and a ruby group is
// selected whole. Vertical and right-to-left lines are selected as they are
// painted.
//
// Text inside a widget that takes the focus, such as a button, a link or an
// Input, belongs to that control: a press on it does what the control does,
// and the selection skips its text. So does a SelectionArea inside this one,
// which keeps its own selection. Widgets that are not text are skipped.
//
// The selection goes when the area loses the focus to something else in the
// window, for example after a press outside it. It stays while the window
// itself is in the background, and while the menu of the area is open. Touch
// does not select: a finger scrolls.
//
// See docs/ADR/0089-selection-area.md and docs/ADR/0105-markdown-selection.md.
type SelectionArea struct {
	core.StatefulBase
	Child core.Widget
}

// CreateState implements core.StatefulWidget.
func (SelectionArea) CreateState() core.State { return &selectionAreaState{} }

// selectionAreaState keeps the selection, so that it outlives a new render
// object.
type selectionAreaState struct {
	core.StateOf[SelectionArea]
	sel selection
}

// Dispose implements core.State. A text that stays in the tree after the area
// is gone must not keep its highlight.
func (s *selectionAreaState) Dispose() {
	s.sel.clear()
	s.sel.dragging = false
	s.sel.region = nil
}

// Build implements core.State.
func (s *selectionAreaState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	var onMenu func(geom.Offset)
	if _, ok := core.Lookup[*overlay.Host](ctx); ok {
		onMenu = s.openMenu
	}
	return selectionRegion{
		sel:    &s.sel,
		color:  th.Color.Accent.WithAlpha(selectionOpacity),
		onMenu: onMenu,
		child:  s.Widget().Child,
	}
}

// openMenu shows Copy and Select all at pos, relative to the area.
func (s *selectionAreaState) openMenu(pos geom.Offset) {
	words := basic.LookupStrings(s.Context())
	var copyText func()
	if s.sel.hasRange() {
		copyText = s.sel.copy
	}
	items := []overlay.MenuItem{
		{Action: interact.Action{
			Label: words.Copy, OnInvoke: copyText,
			Shortcut: interact.Shortcut{Key: input.KeyC, Modifiers: input.ModControl},
		}},
		{Action: interact.Action{
			Label: words.SelectAll, OnInvoke: s.selectAll,
			Shortcut: interact.Shortcut{Key: input.KeyA, Modifiers: input.ModControl},
		}},
	}
	// The menu takes the focus while it is open; the selection is what its
	// items work on, so it must stay.
	s.sel.menuOpen = true
	overlay.ShowPopupMenu(s.Context(), pos, items)
}

// selectAll is the menu item: it selects everything, and gives the keyboard to
// the area so that Ctrl+C works next.
func (s *selectionAreaState) selectAll() {
	s.sel.selectAll()
	if r := s.sel.region; r != nil && r.CanFocus() {
		r.RequestFocus()
	}
}

// selectionRegion is SelectionArea with its look and menu found.
type selectionRegion struct {
	sel    *selection
	color  geom.Color
	onMenu func(geom.Offset)
	child  core.Widget
}

// Key implements core.Widget.
func (selectionRegion) Key() core.Key { return nil }

// ChildWidget implements core.SingleChildWidget.
func (w selectionRegion) ChildWidget() core.Widget { return w.child }

// CreateRenderObject implements core.RenderWidget.
func (w selectionRegion) CreateRenderObject() core.RenderObject {
	r := &renderSelectionRegion{sel: w.sel, onMenu: w.onMenu}
	w.sel.region = r
	w.sel.setColor(w.color)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (w selectionRegion) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderSelectionRegion)
	r.sel, r.onMenu = w.sel, w.onMenu
	w.sel.region = r
	w.sel.setColor(w.color)
}
