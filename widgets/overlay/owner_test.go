package overlay

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// entryListen counts the times the focus leaves an entry opened from below it.
type entryListen struct {
	count *int
	child core.Widget
}

func (entryListen) Key() core.Key { return nil }

func (l entryListen) CreateRenderObject() core.RenderObject {
	return &renderEntryListen{count: l.count}
}

func (entryListen) UpdateRenderObject(core.RenderObject) {}

func (l entryListen) ChildWidget() core.Widget { return l.child }

type renderEntryListen struct {
	layout.RenderProxy
	count *int
}

func (r *renderEntryListen) EntryFocusLeft() { *r.count++ }

var _ EntryFocusListener = (*renderEntryListen)(nil)

// focusBox is a place that takes the focus, whose context lands in got.
func focusBox(got *core.BuildContext) core.Widget {
	return wtest.Capture{Got: got, Child: interact.Focus{Enabled: true, Child: basic.SizedBox{Width: 10, Height: 10}}}
}

func TestContains_FollowsEntriesToTheirPlaces(t *testing.T) {
	var region, other, in, deep, toast, beside core.BuildContext
	left := 0
	tree := wtest.TextTree(t, Overlay{Child: basic.Row{Children: []core.Widget{
		entryListen{count: &left, child: focusBox(&region)},
		focusBox(&other),
	}}})
	wtest.Ops(tree, wtest.Screen)
	host := Of(region)
	host.InsertFrom(region, func() core.Widget { return focusBox(&in) })
	host.Insert(func() core.Widget { return focusBox(&toast) })
	host.InsertFrom(other, func() core.Widget { return focusBox(&beside) })
	wtest.Ops(tree, wtest.Screen)
	host.InsertFrom(in, func() core.Widget { return focusBox(&deep) })
	wtest.Ops(tree, wtest.Screen)

	r := region.RenderObject()
	for _, tt := range []struct {
		name string
		ro   core.RenderObject
		want bool
	}{
		{"自分", r, true},
		{"開いたエントリー", in.RenderObject(), true},
		{"そのエントリーが開いたもの", deep.RenderObject(), true},
		{"場所の無いエントリー", toast.RenderObject(), false},
		{"よそから開いたエントリー", beside.RenderObject(), false},
		{"よそ", other.RenderObject(), false},
		{"nil", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(r, tt.ro); got != tt.want {
				t.Errorf("got %v", got)
			}
		})
	}
	if Contains(nil, r) {
		t.Error("nil holds nothing")
	}

	// The focus leaving an entry of the region tells the listener above the
	// place, also from the entry that entry opened, but not from others.
	focus := func(ctx core.BuildContext) {
		tree.RequestFocus(kit.FirstFocusable(ctx.RenderObject()))
	}
	focus(deep)
	focus(in)
	if left != 1 {
		t.Errorf("leaving the deep entry for the one that opened it, got %d", left)
	}
	focus(other)
	if left != 2 {
		t.Errorf("leaving the entry for elsewhere, got %d", left)
	}
	focus(beside)
	focus(toast)
	tree.RequestFocus(nil)
	if left != 2 {
		t.Errorf("leaving entries of other places, got %d", left)
	}
}
