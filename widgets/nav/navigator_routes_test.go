package nav

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// routesRig is a Navigator whose Routes the test replaces between builds.
type routesRig struct {
	navRig
	routes []RoutePattern
	swap   *wtest.SwapState
}

func newRoutesRig(t testing.TB, routes []RoutePattern) *routesRig {
	t.Helper()
	r := &routesRig{routes: routes}
	r.ctl, r.size, r.now = &NavigatorController{}, geom.Size{W: 400, H: 300}, time.Second
	r.tree = wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1}, Child: overlay.Overlay{
		Child: wtest.Swap{State: &r.swap, Build: func() core.Widget {
			return Navigator{Home: basic.Text{Content: "home"}, Routes: r.routes, Controller: r.ctl}
		}},
	}})
	r.tree.SetWindowFocused(true)
	r.frame()
	return r
}

// rebuild builds the Navigator again with routes.
func (r *routesRig) rebuild(routes []RoutePattern) {
	r.swap.SetState(func() { r.routes = routes })
	r.frame()
}

// recorder returns a Build that stores in *got which route built the page,
// with the parameters id and slug it was given.
func recorder(got *string, route string) func(RouteParams) core.Widget {
	return func(p RouteParams) core.Widget {
		*got = route + " id=" + p.Get("id") + " slug=" + p.Get("slug")
		return basic.Text{Content: route}
	}
}

func TestNavigator_RenamedParameterIsMatchedAgain(t *testing.T) {
	var got string
	r := newRoutesRig(t, []RoutePattern{{Pattern: "/notes/:id", Build: recorder(&got, "a")}})
	r.ctl.PushNamed("/notes/42")
	r.settle(t)
	if got != "a id=42 slug=" {
		t.Fatalf("before the rebuild: %q", got)
	}
	r.rebuild([]RoutePattern{{Pattern: "/notes/:slug", Build: recorder(&got, "b")}})
	if want := "b id= slug=42"; got != want {
		t.Errorf("after the pattern changed: %q, want %q", got, want)
	}
}

func TestNavigator_FirstMatchingRouteWinsAfterRoutesChange(t *testing.T) {
	var got string
	r := newRoutesRig(t, []RoutePattern{
		{Pattern: "/about", Build: recorder(&got, "about")},
		{Pattern: "/notes/:id", Build: recorder(&got, "detail")},
	})
	r.ctl.PushNamed("/notes/new")
	r.settle(t)
	if got != "detail id=new slug=" {
		t.Fatalf("before the rebuild: %q", got)
	}
	// The old index still holds a pattern that matches, but an earlier one
	// now matches first.
	r.rebuild([]RoutePattern{
		{Pattern: "/notes/new", Build: recorder(&got, "new")},
		{Pattern: "/notes/:id", Build: recorder(&got, "detail")},
	})
	if want := "new id= slug="; got != want {
		t.Errorf("after an earlier route was added: %q, want %q", got, want)
	}
	if e := r.state().top(); e.pattern != 0 {
		t.Errorf("the page keeps pattern %d", e.pattern)
	}
	// A route that no longer matches anything falls to NotFound.
	r.rebuild([]RoutePattern{{Pattern: "/about", Build: recorder(&got, "about")}})
	if e := r.state().top(); e.pattern != -1 {
		t.Errorf("the page keeps pattern %d of routes that do not match it", e.pattern)
	}
}

func TestNavigator_UnchangedRoutesAreNotMatchedAgain(t *testing.T) {
	var got string
	routes := []RoutePattern{
		{Pattern: "/about", Build: recorder(&got, "about")},
		{Pattern: "/notes/:id", Build: recorder(&got, "detail")},
	}
	r := newRoutesRig(t, routes)
	r.ctl.PushNamed("/notes/42")
	r.settle(t)
	e := r.state().top()
	// The parameters of a match are new slices; a build that leaves the
	// patterns alone keeps those it has.
	before := &e.params.keys[0]
	r.rebuild(append([]RoutePattern(nil), routes...))
	if e.pattern != 1 || &e.params.keys[0] != before {
		t.Error("a build with the same patterns matched the page again")
	}
}
