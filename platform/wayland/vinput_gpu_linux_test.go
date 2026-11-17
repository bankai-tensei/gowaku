//go:build gpu

package wayland

import (
	"bufio"
	"fmt"
	"os"
	"testing"
	"time"
)

// registryCollector remembers the globals of a compositor.
type registryCollector struct {
	wlRegistryNop
	globals map[string][2]uint32
}

// wlRegistryGlobal implements wlRegistryListener.
func (r *registryCollector) wlRegistryGlobal(name uint32, iface string, version uint32) {
	r.globals[iface] = [2]uint32{name, version}
}

// virtualPointer is a pointer of wlroots compositors that a test moves, on a
// connection of its own.
type virtualPointer struct {
	c       *conn
	pointer zwlrVirtualPointerV1
	start   time.Time
}

func newVirtualPointer(t *testing.T) *virtualPointer {
	t.Helper()
	c, err := connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.close)
	reg := &registryCollector{globals: map[string][2]uint32{}}
	registry := wlDisplay(c.display).getRegistry(c)
	c.listenWlRegistry(registry, reg)
	if err := c.roundtrip(); err != nil {
		t.Fatal(err)
	}
	m, okM := reg.globals["zwlr_virtual_pointer_manager_v1"]
	s, okS := reg.globals["wl_seat"]
	if !okM || !okS {
		t.Skip("the compositor has no virtual pointer manager or no seat")
	}
	manager := zwlrVirtualPointerManagerV1(registry.bind(c, m[0], zwlrVirtualPointerManagerV1Interface, 1))
	seat := registry.bind(c, s[0], wlSeatInterface, 1)
	v := &virtualPointer{c: c, pointer: manager.createVirtualPointer(c, seat), start: time.Now()}
	if err := c.roundtrip(); err != nil {
		t.Fatal(err)
	}
	return v
}

func (v *virtualPointer) stamp() uint32 { return uint32(time.Since(v.start).Milliseconds()) }

// moveTo moves to (x, y) of a layout of the given size, and waits until the
// compositor has it.
func (v *virtualPointer) moveTo(t *testing.T, x, y, width, height uint32) {
	v.pointer.motionAbsolute(v.c, v.stamp(), x, y, width, height)
	v.pointer.frame(v.c)
	if err := v.c.roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// TestVirtualPointer is a tool for driving a headless compositor, not a
// test. A headless wlroots compositor has no pointer, so that the seat offers
// none to clients; this holds a virtual one while it reads commands, one per
// line, from the FIFO named by GOWAKU_VPOINTER:
//
//	abs x y width height   move to (x, y) of a layout of that size
//	down / up              press or release the primary button
//	wheel n                turn the wheel n notches, positive towards the user
//	quit
func TestVirtualPointer(t *testing.T) {
	fifo := os.Getenv("GOWAKU_VPOINTER")
	if fifo == "" {
		t.Skip("GOWAKU_VPOINTER is not set")
	}
	c, err := connect()
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	reg := &registryCollector{globals: map[string][2]uint32{}}
	registry := wlDisplay(c.display).getRegistry(c)
	c.listenWlRegistry(registry, reg)
	if err := c.roundtrip(); err != nil {
		t.Fatal(err)
	}
	m, okM := reg.globals["zwlr_virtual_pointer_manager_v1"]
	s, okS := reg.globals["wl_seat"]
	if !okM || !okS {
		t.Fatal("the compositor has no virtual pointer manager or no seat")
	}
	manager := zwlrVirtualPointerManagerV1(registry.bind(c, m[0], zwlrVirtualPointerManagerV1Interface, 1))
	seat := registry.bind(c, s[0], wlSeatInterface, 1)
	pointer := manager.createVirtualPointer(c, seat)
	if err := c.roundtrip(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	stamp := func() uint32 { return uint32(time.Since(start).Milliseconds()) }

	f, err := os.Open(fifo)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		var x, y, w, h uint32
		var n int32
		line := lines.Text()
		switch {
		case line == "quit":
			return
		case line == "down", line == "up":
			state := uint32(0)
			if line == "down" {
				state = 1
			}
			pointer.button(c, stamp(), btnLeft, state)
		case scan(line, "abs %d %d %d %d", &x, &y, &w, &h):
			pointer.motionAbsolute(c, stamp(), x, y, w, h)
		case scan(line, "wheel %d", &n):
			pointer.axisSource(c, 0)
			pointer.axisDiscrete(c, stamp(), axisVertical, float64(15*n), n)
		default:
			t.Logf("unknown command %q", line)
			continue
		}
		pointer.frame(c)
		if err := c.roundtrip(); err != nil {
			t.Fatal(err)
		}
	}
}

func scan(line, format string, args ...any) bool {
	n, err := fmt.Sscanf(line, format, args...)
	return err == nil && n == len(args)
}
