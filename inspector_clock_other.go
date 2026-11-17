//go:build gowakudebug && !windows

package gowaku

import "time"

// clockStart is the zero of frameNow.
var clockStart = time.Now()

// frameNow times the stages of frames for the Inspector. time.Now is fine
// enough outside Windows.
func frameNow() time.Duration { return time.Since(clockStart) }
