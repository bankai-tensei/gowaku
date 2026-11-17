//go:build gowakudebug

package core

// probing is true with -tags gowakudebug: the tree calls its Probe.
const probing = true

// SetProbe installs p, or removes the probe with nil. It exists only with
// -tags gowakudebug.
func (t *Tree) SetProbe(p Probe) { t.probe = p }

// Probe returns the probe installed with SetProbe, or nil. It exists only with
// -tags gowakudebug.
func (t *Tree) Probe() Probe { return t.probe }
