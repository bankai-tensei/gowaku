package collection

import (
	"math"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a tree. See docs/DESIGN.md §4.
const (
	treeRowHeight = 28
	// treeIndent is how far each level moves in.
	treeIndent = theme.Space16
	// treeInset keeps the ground of a row off the sides of the tree, and
	// treeLead is the room before the chevron of a root.
	treeInset  = theme.Space4
	treeLead   = theme.Space4
	treeRadius = theme.Space6
	// treeChevron is the box of the chevron, which leaves show empty so that
	// the labels of siblings line up.
	treeChevron = theme.Space16
	treeIcon    = theme.Space16
	// treeGap is the room after the chevron, and treeIconGap after the icon.
	treeGap     = theme.Space2
	treeIconGap = theme.Space6
	treeRing    = 2

	treeSpinnerSize      = 12
	treeSpinnerThickness = 1.5
	treeSpinnerShare     = 0.3
	// treeSpinTurns is the speed of the spinner, in turns a second.
	treeSpinTurns = 1.2
	// treeEditorShift moves the field of a rename left of the label, so that
	// its text stays near where the label was.
	treeEditorShift = theme.Space4
)

// treeLook is what the rows take from the theme.
type treeLook struct {
	ink, loading, chevron      geom.Color
	hover, selected, ring, bar geom.Color
	text                       text.Style
}

// newTreeLook returns the colors of the rows. The selection is the accent tint
// while the keyboard is in the tree and a quiet grey otherwise, as Finder
// shows which list the keys move.
func newTreeLook(th *theme.Theme, active bool) treeLook {
	c := &th.Color
	l := treeLook{
		ink: c.Ink, loading: c.Ink2, chevron: c.Ink3,
		hover: c.Fill, selected: c.FillStrong, ring: c.AccentInk.WithAlpha(treeRingOpacity), bar: c.Accent,
		text: kit.Colored(th.Text.Body, c.Ink),
	}
	if active {
		l.selected = c.AccentTint
	}
	return l
}

// treeRingOpacity is stronger than the ring around a control, because it is
// drawn inside the row on top of the selection.
const treeRingOpacity = 0.6

// treeHost is what a row asks of the tree that built it.
type treeHost interface {
	// rowPress handles a press on row i at pos, relative to the row, and
	// reports whether the row takes the keyboard.
	rowPress(i int, pos geom.Offset, e *core.PointerEvent) bool
	// rowSecondaryPress handles a press of the secondary button on row i,
	// and reports whether the tree opens context menus itself.
	rowSecondaryPress(i int) bool
	// rowContextMenu asks for the context menu of row i at the place at in
	// the window, and reports whether the tree has OnContextMenu.
	rowContextMenu(i int, at geom.Offset) bool
	// rowPointer follows the pointer after a press, at pos in the window.
	rowPointer(kind core.PointerKind, pos geom.Offset)
	rowKey(e *core.KeyEvent) bool
	rowText(s string)
	rowFocused(i int)
	rowAction(i int, a semantics.Actions) bool
	spinnerAngle() float32
	// dragging reports a drag of rows, during which no row lights up under
	// the pointer: the drop indicator shows where they go.
	dragging() bool
	setActive(active bool)
	bodyPress()
	// lostFocus reports that nothing has the keyboard, as when the row of the
	// focus left the tree with its node.
	lostFocus()
	measured(view float32)
	Wheel(notches float32)
}

// labelX returns where the label of a row at depth starts.
func treeLabelX(depth int, icon bool) float32 {
	x := treeContentX(depth) + treeChevron + treeGap
	if icon {
		x += treeIcon + treeIconGap
	}
	return x
}

// treeContentX returns where the chevron of a row at depth starts.
func treeContentX(depth int) float32 { return treeInset + treeLead + float32(depth)*treeIndent }

// treeRowView is the widget of one row.
type treeRowView struct {
	host   treeHost
	index  int
	depth  int
	shaper *text.Shaper
	look   *treeLook
	label  string
	// level, pos and size are the depth from 1 and the place among the
	// siblings, for assistive technology.
	level, pos, size                          int
	expandable, expanded, selected, focusable bool
	// claim makes the row take the keyboard at its next layout.
	claim   bool
	loading bool
	// angle turns the chevron, from 0 (right) to 90 (down). turning is the
	// path to draw it in while it moves; otherwise chevrons holds both rests.
	angle    float32
	turning  *paint.Path
	chevrons [2]*paint.Path
	arc      *paint.Path
	icon     core.Widget
	editor   core.Widget
	// iconRoom leaves the room of an icon before the label of a row that has
	// none, as the loading row does in a tree with icons.
	iconRoom bool
}

// Key implements core.Widget.
func (treeRowView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v treeRowView) CreateRenderObject() core.RenderObject {
	r := &renderTreeRow{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v treeRowView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTreeRow)
	if r.shaper != v.shaper || r.label != v.label || !r.style.SameLayout(v.look.text) {
		r.shaper, r.label, r.style = v.shaper, v.label, v.look.text
		r.laidOut = nil
		r.MarkNeedsLayout()
	}
	icon, editor := v.icon != nil, v.editor != nil
	if r.depth != v.depth || r.hasIcon != icon || r.hasEditor != editor || r.iconRoom != v.iconRoom || v.claim {
		r.depth, r.hasIcon, r.hasEditor, r.iconRoom = v.depth, icon, editor, v.iconRoom
		r.MarkNeedsLayout()
	}
	r.claim = r.claim || v.claim
	r.host, r.index = v.host, v.index
	r.level, r.pos, r.size = v.level, v.pos, v.size
	if r.look != *v.look || r.expandable != v.expandable || r.expanded != v.expanded || r.selected != v.selected ||
		r.focusable != v.focusable || r.loading != v.loading || r.angle != v.angle || r.turning != v.turning {
		r.look = *v.look
		r.expandable, r.expanded, r.selected = v.expandable, v.expanded, v.selected
		r.focusable, r.loading, r.angle, r.turning = v.focusable, v.loading, v.angle, v.turning
		r.MarkNeedsPaint()
	}
	r.chevrons, r.arc = v.chevrons, v.arc
}

// ChildWidgets implements core.MultiChildWidget.
func (v treeRowView) ChildWidgets() []core.Widget {
	switch {
	case v.icon != nil && v.editor != nil:
		return []core.Widget{v.icon, v.editor}
	case v.icon != nil:
		return []core.Widget{v.icon}
	case v.editor != nil:
		return []core.Widget{v.editor}
	}
	return nil
}

// renderTreeRow draws a row of a tree: its ground, chevron, icon and label.
// It is the keyboard focus of the tree while it is the row of the focus, and
// describes the node to assistive technology. Paint does not allocate.
type renderTreeRow struct {
	core.RenderBase
	host                 treeHost
	index, depth         int
	level, pos, size     int
	shaper               *text.Shaper
	label                string
	style                text.Style
	look                 treeLook
	expandable, expanded bool
	selected, focusable  bool
	loading, claim       bool
	hasIcon, hasEditor   bool
	iconRoom, hovered    bool
	// menuPress is a press of the secondary button that the row keeps for
	// OnContextMenu, and reclaim takes the keyboard back at the next layout
	// after a secondary press that went on to the objects below.
	menuPress, reclaim bool
	angle              float32
	turning            *paint.Path
	// turned is the angle turning was last drawn for.
	turned   float32
	chevrons [2]*paint.Path
	arc      *paint.Path
	kids     []core.RenderObject
	laidOut  *text.Layout
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderTreeRow) SetChildren(children []core.RenderObject) { r.kids = children }

// ChildCount implements core.RenderObject.
func (r *renderTreeRow) ChildCount() int { return len(r.kids) }

// ChildAt implements core.RenderObject.
func (r *renderTreeRow) ChildAt(i int) core.RenderObject { return r.kids[i] }

func (r *renderTreeRow) icon() core.RenderObject {
	if r.hasIcon {
		return r.kids[0]
	}
	return nil
}

func (r *renderTreeRow) editor() core.RenderObject {
	if r.hasEditor {
		return r.kids[len(r.kids)-1]
	}
	return nil
}

// Layout implements core.RenderObject.
func (r *renderTreeRow) Layout(c geom.Constraints) geom.Size {
	sz := c.Constrain(geom.Size{W: c.MaxW, H: treeRowHeight})
	if r.laidOut == nil && r.shaper != nil {
		r.laidOut = r.shaper.Shape(r.label, r.style).Layout(geom.Inf)
	}
	x := treeContentX(r.depth) + treeChevron + treeGap
	if icon := r.icon(); icon != nil {
		isz := core.LayoutChild(icon, geom.Loose(geom.Size{W: treeIcon, H: treeIcon}))
		icon.SetOffset(geom.Offset{X: x + (treeIcon-isz.W)/2, Y: (sz.H - isz.H) / 2})
	}
	if ed := r.editor(); ed != nil {
		left := treeLabelX(r.depth, r.hasIcon) - treeEditorShift
		core.LayoutChild(ed, geom.Tight(geom.Size{W: max(sz.W-left-treeInset, 0), H: sz.H}))
		ed.SetOffset(geom.Offset{X: left})
	}
	if r.claim {
		r.claim = false
		if r.focusable {
			r.RequestFocus()
		}
	}
	if r.reclaim {
		// A press that nothing took cleared the focus; one that opened a menu
		// gave it to the menu, which keeps it.
		r.reclaim = false
		if r.focusable && kit.FocusFrom(r) == nil {
			r.RequestFocus()
		}
	}
	return sz
}

// Paint implements core.RenderObject.
func (r *renderTreeRow) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	l := &r.look
	pill := geom.RRectAll(geom.Rect{X: off.X + treeInset, Y: off.Y + 1, W: sz.W - 2*treeInset, H: sz.H - 2}, treeRadius)
	switch {
	case r.selected:
		cv.DrawRRect(pill, paint.Paint{Color: l.selected})
	case r.hovered && !r.loading && (r.host == nil || !r.host.dragging()):
		cv.DrawRRect(pill, paint.Paint{Color: l.hover})
	}
	if r.focusable && r.Focused() == core.FocusTarget(r) && r.FocusVisible() && !r.hasEditor {
		cv.DrawRRect(pill, paint.Paint{Stroke: &paint.Stroke{Width: treeRing, Color: l.ring}})
	}
	x := off.X + treeContentX(r.depth)
	mid := off.Y + sz.H/2
	switch {
	case r.loading && r.iconRoom:
		r.paintSpinner(cv, x+treeChevron+treeGap+treeIcon/2, mid)
	case r.loading:
		r.paintSpinner(cv, x+treeChevron/2, mid)
	case r.expandable:
		r.paintChevron(cv, x, mid-treeChevron/2)
	}
	for _, k := range r.kids {
		k.Paint(cv, off.Add(k.Offset()))
	}
	if r.hasEditor || r.laidOut == nil {
		return
	}
	labelX := off.X + treeLabelX(r.depth, r.hasIcon || r.iconRoom)
	room := off.X + sz.W - treeInset - treeLead - labelX
	ts := r.laidOut.Size()
	origin := geom.Offset{X: labelX, Y: off.Y + geom.Floor32((sz.H-ts.H)/2+0.5)}
	color := l.ink
	if r.loading {
		color = l.loading
	}
	clipped := ts.W > room
	if clipped {
		cv.Save()
		cv.ClipRect(geom.Rect{X: labelX, Y: off.Y, W: max(room, 0), H: sz.H})
	}
	runs := r.laidOut.Runs()
	for i := range runs {
		cv.DrawGlyphs(&runs[i], origin, color)
	}
	if clipped {
		cv.Restore()
	}
}

