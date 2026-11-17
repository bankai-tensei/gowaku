// Package store keeps what the notes sample saves: the text files, the
// settings, the state of each file, the recent files, the open windows and
// the autosaved text that a crash would otherwise lose. It knows nothing of
// the UI, so that it is tested without a window.
package store

import "time"

// Family is the kind of typeface the editor uses.
type Family uint8

// The families. The zero value is mincho, the face of reading.
const (
	Mincho Family = iota
	Gothic
)

// String returns the name of the family as the settings file writes it.
func (f Family) String() string {
	switch f {
	case Mincho:
		return "mincho"
	case Gothic:
		return "gothic"
	}
	return "unknown"
}

// Settings are the preferences of the application.
type Settings struct {
	// FontSize is the size of the text in logical pixels. Zero selects 18.
	FontSize float32
	Family   Family
	// Library is the folder of the notebook. Empty selects Documents/gowaku
	// notes in the home directory.
	Library string
	// AutosaveEvery is how often unsaved text goes to the recovery folder.
	// Zero selects five seconds.
	AutosaveEvery time.Duration
}

// FileState is what the application remembers of a file between sessions.
type FileState struct {
	// Vertical writes the file top to bottom, right to left.
	Vertical bool
	// Caret is the byte offset of the caret in the text.
	Caret int
}

// Session is the windows open at the last save, oldest first.
type Session struct {
	Windows []WindowState
}

// WindowState is one window of a session.
type WindowState struct {
	Tabs []TabState
	// Active is the index of the tab shown.
	Active int
}

// TabState is one tab of a window: a file, or an untitled text, with the key
// of its autosaved text if it has unsaved changes.
type TabState struct {
	// Path is the file, or empty for an untitled text.
	Path string
	// Recovery is the key of the autosaved text, or empty when the tab was
	// saved.
	Recovery string
}

// Doc is a text file as read from disk, with what writing it back must keep.
type Doc struct {
	Path string
	// Text has LF line ends and no byte order mark.
	Text string
	// CRLF and BOM say how the file was written, so that saving keeps it.
	CRLF, BOM bool
}

// Note is a file of the notebook.
type Note struct {
	Path string
	// Title is the first line that is not empty, with its ruby notation
	// taken out, or the file name when there is none.
	Title    string
	Modified time.Time
	// Preview is the start of the text after the title, on one line.
	Preview string
}

// Hit is a place where a search matched.
type Hit struct {
	Note Note
	// Offset is the byte offset of the match in the text, and Line its line,
	// counted from 0.
	Offset, Line int
	// Before, Match and After are the line of the match cut around it, for
	// a result card.
	Before, Match, After string
}

// Library is the notebook: the .txt files of a folder, its subfolders
// included.
type Library struct {
	Dir string
}
