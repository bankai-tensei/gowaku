//go:build !gowakudebug

package gowaku

import (
	"time"

	"github.com/bankai-tensei/gowaku/platform"
)

// inspecting is false outside -tags gowakudebug, so that the compiler drops
// every hook of the Inspector. See docs/ADR/0073-inspector.md.
const inspecting = false

// OpenInspector opens the Inspector, a window that shows the trees, the
// frames and the pictures of the other windows. F12 in any window does the
// same, and closes it again. Before Run, it opens the Inspector with the first
// window. It is safe to call from any goroutine.
//
// It does something only in builds with -tags gowakudebug; in other builds it
// does nothing, and no code of the Inspector is linked in.
func OpenInspector() {}

type inspectorState struct{}

type frameClock struct{}

type stage uint8

func (*frameClock) lap(stage) {}

const (
	stageBuild stage = iota
	stageLayout
	stagePaint
	stageBatch
	stagePresent
)

func (*app) inspectStart()                                {}
func (*app) inspectKey(*window, *platform.Event) bool     { return false }
func (*app) inspectPointer(*window, *platform.Event) bool { return false }
func (*app) beginInspected(*window) frameClock            { return frameClock{} }
func (*app) endInspected(*window, *frameClock, bool)      {}
func (*app) overlaid(*window) bool                        { return false }
func (*app) drawOverlay(*window)                          {}
func (*app) inspectPumped()                               {}
func (*app) inspectWake() (time.Duration, bool)           { return 0, false }
func (*app) inspectForget(*window)                        {}
