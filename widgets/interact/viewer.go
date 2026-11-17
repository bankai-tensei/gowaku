package interact

import (
	"math"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// defaultMinScale and defaultMaxScale bound the scale of a viewer whose
	// MinScale or MaxScale is zero.
	defaultMinScale = 0.1
	defaultMaxScale = 10
	// viewerKeyStep is how far an arrow key moves the view.
	viewerKeyStep = 40
	// viewerKeyZoom is the change of scale of a key press or a screen reader's
	// increase and decrease.
	viewerKeyZoom = 1.25
	// viewerDoubleTapZoom is how much a double click zooms in.
	viewerDoubleTapZoom = 2
)

// ViewerBoundary says how far the child of an InteractiveViewer may move.
//
// It is ViewerBounded (the zero value) or ViewerUnbounded.
type ViewerBoundary uint8

// The boundaries.
const (
	// ViewerBounded keeps the child, grown by BoundaryMargin on each side,
	// covering the viewer, so that it cannot be dragged out of sight. Along an
	// axis where it is smaller than the viewer, it stays centered.
	ViewerBounded ViewerBoundary = iota
	// ViewerUnbounded lets the child go anywhere, as the canvas of a node
	// editor does.
	ViewerUnbounded
)

// String returns the name of the boundary.
func (b ViewerBoundary) String() string {
	if b == ViewerUnbounded {
		return "ViewerUnbounded"
	}
	return "ViewerBounded"
}

// ViewerKeys says which keys an InteractiveViewer takes while it has the focus,
// and whether it takes the focus at all.
//
// It is ViewerKeysAll (the zero value), ViewerKeysZoom or ViewerKeysNone.
type ViewerKeys uint8

// The key sets.
const (
	// ViewerKeysAll takes the focus by Tab and by a press, pans with the arrow
	// keys while there is room to pan, and zooms with + and - and resets with
	// 0. An arrow key that would not move the view, as when the whole child
	// fits, goes on to the shortcuts of the app.
	ViewerKeysAll ViewerKeys = iota
	// ViewerKeysZoom is ViewerKeysAll without the arrow keys, which always go
	// on to the app: an image viewer whose Left and Right show the previous
	// and the next image.
	ViewerKeysZoom
	// ViewerKeysNone takes no keys and never takes the focus: Tab passes it
	// by, and a press on it leaves the focus where it was. A screen reader can
	// still zoom it.
	ViewerKeysNone
)

// String returns the name of the key set.
func (k ViewerKeys) String() string {
	switch k {
	case ViewerKeysZoom:
		return "ViewerKeysZoom"
	case ViewerKeysNone:
		return "ViewerKeysNone"
	}
	return "ViewerKeysAll"
}

// InteractiveViewer shows its child through a pan and a zoom: the base of an
// image viewer, a drawing or a node editor. See
// docs/ADR/0092-interactive-viewer.md and
// docs/ADR/0130-layout-builder-aspect-ratio-viewer-keys.md.
//
// Dragging the background with the primary or middle button pans, and so
// does a finger. The wheel pans, vertically or, with Shift, sideways, as a
// touchpad's two fingers do; with WheelZooms it zooms instead, as image
// viewers do. A pinch, and Ctrl with the wheel, zoom about the pointer, so that
// the point under it stays put. A fling glides on and slows down, unless
// MediaQuery asks to reduce motion. With the focus, the arrow keys pan, + and -
// zoom about the middle and 0 goes back to the start; Keys narrows that down.
//
// The child is laid out once and painted through the transform: panning and
// zooming neither rebuild nor lay it out again. The pointer reaches the
// widgets inside where they show, so buttons and fields in the child keep
// working, and a press on them does not pan. On a touch screen, a finger that
// pressed a button and moves on pans, as in a scrolling list. A Listener in the
// child takes the presses over it as well, so dragging there does not pan; its
// OnPointer can clear e.Handled on a PointerDown to let the press pan instead.
//
//	var view interact.ViewerController // in the State
//	interact.InteractiveViewer{Controller: &s.view, MaxScale: 8, Child: drawing}
//
// An image viewer zooms with the wheel and leaves Left and Right to the app:
//
//	interact.InteractiveViewer{
//		Controller: &s.view, Constrained: true, WheelZooms: true, Keys: interact.ViewerKeysZoom,
//		Child: basic.Image{Source: photo},
//	}
//
// The viewer takes all the room it is given, so its parent must bound both of
// its lengths.
type InteractiveViewer struct {
	core.StatefulBase
	// Controller reads and moves the view from outside. Without one the viewer
	// keeps its own.
	Controller *ViewerController
	// MinScale and MaxScale bound the zoom. Zero means 0.1 and 10.
	MinScale, MaxScale float32
	// Boundary says whether the child may leave the viewer.
	Boundary ViewerBoundary
	// BoundaryMargin is the room, in logical pixels on screen, that a bounded
	// child may be pulled past each of its edges. Fit leaves it around the
	// child too.
	BoundaryMargin float32
	// Constrained lays the child out within the size of the viewer, as an
	// image that should fill it. Otherwise the child gets unbounded room and
	// takes the size it wants, as a large drawing does.
	Constrained bool
	// DoubleTapZoom zooms in twice about a double click or a double tap, and
	// back to a scale of 1 when zoomed in. Node editors leave it off, as they
	// give double clicks a meaning of their own.
	DoubleTapZoom bool
	// WheelZooms makes the wheel zoom about the pointer, as Ctrl with the
	// wheel does, instead of panning. Shift with the wheel, and a sideways
	// scroll, still pan, and so does a finger on a touch screen.
	WheelZooms bool
	// Keys says which keys the viewer takes, and whether it takes the focus.
	Keys ViewerKeys
	// Label names the viewer to assistive technology.
	Label string
	// OnChange reports every change of the view, by the user, a motion or the
	// controller: the scale, and where the top left of the child lies in the
	// viewer.
	OnChange func(scale float32, offset geom.Offset)
	Child    core.Widget
}

