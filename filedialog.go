package gowaku

import "github.com/bankai-tensei/gowaku/platform"

// FileDialog configures the file dialog of the system. The zero value opens one
// existing file. See ShowFileDialog.
type FileDialog = platform.FileDialogOptions

// FileFilter is a choice of file types in a FileDialog, such as
// {Name: "Images", Patterns: []string{"*.png", "*.jpg"}}.
type FileFilter = platform.FileFilter

// ErrNoFileDialog means that the system has no file dialog to show. Linux needs
// xdg-desktop-portal for one; without it, ask for a path some other way.
var ErrNoFileDialog = platform.ErrNoFileDialog

// ShowFileDialog shows the file dialog of the system in front of the window
// that has the focus, and later calls done on the UI goroutine with the chosen
// paths, or nil if the user canceled. It must be called on the UI goroutine,
// during Run.
//
// On Windows the dialog is modal, and the windows of the application wait
// until it closes. On Linux it comes from xdg-desktop-portal, and the
// application goes on meanwhile.
func ShowFileDialog(d FileDialog, done func(paths []string, err error)) {
	ap := currentApp.Load()
	if ap == nil {
		panic("gowaku: ShowFileDialog called outside Run")
	}
	var target *window
	for _, w := range ap.windows {
		if w.closing {
			continue
		}
		if target == nil || w.focused && !target.focused ||
			w.focused == target.focused && w.native.ID() < target.native.ID() {
			target = w
		}
	}
	if target == nil {
		panic("gowaku: ShowFileDialog called with no window open")
	}
	ap.showFileDialog(target, d, done)
}

// ShowFileDialog is the package function in front of this window.
func (h *WindowHandle) ShowFileDialog(d FileDialog, done func(paths []string, err error)) {
	if h.app.windows[h.window.native.ID()] != h.window {
		panic("gowaku: ShowFileDialog called on a closed window")
	}
	h.app.showFileDialog(h.window, d, done)
}

func (ap *app) showFileDialog(w *window, d FileDialog, done func([]string, error)) {
	if done == nil {
		panic("gowaku: ShowFileDialog needs a done function")
	}
	// The platform may answer on any goroutine, and even before it returns.
	w.native.ShowFileDialog(d, func(paths []string, err error) {
		ap.post(func() { done(paths, err) })
	})
}
