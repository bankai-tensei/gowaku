package field

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// fieldRig drives the fields of a form through a window with an overlay, as
// Run builds it.
type fieldRig struct {
	T    testing.TB
	tree *core.Tree
	host *wtest.FakeHost
	Size geom.Size
	now  time.Duration
}

func newFieldRig(t testing.TB, w core.Widget) *fieldRig {
	t.Helper()
	r := &fieldRig{T: t, host: &wtest.FakeHost{}, Size: geom.Size{W: 500, H: 400}, now: time.Second}
	r.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Padding{Insets: geom.All(20), Child: basic.Align{Alignment: layout.AlignTopLeft, Child: w}}})
	r.tree.SetHost(r.host)
	r.frame()
	return r
}

func (r *fieldRig) frame() string { return testutil.RenderTree(r.tree, r.Size) }

// settle plays frames until nothing moves.
func (r *fieldRig) settle() { wtest.Settled(r.T, r.tree, r.Size, &r.now) }

// wait lets time pass, as an idle window does.
func (r *fieldRig) wait(d time.Duration) {
	r.now += d
	r.tree.Tick(r.now)
	r.frame()
}

func (r *fieldRig) Key(k input.KeyCode, mods input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	r.frame()
}

func (r *fieldRig) typeText(s string) {
	r.tree.DispatchText(s)
	r.frame()
}

func (r *fieldRig) compose(c *input.Composition) {
	r.tree.DispatchComposition(c)
	r.frame()
}

func (r *fieldRig) root() *semantics.Node {
	root, _ := r.tree.SemanticsTree()
	return root
}

// find returns the first node of role, with label unless it is "".
func (r *fieldRig) Find(role semantics.Role, label string) *semantics.Node {
	var found *semantics.Node
	r.root().Walk(func(n *semantics.Node) bool {
		if n.Role == role && (label == "" || n.Label == label) {
			found = n
		}
		return found == nil
	})
	return found
}

// must is find that fails the test without a node.
func (r *fieldRig) must(role semantics.Role, label string) *semantics.Node {
	r.T.Helper()
	n := r.Find(role, label)
	if n == nil {
		r.T.Fatalf("no %v node %q", role, label)
	}
	return n
}

func (r *fieldRig) all(role semantics.Role) []*semantics.Node {
	var out []*semantics.Node
	r.root().Walk(func(n *semantics.Node) bool {
		if n.Role == role {
			out = append(out, n)
		}
		return true
	})
	return out
}

// focused returns the node of what has the keyboard, or nil.
func (r *fieldRig) focused() *semantics.Node {
	root, focus := r.tree.SemanticsTree()
	return root.Find(focus)
}

func (r *fieldRig) wantFocus(role semantics.Role, label string) {
	r.T.Helper()
	n := r.focused()
	if n == nil || n.Role != role || label != "" && n.Label != label {
		r.T.Fatalf("focus on %+v, want %v %q", n, role, label)
	}
}

func (r *fieldRig) click(n *semantics.Node) {
	r.T.Helper()
	if n == nil {
		r.T.Fatal("click on no node")
	}
	wtest.Tap(r.tree, wtest.Center(n.Rect))
	r.frame()
}

func (r *fieldRig) act(n *semantics.Node, a semantics.Actions) {
	r.T.Helper()
	if n == nil || !r.tree.PerformSemanticsAction(n.ID, a, core.SemanticsArg{}) {
		r.T.Fatalf("action %v on %+v failed", a, n)
	}
	r.frame()
}

// focusField gives the keyboard to the text field labeled label.
func (r *fieldRig) focusField(label string) {
	r.T.Helper()
	r.act(r.must(semantics.RoleTextField, label), semantics.ActionFocus)
}

// editing returns the text field that has the keyboard.
func (r *fieldRig) editing() *RenderEditable {
	r.T.Helper()
	e, ok := r.tree.Focused().(*RenderEditable)
	if !ok {
		r.T.Fatalf("the focus is on %T, not a text field", r.tree.Focused())
	}
	return e
}

// formHolder keeps a value for the widget that build makes, as the State of
// an application does.
type formHolder[T any] struct {
	core.StatefulBase
	Initial T
	Build   func(v T, set func(T)) core.Widget
}

type formHolderState[T any] struct {
	core.StateOf[formHolder[T]]
	v T
}

func (formHolder[T]) CreateState() core.State { return &formHolderState[T]{} }

func (s *formHolderState[T]) InitState() { s.v = s.Widget().Initial }

func (s *formHolderState[T]) Build(core.BuildContext) core.Widget {
	return s.Widget().Build(s.v, func(v T) { s.SetState(func() { s.v = v }) })
}
