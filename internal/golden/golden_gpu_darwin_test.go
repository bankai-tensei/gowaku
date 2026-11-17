//go:build gpu && arm64

package golden

import (
	"os"
	"testing"

	"github.com/bankai-tensei/gowaku/gpu/metal"
	"github.com/bankai-tensei/gowaku/platform/cocoa"
)

// onMain runs the GPU test on the main thread, the only one where AppKit
// works; tests run on other goroutines.
var onMain = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case fn := <-onMain:
			fn()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// runOnMain runs fn on the main thread and waits for it.
func runOnMain(fn func()) {
	finished := make(chan struct{})
	onMain <- func() {
		defer close(finished)
		fn()
	}
	<-finished
}

// TestGolden_GPU holds Metal to the pictures of the CPU backend. The cleanups
// go in the order of the Windows test: the renderer, then the device, then
// the platform.
func TestGolden_GPU(t *testing.T) {
	runOnMain(func() {
		p, err := cocoa.New()
		if err != nil {
			t.Error(err)
			return
		}
		t.Cleanup(func() { runOnMain(p.Destroy) })
		d, err := metal.New(metal.Options{})
		if err != nil {
			t.Error(err)
			return
		}
		t.Cleanup(func() { runOnMain(d.Destroy) })
		checkOnWindows(t, p, "metal", d)
	})
}
