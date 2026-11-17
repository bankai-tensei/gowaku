package nav

import (
	"slices"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/motion"
)

const (
	// pageEnter and pageExit are the lengths of the motion of a page, those of
	// a dialog in DESIGN §7: in at ease, out quickly. A page comes in on
	// motion.Standard rather than the spring of a dialog, whose overshoot
	// would make a whole page wobble.
	pageEnter = kit.DialogEnter
	pageExit  = 200 * time.Millisecond
	// pageSlide is how far a page slides while it comes and goes, in logical
	// pixels: enough to show the direction, too little to travel.
	pageSlide = 24
)

// Navigator shows a stack of pages, the top one in front, and moves between
// them: Push brings a page in from the trailing edge, Pop takes it back out, and
// the page below comes back as it was, with its State and the focus it had.
// Alt+Left and the back button of a mouse go back; Alt+Right and the forward
// button bring back what they took away.
//
// Pages have names, matched by Routes:
//
//	Navigator{
//		Home: NoteList{},
//		Routes: []RoutePattern{
//			{Pattern: "/notes/:id", Build: func(p RouteParams) core.Widget {
//				id, _ := p.Int("id")
//				return NoteDetail{ID: id}
//			}},
//		},
//	}
//
// and a page goes to another with NavigatorOf(ctx).PushNamed("/notes/42"). The
// pages below the top are kept, laid out and alive, but hidden from the
// pointer, the keyboard and assistive technology. The page that comes in takes
// the focus, which a screen reader announces: what in it asks for the focus
// with Autofocus, or else its first control, usually its BackButton. Escape
// does not go back, since it belongs to what has the focus. A Navigator inside
// a page of another one goes back first, and hands the request out when it has
// nowhere to go. See docs/ADR/0063-navigator.md.
type Navigator struct {
	core.StatefulBase
	// Home is the first page, named "/". Without it the first page is the one
	// Routes builds for Initial.
	Home core.Widget
	// Initial names the first page when Home is nil. Empty means "/". Only
	// that page is on the stack, even for a deeper name.
	Initial string
	// Routes builds the pages of names. The first pattern that matches wins.
	// The pages are built again from the patterns whenever the Navigator is,
	// so that a page made from the state of the app stays up to date.
	Routes []RoutePattern
	// NotFound builds the page of a name that no pattern matches. Nil shows a
	// short notice with a BackButton.
	NotFound func(name string) core.Widget
	// Controller drives the Navigator from outside its pages, as from a menu.
	// Nil keeps one inside; NavigatorOf returns either.
	Controller *NavigatorController
	// OnChange runs after the stack changed, with the name of the page on top.
	OnChange func(name string)
}

// NavigatorController moves a Navigator between pages. NavigatorOf returns
// the one of the nearest Navigator. Its methods must not be called during a
// Build, and panic before the Navigator is in the tree.
type NavigatorController struct {
	s *navigatorState
}

// NavigatorOf returns the controller of the nearest Navigator above ctx. It
// panics if there is none.
func NavigatorOf(ctx core.BuildContext) *NavigatorController {
	p, ok := core.Lookup[pageScope](ctx)
	if !ok {
		panic("gowaku: NavigatorOf found no Navigator above the widget")
	}
	return p.c
}

func (c *NavigatorController) state() *navigatorState {
	if c.s == nil {
		panic("gowaku: NavigatorController is not attached to a Navigator")
	}
	return c.s
}

// Push puts route on top of the stack.
func (c *NavigatorController) Push(route PageRoute) {
	if route.Page == nil {
		panic("gowaku: PageRoute.Page is nil")
	}
	s := c.state()
	s.forward = s.forward[:0]
	s.push(routeSpec{route: route}, false)
}

// PushNamed puts the page of name on top of the stack: the one Routes builds,
// or that of NotFound.
func (c *NavigatorController) PushNamed(name string) {
	s := c.state()
	s.forward = s.forward[:0]
	s.push(routeSpec{route: PageRoute{Name: name}, named: true}, false)
}

// Replace puts route in the place of the page on top, which goes once the new
// one has come in.
func (c *NavigatorController) Replace(route PageRoute) {
	if route.Page == nil {
		panic("gowaku: PageRoute.Page is nil")
	}
	s := c.state()
	s.forward = s.forward[:0]
	s.push(routeSpec{route: route}, true)
}

