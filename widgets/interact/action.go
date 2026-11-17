package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
)

// Shortcut is a key with modifiers that runs an Action from anywhere in the
// window. The zero value is no shortcut.
//
//	Shortcut{Key: input.KeyS, Modifiers: input.ModControl} // Ctrl+S
type Shortcut struct {
	Key       input.KeyCode
	Modifiers input.Modifiers
}

// String returns the shortcut as a menu shows it: "Ctrl+Shift+S". It is "" for
// the zero value.
func (s Shortcut) String() string {
	if s.Key == input.KeyNone {
		return ""
	}
	return s.Modifiers.String() + s.Key.String()
}

// Matches reports whether e is a press of the shortcut, with exactly its
// modifiers.
func (s Shortcut) Matches(e *core.KeyEvent) bool {
	return s.Key != input.KeyNone && e.Key == s.Key && e.Modifiers == s.Modifiers
}

// Action is a command of an application: what a menu item, a shortcut and a
// toolbar button share, so that the label, the key and whether it can run now
// are said once. It is a plain value; build it where its state lives, and hand
// the same value to Actions and to the menus.
//
//	save := interact.Action{Label: "保存(&S)", Shortcut: ctrlS, OnInvoke: s.save}
//
// See docs/ADR/0046-menus-and-shortcuts.md.
type Action struct {
	// Label is what a menu shows. An & marks the next letter as the mnemonic,
	// which picks the item while its menu is open, and && is an &. Japanese
	// labels put the letter in parentheses: "ファイル(&F)".
	Label string
	// Shortcut runs the action from anywhere in the window.
	Shortcut Shortcut
	// OnInvoke runs the action. Without it the action is disabled: its menu
	// items are grayed out, and its shortcut does nothing.
	OnInvoke func()
	// Checkable makes the action an option that is on or off, which menus show
	// with a check mark while Checked is set. OnInvoke flips it.
	Checkable bool
	Checked   bool
}

// Enabled reports whether the action can run now.
func (a Action) Enabled() bool { return a.OnInvoke != nil }

// Actions runs the shortcuts of its actions when nothing in focus uses the key,
// whether the focus is inside Child or not; a modal dialog keeps them from
// running while it is open. Several Actions may be nested, and the one nearest
// to the focus wins. Repeats of a held shortcut run it again.
//
// A focused text field (field.Input, the fields built on it, and
// editor.Editor) keeps the keys it uses, and those run no shortcut:
//
//   - the keys that type, alone or with Shift, so that Space for play does
//     not run while the user types;
//   - Enter and Shift+Enter, which break the line or submit, even where they
//     do nothing;
//   - Backspace, Delete, Left, Right, Home and End with any modifier, and Up
//     and Down in a multi-line field; in an editor also Page Up, Page Down
//     and Tab;
//   - Ctrl+A, C, V, X, Y and Z; in an editor also Ctrl+F, Ctrl+H and F3, and
//     Escape while its find bar is open;
//   - every key while an IME composes.
//
// Every other key runs its shortcut: Ctrl or Alt with any other letter, a
// digit or a symbol, Ctrl or Alt with Enter (Ctrl+Enter for "commit" or
// "confirm and next"), the function keys, and Escape. On macOS, Command counts
// as Ctrl (see input.Modifiers). To give the keys a field keeps back to the
// shortcuts, take the focus off the field: a click where nothing takes the
// press does, and so does Unfocus, as from the OnSubmit of a search field or
// on Escape. Escape alone leaves the field focused.
type Actions struct {
	Actions []Action
	Child   core.Widget
}

// Key implements core.Widget.
func (Actions) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (a Actions) CreateRenderObject() core.RenderObject {
	return &layout.RenderShortcuts{OnShortcut: a.shortcut}
}

// UpdateRenderObject implements core.RenderWidget.
func (a Actions) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderShortcuts).OnShortcut = a.shortcut
}

// ChildWidget implements core.SingleChildWidget.
func (a Actions) ChildWidget() core.Widget { return a.Child }

func (a Actions) shortcut(e *core.KeyEvent) bool { return invokeShortcut(a.Actions, e) }

// Unfocus takes the keyboard away from whatever has it in the window of ctx,
// as a click on nothing does, so that the shortcuts of every Actions run
// again. Call it from a callback, not during Build. Before the first Build of
// ctx it does nothing.
func Unfocus(ctx core.BuildContext) {
	if ro, ok := ctx.RenderObject().(interface{ ClearFocus() }); ok {
		ro.ClearFocus()
	}
}

// invokeShortcut runs the first enabled action of list whose shortcut e is.
func invokeShortcut(list []Action, e *core.KeyEvent) bool {
	for i := range list {
		if list[i].Shortcut.Matches(e) && list[i].OnInvoke != nil {
			list[i].OnInvoke()
			return true
		}
	}
	return false
}
