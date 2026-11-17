package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// A frame that paints again what did not change must not allocate, with or
// without -tags gowakudebug, as long as the Inspector is closed: its hooks in
// frame are compiled out, or cost a test of a nil session.
func TestFrame_RepaintDoesNotAllocate(t *testing.T) {
	p, b := newFakes(t)
	p.script = []func(){
		func() {
			ap := currentApp.Load()
			w := ap.windows[1]
			allocs := testing.AllocsPerRun(20, func() {
				w.tree.MarkNeedsPaint()
				ap.frame(w)
			})
			if allocs != 0 {
				t.Errorf("a frame allocates %v times", allocs)
			}
		},
		closeWindow(p, 1),
	}
	home := basic.Column{Children: []Widget{
		basic.Container{Width: 40, Height: 20, Color: geom.Hex(0x6750a4)},
		basic.RepaintBoundary{Child: basic.Container{Width: 40, Height: 20, Color: geom.Hex(0x2a7d4f)}},
		basic.Padding{Insets: geom.All(4), Child: basic.SizedBox{Width: 10, Height: 10}},
	}}
	if err := run(p, b, testutil.NewShaper(t), App{Home: home}); err != nil {
		t.Fatal(err)
	}
}
