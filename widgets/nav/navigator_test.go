package nav_test

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestRoutePattern_Match(t *testing.T) {
	for _, tt := range []struct {
		pattern, name string
		ok            bool
		params        map[string]string
	}{
		{"/", "/", true, nil},
		{"/", "", true, nil},
		{"/notes", "/notes/", true, nil},
		{"/notes", "/notes/42", false, nil},
		{"/notes/:id", "/notes/42", true, map[string]string{"id": "42"}},
		{"/notes/:id", "/notes", false, nil},
		{"/notes/:id", "/notes/42/edit", false, nil},
		{"/notes/:id/edit", "/notes/7/edit", true, map[string]string{"id": "7"}},
		{"/notes/:id", "/notes/42?tab=history#top", true, map[string]string{"id": "42", "tab": "history"}},
		{"/tags/:tag", "/tags/a%2Fb", true, map[string]string{"tag": "a/b"}},
		{"/files/*", "/files/docs/a.txt", true, map[string]string{"*": "docs/a.txt"}},
		{"/files/*", "/files", true, map[string]string{"*": ""}},
		{"/files/*", "/other/a", false, nil},
	} {
		p, ok := nav.RoutePattern{Pattern: tt.pattern}.Match(tt.name)
		if ok != tt.ok {
			t.Errorf("%q on %q: matched %v, want %v", tt.pattern, tt.name, ok, tt.ok)
			continue
		}
		for k, v := range tt.params {
			if got, found := p.Lookup(k); !found || got != v {
				t.Errorf("%q on %q: %s = %q, %v; want %q", tt.pattern, tt.name, k, got, found, v)
			}
		}
		if ok && p.Name != tt.name {
			t.Errorf("Name %q, want %q", p.Name, tt.name)
		}
	}
	p, _ := nav.RoutePattern{Pattern: "/notes/:id"}.Match("/notes/42")
	if n, ok := p.Int("id"); !ok || n != 42 {
		t.Errorf("Int(id) = %d, %v", n, ok)
	}
	if _, ok := p.Int("missing"); ok {
		t.Error("a missing key is not an integer")
	}
	if p.Get("missing") != "" {
		t.Error("a missing key reads as empty")
	}
}

// counter is a page with State: it counts its InitStates and its taps.
type counter struct {
	core.StatefulBase
	label string
	inits *int
}

type counterState struct {
	core.StateOf[counter]
	taps int
}

func (counter) CreateState() core.State { return &counterState{} }

func (s *counterState) InitState() { *s.Widget().inits++ }

func (s *counterState) Build(core.BuildContext) core.Widget {
	w := s.Widget()
	return control.Button{Label: w.label + " " + itoa(s.taps), OnPress: func() { s.SetState(func() { s.taps++ }) }}
}

func itoa(n int) string { return string(rune('0' + n)) }

// navApp is a Navigator with a home page and named note pages, and a log of
// what happened.
type navApp struct {
	ctl     *nav.NavigatorController
	changes []string
	popped  []string
	inits   int
}

func (a *navApp) widget() nav.Navigator {
	return nav.Navigator{
		Home:       basic.Column{Children: []core.Widget{counter{label: "home", inits: &a.inits}}},
		Controller: a.ctl,
		Routes: []nav.RoutePattern{
			{Pattern: "/notes/:id", Build: func(p nav.RouteParams) core.Widget {
				return basic.Column{Children: []core.Widget{
					nav.BackButton{},
					control.Button{Label: "note " + p.Get("id"), OnPress: func() {}},
				}}
			}},
		},
		OnChange: func(name string) { a.changes = append(a.changes, name) },
	}
}

func newNavApp(t *testing.T, opts gowakutest.Options) (*navApp, *gowakutest.Tester) {
	t.Helper()
	a := &navApp{ctl: &nav.NavigatorController{}}
	return a, gowakutest.New(t, a.widget(), opts)
}

func has(tt *gowakutest.Tester, label string) bool {
	_, ok := tt.Find(label)
	return ok
}