// ReplaceNamed puts the page of name in the place of the page on top.
func (c *NavigatorController) ReplaceNamed(name string) {
	s := c.state()
	s.forward = s.forward[:0]
	s.push(routeSpec{route: PageRoute{Name: name}, named: true}, true)
}

// Pop takes the page on top away, and reports whether there was one above the
// first. The first page stays.
func (c *NavigatorController) Pop() bool { return c.state().pop(false) }

// PopUntil takes pages away until the one on top is named name, or only the
// first is left. Only the page on top moves; those between go at once.
func (c *NavigatorController) PopUntil(name string) {
	c.state().popUntil(name)
}

// CanPop reports whether there is a page to go back to.
func (c *NavigatorController) CanPop() bool { return c.state().canPop() }

// Forward brings back the page that the last request to go back took away, as
// a browser does, and reports whether there was one. The page comes back new:
// the State it had is gone. Pushing a page forgets what Forward could bring.
func (c *NavigatorController) Forward() bool { return c.state().goForward() }

// CanForward reports whether Forward has a page to bring back.
func (c *NavigatorController) CanForward() bool { return len(c.state().forward) > 0 }

// Top returns the name of the page on top.
func (c *NavigatorController) Top() string {
	s := c.state()
	return s.top().name()
}

// Depth returns the number of pages on the stack, the first one included.
func (c *NavigatorController) Depth() int { return len(c.state().stack()) }

// back goes back in the nearest Navigator that can, this one or one further
// out, as a BackButton or a request of the user does. It records what it took
// away for Forward.
func (c *NavigatorController) back() bool {
	for ; c != nil && c.s != nil; c = c.s.parent {
		if c.s.pop(true) {
			return true
		}
	}
	return false
}

// pageScope is what the pages of a Navigator find above them: its controller,
// and whether a BackButton on the page has somewhere to go.
type pageScope struct {
	c    *NavigatorController
	back bool
}

// routeSpec is what makes a page: a route given as a value, or a name for
// Routes to build.
type routeSpec struct {
	route PageRoute
	named bool
}

// navEntry is a page on the stack, on its way in, in place, or on its way out.
type navEntry struct {
	id   int
	spec routeSpec
	// pattern is the index in Routes of the pattern that built a named page,
	// -1 for one of NotFound, and params what it picked out.
	pattern int
	params  RouteParams
	home    bool
	page    core.Widget
	// controller runs from 0, out of sight, to 1, in place.
	controller motion.AnimationController
	// exiting means the page is on its way out, and replaced that it goes once
	// the page above it has come in.
	exiting, replaced bool
	// arriving means the page waits for its first layout, to take the focus
	// and start the flights of its heroes.
	arriving bool
	// active means the page is the one on top that takes input, and painted
	// that it shows. shift is how far the standard transition moves it.
	active, painted bool
	shift           float32
	// focus is what had the keyboard in the page when a page covered it.
	focus   core.FocusTarget
	slot    *renderPageSlot
	host    *pageHostState
	flights []heroFlight
}

func (e *navEntry) name() string { return e.spec.route.Name }

// animated reports whether the page moves when it comes and goes.
func (e *navEntry) animated() bool { return !e.spec.route.Instant }

// settled reports whether the page is in place and covers what is below.
func (e *navEntry) settled() bool {
	return !e.exiting && e.controller.Status() == motion.Completed
}

type navigatorState struct {
	core.StateOf[Navigator]
	// entries are the pages from the bottom, those on their way out included.
	entries []*navEntry
	// forward holds what requests to go back took away, the last on top.
	forward []routeSpec
	nextID  int
	own     NavigatorController
	ctl     *NavigatorController
	parent  *NavigatorController
	// parentBack says whether a Navigator further out has a page to go back
	// to.
	parentBack bool
	render     *renderNavigator
	// active is scratch for stack.
	active []*navEntry
	// patterns holds the Pattern of each of Routes that the named pages were
	// last matched against, to tell when a build has to match them again.
	patterns []string
}

// CreateState implements core.StatefulWidget.
func (Navigator) CreateState() core.State { return &navigatorState{} }

