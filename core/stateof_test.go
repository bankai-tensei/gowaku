package core

import (
	"strings"
	"testing"
)

// typedWidget is a stateful widget whose state reads it through StateOf.
type typedWidget struct {
	StatefulBase
	name string
	log  *[]string
}

type typedState struct {
	StateOf[typedWidget]
}

func (typedWidget) CreateState() State { return &typedState{} }

func (s *typedState) InitState() {
	w := s.Widget()
	*w.log = append(*w.log, "init:"+w.name)
}

func (s *typedState) DidUpdateWidget(old Widget) {
	w := s.Widget()
	*w.log = append(*w.log, "update:"+old.(typedWidget).name+"->"+w.name)
}

func (s *typedState) Build(BuildContext) Widget {
	w := s.Widget()
	*w.log = append(*w.log, "build:"+w.name)
	return leaf{name: w.name, w: 1, h: 1}
}

// stateBelow returns the first state of type S under the root of h.
func stateBelow[S State](t testing.TB, h *harness) S {
	t.Helper()
	for el := h.tree.root; el != nil; el = el.child {
		if s, ok := el.state.(S); ok {
			return s
		}
	}
	t.Fatalf("no %T in the tree", *new(S))
	return *new(S)
}

func TestStateOf_WidgetIsTyped(t *testing.T) {
	var got []string
	h := newHarness(typedWidget{name: "a", log: &got})
	expectLog(t, &got, "init:a", "build:a")
	h.set(typedWidget{name: "b", log: &got})
	expectLog(t, &got, "update:a->b", "build:b")
	if name := h.rootRO().(*renderLeaf).name; name != "b" {
		t.Errorf("leaf name = %q, want b", name)
	}
}

func TestStateOf_StateBaseStillWorks(t *testing.T) {
	var got []string
	h := newHarness(typedWidget{name: "a", log: &got})
	s := stateBelow[*typedState](t, h)
	if w, ok := s.StateBase.Widget().(typedWidget); !ok || w.name != "a" {
		t.Errorf("StateBase.Widget() = %#v, want typedWidget a", s.StateBase.Widget())
	}
	if !s.Mounted() {
		t.Error("state is not mounted")
	}
}

// wrongWidget creates a state that names another widget type.
type wrongWidget struct{ StatefulBase }

func (wrongWidget) CreateState() State { return &typedState{} }

func TestStateOf_WrongTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		msg, _ := r.(string)
		if !strings.HasPrefix(msg, "gowaku: core.wrongWidget.CreateState") || !strings.Contains(msg, "StateOf[core.typedWidget]") {
			t.Errorf("panic = %v", r)
		}
	}()
	newHarness(wrongWidget{})
	t.Error("no panic")
}

// bigWidget is larger than a pointer, so reading it copies it out of the
// interface.
type bigWidget struct {
	StatefulBase
	a, b, c, d float64
	text       string
}

type bigState struct {
	StateOf[bigWidget]
	// child is boxed once, so that Build itself allocates nothing.
	child Widget
}

func (bigWidget) CreateState() State { return &bigState{} }

func (s *bigState) InitState() { s.child = leaf{w: float32(s.Widget().a), h: 1} }

func (s *bigState) Build(BuildContext) Widget {
	if s.Widget().text == "" {
		panic("bigState: lost its widget")
	}
	return s.child
}

func (s *bigState) sum() float64 {
	w := s.Widget()
	return w.a + w.b + w.c + w.d + float64(len(w.text))
}

func (s *bigState) assertSum() float64 {
	w := s.StateBase.Widget().(bigWidget)
	return w.a + w.b + w.c + w.d + float64(len(w.text))
}

func newBig() *harness { return newHarness(bigWidget{a: 1, b: 2, c: 3, d: 4, text: "hello"}) }

func TestStateOf_WidgetDoesNotAllocate(t *testing.T) {
	s := stateBelow[*bigState](t, newBig())
	var sum float64
	if allocs := testing.AllocsPerRun(100, func() { sum += s.sum() }); allocs != 0 {
		t.Errorf("Widget allocated %v times per call", allocs)
	}
	if sum != 101*15 {
		t.Errorf("sum = %v, want %v", sum, 101*15)
	}
}

var sinkFloat float64

func BenchmarkStateOf_Widget(b *testing.B) {
	s := stateBelow[*bigState](b, newBig())
	b.ReportAllocs()
	for b.Loop() {
		sinkFloat += s.sum()
	}
}

// BenchmarkStateBase_WidgetAssert is the type assertion that StateOf replaces,
// for comparison.
func BenchmarkStateBase_WidgetAssert(b *testing.B) {
	s := stateBelow[*bigState](b, newBig())
	b.ReportAllocs()
	for b.Loop() {
		sinkFloat += s.assertSum()
	}
}

// BenchmarkStateOf_Rebuild rebuilds a state that reads its widget in Build.
func BenchmarkStateOf_Rebuild(b *testing.B) {
	h := newBig()
	h.layout()
	s := stateBelow[*bigState](b, h)
	b.ReportAllocs()
	for b.Loop() {
		s.SetState(func() {})
		h.tree.Build()
	}
}
