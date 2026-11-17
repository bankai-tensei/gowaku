package overlay

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The timing and measures of a sheet.
const (
	sheetEnter = 280 * time.Millisecond
	sheetExit  = 160 * time.Millisecond
	// sheetMaxWidth is the widest a bottom sheet gets, and sheetWidth the
	// width of a side sheet without a Size.
	sheetMaxWidth = 640
	sheetWidth    = 320
	// sheetMaxShare is the most of the window a sheet covers across its edge.
	sheetMaxShare = 0.9
	sheetPadding  = theme.Space24
)

// SheetEdge is the edge of the window a sheet comes from.
//
// It is SheetBottom (the zero value), SheetTrailing or SheetLeading.
type SheetEdge uint8

// The edges.
const (
	// SheetBottom rises from the bottom, centered and at most 640 wide, as
	// tall as its content.
	SheetBottom SheetEdge = iota
	// SheetTrailing slides in from the right, the full height of the window.
	SheetTrailing
	// SheetLeading slides in from the left, the full height of the window.
	SheetLeading
)

// String returns the name of the edge.
func (e SheetEdge) String() string {
	switch e {
	case SheetBottom:
		return "SheetBottom"
	case SheetTrailing:
		return "SheetTrailing"
	case SheetLeading:
		return "SheetLeading"
	}
	return "unknown"
}

// Sheet says how ShowSheet shows a panel.
type Sheet struct {
	Edge SheetEdge
	// Size is the height of a bottom sheet or the width of a side sheet, at
	// most nine tenths of the window. Zero fits a bottom sheet to its content
	// and makes a side sheet 320 wide.
	Size float32
	// Label names the sheet for assistive technology, as a title does.
	Label string
	// Persistent keeps the sheet open on a click beside it. Escape and the
	// dismiss function still close it.
	Persistent bool
	// OnDismissRequest, if set, runs instead of closing the sheet when Escape
	// or a click beside it asks to close it, as Dialog.OnDismissRequest does;
	// a Persistent sheet still ignores the click. The dismiss function always
	// closes the sheet.
	OnDismissRequest func()
}

// ShowSheet opens a modal panel from an edge of the window, over a scrim, with
// what build returns inside, padded by 24. Call the function that build gets to
// close it; Escape and a click beside the panel do the same unless the sheet is
// Persistent or has an OnDismissRequest. The panel slides in and out, or fades
// when the window asks for less motion, keeps Tab to itself, and gives the
// focus back when it is gone.
// While it is open, what lies under it is out of the accessibility tree, as
// for ShowDialog.
//
// The sheet takes the theme, MediaQuery and Localizations above ctx, as
// ShowDialog does. build runs again whenever the overlay rebuilds or they
// change. It must not be called during a Build.
//
//	ShowSheet(ctx, Sheet{Edge: SheetTrailing, Label: "詳細"}, func(dismiss func()) core.Widget {
//		return Column{Children: []core.Widget{info, Button{Label: "閉じる", OnPress: dismiss}}}
//	})
func ShowSheet(ctx core.BuildContext, s Sheet, build func(dismiss func()) core.Widget) {
	var entry *Entry
	entry = Of(ctx).InsertFrom(ctx, func() core.Widget {
		return sheetRoute{sheet: s, build: build, onGone: func() { entry.Remove() }}
	})
}

// sheetRoute is the scrim and transition around a sheet.
type sheetRoute struct {
	core.StatefulBase
	sheet  Sheet
	build  func(dismiss func()) core.Widget
	onGone func()
}

type sheetRouteState struct {
	core.StateBase
	controller motion.AnimationController
	// previous had the focus before the sheet.
	previous core.FocusTarget
}

// CreateState implements core.StatefulWidget.
func (sheetRoute) CreateState() core.State { return &sheetRouteState{} }

// InitState implements core.State.
func (s *sheetRouteState) InitState() {
	c := &s.controller
	c.Duration, c.ReverseDuration = sheetEnter, sheetExit
	c.Curve, c.ReverseCurve = motion.SpringOut, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { kit.Rebuild(&s.StateBase) }
	c.OnStatus = func(status motion.Status) {
		if status == motion.Dismissed {
			s.gone()
		}
	}
	c.Forward()
}

// Dispose implements core.State.
func (s *sheetRouteState) Dispose() { s.controller.Dispose() }

func (s *sheetRouteState) dismiss() {
	if s.controller.Status() != motion.Reversing {
		s.controller.Reverse()
	}
}

func (s *sheetRouteState) scrimTap() {
	if !s.Widget().(sheetRoute).sheet.Persistent {
		s.requestDismiss()
	}
}

// requestDismiss is Escape or a click beside the sheet: it closes the sheet,
// or runs its OnDismissRequest.
func (s *sheetRouteState) requestDismiss() {
	if s.controller.Status() == motion.Reversing {
		return
	}
	if f := s.Widget().(sheetRoute).sheet.OnDismissRequest; f != nil {
		f()
		return
	}
	s.dismiss()
}

func (s *sheetRouteState) gone() {
	if s.previous != nil {
		core.RestoreFocus(s.previous)
	}
	s.Widget().(sheetRoute).onGone()
}