// CreateState implements core.StatefulWidget.
func (InteractiveViewer) CreateState() core.State { return &viewerState{} }

// viewerState runs a viewer: it moves the transform that its controller holds,
// and the render object paints and hit tests through it.
type viewerState struct {
	core.StateOf[InteractiveViewer]
	own    ViewerController
	ctl    *ViewerController
	ro     *renderViewer
	ticker *core.Ticker
	motion viewerMotion
	reduce bool
	// focusShown says that the focus ring shows.
	focusShown bool

	// The drag in progress: whether there is one, where the pointer was, and
	// its recent places for the speed of a fling.
	dragging bool
	last     geom.Offset
	samples  velocityTracker

	// view and content are the sizes of the viewer and of its child after the
	// last layout, which laidOut says has happened.
	view, content geom.Size
	laidOut       bool
	// deferNotify holds OnChange back until the next tick, during a build or
	// a layout, where the callback must not set state; notifyPending says one
	// is owed.
	deferNotify   bool
	notifyPending bool
}

// InitState implements core.State.
func (s *viewerState) InitState() {
	s.attach(s.wantController())
	s.ticker = s.NewTicker(s.tick)
}

// DidUpdateWidget implements core.State.
func (s *viewerState) DidUpdateWidget(core.Widget) {
	if want := s.wantController(); want != s.ctl {
		if s.ctl.state == s {
			s.ctl.state = nil
		}
		s.attach(want)
		s.stop()
		if s.ro != nil {
			s.ro.MarkNeedsPaint()
		}
	}
	s.settle()
}

// Dispose implements core.State.
func (s *viewerState) Dispose() {
	if s.ctl.state == s {
		s.ctl.state = nil
	}
}

// Build implements core.State.
func (s *viewerState) Build(ctx core.BuildContext) core.Widget {
	s.reduce = basic.ReduceMotionOf(ctx)
	w := s.Widget()
	s.limits()
	return FocusRing{Visible: s.focusShown, Child: viewerBox{s: s, constrained: w.Constrained, child: w.Child}}
}

func (s *viewerState) wantController() *ViewerController {
	if c := s.Widget().Controller; c != nil {
		return c
	}
	return &s.own
}

// attach makes c the controller of the viewer. The last viewer to take a
// controller keeps it.
func (s *viewerState) attach(c *ViewerController) {
	s.ctl = c
	c.state = s
}

// limits returns the bounds of the scale, and panics if they make no sense.
func (s *viewerState) limits() (lo, hi float32) {
	w := s.Widget()
	lo, hi = w.MinScale, w.MaxScale
	if lo == 0 {
		lo = defaultMinScale
	}
	if hi == 0 {
		hi = defaultMaxScale
	}
	if lo < 0 || hi < lo {
		panic("gowaku: InteractiveViewer needs 0 < MinScale <= MaxScale")
	}
	return lo, hi
}

