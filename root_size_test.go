package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// mediaProbe counts its builds, and keeps the window size if it reads it.
type mediaProbe struct {
	StatelessBase
	builds *int
	size   *geom.Size
}

func (p mediaProbe) Build(ctx BuildContext) Widget {
	*p.builds++
	if p.size != nil {
		*p.size = basic.WindowSizeOf(ctx)
	} else {
		_ = basic.ReduceMotionOf(ctx)
	}
	return basic.SizedBox{}
}

func TestRun_ResizeRebuildsOnlyWhatReadsTheSize(t *testing.T) {
	var sized, other int
	var size geom.Size
	home := basic.Column{Children: []Widget{
		mediaProbe{builds: &sized, size: &size},
		mediaProbe{builds: &other},
	}}
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			sized, other = 0, 0
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 640, PhysicalHeight: 480, Scale: 2})
		},
		func() {
			if sized != 1 || other != 0 {
				t.Errorf("a resize built the size reader %d times and the other %d times, want 1 and 0", sized, other)
			}
			if size != (geom.Size{W: 320, H: 240}) {
				t.Errorf("WindowSizeOf is %v, want 320×240", size)
			}
			// The same size again builds nothing.
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 640, PhysicalHeight: 480, Scale: 2})
		},
		func() {
			if sized != 1 {
				t.Errorf("a resize to the same size built %d times", sized)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
}
