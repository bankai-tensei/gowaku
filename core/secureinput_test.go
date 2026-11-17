package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// secureHost is a Host that records secure input.
type secureHost struct {
	recordingHost
	secure []bool
}

func (h *secureHost) SetSecureInput(secure bool) { h.secure = append(h.secure, secure) }

var _ SecureInputHost = (*secureHost)(nil)

func TestFocus_RemovedTargetEndsSecureInput(t *testing.T) {
	var log []string
	var state *switcherState
	tree := NewTree(switcher{first: probe{name: "a", log: &log, focus: true, size: geom.Size{W: 10, H: 10}}, state: &state})
	t.Cleanup(tree.Dispose)
	host := &secureHost{}
	tree.SetHost(host)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	if len(host.secure) != 0 {
		t.Fatalf("focus alone must not touch secure input, got %v", host.secure)
	}
	state.SetState(func() { state.child = localProbe{got: &geom.Offset{}} })
	tree.Build()
	if len(host.secure) != 1 || host.secure[0] {
		t.Errorf("secure input must end with the focused object, got %v", host.secure)
	}
}