func (s *viewerState) clampScale(scale float32) float32 {
	lo, hi := s.limits()
	return geom.Clamp32(scale, lo, hi)
}

// clampOffset keeps a bounded child at scale in sight.
func (s *viewerState) clampOffset(scale float32, off geom.Offset) geom.Offset {
	w := s.Widget()
	if w.Boundary == ViewerUnbounded || !s.laidOut {
		return off
	}
	m := w.BoundaryMargin
	off.X = clampAxis(off.X, s.view.W, s.content.W*scale, m)
	off.Y = clampAxis(off.Y, s.view.H, s.content.H*scale, m)
	return off
}

// clampAxis keeps the offset o of a child extent long, with margin m on each
// side, covering a view: centered when it is shorter.
func clampAxis(o, view, extent, m float32) float32 {
	if extent+2*m <= view {
		return (view - extent) / 2
	}
	return geom.Clamp32(o, view-extent-m, m)
}

// laidOutAt records the sizes of a layout, carries out what the controller
// asked for before there was one, and keeps the child in its bounds.
func (s *viewerState) laidOutAt(view, content geom.Size) {
	s.view, s.content, s.laidOut = view, content, true
	s.settle()
}

// settle carries out a request the controller kept and clamps the transform
// to the current limits, holding OnChange back to the next tick.
func (s *viewerState) settle() {
	if !s.laidOut {
		return
	}
	s.deferNotify = true
	c := s.ctl
	switch req := c.pending; req.kind {
	case requestFit:
		c.pending = viewerRequest{}
		s.fit(false)
	case requestRect:
		c.pending = viewerRequest{}
		s.zoomToRect(req.rect, false)
	}
	s.set(s.clampScale(c.Scale()), c.offset)
	s.deferNotify = false
}

// set moves the view to scale and off, kept in bounds, and reports whether
// it moved.
func (s *viewerState) set(scale float32, off geom.Offset) bool {
	off = s.clampOffset(scale, off)
	c := s.ctl
	if c.Scale() == scale && c.offset == off {
		return false
	}
	c.scale, c.offset = scale, off
	if s.ro != nil {
		s.ro.MarkNeedsPaint()
	}
	s.changed()
	return true
}

func (s *viewerState) changed() {
	if s.deferNotify {
		s.notifyPending = true
		s.ticker.Start()
		return
	}
	s.notify()
}

func (s *viewerState) notify() {
	if f := s.Widget().OnChange; f != nil {
		f(s.ctl.Scale(), s.ctl.offset)
	}
}

// tick runs a frame of the motion, and a held back OnChange.
func (s *viewerState) tick(now time.Duration) {
	if s.notifyPending {
		s.notifyPending = false
		s.notify()
	}
	if s.motion.kind == motionNone {
		s.ticker.Stop()
		return
	}
	scale, off, more := s.motion.at(now, s.view, s.ctl.Scale(), s.ctl.offset)
	s.set(scale, off)
	if s.motion.kind == motionFling {
		// A fling stops along an axis at the edge it runs into.
		got := s.ctl.offset
		if got.X != off.X {
			s.motion.xDone = true
		}
		if got.Y != off.Y {
			s.motion.yDone = true
		}
		more = more && (!s.motion.xDone || !s.motion.yDone)
	}
	if !more {
		s.motion.kind = motionNone
		s.ticker.Stop()
	}
}

// stop ends any motion where it is.
func (s *viewerState) stop() {
	s.motion.kind = motionNone
	if !s.notifyPending {
		s.ticker.Stop()
	}
}

// moveTo goes to scale and off, gliding there if animate is set and motion is
// not reduced.
func (s *viewerState) moveTo(scale float32, off geom.Offset, animate bool) {
	s.stop()
	scale = s.clampScale(scale)
	off = s.clampOffset(scale, off)
	if !animate || s.reduce || !s.laidOut {
		s.set(scale, off)
		return
	}
	from := s.ctl
	if from.Scale() == scale && from.offset == off {
		return
	}
	s.motion.tween(s.view, from.Scale(), from.offset, scale, off)
	s.ticker.Start()
}

// zoomAt scales the view by factor about focus, a point of the viewer, which
// stays where it is unless a limit stops it.
func (s *viewerState) zoomAt(factor float32, focus geom.Offset, animate bool) {
	s0, o0 := s.ctl.Scale(), s.ctl.offset
	s1 := s.clampScale(s0 * factor)
	p := geom.Offset{X: (focus.X - o0.X) / s0, Y: (focus.Y - o0.Y) / s0}
	s.moveTo(s1, geom.Offset{X: focus.X - p.X*s1, Y: focus.Y - p.Y*s1}, animate)
}

