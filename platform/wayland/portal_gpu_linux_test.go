//go:build gpu

package wayland

import (
	"os"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/platform"
)

// This needs xdg-desktop-portal and someone to answer it: set
// GOWAKU_PORTAL_TEST and click Save in the dialog.

func portalPlatform(t *testing.T) *Platform {
	t.Helper()
	if os.Getenv("GOWAKU_PORTAL_TEST") == "" {
		t.Skip("GOWAKU_PORTAL_TEST is not set")
	}
	return newTestPlatform(t)
}

func TestFileDialog_ThroughThePortal(t *testing.T) {
	p := portalPlatform(t)
	w, err := p.CreateWindow(platform.WindowOptions{Title: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	mapWindow(t, w)
	drain(p, nil)
	if os.Getenv("GOWAKU_PORTAL_NO_PARENT") != "" {
		p.exporter = 0
	}
	type result struct {
		paths []string
		err   error
	}
	done := make(chan result, 1)
	folder := t.TempDir()
	w.ShowFileDialog(platform.FileDialogOptions{Title: "gowaku portal test", Save: true, Name: "メモ.txt", Folder: folder},
		func(paths []string, err error) { done <- result{paths, err} })
	t.Logf("parent handle %q", handleOrEmpty(w.(*Window).export))
	deadline := time.After(40 * time.Second)
	for {
		select {
		case r := <-done:
			if r.err != nil {
				t.Fatal(r.err)
			}
			if want := folder + "/メモ.txt"; len(r.paths) != 1 || r.paths[0] != want {
				t.Errorf("got %q, want %q", r.paths, want)
			}
			return
		case <-deadline:
			t.Fatal("the portal did not answer")
		default:
			// The window must go on answering the compositor meanwhile.
			p.WaitEvent(20 * time.Millisecond)
		}
	}
}

// handleOrEmpty returns the handle of e, for logs, also of a nil export.
func handleOrEmpty(e *exported) string {
	if e == nil {
		return ""
	}
	return e.handle
}
