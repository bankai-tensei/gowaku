package interact

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

func finger(kind core.PointerKind, id uint32, x, y float32) core.PointerEvent {
	return core.PointerEvent{Kind: kind, ID: id, Pos: geom.Offset{X: x, Y: y}, Device: input.DeviceTouch}
}

func TestLongPress_FiresOnceAndEatsTheTap(t *testing.T) {
	var taps int
	var long []input.PointerDevice
	tree := wtest.TextTree(t, basic.Align{Child: GestureDetector{
		OnTap:       func() { taps++ },
		OnLongPress: func(_ geom.Offset, d input.PointerDevice) { long = append(long, d) },
		Child:       basic.Container{Width: 50, Height: 50},
	}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	tree.DispatchPointer(finger(core.PointerDown, 1, 10, 10))
	wtest.Advance(tree, wtest.Screen, &now, 300*time.Millisecond)
	if len(long) != 0 {
		t.Fatal("too soon")
	}
	wtest.Advance(tree, wtest.Screen, &now, 300*time.Millisecond)
	tree.DispatchPointer(finger(core.PointerUp, 1, 10, 10))
	if len(long) != 1 || long[0] != input.DeviceTouch || taps != 0 {
		t.Errorf("long %v, taps %d", long, taps)
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 10, Y: 10}})
	wtest.Advance(tree, wtest.Screen, &now, 100*time.Millisecond)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 10, Y: 10}})
	wtest.Advance(tree, wtest.Screen, &now, 600*time.Millisecond)
	if taps != 1 || len(long) != 1 {
		t.Errorf("a short press taps: taps %d, long %v", taps, long)
	}
}

func TestListener_SeesThePen(t *testing.T) {
	var got []core.PointerEvent
	tree := wtest.TextTree(t, basic.Align{Child: Listener{
		OnPointer: func(e *core.PointerEvent) { got = append(got, *e) },
		Child:     basic.Container{Width: 50, Height: 50},
	}})
	wtest.Ops(tree, wtest.Screen)
	for _, k := range []core.PointerKind{core.PointerDown, core.PointerMove, core.PointerUp} {
		tree.DispatchPointer(core.PointerEvent{Kind: k, Pos: geom.Offset{X: 10, Y: 10}, Device: input.DevicePen, Pressure: 0.25})
	}
	if len(got) != 3 || got[1].Device != input.DevicePen || got[1].Pressure != 0.25 {
		t.Errorf("got %+v", got)
	}
}
