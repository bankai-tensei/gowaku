package gowaku

import "github.com/bankai-tensei/gowaku/text"

// warmText holds the scripts that the first frame is likely to need. Reading a
// Japanese font takes longer than creating the GPU device.
const warmText = "aあ亜"

// shaperResult is what loadShaper delivers.
type shaperResult struct {
	shaper *text.Shaper
	err    error
}

// loadShaper finds and reads the system fonts on another goroutine, while the
// caller creates the window and the GPU device. Nothing else touches the shaper
// until it has arrived. See docs/ADR/0018-parallel-startup.md.
func loadShaper() <-chan shaperResult {
	done := make(chan shaperResult, 1)
	go func() {
		shaper, err := text.NewSystemShaper()
		if err == nil {
			shaper.Shape(warmText, text.Style{Size: 13})
		}
		done <- shaperResult{shaper, err}
	}()
	return done
}
