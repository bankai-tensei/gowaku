package gowaku

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/platform"
)

// FileDialog configures the file dialog of the system. The zero value opens one
// existing file. See ShowFileDialog.
type FileDialog = core.FileDialog

// FileFilter is a choice of file types in a FileDialog, such as
// {Name: "Images", Patterns: []string{"*.png", "*.jpg"}}.
type FileFilter = core.FileFilter

// ErrNoFileDialog means that the system has no file dialog to show. Linux needs
// xdg-desktop-portal for one; without it, ask for a path some other way.
var ErrNoFileDialog = platform.ErrNoFileDialog

// ShowFileDialog shows the file dialog of the system in front of the window of
// ctx, and later calls done on the UI goroutine with the chosen paths, or nil
// if the user canceled. It must be called on the UI goroutine, outside a
// Build, as from a callback.
//
// The dialog goes through the core.Host of the tree of ctx, as ShowTray and
// the clipboard functions do: in a window of Run that is the dialog of the OS,
// and under a gowakutest.Tester a fake one that the test answers. A host that
// shows no file dialogs, as outside both, calls done with ErrNoFileDialog
// before ShowFileDialog returns.
//
// On Windows the dialog is modal, and the windows of the application wait
// until it closes. On Linux it comes from xdg-desktop-portal, and the
// application goes on meanwhile.
func ShowFileDialog(ctx BuildContext, d FileDialog, done func(paths []string, err error)) {
	if done == nil {
		panic("gowaku: ShowFileDialog needs a done function")
	}
	h, ok := ctx.Host().(core.FileDialogHost)
	if !ok {
		done(nil, ErrNoFileDialog)
		return
	}
	h.ShowFileDialog(d, done)
}

// ShowFileDialog is the package function in front of this window, for code
// that holds the window and no BuildContext of it.
func (h *WindowHandle) ShowFileDialog(d FileDialog, done func(paths []string, err error)) {
	if !h.app.alive(h.window) {
		panic("gowaku: ShowFileDialog called on a closed window")
	}
	if done == nil {
		panic("gowaku: ShowFileDialog needs a done function")
	}
	h.app.showFileDialog(h.window, d, done)
}

// ShowFileDialog implements core.FileDialogHost, for ShowFileDialog. A ctx
// can outlive its window, as that of a tray menu or of a timer: the dialog
// then goes in front of another window, the focused one first, and with no
// window open, done gets ErrNoFileDialog.
func (h windowHost) ShowFileDialog(d FileDialog, done func([]string, error)) {
	ap := h.w.app
	w := h.w
	if !ap.alive(w) {
		w = ap.frontWindow()
	}
	if w == nil {
		ap.post(func() { done(nil, ErrNoFileDialog) })
		return
	}
	ap.showFileDialog(w, d, done)
}

// alive reports whether w is open and not closing.
func (ap *app) alive(w *window) bool {
	return ap.windows[w.native.ID()] == w && !w.closing
}

// frontWindow returns the focused open window, or the first opened, or nil.
func (ap *app) frontWindow() *window {
	var front *window
	for _, w := range ap.windows {
		if w.closing {
			continue
		}
		if front == nil || w.focused && !front.focused ||
			w.focused == front.focused && w.native.ID() < front.native.ID() {
			front = w
		}
	}
	return front
}

func (ap *app) showFileDialog(w *window, d FileDialog, done func([]string, error)) {
	// The platform may answer on any goroutine, and even before it returns.
	w.native.ShowFileDialog(platformDialog(d), func(paths []string, err error) {
		ap.post(func() { done(paths, err) })
	})
}

// dialogFields and platformDialogFields are the fields of FileDialog and of
// platform.FileDialogOptions. The conversions below fail to build when a field
// is added to one of them, so that platformDialog is changed with it.
type (
	dialogFields struct {
		Title               string
		Save                bool
		Name                string
		Multiple, Directory bool
		Folder              string
		Filters             []FileFilter
	}
	platformDialogFields struct {
		Title               string
		Save                bool
		Name                string
		Multiple, Directory bool
		Folder              string
		Filters             []platform.FileFilter
	}
)

var (
	_ = dialogFields(FileDialog{})
	_ = platformDialogFields(platform.FileDialogOptions{})
)

// platformDialog returns d as the platform takes it.
func platformDialog(d FileDialog) platform.FileDialogOptions {
	o := platform.FileDialogOptions{
		Title: d.Title, Save: d.Save, Name: d.Name, Multiple: d.Multiple,
		Directory: d.Directory, Folder: d.Folder,
	}
	if d.Filters != nil {
		o.Filters = make([]platform.FileFilter, len(d.Filters))
		for i, f := range d.Filters {
			o.Filters[i] = platform.FileFilter(f)
		}
	}
	return o
}
