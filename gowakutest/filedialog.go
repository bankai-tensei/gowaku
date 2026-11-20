package gowakutest

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
)

// FileDialog is a file dialog that gowaku.ShowFileDialog showed under a
// Tester. It stands for the dialog of the system: the test reads what it asks
// for, and answers as the user would. A dialog answers once.
type FileDialog struct {
	tt       *Tester
	opts     core.FileDialog
	done     func([]string, error)
	answered bool
}

// ShowFileDialog implements core.FileDialogHost, for gowaku.ShowFileDialog: it
// records the dialog, which waits for the test to answer.
func (h *host) ShowFileDialog(d core.FileDialog, done func([]string, error)) {
	d.Filters = slices.Clone(d.Filters)
	h.dialogs = append(h.dialogs, &FileDialog{tt: h.tt, opts: d, done: done})
}

// Options returns what the dialog asks for.
func (d *FileDialog) Options() core.FileDialog { return d.opts }

// Choose answers with paths, as the user choosing them, then settles. It
// fails the test if the dialog was answered.
func (d *FileDialog) Choose(paths ...string) {
	d.tt.t.Helper()
	d.answer(slices.Clone(paths), nil)
}

// Cancel answers as the user closing the dialog, with nil paths, then
// settles.
func (d *FileDialog) Cancel() {
	d.tt.t.Helper()
	d.answer(nil, nil)
}

// Fail answers with err, as the system failing to show the dialog, then
// settles.
func (d *FileDialog) Fail(err error) {
	d.tt.t.Helper()
	d.answer(nil, err)
}

func (d *FileDialog) answer(paths []string, err error) {
	d.tt.t.Helper()
	if d.answered {
		d.tt.t.Fatal("gowakutest: the file dialog was answered already")
	}
	d.answered = true
	h := d.tt.host
	if i := slices.Index(h.dialogs, d); i >= 0 {
		h.dialogs = slices.Delete(h.dialogs, i, i+1)
	}
	d.done(paths, err)
	d.tt.Pump()
	d.tt.Settle()
}

// FileDialogs returns the file dialogs that gowaku.ShowFileDialog showed in
// the tree and the test has not answered, oldest first.
func (tt *Tester) FileDialogs() []*FileDialog { return slices.Clone(tt.host.dialogs) }

// FileDialog returns the dialog shown last and not answered. It fails the
// test if there is none.
func (tt *Tester) FileDialog() *FileDialog {
	tt.t.Helper()
	if len(tt.host.dialogs) == 0 {
		tt.t.Fatal("gowakutest: no file dialog is showing")
	}
	return tt.host.dialogs[len(tt.host.dialogs)-1]
}