// InitState implements core.State.
func (s *navigatorState) InitState() {
	w := s.Widget()
	s.attach(w.Controller)
	first := routeSpec{route: PageRoute{Name: "/"}, named: true}
	if w.Home == nil && w.Initial != "" {
		first.route.Name = w.Initial
	}
	e := s.newEntry(first)
	e.home = w.Home != nil
	e.controller.SetValue(1)
	s.entries = append(s.entries, e)
	s.refresh()
}

func (s *navigatorState) attach(c *NavigatorController) {
	if s.ctl != nil {
		s.ctl.s = nil
	}
	if c == nil {
		c = &s.own
	}
	c.s = s
	s.ctl = c
}

// DidUpdateWidget implements core.State.
func (s *navigatorState) DidUpdateWidget(old core.Widget) {
	if c := s.Widget().Controller; c != old.(Navigator).Controller {
		s.attach(c)
	}
}

// Dispose implements core.State.
func (s *navigatorState) Dispose() {
	for _, e := range s.entries {
		e.controller.Dispose()
	}
	if s.ctl != nil && s.ctl.s == s {
		s.ctl.s = nil
	}
}

func (s *navigatorState) newEntry(spec routeSpec) *navEntry {
	e := &navEntry{id: s.nextID, spec: spec, pattern: -1}
	s.nextID++
	if spec.named {
		e.pattern, e.params = match(s.Widget().Routes, spec.route.Name)
		if e.pattern >= 0 {
			p := s.Widget().Routes[e.pattern]
			e.spec.route.Transition, e.spec.route.Instant = p.Transition, p.Instant
		}
	}
	c := &e.controller
	c.Duration, c.ReverseDuration = pageEnter, pageExit
	c.Curve, c.ReverseCurve = motion.Standard, motion.Leave
	c.Attach(&s.StateBase)
	c.OnChange = func() { s.frame(e) }
	c.OnStatus = func(status motion.Status) { s.status(e, status) }
	return e
}

// routesChanged reports whether the patterns of Routes differ from those the
// named pages were matched against, and remembers the new ones. Only the
// Pattern of a route decides what it matches and which parameters it picks
// out, so comparing the strings is enough, and costs a build with the same
// patterns nothing but that.
func (s *navigatorState) routesChanged() bool {
	routes := s.Widget().Routes
	same := len(routes) == len(s.patterns)
	for i := 0; same && i < len(routes); i++ {
		same = routes[i].Pattern == s.patterns[i]
	}
	if same {
		return false
	}
	s.patterns = s.patterns[:0]
	for i := range routes {
		s.patterns = append(s.patterns, routes[i].Pattern)
	}
	return true
}

// pageOf builds the page of e from the Navigator as it is now. rematch says
// the patterns of Routes changed since e was matched: the first that matches
// wins again, with the parameters it picks out, as for a page pushed now.
func (s *navigatorState) pageOf(e *navEntry, rematch bool) core.Widget {
	w := s.Widget()
	switch {
	case e.home:
		return w.Home
	case !e.spec.named:
		return e.spec.route.Page
	}
	if rematch {
		e.pattern, e.params = match(w.Routes, e.name())
	}
	if e.pattern >= 0 {
		return w.Routes[e.pattern].Build(e.params)
	}
	if w.NotFound != nil {
		return w.NotFound(e.name())
	}
	return notFoundPage{name: e.name()}
}

// stack returns the pages that are not on their way out, from the bottom. The
// slice is scratch, valid until the next call.
func (s *navigatorState) stack() []*navEntry {
	s.active = s.active[:0]
	for _, e := range s.entries {
		if !e.exiting {
			s.active = append(s.active, e)
		}
	}
	return s.active
}

func (s *navigatorState) top() *navEntry {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if !s.entries[i].exiting {
			return s.entries[i]
		}
	}
	panic("gowaku: Navigator has no page")
}

func (s *navigatorState) canPop() bool {
	n := 0
	for _, e := range s.entries {
		if !e.exiting && !e.replaced {
			n++
		}
	}
	return n > 1
}

// reduce reports whether the window asks for less motion.
func (s *navigatorState) reduce() bool { return basic.ReduceMotionOf(s.Context()) }

// push puts the page of spec on top, in the place of the top one if replace is
// set.
func (s *navigatorState) push(spec routeSpec, replace bool) {
	below := s.top()
	s.keepFocus(below)
	e := s.newEntry(spec)
	e.arriving = true
	if replace {
		below.replaced = true
	}
	s.SetState(func() { s.entries = append(s.entries, e) })
	if e.animated() {
		e.controller.Forward()
	} else {
		e.controller.SetValue(1)
		s.settle(e)
	}
	s.refresh()
	s.changed()
}

