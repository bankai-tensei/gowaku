package core

import "image"

// The data of the services of the desktop that a Host may offer beyond
// core.Host: the clipboard beyond text and the file dialog. The gowaku
// package names them, and gowakutest takes them. See
// docs/ADR/0160-desktop-api-alignment.md.

// ClipboardKinds is a set of the kinds of data on the clipboard.
type ClipboardKinds uint8

// The kinds of clipboard data.
const (
	ClipboardText ClipboardKinds = 1 << iota
	// ClipboardHTML is a fragment of HTML, as a browser copies it.
	ClipboardHTML
	ClipboardImage
	// ClipboardFiles is a list of files, as a file manager copies them.
	ClipboardFiles
)

// ClipboardContent is what the clipboard holds, in the forms gowaku knows. A
// zero field is a kind that is not there.
type ClipboardContent struct {
	Text string
	// HTML is a fragment, without the html and body elements.
	HTML  string
	Image *image.NRGBA
	// Files are absolute paths.
	Files []string
}

// Kinds returns the kinds that c holds.
func (c *ClipboardContent) Kinds() ClipboardKinds {
	var k ClipboardKinds
	if c.Text != "" {
		k |= ClipboardText
	}
	if c.HTML != "" {
		k |= ClipboardHTML
	}
	if c.Image != nil {
		k |= ClipboardImage
	}
	if len(c.Files) > 0 {
		k |= ClipboardFiles
	}
	return k
}

// FileDialog configures the file dialog of the system. The zero value opens one
// existing file.
type FileDialog struct {
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

// FileFilter is a choice of file types in a FileDialog, such as
// {Name: "Images", Patterns: []string{"*.png", "*.jpg"}}.
type FileFilter struct {
	// Name describes the files, such as "Images".
	Name string
	// Patterns are globs such as "*.png".
	Patterns []string
}

// FileDialogHost is a Host that shows file dialogs: a window of gowaku.Run,
// and a gowakutest.Tester. A Host need not implement it.
type FileDialogHost interface {
	// ShowFileDialog shows d and later calls done on the UI goroutine with the
	// chosen paths, or nil if the user canceled.
	ShowFileDialog(d FileDialog, done func(paths []string, err error))
}
