package gowaku

import "github.com/bankai-tensei/gowaku/core"

// The types that every UI file writes, so that a widget of the application
// needs no import of package core. The widgets themselves are in the packages
// under widgets; see the package documentation.
type (
	// Widget is core.Widget.
	Widget = core.Widget
	// Key is core.Key.
	Key = core.Key
	// BuildContext is core.BuildContext.
	BuildContext = core.BuildContext
	// State is core.State.
	State = core.State
	// StateBase is core.StateBase.
	StateBase = core.StateBase
	// StateOf is core.StateOf: a StateBase whose Widget returns a W.
	StateOf[W Widget] = core.StateOf[W]
	// StatelessBase is core.StatelessBase.
	StatelessBase = core.StatelessBase
	// StatefulBase is core.StatefulBase.
	StatefulBase = core.StatefulBase
)

// Keyed returns w with the identity key. It panics if key is not comparable.
func Keyed(key Key, w Widget) Widget { return core.Keyed(key, w) }
