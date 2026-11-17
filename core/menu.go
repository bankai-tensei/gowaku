package core

import "github.com/bankai-tensei/gowaku/input"

// NativeMenuHost is a Host whose window system shows the menus of the
// application itself, as macOS does in the menu bar at the top of the screen.
// A Host need not implement it. See docs/ADR/0083-macos-integration.md.
type NativeMenuHost interface {
	// SetNativeMenu replaces the menus that the window system shows for the
	// window with menu, nil for none. It reports false where the window system
	// shows no menus of applications, and the application draws its own.
	SetNativeMenu(menu []NativeMenuItem) bool
}

// NativeMenuItem is an entry of a menu that the window system shows.
type NativeMenuItem struct {
	// Label may mark its mnemonic with &, as interact.Action does; a menu
	// that has no mnemonics drops it.
	Label string
	// Key and Modifiers are the shortcut, which the menu shows and runs.
	Key       input.KeyCode
	Modifiers input.Modifiers
	Enabled   bool
	Checkable bool
	Checked   bool
	Separator bool
	// Items make the item a submenu.
	Items []NativeMenuItem
	// OnInvoke runs when the user chooses the item, on the UI goroutine.
	OnInvoke func()
}

// Host returns the window services of the tree of ctx, which are never nil.
func (ctx BuildContext) Host() Host {
	if ctx.el == nil || ctx.el.tree == nil {
		return noHost{}
	}
	return ctx.el.tree.Host()
}