// keepFocus remembers what has the keyboard in the page e, before another
// covers it.
func (s *navigatorState) keepFocus(e *navEntry) {
	e.focus = nil
	if s.render == nil || e.slot == nil {
		return
	}
	if f := s.render.Focused(); f != nil && isInside(f, e.slot) {
		e.focus = f
	}
}

// pop takes the page on top away. forward keeps it for Forward.
func (s *navigatorState) pop(forward bool) bool {
	if !s.canPop() {
		return false
	}
	e := s.top()
	if forward {
		s.forward = append(s.forward, e.spec)
	}
	s.leave(e)
	s.refresh()
	s.reveal()
	s.changed()
	return true
}

// popUntil pops pages until the top one is named name.
func (s *navigatorState) popUntil(name string) {
	top := s.top()
	if top.name() == name || !s.canPop() {
		return
	}
	stack := s.stack()
	keep := 0
	for i := len(stack) - 2; i > 0; i-- {
		if stack[i].name() == name {
			keep = i
			break
		}
	}
	gone := slices.Clone(stack[keep+1 : len(stack)-1])
	s.SetState(func() {
		for _, e := range gone {
			s.remove(e)
		}
	})
	for _, e := range gone {
		if e.spec.route.OnPop != nil {
			e.spec.route.OnPop()
		}
	}
	s.leave(top)
	s.refresh()
	s.reveal()
	s.changed()
}

// leave starts e on its way out.
func (s *navigatorState) leave(e *navEntry) {
	e.exiting = true
	if e.spec.route.OnPop != nil {
		e.spec.route.OnPop()
	}
	if !e.animated() || e.controller.Status() == motion.Dismissed {
		s.SetState(func() { s.remove(e) })
		return
	}
	c := &e.controller
	if c.Status() == motion.Forwarding {
		// A page that has not come in yet goes back the way it came, with no
		// jump between the curves.
		c.ReverseCurve, c.ReverseDuration = c.Curve, 0
	}
	s.startFlights(e)
	c.Reverse()
}

// reveal gives the keyboard back to what had it in the page that is now on
// top, or to the first thing there that takes it.
func (s *navigatorState) reveal() {
	e := s.top()
	if e.focus != nil {
		f := e.focus
		e.focus = nil
		core.RestoreFocus(f)
		return
	}
	if e.slot != nil {
		if f := firstShownFocusable(e.slot); f != nil {
			core.RestoreFocus(f)
		}
	}
}

// goForward brings back the page the last request to go back took away.
func (s *navigatorState) goForward() bool {
	n := len(s.forward)
	if n == 0 {
		return false
	}
	spec := s.forward[n-1]
	s.forward = s.forward[:n-1]
	s.push(spec, false)
	return true
}

// remove takes e off the stack for good. It runs inside SetState.
func (s *navigatorState) remove(e *navEntry) {
	s.endFlights(e)
	e.controller.Dispose()
	if i := slices.Index(s.entries, e); i >= 0 {
		s.entries = slices.Delete(s.entries, i, i+1)
	}
}

// status follows the controller of e to the end of its motion.
func (s *navigatorState) status(e *navEntry, status motion.Status) {
	switch {
	case status == motion.Completed && !e.exiting:
		s.settle(e)
		s.refresh()
	case status == motion.Dismissed && e.exiting:
		if s.Mounted() {
			s.SetState(func() { s.remove(e) })
		}
		s.refresh()
	}
}

// settle ends the coming in of e: its flights land, and the page it replaced
// goes.
func (s *navigatorState) settle(e *navEntry) {
	s.endFlights(e)
	var gone []*navEntry
	for _, o := range s.entries {
		if o.replaced && o != e {
			gone = append(gone, o)
		}
	}
	if len(gone) == 0 {
		return
	}
	s.SetState(func() {
		for _, o := range gone {
			s.remove(o)
		}
	})
	for _, o := range gone {
		if o.spec.route.OnPop != nil {
			o.spec.route.OnPop()
		}
	}
}

// changed tells OnChange about the new top.
func (s *navigatorState) changed() {
	if f := s.Widget().OnChange; f != nil {
		f(s.top().name())
	}
}