// zoomCenter zooms about the middle of the viewer.
func (s *viewerState) zoomCenter(factor float32) {
	s.zoomAt(factor, geom.Offset{X: s.view.W / 2, Y: s.view.H / 2}, true)
}

// fit shows the whole child, centered, with the margin around it.
func (s *viewerState) fit(animate bool) {
	if s.content.W <= 0 || s.content.H <= 0 {
		return
	}
	m := s.Widget().BoundaryMargin
	scale := s.clampScale(min((s.view.W-2*m)/s.content.W, (s.view.H-2*m)/s.content.H))
	off := geom.Offset{X: (s.view.W - s.content.W*scale) / 2, Y: (s.view.H - s.content.H*scale) / 2}
	s.moveTo(scale, off, animate)
}

// zoomToRect shows r, a rectangle of the child, as large as fits, centered.
func (s *viewerState) zoomToRect(r geom.Rect, animate bool) {
	scale := s.clampScale(min(s.view.W/r.W, s.view.H/r.H))
	off := geom.Offset{X: s.view.W/2 - (r.X+r.W/2)*scale, Y: s.view.H/2 - (r.Y+r.H/2)*scale}
	s.moveTo(scale, off, animate)
}

// panNow moves the view by d at once and reports whether it moved.
func (s *viewerState) panNow(d geom.Offset) bool {
	return s.set(s.ctl.Scale(), s.ctl.offset.Add(d))
}

// glideBy moves the view by d, from the end of a glide under way, and reports
// whether there is anywhere to go.
func (s *viewerState) glideBy(d geom.Offset) bool {
	scale, cur := s.ctl.Scale(), s.ctl.offset
	base := cur
	gliding := s.motion.kind == motionGlide
	if gliding {
		base = s.motion.to
	}
	target := s.clampOffset(scale, base.Add(d))
	if target == base {
		return gliding
	}
	if s.reduce {
		s.stop()
		return s.set(scale, target)
	}
	if !gliding {
		s.stop()
	}
	s.motion.glide(cur, target)
	s.ticker.Start()
	return true
}

// fling glides on from the end of a drag at velocity, in pixels per second.
func (s *viewerState) fling(v geom.Offset) {
	speed := v.Length()
	if s.reduce || speed < minFlingSpeed {
		return
	}
	if speed > maxFlingSpeed {
		v = geom.Offset{X: v.X * maxFlingSpeed / speed, Y: v.Y * maxFlingSpeed / speed}
	}
	s.motion.fling(s.ctl.offset, v)
	s.ticker.Start()
}

// pointer handles what the pointer does over the background of the child.
func (s *viewerState) pointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary && e.Button != input.ButtonMiddle {
			return
		}
		e.Handled = true
		s.stop()
		s.dragging = false
		if e.Clicks == 2 && e.Button == input.ButtonPrimary && s.Widget().DoubleTapZoom {
			s.doubleTap(e.Pos)
			return
		}
		s.dragging, s.last = true, e.Pos
		s.samples.reset()
		s.samples.add(e.Pos, e.Time)
	case core.PointerMove:
		if !s.dragging {
			return
		}
		d := e.Pos.Sub(s.last)
		s.last = e.Pos
		s.samples.add(e.Pos, e.Time)
		s.panNow(d)
	case core.PointerUp:
		if !s.dragging {
			return
		}
		s.dragging = false
		s.samples.add(e.Pos, e.Time)
		s.fling(s.samples.velocity(e.Time))
	case core.PointerCancel:
		s.dragging = false
	case core.PointerScroll:
		s.scroll(e)
	case core.PointerPinch:
		if e.Pinch > 0 {
			e.Handled = true
			s.stop()
			s.zoomAt(e.Pinch, e.Pos, false)
		}
	}
}

