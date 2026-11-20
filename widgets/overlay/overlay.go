package overlay

import (
	"slices"
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

const (
	dialogExit = 120 * time.Millisecond
	// dialogStartScale is how small a dialog starts.
	dialogStartScale = 0.96
	dialogPadding    = theme.Space24
	dialogMinWidth   = 280
	dialogMaxWidth   = 560
	// dialogWidth is what a dialog takes without a Width, since layout has no way
	// to ask its content for a natural width.
	dialogWidth = 340
	// dialogGlassBlur is the standard deviation of the blur behind a glass
	// dialog, in logical pixels.
	dialogGlassBlur = 20
)

// Overlay shows entries on top of its child. Run puts one around the root of each
// window; use this widget for a part that needs its own.
type Overlay struct {
	core.StatefulBase
	Child core.Widget
}

// Entry is something shown in an Overlay.
type Entry struct {
	overlay *Host
	build   func() core.Widget
	// from is the place the entry was opened from, which gives it its theme,
	// or the zero BuildContext for an entry that takes those of the overlay.
	from core.BuildContext
}

// Host is what Of returns: the state of the nearest Overlay.
type Host struct {
	core.StateBase
	entries []*Entry
	// tooltips is what the tooltips of the overlay share, and toasts the stack
	// of toasts, nil until the first.
	tooltips tooltipWarmth
	toasts   *toastHost
}

// CreateState implements core.StatefulWidget.
func (Overlay) CreateState() core.State { return &Host{} }

// Of returns the nearest Overlay above ctx. It panics if there is none,
// which cannot happen below Run.
func Of(ctx core.BuildContext) *Host {
	o, ok := core.Lookup[*Host](ctx)
	if !ok {
		panic("gowaku: overlay.Of found no Overlay; wrap the root in overlay.Overlay")
	}
	return o
}

// Insert shows what build returns on top of everything else. It must not be
// called during a Build.
func (s *Host) Insert(build func() core.Widget) *Entry {
	e := &Entry{overlay: s, build: build}
	s.SetState(func() { s.entries = append(s.entries, e) })
	return e
}

// InsertFrom is Insert for something that belongs to the widget of ctx, as a
// popup or a dialog does: what build returns gets the theme, the MediaQuery
// and the Localizations above ctx instead of those above the overlay, which
// may sit above a basic.Theme of the part of the tree that opened it. The
// entry follows them while it shows, and keeps what was last above ctx if
// that widget leaves the tree first. It must not be called during a Build.
// See docs/ADR/0067-runtime-theme.md.
func (s *Host) InsertFrom(ctx core.BuildContext, build func() core.Widget) *Entry {
	e := &Entry{overlay: s, build: build, from: ctx}
	s.SetState(func() { s.entries = append(s.entries, e) })
	return e
}

// Remove takes the entry away. It must not be called during a Build.
func (e *Entry) Remove() {
	s := e.overlay
	if i := slices.Index(s.entries, e); i >= 0 {
		s.SetState(func() { s.entries = slices.Delete(s.entries, i, i+1) })
	}
}

// Build implements core.State.
func (s *Host) Build(core.BuildContext) core.Widget {
	children := make([]core.Widget, 0, len(s.entries)+1)
	children = append(children, s.Widget().(Overlay).Child)
	for _, e := range s.entries {
		children = append(children, core.Keyed(e, overlayEntry{entry: e}))
	}
	return core.Inherited[*Host]{Value: s, Child: basic.Stack{Fit: layout.StackExpand, Children: children}}
}

// overlayEntry shows one entry.
type overlayEntry struct {
	core.StatelessBase
	entry *Entry
}

// Build implements core.StatelessWidget.
func (o overlayEntry) Build(core.BuildContext) core.Widget {
	child := o.entry.build()
	if o.entry.from != (core.BuildContext{}) {
		child = callerScope{at: o.entry.from, child: child}
	}
	return entryHits{entry: o.entry, child: basic.RepaintBoundary{Child: child}}
}

// entryHits is the widget of renderEntryHits.
type entryHits struct {
	entry *Entry
	child core.Widget
}

// Key implements core.Widget.
func (entryHits) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (e entryHits) CreateRenderObject() core.RenderObject { return &renderEntryHits{entry: e.entry} }

// UpdateRenderObject implements core.RenderWidget.
func (e entryHits) UpdateRenderObject(ro core.RenderObject) { ro.(*renderEntryHits).entry = e.entry }

// ChildWidget implements core.SingleChildWidget.
func (e entryHits) ChildWidget() core.Widget { return e.child }

// renderEntryHits lets the pointer through an entry where nothing in it takes
// it, as around a tooltip or a toast. The repaint boundary of the entry fills
// the overlay and would otherwise take every press for itself.
//
// It is also the top of what the entry shows, where Contains goes on from the
// place the entry was opened from, and it tells the EntryFocusListeners above
// that place when the focus leaves the entry.
type renderEntryHits struct {
	layout.RenderProxy
	entry *Entry
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderEntryHits) FocusWithinChanged(within bool) {
	if !within {
		entryFocusLeft(r)
	}
}

// HitTest implements core.RenderObject.
func (r *renderEntryHits) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	child := r.ChildAt(0)
	n := len(res.Path)
	if child == nil || !child.HitTest(pos.Sub(child.Offset()), res) {
		return false
	}
	if len(res.Path) == n+1 {
		// Only the boundary itself.
		res.Path = res.Path[:n]
		return false
	}
	res.Path = append(res.Path, r)
	return true
}