// paintChevron draws the chevron in the box at x, y.
func (r *renderTreeRow) paintChevron(cv *paint.Canvas, x, y float32) {
	p := r.chevrons[0]
	switch {
	case r.turning != nil:
		if r.turned != r.angle || r.turning.Empty() {
			turnedChevron(r.turning, r.angle)
			r.turned = r.angle
		}
		p = r.turning
	case r.angle >= 45:
		p = r.chevrons[1]
	}
	if p == nil {
		return
	}
	cv.Save()
	cv.Translate(x, y)
	cv.Scale(float32(treeChevron) / 24)
	cv.DrawPath(p, paint.Paint{Color: r.look.chevron})
	cv.Restore()
}

// paintSpinner draws the turning arc of a loading row around cx, cy.
func (r *renderTreeRow) paintSpinner(cv *paint.Canvas, cx, cy float32) {
	if r.arc == nil {
		return
	}
	angle := float32(0)
	if r.host != nil {
		angle = r.host.spinnerAngle()
	}
	cv.Save()
	cv.Translate(cx, cy)
	if angle != 0 {
		cv.Transform(geom.Rotation(angle * math.Pi / 180))
	}
	cv.Translate(-treeSpinnerSize/2, -treeSpinnerSize/2)
	cv.DrawPath(r.arc, paint.Paint{Color: r.look.loading})
	cv.Restore()
}

