//go:build gowakudebug

package inspect

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// sampleApp is the window under inspection in the tests: a text, a keyed
// button and a colored box in a layer.
func sampleApp() core.Widget {
	return basic.Column{Cross: layout.CrossStretch, Spacing: 8, Children: []core.Widget{
		basic.Text{Content: "Hello"},
		core.Keyed("ok", control.Button{Label: "OK", OnPress: func() {}}),
		basic.RepaintBoundary{Child: basic.Container{Width: 40, Height: 20, Color: geom.Hex(0x336699)}},
	}}
}

// newSample returns the sample as a target, and a session over it.
func newSample(t *testing.T) (*fakeTarget, *Session) {
	t.Helper()
	shaper := testutil.NewShaper(t)
	ft := newFakeTarget("Sample", core.Inherited[*text.Shaper]{Value: shaper, Child: sampleApp()}, geom.Size{W: 400, H: 300})
	return ft, NewSession(&fakeHost{targets: []Target{ft}}, shaper)
}

// findElement returns the first element whose widget is of type name.
func findElement(t *testing.T, root core.ElementView, name string) core.ElementView {
	t.Helper()
	found, ok := findNext([]core.ElementView{root}, elementChildren, func(v core.ElementView) string {
		if TypeName(v.Widget()) == name {
			return "\x00"
		}
		return ""
	}, core.ElementView{}, "\x00")
	if !ok {
		t.Fatalf("no element of %s", name)
	}
	return found
}

func TestElementLabel_ShowsTypeAndKey(t *testing.T) {
	ft, _ := newSample(t)
	root := ft.tree.RootElement()
	if got := elementLabel(root); got != "core.Inherited[*text.Shaper]" {
		t.Errorf("root: got %q", got)
	}
	// core.Keyed puts the key on a widget of its own above the button.
	keyed := findElement(t, root, "Keyed")
	if got, want := elementLabel(keyed), `Keyed  #ok`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	button := findElement(t, root, "control.Button")
	if got, want := elementLabel(button), `control.Button`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := keyLabel(42); got != "42" {
		t.Errorf("int key: got %q", got)
	}
	if got := keyLabel(strings.Repeat("x", 40)); len([]rune(got)) != maxKeyLabel || !strings.HasSuffix(got, "…") {
		t.Errorf("long key: got %q", got)
	}
	if got := kindLetter(button.Kind()); got != "F" {
		t.Errorf("a button is stateful: got %q", got)
	}
}

func TestElementChildren_FollowTheTree(t *testing.T) {
	ft, _ := newSample(t)
	root := ft.tree.RootElement()
	kids := elementChildren(root)
	if len(kids) != root.ChildCount() || len(kids) != 1 || kids[0].Parent() != root {
		t.Fatalf("got %d children", len(kids))
	}
	flex := findElement(t, root, "basic.Flex")
	if n := len(elementChildren(flex)); n != 3 {
		t.Errorf("flex: got %d children, want 3", n)
	}
}

func TestObjectLabel_ShowsTypeAndSize(t *testing.T) {
	ft, _ := newSample(t)
	root := ft.tree.RootRenderObject()
	if !strings.HasSuffix(describe(root), "400×300") {
		t.Errorf("the root fills the window: got %q", describe(root))
	}
	if n := len(objectChildren(root)); n != 3 || !objectHasChildren(root) {
		t.Errorf("got %d children, want 3", n)
	}
}

func TestSemIndex_MapsNodesAndObjects(t *testing.T) {
	ft, _ := newSample(t)
	x := newSemIndex(ft.tree)
	var button semKey
	for k, n := range x.nodes {
		if n.Role == semantics.RoleButton && n.Label == "OK" {
			button = k
		}
	}
	if button == rootSem {
		t.Fatal("no button node")
	}
	if got := semLabel(x.nodes[button]); got != `button "OK"` {
		t.Errorf("label: got %q", got)
	}
	owner := ownerOf(ft.tree.RootRenderObject(), button.id)
	if owner == nil {
		t.Fatal("no owner of the button node")
	}
	k, by, ok := x.of(owner)
	if !ok || k != button || by != owner {
		t.Errorf("of(owner): got %v %v %v", k, by, ok)
	}
	// A child of the owner speaks through it.
	for c := owner; c.ChildCount() > 0; {
		c = c.ChildAt(0)
		if core.SemanticsIDOf(c) != 0 {
			continue
		}
		if k, by, ok := x.of(c); !ok || k != button || by != owner {
			t.Errorf("of(child): got %v %v %v", k, by, ok)
		}
		break
	}
	// The parents lead to the root.
	p, steps := button, 0
	for p != rootSem && steps < 100 {
		p, steps = x.parent[p], steps+1
	}
	if p != rootSem {
		t.Error("the parents do not reach the root")
	}
}

func TestFindNext_WrapsAround(t *testing.T) {
	roots := []string{"a"}
	tree := map[string][]string{"a": {"b", "c"}, "b": {"b1"}, "c": {"c1"}}
	children := func(s string) []string { return tree[s] }
	label := func(s string) string { return "node " + s }
	cases := []struct {
		from, query, want string
		ok                bool
	}{
		{"", "1", "b1", true},
		{"b1", "1", "c1", true},
		{"c1", "1", "b1", true},
		{"c1", "c1", "c1", true},
		{"", "zzz", "", false},
		{"", "NODE A", "a", true},
	}
	for _, c := range cases {
		got, ok := findNext(roots, children, label, c.from, c.query)
		if got != c.want || ok != c.ok {
			t.Errorf("findNext from %q for %q: got %q %v, want %q %v", c.from, c.query, got, ok, c.want, c.ok)
		}
	}
}

func TestExpandDefault_OpensChainsAndLevels(t *testing.T) {
	// a - b - c(branch: d, e) - d - f(branch: g, h) - g - i
	tree := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"d", "e"}, "d": {"f"}, "f": {"g", "h"}, "g": {"i"}}
	opened := map[string]bool{}
	budget := 100
	expandDefault(func(s string) { opened[s] = true }, func(s string) []string { return tree[s] }, "a", 1, &budget)
	for _, s := range []string{"a", "b", "c", "d", "f"} {
		if !opened[s] {
			t.Errorf("%s is closed", s)
		}
	}
	if opened["g"] {
		t.Error("g is below the second branch and must stay closed")
	}
	if opened["e"] || opened["i"] {
		t.Error("leaves do not open")
	}
	opened = map[string]bool{}
	budget = 2
	expandDefault(func(s string) { opened[s] = true }, func(s string) []string { return tree[s] }, "a", 5, &budget)
	if len(opened) != 2 {
		t.Errorf("budget: got %d opened, want 2", len(opened))
	}
}