// ShowDialog opens a modal dialog in the nearest Overlay, with what build returns
// inside, usually a Dialog, and returns a function that closes it. build gets
// the same function. Call it to close the dialog from a button in it or from
// elsewhere, as when the work it waits for ends; once the dialog is closing or
// gone, calling it again does nothing. Escape and a click beside the dialog
// close it too, unless the OnDismissRequest of the Dialog returns false. The
// dialog fades and scales in and out, keeps Tab to itself, and gives the focus
// back when it is gone.
//
// Closing it starts a fade of about 120 ms, a timer of the clock, and the
// dialog leaves the overlay only when that ends. In a gowakutest.Tester, a tap
// that closes it settles through the fade; a dismiss from an After, or from
// code the test calls itself, needs tt.Advance(200 * time.Millisecond), or a
// Settle, before the test checks that the dialog is gone.
//
// While the dialog is open, what lies under it is out of the accessibility
// tree, as aria-modal hides it: screen readers stay in the dialog, and
// gowakutest's Find and Tap find the dialog's buttons, not those of the page
// with the same label. What opens over the dialog, such as a toast or the
// list of a dropdown in it, stays in the tree. The page comes back as the
// dialog starts to close.
//
// The dialog takes the theme, MediaQuery and Localizations above ctx, so a
// basic.Theme around part of the tree gives its dialogs its look; see
// Host.InsertFrom. build runs again whenever the overlay rebuilds or
// they change. It must not be called during a Build.
func ShowDialog(ctx core.BuildContext, build func(dismiss func()) core.Widget) (dismiss func()) {
	h := &routeHandle{}
	dismiss = h.dismiss
	h.entry = Of(ctx).InsertFrom(ctx, func() core.Widget {
		return dialogRoute{build: build, handle: h, dismiss: dismiss}
	})
	return dismiss
}

// routeHandle is what ShowDialog and ShowSheet share with the route they open:
// its dismiss is the function they return and build gets.
type routeHandle struct {
	entry *Entry
	// leave starts the exit of the route while its state is mounted.
	leave func()
	// done is set once the route is closing or gone.
	done bool
}

// dismiss closes the route. It does nothing once the route is closing or gone.
func (h *routeHandle) dismiss() {
	if h.done {
		return
	}
	if h.leave != nil {
		h.leave()
		return
	}
	// The route has not been built yet: take it out before it shows.
	h.done = true
	h.entry.Remove()
}

// dialogRoute is the scrim and transition around a dialog.
type dialogRoute struct {
	core.StatefulBase
	build   func(dismiss func()) core.Widget
	handle  *routeHandle
	dismiss func()
}

// enteringOpacity is the least opacity of a dialog that comes in: too faint
// to see, but not 0, which would hide the dialog from the focus.
const enteringOpacity = 1.0 / 256

type dialogRouteState struct {
	core.StateBase
	controller motion.AnimationController
	// previous had the focus before the dialog.
	previous core.FocusTarget
	// gate holds the OnDismissRequest of the Dialog inside.
	gate dialogGate
}

// dialogGate is how a Dialog tells the route around it what Escape and a
// click beside it do. The Dialog sets request as it builds; nil closes.
type dialogGate struct {
	request func() bool
}

// CreateState implements core.StatefulWidget.
func (dialogRoute) CreateState() core.State { return &dialogRouteState{} }

// InitState implements core.State.
func (s *dialogRouteState) InitState() {
	c := &s.controller
	c.Duration, c.ReverseDuration = kit.DialogEnter, dialogExit
	c.Curve, c.ReverseCurve = motion.SpringOut, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { kit.Rebuild(&s.StateBase) }
	c.OnStatus = func(status motion.Status) {
		if status == motion.Dismissed {
			s.gone()
		}
	}
	c.Forward()
	s.Widget().(dialogRoute).handle.leave = s.dismiss
}

// Dispose implements core.State.
func (s *dialogRouteState) Dispose() {
	h := s.Widget().(dialogRoute).handle
	h.leave, h.done = nil, true
	s.controller.Dispose()
}

func (s *dialogRouteState) dismiss() {
	if s.controller.Status() != motion.Reversing {
		s.Widget().(dialogRoute).handle.done = true
		s.controller.Reverse()
	}
}

// requestDismiss is Escape or a click beside the dialog: it closes the dialog,
// or asks the Dialog inside whether to.
func (s *dialogRouteState) requestDismiss() {
	if s.controller.Status() == motion.Reversing {
		return
	}
	if f := s.gate.request; f != nil && !f() {
		return
	}
	s.dismiss()
}

func (s *dialogRouteState) gone() {
	if s.previous != nil {
		core.RestoreFocus(s.previous)
	}
	s.Widget().(dialogRoute).handle.entry.Remove()
}

