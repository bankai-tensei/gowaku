package main

import (
	"sync"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/nav"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
)

// Size of a new window.
const (
	windowWidth  = 1100
	windowHeight = 720
)

// gowakuShell is the shell of Run.
type gowakuShell struct{}

func (gowakuShell) post(fn func()) { gowaku.Post(fn) }

func (gowakuShell) async(work func() func()) {
	go func() {
		done := work()
		gowaku.Post(done)
	}()
}

func (gowakuShell) after(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { gowaku.Post(fn) })
}

func (gowakuShell) openWindow(title string, root core.Widget, onClose func()) error {
	_, err := gowaku.OpenWindow(gowaku.WindowOptions{Title: title, Width: windowWidth, Height: windowHeight, OnClose: onClose}, root)
	return err
}

func (gowakuShell) tearOff(e nav.TabTearOff, title string, root core.Widget, onClose func()) error {
	_, err := gowaku.TearOffWindow(e, gowaku.WindowOptions{Title: title, Width: windowWidth, Height: windowHeight, OnClose: onClose}, root)
	return err
}

func (gowakuShell) fileDialog(ctx core.BuildContext, d gowaku.FileDialog, done func([]string, error)) {
	gowaku.ShowFileDialog(ctx, d, done)
}

func (gowakuShell) window(ctx core.BuildContext) windowHandle { return gowaku.WindowOf(ctx) }

func (gowakuShell) quit() { gowaku.Quit() }

// localShell runs the application without Run, for tests and screenshots:
// what gowaku would post waits in a queue until flush, windows are recorded
// rather than opened, and the file dialog answers with dialogPaths.
type localShell struct {
	mu    sync.Mutex
	queue []func()

	opened      []core.Widget
	dialogs     []gowaku.FileDialog
	dialogPaths []string
	quitted     bool
	// closeRequest is what a window put on its close button last.
	closeRequest func() bool
}

func (l *localShell) post(fn func()) {
	l.mu.Lock()
	l.queue = append(l.queue, fn)
	l.mu.Unlock()
}

func (l *localShell) async(work func() func()) { l.post(work()) }

func (l *localShell) after(_ time.Duration, fn func()) { l.post(fn) }

func (l *localShell) openWindow(_ string, root core.Widget, _ func()) error {
	l.opened = append(l.opened, root)
	return nil
}

func (l *localShell) tearOff(_ nav.TabTearOff, _ string, root core.Widget, _ func()) error {
	l.opened = append(l.opened, root)
	return nil
}

func (l *localShell) fileDialog(_ core.BuildContext, d gowaku.FileDialog, done func([]string, error)) {
	l.dialogs = append(l.dialogs, d)
	paths := l.dialogPaths
	l.post(func() { done(paths, nil) })
}

func (l *localShell) window(core.BuildContext) windowHandle { return localWindow{l} }

func (l *localShell) quit() { l.quitted = true }

// flush runs what was posted, and what that posts in turn, and reports
// whether there was anything.
func (l *localShell) flush() bool {
	ran := false
	for range 100 {
		l.mu.Lock()
		q := l.queue
		l.queue = nil
		l.mu.Unlock()
		if len(q) == 0 {
			return ran
		}
		for _, fn := range q {
			fn()
		}
		ran = true
	}
	return ran
}

// localWindow is a window that is not there.
type localWindow struct{ shell *localShell }

func (localWindow) SetTitle(string)                    {}
func (localWindow) Close()                             {}
func (localWindow) MoveTo(nav.TabTearOff)              {}
func (localWindow) Activate()                          {}
func (l localWindow) SetOnCloseRequest(fn func() bool) { l.shell.closeRequest = fn }
