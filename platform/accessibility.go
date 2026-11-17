package platform

import "github.com/bankai-tensei/gowaku/semantics"

// Accessibility connects a window to the screen readers of the OS (ADR 0042).
// The reader asks about the tree whenever it likes, from what the last Update
// handed over; what it asks the application to do comes back as
// EventAccessibilityAction.
//
// All methods must be called on the UI goroutine.
type Accessibility interface {
	// Listening reports whether assistive technology has asked about the
	// window. Until it has, the application need not build the tree. The first
	// question queues an EventRedraw, so that a tree follows.
	Listening() bool
	// Update hands over the tree after a frame, with the node in focus (zero for
	// none) and what changed since the last Update. The bridge keeps root, which
	// the application must not change afterward. The root node has the ID zero
	// and stands for the window.
	Update(root *semantics.Node, focus semantics.ID, d *semantics.Diff)
}