// refresh decides which pages take input and which show: the top one takes
// input, and every page shows from the top down to the first that is in place.
func (s *navigatorState) refresh() {
	top := s.top()
	covered := false
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		e.active = e == top
		e.painted = !covered
		if e.settled() {
			covered = true
		}
		s.frame(e)
	}
	if s.render != nil {
		s.render.MarkNeedsPaint()
	}
}

// frame moves the page of e for the value of its controller. The standard
// transition only paints, so a frame of the motion builds nothing.
func (s *navigatorState) frame(e *navEntry) {
	t := e.controller.Value()
	if e.spec.route.Transition != nil {
		if e.host != nil {
			e.host.moved()
		}
		t = 1
	}
	shift := float32(0)
	if t != 1 && !s.reduce() {
		shift = (1 - t) * pageSlide
	}
	e.shift = shift
	if e.slot != nil {
		e.slot.moved(min(max(t, 0), 1))
	}
	if len(e.flights) > 0 && s.render != nil {
		s.render.MarkNeedsPaint()
	}
}

// laidOut runs after every layout of the Navigator. A page that just arrived
// takes the focus, unless something in it took it already, and its heroes
// take off.
func (s *navigatorState) laidOut() {
	for _, e := range s.entries {
		if !e.arriving || e.slot == nil {
			continue
		}
		e.arriving = false
		// The tick of this frame ran before the page had a slot, which would
		// otherwise show it at full opacity for a frame.
		s.frame(e)
		if e.exiting {
			continue
		}
		if f := s.render.Focused(); f == nil || !isInside(f, e.slot) {
			if f := firstShownFocusable(e.slot); f != nil {
				core.RestoreFocus(f)
			}
		}
		if e.controller.Status() == motion.Forwarding {
			s.startFlights(e)
		}
	}
}

// Build implements core.State.
func (s *navigatorState) Build(ctx core.BuildContext) core.Widget {
	if p, ok := core.DependOn[pageScope](ctx); ok {
		s.parent, s.parentBack = p.c, p.back
	} else {
		s.parent, s.parentBack = nil, false
	}
	children := make([]core.Widget, len(s.entries))
	rematch := s.routesChanged()
	bottom := true
	for i, e := range s.entries {
		e.page = s.pageOf(e, rematch)
		scope := pageScope{c: s.ctl, back: s.parentBack || !bottom}
		if !e.replaced {
			bottom = false
		}
		children[i] = core.Keyed(e.id, pageSlot{entry: e, child: basic.RepaintBoundary{Child: pageHost{
			entry: e,
			child: core.Inherited[pageScope]{Value: scope, Child: e.page},
		}}})
	}
	return navigatorLayout{state: s, children: children}
}

// pageHost holds a page, and wraps it in the custom transition of its route.
type pageHost struct {
	core.StatefulBase
	entry *navEntry
	child core.Widget
}

type pageHostState struct {
	core.StateOf[pageHost]
}

// CreateState implements core.StatefulWidget.
func (pageHost) CreateState() core.State { return &pageHostState{} }

// InitState implements core.State.
func (s *pageHostState) InitState() { s.Widget().entry.host = s }

// moved builds the page again for a new frame of a custom transition.
func (s *pageHostState) moved() { kit.Rebuild(&s.StateBase) }

// Build implements core.State.
func (s *pageHostState) Build(core.BuildContext) core.Widget {
	w := s.Widget()
	if tr := w.entry.spec.route.Transition; tr != nil {
		return tr(w.child, w.entry.controller.Value())
	}
	return w.child
}

// isInside reports whether ro is anc or below it.
func isInside(ro, anc core.RenderObject) bool {
	for ; ro != nil; ro = ro.Parent() {
		if ro == anc {
			return true
		}
	}
	return false
}

// firstShownFocusable returns the first object below ro, in the order of painting,
// that takes the focus and is not hidden, or nil.
func firstShownFocusable(ro core.RenderObject) core.FocusTarget {
	for i := range ro.ChildCount() {
		c := ro.ChildAt(i)
		if h, ok := c.(core.Hider); ok && h.Hidden() {
			continue
		}
		if f, ok := c.(core.FocusTarget); ok && f.CanFocus() {
			return f
		}
		if f := firstShownFocusable(c); f != nil {
			return f
		}
	}
	return nil
}
