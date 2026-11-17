package platform

import "github.com/bankai-tensei/gowaku/input"

// MenuItem is an entry of a menu bar that the window system shows itself.
// See docs/ADR/0083-macos-integration.md.
type MenuItem struct {
	// ID is what the handler of SetMenuBar hears when the item is chosen.
	ID int
	// Label may mark its mnemonic with &.
	Label     string
	Key       input.KeyCode
	Modifiers input.Modifiers
	Enabled   bool
	Checkable bool
	Checked   bool
	Separator bool
	// Items make the item a submenu.
	Items []MenuItem
}

// MenuBarWindow is a Window whose window system shows the menus of the
// application, as the menu bar of macOS. A Window need not implement it.
type MenuBarWindow interface {
	// SetMenuBar replaces the menus shown while the window is active. on
	// hears the ID of a chosen item, on the UI goroutine, inside WaitEvent.
	SetMenuBar(items []MenuItem, on func(id int))
}
