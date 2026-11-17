package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

// motionProbe records ReduceMotion at each build.
type motionProbe struct {
	core.StatelessBase
	seen *[]bool
}

func (m motionProbe) Build(ctx core.BuildContext) core.Widget {
	*m.seen = append(*m.seen, basic.ReduceMotionOf(ctx))
	return basic.SizedBox{}
}

func findLabel(n *semantics.Node, label string) *semantics.Node {
	var found *semantics.Node
	n.Walk(func(m *semantics.Node) bool {
		if m.Label == label {
			found = m
			return false
		}
		return true
	})
	return found
}

func TestRun_Accessibility(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	presses := 0
	var motion []bool
	a11y := func() *fakeA11y { return p.windows[1].a11y }
	var save semantics.ID
	p, b = newFakes(t,
		func() {
			if a11y().updates != 0 {
				t.Fatal("nothing listens yet, so no tree must be built")
			}
			a11y().listening = true
			p.send(platform.Event{Kind: platform.EventRedraw, Window: 1})
		},
		func() {
			a := a11y()
			if a.updates != 1 {
				t.Fatalf("got %d updates after the first question, want 1", a.updates)
			}
			n := findLabel(a.root, "Save")
			if n == nil || n.Role != semantics.RoleButton || n.ID == 0 {
				t.Fatalf("no button Save in the tree: %+v", n)
			}
			if n.State&semantics.Focusable == 0 || n.Actions&semantics.ActionTap == 0 {
				t.Errorf("the button must be focusable and tappable: state %b actions %b", n.State, n.Actions)
			}
			if len(a.diff.Added) == 0 {
				t.Error("the first tree must come as added nodes")
			}
			save = n.ID
			p.send(platform.Event{Kind: platform.EventAccessibilityAction, Window: 1, Node: save, Action: semantics.ActionTap})
			p.send(platform.Event{Kind: platform.EventAccessibilityAction, Window: 1, Node: save, Action: semantics.ActionFocus})
		},
		func() {
			if presses != 1 {
				t.Errorf("Invoke from the reader pressed %d times, want 1", presses)
			}
			a := a11y()
			if a.focus != save || a.focusMoves != 1 {
				t.Errorf("focus %d (moved %d times), want the button %d", a.focus, a.focusMoves, save)
			}
			if n := findLabel(a.root, "Save"); n == nil || n.ID != save || n.State&semantics.Focused == 0 {
				t.Errorf("the button must keep its ID and show the focus: %+v", n)
			}
			p.reduceMotion = true
			p.send(platform.Event{Kind: platform.EventSettings})
		},
		func() { p.send(platform.Event{Kind: platform.EventClose, Window: 1}) },
	)
	p.a11y = &fakeA11y{}
	home := basic.Column{Children: []core.Widget{
		control.Button{Label: "Save", OnPress: func() { presses++ }},
		motionProbe{seen: &motion},
	}}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
	if len(motion) < 2 || motion[0] || !motion[len(motion)-1] {
		t.Errorf("ReduceMotion seen as %v, want false then true", motion)
	}
}
