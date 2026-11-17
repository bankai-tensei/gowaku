package core

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

// reentry is what the boxes of a focus re-entrancy test share: their log, the
// objects by name, and what a FocusChanged does besides logging.
type reentry struct {
	log []string
	// quiet keeps the log empty, for counting allocations.
	quiet   bool
	objects map[string]*reentryObject
	onFocus func(name string, focused bool)
}

// reentryBox is a box around one child. With focus it takes the keyboard, and
// otherwise it listens for the focus below it.
type reentryBox struct {
	StatelessBase
	name  string
	focus bool
	env   *reentry
	child Widget
}

func (b reentryBox) CreateRenderObject() RenderObject {
	ro := &reentryObject{w: b}
	b.env.objects[b.name] = ro
	return ro
}

func (b reentryBox) UpdateRenderObject(ro RenderObject) { ro.(*reentryObject).w = b }

func (b reentryBox) ChildWidget() Widget { return b.child }

type reentryObject struct {
	RenderBase
	w     reentryBox
	child RenderObject
}

func (r *reentryObject) SetChild(c RenderObject) { r.child = c }

func (r *reentryObject) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

func (r *reentryObject) ChildAt(int) RenderObject { return r.child }

func (r *reentryObject) Layout(c geom.Constraints) geom.Size {
	if r.child != nil {
		LayoutChild(r.child, c.Loosen())
	}
	return c.Biggest()
}

func (r *reentryObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *reentryObject) CanFocus() bool { return r.w.focus }

func (r *reentryObject) HandleKey(*KeyEvent) bool { return false }

func (r *reentryObject) HandleText(string) {}

func (r *reentryObject) HandleComposition(*input.Composition) {}

func (r *reentryObject) FocusChanged(focused bool) {
	if !r.w.focus {
		panic("a listener is no focus target")
	}
	switch {
	case r.w.env.quiet:
	case focused:
		r.w.env.log = append(r.w.env.log, r.w.name+":focus")
	default:
		r.w.env.log = append(r.w.env.log, r.w.name+":blur")
	}
	if r.w.env.onFocus != nil {
		r.w.env.onFocus(r.w.name, focused)
	}
}

func (r *reentryObject) FocusWithinChanged(within bool) {
	if r.w.focus || r.w.env.quiet {
		return
	}
	if within {
		r.w.env.log = append(r.w.env.log, r.w.name+":in")
	} else {
		r.w.env.log = append(r.w.env.log, r.w.name+":out")
	}
}

// reentryTree is three focus targets, each below a listener of its own:
// wa > a > wb > b > wc > c.
func reentryTree(t *testing.T) (*Tree, *reentry) {
	t.Helper()
	env := &reentry{objects: map[string]*reentryObject{}}
	box := func(name string, focus bool, child Widget) Widget {
		return reentryBox{name: name, focus: focus, env: env, child: child}
	}
	root := box("wa", false, box("a", true, box("wb", false, box("b", true, box("wc", false, box("c", true, nil))))))
	return inputTree(t, root), env
}

// checkFocusPaired fails if any object heard the same news twice in a row, or
// ends up believing something else than the tree says.
func checkFocusPaired(t *testing.T, tree *Tree, env *reentry) {
	t.Helper()
	state := map[string]bool{}
	for _, l := range env.log {
		for _, on := range []string{":focus", ":in"} {
			if name, ok := strings.CutSuffix(l, on); ok {
				if state[name] {
					t.Errorf("%s heard %s twice in a row: %s", name, on, joined(env.log))
				}
				state[name] = true
			}
		}
		for _, off := range []string{":blur", ":out"} {
			if name, ok := strings.CutSuffix(l, off); ok {
				if !state[name] {
					t.Errorf("%s heard %s without the other first: %s", name, off, joined(env.log))
				}
				state[name] = false
			}
		}
	}
	for name, ro := range env.objects {
		var want bool
		if ro.w.focus {
			want = tree.Focused() == FocusTarget(ro) && tree.WindowFocused()
		} else {
			want = tree.Focused() != nil && isAncestor(ro, tree.Focused())
		}
		if state[name] != want {
			t.Errorf("%s believes %v, and the tree says %v: %s", name, state[name], want, joined(env.log))
		}
	}
}

func TestFocus_BlurThatMovesTheFocusWins(t *testing.T) {
	tree, env := reentryTree(t)
	a, b, c := env.objects["a"], env.objects["b"], env.objects["c"]
	tree.RequestFocus(a)
	env.onFocus = func(name string, focused bool) {
		if name == "a" && !focused {
			tree.RequestFocus(c)
		}
	}
	tree.RequestFocus(b)
	if tree.Focused() != FocusTarget(c) {
		t.Fatal("the focus must be where the callback put it")
	}
	// b was only passed through: it hears nothing, and neither does wb twice.
	if got, want := joined(env.log), "a:focus wa:in a:blur c:focus wc:in wb:in"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	checkFocusPaired(t, tree, env)
}

func TestFocus_FocusThatMovesTheFocusIsToldItLeft(t *testing.T) {
	tree, env := reentryTree(t)
	b, c := env.objects["b"], env.objects["c"]
	env.onFocus = func(name string, focused bool) {
		if name == "c" && focused {
			tree.RequestFocus(b)
		}
	}
	tree.RequestFocus(c)
	if tree.Focused() != FocusTarget(b) {
		t.Fatal("the focus must be where the callback put it")
	}
	if got, want := joined(env.log), "c:focus c:blur b:focus wb:in wa:in"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	checkFocusPaired(t, tree, env)
}

func TestFocus_BlurThatTakesTheFocusBackKeepsIt(t *testing.T) {
	tree, env := reentryTree(t)
	a, b := env.objects["a"], env.objects["b"]
	tree.RequestFocus(a)
	env.onFocus = func(name string, focused bool) {
		if name == "a" && !focused {
			tree.RequestFocus(a)
		}
	}
	tree.RequestFocus(b)
	if tree.Focused() != FocusTarget(a) {
		t.Fatal("the focus must be where the callback put it")
	}
	if got, want := joined(env.log), "a:focus wa:in a:blur a:focus"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	checkFocusPaired(t, tree, env)
}

func TestFocus_WindowBlurThatMovesTheFocus(t *testing.T) {
	tree, env := reentryTree(t)
	a, c := env.objects["a"], env.objects["c"]
	tree.RequestFocus(a)
	env.onFocus = func(name string, focused bool) {
		switch {
		case name == "a" && !focused:
			tree.RequestFocus(c)
		case name == "c" && focused:
			// Losing the window again from the news of getting it back.
			tree.SetWindowFocused(false)
		}
	}
	tree.SetWindowFocused(false)
	if got, want := joined(env.log), "a:focus wa:in a:blur wc:in wb:in"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	checkFocusPaired(t, tree, env)
	tree.SetWindowFocused(true)
	if tree.WindowFocused() {
		t.Fatal("the window must be as the callback left it")
	}
	if got, want := joined(env.log), "a:focus wa:in a:blur wc:in wb:in c:focus c:blur"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	checkFocusPaired(t, tree, env)
}

func TestFocus_RequestDoesNotAllocate(t *testing.T) {
	tree, env := reentryTree(t)
	a, b, c := FocusTarget(env.objects["a"]), FocusTarget(env.objects["b"]), FocusTarget(env.objects["c"])
	env.quiet = true
	env.onFocus = func(name string, focused bool) {
		if name == "a" && !focused {
			tree.RequestFocus(c)
		}
	}
	allocs := testing.AllocsPerRun(100, func() {
		tree.RequestFocus(a)
		tree.RequestFocus(b)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}