// HandlePointer implements core.PointerTarget. The press goes to the tree,
// and so does the drag that may follow.
func (r *renderTreeRow) HandlePointer(e *core.PointerEvent) {
	if r.host == nil || r.loading {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		r.menuPress = false
		if e.Button == input.ButtonSecondary {
			r.secondaryPress(e)
			return
		}
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		if r.host.rowPress(r.index, e.Pos, e) {
			// The tree moves the focus here; the press gives the keyboard to
			// the nearest object that takes it, which is now this row.
			r.focusable = true
		}
	case core.PointerUp:
		if r.menuPress {
			r.menuPress = false
			if sz := r.Size(); e.Pos.X >= 0 && e.Pos.Y >= 0 && e.Pos.X < sz.W && e.Pos.Y < sz.H {
				r.host.rowContextMenu(r.index, core.LocalToGlobal(r, e.Pos))
			}
			return
		}
		r.host.rowPointer(e.Kind, core.GlobalOffset(r).Add(e.Pos))
	case core.PointerMove, core.PointerCancel:
		if r.menuPress {
			r.menuPress = e.Kind == core.PointerMove
			return
		}
		r.host.rowPointer(e.Kind, core.GlobalOffset(r).Add(e.Pos))
	}
}

// secondaryPress selects the row and gives it the keyboard, as a desktop does
// before it shows a context menu. The tree keeps the press for OnContextMenu
// when it has one, and lets it go on to an overlay.ContextMenu around it
// otherwise.
func (r *renderTreeRow) secondaryPress(e *core.PointerEvent) {
	owns := r.host.rowSecondaryPress(r.index)
	r.focusable = true
	r.RequestFocus()
	if owns {
		e.Handled = true
		r.menuPress = true
		return
	}
	r.reclaim = true
	r.MarkNeedsLayout()
}

