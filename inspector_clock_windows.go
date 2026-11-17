//go:build gowakudebug

package gowaku

import (
	"time"

	"github.com/bankai-tensei/gowaku/platform/win32"
)

// frameNow times the stages of frames for the Inspector. time.Now on Windows
// moves in steps of about half a millisecond, too coarse for a stage, so it
// reads the performance counter (ADR 0073).
func frameNow() time.Duration { return win32.PreciseTime() }