func TestNavigator_StackOperations(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	c := a.ctl
	if c.Top() != "/" || c.Depth() != 1 || c.CanPop() {
		t.Fatalf("start: top %q depth %d canPop %v", c.Top(), c.Depth(), c.CanPop())
	}
	if c.Pop() {
		t.Error("the first page stays")
	}
	c.PushNamed("/notes/1")
	c.PushNamed("/notes/2")
	tt.Settle()
	if c.Top() != "/notes/2" || c.Depth() != 3 || !c.CanPop() {
		t.Errorf("after two pushes: top %q depth %d", c.Top(), c.Depth())
	}
	if !has(tt, "note 2") || has(tt, "note 1") || has(tt, "home 0") {
		t.Errorf("only the page on top is in the semantics tree:\n%s", tt.Dump())
	}

	c.Replace(nav.PageRoute{Name: "/about", Page: basic.Text{Content: "about"}, OnPop: func() { a.popped = append(a.popped, "/about") }})
	tt.Settle()
	if c.Top() != "/about" || c.Depth() != 3 {
		t.Errorf("after Replace: top %q depth %d", c.Top(), c.Depth())
	}
	c.Push(nav.PageRoute{Name: "/x", Page: basic.Text{Content: "x"}, Instant: true})
	if tt.Tree().Ticking() {
		t.Error("an instant route does not move")
	}
	tt.Pump()
	c.PopUntil("/notes/1")
	tt.Settle()
	if c.Top() != "/notes/1" || c.Depth() != 2 {
		t.Errorf("after PopUntil: top %q depth %d", c.Top(), c.Depth())
	}
	if len(a.popped) != 1 {
		t.Errorf("OnPop of a page that PopUntil passed: %v", a.popped)
	}
	c.PopUntil("/nowhere")
	tt.Settle()
	if c.Depth() != 1 || !has(tt, "home 0") {
		t.Errorf("PopUntil of a missing name goes to the first page: depth %d", c.Depth())
	}
	want := "/notes/1 /notes/2 /about /x /notes/1 /"
	if got := strings.Join(a.changes, " "); got != want {
		t.Errorf("OnChange: %s, want %s", got, want)
	}
}

func TestNavigator_PagesBelowKeepTheirState(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	tt.Tap("home 0")
	tt.Tap("home 1")
	a.ctl.PushNamed("/notes/9")
	tt.Settle()
	a.ctl.Pop()
	tt.Settle()
	if !has(tt, "home 2") || a.inits != 1 {
		t.Errorf("the home page must come back as it was: inits %d\n%s", a.inits, tt.Dump())
	}
}

func TestNavigator_UnknownNameShowsNotFound(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	a.ctl.PushNamed("/missing/page")
	tt.Settle()
	if !has(tt, "Page not found") || !has(tt, "Back") {
		t.Fatalf("got\n%s", tt.Dump())
	}
	tt.Tap("Back")
	tt.Settle()
	if a.ctl.Depth() != 1 {
		t.Error("the BackButton of the notice goes back")
	}
}

func TestNavigator_BackButtonOnlyWhereThereIsAWayBack(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	a.ctl.Push(nav.PageRoute{Page: basic.Column{Children: []core.Widget{nav.BackButton{Label: "Notes"}}}})
	tt.Settle()
	n, ok := tt.Find("Notes")
	if !ok || n.Role != semantics.RoleButton {
		t.Fatalf("a BackButton with a label is a button named by it:\n%s", tt.Dump())
	}
	tt.Tap("Notes")
	tt.Settle()
	if a.ctl.Depth() != 1 {
		t.Fatal("the BackButton goes back")
	}
	if !a.ctl.CanForward() {
		t.Error("going back with the BackButton can be undone with Forward")
	}
	a.ctl.Push(nav.PageRoute{Page: basic.Column{Cross: layout.CrossStart, Children: []core.Widget{nav.BackButton{}}}})
	tt.Settle()
	if n, ok := tt.Find("Back"); !ok || n.Rect.W > 48 || n.Rect.X != 0 {
		t.Errorf("a BackButton without a label is a small square at the start, got %+v", n.Rect)
	}
	root := gowakutest.New(t, nav.Navigator{Home: nav.BackButton{}}, pressableFonts)
	if has(root, "Back") {
		t.Error("the first page has no way back, and no BackButton")
	}
}

func TestNavigator_FocusMovesInAndComesBack(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	tt.Key(input.KeyTab, 0)
	if n, _ := tt.Find("home 0"); n.State&semantics.Focused == 0 {
		t.Fatal("Tab focuses the home button")
	}
	a.ctl.PushNamed("/notes/3")
	tt.Settle()
	if n, ok := tt.Find("Back"); !ok || n.State&semantics.Focused == 0 {
		t.Errorf("the new page takes the focus, on its first control:\n%s", tt.Dump())
	}
	tt.Key(input.KeyTab, 0)
	if n, _ := tt.Find("note 3"); n.State&semantics.Focused == 0 {
		t.Error("Tab stays on the page on top")
	}
	tt.Key(input.KeyTab, 0)
	if n, _ := tt.Find("Back"); n.State&semantics.Focused == 0 {
		t.Errorf("Tab wraps around the page on top, past the hidden page:\n%s", tt.Dump())
	}
	a.ctl.Pop()
	tt.Settle()
	if n, _ := tt.Find("home 0"); n.State&semantics.Focused == 0 {
		t.Errorf("the focus comes back to what had it:\n%s", tt.Dump())
	}
}

