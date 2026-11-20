package collection

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
)

// treeRenamer is the field over the label of a row being renamed.
type treeRenamer struct {
	core.StatefulBase
	initial string
	// onCommit takes the name, and whether the field lost the keyboard to
	// something else rather than to Enter.
	onCommit func(name string, blurred bool)
	onCancel func()
}

type treeRenamerState struct {
	core.StateOf[treeRenamer]
	text string
	done bool
}

// CreateState implements core.StatefulWidget.
func (treeRenamer) CreateState() core.State { return &treeRenamerState{} }

// InitState implements core.State.
func (s *treeRenamerState) InitState() { s.text = s.Widget().initial }

// Build implements core.State.
func (s *treeRenamerState) Build(core.BuildContext) core.Widget {
	return treeAutofocus{child: interact.Focus{OnKey: s.key, Child: field.Input{
		Value:    s.Widget().initial,
		OnChange: func(v string) { s.text = v },
		OnSubmit: func(v string) { s.finish(true, v, false) },
		OnFocus: func(focused bool) {
			if !focused {
				s.finish(true, s.text, true)
			}
		},
	}}}
}

// key cancels on Escape, unless an IME is converting.
func (s *treeRenamerState) key(e *core.KeyEvent) bool {
	if e.Key != input.KeyEscape || e.Modifiers != 0 {
		return false
	}
	if r := field.FindEditable(s.Context().RenderObject()); r != nil && r.Composition != nil {
		return false
	}
	s.finish(false, "", false)
	return true
}

// finish ends the edit once, however many ways it ends.
func (s *treeRenamerState) finish(commit bool, name string, blurred bool) {
	if s.done {
		return
	}
	s.done = true
	if commit {
		s.Widget().onCommit(name, blurred)
	} else {
		s.Widget().onCancel()
	}
}

// treeAutofocus gives the keyboard to the text field inside it when it is
// first laid out, with the name selected up to its extension.
type treeAutofocus struct{ child core.Widget }

// Key implements core.Widget.
func (treeAutofocus) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (treeAutofocus) CreateRenderObject() core.RenderObject { return &renderTreeAutofocus{} }

// UpdateRenderObject implements core.RenderWidget.
func (treeAutofocus) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (a treeAutofocus) ChildWidget() core.Widget { return a.child }

type renderTreeAutofocus struct {
	layout.RenderProxy
	done bool
}

// Layout implements core.RenderObject.
func (r *renderTreeAutofocus) Layout(c geom.Constraints) geom.Size {
	sz := r.RenderProxy.Layout(c)
	if !r.done {
		r.done = true
		if e := field.FindEditable(r); e != nil {
			e.Anchor, e.Caret = 0, nameStem(e.Content)
			e.RequestFocus()
		}
	}
	return sz
}

// nameStem returns the length of the name before its extension, as Explorer
// and Finder select it for a rename: "notes.txt" selects "notes". A name
// that starts with its only dot, as ".gitignore" does, is all stem.
func nameStem(name string) int {
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		return i
	}
	return len(name)
}
