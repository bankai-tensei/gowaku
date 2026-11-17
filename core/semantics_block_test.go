package core

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

// blocker is a box that takes what is painted before it out of the semantics
// tree while on is set.
type blocker struct {
	on    bool
	child Widget
}

type renderBlocker struct {
	renderBox
	on bool
}

func (blocker) Key() Key { return nil }

func (b blocker) CreateRenderObject() RenderObject { return &renderBlocker{on: b.on} }

func (b blocker) UpdateRenderObject(ro RenderObject) { ro.(*renderBlocker).on = b.on }

func (b blocker) ChildWidget() Widget { return b.child }

func (r *renderBlocker) BlocksSemantics() bool { return r.on }

func button(name string) Widget { return labeled{leaf{name: name, w: 10, h: 10}} }

// labels returns the labels of the tree in reading order.
func labels(root *semantics.Node) []string {
	var out []string
	root.Walk(func(n *semantics.Node) bool {
		if n.Label != "" {
			out = append(out, n.Label)
		}
		return true
	})
	return out
}

func TestSemanticsBlocker_HidesWhatIsPaintedBefore(t *testing.T) {
	cases := []struct {
		name string
		root Widget
		want []string
	}{
		{"none blocks", list{children: []Widget{
			button("page"), blocker{child: button("dialog")},
		}}, []string{"page", "dialog"}},
		{"the page goes, what is above stays", list{children: []Widget{
			button("page"),
			box{child: list{children: []Widget{button("deep page"), blocker{on: true, child: list{children: []Widget{button("dialog")}}}, button("toast")}}},
			button("later"),
		}}, []string{"dialog", "toast", "later"}},
		{"the last one painted wins", list{children: []Widget{
			blocker{on: true, child: button("first")}, blocker{on: true, child: button("second")},
		}}, []string{"second"}},
		{"one inside another", list{children: []Widget{
			button("page"), blocker{on: true, child: list{children: []Widget{button("dialog"), blocker{on: true, child: button("inner")}}}},
		}}, []string{"inner"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(tc.root)
			h.layout()
			if got := labels(h.tree.Semantics()); !slices.Equal(got, tc.want) {
				t.Errorf("Semantics: got %q, want %q", got, tc.want)
			}
			root, _ := h.tree.SemanticsTree()
			if got := labels(root); !slices.Equal(got, tc.want) {
				t.Errorf("SemanticsTree: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSemanticsBlocker_ComesBackWhenItStops(t *testing.T) {
	page := button("page")
	h := newHarness(list{children: []Widget{page, blocker{on: true, child: button("dialog")}}})
	h.layout()
	if got := labels(h.tree.Semantics()); !slices.Equal(got, []string{"dialog"}) {
		t.Fatalf("got %q", got)
	}
	h.set(list{children: []Widget{page, blocker{child: button("dialog")}}})
	h.layout()
	if got := labels(h.tree.Semantics()); !slices.Equal(got, []string{"page", "dialog"}) {
		t.Errorf("got %q", got)
	}
}