func (s *dialogRouteState) key(e *core.KeyEvent) bool {
	if e.Key != input.KeyEscape {
		return false
	}
	if !e.Repeat {
		s.requestDismiss()
	}
	return true
}

// Build implements core.State.
func (s *dialogRouteState) Build(ctx core.BuildContext) core.Widget {
	route := s.Widget().(dialogRoute)
	th := basic.ThemeOf(ctx)
	v := s.controller.Value()
	fade := geom.Clamp32(v, 0, 1)
	if s.controller.Status() != motion.Reversing {
		// An Opacity of 0 hides its subtree from the focus (core.Hider), so a
		// field that takes the keyboard on the first frame of the dialog would
		// lose it at the next one. Coming in, the dialog is never quite clear.
		fade = max(fade, enteringOpacity)
	}
	if basic.ReduceMotionOf(s.Context()) {
		// The dialog fades in at its size.
		v = 1
	}
	scrim := th.Color.Scrim
	return kit.ModalScope{
		Active:         s.controller.Status() != motion.Reversing,
		BlockSemantics: true,
		Child: interact.Focus{
			Enabled:     true,
			Autofocus:   true,
			OnAutofocus: func(previous core.FocusTarget) { s.previous = previous },
			OnKey:       s.key,
			// The tap goes around everything, because Center covers the scrim.
			Child: interact.GestureDetector{OnTap: s.requestDismiss, Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
				basic.Container{Color: scrim.WithAlpha(scrim.A * fade)},
				basic.Center{Child: basic.Opacity{Opacity: fade, Child: basic.Scale{
					Scale:  dialogStartScale + (1-dialogStartScale)*v,
					Origin: layout.AlignCenter,
					// A press on the dialog must not reach the scrim, or take the
					// keyboard away.
					Child: interact.GestureDetector{OnTapDown: func() {}, Child: core.Inherited[*dialogGate]{
						Value: &s.gate,
						Child: route.build(route.dismiss),
					}},
				}}},
			}}},
		},
	}
}

// Dialog is the card of a dialog. Put it in ShowDialog, which hides the page
// under it from assistive technology; a Dialog shown on its own does not.
type Dialog struct {
	core.StatelessBase
	Title string
	// Message is what the dialog says, and Content whatever else it shows.
	Message string
	Content core.Widget
	// Actions go in a row at the bottom right, the main one last.
	Actions []core.Widget
	// Width is the width of the dialog, between 280 and 560. Zero selects 340.
	Width float32
	// Glass makes the card translucent over a blur of what lies behind it:
	// the Glass color of the theme over a backdrop blur of 20 logical pixels.
	// The default is the opaque Raised card. See docs/ADR/0044-drawing-extensions.md.
	Glass bool
	// OnDismissRequest, if set, runs when Escape or a click beside the
	// dialog asks to close it, as OnCloseRequest does for a window: the
	// dialog closes if it returns true and stays open if it returns false.
	// A dialog with unsaved work returns false to keep itself open, or to
	// ask first, as a "discard the draft?" prompt does, and calls the
	// dismiss function of ShowDialog once the answer is to close. Nil
	// closes the dialog. The dismiss function always closes it, without
	// asking.
	OnDismissRequest func() bool
}

// Build implements core.StatelessWidget.
func (d Dialog) Build(ctx core.BuildContext) core.Widget {
	if gate, ok := core.Lookup[*dialogGate](ctx); ok {
		// The route reads it only when a key or a click comes, after this
		// build.
		gate.request = d.OnDismissRequest
	}
	th := basic.ThemeOf(ctx)
	parts := make([]core.Widget, 0, 4)
	if d.Title != "" {
		parts = append(parts, interact.Semantics{Role: semantics.RoleHeading, Level: 1, Label: d.Title, Child: basic.Text{
			Content: d.Title, Style: kit.Colored(th.Text.Title, th.Color.Ink),
		}})
	}
	if d.Message != "" {
		parts = append(parts, basic.Text{Content: d.Message, Style: kit.Colored(th.Text.Body, th.Color.Ink2)})
	}
	if d.Content != nil {
		parts = append(parts, d.Content)
	}
	if len(d.Actions) > 0 {
		parts = append(parts, basic.Padding{
			Insets: geom.Insets{Top: theme.Space12},
			Child:  basic.Row{Main: layout.MainEnd, Spacing: theme.Space8, Children: d.Actions},
		})
	}
	width := d.Width
	if width <= 0 {
		width = dialogWidth
	}
	card := basic.Container{
		Width:   geom.Clamp32(width, dialogMinWidth, dialogMaxWidth),
		Color:   th.Color.Raised,
		Radius:  th.Radius.Dialog,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Padding: geom.All(dialogPadding),
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: parts},
	}
	var body core.Widget = card
	if d.Glass {
		card.Color = th.Color.Glass
		body = basic.BackdropFilter{Sigma: dialogGlassBlur, Radius: th.Radius.Dialog, Child: card}
	}
	return interact.Semantics{Role: semantics.RoleDialog, Label: d.Title, Hint: d.Message, Child: body}
}
