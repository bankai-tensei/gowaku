package core

import "fmt"

// StateOf is a StateBase whose Widget method returns the widget as its own type
// W, so that a State reads its fields without a type assertion:
//
//	type counter struct {
//		core.StatefulBase
//		Step int
//	}
//
//	type counterState struct {
//		core.StateOf[counter]
//		n int
//	}
//
//	func (counter) CreateState() core.State { return &counterState{} }
//
//	func (s *counterState) Build(core.BuildContext) core.Widget {
//		step := s.Widget().Step // a counter, not a core.Widget
//		...
//	}
//
// W must be the type of the widget whose CreateState returns the state. The
// state checks this once, when it enters the tree, and panics there if W is
// wrong; after that Widget cannot fail. Reading the widget does not allocate.
//
// The old widget given to DidUpdateWidget stays a Widget, because the State
// interface is shared by every state. It always has type W: the reconciler
// only updates an element with a widget of the same type, so old.(W) never
// fails.
//
// The other methods, such as SetState, Context and NewTicker, come from the
// embedded StateBase, which is also reachable as the StateBase field.
type StateOf[W Widget] struct {
	StateBase
}

// attach checks that the widget of el has type W before the state is used.
func (s *StateOf[W]) attach(el *element) {
	if _, ok := el.widget.(W); !ok {
		panic(fmt.Sprintf("gowaku: %T.CreateState returned a State embedding StateOf[%s]; the type argument must be the widget type",
			el.widget, typeName[W]()))
	}
	s.el = el
}

// Widget returns the current widget of this state as a W.
func (s *StateOf[W]) Widget() W { return s.el.widget.(W) }

// typeName returns the name of W for a panic message.
func typeName[W any]() string {
	var p *W
	return fmt.Sprintf("%T", p)[1:]
}
