//go:build gpu

package gowaku

import (
	"testing"
	"time"
)

// This is the only test of Run itself: real window, real GPU, and a goroutine
// outside the UI goroutine that reaches SetState through Post. Everything that
// touches lastCounter runs on the UI goroutine.
func TestRun_RealWindow(t *testing.T) {
	lastCounter = nil
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond): // real time: let frames happen
				Post(func() {
					if lastCounter == nil || !lastCounter.Mounted() {
						return
					}
					lastCounter.SetState(func() { lastCounter.n++ })
					if lastCounter.n == 5 {
						Quit()
					}
				})
			}
		}
	}()
	if err := Run(App{Title: "gowaku test", Width: 200, Height: 200, Home: counter{}}); err != nil {
		t.Fatal(err)
	}
	if lastCounter.n != 5 {
		t.Errorf("got count %d, want 5", lastCounter.n)
	}
	if lastCounter.Mounted() {
		t.Error("state must be disposed after Run")
	}
}