// scroll pans by the wheel, a touchpad, or a finger that pressed a widget in
// the child and moved on. A bounded viewer that cannot move passes the wheel
// on to the scrolling view around it.
func (s *viewerState) scroll(e *core.PointerEvent) {
	if s.Widget().WheelZooms && e.Device != input.DeviceTouch && e.Scroll.X == 0 && e.Scroll.Y != 0 &&
		e.Modifiers&input.ModShift == 0 {
		e.Handled = true
		s.stop()
		s.zoomAt(wheelZoomFactor(e.Scroll.Y), e.Pos, false)
		return
	}
	d := geom.Offset{X: -e.Scroll.X, Y: e.Scroll.Y}
	if e.Scroll.X == 0 && e.Modifiers&input.ModShift != 0 {
		d = geom.Offset{X: e.Scroll.Y}
	}
	d = geom.Offset{X: d.X * kit.NotchDistance, Y: d.Y * kit.NotchDistance}
	var moved bool
	if e.Device == input.DeviceTouch || !wholeNotches(e.Scroll) {
		// A finger or a touchpad sends a stream of small steps, which is a
		// motion already; a wheel that clicks glides, as in Scroll.
		s.stop()
		moved = s.panNow(d)
	} else {
		moved = s.glideBy(d)
	}
	if moved || s.Widget().Boundary == ViewerUnbounded {
		e.Handled = true
	}
}

// wheelZoomFactor is the zoom of a turn of the wheel by notches: that of Ctrl
// with the wheel, which core turns into a pinch.
func wheelZoomFactor(notches float32) float32 {
	return float32(math.Pow(1.1, float64(notches)))
}

func wholeNotches(o geom.Offset) bool {
	return o.X == geom.Floor32(o.X) && o.Y == geom.Floor32(o.Y)
}

// doubleTap zooms in about pos, or back to a scale of 1.
func (s *viewerState) doubleTap(pos geom.Offset) {
	if s.ctl.Scale() > 1.01 {
		s.moveTo(1, s.zoomedOffset(1, pos), true)
		return
	}
	s.zoomAt(viewerDoubleTapZoom, pos, true)
}

// zoomedOffset returns the offset at scale that keeps focus in place.
func (s *viewerState) zoomedOffset(scale float32, focus geom.Offset) geom.Offset {
	s0, o0 := s.ctl.Scale(), s.ctl.offset
	return geom.Offset{X: focus.X - (focus.X-o0.X)/s0*scale, Y: focus.Y - (focus.Y-o0.Y)/s0*scale}
}

// key pans with the arrows, and zooms with Ctrl and +, - or 0. The keys
// without Ctrl arrive as text; see text. An arrow that would not move the view
// is not taken, so that it reaches the shortcuts of the app.
func (s *viewerState) key(e *core.KeyEvent) bool {
	keys := s.Widget().Keys
	if keys == ViewerKeysNone {
		return false
	}
	var d geom.Offset
	switch e.Key {
	case input.KeyLeft:
		d.X = viewerKeyStep
	case input.KeyRight:
		d.X = -viewerKeyStep
	case input.KeyUp:
		d.Y = viewerKeyStep
	case input.KeyDown:
		d.Y = -viewerKeyStep
	}
	if d != (geom.Offset{}) {
		return keys == ViewerKeysAll && s.glideBy(d)
	}
	switch e.Key {
	case input.KeyEqual, input.KeyPlus, input.KeySemicolon, input.KeyMinus, input.Key0:
		if e.Modifiers&input.ModControl == 0 {
			return false
		}
		s.zoomKey(e.Key)
	default:
		return false
	}
	return true
}

// text zooms with + and -, and resets with 0. Typed text, unlike key codes,
// finds + on every keyboard layout and on the numeric keypad.
func (s *viewerState) text(t string) {
	if s.Widget().Keys == ViewerKeysNone {
		return
	}
	switch t {
	case "+", "=":
		s.zoomKey(input.KeyEqual)
	case "-":
		s.zoomKey(input.KeyMinus)
	case "0":
		s.zoomKey(input.Key0)
	}
}

// zoomKey zooms in for the keys that type + or =: KeyEqual on US layouts,
// KeySemicolon for the ;+ key of JIS layouts (which have no = key of its
// own), and KeyPlus for the keypad and layouts such as German, as Firefox
// does. On a US layout Ctrl+; zooms in too.
func (s *viewerState) zoomKey(k input.KeyCode) {
	switch k {
	case input.KeyEqual, input.KeyPlus, input.KeySemicolon:
		s.zoomCenter(viewerKeyZoom)
	case input.KeyMinus:
		s.zoomCenter(1 / viewerKeyZoom)
	default:
		s.moveTo(1, geom.Offset{}, true)
	}
}

// showFocus shows or hides the focus ring.
func (s *viewerState) showFocus(show bool) {
	if s.focusShown != show {
		s.focusShown = show
		kit.Rebuild(&s.StateBase)
	}
}
