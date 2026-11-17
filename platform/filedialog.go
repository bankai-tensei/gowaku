package platform

import "errors"

// ErrNoFileDialog means that the system has no file dialog to show, as on a
// Linux desktop without xdg-desktop-portal.
var ErrNoFileDialog = errors.New("gowaku: the system has no file dialog")

// FileDialogOptions configures Window.ShowFileDialog. The zero value opens one
// existing file.
type FileDialogOptions struct {
	Title string
	// Save asks for a file to write, which may not exist yet. Name suggests its
	// name.
	Save bool
	Name string
	// Multiple lets the user choose several files to open.
	Multiple bool
	// Directory chooses folders instead of files.
	Directory bool
	// Folder is where the dialog starts. "" leaves it to the system.
	Folder  string
	Filters []FileFilter
}

// FileFilter is a choice of the file types a dialog shows.
type FileFilter struct {
	// Name describes the files, such as "Images".
	Name string
	// Patterns are globs such as "*.png".
	Patterns []string
}