func TestNavigator_AltArrowsAndMouseButtons(t *testing.T) {
	a, tt := newNavApp(t, pressableFonts)
	a.ctl.PushNamed("/notes/1")
	a.ctl.PushNamed("/notes/2")
	tt.Settle()

	tt.Key(input.KeyLeft, input.ModAlt)
	tt.Settle()
	if a.ctl.Top() != "/notes/1" {
		t.Fatalf("Alt+Left goes back: top %q", a.ctl.Top())
	}
	at := geom.Offset{X: 100, Y: 100}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonBack})
	if a.ctl.Top() != "/notes/1" {
		t.Error("the back button acts when it comes up")
	}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonBack})
	tt.Settle()
	if a.ctl.Top() != "/" {
		t.Fatalf("the back button goes back: top %q", a.ctl.Top())
	}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonForward})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonForward})
	tt.Settle()
	if a.ctl.Top() != "/notes/1" {
		t.Fatalf("the forward button brings back what back took: top %q", a.ctl.Top())
	}
	tt.Key(input.KeyRight, input.ModAlt)
	tt.Settle()
	if a.ctl.Top() != "/notes/2" || a.ctl.CanForward() {
		t.Fatalf("Alt+Right too: top %q", a.ctl.Top())
	}
	tt.Key(input.KeyLeft, input.ModAlt)
	tt.Settle()
	a.ctl.PushNamed("/notes/5")
	if a.ctl.CanForward() {
		t.Error("a push forgets what Forward could bring")
	}
	tt.Settle()
	tt.Key(input.KeyEscape, 0)
	tt.Settle()
	if a.ctl.Top() != "/notes/5" {
		t.Error("Escape does not go back")
	}
}

func TestNavigator_InnerNavigatorGoesBackFirst(t *testing.T) {
	outer := &nav.NavigatorController{}
	inner := &nav.NavigatorController{}
	tt := gowakutest.New(t, nav.Navigator{Controller: outer, Home: nav.Navigator{
		Controller: inner,
		Home:       basic.Text{Content: "inner home"},
	}}, pressableFonts)
	outer.Push(nav.PageRoute{Page: nav.Navigator{Home: basic.Text{Content: "unrelated"}}})
	tt.Settle()
	outer.Pop()
	tt.Settle()
	inner.Push(nav.PageRoute{Name: "/a", Page: nav.BackButton{}})
	tt.Settle()
	outer.Push(nav.PageRoute{Name: "/b", Page: basic.Text{Content: "b"}})
	tt.Settle()
	outer.Pop()
	tt.Settle()

	tt.Key(input.KeyLeft, input.ModAlt)
	tt.Settle()
	if inner.Depth() != 1 || outer.Depth() != 1 {
		t.Fatalf("the inner Navigator goes back first: inner %d, outer %d", inner.Depth(), outer.Depth())
	}
	outer.Push(nav.PageRoute{Name: "/c", Page: basic.Text{Content: "c"}})
	tt.Settle()
	// The inner Navigator sits on the hidden first page, so it hears nothing.
	tt.Key(input.KeyLeft, input.ModAlt)
	tt.Settle()
	if outer.Depth() != 1 {
		t.Fatal("with nowhere to go inside, the request goes out")
	}
}

func TestNavigator_BackButtonOfAnInnerFirstPageGoesOut(t *testing.T) {
	outer := &nav.NavigatorController{}
	tt := gowakutest.New(t, nav.Navigator{Controller: outer, Home: basic.Text{Content: "home"}}, pressableFonts)
	outer.Push(nav.PageRoute{Page: nav.Navigator{Home: nav.BackButton{}}})
	tt.Settle()
	tt.Tap("Back")
	tt.Settle()
	if outer.Depth() != 1 {
		t.Error("the BackButton on the first page of an inner Navigator goes back in the outer one")
	}
}

func TestNavigator_CustomTransition(t *testing.T) {
	var seen []float32
	a, tt := newNavApp(t, pressableFonts)
	a.ctl.Push(nav.PageRoute{
		Page: basic.Text{Content: "custom"},
		Transition: func(child core.Widget, v float32) core.Widget {
			seen = append(seen, v)
			return basic.Opacity{Opacity: min(max(v, 0), 1), Child: child}
		},
	})
	tt.Advance(100 * time.Millisecond)
	tt.Settle()
	if len(seen) < 3 || seen[0] != 0 || seen[len(seen)-1] != 1 {
		t.Errorf("the transition sees the motion from 0 to 1: %v", seen)
	}
}

func TestNavigator_ControllerBeforeTheTreePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "not attached") {
			t.Errorf("got %v", r)
		}
	}()
	(&nav.NavigatorController{}).Pop()
}

func TestNavigatorOf_FindsTheNearest(t *testing.T) {
	var got *nav.NavigatorController
	ctl := &nav.NavigatorController{}
	gowakutest.New(t, nav.Navigator{Controller: ctl, Home: basic.Builder{Content: func(ctx core.BuildContext) core.Widget {
		got = nav.NavigatorOf(ctx)
		return basic.SizedBox{}
	}}}, pressableFonts)
	if got != ctl {
		t.Error("NavigatorOf returns the controller of the Navigator")
	}
}

// pressableFonts keeps the tests independent of the fonts of the machine.
var pressableFonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}