// HandleHover implements core.HoverTarget.
func (r *renderTreeRow) HandleHover(inside bool) {
	if r.hovered != inside {
		r.hovered = inside
		r.MarkNeedsPaint()
	}
}

// CanFocus implements core.FocusTarget. Only the row of the focus takes it,
// so that Tab stops once in the tree.
func (r *renderTreeRow) CanFocus() bool { return r.focusable && !r.loading }

// HandleKey implements core.FocusTarget.
func (r *renderTreeRow) HandleKey(e *core.KeyEvent) bool {
	if r.host == nil {
		return false
	}
	if kit.IsContextMenuKey(e) {
		// Below the label, as Explorer opens it.
		at := geom.Offset{X: treeLabelX(r.depth, r.hasIcon), Y: r.Size().H}
		return r.host.rowContextMenu(r.index, core.LocalToGlobal(r, at))
	}
	return r.host.rowKey(e)
}

// HandleText implements core.FocusTarget. Typed letters find a row.
func (r *renderTreeRow) HandleText(s string) {
	if r.host != nil {
		r.host.rowText(s)
	}
}

// HandleComposition implements core.FocusTarget.
func (r *renderTreeRow) HandleComposition(*input.Composition) {}

// FocusChanged implements core.FocusTarget.
func (r *renderTreeRow) FocusChanged(focused bool) {
	if focused && r.host != nil {
		r.host.rowFocused(r.index)
	}
	r.MarkNeedsPaint()
}

// FocusVisibleChanged implements core.FocusVisibleListener.
func (r *renderTreeRow) FocusVisibleChanged(bool) { r.MarkNeedsPaint() }

// Semantics implements core.RenderObject.
func (r *renderTreeRow) Semantics(sb *core.SemanticsBuilder) {
	n := semantics.Node{Role: semantics.RoleTreeItem, Label: r.label, Level: r.level, PosInSet: r.pos, SetSize: r.size}
	if !r.loading {
		n.Actions = semantics.ActionTap | semantics.ActionScrollIntoView
		if r.selected {
			n.State |= semantics.Selected
		}
		if r.expandable {
			if r.expanded {
				n.State |= semantics.Expanded
				n.Actions |= semantics.ActionCollapse
			} else {
				n.State |= semantics.Collapsed
				n.Actions |= semantics.ActionExpand
			}
		}
	}
	sb.Open(n, r.Size())
	for _, k := range r.kids {
		sb.Visit(k)
	}
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor.
func (r *renderTreeRow) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	return r.host != nil && !r.loading && r.host.rowAction(r.index, a)
}

// turnedChevron fills p with the chevron of chevronPath(true), which points
// right, turned clockwise by angle degrees about its middle, and returns p,
// made if nil.
func turnedChevron(p *paint.Path, angle float32) *paint.Path {
	if p == nil {
		p = &paint.Path{}
	}
	p.Reset()
	pts := [...]geom.Offset{{X: 15.4, Y: 6.4}, {X: 9.8, Y: 12}, {X: 15.4, Y: 17.6}, {X: 14, Y: 19}, {X: 7, Y: 12}, {X: 14, Y: 5}}
	sin, cos := math.Sincos(float64(angle) * math.Pi / 180)
	for i, pt := range pts {
		// Mirror to point right, then turn about the middle of the grid.
		x, y := float64(24-pt.X)-12, float64(pt.Y)-12
		q := geom.Offset{X: float32(x*cos-y*sin) + 12, Y: float32(x*sin+y*cos) + 12}
		if i == 0 {
			p.MoveTo(q.X, q.Y)
		} else {
			p.LineTo(q.X, q.Y)
		}
	}
	p.Close()
	return p
}
