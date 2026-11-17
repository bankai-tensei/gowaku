package platform

import (
	"errors"
	"image"
)

// ErrNoTray reports that the desktop shows no icons of applications, such as
// GNOME without an extension for them. ErrNoNotifications reports that it
// shows no notifications.
var (
	ErrNoTray          = errors.New("gowaku: the desktop has no tray")
	ErrNoNotifications = errors.New("gowaku: the desktop shows no notifications")
)

// TrayItem is an entry of the menu of a tray icon. See
// docs/ADR/0051-tray-and-notifications.md.
type TrayItem struct {
	// ID is what TrayEvent reports when the item is chosen.
	ID int
	// Label may mark its mnemonic with &, as an Action does.
	Label string
	// Shortcut is shown beside the label, as "Ctrl+Q".
	Shortcut  string
	Enabled   bool
	Checkable bool
	Checked   bool
	Separator bool
	// Items make the item a submenu.
	Items []TrayItem
}

// TrayOptions is what a tray icon shows.
type TrayOptions struct {
	Icon    *image.NRGBA
	Tooltip string
	Items   []TrayItem
}

// TrayEventKind says what the user did to a tray icon.
type TrayEventKind uint8

// The tray events.
const (
	// TrayActivate is a click on the icon, or Enter on it from the keyboard.
	TrayActivate TrayEventKind = iota
	// TrayChoose is the choice of an item of its menu.
	TrayChoose
	// TrayNotificationClick is a click on a notification of the application.
	TrayNotificationClick
)

// TrayEvent is something the user did to a tray icon.
type TrayEvent struct {
	Kind TrayEventKind
	// Item is the ID of the chosen item.
	Item int
}

// Tray is an icon of the application in the tray of the desktop.
type Tray interface {
	// Update replaces what the icon shows.
	Update(opts TrayOptions)
	// Close takes the icon away. Closing twice is a no-op.
	Close()
}

// Trays are the tray icons and notifications of the desktop. A Platform
// implements it.
type Trays interface {
	// CreateTray shows an icon in the tray. on hears what the user does to it,
	// on any goroutine. err is ErrNoTray where the desktop has no tray.
	CreateTray(opts TrayOptions, on func(TrayEvent)) (Tray, error)
	// Notify shows a notification of the application. A click on it reaches
	// the on of the tray it came from, or of the first tray. err is
	// ErrNoNotifications where the desktop shows none.
	Notify(title, body string) error
}
