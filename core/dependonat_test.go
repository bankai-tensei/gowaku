package core

import "testing"

func TestDependOnAt_FollowsAnotherBranch(t *testing.T) {
	var at BuildContext
	var source *testState
	value := 1
	// One branch holds the value; the reader sits in another, under a value
	// of its own that it must not see.
	holder := stateful{build: func(s *testState, _ BuildContext) Widget {
		source = s
		return Inherited[int]{Value: value, Child: stateful{build: func(_ *testState, ctx BuildContext) Widget {
			at = ctx
			return leaf{}
		}}}
	}}
	var seen []int
	reader := stateful{build: func(_ *testState, ctx BuildContext) Widget {
		v, ok := DependOnAt[int](ctx, at)
		if !ok {
			t.Error("value not found")
		}
		seen = append(seen, v)
		return leaf{}
	}}
	tree := NewTree(list{children: []Widget{holder, Inherited[int]{Value: 100, Child: reader}}})
	if len(seen) != 1 || seen[0] != 1 {
		t.Fatalf("got %v, want [1]", seen)
	}
	source.SetState(func() { value = 2 })
	tree.Build()
	if len(seen) != 2 || seen[1] != 2 {
		t.Errorf("got %v, want [1 2]: the reader must rebuild with the other branch", seen)
	}
}

func TestDependOnAt_MissingReportsFalse(t *testing.T) {
	var at BuildContext
	NewTree(list{children: []Widget{
		stateful{build: func(_ *testState, ctx BuildContext) Widget { at = ctx; return leaf{} }},
		stateful{build: func(_ *testState, ctx BuildContext) Widget {
			if _, ok := DependOnAt[int](ctx, at); ok {
				t.Error("got true, want false")
			}
			return leaf{}
		}},
	}})
}
