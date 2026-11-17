//go:build gpu

package win32

import (
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/platform"
)

// These show the real dialog and press its buttons, so they run with the GPU
// tests, locally only. A window belongs to its thread, so no t.Run.

var (
	procFindWindowW = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")
)

const (
	WM_COMMAND = 0x0111
	IDOK       = 1
	IDCANCEL   = 2
)

// press waits for the dialog titled title and presses a button of it, again
// until it closes: a dialog that has only just appeared ignores commands.
func press(t *testing.T, title string, button uintptr) {
	name, _ := windows.UTF16PtrFromString(title) //nolint:errcheck // no NUL
	go func() {
		seen := false
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
			hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(name)))
			switch {
			case hwnd != 0:
				seen = true
				procPostMessageW.Call(hwnd, WM_COMMAND, button, 0)
			case seen:
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Error("the dialog did not appear, or did not close")
	}()
}

func showDialog(t *testing.T, opts platform.FileDialogOptions) ([]string, error) {
	t.Helper()
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	w, err := p.CreateWindow(platform.WindowOptions{Title: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	var called bool
	w.ShowFileDialog(opts, func(p []string, e error) { paths, err, called = p, e, true })
	if !called {
		t.Fatal("done did not run before ShowFileDialog returned")
	}
	return paths, err
}

func TestShowFileDialog_SaveReturnsThePath(t *testing.T) {
	want := filepath.Join(t.TempDir(), "日本語のメモ.txt")
	press(t, "gowaku save test", IDOK)
	paths, err := showDialog(t, platform.FileDialogOptions{
		Title: "gowaku save test", Save: true, Name: want,
		Filters: []platform.FileFilter{{Name: "Text", Patterns: []string{"*.txt"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != want {
		t.Errorf("got %q, want %q", paths, want)
	}
}

func TestShowFileDialog_CancelReturnsNil(t *testing.T) {
	press(t, "gowaku cancel test", IDCANCEL)
	paths, err := showDialog(t, platform.FileDialogOptions{Title: "gowaku cancel test", Folder: t.TempDir()})
	if err != nil || paths != nil {
		t.Errorf("got %q, %v; want nil, nil", paths, err)
	}
}
