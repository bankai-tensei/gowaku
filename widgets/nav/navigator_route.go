package nav

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// PageRoute is a page that a Navigator shows, with how it comes and goes.
//
//	NavigatorOf(ctx).Push(PageRoute{Name: "/settings", Page: SettingsPage{}})
type PageRoute struct {
	// Name names the page, as "/notes/42". It may stay empty for a page pushed
	// as a value; PopUntil finds pages by it, and OnChange reports it.
	Name string
	// Page is what the route shows. It is required.
	Page core.Widget
	// Transition moves the page while it comes and goes. Nil selects the
	// standard one: it slides in from the trailing edge by a few pixels while
	// it fades in, and goes back the same way. A window that asks for less
	// motion keeps the fade and drops the slide.
	Transition PageTransition
	// Instant shows and takes away the page without any motion, and without
	// Hero flights.
	Instant bool
	// OnPop runs when the page leaves the stack, by Pop, PopUntil, Replace or a
	// request to go back. A page that hands a result back calls a callback of
	// its own instead, as in OnSave func(Note), before it pops.
	OnPop func()
}

// PageTransition wraps a page for one frame of its motion. t is the progress of
// its route, from 0, out of sight, to 1, in place, with the curve applied; it
// may overshoot 1 a little. It must return the same widget types for every t,
// or the page loses its State mid-motion.
//
// A custom transition builds the page again on every frame of the motion. The
// standard one, which a nil Transition selects, only paints it again.
type PageTransition func(child core.Widget, t float32) core.Widget

// RoutePattern builds the pages of the names that match Pattern.
//
// A pattern is a path of segments. A segment that starts with a colon, as in
// "/notes/:id", matches any one segment of a name and makes it a parameter; a
// last segment "*" matches the rest of the name, which is the parameter "*".
// Other segments match themselves. A trailing slash makes no difference, and
// the query after "?" is not part of the match: RouteParams.Get reads it.
type RoutePattern struct {
	Pattern string
	// Build returns the page of a name that matched.
	Build func(p RouteParams) core.Widget
	// Transition and Instant are those of the routes the pattern makes. See
	// PageRoute.
	Transition PageTransition
	Instant    bool
}

// RouteParams are the parts of a name that a RoutePattern picked out.
type RouteParams struct {
	// Name is the whole name, as "/notes/42?tab=history".
	Name string
	// keys and values are the parameters of the path, in order.
	keys, values []string
	query        string
}

// Get returns the parameter key: a segment of the path, as "id" for
// "/notes/:id", or else a value of the query, as "tab" for "?tab=history". It
// returns "" for a key it does not have.
func (p RouteParams) Get(key string) string {
	v, _ := p.Lookup(key)
	return v
}

// Lookup is Get that also reports whether the key is there.
func (p RouteParams) Lookup(key string) (string, bool) {
	for i, k := range p.keys {
		if k == key {
			return p.values[i], true
		}
	}
	if p.query == "" {
		return "", false
	}
	q, err := url.ParseQuery(p.query)
	if err != nil || !q.Has(key) {
		return "", false
	}
	return q.Get(key), true
}

// Int returns the parameter key as a decimal integer, and false if it is
// missing or not one.
func (p RouteParams) Int(key string) (int, bool) {
	v, ok := p.Lookup(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// Match reports whether name matches the pattern, and with which parameters.
// The segments of the path are unescaped, so "/tags/a%2Fb" gives "a/b".
func (r RoutePattern) Match(name string) (RouteParams, bool) {
	p := RouteParams{Name: name}
	path := name
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		if path[i] == '?' {
			p.query = path[i+1:]
			if j := strings.IndexByte(p.query, '#'); j >= 0 {
				p.query = p.query[:j]
			}
		}
		path = path[:i]
	}
	want, got := segments(r.Pattern), segments(path)
	for i, seg := range want {
		if seg == "*" && i == len(want)-1 {
			p.keys = append(p.keys, "*")
			p.values = append(p.values, unescape(strings.Join(got[min(i, len(got)):], "/")))
			return p, true
		}
		if i >= len(got) {
			return RouteParams{}, false
		}
		if key, ok := strings.CutPrefix(seg, ":"); ok {
			p.keys = append(p.keys, key)
			p.values = append(p.values, unescape(got[i]))
			continue
		}
		if seg != got[i] {
			return RouteParams{}, false
		}
	}
	if len(got) != len(want) {
		return RouteParams{}, false
	}
	return p, true
}

// segments splits a path at its slashes, without the empty ends.
func segments(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func unescape(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
}

// match finds the first pattern of routes that name matches, and -1 if none
// does.
func match(routes []RoutePattern, name string) (int, RouteParams) {
	for i, r := range routes {
		if p, ok := r.Match(name); ok {
			return i, p
		}
	}
	return -1, RouteParams{Name: name}
}

// notFoundPage is the page of a name that no pattern matches, when the
// Navigator has no NotFound of its own.
type notFoundPage struct {
	core.StatelessBase
	name string
}

// Build implements core.StatelessWidget.
func (p notFoundPage) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	l := basic.StringsOf(ctx)
	return basic.Center{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
		interact.Semantics{Role: semantics.RoleHeading, Level: 1, Label: l.PageNotFound, Child: basic.Text{Content: l.PageNotFound, Style: th.Text.Title}},
		basic.Text{Content: p.name, Style: kit.Colored(th.Text.Body, th.Color.Ink2)},
		basic.SizedBox{Height: theme.Space8},
		BackButton{Label: l.Back},
	}}}
}
