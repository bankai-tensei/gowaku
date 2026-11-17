package core

// State holds what a StatefulWidget remembers between rebuilds. Implementations
// embed StateBase, which provides everything except Build.
type State interface {
	// Build describes the UI for the current state.
	Build(ctx BuildContext) Widget
	// InitState runs once, before the first Build.
	InitState()
	// DidUpdateWidget runs when a parent rebuild replaces the widget. Widget
	// returns the new one, and old is the previous one.
	DidUpdateWidget(old Widget)
	// Dispose runs once, when the state leaves the tree. When a whole tree
	// or subtree is torn down, the states of the parents may already be
	// unmounted when that of a child disposes, so a callback that Dispose
	// calls into a parent, such as an OnClose, must check Mounted before it
	// calls SetState there.
	Dispose()

	attach(el *element)
}

// StateBase is embedded by every State. Its lifecycle methods do nothing.
type StateBase struct {
	el *element
}

func (b *StateBase) attach(el *element) { b.el = el }

// InitState implements State.
func (b *StateBase) InitState() {}

// DidUpdateWidget implements State.
func (b *StateBase) DidUpdateWidget(Widget) {}

// Dispose implements State. It does nothing. An override runs as the state
// leaves the tree, possibly after its parents have, so it calls SetState on
// another state only if that one is still Mounted.
func (b *StateBase) Dispose() {}

// Widget returns the current widget of this state.
func (b *StateBase) Widget() Widget { return b.el.widget }

// Mounted reports whether the state is in the tree. A callback that outlives
// the state, such as one sent through Post, should check it before SetState.
func (b *StateBase) Mounted() bool { return b.el != nil && b.el.mounted }

// Context returns the BuildContext of this state.
func (b *StateBase) Context() BuildContext { return BuildContext{b.el} }

// SetState runs fn, which changes the state, and schedules a rebuild.
func (b *StateBase) SetState(fn func()) {
	if !b.Mounted() {
		panic("gowaku: SetState called on a State that is not in the tree; check Mounted first")
	}
	if b.el.tree.inBuild {
		panic("gowaku: SetState called during Build; move the call to a callback or InitState")
	}
	fn()
	b.el.markNeedsBuild()
}
