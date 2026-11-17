package core

// Probe hears what a tree does in a frame. Only builds with -tags gowakudebug
// can install one (Tree.SetProbe); in other builds the calls are compiled out.
// The methods run on the UI goroutine, in the middle of the frame, and must not
// change the tree. See docs/ADR/0073-inspector.md.
type Probe interface {
	// Rebuilt runs after el ran Build, or was updated to a new widget.
	Rebuilt(el ElementView)
	// LaidOut runs after ro ran Layout.
	LaidOut(ro RenderObject)
	// Recorded runs after the layer of r was recorded again.
	Recorded(r *RenderRepaintBoundary)
}