func (s *sheetRouteState) key(e *core.KeyEvent) bool {
	if e.Key != input.KeyEscape {
		return false
	}
	if !e.Repeat {
		s.requestDismiss()
	}
	return true
}

// Build implements core.State.
func (s *sheetRouteState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	route := s.Widget().(sheetRoute)
	sh := route.sheet
	v := s.controller.Value()
	fade := geom.Clamp32(v, 0, 1)
	var opacity float32 = 1
	if basic.ReduceMotionOf(s.Context()) {
		// The sheet fades in where it rests.
		v, opacity = 1, fade
		if s.controller.Status() != motion.Reversing {
			// As for a dialog: an Opacity of 0 would take the keyboard from a
			// field that took it on the first frame.
			opacity = max(opacity, enteringOpacity)
		}
	}
	radius := th.Radius.Dialog
	// The corners on the edge of the window hang outside it, so that only the
	// exposed ones show rounded.
	pad := geom.All(sheetPadding)
	switch sh.Edge {
	case SheetTrailing:
		pad.Right += radius
	case SheetLeading:
		pad.Left += radius
	default:
		pad.Bottom += radius
	}
	scrim := th.Color.Scrim
	card := basic.Container{
		Color:   th.Color.Raised,
		Radius:  radius,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: pad,
		Child:   route.build(s.dismiss),
	}
	return kit.ModalScope{
		Active:         s.controller.Status() != motion.Reversing,
		BlockSemantics: true,
		Child: interact.Focus{
			Enabled:     true,
			Autofocus:   true,
			OnAutofocus: func(previous core.FocusTarget) { s.previous = previous },
			OnKey:       s.key,
			Child: interact.GestureDetector{OnTap: s.scrimTap, Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
				basic.Container{Color: scrim.WithAlpha(scrim.A * fade)},
				sheetSlot{edge: sh.Edge, size: sh.Size, overhang: radius, progress: v, child: basic.Opacity{
					Opacity: opacity,
					// A press on the sheet must not reach the scrim, or take the
					// keyboard away.
					Child: interact.GestureDetector{OnTapDown: func() {}, Child: interact.Semantics{
						Role: semantics.RoleDialog, Label: sh.Label, Child: card,
					}},
				}},
			}}},
		},
	}
}

// sheetSlot is the widget of renderSheetSlot.
type sheetSlot struct {
	edge           SheetEdge
	size, overhang float32
	progress       float32
	child          core.Widget
}

// Key implements core.Widget.
func (sheetSlot) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p sheetSlot) CreateRenderObject() core.RenderObject {
	r := &renderSheetSlot{}
	p.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p sheetSlot) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderSheetSlot)
	if r.edge != p.edge || r.size != p.size || r.overhang != p.overhang || r.progress != p.progress {
		r.edge, r.size, r.overhang, r.progress = p.edge, p.size, p.overhang, p.progress
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p sheetSlot) ChildWidget() core.Widget { return p.child }

// renderSheetSlot fills the window and puts its child against an edge, pulled
// in by progress from just outside it. The child reaches overhang past the
// edge, so that its corners there are hidden: the slot cuts it to its box,
// the area of the overlay, rather than leave that to the edge of the window. Only the child takes the
// pointer; the scrim under it takes the rest.
type renderSheetSlot struct {
	layout.RenderProxy
	edge           SheetEdge
	size, overhang float32
	progress       float32
}

// Layout implements core.RenderObject.
func (r *renderSheetSlot) Layout(c layout.Constraints) geom.Size {
	sz := c.Biggest()
	child := r.ChildAt(0)
	if child == nil {
		return sz
	}
	over := r.overhang
	var off geom.Offset
	switch r.edge {
	case SheetTrailing, SheetLeading:
		w := r.size
		if w <= 0 {
			w = sheetWidth
		}
		w = min(w, sz.W*sheetMaxShare)
		csz := core.LayoutChild(child, geom.Tight(geom.Size{W: w + over, H: sz.H}))
		shown := (csz.W - over) * r.progress
		off.X = sz.W - shown
		if r.edge == SheetLeading {
			off.X = shown - csz.W
		}
	default:
		w := min(sz.W, sheetMaxWidth)
		most := sz.H*sheetMaxShare + over
		cc := geom.Constraints{MinW: w, MaxW: w, MaxH: most}
		if r.size > 0 {
			h := min(r.size, sz.H*sheetMaxShare) + over
			cc.MinH, cc.MaxH = h, h
		}
		csz := core.LayoutChild(child, cc)
		off.X = geom.Floor32((sz.W-csz.W)/2 + 0.5)
		off.Y = sz.H - (csz.H-over)*r.progress
	}
	child.SetOffset(off)
	return sz
}

// ChildClip implements core.Clipper.
func (r *renderSheetSlot) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), true
}

// Paint implements core.RenderObject.
func (r *renderSheetSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	child := r.ChildAt(0)
	if child == nil {
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	child.Paint(cv, off.Add(child.Offset()))
	cv.Restore()
}

// HitTest implements core.RenderObject.
func (r *renderSheetSlot) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	child := r.ChildAt(0)
	return child != nil && child.HitTest(pos.Sub(child.Offset()), res)
}
