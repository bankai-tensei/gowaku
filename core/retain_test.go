package core

import "testing"

// buildCounter counts its builds.
type buildCounter struct {
	StatelessBase
	n *int
}

func (b buildCounter) Build(BuildContext) Widget {
	*b.n++
	return leaf{w: 1, h: 1}
}

// heldBody is a widget held by pointer.
type heldBody struct {
	StatelessBase
	child Widget
}

func (b *heldBody) Build(BuildContext) Widget { return b.child }

// holder rebuilds to whatever body points at.
type holder struct {
	StatefulBase
	body **heldBody
	st   **holderState
}

type holderState struct{ StateBase }

func (h holder) CreateState() State {
	s := &holderState{}
	*h.st = s
	return s
}

func (s *holderState) Build(BuildContext) Widget { return *s.Widget().(holder).body }

func TestUpdate_SamePointerWidgetIsLeftAlone(t *testing.T) {
	var builds int
	body := &heldBody{child: buildCounter{n: &builds}}
	var st *holderState
	tree := NewTree(holder{body: &body, st: &st})
	if builds != 1 {
		t.Fatalf("built %d times, want 1", builds)
	}
	st.SetState(func() {})
	tree.Build()
	if builds != 1 {
		t.Errorf("the same pointer rebuilt what is below it: %d builds", builds)
	}
	body = &heldBody{child: buildCounter{n: &builds}}
	st.SetState(func() {})
	tree.Build()
	if builds != 2 {
		t.Errorf("a new pointer did not rebuild: %d builds", builds)
	}
}
